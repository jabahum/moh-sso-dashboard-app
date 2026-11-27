package announcements

import (
	"context"

	"github.com/google/uuid"
)

// persistenceAdapter translates contract values around package-owned SQL queries.
type persistenceAdapter struct{ queries *announcementsRepository }

var _ Repository = (*persistenceAdapter)(nil)

func (a *persistenceAdapter) Create(ctx context.Context, params CreateAnnouncementParams) (Announcement, error) {
	value, err := a.queries.Create(ctx, toSQLCreateAnnouncementParams(params))
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) GetByID(ctx context.Context, id uuid.UUID) (Announcement, error) {
	value, err := a.queries.GetByID(ctx, id)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) Update(ctx context.Context, params UpdateAnnouncementParams) (Announcement, error) {
	value, err := a.queries.Update(ctx, toSQLUpdateAnnouncementParams(params))
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) MarkEmailNotificationSent(ctx context.Context, id uuid.UUID) (Announcement, error) {
	value, err := a.queries.MarkEmailNotificationSent(ctx, id)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) MarkSMSNotificationQueued(ctx context.Context, id uuid.UUID) (Announcement, error) {
	value, err := a.queries.MarkSMSNotificationQueued(ctx, id)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) ListSMSRecipientsForUsers(ctx context.Context, userIDs []uuid.UUID) ([]AnnouncementSMSRecipient, error) {
	return a.queries.ListSMSRecipientsForUsers(ctx, userIDs)
}

func (a *persistenceAdapter) CreateAttachment(ctx context.Context, params CreateAnnouncementAttachmentParams) (AnnouncementAttachment, error) {
	value, err := a.queries.CreateAttachment(ctx, toSQLCreateAnnouncementAttachmentParams(params))
	return fromSQLAnnouncementAttachment(value), err
}

func (a *persistenceAdapter) ListAttachmentsByAnnouncementID(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error) {
	value, err := a.queries.ListAttachmentsByAnnouncementID(ctx, announcementID)
	if value == nil {
		return nil, err
	}
	result := make([]AnnouncementAttachment, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncementAttachment(item)
	}
	return result, err
}

func (a *persistenceAdapter) ListEmailAttachments(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error) {
	value, err := a.queries.ListEmailAttachments(ctx, announcementID)
	if value == nil {
		return nil, err
	}
	result := make([]AnnouncementAttachment, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncementAttachment(item)
	}
	return result, err
}

func (a *persistenceAdapter) CountAttachments(ctx context.Context, announcementID uuid.UUID) (int64, error) {
	return a.queries.CountAttachments(ctx, announcementID)
}

func (a *persistenceAdapter) GetAttachmentByID(ctx context.Context, params GetAnnouncementAttachmentByIDParams) (AnnouncementAttachment, error) {
	value, err := a.queries.GetAttachmentByID(ctx, toSQLGetAnnouncementAttachmentByIDParams(params))
	return fromSQLAnnouncementAttachment(value), err
}

func (a *persistenceAdapter) UpdateAttachment(ctx context.Context, params UpdateAnnouncementAttachmentParams) (AnnouncementAttachment, error) {
	value, err := a.queries.UpdateAttachment(ctx, toSQLUpdateAnnouncementAttachmentParams(params))
	return fromSQLAnnouncementAttachment(value), err
}

func (a *persistenceAdapter) SoftDeleteAttachment(ctx context.Context, params SoftDeleteAnnouncementAttachmentParams) (AnnouncementAttachment, error) {
	value, err := a.queries.SoftDeleteAttachment(ctx, toSQLSoftDeleteAnnouncementAttachmentParams(params))
	return fromSQLAnnouncementAttachment(value), err
}

func (a *persistenceAdapter) SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	return a.queries.SoftDelete(ctx, id, deletedBy)
}

