package announcements

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

type UploadAnnouncementAttachmentInput struct {
	FileName       string
	ContentType    string
	DataBase64     string
	Reader         io.Reader
	Size           int64
	IncludeInEmail bool
	Inline         bool
	ContentID      string
	SortOrder      int32
	UploadedBy     uuid.UUID
}

type UpdateAnnouncementAttachmentInput struct {
	IncludeInEmail *bool
	Inline         *bool
	ContentID      *string
	SortOrder      *int32
	UpdatedBy      uuid.UUID
}

type AnnouncementAttachmentDownload struct {
	Attachment AnnouncementAttachment
	Reader     io.ReadCloser
}

func (s *Service) UploadAttachment(
	ctx context.Context,
	announcementID uuid.UUID,
	input UploadAnnouncementAttachmentInput,
) (AnnouncementAttachment, error) {
	if s == nil {
		return AnnouncementAttachment{}, errors.New("announcement service is nil")
	}
	if s.repo == nil {
		return AnnouncementAttachment{}, errors.New("announcement repository is nil")
	}
	if s.storage == nil {
		return AnnouncementAttachment{}, errors.New("announcement attachment storage is nil")
	}
	if announcementID == uuid.Nil {
		return AnnouncementAttachment{}, errors.New("announcement id is required")
	}

	if _, err := s.repo.GetByID(ctx, announcementID); err != nil {
		return AnnouncementAttachment{}, fmt.Errorf("get announcement: %w", err)
	}

	existingCount, err := s.repo.CountAttachments(ctx, announcementID)
	if err != nil {
		return AnnouncementAttachment{}, err
	}
	if existingCount >= int64(s.announcementMaxAttachments()) {
		return AnnouncementAttachment{}, fmt.Errorf("announcement attachment limit exceeded: max %d", s.announcementMaxAttachments())
	}

	originalName := strings.TrimSpace(input.FileName)
	safeName := sanitizeAttachmentFileName(originalName)
	if safeName == "" {
		return AnnouncementAttachment{}, errors.New("attachment file name is required")
	}
	if !s.announcementAttachmentExtensionAllowed(safeName) {
		return AnnouncementAttachment{}, fmt.Errorf("attachment type is not allowed: %s", filepath.Ext(safeName))
	}
	if input.Inline && strings.TrimSpace(input.ContentID) == "" {
		return AnnouncementAttachment{}, errors.New("content_id is required for inline attachments")
	}

	data, err := readAttachmentBytes(input, s.announcementMaxAttachmentBytes())
	if err != nil {
		return AnnouncementAttachment{}, err
	}

	attachmentID := uuid.New()
	checksum := sha256.Sum256(data)
	checksumHex := hex.EncodeToString(checksum[:])
	storageProvider := strings.TrimSpace(s.storageProvider())
	objectKey := path.Join(
		"announcements",
		announcementID.String(),
		attachmentID.String(),
		safeName,
	)

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := s.storage.Upload(ctx, objectKey, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return AnnouncementAttachment{}, fmt.Errorf("upload announcement attachment: %w", err)
	}

	params := CreateAnnouncementAttachmentParams{
		ID:               attachmentID,
		AnnouncementID:   announcementID,
		FileName:         safeName,
		OriginalFileName: originalName,
		ContentType:      nullableStringValue(contentType),
		FileSize:         int64(len(data)),
		StorageProvider:  storageProvider,
		StorageKey:       objectKey,
		Checksum:         nullableStringValue(checksumHex),
		UploadedBy:       uuid.NullUUID{UUID: input.UploadedBy, Valid: input.UploadedBy != uuid.Nil},
		IncludeInEmail:   input.IncludeInEmail,
		Inline:           input.Inline,
		ContentID:        nullableStringValue(input.ContentID),
		SortOrder:        input.SortOrder,
	}

	attachment, err := s.repo.CreateAttachment(ctx, params)
	if err != nil {
		_ = s.storage.Delete(ctx, objectKey)
		return AnnouncementAttachment{}, err
	}

	return attachment, nil
}

func (s *Service) ListAttachments(
	ctx context.Context,
	announcementID uuid.UUID,
) ([]AnnouncementAttachment, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("announcement service repository is nil")
	}
	if announcementID == uuid.Nil {
		return nil, errors.New("announcement id is required")
	}
	return s.repo.ListAttachmentsByAnnouncementID(ctx, announcementID)
}

