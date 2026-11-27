package announcements

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo             Repository
	users            UserDirectory
	groups           GroupDirectory
	healthContexts   HealthAudience
	notifications    NotificationPublisher
	storage          AttachmentStorage
	settings         *Settings
	emailAttachments EmailAttachmentNormalizer
}

type AnnouncementEmailOptions struct {
	Attachments               []Attachment
	AttachmentLinks           []AnnouncementEmailAttachmentLink
	IncludeAttachmentsInEmail bool
	ScheduledAt               *time.Time
}

type AnnouncementEmailAttachmentLink struct {
	FileName    string
	ContentType string
	FileSize    int64
	URL         string
}

// NewServiceWithSettings composes portable contracts and non-secret settings.
// Hosts supply email normalization policy through the attachment port.
func NewServiceWithSettings(repo Repository, ports ServicePorts, settings *Settings) *Service {
	return &Service{repo: repo, users: ports.Users, groups: ports.Groups,
		healthContexts: ports.HealthContexts, notifications: ports.Notifications,
		storage: ports.Storage, settings: settings, emailAttachments: ports.EmailAttachments}
}

func (s *Service) ValidateGroupAudience(ctx context.Context, groupIDs []uuid.UUID) error {
	if len(groupIDs) == 0 {
		return nil
	}

	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.groups == nil {
		return errors.New("rbac repository is required for group audience")
	}

	seen := make(map[uuid.UUID]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID == uuid.Nil {
			return errors.New("group id is required")
		}

		if _, exists := seen[groupID]; exists {
			continue
		}
		seen[groupID] = struct{}{}

		if err := s.groups.ValidateGroup(ctx, groupID.String()); err != nil {
			return fmt.Errorf("validate group audience [%s]: %w", groupID, err)
		}
	}

	return nil
}

func (s *Service) ResolveHealthContextRecipients(
	ctx context.Context,
	actorID string,
	contextIDs []uuid.UUID,
	includeDescendants bool,
) ([]AnnouncementEmailRecipient, error) {
	if s == nil || s.healthContexts == nil {
		return nil, errors.New("health context service is required for contextual audience resolution")
	}
	userIDs, err := s.healthContexts.ResolveAudienceUserIDs(
		ctx,
		actorID,
		contextIDs,
		includeDescendants,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve health context audience: %w", err)
	}
	if len(userIDs) == 0 {
		return nil, nil
	}

	wanted := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		wanted[strings.TrimSpace(userID)] = struct{}{}
	}
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list health context audience users: %w", err)
	}
	seen := make(map[string]AnnouncementEmailRecipient)
	for _, user := range users {
		if _, ok := wanted[strings.TrimSpace(user.ID)]; !ok {
			continue
		}
		recipient, ok := announcementRecipientFromUser(user)
		if !ok {
			continue
		}
		seen[strings.ToLower(recipient.Email)] = recipient
	}
	return announcementRecipientMapToSlice(seen), nil
}

func (s *Service) resolveStoredHealthContextRecipients(
	ctx context.Context,
	audience []AnnouncementHealthContextAudience,
) ([]AnnouncementEmailRecipient, error) {
	if s.healthContexts == nil || len(audience) == 0 {
		return nil, errors.New("health context audience is unavailable")
	}
	contextIDs := make([]uuid.UUID, 0, len(audience))
	includeDescendants := false
	for _, item := range audience {
		contextIDs = append(contextIDs, item.ContextNodeID)
		includeDescendants = includeDescendants || item.IncludeDescendants
	}
	userIDs, err := s.healthContexts.ListAudienceUserIDs(ctx, contextIDs, includeDescendants)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		wanted[strings.TrimSpace(userID)] = struct{}{}
	}
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]AnnouncementEmailRecipient)
	for _, user := range users {
		if _, ok := wanted[strings.TrimSpace(user.ID)]; !ok {
			continue
		}
		recipient, ok := announcementRecipientFromUser(user)
		if ok {
			seen[strings.ToLower(recipient.Email)] = recipient
		}
	}
	return announcementRecipientMapToSlice(seen), nil
}

func (s *Service) ReplaceHealthContextAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	contextIDs []uuid.UUID,
	includeDescendants bool,
) error {
	return s.repo.ReplaceHealthContextAudience(
		ctx,
		announcementID,
		contextIDs,
		includeDescendants,
	)
}

func (s *Service) ListHealthContextAudience(
	ctx context.Context,
	announcementID uuid.UUID,
) ([]AnnouncementHealthContextAudience, error) {
	return s.repo.ListHealthContextAudience(ctx, announcementID)
}

// ---------------------------------
// Core CRUD
// ---------------------------------

func (s *Service) CreateAnnouncement(
	ctx context.Context,
	params CreateAnnouncementParams,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	item, err := s.repo.Create(ctx, params)
	if err != nil {
		return Announcement{}, fmt.Errorf("create announcement: %w", err)
	}

	nt := AnnouncementCreated
	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q created", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
			"created_by":      item.CreatedBy.String(),
			"notify_by_email": item.NotifyByEmail,
		}),
	})

	return item, nil
}

func (s *Service) CreateAnnouncementFromInput(
	ctx context.Context,
	input CreateAnnouncementInput,
) (Announcement, error) {
	if err := s.validateAnnouncementSMSInput(input.NotifyBySMS, input.SMSMessage); err != nil {
		return Announcement{}, err
	}

	item, err := s.CreateAnnouncement(ctx, CreateAnnouncementParams{
		Title:         strings.TrimSpace(input.Title),
		Message:       strings.TrimSpace(input.Message),
		Summary:       input.Summary,
		Level:         announcementLevelValue(input.Level),
		Tag:           input.Tag,
		LinkUrl:       input.LinkURL,
		LinkLabel:     input.LinkLabel,
		Priority:      input.Priority,
		IsPinned:      input.IsPinned,
		Status:        announcementStatusValue(input.Status),
		PublishAt:     input.PublishAt,
		ExpiresAt:     input.ExpiresAt,
		AudienceType:  announcementAudienceTypeValue(input.AudienceType),
		NotifyByEmail: input.NotifyByEmail,
		NotifyBySms:   input.NotifyBySMS,
		SmsMessage:    normalizeSMSMessage(input.SMSMessage),
		CreatedBy:     input.CreatedBy,
	})
	if err != nil {
		return Announcement{}, err
	}

	return item, nil
}

func (s *Service) GetAnnouncementByID(
	ctx context.Context,
	id uuid.UUID,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return Announcement{}, errors.New("announcement id is required")
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Announcement{}, fmt.Errorf("get announcement by id: %w", err)
	}

	return item, nil
}

func (s *Service) UpdateAnnouncement(
	ctx context.Context,
	params UpdateAnnouncementParams,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	item, err := s.repo.Update(ctx, params)
	if err != nil {
		return Announcement{}, fmt.Errorf("update announcement: %w", err)
	}

	nt := AnnouncementUpdated
	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q updated", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
			"updated_by":      item.UpdatedBy,
			"notify_by_email": item.NotifyByEmail,
		}),
	})

	return item, nil
}

