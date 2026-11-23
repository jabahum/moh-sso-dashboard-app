// Package postgres holds the module's SQL, copied from the host's sqlc
// output for the document_templates, document_template_sheets and
// document_template_columns tables.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type Queries struct {
	db DBTX
}

type DocumentTemplateColumnType string

const (
	DocumentTemplateColumnTypeSTRING   DocumentTemplateColumnType = "STRING"
	DocumentTemplateColumnTypeTEXT     DocumentTemplateColumnType = "TEXT"
	DocumentTemplateColumnTypeINTEGER  DocumentTemplateColumnType = "INTEGER"
	DocumentTemplateColumnTypeDECIMAL  DocumentTemplateColumnType = "DECIMAL"
	DocumentTemplateColumnTypeBOOLEAN  DocumentTemplateColumnType = "BOOLEAN"
	DocumentTemplateColumnTypeDATE     DocumentTemplateColumnType = "DATE"
	DocumentTemplateColumnTypeDATETIME DocumentTemplateColumnType = "DATETIME"
	DocumentTemplateColumnTypeTIME     DocumentTemplateColumnType = "TIME"
	DocumentTemplateColumnTypeENUM     DocumentTemplateColumnType = "ENUM"
	DocumentTemplateColumnTypeUUID     DocumentTemplateColumnType = "UUID"
	DocumentTemplateColumnTypeEMAIL    DocumentTemplateColumnType = "EMAIL"
	DocumentTemplateColumnTypePHONE    DocumentTemplateColumnType = "PHONE"
	DocumentTemplateColumnTypeJSON     DocumentTemplateColumnType = "JSON"
)

func (e *DocumentTemplateColumnType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = DocumentTemplateColumnType(s)
	case string:
		*e = DocumentTemplateColumnType(s)
	default:
		return fmt.Errorf("unsupported scan type for DocumentTemplateColumnType: %T", src)
	}
	return nil
}

type DocumentTemplate struct {
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
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ArchivedAt    sql.NullTime    `json:"archived_at"`
}

type DocumentTemplateColumn struct {
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
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
	ArchivedAt    sql.NullTime               `json:"archived_at"`
}

type DocumentTemplateSheet struct {
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
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	ArchivedAt            sql.NullTime    `json:"archived_at"`
}
