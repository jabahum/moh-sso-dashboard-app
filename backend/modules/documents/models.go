package documents

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Document statuses stored in documents.status.
const (
	DocumentStatusPending    = "PENDING"
	DocumentStatusProcessing = "PROCESSING"
	DocumentStatusCompleted  = "COMPLETED"
	DocumentStatusFailed     = "FAILED"
)

// ProcessStatusProcessing is the processes.status of a claimed job.
const ProcessStatusProcessing = "PROCESSING"

// Document is a documents row.
type Document struct {
	ID                 uuid.UUID
	OriginalFilename   string
	NormalizedFilename sql.NullString
	ContentType        sql.NullString
	Extension          sql.NullString
	SizeBytes          int64
	ChecksumSha256     sql.NullString
	StorageLocationID  uuid.UUID
	ObjectKey          string
	UploadedBy         uuid.UUID
	Status             string
	Version            int32
	ParentDocumentID   uuid.NullUUID
	Metadata           json.RawMessage
	Tags               json.RawMessage
	IsTemplate         bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          sql.NullTime
	HealthContextID    uuid.NullUUID
}

// Process is a processes row: one processing job for a document.
type Process struct {
	ID          uuid.UUID
	DocumentID  uuid.UUID
	ProcessType string
	Status      string
	Progress    int32
	Message     sql.NullString
	Error       sql.NullString
	Attempts    int32
	CreatedBy   uuid.UUID
	StartedAt   sql.NullTime
	FinishedAt  sql.NullTime
	CreatedAt   sql.NullTime
	UpdatedAt   sql.NullTime
}

// NewDocument is the data stored for an upload.
type NewDocument struct {
	ID                uuid.UUID
	OriginalFilename  string
	ContentType       sql.NullString
	SizeBytes         int64
	ChecksumSha256    sql.NullString
	StorageLocationID uuid.UUID
	ObjectKey         string
	UploadedBy        uuid.UUID
	Status            string
	Metadata          json.RawMessage
	IsTemplate        bool
	HealthContextID   uuid.NullUUID
}

// NewProcess is the data stored for a queued processing job.
type NewProcess struct {
	ID          uuid.UUID
	DocumentID  uuid.UUID
	ProcessType string
	CreatedBy   uuid.UUID
}

// DocumentEdit is the editable part of a document.
type DocumentEdit struct {
	ID               uuid.UUID
	OriginalFilename string
	ContentType      sql.NullString
}

// HealthContextScope restricts reads to documents in a health context and,
// optionally, its descendants.
type HealthContextScope struct {
	ID                 uuid.UUID
	IncludeDescendants bool
}

// Pagination bounds list queries.
type Pagination struct {
	Limit  int32
	Offset int32
}