func (s *Service) UpdateAnnouncementFromInput(
	ctx context.Context,
	input UpdateAnnouncementInput,
) (Announcement, error) {
	if err := s.validateAnnouncementSMSInput(input.NotifyBySMS, input.SMSMessage); err != nil {
		return Announcement{}, err
	}

	item, err := s.UpdateAnnouncement(ctx, UpdateAnnouncementParams{
		ID:            input.ID,
		Title:         strings.TrimSpace(input.Title),
		Message:       strings.TrimSpace(input.Message),
		Summary:       input.Summary,
		Level:         announcementLevelValue(input.Level),
		Tag:           input.Tag,
		LinkUrl:       input.LinkURL,
		LinkLabel:     input.LinkLabel,
		Priority:      input.Priority,
		IsPinned:      input.IsPinned,
		PublishAt:     input.PublishAt,
		ExpiresAt:     input.ExpiresAt,
		AudienceType:  announcementAudienceTypeValue(input.AudienceType),
		NotifyByEmail: input.NotifyByEmail,
		NotifyBySms:   input.NotifyBySMS,
		SmsMessage:    normalizeSMSMessage(input.SMSMessage),
		UpdatedBy: uuid.NullUUID{
			UUID:  input.UpdatedBy,
			Valid: input.UpdatedBy != uuid.Nil,
		},
	})
	if err != nil {
		return Announcement{}, err
	}

	return item, nil
}

func (s *Service) DeleteAnnouncement(
	ctx context.Context,
	id uuid.UUID,
	deletedBy uuid.UUID,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("announcement id is required")
	}

	item, _ := s.repo.GetByID(ctx, id)

	if err := s.repo.SoftDelete(ctx, id, deletedBy); err != nil {
		return fmt.Errorf("delete announcement: %w", err)
	}

	nt := AnnouncementDeleted
	notification := Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "Announcement deleted",
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": id.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
			"deleted_by":      deletedBy.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"admin-alert",
		"Announcement deleted",
		fmt.Sprintf("Announcement %q was deleted.", item.Title),
		map[string]any{
			"Name":           s.systemAdminName(),
			"Platform":       s.platformName(),
			"Message":        "An announcement was deleted.",
			"AnnouncementID": id.String(),
			"Title":          item.Title,
			"Status":         announcementStatusString(item.Status),
			"ActionURL":      s.adminAnnouncementsURL(),
			"Details": fmt.Sprintf(
				"Announcement ID: %s\nTitle: %s\nStatus: %s\nDeleted By: %s",
				id.String(),
				item.Title,
				announcementStatusString(item.Status),
				deletedBy.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

func (s *Service) RestoreAnnouncement(
	ctx context.Context,
	id uuid.UUID,
	updatedBy uuid.UUID,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return Announcement{}, errors.New("announcement id is required")
	}

	item, err := s.repo.Restore(ctx, id, updatedBy)
	if err != nil {
		return Announcement{}, fmt.Errorf("restore announcement: %w", err)
	}

	nt := AnnouncementUpdated
	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q restored", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
			"updated_by":      updatedBy.String(),
		}),
	})

	return item, nil
}

// ---------------------------------
// Admin listing / search / stats
// ---------------------------------

func (s *Service) ListAnnouncementsAdmin(
	ctx context.Context,
	params ListAnnouncementsAdminParams,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	items, err := s.repo.ListAdmin(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list announcements admin: %w", err)
	}

	return items, nil
}

