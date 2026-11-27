package announcements

import (
	"time"
)

type createAnnouncementRequest struct {
	Title            string   `json:"title" binding:"required"`
	Message          string   `json:"message" binding:"required"`
	Summary          *string  `json:"summary"`
	Level            string   `json:"level" binding:"required"`
	Tag              *string  `json:"tag"`
	LinkURL          *string  `json:"link_url"`
	LinkLabel        *string  `json:"link_label"`
	Priority         int32    `json:"priority"`
	IsPinned         bool     `json:"is_pinned"`
	Status           string   `json:"status"`
	PublishAt        *string  `json:"publish_at"`
	ExpiresAt        *string  `json:"expires_at"`
	AudienceType     string   `json:"audience_type"`
	ClientIDs        []string `json:"client_ids"`
	RoleNames        []string `json:"role_names"`
	UserIDs          []string `json:"user_ids"`
	GroupIDs         []string `json:"group_ids"`
	HealthContextIDs []string `json:"health_context_ids"`
	NotifyByEmail    bool     `json:"notify_by_email"`
	NotifyBySMS      bool     `json:"notify_by_sms"`
	SMSMessage       *string  `json:"sms_message"`
}

type updateAnnouncementRequest struct {
	Title            string   `json:"title" binding:"required"`
	Message          string   `json:"message" binding:"required"`
	Summary          *string  `json:"summary"`
	Level            string   `json:"level" binding:"required"`
	Tag              *string  `json:"tag"`
	LinkURL          *string  `json:"link_url"`
	LinkLabel        *string  `json:"link_label"`
	Priority         int32    `json:"priority"`
	IsPinned         bool     `json:"is_pinned"`
	PublishAt        *string  `json:"publish_at"`
	ExpiresAt        *string  `json:"expires_at"`
	AudienceType     string   `json:"audience_type"`
	ClientIDs        []string `json:"client_ids"`
	RoleNames        []string `json:"role_names"`
	UserIDs          []string `json:"user_ids"`
	GroupIDs         []string `json:"group_ids"`
	HealthContextIDs []string `json:"health_context_ids"`
	NotifyByEmail    bool     `json:"notify_by_email"`
	NotifyBySMS      bool     `json:"notify_by_sms"`
	SMSMessage       *string  `json:"sms_message"`
	// Status is optional; when present and different from the stored status
	// it is applied through the publish/schedule/draft/archive lifecycle.
	Status *string `json:"status"`
}

type AnnouncementResponse struct {
	ID                        string                           `json:"id"`
	Title                     string                           `json:"title"`
	Message                   string                           `json:"message"`
	Summary                   *string                          `json:"summary,omitempty"`
	Level                     string                           `json:"level"`
	Tag                       *string                          `json:"tag,omitempty"`
	LinkURL                   *string                          `json:"link_url,omitempty"`
	LinkLabel                 *string                          `json:"link_label,omitempty"`
	Priority                  int32                            `json:"priority"`
	IsPinned                  bool                             `json:"is_pinned"`
	Status                    string                           `json:"status"`
	PublishAt                 *time.Time                       `json:"publish_at,omitempty"`
	ExpiresAt                 *time.Time                       `json:"expires_at,omitempty"`
	AudienceType              string                           `json:"audience_type"`
	ClientIDs                 []string                         `json:"client_ids,omitempty"`
	RoleNames                 []string                         `json:"role_names,omitempty"`
	UserIDs                   []string                         `json:"user_ids,omitempty"`
	GroupIDs                  []string                         `json:"group_ids,omitempty"`
	HealthContextIDs          []string                         `json:"health_context_ids,omitempty"`
	IncludeContextDescendants bool                             `json:"include_context_descendants,omitempty"`
	NotifyByEmail             bool                             `json:"notify_by_email"`
	EmailNotificationSentAt   *time.Time                       `json:"email_notification_sent_at,omitempty"`
	NotifyBySMS               bool                             `json:"notify_by_sms"`
	SMSMessage                *string                          `json:"sms_message,omitempty"`
	SMSNotificationQueuedAt   *time.Time                       `json:"sms_notification_queued_at,omitempty"`
	CreatedBy                 string                           `json:"created_by"`
	UpdatedBy                 string                           `json:"updated_by"`
	PublishedBy               *string                          `json:"published_by,omitempty"`
	ArchivedBy                *string                          `json:"archived_by,omitempty"`
	CreatedAt                 time.Time                        `json:"created_at"`
	UpdatedAt                 time.Time                        `json:"updated_at"`
	PublishedAt               *time.Time                       `json:"published_at,omitempty"`
	ArchivedAt                *time.Time                       `json:"archived_at,omitempty"`
	DeletedAt                 *time.Time                       `json:"deleted_at,omitempty"`
	DeletedBy                 *string                          `json:"deleted_by,omitempty"`
	Version                   int32                            `json:"version"`
	Attachments               []AnnouncementAttachmentResponse `json:"attachments,omitempty"`
	AttachmentCount           int                              `json:"attachment_count"`
}

