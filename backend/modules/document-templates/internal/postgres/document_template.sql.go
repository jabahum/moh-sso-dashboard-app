// Copied from the host's sqlc output (internal/db/sqlc) when the feature
// moved into this module. The tables stay in the host's migrations.
// source: document_template.sql

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

const archiveDocumentTemplate = `-- name: ArchiveDocumentTemplate :exec
UPDATE document_templates
SET archived_at = NOW()
WHERE id = $1
`

func (q *Queries) ArchiveDocumentTemplate(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, archiveDocumentTemplate, id)
	return err
}

const createDocumentTemplate = `-- name: CreateDocumentTemplate :one
INSERT INTO document_templates (
    id,
    document_id,
    code,
    name,
    description,
    file_type,
    version,
    is_active,
    configuration,
    created_by
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
    $10
)
RETURNING id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
`

type CreateDocumentTemplateParams struct {
	ID            uuid.UUID       `json:"id"`
	DocumentID    uuid.NullUUID   `json:"document_id"`
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	Description   sql.NullString  `json:"description"`
	FileType      string          `json:"file_type"`
	Version       int32           `json:"version"`
	IsActive      bool            `json:"is_active"`
	Configuration json.RawMessage `json:"configuration"`
	CreatedBy     uuid.UUID       `json:"created_by"`
}