func (s *Service) UpdateAttachment(
	ctx context.Context,
	announcementID uuid.UUID,
	attachmentID uuid.UUID,
	input UpdateAnnouncementAttachmentInput,
) (AnnouncementAttachment, error) {
	if s == nil || s.repo == nil {
		return AnnouncementAttachment{}, errors.New("announcement service repository is nil")
	}
	if announcementID == uuid.Nil || attachmentID == uuid.Nil {
		return AnnouncementAttachment{}, errors.New("announcement id and attachment id are required")
	}

	current, err := s.repo.GetAttachmentByID(ctx, GetAnnouncementAttachmentByIDParams{
		ID:             attachmentID,
		AnnouncementID: announcementID,
	})
	if err != nil {
		return AnnouncementAttachment{}, err
	}

	includeInEmail := current.IncludeInEmail
	if input.IncludeInEmail != nil {
		includeInEmail = *input.IncludeInEmail
	}

	inline := current.Inline
	if input.Inline != nil {
		inline = *input.Inline
	}

	contentID := current.ContentID
	if input.ContentID != nil {
		contentID = nullableStringValue(*input.ContentID)
	}
	if inline && strings.TrimSpace(contentID.String) == "" {
		return AnnouncementAttachment{}, errors.New("content_id is required for inline attachments")
	}

	sortOrder := current.SortOrder
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}

	return s.repo.UpdateAttachment(ctx, UpdateAnnouncementAttachmentParams{
		ID:             attachmentID,
		AnnouncementID: announcementID,
		IncludeInEmail: includeInEmail,
		Inline:         inline,
		ContentID:      contentID,
		SortOrder:      sortOrder,
	})
}

func (s *Service) DeleteAttachment(
	ctx context.Context,
	announcementID uuid.UUID,
	attachmentID uuid.UUID,
	deletedBy uuid.UUID,
) (AnnouncementAttachment, error) {
	if s == nil || s.repo == nil {
		return AnnouncementAttachment{}, errors.New("announcement service repository is nil")
	}
	if announcementID == uuid.Nil || attachmentID == uuid.Nil {
		return AnnouncementAttachment{}, errors.New("announcement id and attachment id are required")
	}
	return s.repo.SoftDeleteAttachment(ctx, SoftDeleteAnnouncementAttachmentParams{
		ID:             attachmentID,
		AnnouncementID: announcementID,
		DeletedBy:      uuid.NullUUID{UUID: deletedBy, Valid: deletedBy != uuid.Nil},
	})
}

func (s *Service) OpenAttachmentDownload(
	ctx context.Context,
	announcementID uuid.UUID,
	attachmentID uuid.UUID,
) (AnnouncementAttachmentDownload, error) {
	if s == nil || s.repo == nil {
		return AnnouncementAttachmentDownload{}, errors.New("announcement service repository is nil")
	}
	if s.storage == nil {
		return AnnouncementAttachmentDownload{}, errors.New("announcement attachment storage is nil")
	}

	attachment, err := s.repo.GetAttachmentByID(ctx, GetAnnouncementAttachmentByIDParams{
		ID:             attachmentID,
		AnnouncementID: announcementID,
	})
	if err != nil {
		return AnnouncementAttachmentDownload{}, err
	}

	reader, err := s.storage.Download(ctx, attachment.StorageKey)
	if err != nil {
		return AnnouncementAttachmentDownload{}, fmt.Errorf("download announcement attachment: %w", err)
	}

	return AnnouncementAttachmentDownload{
		Attachment: attachment,
		Reader:     reader,
	}, nil
}

