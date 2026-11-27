package announcements

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type httpHandler struct {
	announcementService *Service
	auditService        AuditRecorder
}

func newHTTPHandler(
	announcementService *Service,
	auditService AuditRecorder,
) *httpHandler {
	return &httpHandler{
		announcementService: announcementService,
		auditService:        auditService,
	}
}

// rejectServerPathAttachments refuses request attachments that name a server
// file. Clients must send file contents (data_base64) or upload attachments;
// a path would let a caller email arbitrary server files to themselves.
func rejectServerPathAttachments(c *httpRequest, attachments []announcementAttachmentRequest) bool {
	for _, attachment := range attachments {
		if strings.TrimSpace(attachment.Path) != "" {
			c.fail(http.StatusBadRequest, "BAD_REQUEST", "attachment path is not supported; send data_base64 or upload the attachment")
			return true
		}
	}
	return false
}

func getPageLimit(c *httpRequest, defaultValue int32) int32 {
	limit := defaultValue

	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}

	return limit
}

func getPageOffset(c *httpRequest) int32 {
	if raw := c.Query("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			return int32(parsed)
		}
	}

	return 0
}

func getAnnouncementID(c *httpRequest) (uuid.UUID, bool) {
	idParam := c.Param("id")
	if idParam == "" {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "announcement id is required")
		return uuid.Nil, false
	}

	announcementID, err := uuid.Parse(idParam)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "invalid announcement id")
		return uuid.Nil, false
	}

	return announcementID, true
}

func getAnnouncementAttachmentID(c *httpRequest) (uuid.UUID, bool) {
	idParam := c.Param("attachmentId")
	if idParam == "" {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "attachment id is required")
		return uuid.Nil, false
	}

	attachmentID, err := uuid.Parse(idParam)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "invalid attachment id")
		return uuid.Nil, false
	}

	return attachmentID, true
}

func parseAnnouncementAudienceSelection(
	reqAudienceType string,
	clientIDValues []string,
	userIDValues []string,
	groupIDValues []string,
	healthContextIDValues []string,
) ([]uuid.UUID, []uuid.UUID, []uuid.UUID, []uuid.UUID, bool) {
	clientIDs, err := parseUUIDList(clientIDValues)
	if err != nil {
		return nil, nil, nil, nil, false
	}

	userIDs, err := parseUUIDList(userIDValues)
	if err != nil {
		return nil, nil, nil, nil, false
	}

	groupIDs, err := parseUUIDList(groupIDValues)
	if err != nil {
		return nil, nil, nil, nil, false
	}
	healthContextIDs, err := parseUUIDList(healthContextIDValues)
	if err != nil {
		return nil, nil, nil, nil, false
	}

	audienceType := strings.ToUpper(strings.TrimSpace(reqAudienceType))
	switch audienceType {
	case "SPECIFIC_CLIENTS":
		if len(clientIDs) == 0 {
			return nil, nil, nil, nil, false
		}
	case "SPECIFIC_USERS":
		if len(userIDs) == 0 {
			return nil, nil, nil, nil, false
		}
	case "SPECIFIC_GROUPS":
		if len(groupIDs) == 0 {
			return nil, nil, nil, nil, false
		}
	case "SPECIFIC_HEALTH_CONTEXTS", "HEALTH_CONTEXT_AND_DESCENDANTS":
		if len(healthContextIDs) == 0 {
			return nil, nil, nil, nil, false
		}
	}

	return clientIDs, userIDs, groupIDs, healthContextIDs, true
}