func (s *Service) ListAnnouncementsAdminPage(
	ctx context.Context,
	limit int32,
	offset int32,
) ([]Announcement, error) {
	return s.ListAnnouncementsAdmin(ctx, ListAnnouncementsAdminParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Service) CountAnnouncementsAdmin(
	ctx context.Context,
) (int64, error) {
	if s == nil {
		return 0, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return 0, errors.New("announcement repository is nil")
	}

	count, err := s.repo.CountAdmin(ctx)
	if err != nil {
		return 0, fmt.Errorf("count announcements admin: %w", err)
	}

	return count, nil
}

func (s *Service) ListAnnouncementsByStatus(
	ctx context.Context,
	params ListAnnouncementsByStatusParams,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	items, err := s.repo.ListByStatus(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list announcements by status: %w", err)
	}

	return items, nil
}

func (s *Service) CountAnnouncementsByStatus(
	ctx context.Context,
	status AnnouncementStatus,
) (int64, error) {
	if s == nil {
		return 0, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return 0, errors.New("announcement repository is nil")
	}

	count, err := s.repo.CountByStatus(ctx, status)
	if err != nil {
		return 0, fmt.Errorf("count announcements by status: %w", err)
	}

	return count, nil
}

func (s *Service) SearchAnnouncementsAdmin(
	ctx context.Context,
	params SearchAnnouncementsAdminParams,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	items, err := s.repo.SearchAdmin(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("search announcements admin: %w", err)
	}

	return items, nil
}

func (s *Service) CountSearchAnnouncementsAdmin(
	ctx context.Context,
	search string,
) (int64, error) {
	if s == nil {
		return 0, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return 0, errors.New("announcement repository is nil")
	}

	count, err := s.repo.CountSearchAdmin(ctx, strings.TrimSpace(search))
	if err != nil {
		return 0, fmt.Errorf("count search announcements admin: %w", err)
	}

	return count, nil
}

func (s *Service) GetAnnouncementStats(
	ctx context.Context,
) (GetAnnouncementStatsRow, error) {
	if s == nil {
		return GetAnnouncementStatsRow{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return GetAnnouncementStatsRow{}, errors.New("announcement repository is nil")
	}

	stats, err := s.repo.GetStats(ctx)
	if err != nil {
		return GetAnnouncementStatsRow{}, fmt.Errorf("get announcement stats: %w", err)
	}

	return stats, nil
}

// ---------------------------------
// Public / end-user listing
// ---------------------------------

func (s *Service) ListActivePublishedAnnouncements(
	ctx context.Context,
	params ListActivePublishedAnnouncementsParams,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	items, err := s.repo.ListActivePublished(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list active published announcements: %w", err)
	}

	return items, nil
}

func (s *Service) ListActivePublishedAnnouncementsPage(
	ctx context.Context,
	limit int32,
	offset int32,
) ([]Announcement, error) {
	return s.ListActivePublishedAnnouncements(ctx, ListActivePublishedAnnouncementsParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Service) CountActivePublishedAnnouncements(
	ctx context.Context,
) (int64, error) {
	if s == nil {
		return 0, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return 0, errors.New("announcement repository is nil")
	}

	count, err := s.repo.CountActivePublished(ctx)
	if err != nil {
		return 0, fmt.Errorf("count active published announcements: %w", err)
	}

	return count, nil
}

func (s *Service) ListAnnouncementsForClient(
	ctx context.Context,
	params ListAnnouncementsForClientParams,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	items, err := s.repo.ListForClient(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list announcements for client: %w", err)
	}

	return items, nil
}

func (s *Service) ListAnnouncementsForClientPage(
	ctx context.Context,
	clientID uuid.UUID,
	limit int32,
	offset int32,
) ([]Announcement, error) {
	return s.ListAnnouncementsForClient(ctx, ListAnnouncementsForClientParams{
		ClientID: clientID,
		Limit:    limit,
		Offset:   offset,
	})
}

func (s *Service) ListAnnouncementsForRole(
	ctx context.Context,
	params ListAnnouncementsForRoleParams,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	items, err := s.repo.ListForRole(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list announcements for role: %w", err)
	}

	return items, nil
}

func (s *Service) ListAnnouncementsForRolePage(
	ctx context.Context,
	roleName string,
	limit int32,
	offset int32,
) ([]Announcement, error) {
	return s.ListAnnouncementsForRole(ctx, ListAnnouncementsForRoleParams{
		RoleName:   strings.TrimSpace(roleName),
		PageLimit:  limit,
		PageOffset: offset,
	})
}

func (s *Service) ListAnnouncementsForUser(
	ctx context.Context,
	params ListAnnouncementsForUserParams,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	items, err := s.repo.ListForUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list announcements for user: %w", err)
	}

	return items, nil
}

func (s *Service) ListAnnouncementsForUserPage(
	ctx context.Context,
	userID uuid.UUID,
	limit int32,
	offset int32,
) ([]Announcement, error) {
	return s.ListAnnouncementsForUser(ctx, ListAnnouncementsForUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Service) ListMyAnnouncements(
	ctx context.Context,
	userID uuid.UUID,
	roleName string,
	clientID uuid.UUID,
	limit int32,
	offset int32,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	limit = normalizeLimit(limit)
	offset = normalizeOffset(offset)
	roleName = strings.TrimSpace(roleName)

	results := make(map[uuid.UUID]Announcement)

	activeItems, err := s.repo.ListActivePublished(ctx, ListActivePublishedAnnouncementsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list active announcements: %w", err)
	}

	for _, item := range activeItems {
		results[item.ID] = item
	}

	if roleName != "" {
		roleItems, err := s.repo.ListForRole(ctx, ListAnnouncementsForRoleParams{
			RoleName:   roleName,
			PageLimit:  limit,
			PageOffset: offset,
		})
		if err != nil {
			return nil, fmt.Errorf("list role announcements: %w", err)
		}

		for _, item := range roleItems {
			results[item.ID] = item
		}
	}

	if clientID != uuid.Nil {
		clientItems, err := s.repo.ListForClient(ctx, ListAnnouncementsForClientParams{
			ClientID: clientID,
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			return nil, fmt.Errorf("list client announcements: %w", err)
		}

		for _, item := range clientItems {
			results[item.ID] = item
		}
	}

	if userID != uuid.Nil {
		userItems, err := s.repo.ListForUser(ctx, ListAnnouncementsForUserParams{
			UserID: userID,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, fmt.Errorf("list user announcements: %w", err)
		}

		for _, item := range userItems {
			results[item.ID] = item
		}
	}

	items := make([]Announcement, 0, len(results))
	for _, item := range results {
		items = append(items, item)
	}

	return items, nil
}

func (s *Service) ListPublicAnnouncements(
	ctx context.Context,
	limit int32,
	offset int32,
) ([]Announcement, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	limit = normalizeLimit(limit)
	offset = normalizeOffset(offset)

	return s.repo.ListPublicAnnouncements(ctx, limit, offset)
}

func normalizeLimit(limit int32) int32 {
	switch {
	case limit <= 0:
		return 20
	case limit > 100:
		return 100
	default:
		return limit
	}
}

func normalizeOffset(offset int32) int32 {
	if offset < 0 {
		return 0
	}

	return offset
}

// ---------------------------------
// Lifecycle actions
// ---------------------------------

// ErrInvalidEmailAttachments wraps request email attachments that the host's
// attachment policy rejected. Nothing is changed when it is returned.
var ErrInvalidEmailAttachments = errors.New("invalid announcement email attachments")

func (s *Service) PublishAnnouncementNow(
	ctx context.Context,
	id uuid.UUID,
	publishedBy uuid.UUID,
	options ...AnnouncementEmailOptions,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return Announcement{}, errors.New("announcement id is required")
	}

	// Validate request attachments before changing state, so a bad attachment
	// fails the request without leaving a half-notified published announcement.
	emailOptions, err := s.normalizeAnnouncementEmailOptions(options...)
	if err != nil {
		return Announcement{}, fmt.Errorf("%w: %w", ErrInvalidEmailAttachments, err)
	}

	item, err := s.repo.PublishNow(ctx, id, publishedBy)
	if err != nil {
		return Announcement{}, fmt.Errorf("publish announcement: %w", err)
	}

	nt := AnnouncementPublished

	notification := Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q published", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id":                    item.ID.String(),
			"title":                              item.Title,
			"status":                             announcementStatusString(item.Status),
			"published_by":                       publishedBy.String(),
			"notify_by_email":                    item.NotifyByEmail,
			"email_notification_sent_at_present": item.EmailNotificationSentAt.Valid,
		}),
	}

	smsQueued, err := s.attachAnnouncementSMSDelivery(ctx, &notification, item, nil)
	if err != nil {
		return item, err
	}

	emailQueued := false
	if s.shouldSendAnnouncementEmail(item) {
		recipients, err := s.resolveAnnouncementEmailRecipients(ctx, item)
		if err != nil {
			return item, fmt.Errorf("resolve announcement email recipients: %w", err)
		}

		if len(recipients) > 0 {
			if err := s.repo.SaveAudienceSnapshot(
				ctx,
				item.ID,
				"EMAIL",
				announcementRecipientUserIDs(recipients),
			); err != nil {
				return item, fmt.Errorf("save announcement email audience snapshot: %w", err)
			}
			withStored, err := s.withPersistedAnnouncementEmailAttachments(ctx, item.ID, emailOptions)
			if err != nil {
				return item, err
			}
			s.attachAnnouncementEmailDelivery(&notification, item, recipients, withStored)
			emailQueued = true
		}
	}

	return s.publishAnnouncementDeliveries(ctx, notification, item, emailQueued, smsQueued)
}

func (s *Service) MoveAnnouncementToDraft(
	ctx context.Context,
	params DraftAnnouncementParams,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	item, err := s.repo.Draft(ctx, params)
	if err != nil {
		return Announcement{}, fmt.Errorf("drafted announcement: %w", err)
	}

	nt := AnnouncementDrafted
	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q drafted", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
		}),
	})

	return item, nil
}

func (s *Service) MoveAnnouncementToDraftByUser(
	ctx context.Context,
	id uuid.UUID,
	updatedBy uuid.UUID,
) (Announcement, error) {
	return s.MoveAnnouncementToDraft(ctx, DraftAnnouncementParams{
		ID: id,
		UpdatedBy: uuid.NullUUID{
			UUID:  updatedBy,
			Valid: updatedBy != uuid.Nil,
		},
	})
}