func (s *Service) withPersistedAnnouncementEmailAttachments(
	ctx context.Context,
	announcementID uuid.UUID,
	options AnnouncementEmailOptions,
) (AnnouncementEmailOptions, error) {
	if !options.IncludeAttachmentsInEmail {
		return options, nil
	}
	if s == nil || s.repo == nil || s.storage == nil {
		return options, nil
	}

	attachments, err := s.repo.ListEmailAttachments(ctx, announcementID)
	if err != nil {
		return AnnouncementEmailOptions{}, err
	}
	if len(attachments) == 0 {
		return options, nil
	}

	// A stored attachment is skipped when the request already attaches a file
	// with the same name (as recipients see it), or when an identical stored
	// file (same checksum and name) was already added.
	requested := make(map[string]struct{}, len(options.Attachments))
	for _, existing := range options.Attachments {
		if key := attachmentNameKey(existing.FileName); key != "" {
			requested[key] = struct{}{}
		}
	}
	stored := make(map[string]struct{}, len(attachments))

	for _, attachment := range attachments {
		name := attachmentNameKey(attachment.OriginalFileName)
		if _, ok := requested[name]; ok && name != "" {
			continue
		}
		key := attachment.Checksum.String + ":" + name
		if _, ok := stored[key]; ok {
			continue
		}

		reader, err := s.storage.Download(ctx, attachment.StorageKey)
		if err != nil {
			return AnnouncementEmailOptions{}, fmt.Errorf("download persisted announcement attachment: %w", err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return AnnouncementEmailOptions{}, readErr
		}
		if closeErr != nil {
			return AnnouncementEmailOptions{}, closeErr
		}

		options.Attachments = append(options.Attachments, Attachment{
			FileName:    attachment.OriginalFileName,
			ContentType: attachment.ContentType.String,
			Data:        data,
			DataBase64:  base64.StdEncoding.EncodeToString(data),
			ContentID:   attachment.ContentID.String,
			Inline:      attachment.Inline,
		})
		options.AttachmentLinks = append(options.AttachmentLinks, AnnouncementEmailAttachmentLink{
			FileName:    attachment.OriginalFileName,
			ContentType: attachment.ContentType.String,
			FileSize:    attachment.FileSize,
			URL:         s.announcementAttachmentDownloadURL(announcementID, attachment.ID),
		})
		stored[key] = struct{}{}
	}

	return options, nil
}

func attachmentNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func readAttachmentBytes(input UploadAnnouncementAttachmentInput, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	if strings.TrimSpace(input.DataBase64) != "" {
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.DataBase64))
		if err != nil {
			return nil, fmt.Errorf("decode attachment data_base64: %w", err)
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("attachment exceeds maximum size of %d bytes", maxBytes)
		}
		return data, nil
	}
	if input.Reader == nil {
		return nil, errors.New("attachment file is required")
	}
	if input.Size > maxBytes {
		return nil, fmt.Errorf("attachment exceeds maximum size of %d bytes", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(input.Reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("attachment exceeds maximum size of %d bytes", maxBytes)
	}
	return data, nil
}

func sanitizeAttachmentFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			return r
		}
		if unicode.IsSpace(r) {
			return '_'
		}
		return -1
	}, name)
	name = strings.Trim(name, ".")
	if name == "" {
		return ""
	}
	return name
}

func (s *Service) announcementAttachmentExtensionAllowed(fileName string) bool {
	allowed := s.announcementAllowedAttachmentTypes()
	if len(allowed) == 0 {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if ext == "" {
		return false
	}
	_, ok := allowed[ext]
	return ok
}

func (s *Service) announcementAllowedAttachmentTypes() map[string]struct{} {
	raw := ""
	if s != nil && s.settings != nil {
		raw = s.settings.Announcement.AllowedAttachmentTypes
	}
	if strings.TrimSpace(raw) == "" && s != nil && s.settings != nil {
		raw = s.settings.Email.AllowedAttachmentTypes
	}
	out := map[string]struct{}{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
		if item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}

func (s *Service) announcementMaxAttachments() int {
	if s != nil && s.settings != nil && s.settings.Announcement.MaxAttachments > 0 {
		return s.settings.Announcement.MaxAttachments
	}
	return 10
}

func (s *Service) announcementMaxAttachmentBytes() int64 {
	if s != nil && s.settings != nil && s.settings.Announcement.MaxAttachmentBytes > 0 {
		return s.settings.Announcement.MaxAttachmentBytes
	}
	return 10 * 1024 * 1024
}

func (s *Service) storageProvider() string {
	if s != nil && s.settings != nil && strings.TrimSpace(s.settings.StorageProvider) != "" {
		return strings.ToLower(strings.TrimSpace(s.settings.StorageProvider))
	}
	return "local"
}

func nullableStringValue(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
