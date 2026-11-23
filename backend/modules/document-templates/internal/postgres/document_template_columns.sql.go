// Copied from the host's sqlc output (internal/db/sqlc) when the feature
// moved into this module. The tables stay in the host's migrations.
// source: document_template_columns.sql

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

const archiveDocumentTemplateColumn = `-- name: ArchiveDocumentTemplateColumn :exec
UPDATE document_template_columns
SET
    archived_at = NOW(),
    updated_at = NOW()
WHERE id = $1
`

func (q *Queries) ArchiveDocumentTemplateColumn(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, archiveDocumentTemplateColumn, id)
	return err
}

const createDocumentTemplateColumn = `-- name: CreateDocumentTemplateColumn :one
INSERT INTO document_template_columns (
    id,
    sheet_id,
    column_key,
    column_name,
    display_name,
    data_type,
    required,
    is_unique,
    column_order,
    default_value,
    configuration,
    allowed_values,
    aliases
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
    $12,
    $13
)
RETURNING id, sheet_id, column_key, column_name, display_name, data_type, required, is_unique, column_order, default_value, configuration, allowed_values, aliases, created_at, updated_at, archived_at
`

type CreateDocumentTemplateColumnParams struct {
	ID            uuid.UUID                  `json:"id"`
	SheetID       uuid.UUID                  `json:"sheet_id"`
	ColumnKey     string                     `json:"column_key"`
	ColumnName    string                     `json:"column_name"`
	DisplayName   sql.NullString             `json:"display_name"`
	DataType      DocumentTemplateColumnType `json:"data_type"`
	Required      bool                       `json:"required"`
	IsUnique      bool                       `json:"is_unique"`
	ColumnOrder   sql.NullInt32              `json:"column_order"`
	DefaultValue  sql.NullString             `json:"default_value"`
	Configuration json.RawMessage            `json:"configuration"`
	AllowedValues json.RawMessage            `json:"allowed_values"`
	Aliases       json.RawMessage            `json:"aliases"`
}