func (s *Service) ScheduleAnnouncement(
	ctx context.Context,
	params ScheduleAnnouncementParams,
	options ...AnnouncementEmailOptions,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	emailOptions, err := s.normalizeAnnouncementEmailOptions(options...)
	if err != nil {
		return Announcement{}, fmt.Errorf("%w: %w", ErrInvalidEmailAttachments, err)
	}

	item, err := s.repo.Schedule(ctx, params)
	if err != nil {
		return Announcement{}, fmt.Errorf("schedule announcement: %w", err)
	}

	nt := AnnouncementScheduled
	notification := Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q scheduled", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
		}),
	}

	var scheduledAt *time.Time
	if item.PublishAt.Valid {
		publishAt := item.PublishAt.Time
		scheduledAt = &publishAt
	}
	smsQueued, err := s.attachAnnouncementSMSDelivery(ctx, &notification, item, scheduledAt)
	if err != nil {
		return item, err
	}

	emailQueued := false
	if s.shouldSendAnnouncementEmail(item) {
		recipients, err := s.resolveAnnouncementEmailRecipients(ctx, item)
		if err != nil {
			return item, fmt.Errorf("resolve announcement email recipients: %w", err)
		}

		if len(recipients) > 0 {
			if err := s.repo.SaveAudienceSnapshot(
				ctx,
				item.ID,
				"EMAIL",
				announcementRecipientUserIDs(recipients),
			); err != nil {
				return item, fmt.Errorf("save scheduled announcement email audience snapshot: %w", err)
			}
			withStored, err := s.withPersistedAnnouncementEmailAttachments(ctx, item.ID, emailOptions)
			if err != nil {
				return item, err
			}
			if withStored.ScheduledAt == nil && item.PublishAt.Valid {
				publishAt := item.PublishAt.Time
				withStored.ScheduledAt = &publishAt
			}
			s.attachAnnouncementEmailDelivery(&notification, item, recipients, withStored)
			emailQueued = true
		}
	}

	return s.publishAnnouncementDeliveries(ctx, notification, item, emailQueued, smsQueued)
}

func (s *Service) ScheduleAnnouncementByUser(
	ctx context.Context,
	id uuid.UUID,
	publishAt time.Time,
	updatedBy uuid.UUID,
	options ...AnnouncementEmailOptions,
) (Announcement, error) {
	return s.ScheduleAnnouncement(
		ctx,
		ScheduleAnnouncementParams{
			ID: id,
			PublishAt: sql.NullTime{
				Time:  publishAt,
				Valid: true,
			},
			UpdatedBy: uuid.NullUUID{
				UUID:  updatedBy,
				Valid: updatedBy != uuid.Nil,
			},
		},
		options...,
	)
}

func (s *Service) ArchiveAnnouncement(
	ctx context.Context,
	id uuid.UUID,
	archivedBy uuid.UUID,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return Announcement{}, errors.New("announcement id is required")
	}

	item, err := s.repo.Archive(ctx, id, archivedBy)
	if err != nil {
		return Announcement{}, fmt.Errorf("archive announcement: %w", err)
	}

	nt := AnnouncementArchived
	notification := Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q archived", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
			"archived_by":     archivedBy.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"admin-alert",
		"Announcement archived",
		fmt.Sprintf("Announcement %q was archived.", item.Title),
		map[string]any{
			"Name":           s.systemAdminName(),
			"Platform":       s.platformName(),
			"Message":        fmt.Sprintf("Announcement %q was archived.", item.Title),
			"AnnouncementID": item.ID.String(),
			"Title":          item.Title,
			"Status":         announcementStatusString(item.Status),
			"ActionURL":      s.adminAnnouncementsURL(),
			"Details": fmt.Sprintf(
				"Announcement ID: %s\nTitle: %s\nStatus: %s\nArchived By: %s",
				item.ID.String(),
				item.Title,
				announcementStatusString(item.Status),
				archivedBy.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return item, nil
}

func (s *Service) UnarchiveAnnouncementToDraft(
	ctx context.Context,
	id uuid.UUID,
	updatedBy uuid.UUID,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return Announcement{}, errors.New("announcement id is required")
	}

	item, err := s.repo.UnarchiveToDraft(ctx, id, updatedBy)
	if err != nil {
		return Announcement{}, fmt.Errorf("unarchive announcement to draft: %w", err)
	}

	nt := AnnouncementUpdated
	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q moved back to draft", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
			"updated_by":      updatedBy.String(),
		}),
	})

	return item, nil
}

func (s *Service) UpdateAnnouncementStatus(
	ctx context.Context,
	params UpdateAnnouncementStatusParams,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	item, err := s.repo.UpdateStatus(ctx, params)
	if err != nil {
		return Announcement{}, fmt.Errorf("update announcement status: %w", err)
	}

	nt := AnnouncementUpdated
	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q status updated", item.Title),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
		}),
	})

	return item, nil
}

func (s *Service) SetAnnouncementPinned(
	ctx context.Context,
	id uuid.UUID,
	isPinned bool,
	updatedBy uuid.UUID,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return Announcement{}, errors.New("announcement id is required")
	}

	item, err := s.repo.SetPinned(ctx, id, isPinned, updatedBy)
	if err != nil {
		return Announcement{}, fmt.Errorf("set announcement pinned: %w", err)
	}

	nt := AnnouncementUpdated
	msg := fmt.Sprintf("Announcement %q unpinned", item.Title)
	if isPinned {
		msg = fmt.Sprintf("Announcement %q pinned", item.Title)
	}

	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    msg,
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"status":          announcementStatusString(item.Status),
			"is_pinned":       isPinned,
			"updated_by":      updatedBy.String(),
		}),
	})

	return item, nil
}

func (s *Service) SetAnnouncementPriority(
	ctx context.Context,
	id uuid.UUID,
	priority int32,
	updatedBy uuid.UUID,
) (Announcement, error) {
	if s == nil {
		return Announcement{}, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return Announcement{}, errors.New("announcement repository is nil")
	}

	if id == uuid.Nil {
		return Announcement{}, errors.New("announcement id is required")
	}

	item, err := s.repo.SetPriority(ctx, id, priority, updatedBy)
	if err != nil {
		return Announcement{}, fmt.Errorf("set announcement priority: %w", err)
	}

	nt := AnnouncementUpdated
	s.notify(ctx, Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    fmt.Sprintf("Announcement %q priority updated to %d", item.Title, priority),
		TargetRole: "admin",
		Metadata: announcementJSON(map[string]any{
			"announcement_id": item.ID.String(),
			"title":           item.Title,
			"priority":        priority,
			"updated_by":      updatedBy.String(),
		}),
	})

	return item, nil
}

// ---------------------------------
// Audience management
// ---------------------------------

func (s *Service) AddClientAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	clientID uuid.UUID,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	if clientID == uuid.Nil {
		return errors.New("client id is required")
	}

	if err := s.repo.AddClientAudience(ctx, announcementID, clientID); err != nil {
		return fmt.Errorf("add client audience: %w", err)
	}

	return nil
}

func (s *Service) ReplaceClientAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	clientIDs []uuid.UUID,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	if err := s.repo.ReplaceClientAudience(ctx, announcementID, clientIDs); err != nil {
		return fmt.Errorf("replace client audience: %w", err)
	}

	return nil
}

func (s *Service) ListClientAudience(
	ctx context.Context,
	announcementID uuid.UUID,
) ([]uuid.UUID, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return nil, errors.New("announcement id is required")
	}

	items, err := s.repo.ListClientAudience(ctx, announcementID)
	if err != nil {
		return nil, fmt.Errorf("list client audience: %w", err)
	}

	return items, nil
}

func (s *Service) AddRoleAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	roleName string,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return errors.New("role name is required")
	}

	if err := s.repo.AddRoleAudience(ctx, announcementID, roleName); err != nil {
		return fmt.Errorf("add role audience: %w", err)
	}

	return nil
}

