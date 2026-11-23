// Copied from the host's sqlc output (internal/db/sqlc) when the feature
// moved into this module. The tables stay in the host's migrations.
// source: document_template_sheet.sql

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

const archiveDocumentTemplateSheet = `-- name: ArchiveDocumentTemplateSheet :exec
UPDATE document_template_sheets
SET
    archived_at = NOW(),
    updated_at = NOW()
WHERE id = $1
`

func (q *Queries) ArchiveDocumentTemplateSheet(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, archiveDocumentTemplateSheet, id)
	return err
}

const createDocumentTemplateSheet = `-- name: CreateDocumentTemplateSheet :one
INSERT INTO document_template_sheets (
    id,
    template_id,
    code,
    name,
    display_name,
    required,
    sheet_order,
    header_row,
    start_row,
    allow_extra_columns,
    allow_duplicate_headers,
    configuration
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12
)
RETURNING id, template_id, code, name, display_name, required, sheet_order, header_row, start_row, allow_extra_columns, allow_duplicate_headers, configuration, created_at, updated_at, archived_at
`

type CreateDocumentTemplateSheetParams struct {
	ID                    uuid.UUID       `json:"id"`
	TemplateID            uuid.UUID       `json:"template_id"`
	Code                  string          `json:"code"`
	Name                  string          `json:"name"`
	DisplayName           sql.NullString  `json:"display_name"`
	Required              bool            `json:"required"`
	SheetOrder            sql.NullInt32   `json:"sheet_order"`
	HeaderRow             int32           `json:"header_row"`
	StartRow              int32           `json:"start_row"`
	AllowExtraColumns     bool            `json:"allow_extra_columns"`
	AllowDuplicateHeaders bool            `json:"allow_duplicate_headers"`
	Configuration         json.RawMessage `json:"configuration"`
}