func getCurrentUserID(c *httpRequest) (uuid.UUID, bool) {
	userID := uuid.NullUUID{UUID: c.actor.UserID, Valid: c.actor.UserID != uuid.Nil}
	if !userID.Valid {
		c.fail(http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated")
		return uuid.Nil, false
	}

	return userID.UUID, true
}

func (h *httpHandler) announcementResponsesWithAttachments(
	ctx *httpRequest,
	items []announcement,
	adminLinks bool,
) []AnnouncementResponse {
	res := make([]AnnouncementResponse, len(items))
	for i, item := range items {
		attachments, err := h.announcementService.ListAttachments(ctx.Request.Context(), item.ID)
		if err != nil {
			res[i] = toAnnouncementResponse(item)
			continue
		}
		if adminLinks {
			res[i] = toAnnouncementResponseWithAttachments(item, attachments)
			h.attachAnnouncementAudience(ctx, item.ID, &res[i])
		} else {
			res[i] = toUserAnnouncementResponseWithAttachments(item, attachments)
		}
	}
	return res
}

func (h *httpHandler) attachAnnouncementAudience(
	c *httpRequest,
	announcementID uuid.UUID,
	res *AnnouncementResponse,
) {
	if h == nil || h.announcementService == nil || res == nil || announcementID == uuid.Nil {
		return
	}

	clientIDs, err := h.announcementService.ListClientAudience(c.Request.Context(), announcementID)
	if err != nil {
		return
	}

	roleNames, err := h.announcementService.ListRoleAudience(c.Request.Context(), announcementID)
	if err != nil {
		return
	}

	userIDs, err := h.announcementService.ListUserAudience(c.Request.Context(), announcementID)
	if err != nil {
		return
	}

	groupIDs, err := h.announcementService.ListGroupAudience(c.Request.Context(), announcementID)
	if err != nil {
		return
	}

	contextAudience, err := h.announcementService.ListHealthContextAudience(c.Request.Context(), announcementID)
	if err != nil {
		contextAudience = nil
	}
	*res = withAnnouncementAudience(*res, clientIDs, roleNames, userIDs, groupIDs, contextAudience)
}

func (h *httpHandler) ListAnnouncementsAdmin(c *httpRequest) {
	limit := getPageLimit(c, 20)
	offset := getPageOffset(c)

	items, err := h.announcementService.ListAnnouncementsAdminPage(
		c.Request.Context(),
		limit,
		offset,
	)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch announcements")
		return
	}

	c.ok(http.StatusOK, h.announcementResponsesWithAttachments(c, items, true))
}

func (h *httpHandler) GetAnnouncementByID(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	item, err := h.announcementService.GetAnnouncementByID(c.Request.Context(), announcementID)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch announcement")
		return
	}

	attachments, _ := h.announcementService.ListAttachments(c.Request.Context(), item.ID)
	res := toAnnouncementResponseWithAttachments(item, attachments)
	h.attachAnnouncementAudience(c, item.ID, &res)
	c.ok(http.StatusOK, res)
}

func (h *httpHandler) ListPublicAnnouncements(c *httpRequest) {
	ctx := c.Request.Context()

	limit, err := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 32)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}

	offset, err := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 32)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}

	announcements, err := h.announcementService.ListPublicAnnouncements(
		ctx,
		int32(limit),
		int32(offset),
	)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load public announcements")
		return
	}

	c.ok(http.StatusOK, h.announcementResponsesWithAttachments(c, announcements, false))
}

