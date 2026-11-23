package documentimports

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// CustomImportRow is a single parsed data row ready to be persisted.
type CustomImportRow struct {
	FileKey      int64
	DocumentID   uuid.UUID
	TemplateCode string
	SheetCode    string
	RowNumber    int
	ReportDate   *time.Time
	Data         map[string]any
	RowHash      string
}

// FileRepository persists data for any template-driven import (Excel or CSV)
// using the generic import.custom_files + import.custom_data_files tables.
type FileRepository interface {
	// CreateFile inserts a row into import.custom_files and returns its file_key.
	// If a file already exists for the given document, it is refreshed in place.
	CreateFile(
		ctx context.Context,
		tx *sql.Tx,
		documentID uuid.UUID,
		templateCode, fileName, filePath string,
		reportDate *time.Time,
	) (int64, error)

	// UpsertRowsBatch inserts or updates a batch of data rows.
	UpsertRowsBatch(ctx context.Context, tx *sql.Tx, rows []CustomImportRow) error

	// SoftDeleteBySheet marks rows is_current='N' for the given template+sheet
	// whose row_hash is NOT in keepHashes (rows removed from the latest upload).
	SoftDeleteBySheet(
		ctx context.Context,
		tx *sql.Tx,
		documentID uuid.UUID,
		templateCode, sheetCode string,
		keepHashes []string,
	) error

	// InvalidateByDocument marks all rows for a given document as is_current='N',
	// used when a document is replaced by a newer upload.
	InvalidateByDocument(ctx context.Context, tx *sql.Tx, documentID uuid.UUID) error

	// InvalidateFile marks the custom_files row is_current='N'.
	InvalidateFile(ctx context.Context, tx *sql.Tx, documentID uuid.UUID) error
}
