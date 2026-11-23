package documents

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// Repository persists documents and their processing jobs in the primary
// database (documents, processes and the document-template structure tables).
type Repository interface {
	CreateDocument(ctx context.Context, arg NewDocument) (Document, error)
	GetDocument(ctx context.Context, id uuid.UUID) (Document, error)
	GetDocumentInHealthContext(ctx context.Context, id uuid.UUID, scope HealthContextScope) (Document, error)
	ListDocuments(ctx context.Context, page Pagination) ([]Document, error)
	ListDocumentsInHealthContext(ctx context.Context, scope HealthContextScope, page Pagination) ([]Document, error)
	EditDocument(ctx context.Context, arg DocumentEdit) (Document, error)
	MarkDocumentPending(ctx context.Context, id uuid.UUID) (Document, error)
	MarkDocumentProcessing(ctx context.Context, id uuid.UUID) (Document, error)
	MarkDocumentCompleted(ctx context.Context, id uuid.UUID) (Document, error)
	MarkDocumentFailed(ctx context.Context, id uuid.UUID) (Document, error)
	DeleteDocument(ctx context.Context, id uuid.UUID) error
	Stats(ctx context.Context, scope *HealthContextScope) (Stats, error)

	CreateProcess(ctx context.Context, arg NewProcess) (Process, error)
	ListProcessesByDocument(ctx context.Context, documentID uuid.UUID) ([]Process, error)
	GetLatestProcessByDocumentID(ctx context.Context, documentID uuid.UUID) (Process, error)
	// ClaimNextPendingProcess atomically moves the oldest PENDING job to
	// PROCESSING; it returns sql.ErrNoRows when the queue is empty.
	ClaimNextPendingProcess(ctx context.Context) (Process, error)

	// TemplateSheetColumns returns a template sheet's display name and its
	// column keys in column order.
	TemplateSheetColumns(ctx context.Context, templateCode, sheetCode string) (string, []string, error)
}

// NewPostgresRepository stores documents in the primary database.
func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepository{db: db} }

type postgresRepository struct{ db *sql.DB }

const documentColumns = `id, original_filename, normalized_filename, content_type, extension, size_bytes, checksum_sha256, storage_location_id, object_key, uploaded_by, status, version, parent_document_id, metadata, tags, is_template, created_at, updated_at, deleted_at, health_context_id`

const prefixedDocumentColumns = `d.id, d.original_filename, d.normalized_filename, d.content_type, d.extension, d.size_bytes, d.checksum_sha256, d.storage_location_id, d.object_key, d.uploaded_by, d.status, d.version, d.parent_document_id, d.metadata, d.tags, d.is_template, d.created_at, d.updated_at, d.deleted_at, d.health_context_id`