func (s *Service) ReplaceRoleAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	roleNames []string,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	cleaned := make([]string, 0, len(roleNames))
	for _, role := range roleNames {
		role = strings.TrimSpace(role)
		if role != "" {
			cleaned = append(cleaned, role)
		}
	}

	if err := s.repo.ReplaceRoleAudience(ctx, announcementID, cleaned); err != nil {
		return fmt.Errorf("replace role audience: %w", err)
	}

	return nil
}

func (s *Service) ListRoleAudience(
	ctx context.Context,
	announcementID uuid.UUID,
) ([]string, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return nil, errors.New("announcement id is required")
	}

	items, err := s.repo.ListRoleAudience(ctx, announcementID)
	if err != nil {
		return nil, fmt.Errorf("list role audience: %w", err)
	}

	return items, nil
}

func (s *Service) AddUserAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	userID uuid.UUID,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	if err := s.repo.AddUserAudience(ctx, announcementID, userID); err != nil {
		return fmt.Errorf("add user audience: %w", err)
	}

	return nil
}

func (s *Service) ReplaceUserAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	userIDs []uuid.UUID,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	if err := s.repo.ReplaceUserAudience(ctx, announcementID, userIDs); err != nil {
		return fmt.Errorf("replace user audience: %w", err)
	}

	return nil
}

func (s *Service) ListUserAudience(
	ctx context.Context,
	announcementID uuid.UUID,
) ([]uuid.UUID, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return nil, errors.New("announcement id is required")
	}

	items, err := s.repo.ListUserAudience(ctx, announcementID)
	if err != nil {
		return nil, fmt.Errorf("list user audience: %w", err)
	}

	return items, nil
}

func (s *Service) AddGroupAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	groupID uuid.UUID,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	if groupID == uuid.Nil {
		return errors.New("group id is required")
	}

	if err := s.repo.AddGroupAudience(ctx, announcementID, groupID); err != nil {
		return fmt.Errorf("add group audience: %w", err)
	}

	return nil
}

func (s *Service) ReplaceGroupAudience(
	ctx context.Context,
	announcementID uuid.UUID,
	groupIDs []uuid.UUID,
) error {
	if s == nil {
		return errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return errors.New("announcement id is required")
	}

	if err := s.repo.ReplaceGroupAudience(ctx, announcementID, groupIDs); err != nil {
		return fmt.Errorf("replace group audience: %w", err)
	}

	return nil
}

func (s *Service) ListGroupAudience(
	ctx context.Context,
	announcementID uuid.UUID,
) ([]uuid.UUID, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	if announcementID == uuid.Nil {
		return nil, errors.New("announcement id is required")
	}

	items, err := s.repo.ListGroupAudience(ctx, announcementID)
	if err != nil {
		return nil, fmt.Errorf("list group audience: %w", err)
	}

	return items, nil
}

// ---------------------------------
// Announcement email recipient resolution
// ---------------------------------

func (s *Service) resolveAnnouncementEmailRecipients(
	ctx context.Context,
	item Announcement,
) ([]AnnouncementEmailRecipient, error) {
	if s == nil {
		return nil, errors.New("announcement service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("announcement repository is nil")
	}

	if s.users == nil {
		return nil, errors.New("user repository is nil")
	}

	audienceType := announcementAudienceTypeString(item.AudienceType)

	switch audienceType {
	case "ALL_USERS":
		return s.resolveAllUsersEmailRecipients(ctx)

	case "ADMINS_ONLY":
		return s.resolveUsersByRealmRoles(ctx, []string{"admin"})

	case "SPECIFIC_ROLES":
		roleNames, err := s.repo.ListRoleAudience(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list announcement role audience: %w", err)
		}

		return s.resolveUsersByRealmRoles(ctx, roleNames)

	case "SPECIFIC_USERS":
		userIDs, err := s.repo.ListUserAudience(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list announcement user audience: %w", err)
		}

		return s.resolveSpecificUsersEmailRecipients(ctx, userIDs)

	case "SPECIFIC_CLIENTS":
		clientIDs, err := s.repo.ListClientAudience(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list announcement client audience: %w", err)
		}

		return s.resolveUsersByClientAccess(ctx, clientIDs)

	case "SPECIFIC_GROUPS":
		groupIDs, err := s.repo.ListGroupAudience(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list announcement group audience: %w", err)
		}

		return s.resolveUsersByRBACGroups(ctx, groupIDs)

	case "SPECIFIC_HEALTH_CONTEXTS", "HEALTH_CONTEXT_AND_DESCENDANTS":
		audience, err := s.repo.ListHealthContextAudience(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("list announcement health context audience: %w", err)
		}
		return s.resolveStoredHealthContextRecipients(ctx, audience)

	default:
		return nil, fmt.Errorf("unsupported announcement audience type: %s", audienceType)
	}
}

func (s *Service) resolveAllUsersEmailRecipients(
	ctx context.Context,
) ([]AnnouncementEmailRecipient, error) {
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return uniqueAnnouncementRecipientsFromUsers(users), nil
}

func (s *Service) resolveSpecificUsersEmailRecipients(
	ctx context.Context,
	userIDs []uuid.UUID,
) ([]AnnouncementEmailRecipient, error) {
	seen := make(map[string]AnnouncementEmailRecipient)

	for _, userID := range userIDs {
		if userID == uuid.Nil {
			continue
		}

		user, err := s.users.GetUserByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get user by id %s: %w", userID.String(), err)
		}

		if user == nil {
			continue
		}

		recipient, ok := announcementRecipientFromUser(*user)
		if !ok {
			continue
		}

		seen[strings.ToLower(recipient.Email)] = recipient
	}

	return announcementRecipientMapToSlice(seen), nil
}

func (s *Service) resolveUsersByRealmRoles(
	ctx context.Context,
	roleNames []string,
) ([]AnnouncementEmailRecipient, error) {
	seen := make(map[string]AnnouncementEmailRecipient)

	for _, roleName := range roleNames {
		roleName = strings.TrimSpace(roleName)
		if roleName == "" {
			continue
		}

		users, err := s.users.GetUsersByRealmRole(ctx, roleName)
		if err != nil {
			return nil, fmt.Errorf("get users by realm role %q: %w", roleName, err)
		}

		for _, user := range users {
			recipient, ok := announcementRecipientFromUser(user)
			if !ok {
				continue
			}

			seen[strings.ToLower(recipient.Email)] = recipient
		}
	}

	return announcementRecipientMapToSlice(seen), nil
}

func (s *Service) resolveUsersByClientAccess(
	ctx context.Context,
	clientIDs []uuid.UUID,
) ([]AnnouncementEmailRecipient, error) {
	allUsers, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users for client access resolution: %w", err)
	}

	clientIDSet := make(map[string]struct{}, len(clientIDs))
	for _, clientID := range clientIDs {
		if clientID == uuid.Nil {
			continue
		}

		clientIDSet[strings.ToLower(clientID.String())] = struct{}{}
	}

	if len(clientIDSet) == 0 {
		return nil, nil
	}

	seen := make(map[string]AnnouncementEmailRecipient)

	for _, user := range allUsers {
		userID := strings.TrimSpace(user.ID)
		if userID == "" {
			continue
		}

		assignments, err := s.users.GetUserClientRoles(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get user client roles for user %s: %w", userID, err)
		}

		if !userHasAnyAnnouncementClientAccess(assignments, clientIDSet) {
			continue
		}

		recipient, ok := announcementRecipientFromUser(user)
		if !ok {
			continue
		}

		seen[strings.ToLower(recipient.Email)] = recipient
	}

	return announcementRecipientMapToSlice(seen), nil
}

