// Package postgres owns Announcements SQL and transactions, initially extracted from host SQLC queries.
package postgres

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"time"
)

type Announcement struct {
	ID                      uuid.UUID                `json:"id"`
	Title                   string                   `json:"title"`
	Message                 string                   `json:"message"`
	Summary                 sql.NullString           `json:"summary"`
	Level                   AnnouncementLevel        `json:"level"`
	Tag                     sql.NullString           `json:"tag"`
	LinkUrl                 sql.NullString           `json:"link_url"`
	Priority                int32                    `json:"priority"`
	IsPinned                bool                     `json:"is_pinned"`
	Status                  AnnouncementStatus       `json:"status"`
	PublishAt               sql.NullTime             `json:"publish_at"`
	ExpiresAt               sql.NullTime             `json:"expires_at"`
	AudienceType            AnnouncementAudienceType `json:"audience_type"`
	CreatedBy               uuid.UUID                `json:"created_by"`
	UpdatedBy               uuid.NullUUID            `json:"updated_by"`
	PublishedBy             uuid.NullUUID            `json:"published_by"`
	ArchivedBy              uuid.NullUUID            `json:"archived_by"`
	CreatedAt               time.Time                `json:"created_at"`
	UpdatedAt               time.Time                `json:"updated_at"`
	PublishedAt             sql.NullTime             `json:"published_at"`
	ArchivedAt              sql.NullTime             `json:"archived_at"`
	DeletedAt               sql.NullTime             `json:"deleted_at"`
	DeletedBy               uuid.NullUUID            `json:"deleted_by"`
	Version                 int32                    `json:"version"`
	NotifyByEmail           bool                     `json:"notify_by_email"`
	EmailNotificationSentAt sql.NullTime             `json:"email_notification_sent_at"`
	LinkLabel               sql.NullString           `json:"link_label"`
	NotifyBySms             bool                     `json:"notify_by_sms"`
	SmsMessage              sql.NullString           `json:"sms_message"`
	SmsNotificationQueuedAt sql.NullTime             `json:"sms_notification_queued_at"`
}

type AnnouncementAttachment struct {
	ID               uuid.UUID      `json:"id"`
	AnnouncementID   uuid.UUID      `json:"announcement_id"`
	FileName         string         `json:"file_name"`
	OriginalFileName string         `json:"original_file_name"`
	ContentType      sql.NullString `json:"content_type"`
	FileSize         int64          `json:"file_size"`
	StorageProvider  string         `json:"storage_provider"`
	StorageKey       string         `json:"storage_key"`
	Checksum         sql.NullString `json:"checksum"`
	UploadedBy       uuid.NullUUID  `json:"uploaded_by"`
	IncludeInEmail   bool           `json:"include_in_email"`
	Inline           bool           `json:"inline"`
	ContentID        sql.NullString `json:"content_id"`
	SortOrder        int32          `json:"sort_order"`
	CreatedAt        time.Time      `json:"created_at"`
	DeletedAt        sql.NullTime   `json:"deleted_at"`
	DeletedBy        uuid.NullUUID  `json:"deleted_by"`
}

type AnnouncementAudienceType string

type AnnouncementLevel string

type AnnouncementStatus string