// Stats summarises non-template documents.
type Stats struct {
	Total      int64 `json:"total"`
	TotalSize  int64 `json:"total_size"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
}

// ProcessType names a processing pipeline selected at upload.
type ProcessType string

// validProcessTypes mirrors the host's accepted process types.
var validProcessTypes = map[ProcessType]struct{}{
	"SURVEILLANCE_CSV_IMPORT":    {},
	"CSV_IMPORT":                 {},
	"EXCEL_IMPORT":               {},
	"FHIR_IMPORT":                {},
	"USER_BULK_IMPORT":           {},
	"SURVEILLANCE_EXCEL_IMPORT":  {},
	"FACILITY_IMPORT":            {},
	"LAB_RESULT_IMPORT":          {},
	"INVENTORY_IMPORT":           {},
	"HISTORICAL_BACKFILL_IMPORT": {},
	"DRY_RUN_IMPORT":             {},
	"ETL_IMPORT":                 {},
	"ARCHIVE_IMPORT":             {},
	"IMAGE_PROCESSING":           {},
	"DOCUMENT_EXTRACTION":        {},
	"NO_OP_UPLOAD":               {},
}

// IsValid reports whether the process type is accepted.
func (p ProcessType) IsValid() bool {
	_, ok := validProcessTypes[p]
	return ok
}

// ParseProcessType validates a submitted process type.
func ParseProcessType(value string) (ProcessType, bool) {
	p := ProcessType(value)
	return p, p.IsValid()
}

type UpdateDocumentRequest struct {
	OriginalFilename *string `json:"original_filename"`
	ContentType      *string `json:"content_type"`
}

type DocumentResponse struct {
	ID               uuid.UUID              `json:"id"`
	OriginalFilename string                 `json:"original_filename"`
	ContentType      string                 `json:"content_type"`
	SizeBytes        int64                  `json:"size_bytes"`
	ChecksumSHA256   *string                `json:"checksum_sha256,omitempty"`
	StorageLocation  uuid.UUID              `json:"storage_location"`
	ObjectKey        string                 `json:"object_key"`
	UploadedBy       uuid.UUID              `json:"uploaded_by"`
	Status           string                 `json:"status"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	ObjectURL        string                 `json:"object_url,omitempty"`
	ViewURL          string                 `json:"view_url,omitempty"`
	DownloadURL      string                 `json:"download_url,omitempty"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
	HealthContextID  *uuid.UUID             `json:"health_context_id,omitempty"`
}

type ProcessResponse struct {
	ID          uuid.UUID  `json:"id"`
	DocumentID  uuid.UUID  `json:"document_id"`
	ProcessType string     `json:"process_type"`
	Status      string     `json:"status"`
	Progress    int32      `json:"progress"`
	Message     *string    `json:"message,omitempty"`
	Error       *string    `json:"error,omitempty"`
	Attempts    int32      `json:"attempts"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type messageResponse struct {
	Message string `json:"message"`
}

func toDocumentResponse(doc Document, objectURL, viewURL, downloadURL string) DocumentResponse {
	var metadata map[string]interface{}
	if len(doc.Metadata) > 0 {
		_ = json.Unmarshal(doc.Metadata, &metadata)
	}
	return DocumentResponse{
		ID:               doc.ID,
		Metadata:         metadata,
		OriginalFilename: doc.OriginalFilename,
		ContentType:      nullStringValue(doc.ContentType),
		SizeBytes:        doc.SizeBytes,
		ChecksumSHA256:   nullStringPtr(doc.ChecksumSha256),
		StorageLocation:  doc.StorageLocationID,
		ObjectKey:        doc.ObjectKey,
		UploadedBy:       doc.UploadedBy,
		Status:           normalizeStatus(doc.Status),
		ObjectURL:        objectURL,
		ViewURL:          viewURL,
		DownloadURL:      downloadURL,
		CreatedAt:        doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        doc.UpdatedAt.Format(time.RFC3339),
		HealthContextID:  nullUUIDPtr(doc.HealthContextID),
	}
}

func toProcessResponse(process Process) ProcessResponse {
	return ProcessResponse{
		ID:          process.ID,
		DocumentID:  process.DocumentID,
		ProcessType: process.ProcessType,
		Status:      normalizeStatus(process.Status),
		Progress:    process.Progress,
		Message:     nullStringPtr(process.Message),
		Error:       nullStringPtr(process.Error),
		Attempts:    process.Attempts,
		CreatedBy:   process.CreatedBy,
		StartedAt:   nullTimePtr(process.StartedAt),
		FinishedAt:  nullTimePtr(process.FinishedAt),
		CreatedAt:   nullTimePtr(process.CreatedAt),
		UpdatedAt:   nullTimePtr(process.UpdatedAt),
	}
}

func normalizeStatus(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	case []byte:
		return string(s)
	case fmt.Stringer:
		return s.String()
	default:
		return fmt.Sprint(v)
	}
}

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

func nullUUIDPtr(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	return &value.UUID
}

func nullableUUID(value *uuid.UUID) uuid.NullUUID {
	if value == nil || *value == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *value, Valid: true}
}

func nullStringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func requiresProcessing(mimeType, fileName string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "text/csv",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return true
	case "application/pdf":
		return false
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".csv", ".xls", ".xlsx":
		return true
	case ".pdf":
		return false
	default:
		return false
	}
}

// isCSV reports whether the file is specifically CSV (not Excel). Unlike
// Excel, CSV import needs no template-defined sheet/column structure to
// parse, so it's the only format that can be queued for processing without
// a template_code.
func isCSV(mimeType, fileName string) bool {
	if strings.ToLower(strings.TrimSpace(mimeType)) == "text/csv" {
		return true
	}
	return strings.ToLower(filepath.Ext(fileName)) == ".csv"
}

func shouldQueueProcessing(
	contentType string,
	filename string,
	isTemplate bool,
	templateCode string,
) bool {
	if isTemplate || !requiresProcessing(contentType, filename) {
		return false
	}
	if strings.TrimSpace(templateCode) != "" {
		return true
	}
	// No template selected: only CSV can still be queued, since it needs no
	// template-defined structure to parse (unlike Excel).
	return isCSV(contentType, filename)
}
