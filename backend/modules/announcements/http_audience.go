package announcements

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func actorHas(values []string, value string) bool {
	for _, candidate := range values {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}
func actorIsAdmin(actor Actor) bool {
	return actorHas(actor.RealmRoles, "admin") || actorHas(actor.RealmRoles, "super_admin")
}

// visible owns module-specific audience policy. Platform capabilities supply
// identity, group membership and contextual expansion, not Announcements DTOs.
func (h *httpHandler) visible(ctx context.Context, actor Actor, item Announcement) (bool, error) {
	now := time.Now()
	if item.DeletedAt.Valid || item.Status != AnnouncementStatusPUBLISHED ||
		item.PublishAt.Valid && item.PublishAt.Time.After(now) ||
		item.ExpiresAt.Valid && !item.ExpiresAt.Time.After(now) {
		return false, nil
	}
	s := h.announcementService
	switch item.AudienceType {
	case AnnouncementAudienceTypeALLUSERS:
		return true, nil
	case AnnouncementAudienceTypeADMINSONLY:
		return actorIsAdmin(actor), nil
	case AnnouncementAudienceTypeSPECIFICUSERS:
		ids, err := s.repo.ListUserAudience(ctx, item.ID)
		if err != nil {
			return false, err
		}
		for _, id := range ids {
			if id == actor.UserID {
				return true, nil
			}
		}
	case AnnouncementAudienceTypeSPECIFICROLES:
		roles, err := s.repo.ListRoleAudience(ctx, item.ID)
		if err != nil {
			return false, err
		}
		for _, role := range roles {
			if actorHas(actor.RealmRoles, role) {
				return true, nil
			}
		}
	case AnnouncementAudienceTypeSPECIFICCLIENTS:
		ids, err := s.repo.ListClientAudience(ctx, item.ID)
		if err != nil {
			return false, err
		}
		for _, id := range ids {
			if actorHas(actor.ClientIDs, id.String()) {
				return true, nil
			}
		}
	case AnnouncementAudienceTypeSPECIFICGROUPS:
		if s.groups == nil {
			return false, fmt.Errorf("group directory unavailable")
		}
		ids, err := s.repo.ListGroupAudience(ctx, item.ID)
		if err != nil {
			return false, err
		}
		for _, id := range ids {
			members, err := s.groups.ListGroupMembers(ctx, id.String())
			if err != nil {
				return false, err
			}
			for _, member := range members {
				if strings.EqualFold(member.UserID, actor.UserID.String()) {
					return true, nil
				}
			}
		}
	case AnnouncementAudienceTypeSPECIFICHEALTHCONTEXTS, AnnouncementAudienceTypeHEALTHCONTEXTANDDESCENDANTS:
		if s.healthContexts == nil {
			return false, fmt.Errorf("health audience capability unavailable")
		}
		mappings, err := s.repo.ListHealthContextAudience(ctx, item.ID)
		if err != nil {
			return false, err
		}
		for _, mapping := range mappings {
			ids, err := s.healthContexts.ListAudienceUserIDs(ctx, []uuid.UUID{mapping.ContextNodeID}, mapping.IncludeDescendants)
			if err != nil {
				return false, err
			}
			if actorHas(ids, actor.UserID.String()) {
				return true, nil
			}
		}
	}
	return false, nil
}

// reportSkippedAnnouncement records an announcement left out of a feed because
// its audience could not be checked, through the host's error reporter.
func (h *httpHandler) reportSkippedAnnouncement(c *httpRequest, item Announcement, err error) {
	if c.streamError != nil {
		c.streamError(c.Request, fmt.Errorf("announcement %s hidden: audience check failed: %w", item.ID, err))
	}
}

func (h *httpHandler) listVisible(c *httpRequest) {
	role := strings.TrimSpace(c.Param("role_name"))
	if role == "" {
		role = strings.TrimSpace(c.Query("role"))
	}
	client := strings.TrimSpace(c.Param("client_id"))
	if client == "" {
		client = strings.TrimSpace(c.Query("client_id"))
	}
	if role != "" && !actorHas(c.actor.RealmRoles, role) {
		c.reject(ErrForbidden)
		return
	}
	if client != "" {
		id, err := uuid.Parse(client)
		if err != nil || id == uuid.Nil {
			c.fail(400, "BAD_REQUEST", "invalid client_id")
			return
		}
		client = id.String()
		if !actorHas(c.actor.ClientIDs, client) {
			c.reject(ErrForbidden)
			return
		}
	}
	limit, offset := normalizeLimit(getPageLimit(c, 20)), normalizeOffset(getPageOffset(c))
	result := make([]Announcement, 0, limit)
	// Apply pagination AFTER audience filtering, never to an unfiltered union.
	for page := int32(0); ; page += 100 {
		items, err := h.announcementService.repo.ListActivePublished(c.Request.Context(), ListActivePublishedAnnouncementsParams{Limit: 100, Offset: page})
		if err != nil {
			c.fail(500, "INTERNAL_ERROR", "failed to fetch announcements")
			return
		}
		for _, item := range items {
			allowed, err := h.visible(c.Request.Context(), c.actor, item)
			if err != nil {
				// Fail closed for this item only: one group or health-context
				// lookup failing must not take down the whole feed.
				h.reportSkippedAnnouncement(c, item, err)
				continue
			}
			if !allowed {
				continue
			}
			// Optional selectors narrow the verified audience; they never grant access.
			if role != "" || client != "" {
				selected, err := h.selected(c.Request.Context(), item, role, client)
				if err != nil {
					h.reportSkippedAnnouncement(c, item, err)
					continue
				}
				if !selected {
					continue
				}
			}
			if offset > 0 {
				offset--
				continue
			}
			result = append(result, item)
			if len(result) == int(limit) {
				c.ok(http.StatusOK, h.announcementResponsesWithAttachments(c, result, false))
				return
			}
		}
		if len(items) < 100 {
			break
		}
		if err := c.Request.Context().Err(); err != nil {
			c.reject(err)
			return
		}
	}
	c.ok(http.StatusOK, h.announcementResponsesWithAttachments(c, result, false))
}

// selected matches explicit role/client selectors after actor visibility has passed.

func (h *httpHandler) selected(ctx context.Context, item Announcement, role, client string) (bool, error) {
	switch item.AudienceType {
	case AnnouncementAudienceTypeALLUSERS:
		return true, nil
	case AnnouncementAudienceTypeADMINSONLY:
		return strings.EqualFold(role, "admin"), nil
	case AnnouncementAudienceTypeSPECIFICROLES:
		if role == "" {
			return false, nil
		}
		roles, err := h.announcementService.repo.ListRoleAudience(ctx, item.ID)
		return actorHas(roles, role), err
	case AnnouncementAudienceTypeSPECIFICCLIENTS:
		if client == "" {
			return false, nil
		}
		ids, err := h.announcementService.repo.ListClientAudience(ctx, item.ID)
		if err != nil {
			return false, err
		}
		for _, id := range ids {
			if strings.EqualFold(id.String(), client) {
				return true, nil
			}
		}
	}
	return false, nil
}