func (q *Queries) CreateDocumentTemplateSheet(ctx context.Context, arg CreateDocumentTemplateSheetParams) (DocumentTemplateSheet, error) {
	row := q.db.QueryRowContext(ctx, createDocumentTemplateSheet,
		arg.ID,
		arg.TemplateID,
		arg.Code,
		arg.Name,
		arg.DisplayName,
		arg.Required,
		arg.SheetOrder,
		arg.HeaderRow,
		arg.StartRow,
		arg.AllowExtraColumns,
		arg.AllowDuplicateHeaders,
		arg.Configuration,
	)
	var i DocumentTemplateSheet
	err := row.Scan(
		&i.ID,
		&i.TemplateID,
		&i.Code,
		&i.Name,
		&i.DisplayName,
		&i.Required,
		&i.SheetOrder,
		&i.HeaderRow,
		&i.StartRow,
		&i.AllowExtraColumns,
		&i.AllowDuplicateHeaders,
		&i.Configuration,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const deleteDocumentTemplateSheet = `-- name: DeleteDocumentTemplateSheet :exec
DELETE FROM document_template_sheets
WHERE id = $1
`

func (q *Queries) DeleteDocumentTemplateSheet(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, deleteDocumentTemplateSheet, id)
	return err
}

const existsDocumentTemplateSheetCode = `-- name: ExistsDocumentTemplateSheetCode :one
SELECT EXISTS (
    SELECT 1
    FROM document_template_sheets
    WHERE template_id = $1
      AND code = $2
      AND archived_at IS NULL
)
`

type ExistsDocumentTemplateSheetCodeParams struct {
	TemplateID uuid.UUID `json:"template_id"`
	Code       string    `json:"code"`
}

func (q *Queries) ExistsDocumentTemplateSheetCode(ctx context.Context, arg ExistsDocumentTemplateSheetCodeParams) (bool, error) {
	row := q.db.QueryRowContext(ctx, existsDocumentTemplateSheetCode, arg.TemplateID, arg.Code)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const existsDocumentTemplateSheetName = `-- name: ExistsDocumentTemplateSheetName :one
SELECT EXISTS (
    SELECT 1
    FROM document_template_sheets
    WHERE template_id = $1
      AND name = $2
      AND archived_at IS NULL
)
`

type ExistsDocumentTemplateSheetNameParams struct {
	TemplateID uuid.UUID `json:"template_id"`
	Name       string    `json:"name"`
}

func (q *Queries) ExistsDocumentTemplateSheetName(ctx context.Context, arg ExistsDocumentTemplateSheetNameParams) (bool, error) {
	row := q.db.QueryRowContext(ctx, existsDocumentTemplateSheetName, arg.TemplateID, arg.Name)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const getDocumentTemplateSheetByCode = `-- name: GetDocumentTemplateSheetByCode :one
SELECT id, template_id, code, name, display_name, required, sheet_order, header_row, start_row, allow_extra_columns, allow_duplicate_headers, configuration, created_at, updated_at, archived_at
FROM document_template_sheets
WHERE template_id = $1
  AND code = $2
  AND archived_at IS NULL
LIMIT 1
`

type GetDocumentTemplateSheetByCodeParams struct {
	TemplateID uuid.UUID `json:"template_id"`
	Code       string    `json:"code"`
}

func (q *Queries) GetDocumentTemplateSheetByCode(ctx context.Context, arg GetDocumentTemplateSheetByCodeParams) (DocumentTemplateSheet, error) {
	row := q.db.QueryRowContext(ctx, getDocumentTemplateSheetByCode, arg.TemplateID, arg.Code)
	var i DocumentTemplateSheet
	err := row.Scan(
		&i.ID,
		&i.TemplateID,
		&i.Code,
		&i.Name,
		&i.DisplayName,
		&i.Required,
		&i.SheetOrder,
		&i.HeaderRow,
		&i.StartRow,
		&i.AllowExtraColumns,
		&i.AllowDuplicateHeaders,
		&i.Configuration,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const getDocumentTemplateSheetByID = `-- name: GetDocumentTemplateSheetByID :one
SELECT id, template_id, code, name, display_name, required, sheet_order, header_row, start_row, allow_extra_columns, allow_duplicate_headers, configuration, created_at, updated_at, archived_at
FROM document_template_sheets
WHERE id = $1
LIMIT 1
`

func (q *Queries) GetDocumentTemplateSheetByID(ctx context.Context, id uuid.UUID) (DocumentTemplateSheet, error) {
	row := q.db.QueryRowContext(ctx, getDocumentTemplateSheetByID, id)
	var i DocumentTemplateSheet
	err := row.Scan(
		&i.ID,
		&i.TemplateID,
		&i.Code,
		&i.Name,
		&i.DisplayName,
		&i.Required,
		&i.SheetOrder,
		&i.HeaderRow,
		&i.StartRow,
		&i.AllowExtraColumns,
		&i.AllowDuplicateHeaders,
		&i.Configuration,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const listDocumentTemplateSheets = `-- name: ListDocumentTemplateSheets :many
SELECT id, template_id, code, name, display_name, required, sheet_order, header_row, start_row, allow_extra_columns, allow_duplicate_headers, configuration, created_at, updated_at, archived_at
FROM document_template_sheets
WHERE template_id = $1
  AND archived_at IS NULL
ORDER BY sheet_order NULLS LAST, created_at
`

func (q *Queries) ListDocumentTemplateSheets(ctx context.Context, templateID uuid.UUID) ([]DocumentTemplateSheet, error) {
	rows, err := q.db.QueryContext(ctx, listDocumentTemplateSheets, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplateSheet{}
	for rows.Next() {
		var i DocumentTemplateSheet
		if err := rows.Scan(
			&i.ID,
			&i.TemplateID,
			&i.Code,
			&i.Name,
			&i.DisplayName,
			&i.Required,
			&i.SheetOrder,
			&i.HeaderRow,
			&i.StartRow,
			&i.AllowExtraColumns,
			&i.AllowDuplicateHeaders,
			&i.Configuration,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ArchivedAt,
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

const listRequiredDocumentTemplateSheets = `-- name: ListRequiredDocumentTemplateSheets :many
SELECT id, template_id, code, name, display_name, required, sheet_order, header_row, start_row, allow_extra_columns, allow_duplicate_headers, configuration, created_at, updated_at, archived_at
FROM document_template_sheets
WHERE template_id = $1
  AND required = TRUE
  AND archived_at IS NULL
ORDER BY sheet_order NULLS LAST
`

func (q *Queries) ListRequiredDocumentTemplateSheets(ctx context.Context, templateID uuid.UUID) ([]DocumentTemplateSheet, error) {
	rows, err := q.db.QueryContext(ctx, listRequiredDocumentTemplateSheets, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplateSheet{}
	for rows.Next() {
		var i DocumentTemplateSheet
		if err := rows.Scan(
			&i.ID,
			&i.TemplateID,
			&i.Code,
			&i.Name,
			&i.DisplayName,
			&i.Required,
			&i.SheetOrder,
			&i.HeaderRow,
			&i.StartRow,
			&i.AllowExtraColumns,
			&i.AllowDuplicateHeaders,
			&i.Configuration,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.ArchivedAt,
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

const updateDocumentTemplateSheet = `-- name: UpdateDocumentTemplateSheet :one
UPDATE document_template_sheets
SET
    name = $2,
    display_name = $3,
    required = $4,
    sheet_order = $5,
    header_row = $6,
    start_row = $7,
    allow_extra_columns = $8,
    allow_duplicate_headers = $9,
    configuration = $10,
    updated_at = NOW()
WHERE id = $1
RETURNING id, template_id, code, name, display_name, required, sheet_order, header_row, start_row, allow_extra_columns, allow_duplicate_headers, configuration, created_at, updated_at, archived_at
`

type UpdateDocumentTemplateSheetParams struct {
	ID                    uuid.UUID       `json:"id"`
	Name                  string          `json:"name"`
	DisplayName           sql.NullString  `json:"display_name"`
	Required              bool            `json:"required"`
	SheetOrder            sql.NullInt32   `json:"sheet_order"`
	HeaderRow             int32           `json:"header_row"`
	StartRow              int32           `json:"start_row"`
	AllowExtraColumns     bool            `json:"allow_extra_columns"`
	AllowDuplicateHeaders bool            `json:"allow_duplicate_headers"`
	Configuration         json.RawMessage `json:"configuration"`
}

func (q *Queries) UpdateDocumentTemplateSheet(ctx context.Context, arg UpdateDocumentTemplateSheetParams) (DocumentTemplateSheet, error) {
	row := q.db.QueryRowContext(ctx, updateDocumentTemplateSheet,
		arg.ID,
		arg.Name,
		arg.DisplayName,
		arg.Required,
		arg.SheetOrder,
		arg.HeaderRow,
		arg.StartRow,
		arg.AllowExtraColumns,
		arg.AllowDuplicateHeaders,
		arg.Configuration,
	)
	var i DocumentTemplateSheet
	err := row.Scan(
		&i.ID,
		&i.TemplateID,
		&i.Code,
		&i.Name,
		&i.DisplayName,
		&i.Required,
		&i.SheetOrder,
		&i.HeaderRow,
		&i.StartRow,
		&i.AllowExtraColumns,
		&i.AllowDuplicateHeaders,
		&i.Configuration,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}