func (q *Queries) CreateDocumentTemplate(ctx context.Context, arg CreateDocumentTemplateParams) (DocumentTemplate, error) {
	row := q.db.QueryRowContext(ctx, createDocumentTemplate,
		arg.ID,
		arg.DocumentID,
		arg.Code,
		arg.Name,
		arg.Description,
		arg.FileType,
		arg.Version,
		arg.IsActive,
		arg.Configuration,
		arg.CreatedBy,
	)
	var i DocumentTemplate
	err := row.Scan(
		&i.ID,
		&i.DocumentID,
		&i.Code,
		&i.Name,
		&i.Description,
		&i.FileType,
		&i.Version,
		&i.IsActive,
		&i.Configuration,
		&i.CreatedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const deleteDocumentTemplate = `-- name: DeleteDocumentTemplate :exec
DELETE FROM document_templates
WHERE id = $1
`

func (q *Queries) DeleteDocumentTemplate(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, deleteDocumentTemplate, id)
	return err
}

const existsDocumentTemplateCode = `-- name: ExistsDocumentTemplateCode :one
SELECT EXISTS (
    SELECT 1
    FROM document_templates
    WHERE code = $1
)
`

func (q *Queries) ExistsDocumentTemplateCode(ctx context.Context, code string) (bool, error) {
	row := q.db.QueryRowContext(ctx, existsDocumentTemplateCode, code)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

const getDocumentTemplateByCode = `-- name: GetDocumentTemplateByCode :one
SELECT id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
FROM document_templates
WHERE code = $1
  AND archived_at IS NULL
ORDER BY version DESC
LIMIT 1
`

func (q *Queries) GetDocumentTemplateByCode(ctx context.Context, code string) (DocumentTemplate, error) {
	row := q.db.QueryRowContext(ctx, getDocumentTemplateByCode, code)
	var i DocumentTemplate
	err := row.Scan(
		&i.ID,
		&i.DocumentID,
		&i.Code,
		&i.Name,
		&i.Description,
		&i.FileType,
		&i.Version,
		&i.IsActive,
		&i.Configuration,
		&i.CreatedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const getDocumentTemplateByDocumentID = `-- name: GetDocumentTemplateByDocumentID :one
SELECT id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
FROM document_templates
WHERE document_id = $1
  AND archived_at IS NULL
LIMIT 1
`

func (q *Queries) GetDocumentTemplateByDocumentID(ctx context.Context, documentID uuid.NullUUID) (DocumentTemplate, error) {
	row := q.db.QueryRowContext(ctx, getDocumentTemplateByDocumentID, documentID)
	var i DocumentTemplate
	err := row.Scan(
		&i.ID,
		&i.DocumentID,
		&i.Code,
		&i.Name,
		&i.Description,
		&i.FileType,
		&i.Version,
		&i.IsActive,
		&i.Configuration,
		&i.CreatedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const getDocumentTemplateByID = `-- name: GetDocumentTemplateByID :one
SELECT id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
FROM document_templates
WHERE id = $1
LIMIT 1
`

func (q *Queries) GetDocumentTemplateByID(ctx context.Context, id uuid.UUID) (DocumentTemplate, error) {
	row := q.db.QueryRowContext(ctx, getDocumentTemplateByID, id)
	var i DocumentTemplate
	err := row.Scan(
		&i.ID,
		&i.DocumentID,
		&i.Code,
		&i.Name,
		&i.Description,
		&i.FileType,
		&i.Version,
		&i.IsActive,
		&i.Configuration,
		&i.CreatedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}

const listActiveDocumentTemplates = `-- name: ListActiveDocumentTemplates :many
SELECT id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
FROM document_templates
WHERE is_active = TRUE
  AND archived_at IS NULL
ORDER BY name
`

func (q *Queries) ListActiveDocumentTemplates(ctx context.Context) ([]DocumentTemplate, error) {
	rows, err := q.db.QueryContext(ctx, listActiveDocumentTemplates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplate{}
	for rows.Next() {
		var i DocumentTemplate
		if err := rows.Scan(
			&i.ID,
			&i.DocumentID,
			&i.Code,
			&i.Name,
			&i.Description,
			&i.FileType,
			&i.Version,
			&i.IsActive,
			&i.Configuration,
			&i.CreatedBy,
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

const listDocumentTemplateVersions = `-- name: ListDocumentTemplateVersions :many
SELECT id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
FROM document_templates
WHERE code = $1
ORDER BY version DESC
`

func (q *Queries) ListDocumentTemplateVersions(ctx context.Context, code string) ([]DocumentTemplate, error) {
	rows, err := q.db.QueryContext(ctx, listDocumentTemplateVersions, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplate{}
	for rows.Next() {
		var i DocumentTemplate
		if err := rows.Scan(
			&i.ID,
			&i.DocumentID,
			&i.Code,
			&i.Name,
			&i.Description,
			&i.FileType,
			&i.Version,
			&i.IsActive,
			&i.Configuration,
			&i.CreatedBy,
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

const listDocumentTemplates = `-- name: ListDocumentTemplates :many
SELECT id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
FROM document_templates
WHERE archived_at IS NULL
ORDER BY created_at DESC
`

func (q *Queries) ListDocumentTemplates(ctx context.Context) ([]DocumentTemplate, error) {
	rows, err := q.db.QueryContext(ctx, listDocumentTemplates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DocumentTemplate{}
	for rows.Next() {
		var i DocumentTemplate
		if err := rows.Scan(
			&i.ID,
			&i.DocumentID,
			&i.Code,
			&i.Name,
			&i.Description,
			&i.FileType,
			&i.Version,
			&i.IsActive,
			&i.Configuration,
			&i.CreatedBy,
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

const updateDocumentTemplate = `-- name: UpdateDocumentTemplate :one
UPDATE document_templates
SET
    name = $2,
    description = $3,
    file_type = $4,
    configuration = $5,
    is_active = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING id, document_id, code, name, description, file_type, version, is_active, configuration, created_by, created_at, updated_at, archived_at
`

type UpdateDocumentTemplateParams struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Description   sql.NullString  `json:"description"`
	FileType      string          `json:"file_type"`
	Configuration json.RawMessage `json:"configuration"`
	IsActive      bool            `json:"is_active"`
}

func (q *Queries) UpdateDocumentTemplate(ctx context.Context, arg UpdateDocumentTemplateParams) (DocumentTemplate, error) {
	row := q.db.QueryRowContext(ctx, updateDocumentTemplate,
		arg.ID,
		arg.Name,
		arg.Description,
		arg.FileType,
		arg.Configuration,
		arg.IsActive,
	)
	var i DocumentTemplate
	err := row.Scan(
		&i.ID,
		&i.DocumentID,
		&i.Code,
		&i.Name,
		&i.Description,
		&i.FileType,
		&i.Version,
		&i.IsActive,
		&i.Configuration,
		&i.CreatedBy,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ArchivedAt,
	)
	return i, err
}