const processColumns = `id, document_id, process_type, status, progress, message, error, attempts, created_by, started_at, finished_at, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scanDocument(row scanner) (Document, error) {
	var d Document
	err := row.Scan(&d.ID, &d.OriginalFilename, &d.NormalizedFilename, &d.ContentType, &d.Extension,
		&d.SizeBytes, &d.ChecksumSha256, &d.StorageLocationID, &d.ObjectKey, &d.UploadedBy, &d.Status,
		&d.Version, &d.ParentDocumentID, &d.Metadata, &d.Tags, &d.IsTemplate, &d.CreatedAt, &d.UpdatedAt,
		&d.DeletedAt, &d.HealthContextID)
	return d, err
}

func scanProcess(row scanner) (Process, error) {
	var p Process
	err := row.Scan(&p.ID, &p.DocumentID, &p.ProcessType, &p.Status, &p.Progress, &p.Message, &p.Error,
		&p.Attempts, &p.CreatedBy, &p.StartedAt, &p.FinishedAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func scanDocuments(rows *sql.Rows, err error) ([]Document, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Document{}
	for rows.Next() {
		item, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *postgresRepository) CreateDocument(ctx context.Context, arg NewDocument) (Document, error) {
	return scanDocument(r.db.QueryRowContext(ctx, `INSERT INTO documents (
    id,
    original_filename,
    content_type,
    size_bytes,
    checksum_sha256,
    storage_location_id,
    object_key,
    uploaded_by,
    status,
    metadata,
    is_template,
    health_context_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING `+documentColumns,
		arg.ID, arg.OriginalFilename, arg.ContentType, arg.SizeBytes, arg.ChecksumSha256,
		arg.StorageLocationID, arg.ObjectKey, arg.UploadedBy, arg.Status, arg.Metadata,
		arg.IsTemplate, arg.HealthContextID))
}

func (r *postgresRepository) GetDocument(ctx context.Context, id uuid.UUID) (Document, error) {
	return scanDocument(r.db.QueryRowContext(ctx, `SELECT `+documentColumns+`
FROM documents
WHERE id = $1`, id))
}

func (r *postgresRepository) GetDocumentInHealthContext(ctx context.Context, id uuid.UUID, scope HealthContextScope) (Document, error) {
	return scanDocument(r.db.QueryRowContext(ctx, `SELECT `+prefixedDocumentColumns+`
FROM documents d
WHERE d.id = $1
  AND (
      d.health_context_id = $2
      OR (
          $3::boolean
          AND EXISTS (
              SELECT 1
              FROM health_context_closure hc
              WHERE hc.ancestor_id = $2
                AND hc.descendant_id = d.health_context_id
          )
      )
  )`, id, uuid.NullUUID{UUID: scope.ID, Valid: true}, scope.IncludeDescendants))
}

func (r *postgresRepository) ListDocuments(ctx context.Context, page Pagination) ([]Document, error) {
	limit := page.Limit
	if limit <= 0 {
		limit = 25
	}
	return scanDocuments(r.db.QueryContext(ctx, `SELECT `+documentColumns+`
FROM documents
WHERE is_template = FALSE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2`, limit, page.Offset))
}

func (r *postgresRepository) ListDocumentsInHealthContext(ctx context.Context, scope HealthContextScope, page Pagination) ([]Document, error) {
	limit := page.Limit
	if limit <= 0 {
		limit = 25
	}
	return scanDocuments(r.db.QueryContext(ctx, `SELECT `+prefixedDocumentColumns+`
FROM documents d
WHERE d.is_template = FALSE
  AND (
      d.health_context_id = $1
      OR (
          $2::boolean
          AND EXISTS (
              SELECT 1
              FROM health_context_closure hc
              WHERE hc.ancestor_id = $1
                AND hc.descendant_id = d.health_context_id
          )
      )
  )
ORDER BY d.created_at DESC
LIMIT $4 OFFSET $3`, uuid.NullUUID{UUID: scope.ID, Valid: true}, scope.IncludeDescendants, page.Offset, limit))
}

func (r *postgresRepository) EditDocument(ctx context.Context, arg DocumentEdit) (Document, error) {
	return scanDocument(r.db.QueryRowContext(ctx, `UPDATE documents
SET original_filename = $2,
    content_type = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING `+documentColumns, arg.ID, arg.OriginalFilename, arg.ContentType))
}

func (r *postgresRepository) markDocument(ctx context.Context, id uuid.UUID, status string) (Document, error) {
	// status is one of the fixed literals below, never caller input.
	return scanDocument(r.db.QueryRowContext(ctx, `UPDATE documents
SET status = '`+status+`',
    updated_at = NOW()
WHERE id = $1
RETURNING `+documentColumns, id))
}

func (r *postgresRepository) MarkDocumentPending(ctx context.Context, id uuid.UUID) (Document, error) {
	return r.markDocument(ctx, id, DocumentStatusPending)
}

func (r *postgresRepository) MarkDocumentProcessing(ctx context.Context, id uuid.UUID) (Document, error) {
	return r.markDocument(ctx, id, DocumentStatusProcessing)
}

func (r *postgresRepository) MarkDocumentCompleted(ctx context.Context, id uuid.UUID) (Document, error) {
	return r.markDocument(ctx, id, DocumentStatusCompleted)
}

func (r *postgresRepository) MarkDocumentFailed(ctx context.Context, id uuid.UUID) (Document, error) {
	return r.markDocument(ctx, id, DocumentStatusFailed)
}

func (r *postgresRepository) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM documents
WHERE id = $1`, id)
	return err
}