type AnnouncementAttachmentResponse struct {
	ID               string     `json:"id"`
	AnnouncementID   string     `json:"announcement_id"`
	FileName         string     `json:"file_name"`
	OriginalFileName string     `json:"original_file_name"`
	ContentType      *string    `json:"content_type,omitempty"`
	FileSize         int64      `json:"file_size"`
	StorageProvider  string     `json:"storage_provider"`
	Checksum         *string    `json:"checksum,omitempty"`
	UploadedBy       *string    `json:"uploaded_by,omitempty"`
	IncludeInEmail   bool       `json:"include_in_email"`
	Inline           bool       `json:"inline"`
	ContentID        *string    `json:"content_id,omitempty"`
	SortOrder        int32      `json:"sort_order"`
	CreatedAt        time.Time  `json:"created_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	DeletedBy        *string    `json:"deleted_by,omitempty"`
	DownloadURL      string     `json:"download_url,omitempty"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type AnnouncementStatsResponse struct {
	Total          int64 `json:"total"`
	DraftCount     int64 `json:"draft_count"`
	ScheduledCount int64 `json:"scheduled_count"`
	PublishedCount int64 `json:"published_count"`
	ArchivedCount  int64 `json:"archived_count"`
	ActiveCount    int64 `json:"active_count"`
}

type createAnnouncementAttachmentRequest struct {
	FileName       string `json:"file_name" binding:"required"`
	ContentType    string `json:"content_type,omitempty"`
	DataBase64     string `json:"data_base64" binding:"required"`
	IncludeInEmail *bool  `json:"include_in_email,omitempty"`
	Inline         bool   `json:"inline,omitempty"`
	ContentID      string `json:"content_id,omitempty"`
	SortOrder      int32  `json:"sort_order,omitempty"`
}

type updateAnnouncementAttachmentRequest struct {
	IncludeInEmail *bool   `json:"include_in_email,omitempty"`
	Inline         *bool   `json:"inline,omitempty"`
	ContentID      *string `json:"content_id,omitempty"`
	SortOrder      *int32  `json:"sort_order,omitempty"`
}

type scheduleAnnouncementRequest struct {
	PublishAt                 string                          `json:"publish_at" binding:"required"`
	Attachments               []announcementAttachmentRequest `json:"attachments,omitempty"`
	IncludeAttachmentsInEmail *bool                           `json:"include_attachments_in_email,omitempty"`
}

type publishAnnouncementRequest struct {
	Attachments               []announcementAttachmentRequest `json:"attachments,omitempty"`
	IncludeAttachmentsInEmail *bool                           `json:"include_attachments_in_email,omitempty"`
}

type announcementAttachmentRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type,omitempty"`
	// Path is accepted only so it can be rejected explicitly; server file
	// paths are never read for announcement email attachments.
	Path       string `json:"path,omitempty"`
	DataBase64 string `json:"data_base64,omitempty"`
	ContentID  string `json:"content_id,omitempty"`
	Inline     bool   `json:"inline,omitempty"`
}

type setPinnedRequest struct {
	IsPinned bool `json:"is_pinned"`
}

type setPriorityRequest struct {
	Priority int32 `json:"priority"`
}

type AnnouncementAudiencePreviewResponse struct {
	RecipientCount int `json:"recipient_count"`
}

type announcementAudiencePreviewRequest struct {
	AudienceType     string   `json:"audience_type" binding:"required"`
	HealthContextIDs []string `json:"health_context_ids"`
}
