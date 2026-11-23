package documenttemplates

import "github.com/google/uuid"

//
// =====================================================
// TEMPLATE DOMAIN MODELS
// =====================================================
//

type DocumentTemplate struct {
	ID            uuid.UUID      `json:"id"`
	DocumentID    *uuid.UUID     `json:"document_id,omitempty"`
	Code          string         `json:"code"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	FileType      string         `json:"file_type"`
	Version       int            `json:"version"`
	IsActive      bool           `json:"is_active"`
	Configuration map[string]any `json:"configuration"`
	CreatedBy     uuid.UUID      `json:"created_by"`
	CreatedAt     string         `json:"created_at,omitempty"`
	UpdatedAt     string         `json:"updated_at,omitempty"`
	ArchivedAt    *string        `json:"archived_at,omitempty"`
}

type DocumentTemplateSheet struct {
	ID                    uuid.UUID      `json:"id"`
	TemplateID            uuid.UUID      `json:"template_id"`
	Code                  string         `json:"code"`
	Name                  string         `json:"name"`
	DisplayName           string         `json:"display_name"`
	Required              bool           `json:"required"`
	SheetOrder            *int           `json:"sheet_order"`
	HeaderRow             int            `json:"header_row"`
	StartRow              int            `json:"start_row"`
	AllowExtraColumns     bool           `json:"allow_extra_columns"`
	AllowDuplicateHeaders bool           `json:"allow_duplicate_headers"`
	Configuration         map[string]any `json:"configuration"`
	CreatedAt             string         `json:"created_at,omitempty"`
	UpdatedAt             string         `json:"updated_at,omitempty"`
	ArchivedAt            *string        `json:"archived_at,omitempty"`
}

type DocumentTemplateColumn struct {
	ID            uuid.UUID      `json:"id"`
	SheetID       uuid.UUID      `json:"sheet_id"`
	ColumnKey     string         `json:"column_key"`
	ColumnName    string         `json:"column_name"`
	DisplayName   string         `json:"display_name"`
	DataType      string         `json:"data_type"`
	Required      bool           `json:"required"`
	IsUnique      bool           `json:"is_unique"`
	ColumnOrder   *int           `json:"column_order"`
	DefaultValue  *string        `json:"default_value"`
	AllowedValues []string       `json:"allowed_values"`
	Aliases       []string       `json:"aliases"`
	Configuration map[string]any `json:"configuration"`
	CreatedAt     string         `json:"created_at,omitempty"`
	UpdatedAt     string         `json:"updated_at,omitempty"`
	ArchivedAt    *string        `json:"archived_at,omitempty"`
}

//
// =====================================================
// TEMPLATE RUNTIME MODELS
// =====================================================
//

type TemplateRuntime struct {
	ID       uuid.UUID                        `json:"id"`
	Code     string                           `json:"code"`
	Version  int                              `json:"version"`
	IsActive bool                             `json:"is_active"`
	Sheets   map[string]*TemplateSheetRuntime `json:"sheets"`
}

type TemplateSheetRuntime struct {
	ID                    uuid.UUID                         `json:"id"`
	Code                  string                            `json:"code"`
	Name                  string                            `json:"name"`
	DisplayName           string                            `json:"display_name"`
	Required              bool                              `json:"required"`
	SheetOrder            *int                              `json:"sheet_order"`
	HeaderRow             int                               `json:"header_row"`
	StartRow              int                               `json:"start_row"`
	AllowExtraColumns     bool                              `json:"allow_extra_columns"`
	AllowDuplicateHeaders bool                              `json:"allow_duplicate_headers"`
	Configuration         map[string]any                    `json:"configuration"`
	Columns               map[string]*TemplateColumnRuntime `json:"columns"`
}

type TemplateColumnRuntime struct {
	ID            uuid.UUID      `json:"id"`
	ColumnKey     string         `json:"column_key"`
	ColumnName    string         `json:"column_name"`
	DisplayName   string         `json:"display_name"`
	DataType      string         `json:"data_type"`
	Required      bool           `json:"required"`
	IsUnique      bool           `json:"is_unique"`
	ColumnOrder   *int           `json:"column_order"`
	DefaultValue  *string        `json:"default_value"`
	AllowedValues []string       `json:"allowed_values"`
	Aliases       []string       `json:"aliases"`
	Configuration map[string]any `json:"configuration"`
}

//
// =====================================================
// TEMPLATE STRUCTURE DTOs
// =====================================================
//

type TemplateStructure struct {
	Template DocumentTemplate         `json:"template"`
	Sheets   []TemplateSheetStructure `json:"sheets"`
}

type TemplateSheetStructure struct {
	ID                    uuid.UUID                 `json:"id"`
	TemplateID            uuid.UUID                 `json:"template_id"`
	Code                  string                    `json:"code"`
	Name                  string                    `json:"name"`
	DisplayName           string                    `json:"display_name"`
	Required              bool                      `json:"required"`
	SheetOrder            *int                      `json:"sheet_order"`
	HeaderRow             int                       `json:"header_row"`
	StartRow              int                       `json:"start_row"`
	AllowExtraColumns     bool                      `json:"allow_extra_columns"`
	AllowDuplicateHeaders bool                      `json:"allow_duplicate_headers"`
	Configuration         map[string]any            `json:"configuration"`
	Columns               []TemplateColumnStructure `json:"columns"`
}

type TemplateColumnStructure struct {
	ID            uuid.UUID      `json:"id"`
	SheetID       uuid.UUID      `json:"sheet_id"`
	ColumnKey     string         `json:"column_key"`
	ColumnName    string         `json:"column_name"`
	DisplayName   string         `json:"display_name"`
	DataType      string         `json:"data_type"`
	Required      bool           `json:"required"`
	IsUnique      bool           `json:"is_unique"`
	ColumnOrder   *int           `json:"column_order"`
	DefaultValue  *string        `json:"default_value"`
	AllowedValues []string       `json:"allowed_values"`
	Aliases       []string       `json:"aliases"`
	Configuration map[string]any `json:"configuration"`
}

//
// =====================================================
// INGESTION / VALIDATION
// =====================================================
//

type ProcessedColumnValue struct {
	ColumnKey string       `json:"column_key"`
	RawValue  any          `json:"raw_value"`
	Value     any          `json:"value"`
	IsValid   bool         `json:"is_valid"`
	Error     *ColumnError `json:"error,omitempty"`
}

type ColumnError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	InvalidValue any    `json:"invalid_value"`
}

//
// =====================================================
// REQUEST MODELS
// =====================================================
//

type CreateTemplateRequest struct {
	DocumentID    *uuid.UUID     `json:"document_id,omitempty"`
	Code          string         `json:"code"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	FileType      string         `json:"file_type"`
	CreatedBy     uuid.UUID      `json:"created_by"`
	Configuration map[string]any `json:"configuration"`
}