func (s *Service) resolveUsersByRBACGroups(
	ctx context.Context,
	groupIDs []uuid.UUID,
) ([]AnnouncementEmailRecipient, error) {
	if s.groups == nil {
		return nil, errors.New("rbac repository is required for group audience resolution")
	}

	seen := make(map[string]AnnouncementEmailRecipient)

	for _, groupID := range groupIDs {
		if groupID == uuid.Nil {
			continue
		}

		members, err := s.groups.ListGroupMembers(ctx, groupID.String())
		if err != nil {
			return nil, fmt.Errorf("list members for group %s: %w", groupID.String(), err)
		}

		for _, member := range members {
			recipient, ok, err := s.announcementRecipientFromGroupMember(ctx, member)
			if err != nil {
				return nil, err
			}

			if !ok {
				continue
			}

			seen[strings.ToLower(recipient.Email)] = recipient
		}
	}

	return announcementRecipientMapToSlice(seen), nil
}

func (s *Service) announcementRecipientFromGroupMember(
	ctx context.Context,
	member AudienceGroupMember,
) (AnnouncementEmailRecipient, bool, error) {
	userID := uuid.Nil
	parsedUserID, err := uuid.Parse(strings.TrimSpace(member.UserID))
	if err == nil && parsedUserID != uuid.Nil {
		userID = parsedUserID
		user, err := s.users.GetUserByID(ctx, parsedUserID)
		if err != nil {
			return AnnouncementEmailRecipient{}, false, fmt.Errorf("get group member user %s: %w", parsedUserID, err)
		}

		if user != nil {
			recipient, ok := announcementRecipientFromUser(*user)
			return recipient, ok, nil
		}
	}

	email := strings.TrimSpace(member.Email)
	if email == "" {
		return AnnouncementEmailRecipient{}, false, nil
	}

	fullName := strings.TrimSpace(member.Username)
	if fullName == "" {
		fullName = email
	}

	return AnnouncementEmailRecipient{
		ID:       userID,
		Email:    email,
		Username: strings.TrimSpace(member.Username),
		FullName: fullName,
	}, true, nil
}

func announcementRecipientFromUser(user AudienceUser) (AnnouncementEmailRecipient, bool) {
	if !user.Enabled {
		return AnnouncementEmailRecipient{}, false
	}

	email := strings.TrimSpace(user.Email)
	if email == "" {
		return AnnouncementEmailRecipient{}, false
	}

	fullName := strings.TrimSpace(user.FullName)
	if fullName == "" {
		fullName = repositorySafeFullName(user.FirstName, user.LastName)
	}

	if fullName == "" {
		fullName = strings.TrimSpace(user.Username)
	}

	if fullName == "" {
		fullName = email
	}

	userID, err := uuid.Parse(strings.TrimSpace(user.ID))
	if err != nil {
		userID = uuid.Nil
	}

	return AnnouncementEmailRecipient{
		ID:       userID,
		Email:    email,
		Username: strings.TrimSpace(user.Username),
		FullName: fullName,
	}, true
}

func uniqueAnnouncementRecipientsFromUsers(
	users []AudienceUser,
) []AnnouncementEmailRecipient {
	seen := make(map[string]AnnouncementEmailRecipient)

	for _, user := range users {
		recipient, ok := announcementRecipientFromUser(user)
		if !ok {
			continue
		}

		seen[strings.ToLower(recipient.Email)] = recipient
	}

	return announcementRecipientMapToSlice(seen)
}

func announcementRecipientMapToSlice(
	items map[string]AnnouncementEmailRecipient,
) []AnnouncementEmailRecipient {
	out := make([]AnnouncementEmailRecipient, 0, len(items))

	for _, item := range items {
		out = append(out, item)
	}

	return out
}

func userHasAnyAnnouncementClientAccess(
	assignments []ClientAccess,
	clientIDSet map[string]struct{},
) bool {
	for _, assignment := range assignments {
		candidates := []string{
			assignment.ID,
			assignment.ClientID,
		}

		for _, candidate := range candidates {
			candidate = strings.ToLower(strings.TrimSpace(candidate))
			if candidate == "" {
				continue
			}

			if _, ok := clientIDSet[candidate]; ok {
				return true
			}
		}
	}

	return false
}

func repositorySafeFullName(firstName string, lastName string) string {
	return strings.Join(
		strings.Fields(
			strings.TrimSpace(firstName)+" "+strings.TrimSpace(lastName),
		),
		" ",
	)
}

func announcementRecipientUserIDs(recipients []AnnouncementEmailRecipient) []string {
	result := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.ID != uuid.Nil {
			result = append(result, recipient.ID.String())
		}
	}
	return result
}

func announcementSMSRecipientUserIDs(recipients []AnnouncementSMSRecipient) []string {
	result := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.ID != uuid.Nil {
			result = append(result, recipient.ID.String())
		}
	}
	return result
}

// ---------------------------------
// Notification helpers
// ---------------------------------

func (s *Service) notify(
	ctx context.Context,
	notification Notification,
) {
	if s == nil || s.notifications == nil {
		return
	}

	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}

	_ = s.notifications.Publish(ctx, notification)
}

func (s *Service) shouldSendAnnouncementEmail(item Announcement) bool {
	return item.NotifyByEmail && !item.EmailNotificationSentAt.Valid
}

func (s *Service) shouldSendAnnouncementSMS(item Announcement) bool {
	if !item.NotifyBySms || item.SmsNotificationQueuedAt.Valid {
		return false
	}

	if s == nil || s.settings == nil {
		return false
	}

	return s.settings.SMS.Enabled
}

func (s *Service) validateAnnouncementSMSInput(
	notifyBySMS bool,
	smsMessage sql.NullString,
) error {
	if !notifyBySMS {
		return nil
	}

	message := strings.TrimSpace(smsMessage.String)
	if message == "" {
		return nil
	}

	maxLength := 160
	if s != nil && s.settings != nil && s.settings.SMS.MaxLength > 0 {
		maxLength = s.settings.SMS.MaxLength
	}

	if len([]rune(message)) > maxLength {
		return fmt.Errorf("sms message exceeds max length of %d characters", maxLength)
	}

	return nil
}

func normalizeSMSMessage(value sql.NullString) sql.NullString {
	message := strings.TrimSpace(value.String)
	if message == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: message, Valid: true}
}