func (h *httpHandler) CreateAnnouncement(c *httpRequest) {
	var req createAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	publishAt, err := nullableTime(req.PublishAt)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "publish_at must be a valid RFC3339 datetime")
		return
	}

	expiresAt, err := nullableTime(req.ExpiresAt)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "expires_at must be a valid RFC3339 datetime")
		return
	}

	clientIDs, userIDs, groupIDs, healthContextIDs, ok := parseAnnouncementAudienceSelection(
		req.AudienceType,
		req.ClientIDs,
		req.UserIDs,
		req.GroupIDs,
		req.HealthContextIDs,
	)
	if !ok {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "selected announcement audience is invalid or empty")
		return
	}
	if len(groupIDs) > 0 {
		if err := h.announcementService.ValidateGroupAudience(c.Request.Context(), groupIDs); err != nil {
			c.fail(http.StatusBadRequest, "BAD_REQUEST", "one or more selected groups do not exist")
			return
		}
	}
	includeContextDescendants := strings.EqualFold(req.AudienceType, "HEALTH_CONTEXT_AND_DESCENDANTS")
	healthContextRecipientCount := 0
	if len(healthContextIDs) > 0 {
		recipients, err := h.announcementService.ResolveHealthContextRecipients(
			c.Request.Context(),
			userID.String(),
			healthContextIDs,
			includeContextDescendants,
		)
		if err != nil {
			c.fail(http.StatusForbidden, "HEALTH_CONTEXT_FORBIDDEN", "one or more selected health contexts are not accessible")
			return
		}
		if len(recipients) == 0 {
			c.fail(http.StatusBadRequest, "EMPTY_AUDIENCE", "selected health contexts contain no eligible recipients")
			return
		}
		healthContextRecipientCount = len(recipients)
	}

	input := CreateAnnouncementInput{
		Title:         strings.TrimSpace(req.Title),
		Message:       strings.TrimSpace(req.Message),
		Summary:       nullableString(req.Summary),
		Level:         req.Level,
		Tag:           nullableString(req.Tag),
		LinkURL:       nullableString(req.LinkURL),
		LinkLabel:     nullableString(req.LinkLabel),
		Priority:      req.Priority,
		IsPinned:      req.IsPinned,
		Status:        req.Status,
		PublishAt:     publishAt,
		ExpiresAt:     expiresAt,
		AudienceType:  req.AudienceType,
		NotifyByEmail: req.NotifyByEmail,
		NotifyBySMS:   req.NotifyBySMS,
		SMSMessage:    nullableString(req.SMSMessage),
		CreatedBy:     userID,
	}

	item, err := h.announcementService.CreateAnnouncementFromInput(c.Request.Context(), input)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create announcement")
		return
	}

	if len(clientIDs) > 0 {
		if err := h.announcementService.ReplaceClientAudience(c.Request.Context(), item.ID, clientIDs); err != nil {
			c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save client audience")
			return
		}
	}

	if len(req.RoleNames) > 0 {
		if err := h.announcementService.ReplaceRoleAudience(c.Request.Context(), item.ID, req.RoleNames); err != nil {
			c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save role audience")
			return
		}
	}

	if len(userIDs) > 0 {
		if err := h.announcementService.ReplaceUserAudience(c.Request.Context(), item.ID, userIDs); err != nil {
			c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save user audience")
			return
		}
	}

	if len(groupIDs) > 0 {
		if err := h.announcementService.ReplaceGroupAudience(c.Request.Context(), item.ID, groupIDs); err != nil {
			c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save group audience")
			return
		}
	}
	if err := h.announcementService.ReplaceHealthContextAudience(
		c.Request.Context(),
		item.ID,
		healthContextIDs,
		includeContextDescendants,
	); err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save health context audience")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_CREATED",
			map[string]any{
				"announcement_id":                  item.ID.String(),
				"title":                            item.Title,
				"level":                            item.Level,
				"priority":                         item.Priority,
				"status":                           item.Status,
				"is_pinned":                        item.IsPinned,
				"notify_by_email":                  item.NotifyByEmail,
				"notify_by_sms":                    req.NotifyBySMS,
				"health_context_ids":               uuidStrings(healthContextIDs),
				"include_context_descendants":      includeContextDescendants,
				"resolved_context_recipient_count": healthContextRecipientCount,
			},
		)
	}

	res := toAnnouncementResponse(item)
	h.attachAnnouncementAudience(c, item.ID, &res)
	c.ok(http.StatusCreated, res)
}

