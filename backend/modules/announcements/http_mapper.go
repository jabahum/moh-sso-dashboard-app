package announcements

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

type announcement = Announcement

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func nullTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

func toAnnouncementResponse(a Announcement) AnnouncementResponse {
	return AnnouncementResponse{
		ID:                      a.ID.String(),
		Title:                   a.Title,
		Message:                 a.Message,
		Summary:                 nullStringPtr(a.Summary),
		Level:                   normalizeLevel(AnnouncementLevel(a.Level)),
		Tag:                     nullStringPtr(a.Tag),
		LinkURL:                 nullStringPtr(a.LinkUrl),
		LinkLabel:               nullStringPtr(a.LinkLabel),
		Priority:                a.Priority,
		IsPinned:                a.IsPinned,
		Status:                  normalizeAnnouncementStatus(AnnouncementStatus(a.Status)),
		PublishAt:               nullTimePtr(a.PublishAt),
		ExpiresAt:               nullTimePtr(a.ExpiresAt),
		AudienceType:            normalizeAudienceType(AnnouncementAudienceType(a.AudienceType)),
		NotifyByEmail:           a.NotifyByEmail,
		EmailNotificationSentAt: nullTimePtr(a.EmailNotificationSentAt),
		NotifyBySMS:             a.NotifyBySms,
		SMSMessage:              nullStringPtr(a.SmsMessage),
		SMSNotificationQueuedAt: nullTimePtr(a.SmsNotificationQueuedAt),
		CreatedBy:               a.CreatedBy.String(),
		UpdatedBy:               nullUUIDString(a.UpdatedBy),
		PublishedBy:             nullableUUID(a.PublishedBy),
		ArchivedBy:              nullableUUID(a.ArchivedBy),
		CreatedAt:               a.CreatedAt,
		UpdatedAt:               a.UpdatedAt,
		PublishedAt:             nullTimePtr(a.PublishedAt),
		ArchivedAt:              nullTimePtr(a.ArchivedAt),
		DeletedAt:               nullTimePtr(a.DeletedAt),
		DeletedBy:               nullableUUID(a.DeletedBy),
		Version:                 a.Version,
	}
}

func toAnnouncementResponseWithAttachments(
	a Announcement,
	attachments []AnnouncementAttachment,
) AnnouncementResponse {
	return toAnnouncementResponseWithAttachmentBase(a, attachments, "/api/v1/admin/announcements")
}

func toUserAnnouncementResponseWithAttachments(
	a Announcement,
	attachments []AnnouncementAttachment,
) AnnouncementResponse {
	return toAnnouncementResponseWithAttachmentBase(a, attachments, "/api/v1/announcements")
}

func toAnnouncementResponseWithAttachmentBase(
	a Announcement,
	attachments []AnnouncementAttachment,
	downloadBasePath string,
) AnnouncementResponse {
	res := toAnnouncementResponse(a)
	res.Attachments = toAnnouncementAttachmentResponses(a.ID, attachments, downloadBasePath)
	res.AttachmentCount = len(res.Attachments)
	return res
}

func withAnnouncementAudience(
	res AnnouncementResponse,
	clientIDs []uuid.UUID,
	roleNames []string,
	userIDs []uuid.UUID,
	groupIDs []uuid.UUID,
	contextAudience []AnnouncementHealthContextAudience,
) AnnouncementResponse {
	res.ClientIDs = uuidStrings(clientIDs)
	res.RoleNames = roleNames
	res.UserIDs = uuidStrings(userIDs)
	res.GroupIDs = uuidStrings(groupIDs)
	res.HealthContextIDs = make([]string, 0, len(contextAudience))
	for _, context := range contextAudience {
		res.HealthContextIDs = append(res.HealthContextIDs, context.ContextNodeID.String())
		if context.IncludeDescendants {
			res.IncludeContextDescendants = true
		}
	}
	return res
}

func uuidStrings(values []uuid.UUID) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != uuid.Nil {
			out = append(out, value.String())
		}
	}

	return out
}

func toAnnouncementAttachmentResponse(
	announcementID uuid.UUID,
	attachment AnnouncementAttachment,
) AnnouncementAttachmentResponse {
	return toAnnouncementAttachmentResponseWithDownloadBase(announcementID, attachment, "/api/v1/admin/announcements")
}

func toUserAnnouncementAttachmentResponse(
	announcementID uuid.UUID,
	attachment AnnouncementAttachment,
) AnnouncementAttachmentResponse {
	return toAnnouncementAttachmentResponseWithDownloadBase(announcementID, attachment, "/api/v1/announcements")
}

