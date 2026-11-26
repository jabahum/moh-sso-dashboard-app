package announcements

import (
	"context"

	"github.com/google/uuid"
)

type AnnouncementRepository interface {
	// ---------------------------------
	// Core CRUD
	// ---------------------------------

	Create(ctx context.Context, params CreateAnnouncementParams) (Announcement, error)
	GetByID(ctx context.Context, id uuid.UUID) (Announcement, error)
	Update(ctx context.Context, params UpdateAnnouncementParams) (Announcement, error)
	MarkEmailNotificationSent(ctx context.Context, id uuid.UUID) (Announcement, error)
	MarkSMSNotificationQueued(ctx context.Context, id uuid.UUID) (Announcement, error)
	ListSMSRecipientsForUsers(ctx context.Context, userIDs []uuid.UUID) ([]AnnouncementSMSRecipient, error)
	CreateAttachment(ctx context.Context, params CreateAnnouncementAttachmentParams) (AnnouncementAttachment, error)
	ListAttachmentsByAnnouncementID(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error)
	ListEmailAttachments(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error)
	CountAttachments(ctx context.Context, announcementID uuid.UUID) (int64, error)
	GetAttachmentByID(ctx context.Context, params GetAnnouncementAttachmentByIDParams) (AnnouncementAttachment, error)
	UpdateAttachment(ctx context.Context, params UpdateAnnouncementAttachmentParams) (AnnouncementAttachment, error)
	SoftDeleteAttachment(ctx context.Context, params SoftDeleteAnnouncementAttachmentParams) (AnnouncementAttachment, error)

	SoftDelete(
		ctx context.Context,
		id uuid.UUID,
		deletedBy uuid.UUID,
	) error

	Restore(
		ctx context.Context,
		id uuid.UUID,
		updatedBy uuid.UUID,
	) (Announcement, error)

	// ---------------------------------
	// Listing / search
	// ---------------------------------

	ListAdmin(
		ctx context.Context,
		params ListAnnouncementsAdminParams,
	) ([]Announcement, error)

	CountAdmin(
		ctx context.Context,
	) (int64, error)

	ListByStatus(
		ctx context.Context,
		params ListAnnouncementsByStatusParams,
	) ([]Announcement, error)

	CountByStatus(
		ctx context.Context,
		status AnnouncementStatus,
	) (int64, error)

	SearchAdmin(
		ctx context.Context,
		params SearchAnnouncementsAdminParams,
	) ([]Announcement, error)

	CountSearchAdmin(
		ctx context.Context,
		search string,
	) (int64, error)

	ListActivePublished(
		ctx context.Context,
		params ListActivePublishedAnnouncementsParams,
	) ([]Announcement, error)

	CountActivePublished(
		ctx context.Context,
	) (int64, error)

	ListCreatedByUser(
		ctx context.Context,
		params ListAnnouncementsCreatedByUserParams,
	) ([]Announcement, error)

	CountCreatedByUser(
		ctx context.Context,
		userID uuid.UUID,
	) (int64, error)

	// ---------------------------------
	// Lifecycle actions
	// ---------------------------------

	PublishNow(ctx context.Context, id uuid.UUID, publishedBy uuid.UUID) (Announcement, error)

	// ClaimDueScheduled atomically moves up to limit SCHEDULED announcements
	// whose publish_at has passed (and that have not expired) to PUBLISHED and
	// returns them. Concurrent callers never claim the same announcement.
	ClaimDueScheduled(ctx context.Context, limit int32) ([]Announcement, error)
	Draft(
		ctx context.Context,
		params DraftAnnouncementParams,
	) (Announcement, error)

	Schedule(
		ctx context.Context,
		params ScheduleAnnouncementParams,
	) (Announcement, error)

	Archive(
		ctx context.Context,
		id uuid.UUID,
		archivedBy uuid.UUID,
	) (Announcement, error)

	UnarchiveToDraft(
		ctx context.Context,
		id uuid.UUID,
		updatedBy uuid.UUID,
	) (Announcement, error)

	UpdateStatus(
		ctx context.Context,
		params UpdateAnnouncementStatusParams,
	) (Announcement, error)

	SetPinned(
		ctx context.Context,
		id uuid.UUID,
		isPinned bool,
		updatedBy uuid.UUID,
	) (Announcement, error)

	SetPriority(
		ctx context.Context,
		id uuid.UUID,
		priority int32,
		updatedBy uuid.UUID,
	) (Announcement, error)

	// ---------------------------------
	// Audience mappings
	// ---------------------------------

	AddClientAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		clientID uuid.UUID,
	) error

	ReplaceClientAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		clientIDs []uuid.UUID,
	) error

	ListClientAudience(
		ctx context.Context,
		announcementID uuid.UUID,
	) ([]uuid.UUID, error)

	AddRoleAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		roleName string,
	) error

	ReplaceRoleAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		roleNames []string,
	) error

	ListRoleAudience(
		ctx context.Context,
		announcementID uuid.UUID,
	) ([]string, error)

	AddUserAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		userID uuid.UUID,
	) error

	ReplaceUserAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		userIDs []uuid.UUID,
	) error

	ListUserAudience(
		ctx context.Context,
		announcementID uuid.UUID,
	) ([]uuid.UUID, error)

	AddGroupAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		groupID uuid.UUID,
	) error

	ReplaceGroupAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		groupIDs []uuid.UUID,
	) error

	ListGroupAudience(
		ctx context.Context,
		announcementID uuid.UUID,
	) ([]uuid.UUID, error)

	ReplaceHealthContextAudience(
		ctx context.Context,
		announcementID uuid.UUID,
		contextIDs []uuid.UUID,
		includeDescendants bool,
	) error

	ListHealthContextAudience(
		ctx context.Context,
		announcementID uuid.UUID,
	) ([]AnnouncementHealthContextAudience, error)

	SaveAudienceSnapshot(
		ctx context.Context,
		announcementID uuid.UUID,
		channel string,
		recipientUserIDs []string,
	) error

	// ---------------------------------
	// End-user targeting queries
	// ---------------------------------

	ListForClient(
		ctx context.Context,
		params ListAnnouncementsForClientParams,
	) ([]Announcement, error)

	ListForRole(
		ctx context.Context,
		params ListAnnouncementsForRoleParams,
	) ([]Announcement, error)

	ListForUser(
		ctx context.Context,
		params ListAnnouncementsForUserParams,
	) ([]Announcement, error)

	ListPublicAnnouncements(
		ctx context.Context,
		limit int32,
		offset int32,
	) ([]Announcement, error)

	// ---------------------------------
	// Reporting
	// ---------------------------------

	GetStats(
		ctx context.Context,
	) (GetAnnouncementStatsRow, error)
}

type Repository = AnnouncementRepository