func (q *Queries) ArchiveAnnouncement(ctx context.Context, arg ArchiveAnnouncementParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, archiveAnnouncement, arg.ID, arg.ArchivedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type ArchiveAnnouncementParams struct {
	ID         uuid.UUID     `json:"id"`
	ArchivedBy uuid.NullUUID `json:"archived_by"`
}

func (q *Queries) CountActivePublishedAnnouncements(ctx context.Context) (int64, error) {
	row := q.db.QueryRowContext(ctx, countActivePublishedAnnouncements)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

func (q *Queries) CountAnnouncementAttachments(ctx context.Context, announcementID uuid.UUID) (int64, error) {
	row := q.db.QueryRowContext(ctx, countAnnouncementAttachments, announcementID)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

func (q *Queries) CountAnnouncementsAdmin(ctx context.Context) (int64, error) {
	row := q.db.QueryRowContext(ctx, countAnnouncementsAdmin)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

func (q *Queries) CountAnnouncementsByStatus(ctx context.Context, status AnnouncementStatus) (int64, error) {
	row := q.db.QueryRowContext(ctx, countAnnouncementsByStatus, status)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

func (q *Queries) CountAnnouncementsCreatedByUser(ctx context.Context, createdBy uuid.UUID) (int64, error) {
	row := q.db.QueryRowContext(ctx, countAnnouncementsCreatedByUser, createdBy)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

func (q *Queries) CountSearchAnnouncementsAdmin(ctx context.Context, searchText sql.NullString) (int64, error) {
	row := q.db.QueryRowContext(ctx, countSearchAnnouncementsAdmin, searchText)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

func (q *Queries) CreateAnnouncement(ctx context.Context, arg CreateAnnouncementParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, createAnnouncement,
		arg.Title,
		arg.Message,
		arg.Summary,
		arg.Level,
		arg.Tag,
		arg.LinkUrl,
		arg.LinkLabel,
		arg.Priority,
		arg.IsPinned,
		arg.Status,
		arg.PublishAt,
		arg.ExpiresAt,
		arg.AudienceType,
		arg.NotifyByEmail,
		arg.NotifyBySms,
		arg.SmsMessage,
		arg.CreatedBy,
	)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

func (q *Queries) CreateAnnouncementAttachment(ctx context.Context, arg CreateAnnouncementAttachmentParams) (AnnouncementAttachment, error) {
	row := q.db.QueryRowContext(ctx, createAnnouncementAttachment,
		arg.ID,
		arg.AnnouncementID,
		arg.FileName,
		arg.OriginalFileName,
		arg.ContentType,
		arg.FileSize,
		arg.StorageProvider,
		arg.StorageKey,
		arg.Checksum,
		arg.UploadedBy,
		arg.IncludeInEmail,
		arg.Inline,
		arg.ContentID,
		arg.SortOrder,
	)
	var i AnnouncementAttachment
	err := row.Scan(
		&i.ID,
		&i.AnnouncementID,
		&i.FileName,
		&i.OriginalFileName,
		&i.ContentType,
		&i.FileSize,
		&i.StorageProvider,
		&i.StorageKey,
		&i.Checksum,
		&i.UploadedBy,
		&i.IncludeInEmail,
		&i.Inline,
		&i.ContentID,
		&i.SortOrder,
		&i.CreatedAt,
		&i.DeletedAt,
		&i.DeletedBy,
	)
	return i, err
}

type CreateAnnouncementAttachmentParams struct {
	ID               uuid.UUID      `json:"id"`
	AnnouncementID   uuid.UUID      `json:"announcement_id"`
	FileName         string         `json:"file_name"`
	OriginalFileName string         `json:"original_file_name"`
	ContentType      sql.NullString `json:"content_type"`
	FileSize         int64          `json:"file_size"`
	StorageProvider  string         `json:"storage_provider"`
	StorageKey       string         `json:"storage_key"`
	Checksum         sql.NullString `json:"checksum"`
	UploadedBy       uuid.NullUUID  `json:"uploaded_by"`
	IncludeInEmail   bool           `json:"include_in_email"`
	Inline           bool           `json:"inline"`
	ContentID        sql.NullString `json:"content_id"`
	SortOrder        int32          `json:"sort_order"`
}

type CreateAnnouncementParams struct {
	Title         string                   `json:"title"`
	Message       string                   `json:"message"`
	Summary       sql.NullString           `json:"summary"`
	Level         AnnouncementLevel        `json:"level"`
	Tag           sql.NullString           `json:"tag"`
	LinkUrl       sql.NullString           `json:"link_url"`
	LinkLabel     sql.NullString           `json:"link_label"`
	Priority      int32                    `json:"priority"`
	IsPinned      bool                     `json:"is_pinned"`
	Status        AnnouncementStatus       `json:"status"`
	PublishAt     sql.NullTime             `json:"publish_at"`
	ExpiresAt     sql.NullTime             `json:"expires_at"`
	AudienceType  AnnouncementAudienceType `json:"audience_type"`
	NotifyByEmail bool                     `json:"notify_by_email"`
	NotifyBySms   bool                     `json:"notify_by_sms"`
	SmsMessage    sql.NullString           `json:"sms_message"`
	CreatedBy     uuid.UUID                `json:"created_by"`
}

func (q *Queries) DeleteAnnouncementClients(ctx context.Context, announcementID uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, deleteAnnouncementClients, announcementID)
	return err
}

func (q *Queries) DeleteAnnouncementRoles(ctx context.Context, announcementID uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, deleteAnnouncementRoles, announcementID)
	return err
}

func (q *Queries) DeleteAnnouncementUsers(ctx context.Context, announcementID uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, deleteAnnouncementUsers, announcementID)
	return err
}

func (q *Queries) DraftAnnouncement(ctx context.Context, arg DraftAnnouncementParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, draftAnnouncement, arg.ID, arg.UpdatedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type DraftAnnouncementParams struct {
	ID        uuid.UUID     `json:"id"`
	UpdatedBy uuid.NullUUID `json:"updated_by"`
}

func (q *Queries) GetAnnouncementAttachmentByID(ctx context.Context, arg GetAnnouncementAttachmentByIDParams) (AnnouncementAttachment, error) {
	row := q.db.QueryRowContext(ctx, getAnnouncementAttachmentByID, arg.ID, arg.AnnouncementID)
	var i AnnouncementAttachment
	err := row.Scan(
		&i.ID,
		&i.AnnouncementID,
		&i.FileName,
		&i.OriginalFileName,
		&i.ContentType,
		&i.FileSize,
		&i.StorageProvider,
		&i.StorageKey,
		&i.Checksum,
		&i.UploadedBy,
		&i.IncludeInEmail,
		&i.Inline,
		&i.ContentID,
		&i.SortOrder,
		&i.CreatedAt,
		&i.DeletedAt,
		&i.DeletedBy,
	)
	return i, err
}

type GetAnnouncementAttachmentByIDParams struct {
	ID             uuid.UUID `json:"id"`
	AnnouncementID uuid.UUID `json:"announcement_id"`
}

func (q *Queries) GetAnnouncementByID(ctx context.Context, id uuid.UUID) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, getAnnouncementByID, id)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

func (q *Queries) GetAnnouncementStats(ctx context.Context) (GetAnnouncementStatsRow, error) {
	row := q.db.QueryRowContext(ctx, getAnnouncementStats)
	var i GetAnnouncementStatsRow
	err := row.Scan(
		&i.Total,
		&i.DraftCount,
		&i.ScheduledCount,
		&i.PublishedCount,
		&i.ArchivedCount,
		&i.ActiveCount,
	)
	return i, err
}

type GetAnnouncementStatsRow struct {
	Total          int64 `json:"total"`
	DraftCount     int64 `json:"draft_count"`
	ScheduledCount int64 `json:"scheduled_count"`
	PublishedCount int64 `json:"published_count"`
	ArchivedCount  int64 `json:"archived_count"`
	ActiveCount    int64 `json:"active_count"`
}

func (q *Queries) InsertAnnouncementClient(ctx context.Context, arg InsertAnnouncementClientParams) error {
	_, err := q.db.ExecContext(ctx, insertAnnouncementClient, arg.AnnouncementID, arg.ClientID)
	return err
}

type InsertAnnouncementClientParams struct {
	AnnouncementID uuid.UUID `json:"announcement_id"`
	ClientID       uuid.UUID `json:"client_id"`
}

func (q *Queries) InsertAnnouncementRole(ctx context.Context, arg InsertAnnouncementRoleParams) error {
	_, err := q.db.ExecContext(ctx, insertAnnouncementRole, arg.AnnouncementID, arg.RoleName)
	return err
}

type InsertAnnouncementRoleParams struct {
	AnnouncementID uuid.UUID `json:"announcement_id"`
	RoleName       string    `json:"role_name"`
}

func (q *Queries) InsertAnnouncementUser(ctx context.Context, arg InsertAnnouncementUserParams) error {
	_, err := q.db.ExecContext(ctx, insertAnnouncementUser, arg.AnnouncementID, arg.UserID)
	return err
}

type InsertAnnouncementUserParams struct {
	AnnouncementID uuid.UUID `json:"announcement_id"`
	UserID         uuid.UUID `json:"user_id"`
}

func (q *Queries) ListActivePublishedAnnouncements(ctx context.Context, arg ListActivePublishedAnnouncementsParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listActivePublishedAnnouncements, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListActivePublishedAnnouncementsParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func (q *Queries) ListAnnouncementAttachmentsByAnnouncementID(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementAttachmentsByAnnouncementID, announcementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AnnouncementAttachment{}
	for rows.Next() {
		var i AnnouncementAttachment
		if err := rows.Scan(
			&i.ID,
			&i.AnnouncementID,
			&i.FileName,
			&i.OriginalFileName,
			&i.ContentType,
			&i.FileSize,
			&i.StorageProvider,
			&i.StorageKey,
			&i.Checksum,
			&i.UploadedBy,
			&i.IncludeInEmail,
			&i.Inline,
			&i.ContentID,
			&i.SortOrder,
			&i.CreatedAt,
			&i.DeletedAt,
			&i.DeletedBy,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) ListAnnouncementClients(ctx context.Context, announcementID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementClients, announcementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []uuid.UUID{}
	for rows.Next() {
		var client_id uuid.UUID
		if err := rows.Scan(&client_id); err != nil {
			return nil, err
		}
		items = append(items, client_id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) ListAnnouncementEmailAttachments(ctx context.Context, announcementID uuid.UUID) ([]AnnouncementAttachment, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementEmailAttachments, announcementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AnnouncementAttachment{}
	for rows.Next() {
		var i AnnouncementAttachment
		if err := rows.Scan(
			&i.ID,
			&i.AnnouncementID,
			&i.FileName,
			&i.OriginalFileName,
			&i.ContentType,
			&i.FileSize,
			&i.StorageProvider,
			&i.StorageKey,
			&i.Checksum,
			&i.UploadedBy,
			&i.IncludeInEmail,
			&i.Inline,
			&i.ContentID,
			&i.SortOrder,
			&i.CreatedAt,
			&i.DeletedAt,
			&i.DeletedBy,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) ListAnnouncementRoles(ctx context.Context, announcementID uuid.UUID) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementRoles, announcementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var role_name string
		if err := rows.Scan(&role_name); err != nil {
			return nil, err
		}
		items = append(items, role_name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) ListAnnouncementUsers(ctx context.Context, announcementID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementUsers, announcementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []uuid.UUID{}
	for rows.Next() {
		var user_id uuid.UUID
		if err := rows.Scan(&user_id); err != nil {
			return nil, err
		}
		items = append(items, user_id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) ListAnnouncementsAdmin(ctx context.Context, arg ListAnnouncementsAdminParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementsAdmin, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListAnnouncementsAdminParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func (q *Queries) ListAnnouncementsByStatus(ctx context.Context, arg ListAnnouncementsByStatusParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementsByStatus, arg.Status, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListAnnouncementsByStatusParams struct {
	Status AnnouncementStatus `json:"status"`
	Limit  int32              `json:"limit"`
	Offset int32              `json:"offset"`
}

func (q *Queries) ListAnnouncementsCreatedByUser(ctx context.Context, arg ListAnnouncementsCreatedByUserParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementsCreatedByUser, arg.CreatedBy, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListAnnouncementsCreatedByUserParams struct {
	CreatedBy uuid.UUID `json:"created_by"`
	Limit     int32     `json:"limit"`
	Offset    int32     `json:"offset"`
}

func (q *Queries) ListAnnouncementsForClient(ctx context.Context, arg ListAnnouncementsForClientParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementsForClient, arg.ClientID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListAnnouncementsForClientParams struct {
	ClientID uuid.UUID `json:"client_id"`
	Limit    int32     `json:"limit"`
	Offset   int32     `json:"offset"`
}

func (q *Queries) ListAnnouncementsForRole(ctx context.Context, arg ListAnnouncementsForRoleParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementsForRole, arg.RoleName, arg.PageOffset, arg.PageLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListAnnouncementsForRoleParams struct {
	RoleName   string `json:"role_name"`
	PageOffset int32  `json:"page_offset"`
	PageLimit  int32  `json:"page_limit"`
}

func (q *Queries) ListAnnouncementsForUser(ctx context.Context, arg ListAnnouncementsForUserParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listAnnouncementsForUser, arg.UserID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListAnnouncementsForUserParams struct {
	UserID uuid.UUID `json:"user_id"`
	Limit  int32     `json:"limit"`
	Offset int32     `json:"offset"`
}

func (q *Queries) ListPublicAnnouncements(ctx context.Context, arg ListPublicAnnouncementsParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, listPublicAnnouncements, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type ListPublicAnnouncementsParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func (q *Queries) MarkAnnouncementEmailNotificationSent(ctx context.Context, id uuid.UUID) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, markAnnouncementEmailNotificationSent, id)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

func (q *Queries) MarkAnnouncementSMSNotificationQueued(ctx context.Context, id uuid.UUID) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, markAnnouncementSMSNotificationQueued, id)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

func (q *Queries) ClaimDueScheduledAnnouncements(ctx context.Context, limit int32) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, claimDueScheduledAnnouncements, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) PublishAnnouncementNow(ctx context.Context, arg PublishAnnouncementNowParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, publishAnnouncementNow, arg.ID, arg.PublishedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type PublishAnnouncementNowParams struct {
	ID          uuid.UUID     `json:"id"`
	PublishedBy uuid.NullUUID `json:"published_by"`
}

func (q *Queries) RestoreAnnouncement(ctx context.Context, arg RestoreAnnouncementParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, restoreAnnouncement, arg.ID, arg.UpdatedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type RestoreAnnouncementParams struct {
	ID        uuid.UUID     `json:"id"`
	UpdatedBy uuid.NullUUID `json:"updated_by"`
}

func (q *Queries) ScheduleAnnouncement(ctx context.Context, arg ScheduleAnnouncementParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, scheduleAnnouncement, arg.ID, arg.PublishAt, arg.UpdatedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type ScheduleAnnouncementParams struct {
	ID        uuid.UUID     `json:"id"`
	PublishAt sql.NullTime  `json:"publish_at"`
	UpdatedBy uuid.NullUUID `json:"updated_by"`
}

func (q *Queries) SearchAnnouncementsAdmin(ctx context.Context, arg SearchAnnouncementsAdminParams) ([]Announcement, error) {
	rows, err := q.db.QueryContext(ctx, searchAnnouncementsAdmin, arg.SearchText, arg.PageOffset, arg.PageLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		var i Announcement
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Message,
			&i.Summary,
			&i.Level,
			&i.Tag,
			&i.LinkUrl,
			&i.Priority,
			&i.IsPinned,
			&i.Status,
			&i.PublishAt,
			&i.ExpiresAt,
			&i.AudienceType,
			&i.CreatedBy,
			&i.UpdatedBy,
			&i.PublishedBy,
			&i.ArchivedBy,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PublishedAt,
			&i.ArchivedAt,
			&i.DeletedAt,
			&i.DeletedBy,
			&i.Version,
			&i.NotifyByEmail,
			&i.EmailNotificationSentAt,
			&i.LinkLabel,
			&i.NotifyBySms,
			&i.SmsMessage,
			&i.SmsNotificationQueuedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type SearchAnnouncementsAdminParams struct {
	SearchText sql.NullString `json:"search_text"`
	PageOffset int32          `json:"page_offset"`
	PageLimit  int32          `json:"page_limit"`
}

func (q *Queries) SetAnnouncementPinned(ctx context.Context, arg SetAnnouncementPinnedParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, setAnnouncementPinned, arg.ID, arg.IsPinned, arg.UpdatedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type SetAnnouncementPinnedParams struct {
	ID        uuid.UUID     `json:"id"`
	IsPinned  bool          `json:"is_pinned"`
	UpdatedBy uuid.NullUUID `json:"updated_by"`
}

func (q *Queries) SetAnnouncementPriority(ctx context.Context, arg SetAnnouncementPriorityParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, setAnnouncementPriority, arg.ID, arg.Priority, arg.UpdatedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type SetAnnouncementPriorityParams struct {
	ID        uuid.UUID     `json:"id"`
	Priority  int32         `json:"priority"`
	UpdatedBy uuid.NullUUID `json:"updated_by"`
}

func (q *Queries) SoftDeleteAnnouncement(ctx context.Context, arg SoftDeleteAnnouncementParams) error {
	_, err := q.db.ExecContext(ctx, softDeleteAnnouncement, arg.ID, arg.DeletedBy)
	return err
}

func (q *Queries) SoftDeleteAnnouncementAttachment(ctx context.Context, arg SoftDeleteAnnouncementAttachmentParams) (AnnouncementAttachment, error) {
	row := q.db.QueryRowContext(ctx, softDeleteAnnouncementAttachment, arg.ID, arg.AnnouncementID, arg.DeletedBy)
	var i AnnouncementAttachment
	err := row.Scan(
		&i.ID,
		&i.AnnouncementID,
		&i.FileName,
		&i.OriginalFileName,
		&i.ContentType,
		&i.FileSize,
		&i.StorageProvider,
		&i.StorageKey,
		&i.Checksum,
		&i.UploadedBy,
		&i.IncludeInEmail,
		&i.Inline,
		&i.ContentID,
		&i.SortOrder,
		&i.CreatedAt,
		&i.DeletedAt,
		&i.DeletedBy,
	)
	return i, err
}

type SoftDeleteAnnouncementAttachmentParams struct {
	ID             uuid.UUID     `json:"id"`
	AnnouncementID uuid.UUID     `json:"announcement_id"`
	DeletedBy      uuid.NullUUID `json:"deleted_by"`
}

type SoftDeleteAnnouncementParams struct {
	ID        uuid.UUID     `json:"id"`
	DeletedBy uuid.NullUUID `json:"deleted_by"`
}

func (q *Queries) UnarchiveAnnouncementToDraft(ctx context.Context, arg UnarchiveAnnouncementToDraftParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, unarchiveAnnouncementToDraft, arg.ID, arg.UpdatedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type UnarchiveAnnouncementToDraftParams struct {
	ID        uuid.UUID     `json:"id"`
	UpdatedBy uuid.NullUUID `json:"updated_by"`
}

func (q *Queries) UpdateAnnouncement(ctx context.Context, arg UpdateAnnouncementParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, updateAnnouncement,
		arg.ID,
		arg.Title,
		arg.Message,
		arg.Summary,
		arg.Level,
		arg.Tag,
		arg.LinkUrl,
		arg.LinkLabel,
		arg.Priority,
		arg.IsPinned,
		arg.PublishAt,
		arg.ExpiresAt,
		arg.AudienceType,
		arg.NotifyByEmail,
		arg.NotifyBySms,
		arg.SmsMessage,
		arg.UpdatedBy,
	)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

func (q *Queries) UpdateAnnouncementAttachment(ctx context.Context, arg UpdateAnnouncementAttachmentParams) (AnnouncementAttachment, error) {
	row := q.db.QueryRowContext(ctx, updateAnnouncementAttachment,
		arg.ID,
		arg.AnnouncementID,
		arg.IncludeInEmail,
		arg.Inline,
		arg.ContentID,
		arg.SortOrder,
	)
	var i AnnouncementAttachment
	err := row.Scan(
		&i.ID,
		&i.AnnouncementID,
		&i.FileName,
		&i.OriginalFileName,
		&i.ContentType,
		&i.FileSize,
		&i.StorageProvider,
		&i.StorageKey,
		&i.Checksum,
		&i.UploadedBy,
		&i.IncludeInEmail,
		&i.Inline,
		&i.ContentID,
		&i.SortOrder,
		&i.CreatedAt,
		&i.DeletedAt,
		&i.DeletedBy,
	)
	return i, err
}

type UpdateAnnouncementAttachmentParams struct {
	ID             uuid.UUID      `json:"id"`
	AnnouncementID uuid.UUID      `json:"announcement_id"`
	IncludeInEmail bool           `json:"include_in_email"`
	Inline         bool           `json:"inline"`
	ContentID      sql.NullString `json:"content_id"`
	SortOrder      int32          `json:"sort_order"`
}

type UpdateAnnouncementParams struct {
	ID            uuid.UUID                `json:"id"`
	Title         string                   `json:"title"`
	Message       string                   `json:"message"`
	Summary       sql.NullString           `json:"summary"`
	Level         AnnouncementLevel        `json:"level"`
	Tag           sql.NullString           `json:"tag"`
	LinkUrl       sql.NullString           `json:"link_url"`
	LinkLabel     sql.NullString           `json:"link_label"`
	Priority      int32                    `json:"priority"`
	IsPinned      bool                     `json:"is_pinned"`
	PublishAt     sql.NullTime             `json:"publish_at"`
	ExpiresAt     sql.NullTime             `json:"expires_at"`
	AudienceType  AnnouncementAudienceType `json:"audience_type"`
	NotifyByEmail bool                     `json:"notify_by_email"`
	NotifyBySms   bool                     `json:"notify_by_sms"`
	SmsMessage    sql.NullString           `json:"sms_message"`
	UpdatedBy     uuid.NullUUID            `json:"updated_by"`
}

func (q *Queries) UpdateAnnouncementStatus(ctx context.Context, arg UpdateAnnouncementStatusParams) (Announcement, error) {
	row := q.db.QueryRowContext(ctx, updateAnnouncementStatus, arg.ID, arg.Status, arg.UpdatedBy)
	var i Announcement
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Message,
		&i.Summary,
		&i.Level,
		&i.Tag,
		&i.LinkUrl,
		&i.Priority,
		&i.IsPinned,
		&i.Status,
		&i.PublishAt,
		&i.ExpiresAt,
		&i.AudienceType,
		&i.CreatedBy,
		&i.UpdatedBy,
		&i.PublishedBy,
		&i.ArchivedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublishedAt,
		&i.ArchivedAt,
		&i.DeletedAt,
		&i.DeletedBy,
		&i.Version,
		&i.NotifyByEmail,
		&i.EmailNotificationSentAt,
		&i.LinkLabel,
		&i.NotifyBySms,
		&i.SmsMessage,
		&i.SmsNotificationQueuedAt,
	)
	return i, err
}

type UpdateAnnouncementStatusParams struct {
	ID        uuid.UUID          `json:"id"`
	Status    AnnouncementStatus `json:"status"`
	UpdatedBy uuid.NullUUID      `json:"updated_by"`
}

const archiveAnnouncement = `-- name: ArchiveAnnouncement :one
UPDATE announcements
SET
    status = 'ARCHIVED',
    archived_at = now(),
    archived_by = $2,
    updated_by = $2
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const countActivePublishedAnnouncements = `-- name: CountActivePublishedAnnouncements :one
SELECT COUNT(*)::bigint
FROM announcements
WHERE deleted_at IS NULL
  AND status = 'PUBLISHED'
  AND (publish_at IS NULL OR publish_at <= now())
  AND (expires_at IS NULL OR expires_at > now())
`

const countAnnouncementAttachments = `-- name: CountAnnouncementAttachments :one
SELECT COUNT(*)::bigint
FROM announcement_attachments
WHERE announcement_id = $1
  AND deleted_at IS NULL
`

const countAnnouncementsAdmin = `-- name: CountAnnouncementsAdmin :one
SELECT COUNT(*)::bigint
FROM announcements
WHERE deleted_at IS NULL
`

const countAnnouncementsByStatus = `-- name: CountAnnouncementsByStatus :one
SELECT COUNT(*)::bigint
FROM announcements
WHERE deleted_at IS NULL
  AND status = $1
`

const countAnnouncementsCreatedByUser = `-- name: CountAnnouncementsCreatedByUser :one
SELECT COUNT(*)::bigint
FROM announcements
WHERE deleted_at IS NULL
  AND created_by = $1
`

const countSearchAnnouncementsAdmin = `-- name: CountSearchAnnouncementsAdmin :one
SELECT COUNT(*)::bigint
FROM announcements
WHERE deleted_at IS NULL
  AND (
      title ILIKE '%' || $1 || '%'
      OR message ILIKE '%' || $1 || '%'
      OR COALESCE(summary, '') ILIKE '%' || $1 || '%'
      OR COALESCE(tag, '') ILIKE '%' || $1 || '%'
  )
`

const createAnnouncement = `-- name: CreateAnnouncement :one
INSERT INTO announcements (
    title,
    message,
    summary,
    level,
    tag,
    link_url,
    link_label,
    priority,
    is_pinned,
    status,
    publish_at,
    expires_at,
    audience_type,
    notify_by_email,
    notify_by_sms,
    sms_message,
    created_by,
    updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $17
)
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const createAnnouncementAttachment = `-- name: CreateAnnouncementAttachment :one
INSERT INTO announcement_attachments (
    id,
    announcement_id,
    file_name,
    original_file_name,
    content_type,
    file_size,
    storage_provider,
    storage_key,
    checksum,
    uploaded_by,
    include_in_email,
    inline,
    content_id,
    sort_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING id, announcement_id, file_name, original_file_name, content_type, file_size, storage_provider, storage_key, checksum, uploaded_by, include_in_email, inline, content_id, sort_order, created_at, deleted_at, deleted_by
`

const deleteAnnouncementClients = `-- name: DeleteAnnouncementClients :exec
DELETE FROM announcement_clients
WHERE announcement_id = $1
`

const deleteAnnouncementRoles = `-- name: DeleteAnnouncementRoles :exec
DELETE FROM announcement_roles
WHERE announcement_id = $1
`

const deleteAnnouncementUsers = `-- name: DeleteAnnouncementUsers :exec
DELETE FROM announcement_users
WHERE announcement_id = $1
`

const draftAnnouncement = `-- name: DraftAnnouncement :one
UPDATE announcements
SET
    status = 'DRAFT',
    updated_by = $2
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const getAnnouncementAttachmentByID = `-- name: GetAnnouncementAttachmentByID :one
SELECT id, announcement_id, file_name, original_file_name, content_type, file_size, storage_provider, storage_key, checksum, uploaded_by, include_in_email, inline, content_id, sort_order, created_at, deleted_at, deleted_by
FROM announcement_attachments
WHERE id = $1
  AND announcement_id = $2
  AND deleted_at IS NULL
LIMIT 1
`

const getAnnouncementByID = `-- name: GetAnnouncementByID :one
SELECT id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
FROM announcements
WHERE id = $1
  AND deleted_at IS NULL
LIMIT 1
`

const getAnnouncementStats = `-- name: GetAnnouncementStats :one
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE status = 'DRAFT')::bigint AS draft_count,
    COUNT(*) FILTER (WHERE status = 'SCHEDULED')::bigint AS scheduled_count,
    COUNT(*) FILTER (WHERE status = 'PUBLISHED')::bigint AS published_count,
    COUNT(*) FILTER (WHERE status = 'ARCHIVED')::bigint AS archived_count,
    COUNT(*) FILTER (
        WHERE status = 'PUBLISHED'
          AND (publish_at IS NULL OR publish_at <= now())
          AND (expires_at IS NULL OR expires_at > now())
    )::bigint AS active_count
FROM announcements
WHERE deleted_at IS NULL
`

const insertAnnouncementClient = `-- name: InsertAnnouncementClient :exec
INSERT INTO announcement_clients (
    announcement_id,
    client_id
) VALUES ($1, $2)
ON CONFLICT (announcement_id, client_id) DO NOTHING
`

const insertAnnouncementRole = `-- name: InsertAnnouncementRole :exec
INSERT INTO announcement_roles (
    announcement_id,
    role_name
) VALUES ($1, $2)
ON CONFLICT (announcement_id, role_name) DO NOTHING
`

const insertAnnouncementUser = `-- name: InsertAnnouncementUser :exec
INSERT INTO announcement_users (
    announcement_id,
    user_id
) VALUES ($1, $2)
ON CONFLICT (announcement_id, user_id) DO NOTHING
`

const listActivePublishedAnnouncements = `-- name: ListActivePublishedAnnouncements :many
SELECT id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
FROM announcements
WHERE deleted_at IS NULL
  AND status = 'PUBLISHED'
  AND (publish_at IS NULL OR publish_at <= now())
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY is_pinned DESC, priority DESC, publish_at DESC NULLS LAST, created_at DESC
LIMIT $1 OFFSET $2
`

const listAnnouncementAttachmentsByAnnouncementID = `-- name: ListAnnouncementAttachmentsByAnnouncementID :many
SELECT id, announcement_id, file_name, original_file_name, content_type, file_size, storage_provider, storage_key, checksum, uploaded_by, include_in_email, inline, content_id, sort_order, created_at, deleted_at, deleted_by
FROM announcement_attachments
WHERE announcement_id = $1
  AND deleted_at IS NULL
ORDER BY sort_order ASC, created_at ASC
`

const listAnnouncementClients = `-- name: ListAnnouncementClients :many
SELECT client_id
FROM announcement_clients
WHERE announcement_id = $1
ORDER BY client_id
`

const listAnnouncementEmailAttachments = `-- name: ListAnnouncementEmailAttachments :many
SELECT id, announcement_id, file_name, original_file_name, content_type, file_size, storage_provider, storage_key, checksum, uploaded_by, include_in_email, inline, content_id, sort_order, created_at, deleted_at, deleted_by
FROM announcement_attachments
WHERE announcement_id = $1
  AND deleted_at IS NULL
  AND include_in_email = TRUE
ORDER BY sort_order ASC, created_at ASC
`

const listAnnouncementRoles = `-- name: ListAnnouncementRoles :many
SELECT role_name
FROM announcement_roles
WHERE announcement_id = $1
ORDER BY role_name
`

const listAnnouncementUsers = `-- name: ListAnnouncementUsers :many
SELECT user_id
FROM announcement_users
WHERE announcement_id = $1
ORDER BY user_id
`

const listAnnouncementsAdmin = `-- name: ListAnnouncementsAdmin :many
SELECT id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
FROM announcements
WHERE deleted_at IS NULL
ORDER BY is_pinned DESC, priority DESC, created_at DESC
LIMIT $1 OFFSET $2
`

const listAnnouncementsByStatus = `-- name: ListAnnouncementsByStatus :many
SELECT id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
FROM announcements
WHERE deleted_at IS NULL
  AND status = $1
ORDER BY is_pinned DESC, priority DESC, created_at DESC
LIMIT $2 OFFSET $3
`

const listAnnouncementsCreatedByUser = `-- name: ListAnnouncementsCreatedByUser :many
SELECT id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
FROM announcements
WHERE deleted_at IS NULL
  AND created_by = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

const listAnnouncementsForClient = `-- name: ListAnnouncementsForClient :many
SELECT DISTINCT a.id, a.title, a.message, a.summary, a.level, a.tag, a.link_url, a.priority, a.is_pinned, a.status, a.publish_at, a.expires_at, a.audience_type, a.created_by, a.updated_by, a.published_by, a.archived_by, a.created_at, a.updated_at, a.published_at, a.archived_at, a.deleted_at, a.deleted_by, a.version, a.notify_by_email, a.email_notification_sent_at, a.link_label, a.notify_by_sms, a.sms_message, a.sms_notification_queued_at
FROM announcements a
LEFT JOIN announcement_clients ac
    ON ac.announcement_id = a.id
WHERE a.deleted_at IS NULL
  AND a.status = 'PUBLISHED'
  AND (a.publish_at IS NULL OR a.publish_at <= now())
  AND (a.expires_at IS NULL OR a.expires_at > now())
  AND (
      a.audience_type = 'ALL_USERS'
      OR (a.audience_type = 'SPECIFIC_CLIENTS' AND ac.client_id = $1)
  )
ORDER BY a.is_pinned DESC, a.priority DESC, a.publish_at DESC NULLS LAST, a.created_at DESC
LIMIT $2 OFFSET $3
`

const listAnnouncementsForRole = `-- name: ListAnnouncementsForRole :many
SELECT DISTINCT a.id, a.title, a.message, a.summary, a.level, a.tag, a.link_url, a.priority, a.is_pinned, a.status, a.publish_at, a.expires_at, a.audience_type, a.created_by, a.updated_by, a.published_by, a.archived_by, a.created_at, a.updated_at, a.published_at, a.archived_at, a.deleted_at, a.deleted_by, a.version, a.notify_by_email, a.email_notification_sent_at, a.link_label, a.notify_by_sms, a.sms_message, a.sms_notification_queued_at
FROM announcements a
LEFT JOIN announcement_roles ar
    ON ar.announcement_id = a.id
WHERE a.deleted_at IS NULL
  AND a.status = 'PUBLISHED'
  AND (a.publish_at IS NULL OR a.publish_at <= now())
  AND (a.expires_at IS NULL OR a.expires_at > now())
  AND (
      a.audience_type = 'ALL_USERS'
      OR (a.audience_type = 'ADMINS_ONLY' AND LOWER($1::text) = 'admin')
      OR (a.audience_type = 'SPECIFIC_ROLES' AND LOWER(ar.role_name) = LOWER($1::text))
  )
ORDER BY a.is_pinned DESC, a.priority DESC, a.publish_at DESC NULLS LAST, a.created_at DESC
LIMIT $3 OFFSET $2
`

const listAnnouncementsForUser = `-- name: ListAnnouncementsForUser :many
SELECT DISTINCT a.id, a.title, a.message, a.summary, a.level, a.tag, a.link_url, a.priority, a.is_pinned, a.status, a.publish_at, a.expires_at, a.audience_type, a.created_by, a.updated_by, a.published_by, a.archived_by, a.created_at, a.updated_at, a.published_at, a.archived_at, a.deleted_at, a.deleted_by, a.version, a.notify_by_email, a.email_notification_sent_at, a.link_label, a.notify_by_sms, a.sms_message, a.sms_notification_queued_at
FROM announcements a
LEFT JOIN announcement_users au
    ON au.announcement_id = a.id
WHERE a.deleted_at IS NULL
  AND a.status = 'PUBLISHED'
  AND (a.publish_at IS NULL OR a.publish_at <= now())
  AND (a.expires_at IS NULL OR a.expires_at > now())
  AND (
      a.audience_type = 'ALL_USERS'
      OR (a.audience_type = 'SPECIFIC_USERS' AND au.user_id = $1)
  )
ORDER BY a.is_pinned DESC, a.priority DESC, a.publish_at DESC NULLS LAST, a.created_at DESC
LIMIT $2 OFFSET $3
`

const listPublicAnnouncements = `-- name: ListPublicAnnouncements :many
SELECT id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
FROM announcements
WHERE deleted_at IS NULL
  AND status = 'PUBLISHED'
  AND (publish_at IS NULL OR publish_at <= now())
  AND (expires_at IS NULL OR expires_at > now())
  AND audience_type = 'ALL_USERS'
ORDER BY is_pinned DESC, priority DESC, publish_at DESC NULLS LAST, created_at DESC
LIMIT $1 OFFSET $2
`

const markAnnouncementEmailNotificationSent = `-- name: MarkAnnouncementEmailNotificationSent :one
UPDATE announcements
SET email_notification_sent_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const markAnnouncementSMSNotificationQueued = `-- name: MarkAnnouncementSMSNotificationQueued :one
UPDATE announcements
SET sms_notification_queued_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const claimDueScheduledAnnouncements = `-- name: ClaimDueScheduledAnnouncements :many
UPDATE announcements
SET
    status = 'PUBLISHED',
    published_at = now(),
    published_by = COALESCE(updated_by, created_by)
WHERE id IN (
    SELECT id
    FROM announcements
    WHERE deleted_at IS NULL
      AND status = 'SCHEDULED'
      AND publish_at IS NOT NULL
      AND publish_at <= now()
      AND (expires_at IS NULL OR expires_at > now())
    ORDER BY publish_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
  AND status = 'SCHEDULED'
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const publishAnnouncementNow = `-- name: PublishAnnouncementNow :one
UPDATE announcements
SET
    status = 'PUBLISHED',
    publish_at = LEAST(COALESCE(publish_at, now()), now()),
    published_at = now(),
    published_by = $2,
    updated_by = $2
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const restoreAnnouncement = `-- name: RestoreAnnouncement :one
UPDATE announcements
SET
    deleted_at = NULL,
    deleted_by = NULL,
    updated_by = $2
WHERE id = $1
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const scheduleAnnouncement = `-- name: ScheduleAnnouncement :one
UPDATE announcements
SET
    status = 'SCHEDULED',
    publish_at = $2,
    updated_by = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const searchAnnouncementsAdmin = `-- name: SearchAnnouncementsAdmin :many
SELECT id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
FROM announcements
WHERE deleted_at IS NULL
  AND (
      title ILIKE '%' || $1 || '%'
      OR message ILIKE '%' || $1 || '%'
      OR COALESCE(summary, '') ILIKE '%' || $1 || '%'
      OR COALESCE(tag, '') ILIKE '%' || $1 || '%'
  )
ORDER BY is_pinned DESC, priority DESC, created_at DESC
LIMIT $3 OFFSET $2
`

const setAnnouncementPinned = `-- name: SetAnnouncementPinned :one
UPDATE announcements
SET
    is_pinned = $2,
    updated_by = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const setAnnouncementPriority = `-- name: SetAnnouncementPriority :one
UPDATE announcements
SET
    priority = $2,
    updated_by = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const softDeleteAnnouncement = `-- name: SoftDeleteAnnouncement :exec
UPDATE announcements
SET
    deleted_at = now(),
    deleted_by = $2,
    updated_by = $2
WHERE id = $1
  AND deleted_at IS NULL
`

const softDeleteAnnouncementAttachment = `-- name: SoftDeleteAnnouncementAttachment :one
UPDATE announcement_attachments
SET
    deleted_at = now(),
    deleted_by = $3
WHERE id = $1
  AND announcement_id = $2
  AND deleted_at IS NULL
RETURNING id, announcement_id, file_name, original_file_name, content_type, file_size, storage_provider, storage_key, checksum, uploaded_by, include_in_email, inline, content_id, sort_order, created_at, deleted_at, deleted_by
`

const unarchiveAnnouncementToDraft = `-- name: UnarchiveAnnouncementToDraft :one
UPDATE announcements
SET
    status = 'DRAFT',
    archived_at = NULL,
    archived_by = NULL,
    updated_by = $2
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const updateAnnouncement = `-- name: UpdateAnnouncement :one
UPDATE announcements
SET
    title = $2,
    message = $3,
    summary = $4,
    level = $5,
    tag = $6,
    link_url = $7,
    link_label = $8,
    priority = $9,
    is_pinned = $10,
    publish_at = $11,
    expires_at = $12,
    audience_type = $13,
    notify_by_email = $14,
    notify_by_sms = $15,
    sms_message = $16,
    updated_by = $17
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`

const updateAnnouncementAttachment = `-- name: UpdateAnnouncementAttachment :one
UPDATE announcement_attachments
SET
    include_in_email = $3,
    inline = $4,
    content_id = $5,
    sort_order = $6
WHERE id = $1
  AND announcement_id = $2
  AND deleted_at IS NULL
RETURNING id, announcement_id, file_name, original_file_name, content_type, file_size, storage_provider, storage_key, checksum, uploaded_by, include_in_email, inline, content_id, sort_order, created_at, deleted_at, deleted_by
`

const updateAnnouncementStatus = `-- name: UpdateAnnouncementStatus :one
UPDATE announcements
SET
    status = $2,
    updated_by = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, message, summary, level, tag, link_url, priority, is_pinned, status, publish_at, expires_at, audience_type, created_by, updated_by, published_by, archived_by, created_at, updated_at, published_at, archived_at, deleted_at, deleted_by, version, notify_by_email, email_notification_sent_at, link_label, notify_by_sms, sms_message, sms_notification_queued_at
`