func (h *httpHandler) PreviewAnnouncementAudience(c *httpRequest) {
	var request announcementAudiencePreviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.fail(http.StatusBadRequest, "INVALID_INPUT", "invalid audience preview")
		return
	}

	includeDescendants := strings.EqualFold(
		request.AudienceType,
		"HEALTH_CONTEXT_AND_DESCENDANTS",
	)
	if !includeDescendants &&
		!strings.EqualFold(request.AudienceType, "SPECIFIC_HEALTH_CONTEXTS") {
		c.fail(
			http.StatusBadRequest,
			"INVALID_AUDIENCE_TYPE",
			"audience preview currently supports health-context audiences",
		)
		return
	}

	contextIDs, err := parseUUIDList(request.HealthContextIDs)
	if err != nil || len(contextIDs) == 0 {
		c.fail(
			http.StatusBadRequest,
			"INVALID_HEALTH_CONTEXT",
			"select at least one valid health context",
		)
		return
	}

	recipients, err := h.announcementService.ResolveHealthContextRecipients(
		c.Request.Context(),
		c.GetString("user_id"),
		contextIDs,
		includeDescendants,
	)
	if err != nil {
		c.fail(
			http.StatusForbidden,
			"HEALTH_CONTEXT_FORBIDDEN",
			"one or more selected health contexts are not accessible",
		)
		return
	}

	if h.auditService != nil {
		actorID, actorErr := uuid.Parse(strings.TrimSpace(c.GetString("user_id")))
		if actorErr == nil {
			_ = h.auditService.Log(
				c.Request.Context(),
				uuid.NullUUID{UUID: actorID, Valid: true},
				"ANNOUNCEMENT_AUDIENCE_PREVIEWED",
				map[string]any{
					"health_context_ids":          uuidStrings(contextIDs),
					"include_context_descendants": includeDescendants,
					"resolved_recipient_count":    len(recipients),
				},
			)
		}
	}

	c.ok(http.StatusOK, AnnouncementAudiencePreviewResponse{
		RecipientCount: len(recipients),
	})
}

func (h *httpHandler) UpdateAnnouncement(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	var req updateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	publishAt, err := nullableTime(req.PublishAt)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "publish_at must be a valid RFC3339 datetime")
		return
	}

	expiresAt, err := nullableTime(req.ExpiresAt)
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "expires_at must be a valid RFC3339 datetime")
		return
	}

	clientIDs, userIDs, groupIDs, healthContextIDs, ok := parseAnnouncementAudienceSelection(
		req.AudienceType,
		req.ClientIDs,
		req.UserIDs,
		req.GroupIDs,
		req.HealthContextIDs,
	)
	if !ok {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "selected announcement audience is invalid or empty")
		return
	}
	if len(groupIDs) > 0 {
		if err := h.announcementService.ValidateGroupAudience(c.Request.Context(), groupIDs); err != nil {
			c.fail(http.StatusBadRequest, "BAD_REQUEST", "one or more selected groups do not exist")
			return
		}
	}
	includeContextDescendants := strings.EqualFold(req.AudienceType, "HEALTH_CONTEXT_AND_DESCENDANTS")
	healthContextRecipientCount := 0
	if len(healthContextIDs) > 0 {
		recipients, err := h.announcementService.ResolveHealthContextRecipients(
			c.Request.Context(),
			userID.String(),
			healthContextIDs,
			includeContextDescendants,
		)
		if err != nil {
			c.fail(http.StatusForbidden, "HEALTH_CONTEXT_FORBIDDEN", "one or more selected health contexts are not accessible")
			return
		}
		if len(recipients) == 0 {
			c.fail(http.StatusBadRequest, "EMPTY_AUDIENCE", "selected health contexts contain no eligible recipients")
			return
		}
		healthContextRecipientCount = len(recipients)
	}

	input := UpdateAnnouncementInput{
		ID:            announcementID,
		Title:         strings.TrimSpace(req.Title),
		Message:       strings.TrimSpace(req.Message),
		Summary:       nullableString(req.Summary),
		Level:         req.Level,
		Tag:           nullableString(req.Tag),
		LinkURL:       nullableString(req.LinkURL),
		LinkLabel:     nullableString(req.LinkLabel),
		Priority:      req.Priority,
		IsPinned:      req.IsPinned,
		PublishAt:     publishAt,
		ExpiresAt:     expiresAt,
		AudienceType:  req.AudienceType,
		NotifyByEmail: req.NotifyByEmail,
		NotifyBySMS:   req.NotifyBySMS,
		SMSMessage:    nullableString(req.SMSMessage),
		UpdatedBy:     userID,
	}

	item, err := h.announcementService.UpdateAnnouncementFromInput(c.Request.Context(), input)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update announcement")
		return
	}

	if err := h.announcementService.ReplaceClientAudience(c.Request.Context(), item.ID, clientIDs); err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update client audience")
		return
	}

	if err := h.announcementService.ReplaceRoleAudience(c.Request.Context(), item.ID, req.RoleNames); err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update role audience")
		return
	}

	if err := h.announcementService.ReplaceUserAudience(c.Request.Context(), item.ID, userIDs); err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update user audience")
		return
	}

	if err := h.announcementService.ReplaceGroupAudience(c.Request.Context(), item.ID, groupIDs); err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update group audience")
		return
	}
	if err := h.announcementService.ReplaceHealthContextAudience(
		c.Request.Context(),
		item.ID,
		healthContextIDs,
		includeContextDescendants,
	); err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update health context audience")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_UPDATED",
			map[string]any{
				"announcement_id":                  item.ID.String(),
				"title":                            item.Title,
				"level":                            item.Level,
				"priority":                         item.Priority,
				"status":                           item.Status,
				"is_pinned":                        item.IsPinned,
				"notify_by_email":                  item.NotifyByEmail,
				"notify_by_sms":                    req.NotifyBySMS,
				"health_context_ids":               uuidStrings(healthContextIDs),
				"include_context_descendants":      includeContextDescendants,
				"resolved_context_recipient_count": healthContextRecipientCount,
			},
		)
	}

	res := toAnnouncementResponse(item)
	h.attachAnnouncementAudience(c, item.ID, &res)
	c.ok(http.StatusOK, res)
}