func toAnnouncementAttachmentResponseWithDownloadBase(
	announcementID uuid.UUID,
	attachment AnnouncementAttachment,
	downloadBasePath string,
) AnnouncementAttachmentResponse {
	downloadBasePath = strings.TrimRight(strings.TrimSpace(downloadBasePath), "/")
	if downloadBasePath == "" {
		downloadBasePath = "/api/v1/admin/announcements"
	}

	return AnnouncementAttachmentResponse{
		ID:               attachment.ID.String(),
		AnnouncementID:   attachment.AnnouncementID.String(),
		FileName:         attachment.FileName,
		OriginalFileName: attachment.OriginalFileName,
		ContentType:      nullStringPtr(attachment.ContentType),
		FileSize:         attachment.FileSize,
		StorageProvider:  attachment.StorageProvider,
		Checksum:         nullStringPtr(attachment.Checksum),
		UploadedBy:       nullableUUID(attachment.UploadedBy),
		IncludeInEmail:   attachment.IncludeInEmail,
		Inline:           attachment.Inline,
		ContentID:        nullStringPtr(attachment.ContentID),
		SortOrder:        attachment.SortOrder,
		CreatedAt:        attachment.CreatedAt,
		DeletedAt:        nullTimePtr(attachment.DeletedAt),
		DeletedBy:        nullableUUID(attachment.DeletedBy),
		DownloadURL:      downloadBasePath + "/" + announcementID.String() + "/attachments/" + attachment.ID.String() + "/download",
	}
}

func toAnnouncementAttachmentResponses(
	announcementID uuid.UUID,
	attachments []AnnouncementAttachment,
	downloadBasePath ...string,
) []AnnouncementAttachmentResponse {
	basePath := "/api/v1/admin/announcements"
	if len(downloadBasePath) > 0 && strings.TrimSpace(downloadBasePath[0]) != "" {
		basePath = downloadBasePath[0]
	}

	out := make([]AnnouncementAttachmentResponse, 0, len(attachments))
	for _, attachment := range attachments {
		out = append(out, toAnnouncementAttachmentResponseWithDownloadBase(announcementID, attachment, basePath))
	}
	return out
}

func toAnnouncementStatsResponse(stats GetAnnouncementStatsRow) AnnouncementStatsResponse {
	return AnnouncementStatsResponse{
		Total:          stats.Total,
		DraftCount:     stats.DraftCount,
		ScheduledCount: stats.ScheduledCount,
		PublishedCount: stats.PublishedCount,
		ArchivedCount:  stats.ArchivedCount,
		ActiveCount:    stats.ActiveCount,
	}
}

func mapAnnouncementAttachments(in []announcementAttachmentRequest) []Attachment {
	out := make([]Attachment, 0, len(in))
	for _, attachment := range in {
		out = append(out, Attachment{
			FileName:    strings.TrimSpace(attachment.FileName),
			ContentType: strings.TrimSpace(attachment.ContentType),
			DataBase64:  strings.TrimSpace(attachment.DataBase64),
			ContentID:   strings.TrimSpace(attachment.ContentID),
			Inline:      attachment.Inline,
		})
	}
	return out
}

func announcementEmailOptionsFromPublishRequest(req publishAnnouncementRequest) AnnouncementEmailOptions {
	return AnnouncementEmailOptions{
		Attachments:               mapAnnouncementAttachments(req.Attachments),
		IncludeAttachmentsInEmail: includeAnnouncementAttachments(req.IncludeAttachmentsInEmail, len(req.Attachments) > 0),
	}
}

func announcementEmailOptionsFromScheduleRequest(req scheduleAnnouncementRequest, publishAt time.Time) AnnouncementEmailOptions {
	return AnnouncementEmailOptions{
		Attachments:               mapAnnouncementAttachments(req.Attachments),
		IncludeAttachmentsInEmail: includeAnnouncementAttachments(req.IncludeAttachmentsInEmail, len(req.Attachments) > 0),
		ScheduledAt:               &publishAt,
	}
}

func includeAnnouncementAttachments(value *bool, hasAttachments bool) bool {
	if value != nil {
		return *value
	}
	return true
}

func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}

	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return sql.NullString{}
	}

	return sql.NullString{
		String: trimmed,
		Valid:  true,
	}
}

func nullableTime(s *string) (sql.NullTime, error) {
	if s == nil {
		return sql.NullTime{}, nil
	}

	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return sql.NullTime{}, nil
	}

	t, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return sql.NullTime{}, err
	}

	return sql.NullTime{
		Time:  t,
		Valid: true,
	}, nil
}

func nullableUUID(u uuid.NullUUID) *string {
	if !u.Valid {
		return nil
	}

	s := u.UUID.String()
	return &s
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	if len(values) == 0 {
		return nil, nil
	}

	out := make([]uuid.UUID, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		id, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, err
		}

		out = append(out, id)
	}

	return out, nil
}

func nullUUIDString(v uuid.NullUUID) string {
	if !v.Valid {
		return ""
	}

	return v.UUID.String()
}

func normalizeLevel(v AnnouncementLevel) string {
	return strings.ToUpper(strings.TrimSpace(string(v)))
}

func normalizeAnnouncementStatus(v AnnouncementStatus) string {
	return strings.ToUpper(strings.TrimSpace(string(v)))
}

func normalizeAudienceType(v AnnouncementAudienceType) string {
	return strings.ToUpper(strings.TrimSpace(string(v)))
}