func (q *Queries) CreateDocumentTemplateColumn(ctx context.Context, arg CreateDocumentTemplateColumnParams) (DocumentTemplateColumn, error) {
	row := q.db.QueryRowContext(ctx, createDocumentTemplateColumn,
		arg.ID,
		arg.SheetID,
		arg.ColumnKey,
		arg.ColumnName,
		arg.DisplayName,
		arg.DataType,
		arg.Required,
		arg.IsUnique,
		arg.ColumnOrder,
		arg.DefaultValue,
		arg.Configuration,
		arg.AllowedValues,
		arg.Aliases,
	)
	var i DocumentTemplateColumn
	err := row.Scan(
		&i.ID,
		&i.SheetID,
		&i.ColumnKey,
		&i.ColumnName,
		&i.DisplayName,
		&i.DataType,
		&i.Required,
		&i.IsUnique,
		&i.ColumnOrder,
		&i.DefaultValue,
		&i.Configuration,
		&i.AllowedValues,
		&i.Aliases,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const deleteDocumentTemplateColumn = `-- name: DeleteDocumentTemplateColumn :exec
DELETE FROM document_template_columns
WHERE id = $1
`

func (q *Queries) DeleteDocumentTemplateColumn(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, deleteDocumentTemplateColumn, id)
	return err
}

const existsDocumentTemplateColumnKey = `-- name: ExistsDocumentTemplateColumnKey :one
SELECT EXISTS (
    SELECT 1
    FROM document_template_columns
    WHERE sheet_id = $1
      AND column_key = $2
      AND archived_at IS NULL
)
`

type ExistsDocumentTemplateColumnKeyParams struct {
	SheetID   uuid.UUID `json:"sheet_id"`
	ColumnKey string    `json:"column_key"`
}

func (q *Queries) ExistsDocumentTemplateColumnKey(ctx context.Context, arg ExistsDocumentTemplateColumnKeyParams) (bool, error) {
	row := q.db.QueryRowContext(ctx, existsDocumentTemplateColumnKey, arg.SheetID, arg.ColumnKey)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const existsDocumentTemplateColumnName = `-- name: ExistsDocumentTemplateColumnName :one
SELECT EXISTS (
    SELECT 1
    FROM document_template_columns
    WHERE sheet_id = $1
      AND column_name = $2
      AND archived_at IS NULL
)
`

type ExistsDocumentTemplateColumnNameParams struct {
	SheetID    uuid.UUID `json:"sheet_id"`
	ColumnName string    `json:"column_name"`
}

func (q *Queries) ExistsDocumentTemplateColumnName(ctx context.Context, arg ExistsDocumentTemplateColumnNameParams) (bool, error) {
	row := q.db.QueryRowContext(ctx, existsDocumentTemplateColumnName, arg.SheetID, arg.ColumnName)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const getDocumentTemplateColumnByID = `-- name: GetDocumentTemplateColumnByID :one
SELECT id, sheet_id, column_key, column_name, display_name, data_type, required, is_unique, column_order, default_value, configuration, allowed_values, aliases, created_at, updated_at, archived_at
FROM document_template_columns
WHERE id = $1
LIMIT 1
`

func (q *Queries) GetDocumentTemplateColumnByID(ctx context.Context, id uuid.UUID) (DocumentTemplateColumn, error) {
	row := q.db.QueryRowContext(ctx, getDocumentTemplateColumnByID, id)
	var i DocumentTemplateColumn
	err := row.Scan(
		&i.ID,
		&i.SheetID,
		&i.ColumnKey,
		&i.ColumnName,
		&i.DisplayName,
		&i.DataType,
		&i.Required,
		&i.IsUnique,
		&i.ColumnOrder,
		&i.DefaultValue,
		&i.Configuration,
		&i.AllowedValues,
		&i.Aliases,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const getDocumentTemplateColumnByKey = `-- name: GetDocumentTemplateColumnByKey :one
SELECT id, sheet_id, column_key, column_name, display_name, data_type, required, is_unique, column_order, default_value, configuration, allowed_values, aliases, created_at, updated_at, archived_at
FROM document_template_columns
WHERE sheet_id = $1
  AND column_key = $2
  AND archived_at IS NULL
LIMIT 1
`

type GetDocumentTemplateColumnByKeyParams struct {
	SheetID   uuid.UUID `json:"sheet_id"`
	ColumnKey string    `json:"column_key"`
}

func (q *Queries) GetDocumentTemplateColumnByKey(ctx context.Context, arg GetDocumentTemplateColumnByKeyParams) (DocumentTemplateColumn, error) {
	row := q.db.QueryRowContext(ctx, getDocumentTemplateColumnByKey, arg.SheetID, arg.ColumnKey)
	var i DocumentTemplateColumn
	err := row.Scan(
		&i.ID,
		&i.SheetID,
		&i.ColumnKey,
		&i.ColumnName,
		&i.DisplayName,
		&i.DataType,
		&i.Required,
		&i.IsUnique,
		&i.ColumnOrder,
		&i.DefaultValue,
		&i.Configuration,
		&i.AllowedValues,
		&i.Aliases,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const listDocumentTemplateColumns = `-- name: ListDocumentTemplateColumns :many
SELECT id, sheet_id, column_key, column_name, display_name, data_type, required, is_unique, column_order, default_value, configuration, allowed_values, aliases, created_at, updated_at, archived_at
FROM document_template_columns
WHERE sheet_id = $1
  AND archived_at IS NULL
ORDER BY column_order NULLS LAST, created_at
`

func (q *Queries) ListDocumentTemplateColumns(ctx context.Context, sheetID uuid.UUID) ([]DocumentTemplateColumn, error) {
	rows, err := q.db.QueryContext(ctx, listDocumentTemplateColumns, sheetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplateColumn{}
	for rows.Next() {
		var i DocumentTemplateColumn
		if err := rows.Scan(
			&i.ID,
			&i.SheetID,
			&i.ColumnKey,
			&i.ColumnName,
			&i.DisplayName,
			&i.DataType,
			&i.Required,
			&i.IsUnique,
			&i.ColumnOrder,
			&i.DefaultValue,
			&i.Configuration,
			&i.AllowedValues,
			&i.Aliases,
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

const listRequiredDocumentTemplateColumns = `-- name: ListRequiredDocumentTemplateColumns :many
SELECT id, sheet_id, column_key, column_name, display_name, data_type, required, is_unique, column_order, default_value, configuration, allowed_values, aliases, created_at, updated_at, archived_at
FROM document_template_columns
WHERE sheet_id = $1
  AND required = TRUE
  AND archived_at IS NULL
ORDER BY column_order NULLS LAST
`

func (q *Queries) ListRequiredDocumentTemplateColumns(ctx context.Context, sheetID uuid.UUID) ([]DocumentTemplateColumn, error) {
	rows, err := q.db.QueryContext(ctx, listRequiredDocumentTemplateColumns, sheetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplateColumn{}
	for rows.Next() {
		var i DocumentTemplateColumn
		if err := rows.Scan(
			&i.ID,
			&i.SheetID,
			&i.ColumnKey,
			&i.ColumnName,
			&i.DisplayName,
			&i.DataType,
			&i.Required,
			&i.IsUnique,
			&i.ColumnOrder,
			&i.DefaultValue,
			&i.Configuration,
			&i.AllowedValues,
			&i.Aliases,
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

const listUniqueDocumentTemplateColumns = `-- name: ListUniqueDocumentTemplateColumns :many
SELECT id, sheet_id, column_key, column_name, display_name, data_type, required, is_unique, column_order, default_value, configuration, allowed_values, aliases, created_at, updated_at, archived_at
FROM document_template_columns
WHERE sheet_id = $1
  AND is_unique = TRUE
  AND archived_at IS NULL
ORDER BY column_order NULLS LAST
`

func (q *Queries) ListUniqueDocumentTemplateColumns(ctx context.Context, sheetID uuid.UUID) ([]DocumentTemplateColumn, error) {
	rows, err := q.db.QueryContext(ctx, listUniqueDocumentTemplateColumns, sheetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplateColumn{}
	for rows.Next() {
		var i DocumentTemplateColumn
		if err := rows.Scan(
			&i.ID,
			&i.SheetID,
			&i.ColumnKey,
			&i.ColumnName,
			&i.DisplayName,
			&i.DataType,
			&i.Required,
			&i.IsUnique,
			&i.ColumnOrder,
			&i.DefaultValue,
			&i.Configuration,
			&i.AllowedValues,
			&i.Aliases,
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

const updateDocumentTemplateColumn = `-- name: UpdateDocumentTemplateColumn :one
UPDATE document_template_columns
SET
    column_name = $2,
    display_name = $3,
    data_type = $4,
    required = $5,
    is_unique = $6,
    column_order = $7,
    default_value = $8,
    configuration = $9,
    allowed_values = $10,
    aliases = $11,
    updated_at = NOW()
WHERE id = $1
RETURNING id, sheet_id, column_key, column_name, display_name, data_type, required, is_unique, column_order, default_value, configuration, allowed_values, aliases, created_at, updated_at, archived_at
`

type UpdateDocumentTemplateColumnParams struct {
	ID            uuid.UUID                  `json:"id"`
	ColumnName    string                     `json:"column_name"`
	DisplayName   sql.NullString             `json:"display_name"`
	DataType      DocumentTemplateColumnType `json:"data_type"`
	Required      bool                       `json:"required"`
	IsUnique      bool                       `json:"is_unique"`
	ColumnOrder   sql.NullInt32              `json:"column_order"`
	DefaultValue  sql.NullString             `json:"default_value"`
	Configuration json.RawMessage            `json:"configuration"`
	AllowedValues json.RawMessage            `json:"allowed_values"`
	Aliases       json.RawMessage            `json:"aliases"`
}

func (q *Queries) UpdateDocumentTemplateColumn(ctx context.Context, arg UpdateDocumentTemplateColumnParams) (DocumentTemplateColumn, error) {
	row := q.db.QueryRowContext(ctx, updateDocumentTemplateColumn,
		arg.ID,
		arg.ColumnName,
		arg.DisplayName,
		arg.DataType,
		arg.Required,
		arg.IsUnique,
		arg.ColumnOrder,
		arg.DefaultValue,
		arg.Configuration,
		arg.AllowedValues,
		arg.Aliases,
	)
	var i DocumentTemplateColumn
	err := row.Scan(
		&i.ID,
		&i.SheetID,
		&i.ColumnKey,
		&i.ColumnName,
		&i.DisplayName,
		&i.DataType,
		&i.Required,
		&i.IsUnique,
		&i.ColumnOrder,
		&i.DefaultValue,
		&i.Configuration,
		&i.AllowedValues,
		&i.Aliases,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}