// attachAnnouncementSMSDelivery adds SMS deliveries to notification and
// reports whether any were added. It does not mark the announcement: callers
// mark it only after the notification has been handed to the publisher.
func (s *Service) attachAnnouncementSMSDelivery(
	ctx context.Context,
	notification *Notification,
	item Announcement,
	scheduledAt *time.Time,
) (bool, error) {
	if notification == nil {
		return false, nil
	}

	if !s.shouldSendAnnouncementSMS(item) {
		return false, nil
	}

	emailRecipients, err := s.resolveAnnouncementEmailRecipients(ctx, item)
	if err != nil {
		return false, fmt.Errorf("resolve announcement sms audience: %w", err)
	}

	recipients, err := s.resolveAnnouncementSMSRecipients(ctx, emailRecipients)
	if err != nil {
		return false, fmt.Errorf("resolve announcement sms recipients: %w", err)
	}

	if len(recipients) == 0 {
		return false, nil
	}

	if err := s.repo.SaveAudienceSnapshot(
		ctx,
		item.ID,
		string(NotificationChannelSMS),
		announcementSMSRecipientUserIDs(recipients),
	); err != nil {
		return false, fmt.Errorf("save announcement sms audience snapshot: %w", err)
	}

	if len(notification.Deliveries) == 0 {
		notification.Deliveries = append(notification.Deliveries, NotificationDeliveryRequest{
			Channel: NotificationChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		})
	}

	body := s.announcementSMSMessage(item)
	for _, recipient := range recipients {
		phone := strings.TrimSpace(recipient.PhoneNumber)
		if phone == "" {
			continue
		}

		notification.Deliveries = append(notification.Deliveries, NotificationDeliveryRequest{
			Channel: NotificationChannelSMS,
			Recipient: map[string]any{
				"user_id": recipient.ID.String(),
				"phone":   phone,
			},
			Payload: map[string]any{
				"body":            body,
				"announcement_id": item.ID.String(),
				"title":           item.Title,
			},
			ScheduledAt: scheduledAt,
			MaxAttempts: 3,
		})
	}

	return true, nil
}

// publishAnnouncementDeliveries hands the notification to the publisher and,
// only if that succeeded, marks the email/SMS deliveries it carried as queued.
// A publisher failure stays best effort (the lifecycle change stands) but
// leaves the markers unset, so publishing again re-sends instead of skipping.
func (s *Service) publishAnnouncementDeliveries(
	ctx context.Context,
	notification Notification,
	item Announcement,
	emailQueued bool,
	smsQueued bool,
) (Announcement, error) {
	if s == nil || s.notifications == nil {
		return item, nil
	}
	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}
	if err := s.notifications.Publish(ctx, notification); err != nil {
		return item, nil
	}
	if smsQueued {
		marked, err := s.repo.MarkSMSNotificationQueued(ctx, item.ID)
		if err != nil {
			return item, fmt.Errorf("mark announcement sms notification queued: %w", err)
		}
		item = marked
	}
	if emailQueued {
		marked, err := s.repo.MarkEmailNotificationSent(ctx, item.ID)
		if err != nil {
			return item, fmt.Errorf("mark announcement email notification sent: %w", err)
		}
		item = marked
	}
	return item, nil
}

func (s *Service) resolveAnnouncementSMSRecipients(
	ctx context.Context,
	emailRecipients []AnnouncementEmailRecipient,
) ([]AnnouncementSMSRecipient, error) {
	if len(emailRecipients) == 0 {
		return nil, nil
	}

	userIDs := make([]uuid.UUID, 0, len(emailRecipients))
	recipientByID := make(map[uuid.UUID]AnnouncementEmailRecipient, len(emailRecipients))
	for _, recipient := range emailRecipients {
		if recipient.ID == uuid.Nil {
			continue
		}
		userIDs = append(userIDs, recipient.ID)
		recipientByID[recipient.ID] = recipient
	}

	items, err := s.repo.ListSMSRecipientsForUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for i := range items {
		if base, ok := recipientByID[items[i].ID]; ok {
			items[i].Username = base.Username
			items[i].FullName = base.FullName
		}
	}

	return items, nil
}

func (s *Service) announcementSMSMessage(
	item Announcement,
) string {
	if item.SmsMessage.Valid && strings.TrimSpace(item.SmsMessage.String) != "" {
		return strings.TrimSpace(item.SmsMessage.String)
	}

	message := strings.TrimSpace(item.Summary.String)
	if message == "" {
		message = strings.TrimSpace(item.Message)
	}
	if message == "" {
		message = strings.TrimSpace(item.Title)
	}

	prefix := strings.TrimSpace(s.platformName())
	if prefix == "" {
		prefix = "MOH"
	}

	body := fmt.Sprintf("%s: %s", prefix, message)
	maxLength := 160
	if s != nil && s.settings != nil && s.settings.SMS.MaxLength > 0 {
		maxLength = s.settings.SMS.MaxLength
	}

	runes := []rune(body)
	if len(runes) <= maxLength {
		return body
	}

	if maxLength <= 1 {
		return string(runes[:maxLength])
	}

	return string(runes[:maxLength-1]) + "…"
}

func (s *Service) attachAnnouncementEmailDelivery(
	notification *Notification,
	item Announcement,
	recipients []AnnouncementEmailRecipient,
	options AnnouncementEmailOptions,
) {
	if notification == nil {
		return
	}

	deliveries := notification.Deliveries
	if len(deliveries) == 0 {
		deliveries = append(deliveries, NotificationDeliveryRequest{
			Channel: NotificationChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		})
	}

	for _, recipient := range recipients {
		email := strings.TrimSpace(recipient.Email)
		if email == "" {
			continue
		}

		name := strings.TrimSpace(recipient.FullName)
		if name == "" {
			name = strings.TrimSpace(recipient.Username)
		}

		if name == "" {
			name = email
		}

		attachments := []Attachment(nil)
		if options.IncludeAttachmentsInEmail {
			attachments = options.Attachments
		}
		attachmentNames := announcementEmailAttachmentNames(attachments)
		attachmentLinks := options.AttachmentLinks
		attachmentCount := len(attachments)
		if len(attachmentLinks) > attachmentCount {
			attachmentCount = len(attachmentLinks)
		}
		actionURL := s.announcementActionURL(item)
		relatedLinkURL := s.announcementRelatedLinkURL(item)
		relatedLinkLabel := s.announcementRelatedLinkLabel(item)

		deliveries = append(deliveries, NotificationDeliveryRequest{
			Channel: NotificationChannelEmail,
			Recipient: map[string]any{
				"user_id": recipient.ID,
				"name":    name,
				"email":   email,
			},
			TemplateName: "announcement",
			TemplateData: map[string]any{
				"Name":                  name,
				"Platform":              s.platformName(),
				"Title":                 item.Title,
				"Summary":               nullStringValue(item.Summary),
				"Message":               item.Message,
				"Level":                 announcementLevelString(item.Level),
				"Status":                announcementStatusString(item.Status),
				"AnnouncementID":        item.ID.String(),
				"HasAttachments":        attachmentCount > 0,
				"AttachmentCount":       attachmentCount,
				"AttachmentNames":       attachmentNames,
				"AttachmentLinks":       attachmentLinks,
				"AnnouncementLinkURL":   relatedLinkURL,
				"AnnouncementLinkLabel": relatedLinkLabel,
				"ActionURL":             actionURL,
				"Details": fmt.Sprintf(
					"Title: %s\nLevel: %s\nStatus: %s\nMessage: %s",
					item.Title,
					announcementLevelString(item.Level),
					announcementStatusString(item.Status),
					item.Message,
				),
			},
			Payload: map[string]any{
				"subject":   fmt.Sprintf("[Announcement] %s", item.Title),
				"text_body": item.Message,
			},
			Attachments: attachments,
			ScheduledAt: options.ScheduledAt,
			MaxAttempts: 5,
		})
	}

	notification.Deliveries = deliveries
}