func (h *httpHandler) PublishAnnouncementNow(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	var req publishAnnouncementRequest
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
			c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
			return
		}
	}
	if rejectServerPathAttachments(c, req.Attachments) {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	item, err := h.announcementService.PublishAnnouncementNow(
		c.Request.Context(),
		announcementID,
		userID,
		announcementEmailOptionsFromPublishRequest(req),
	)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to publish announcement")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_PUBLISHED",
			map[string]any{
				"announcement_id":                    item.ID.String(),
				"title":                              item.Title,
				"notify_by_email":                    item.NotifyByEmail,
				"email_notification_sent_at_present": item.EmailNotificationSentAt.Valid,
				"notify_by_sms":                      item.NotifyBySms,
				"sms_notification_queued_at_present": item.SmsNotificationQueuedAt.Valid,
			},
		)
	}

	res := toAnnouncementResponse(item)
	c.ok(http.StatusOK, res)
}

func (h *httpHandler) MoveAnnouncementToDraft(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	item, err := h.announcementService.MoveAnnouncementToDraftByUser(
		c.Request.Context(),
		announcementID,
		userID,
	)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to draft announcement")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_DRAFTED",
			map[string]any{
				"announcement_id": item.ID.String(),
			},
		)
	}

	res := toAnnouncementResponse(item)
	c.ok(http.StatusOK, res)
}

func (h *httpHandler) ScheduleAnnouncement(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	var req scheduleAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}
	if rejectServerPathAttachments(c, req.Attachments) {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	publishAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.PublishAt))
	if err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "publish_at must be a valid RFC3339 datetime")
		return
	}

	item, err := h.announcementService.ScheduleAnnouncementByUser(
		c.Request.Context(),
		announcementID,
		publishAt,
		userID,
		announcementEmailOptionsFromScheduleRequest(req, publishAt),
	)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to schedule announcement")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_SCHEDULED",
			map[string]any{
				"announcement_id": item.ID.String(),
				"title":           item.Title,
				"publish_at":      item.PublishAt,
				"notify_by_sms":   item.NotifyBySms,
			},
		)
	}

	c.ok(http.StatusOK, toAnnouncementResponse(item))
}

func (h *httpHandler) ArchiveAnnouncement(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	item, err := h.announcementService.ArchiveAnnouncement(c.Request.Context(), announcementID, userID)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to archive announcement")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_ARCHIVED",
			map[string]any{
				"announcement_id": item.ID.String(),
				"title":           item.Title,
			},
		)
	}

	c.ok(http.StatusOK, toAnnouncementResponse(item))
}