func (a *persistenceAdapter) Restore(ctx context.Context, id uuid.UUID, updatedBy uuid.UUID) (Announcement, error) {
	value, err := a.queries.Restore(ctx, id, updatedBy)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) ListAdmin(ctx context.Context, params ListAnnouncementsAdminParams) ([]Announcement, error) {
	value, err := a.queries.ListAdmin(ctx, toSQLListAnnouncementsAdminParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) CountAdmin(ctx context.Context) (int64, error) {
	return a.queries.CountAdmin(ctx)
}

func (a *persistenceAdapter) ListByStatus(ctx context.Context, params ListAnnouncementsByStatusParams) ([]Announcement, error) {
	value, err := a.queries.ListByStatus(ctx, toSQLListAnnouncementsByStatusParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) CountByStatus(ctx context.Context, status AnnouncementStatus) (int64, error) {
	return a.queries.CountByStatus(ctx, AnnouncementStatus(status))
}

func (a *persistenceAdapter) SearchAdmin(ctx context.Context, params SearchAnnouncementsAdminParams) ([]Announcement, error) {
	value, err := a.queries.SearchAdmin(ctx, toSQLSearchAnnouncementsAdminParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) CountSearchAdmin(ctx context.Context, search string) (int64, error) {
	return a.queries.CountSearchAdmin(ctx, search)
}

func (a *persistenceAdapter) ListActivePublished(ctx context.Context, params ListActivePublishedAnnouncementsParams) ([]Announcement, error) {
	value, err := a.queries.ListActivePublished(ctx, toSQLListActivePublishedAnnouncementsParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) CountActivePublished(ctx context.Context) (int64, error) {
	return a.queries.CountActivePublished(ctx)
}

func (a *persistenceAdapter) ListCreatedByUser(ctx context.Context, params ListAnnouncementsCreatedByUserParams) ([]Announcement, error) {
	value, err := a.queries.ListCreatedByUser(ctx, toSQLListAnnouncementsCreatedByUserParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) CountCreatedByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return a.queries.CountCreatedByUser(ctx, userID)
}

func (a *persistenceAdapter) ClaimDueScheduled(ctx context.Context, limit int32) ([]Announcement, error) {
	value, err := a.queries.ClaimDueScheduled(ctx, limit)
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) PublishNow(ctx context.Context, id uuid.UUID, publishedBy uuid.UUID) (Announcement, error) {
	value, err := a.queries.PublishNow(ctx, id, publishedBy)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) Draft(ctx context.Context, params DraftAnnouncementParams) (Announcement, error) {
	value, err := a.queries.Draft(ctx, toSQLDraftAnnouncementParams(params))
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) Schedule(ctx context.Context, params ScheduleAnnouncementParams) (Announcement, error) {
	value, err := a.queries.Schedule(ctx, toSQLScheduleAnnouncementParams(params))
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) Archive(ctx context.Context, id uuid.UUID, archivedBy uuid.UUID) (Announcement, error) {
	value, err := a.queries.Archive(ctx, id, archivedBy)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) UnarchiveToDraft(ctx context.Context, id uuid.UUID, updatedBy uuid.UUID) (Announcement, error) {
	value, err := a.queries.UnarchiveToDraft(ctx, id, updatedBy)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) UpdateStatus(ctx context.Context, params UpdateAnnouncementStatusParams) (Announcement, error) {
	value, err := a.queries.UpdateStatus(ctx, toSQLUpdateAnnouncementStatusParams(params))
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) SetPinned(ctx context.Context, id uuid.UUID, isPinned bool, updatedBy uuid.UUID) (Announcement, error) {
	value, err := a.queries.SetPinned(ctx, id, isPinned, updatedBy)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) SetPriority(ctx context.Context, id uuid.UUID, priority int32, updatedBy uuid.UUID) (Announcement, error) {
	value, err := a.queries.SetPriority(ctx, id, priority, updatedBy)
	return fromSQLAnnouncement(value), err
}

func (a *persistenceAdapter) AddClientAudience(ctx context.Context, announcementID uuid.UUID, clientID uuid.UUID) error {
	return a.queries.AddClientAudience(ctx, announcementID, clientID)
}

func (a *persistenceAdapter) ReplaceClientAudience(ctx context.Context, announcementID uuid.UUID, clientIDs []uuid.UUID) error {
	return a.queries.ReplaceClientAudience(ctx, announcementID, clientIDs)
}

func (a *persistenceAdapter) ListClientAudience(ctx context.Context, announcementID uuid.UUID) ([]uuid.UUID, error) {
	return a.queries.ListClientAudience(ctx, announcementID)
}

func (a *persistenceAdapter) AddRoleAudience(ctx context.Context, announcementID uuid.UUID, roleName string) error {
	return a.queries.AddRoleAudience(ctx, announcementID, roleName)
}

func (a *persistenceAdapter) ReplaceRoleAudience(ctx context.Context, announcementID uuid.UUID, roleNames []string) error {
	return a.queries.ReplaceRoleAudience(ctx, announcementID, roleNames)
}

func (a *persistenceAdapter) ListRoleAudience(ctx context.Context, announcementID uuid.UUID) ([]string, error) {
	return a.queries.ListRoleAudience(ctx, announcementID)
}

func (a *persistenceAdapter) AddUserAudience(ctx context.Context, announcementID uuid.UUID, userID uuid.UUID) error {
	return a.queries.AddUserAudience(ctx, announcementID, userID)
}

func (a *persistenceAdapter) ReplaceUserAudience(ctx context.Context, announcementID uuid.UUID, userIDs []uuid.UUID) error {
	return a.queries.ReplaceUserAudience(ctx, announcementID, userIDs)
}

func (a *persistenceAdapter) ListUserAudience(ctx context.Context, announcementID uuid.UUID) ([]uuid.UUID, error) {
	return a.queries.ListUserAudience(ctx, announcementID)
}

func (a *persistenceAdapter) AddGroupAudience(ctx context.Context, announcementID uuid.UUID, groupID uuid.UUID) error {
	return a.queries.AddGroupAudience(ctx, announcementID, groupID)
}

func (a *persistenceAdapter) ReplaceGroupAudience(ctx context.Context, announcementID uuid.UUID, groupIDs []uuid.UUID) error {
	return a.queries.ReplaceGroupAudience(ctx, announcementID, groupIDs)
}

func (a *persistenceAdapter) ListGroupAudience(ctx context.Context, announcementID uuid.UUID) ([]uuid.UUID, error) {
	return a.queries.ListGroupAudience(ctx, announcementID)
}

func (a *persistenceAdapter) ReplaceHealthContextAudience(ctx context.Context, announcementID uuid.UUID, contextIDs []uuid.UUID, includeDescendants bool) error {
	return a.queries.ReplaceHealthContextAudience(ctx, announcementID, contextIDs, includeDescendants)
}

func (a *persistenceAdapter) ListHealthContextAudience(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementHealthContextAudience, error) {
	return a.queries.ListHealthContextAudience(ctx, announcementID)
}

func (a *persistenceAdapter) SaveAudienceSnapshot(ctx context.Context, announcementID uuid.UUID, channel string, recipientUserIDs []string) error {
	return a.queries.SaveAudienceSnapshot(ctx, announcementID, channel, recipientUserIDs)
}

func (a *persistenceAdapter) ListForClient(ctx context.Context, params ListAnnouncementsForClientParams) ([]Announcement, error) {
	value, err := a.queries.ListForClient(ctx, toSQLListAnnouncementsForClientParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) ListForRole(ctx context.Context, params ListAnnouncementsForRoleParams) ([]Announcement, error) {
	value, err := a.queries.ListForRole(ctx, toSQLListAnnouncementsForRoleParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) ListForUser(ctx context.Context, params ListAnnouncementsForUserParams) ([]Announcement, error) {
	value, err := a.queries.ListForUser(ctx, toSQLListAnnouncementsForUserParams(params))
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) ListPublicAnnouncements(ctx context.Context, limit int32, offset int32) ([]Announcement, error) {
	value, err := a.queries.ListPublicAnnouncements(ctx, limit, offset)
	if value == nil {
		return nil, err
	}
	result := make([]Announcement, len(value))
	for i, item := range value {
		result[i] = fromSQLAnnouncement(item)
	}
	return result, err
}

func (a *persistenceAdapter) GetStats(ctx context.Context) (GetAnnouncementStatsRow, error) {
	value, err := a.queries.GetStats(ctx)
	return fromSQLGetAnnouncementStatsRow(value), err
}