func announcementEmailAttachmentNames(attachments []Attachment) []string {
	names := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.FileName)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (s *Service) announcementAttachmentDownloadURL(
	announcementID uuid.UUID,
	attachmentID uuid.UUID,
) string {
	if announcementID == uuid.Nil || attachmentID == uuid.Nil {
		return ""
	}

	base := ""
	if s != nil && s.settings != nil {
		base = strings.TrimRight(strings.TrimSpace(s.settings.AppBaseURL), "/")
	}

	if base == "" {
		base = "http://localhost:9000"
	}

	return fmt.Sprintf(
		"%s/api/v1/announcements/%s/attachments/%s/download",
		base,
		announcementID.String(),
		attachmentID.String(),
	)
}

func (s *Service) normalizeAnnouncementEmailOptions(options ...AnnouncementEmailOptions) (AnnouncementEmailOptions, error) {
	out := AnnouncementEmailOptions{IncludeAttachmentsInEmail: true}
	if len(options) > 0 {
		out = options[0]
		if len(out.Attachments) > 0 && !out.IncludeAttachmentsInEmail {
			return out, nil
		}
	}
	if len(out.Attachments) == 0 {
		return out, nil
	}
	if s.emailAttachments == nil {
		return AnnouncementEmailOptions{}, errors.New("email attachment normalizer is required")
	}
	attachments, err := s.emailAttachments.NormalizeAttachments(out.Attachments)
	if err != nil {
		return AnnouncementEmailOptions{}, fmt.Errorf("validate announcement email attachments: %w", err)
	}
	out.Attachments = attachments
	return out, nil
}

func (s *Service) attachAdminEmailDelivery(
	notification *Notification,
	templateName string,
	subject string,
	textBody string,
	templateData map[string]any,
) {
	if notification == nil {
		return
	}

	adminEmail := strings.TrimSpace(s.systemAdminEmail())
	if adminEmail == "" {
		return
	}

	if templateData == nil {
		templateData = map[string]any{}
	}

	if _, ok := templateData["Name"]; !ok {
		templateData["Name"] = s.systemAdminName()
	}

	if _, ok := templateData["Platform"]; !ok {
		templateData["Platform"] = s.platformName()
	}

	if _, ok := templateData["ActionURL"]; !ok {
		templateData["ActionURL"] = s.adminAnnouncementsURL()
	}

	notification.Deliveries = []NotificationDeliveryRequest{
		{
			Channel: NotificationChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		},
		{
			Channel: NotificationChannelEmail,
			Recipient: map[string]any{
				"name":  s.systemAdminName(),
				"email": adminEmail,
			},
			TemplateName: templateName,
			TemplateData: templateData,
			Payload: map[string]any{
				"subject":   subject,
				"text_body": textBody,
			},
			MaxAttempts: 5,
		},
	}
}

// ---------------------------------
// URL / config helpers
// ---------------------------------

func (s *Service) platformName() string {
	if s != nil && s.settings != nil && strings.TrimSpace(s.settings.Notification.PlatformName) != "" {
		return strings.TrimSpace(s.settings.Notification.PlatformName)
	}

	return "MOH Integrated Health Portal"
}

func (s *Service) systemAdminName() string {
	if s != nil && s.settings != nil && strings.TrimSpace(s.settings.Notification.SystemAdminName) != "" {
		return strings.TrimSpace(s.settings.Notification.SystemAdminName)
	}

	return "System Administrator"
}

func (s *Service) systemAdminEmail() string {
	if s != nil && s.settings != nil && strings.TrimSpace(s.settings.Notification.SystemAdminEmail) != "" {
		return strings.TrimSpace(s.settings.Notification.SystemAdminEmail)
	}

	return ""
}

func (s *Service) adminDashboardURL() string {
	if s != nil && s.settings != nil && strings.TrimSpace(s.settings.Notification.AdminDashboardURL) != "" {
		return strings.TrimSpace(s.settings.Notification.AdminDashboardURL)
	}

	return "http://localhost:3000/admin/home"
}

func (s *Service) adminAnnouncementsURL() string {
	base := strings.TrimRight(s.adminDashboardURL(), "/")

	if strings.HasSuffix(base, "/admin/home") {
		return strings.TrimSuffix(base, "/admin/home") + "/admin/announcements"
	}

	return base + "/announcements"
}

func (s *Service) portalAnnouncementsURL() string {
	base := strings.TrimRight(s.adminDashboardURL(), "/")

	if strings.HasSuffix(base, "/admin/home") {
		return strings.TrimSuffix(base, "/admin/home") + "/apps/news"
	}

	return base + "/apps/news"
}

func (s *Service) announcementActionURL(item Announcement) string {
	return s.absolutePortalURL(announcementLinkOrDefault(item, s.portalAnnouncementsURL()))
}

func (s *Service) announcementRelatedLinkURL(item Announcement) string {
	if !item.LinkUrl.Valid || strings.TrimSpace(item.LinkUrl.String) == "" {
		return ""
	}

	return s.absolutePortalURL(item.LinkUrl.String)
}

func (s *Service) announcementRelatedLinkLabel(item Announcement) string {
	if item.LinkLabel.Valid && strings.TrimSpace(item.LinkLabel.String) != "" {
		return strings.TrimSpace(item.LinkLabel.String)
	}

	return "Open related link"
}

func (s *Service) absolutePortalURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if parsed, err := url.Parse(value); err == nil && parsed.IsAbs() {
		return value
	}

	base := ""
	if s != nil && s.settings != nil {
		base = strings.TrimSpace(s.settings.FrontendBaseURL)
		if base == "" {
			base = strings.TrimSpace(s.settings.FrontendRedirectURI)
		}
	}
	if base == "" {
		base = s.adminDashboardURL()
	}
	if base == "" {
		return value
	}

	parsedBase, err := url.Parse(base)
	if err != nil || parsedBase.Scheme == "" || parsedBase.Host == "" {
		return value
	}

	if strings.HasPrefix(value, "/") {
		return parsedBase.Scheme + "://" + parsedBase.Host + value
	}

	basePath := strings.TrimRight(parsedBase.Path, "/")
	if basePath == "" {
		return parsedBase.Scheme + "://" + parsedBase.Host + "/" + strings.TrimLeft(value, "/")
	}

	return parsedBase.Scheme + "://" + parsedBase.Host + basePath + "/" + strings.TrimLeft(value, "/")
}

// ---------------------------------
// Value helpers
// ---------------------------------

func nullStringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}

	return strings.TrimSpace(ns.String)
}

func announcementStatusString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []byte:
		return strings.TrimSpace(string(v))
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case AnnouncementStatus:
		return strings.TrimSpace(string(v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func announcementLevelString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []byte:
		return strings.TrimSpace(string(v))
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case AnnouncementLevel:
		return strings.TrimSpace(string(v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func announcementAudienceTypeString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []byte:
		return strings.TrimSpace(string(v))
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case AnnouncementAudienceType:
		return strings.TrimSpace(string(v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func announcementLinkOrDefault(item Announcement, fallback string) string {
	if item.LinkUrl.Valid && strings.TrimSpace(item.LinkUrl.String) != "" {
		return strings.TrimSpace(item.LinkUrl.String)
	}

	return fallback
}

func announcementJSON(value any) json.RawMessage { data, _ := json.Marshal(value); return data }