func (h *httpHandler) RestoreAnnouncement(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	item, err := h.announcementService.RestoreAnnouncement(c.Request.Context(), announcementID, userID)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to restore announcement")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_RESTORED",
			map[string]any{
				"announcement_id": item.ID.String(),
				"title":           item.Title,
			},
		)
	}

	c.ok(http.StatusOK, toAnnouncementResponse(item))
}

func (h *httpHandler) DeleteAnnouncement(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	if err := h.announcementService.DeleteAnnouncement(c.Request.Context(), announcementID, userID); err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete announcement")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_DELETED",
			map[string]any{
				"announcement_id": announcementID.String(),
			},
		)
	}

	c.ok(http.StatusOK, MessageResponse{Message: "announcement deleted successfully"})
}

func (h *httpHandler) UploadAnnouncementAttachment(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	var input UploadAnnouncementAttachmentInput
	includeInEmail := true

	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			c.fail(http.StatusBadRequest, "BAD_REQUEST", "file is required")
			return
		}

		reader, err := file.Open()
		if err != nil {
			c.fail(http.StatusBadRequest, "BAD_REQUEST", "failed to open uploaded file")
			return
		}
		defer reader.Close()

		if raw := strings.TrimSpace(c.PostForm("include_in_email")); raw != "" {
			if parsed, err := strconv.ParseBool(raw); err == nil {
				includeInEmail = parsed
			}
		}

		sortOrder := int32(0)
		if raw := strings.TrimSpace(c.PostForm("sort_order")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				sortOrder = int32(parsed)
			}
		}

		input = UploadAnnouncementAttachmentInput{
			FileName:       file.Filename,
			ContentType:    file.Header.Get("Content-Type"),
			Reader:         reader,
			Size:           file.Size,
			IncludeInEmail: includeInEmail,
			Inline:         strings.EqualFold(c.PostForm("inline"), "true"),
			ContentID:      c.PostForm("content_id"),
			SortOrder:      sortOrder,
			UploadedBy:     userID,
		}
	} else {
		var req createAnnouncementAttachmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
			return
		}
		if req.IncludeInEmail != nil {
			includeInEmail = *req.IncludeInEmail
		}
		input = UploadAnnouncementAttachmentInput{
			FileName:       req.FileName,
			ContentType:    req.ContentType,
			DataBase64:     req.DataBase64,
			IncludeInEmail: includeInEmail,
			Inline:         req.Inline,
			ContentID:      req.ContentID,
			SortOrder:      req.SortOrder,
			UploadedBy:     userID,
		}
	}

	attachment, err := h.announcementService.UploadAttachment(c.Request.Context(), announcementID, input)
	if err != nil {
		c.fail(http.StatusBadRequest, "INVALID_INPUT", "request failed")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_ATTACHMENT_UPLOADED",
			map[string]any{
				"announcement_id": announcementID.String(),
				"attachment_id":   attachment.ID.String(),
				"file_name":       attachment.OriginalFileName,
			},
		)
	}

	c.ok(http.StatusCreated, toAnnouncementAttachmentResponse(announcementID, attachment))
}

func (h *httpHandler) ListAnnouncementAttachments(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	attachments, err := h.announcementService.ListAttachments(c.Request.Context(), announcementID)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list announcement attachments")
		return
	}

	c.ok(http.StatusOK, toAnnouncementAttachmentResponses(announcementID, attachments))
}

func (h *httpHandler) DownloadAnnouncementAttachment(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	attachmentID, ok := getAnnouncementAttachmentID(c)
	if !ok {
		return
	}

	if !c.admin {
		item, err := h.announcementService.GetAnnouncementByID(c.Request.Context(), announcementID)
		if err != nil {
			c.fail(404, "NOT_FOUND", "announcement attachment not found")
			return
		}
		allowed, err := h.visible(c.Request.Context(), c.actor, item)
		if err != nil {
			c.reject(err)
			return
		}
		if !allowed {
			c.fail(404, "NOT_FOUND", "announcement attachment not found")
			return
		}
	}
	download, err := h.announcementService.OpenAttachmentDownload(c.Request.Context(), announcementID, attachmentID)
	if err != nil {
		c.fail(http.StatusNotFound, "NOT_FOUND", "announcement attachment not found")
		return
	}
	defer download.Reader.Close()

	contentType := nullStringValue(download.Attachment.ContentType)
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(download.Attachment.OriginalFileName)}))
	c.Header("Content-Type", contentType)
	if _, err := io.Copy(c.Writer, download.Reader); err != nil && c.streamError != nil {
		c.streamError(c.Request, err)
	}
}