type UpdateTemplateRequest struct {
	ID            uuid.UUID      `json:"id"`
	DocumentID    *uuid.UUID     `json:"document_id,omitempty"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	FileType      string         `json:"file_type"`
	IsActive      bool           `json:"is_active"`
	Configuration map[string]any `json:"configuration"`
}

type CreateSheetRequest struct {
	TemplateID            uuid.UUID      `json:"template_id"`
	Code                  string         `json:"code"`
	Name                  string         `json:"name"`
	DisplayName           string         `json:"display_name"`
	Required              bool           `json:"required"`
	SheetOrder            *int           `json:"sheet_order"`
	HeaderRow             int            `json:"header_row"`
	StartRow              int            `json:"start_row"`
	AllowExtraColumns     bool           `json:"allow_extra_columns"`
	AllowDuplicateHeaders bool           `json:"allow_duplicate_headers"`
	Configuration         map[string]any `json:"configuration"`
}

type UpdateSheetRequest struct {
	ID                    uuid.UUID      `json:"id"`
	Name                  string         `json:"name"`
	DisplayName           string         `json:"display_name"`
	Required              bool           `json:"required"`
	SheetOrder            *int           `json:"sheet_order"`
	HeaderRow             int            `json:"header_row"`
	StartRow              int            `json:"start_row"`
	AllowExtraColumns     bool           `json:"allow_extra_columns"`
	AllowDuplicateHeaders bool           `json:"allow_duplicate_headers"`
	Configuration         map[string]any `json:"configuration"`
}

type CreateColumnRequest struct {
	SheetID       uuid.UUID      `json:"sheet_id"`
	ColumnKey     string         `json:"column_key"`
	ColumnName    string         `json:"column_name"`
	DisplayName   string         `json:"display_name"`
	DataType      string         `json:"data_type"`
	Required      bool           `json:"required"`
	IsUnique      bool           `json:"is_unique"`
	ColumnOrder   *int           `json:"column_order"`
	DefaultValue  *string        `json:"default_value"`
	AllowedValues []string       `json:"allowed_values"`
	Aliases       []string       `json:"aliases"`
	Configuration map[string]any `json:"configuration"`
}

type UpdateColumnRequest struct {
	ID            uuid.UUID      `json:"id"`
	ColumnKey     string         `json:"column_key"`
	ColumnName    string         `json:"column_name"`
	DisplayName   string         `json:"display_name"`
	DataType      string         `json:"data_type"`
	Required      bool           `json:"required"`
	IsUnique      bool           `json:"is_unique"`
	ColumnOrder   *int           `json:"column_order"`
	DefaultValue  *string        `json:"default_value"`
	AllowedValues []string       `json:"allowed_values"`
	Aliases       []string       `json:"aliases"`
	Configuration map[string]any `json:"configuration"`
}

type CreateTemplateSheetWithColumnsRequest struct {
	Sheet   CreateSheetRequest    `json:"sheet"`
	Columns []CreateColumnRequest `json:"columns"`
}

type CreateTemplateStructureRequest struct {
	Template CreateTemplateRequest                   `json:"template"`
	Sheets   []CreateTemplateSheetWithColumnsRequest `json:"sheets"`
}
