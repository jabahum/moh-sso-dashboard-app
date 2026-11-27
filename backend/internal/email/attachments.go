package email

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/model"
)

func NormalizeAttachments(cfg *config.Config, attachments []model.Attachment) ([]model.Attachment, error) {
	if len(attachments) == 0 {
		return nil, nil
	}

	maxCount := 5
	maxBytes := int64(10 * 1024 * 1024)
	allowed := allowedAttachmentTypes("pdf,doc,docx,xls,xlsx,csv,png,jpg,jpeg")
	baseDir := ""
	if cfg != nil {
		baseDir = strings.TrimSpace(cfg.Email.AttachmentBaseDir)
		if cfg.Email.MaxAttachments > 0 {
			maxCount = cfg.Email.MaxAttachments
		}
		if cfg.Email.MaxAttachmentBytes > 0 {
			maxBytes = cfg.Email.MaxAttachmentBytes
		}
		if strings.TrimSpace(cfg.Email.AllowedAttachmentTypes) != "" {
			allowed = allowedAttachmentTypes(cfg.Email.AllowedAttachmentTypes)
		}
	}

	if len(attachments) > maxCount {
		return nil, fmt.Errorf("too many attachments: max %d", maxCount)
	}

	out := make([]model.Attachment, 0, len(attachments))
	for i, attachment := range attachments {
		normalized, err := normalizeAttachment(attachment, allowed, maxBytes, baseDir)
		if err != nil {
			return nil, fmt.Errorf("attachment %d: %w", i+1, err)
		}
		out = append(out, normalized)
	}

	return out, nil
}

// ErrPathAttachmentsDisabled is returned for server-file attachments when no
// EMAIL_ATTACHMENT_BASE_DIR is configured.
var ErrPathAttachmentsDisabled = errors.New("server file attachments are disabled; send the file as data_base64")

func normalizeAttachment(att model.Attachment, allowed map[string]bool, maxBytes int64, baseDir string) (model.Attachment, error) {
	att.FileName = strings.TrimSpace(att.FileName)
	att.ContentType = strings.TrimSpace(att.ContentType)
	att.Path = strings.TrimSpace(att.Path)
	att.ContentID = strings.TrimSpace(att.ContentID)
	att.DataBase64 = strings.TrimSpace(att.DataBase64)

	if att.FileName == "" && att.Path != "" {
		att.FileName = filepath.Base(att.Path)
	}
	if att.FileName == "" {
		return model.Attachment{}, fmt.Errorf("file_name is required")
	}
	if att.Inline && att.ContentID == "" {
		return model.Attachment{}, fmt.Errorf("content_id is required for inline attachments")
	}
	if err := validateAllowedAttachmentType(att, allowed); err != nil {
		return model.Attachment{}, err
	}

	hasPath := att.Path != ""
	hasData := len(att.Data) > 0 || att.DataBase64 != ""
	if hasPath == hasData {
		return model.Attachment{}, fmt.Errorf("provide exactly one of path or data_base64")
	}

	if hasPath {
		if err := validateAllowedPathType(att.Path, allowed); err != nil {
			return model.Attachment{}, err
		}
		data, err := readAttachmentFromBaseDir(baseDir, att.Path, maxBytes)
		if err != nil {
			return model.Attachment{}, err
		}
		// Carry the bytes instead of the path so nothing downstream (queue,
		// retry, SMTP send) ever opens a client-supplied path again.
		att.Path = ""
		att.Data = data
		att.DataBase64 = base64.StdEncoding.EncodeToString(data)
		return att, nil
	}

	if len(att.Data) == 0 {
		decoded, err := base64.StdEncoding.DecodeString(att.DataBase64)
		if err != nil {
			return model.Attachment{}, fmt.Errorf("invalid data_base64: %w", err)
		}
		att.Data = decoded
	}
	if int64(len(att.Data)) > maxBytes {
		return model.Attachment{}, fmt.Errorf("file exceeds max attachment size")
	}
	if att.DataBase64 == "" {
		att.DataBase64 = base64.StdEncoding.EncodeToString(att.Data)
	}

	return att, nil
}

// readAttachmentFromBaseDir reads a server-side attachment, confined to baseDir.
// Paths may be relative to baseDir or absolute paths inside it. os.OpenInRoot
// rejects any path (including via symlinks) that resolves outside baseDir.
func readAttachmentFromBaseDir(baseDir, path string, maxBytes int64) ([]byte, error) {
	if baseDir == "" {
		return nil, ErrPathAttachmentsDisabled
	}
	name := filepath.Clean(path)
	if filepath.IsAbs(name) {
		rel, err := filepath.Rel(baseDir, name)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("attachment path must be inside the configured attachment directory")
		}
		name = rel
	}
	file, err := os.OpenInRoot(baseDir, name)
	if err != nil {
		return nil, fmt.Errorf("attachment path must name a file inside the configured attachment directory")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat attachment: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("attachment path is not a regular file")
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file exceeds max attachment size")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file exceeds max attachment size")
	}
	return data, nil
}

// validateAllowedPathType checks the extension of the file actually read, so a
// permitted file_name cannot disguise a different file type on disk.
func validateAllowedPathType(path string, allowed map[string]bool) error {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" || !allowed[ext] {
		return fmt.Errorf("attachment path type %q is not allowed", ext)
	}
	return nil
}

func validateAllowedAttachmentType(att model.Attachment, allowed map[string]bool) error {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(att.FileName)), ".")
	if ext == "" && att.Path != "" {
		ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(att.Path)), ".")
	}
	if ext == "" {
		return fmt.Errorf("attachment extension is required")
	}
	if !allowed[ext] {
		return fmt.Errorf("attachment type %q is not allowed", ext)
	}
	return nil
}

func allowedAttachmentTypes(raw string) map[string]bool {
	out := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
		if item != "" {
			out[item] = true
		}
	}
	return out
}