func (h *httpHandler) UpdateAnnouncementAttachment(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}
	attachmentID, ok := getAnnouncementAttachmentID(c)
	if !ok {
		return
	}

	var req updateAnnouncementAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	attachment, err := h.announcementService.UpdateAttachment(
		c.Request.Context(),
		announcementID,
		attachmentID,
		UpdateAnnouncementAttachmentInput{
			IncludeInEmail: req.IncludeInEmail,
			Inline:         req.Inline,
			ContentID:      req.ContentID,
			SortOrder:      req.SortOrder,
			UpdatedBy:      userID,
		},
	)
	if err != nil {
		c.fail(http.StatusBadRequest, "INVALID_INPUT", "request failed")
		return
	}

	c.ok(http.StatusOK, toAnnouncementAttachmentResponse(announcementID, attachment))
}

func (h *httpHandler) DeleteAnnouncementAttachment(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}
	attachmentID, ok := getAnnouncementAttachmentID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	attachment, err := h.announcementService.DeleteAttachment(c.Request.Context(), announcementID, attachmentID, userID)
	if err != nil {
		c.fail(http.StatusBadRequest, "INVALID_INPUT", "request failed")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_ATTACHMENT_DELETED",
			map[string]any{
				"announcement_id": announcementID.String(),
				"attachment_id":   attachment.ID.String(),
				"file_name":       attachment.OriginalFileName,
			},
		)
	}

	c.ok(http.StatusOK, MessageResponse{Message: "announcement attachment deleted successfully"})
}

func (h *httpHandler) SetAnnouncementPinned(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	var req setPinnedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	item, err := h.announcementService.SetAnnouncementPinned(
		c.Request.Context(),
		announcementID,
		req.IsPinned,
		userID,
	)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update pinned state")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_PIN_UPDATED",
			map[string]any{
				"announcement_id": item.ID.String(),
				"is_pinned":       item.IsPinned,
			},
		)
	}

	c.ok(http.StatusOK, toAnnouncementResponse(item))
}

func (h *httpHandler) SetAnnouncementPriority(c *httpRequest) {
	announcementID, ok := getAnnouncementID(c)
	if !ok {
		return
	}

	var req setPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.fail(http.StatusBadRequest, "BAD_REQUEST", "request failed")
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	item, err := h.announcementService.SetAnnouncementPriority(
		c.Request.Context(),
		announcementID,
		req.Priority,
		userID,
	)
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update priority")
		return
	}

	if h.auditService != nil {
		_ = h.auditService.Log(
			c.Request.Context(),
			uuid.NullUUID{UUID: userID, Valid: true},
			"ANNOUNCEMENT_PRIORITY_UPDATED",
			map[string]any{
				"announcement_id": item.ID.String(),
				"priority":        item.Priority,
			},
		)
	}

	c.ok(http.StatusOK, toAnnouncementResponse(item))
}

func (h *httpHandler) GetAnnouncementStats(c *httpRequest) {
	stats, err := h.announcementService.GetAnnouncementStats(c.Request.Context())
	if err != nil {
		c.fail(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch announcement stats")
		return
	}

	c.ok(http.StatusOK, toAnnouncementStatsResponse(stats))
}

func (h *httpHandler) ListActivePublishedAnnouncements(c *httpRequest) {
	h.ListPublicAnnouncements(c)
}
func (h *httpHandler) ListAnnouncementsForClient(c *httpRequest) { h.listVisible(c) }
func (h *httpHandler) ListAnnouncementsForRole(c *httpRequest)   { h.listVisible(c) }
func (h *httpHandler) ListAnnouncementsForUser(c *httpRequest)   { h.listVisible(c) }
func (h *httpHandler) ListMyAnnouncements(c *httpRequest)        { h.listVisible(c) }
