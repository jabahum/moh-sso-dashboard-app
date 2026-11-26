package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
)

type Store interface {
	Querier
	ExecTx(ctx context.Context, fn func(q Querier) error) error
}

type SQLStore struct {
	db *sql.DB
	*Queries
}

func NewStore(db *sql.DB) *SQLStore {
	return &SQLStore{
		db:      db,
		Queries: New(db),
	}
}

func (s *SQLStore) DB() *sql.DB {
	return s.db
}

func (s *SQLStore) ExecTx(ctx context.Context, fn func(q Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin tx: %w", err)
	}

	q := New(tx)
	defer tx.Rollback()

	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx rollback error: %v, original error: %w", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}

type Querier interface {
	ArchiveAnnouncement(ctx context.Context, arg ArchiveAnnouncementParams) (Announcement, error)
	ClaimDueScheduledAnnouncements(ctx context.Context, limit int32) ([]Announcement, error)
	CountActivePublishedAnnouncements(ctx context.Context) (int64, error)
	CountAnnouncementAttachments(ctx context.Context, announcementID uuid.UUID) (int64, error)
	CountAnnouncementsAdmin(ctx context.Context) (int64, error)
	CountAnnouncementsByStatus(ctx context.Context, status AnnouncementStatus) (int64, error)
	CountAnnouncementsCreatedByUser(ctx context.Context, createdBy uuid.UUID) (int64, error)
	CountSearchAnnouncementsAdmin(ctx context.Context, searchText sql.NullString) (int64, error)
	CreateAnnouncement(ctx context.Context, arg CreateAnnouncementParams) (Announcement, error)
	CreateAnnouncementAttachment(ctx context.Context, arg CreateAnnouncementAttachmentParams) (AnnouncementAttachment, error)
	DeleteAnnouncementClients(ctx context.Context, announcementID uuid.UUID) error
	DeleteAnnouncementRoles(ctx context.Context, announcementID uuid.UUID) error
	DeleteAnnouncementUsers(ctx context.Context, announcementID uuid.UUID) error
	DraftAnnouncement(ctx context.Context, arg DraftAnnouncementParams) (Announcement, error)
	GetAnnouncementAttachmentByID(ctx context.Context, arg GetAnnouncementAttachmentByIDParams) (AnnouncementAttachment, error)
	GetAnnouncementByID(ctx context.Context, id uuid.UUID) (Announcement, error)
	GetAnnouncementStats(ctx context.Context) (GetAnnouncementStatsRow, error)
	InsertAnnouncementClient(ctx context.Context, arg InsertAnnouncementClientParams) error
	InsertAnnouncementRole(ctx context.Context, arg InsertAnnouncementRoleParams) error
	InsertAnnouncementUser(ctx context.Context, arg InsertAnnouncementUserParams) error
	ListActivePublishedAnnouncements(ctx context.Context, arg ListActivePublishedAnnouncementsParams) ([]Announcement, error)
	ListAnnouncementAttachmentsByAnnouncementID(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error)
	ListAnnouncementClients(ctx context.Context, announcementID uuid.UUID) ([]uuid.UUID, error)
	ListAnnouncementEmailAttachments(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error)
	ListAnnouncementRoles(ctx context.Context, announcementID uuid.UUID) ([]string, error)
	ListAnnouncementUsers(ctx context.Context, announcementID uuid.UUID) ([]uuid.UUID, error)
	ListAnnouncementsAdmin(ctx context.Context, arg ListAnnouncementsAdminParams) ([]Announcement, error)
	ListAnnouncementsByStatus(ctx context.Context, arg ListAnnouncementsByStatusParams) ([]Announcement, error)
	ListAnnouncementsCreatedByUser(ctx context.Context, arg ListAnnouncementsCreatedByUserParams) ([]Announcement, error)
	ListAnnouncementsForClient(ctx context.Context, arg ListAnnouncementsForClientParams) ([]Announcement, error)
	ListAnnouncementsForRole(ctx context.Context, arg ListAnnouncementsForRoleParams) ([]Announcement, error)
	ListAnnouncementsForUser(ctx context.Context, arg ListAnnouncementsForUserParams) ([]Announcement, error)
	ListPublicAnnouncements(ctx context.Context, arg ListPublicAnnouncementsParams) ([]Announcement, error)
	MarkAnnouncementEmailNotificationSent(ctx context.Context, id uuid.UUID) (Announcement, error)
	MarkAnnouncementSMSNotificationQueued(ctx context.Context, id uuid.UUID) (Announcement, error)
	PublishAnnouncementNow(ctx context.Context, arg PublishAnnouncementNowParams) (Announcement, error)
	RestoreAnnouncement(ctx context.Context, arg RestoreAnnouncementParams) (Announcement, error)
	ScheduleAnnouncement(ctx context.Context, arg ScheduleAnnouncementParams) (Announcement, error)
	SearchAnnouncementsAdmin(ctx context.Context, arg SearchAnnouncementsAdminParams) ([]Announcement, error)
	SetAnnouncementPinned(ctx context.Context, arg SetAnnouncementPinnedParams) (Announcement, error)
	SetAnnouncementPriority(ctx context.Context, arg SetAnnouncementPriorityParams) (Announcement, error)
	SoftDeleteAnnouncement(ctx context.Context, arg SoftDeleteAnnouncementParams) error
	SoftDeleteAnnouncementAttachment(ctx context.Context, arg SoftDeleteAnnouncementAttachmentParams) (AnnouncementAttachment, error)
	UnarchiveAnnouncementToDraft(ctx context.Context, arg UnarchiveAnnouncementToDraftParams) (Announcement, error)
	UpdateAnnouncement(ctx context.Context, arg UpdateAnnouncementParams) (Announcement, error)
	UpdateAnnouncementAttachment(ctx context.Context, arg UpdateAnnouncementAttachmentParams) (AnnouncementAttachment, error)
	UpdateAnnouncementStatus(ctx context.Context, arg UpdateAnnouncementStatusParams) (Announcement, error)
}