func (r *postgresRepository) Stats(ctx context.Context, scope *HealthContextScope) (Stats, error) {
	query := `
		SELECT
			COUNT(*)                                              AS total,
			COALESCE(SUM(size_bytes), 0)                         AS total_size,
			COUNT(*) FILTER (WHERE status = 'PENDING')           AS pending,
			COUNT(*) FILTER (WHERE status = 'PROCESSING')        AS processing,
			COUNT(*) FILTER (WHERE status = 'COMPLETED')         AS completed,
			COUNT(*) FILTER (WHERE status = 'FAILED')            AS failed
		FROM documents
		WHERE is_template = false
	`
	args := []any{}
	if scope != nil {
		query += `
			AND (
				health_context_id = $1
				OR (
					$2::boolean
					AND EXISTS (
						SELECT 1
						FROM health_context_closure hc
						WHERE hc.ancestor_id = $1
						  AND hc.descendant_id = documents.health_context_id
					)
				)
			)
		`
		args = append(args, scope.ID, scope.IncludeDescendants)
	}
	var stats Stats
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.Total, &stats.TotalSize,
		&stats.Pending, &stats.Processing,
		&stats.Completed, &stats.Failed,
	)
	return stats, err
}

func (r *postgresRepository) CreateProcess(ctx context.Context, arg NewProcess) (Process, error) {
	return scanProcess(r.db.QueryRowContext(ctx, `INSERT INTO processes (
    id,
    document_id,
    process_type,
    created_by
) VALUES (
    $1, $2, $3, $4
)
RETURNING `+processColumns, arg.ID, arg.DocumentID, arg.ProcessType, arg.CreatedBy))
}

func (r *postgresRepository) ListProcessesByDocument(ctx context.Context, documentID uuid.UUID) ([]Process, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+processColumns+`
FROM processes
WHERE document_id = $1
ORDER BY created_at DESC`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Process{}
	for rows.Next() {
		item, err := scanProcess(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *postgresRepository) GetLatestProcessByDocumentID(ctx context.Context, documentID uuid.UUID) (Process, error) {
	return scanProcess(r.db.QueryRowContext(ctx, `SELECT `+processColumns+`
FROM processes
WHERE document_id = $1
ORDER BY created_at DESC
LIMIT 1`, documentID))
}

func (r *postgresRepository) ClaimNextPendingProcess(ctx context.Context) (Process, error) {
	return scanProcess(r.db.QueryRowContext(ctx, `UPDATE processes
SET status = 'PROCESSING',
    started_at = NOW(),
    attempts = attempts + 1,
    updated_at = NOW()
WHERE id = (
    SELECT id
    FROM processes
    WHERE status = 'PENDING'
    ORDER BY created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING `+processColumns))
}

func (r *postgresRepository) TemplateSheetColumns(ctx context.Context, templateCode, sheetCode string) (string, []string, error) {
	rows, err := r.db.QueryContext(ctx, `
			SELECT dts.display_name, dtc.column_key
			FROM document_template_sheets dts
			JOIN document_template_columns dtc ON dtc.sheet_id = dts.id
			JOIN document_templates dt ON dt.id = dts.template_id
			WHERE dt.code = $1 AND dts.code = $2
			ORDER BY dtc.column_order
		`, templateCode, sheetCode)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var displayName string
	var columns []string
	for rows.Next() {
		var name, columnKey string
		if err := rows.Scan(&name, &columnKey); err == nil {
			displayName = name
			columns = append(columns, columnKey)
		}
	}
	return displayName, columns, nil
}
