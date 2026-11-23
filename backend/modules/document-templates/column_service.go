package documenttemplates

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates/internal/postgres"
)

var (
	ErrColumnKeyExists  = errors.New("column key already exists")
	ErrColumnNameExists = errors.New("column name already exists")
	ErrInvalidDataType  = errors.New("invalid column data type")
)

type ColumnService interface {
	CreateColumn(ctx context.Context, req CreateColumnRequest) (*DocumentTemplateColumn, error)
	GetColumn(ctx context.Context, id uuid.UUID) (*DocumentTemplateColumn, error)
	ListColumns(ctx context.Context, sheetID uuid.UUID) ([]DocumentTemplateColumn, error)
	ListRequiredColumns(ctx context.Context, sheetID uuid.UUID) ([]DocumentTemplateColumn, error)
	ListUniqueColumns(ctx context.Context, sheetID uuid.UUID) ([]DocumentTemplateColumn, error)
	UpdateColumn(ctx context.Context, req UpdateColumnRequest) (*DocumentTemplateColumn, error)
	ArchiveColumn(ctx context.Context, id uuid.UUID) error
	DeleteColumn(ctx context.Context, id uuid.UUID) error

	// runtime processing
	Handle(ctx context.Context, col DocumentTemplateColumn, value any) (*ProcessedColumnValue, error)
}

type documentTemplateColumnService struct {
	repo ColumnRepository
	alerts
}

func NewColumnService(
	repo ColumnRepository,
	notifier modulekit.Notifier,
	platform modulekit.Platform,
) ColumnService {
	return &documentTemplateColumnService{
		repo:   repo,
		alerts: alerts{notifier: notifier, platform: platform},
	}
}

/* =========================================================
 * CRUD
 * ========================================================= */

func (s *documentTemplateColumnService) CreateColumn(
	ctx context.Context,
	req CreateColumnRequest,
) (*DocumentTemplateColumn, error) {
	if s == nil {
		return nil, errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	req.ColumnKey = strings.TrimSpace(req.ColumnKey)
	req.ColumnName = strings.TrimSpace(req.ColumnName)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.DataType = strings.ToUpper(strings.TrimSpace(req.DataType))

	if req.SheetID == uuid.Nil {
		return nil, errors.New("sheet id is required")
	}

	if req.ColumnKey == "" {
		return nil, errors.New("column key is required")
	}

	if req.ColumnName == "" {
		return nil, errors.New("column name is required")
	}

	exists, err := s.repo.ExistsKey(ctx, req.SheetID, req.ColumnKey)
	if err != nil {
		return nil, fmt.Errorf("check column key exists: %w", err)
	}
	if exists {
		return nil, ErrColumnKeyExists
	}

	nameExists, err := s.repo.ExistsName(ctx, req.SheetID, req.ColumnName)
	if err != nil {
		return nil, fmt.Errorf("check column name exists: %w", err)
	}
	if nameExists {
		return nil, ErrColumnNameExists
	}

	if err := validateDataType(req.DataType); err != nil {
		return nil, err
	}

	col, err := s.repo.Create(ctx, db.CreateDocumentTemplateColumnParams{
		ID:         uuid.New(),
		SheetID:    req.SheetID,
		ColumnKey:  req.ColumnKey,
		ColumnName: req.ColumnName,
		DisplayName: sql.NullString{
			String: req.DisplayName,
			Valid:  req.DisplayName != "",
		},
		DataType: db.DocumentTemplateColumnType(req.DataType),
		Required: req.Required,
		IsUnique: req.IsUnique,
		DefaultValue: sql.NullString{
			String: stringPtrValue(req.DefaultValue),
			Valid:  req.DefaultValue != nil,
		},
		AllowedValues: toJSON(req.AllowedValues),
		Aliases:       toJSON(req.Aliases),
		Configuration: toJSON(req.Configuration),
	})
	if err != nil {
		return nil, fmt.Errorf("create document template column: %w", err)
	}

	mapped := mapColumn(col)

	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_COLUMN_CREATED",
		Title:      "Template column created",
		Severity:   "info",
		Message:    "Document template column created",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"column_id":    mapped.ID.String(),
			"sheet_id":     mapped.SheetID.String(),
			"column_key":   mapped.ColumnKey,
			"column_name":  mapped.ColumnName,
			"display_name": mapped.DisplayName,
			"data_type":    mapped.DataType,
			"required":     mapped.Required,
			"is_unique":    mapped.IsUnique,
		}),
	})

	return mapped, nil
}

func (s *documentTemplateColumnService) GetColumn(
	ctx context.Context,
	id uuid.UUID,
) (*DocumentTemplateColumn, error) {
	if s == nil {
		return nil, errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	if id == uuid.Nil {
		return nil, errors.New("column id is required")
	}

	col, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get document template column: %w", err)
	}

	return mapColumn(col), nil
}

func (s *documentTemplateColumnService) ListColumns(
	ctx context.Context,
	sheetID uuid.UUID,
) ([]DocumentTemplateColumn, error) {
	if s == nil {
		return nil, errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	if sheetID == uuid.Nil {
		return nil, errors.New("sheet id is required")
	}

	items, err := s.repo.List(ctx, sheetID)
	if err != nil {
		return nil, fmt.Errorf("list document template columns: %w", err)
	}

	out := make([]DocumentTemplateColumn, 0, len(items))
	for _, c := range items {
		out = append(out, *mapColumn(c))
	}

	return out, nil
}

func (s *documentTemplateColumnService) ListRequiredColumns(
	ctx context.Context,
	sheetID uuid.UUID,
) ([]DocumentTemplateColumn, error) {
	if s == nil {
		return nil, errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	if sheetID == uuid.Nil {
		return nil, errors.New("sheet id is required")
	}

	items, err := s.repo.ListRequired(ctx, sheetID)
	if err != nil {
		return nil, fmt.Errorf("list required document template columns: %w", err)
	}

	out := make([]DocumentTemplateColumn, 0, len(items))
	for _, c := range items {
		out = append(out, *mapColumn(c))
	}

	return out, nil
}

func (s *documentTemplateColumnService) ListUniqueColumns(
	ctx context.Context,
	sheetID uuid.UUID,
) ([]DocumentTemplateColumn, error) {
	if s == nil {
		return nil, errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	if sheetID == uuid.Nil {
		return nil, errors.New("sheet id is required")
	}

	items, err := s.repo.ListUnique(ctx, sheetID)
	if err != nil {
		return nil, fmt.Errorf("list unique document template columns: %w", err)
	}

	out := make([]DocumentTemplateColumn, 0, len(items))
	for _, c := range items {
		out = append(out, *mapColumn(c))
	}

	return out, nil
}

func (s *documentTemplateColumnService) UpdateColumn(
	ctx context.Context,
	req UpdateColumnRequest,
) (*DocumentTemplateColumn, error) {
	if s == nil {
		return nil, errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	req.ColumnName = strings.TrimSpace(req.ColumnName)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.DataType = strings.ToUpper(strings.TrimSpace(req.DataType))

	if req.ID == uuid.Nil {
		return nil, errors.New("column id is required")
	}

	if req.ColumnName == "" {
		return nil, errors.New("column name is required")
	}

	if err := validateDataType(req.DataType); err != nil {
		return nil, err
	}

	col, err := s.repo.Update(ctx, db.UpdateDocumentTemplateColumnParams{
		ID:         req.ID,
		ColumnName: req.ColumnName,
		DisplayName: sql.NullString{
			String: req.DisplayName,
			Valid:  req.DisplayName != "",
		},
		DataType: db.DocumentTemplateColumnType(req.DataType),
		Required: req.Required,
		IsUnique: req.IsUnique,
		DefaultValue: sql.NullString{
			String: stringPtrValue(req.DefaultValue),
			Valid:  req.DefaultValue != nil,
		},
		AllowedValues: toJSON(req.AllowedValues),
		Aliases:       toJSON(req.Aliases),
		Configuration: toJSON(req.Configuration),
	})
	if err != nil {
		return nil, fmt.Errorf("update document template column: %w", err)
	}

	mapped := mapColumn(col)

	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_COLUMN_UPDATED",
		Title:      "Template column updated",
		Severity:   "info",
		Message:    "Document template column updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"column_id":    mapped.ID.String(),
			"sheet_id":     mapped.SheetID.String(),
			"column_name":  mapped.ColumnName,
			"display_name": mapped.DisplayName,
			"data_type":    mapped.DataType,
			"required":     mapped.Required,
			"is_unique":    mapped.IsUnique,
		}),
	})

	return mapped, nil
}

func (s *documentTemplateColumnService) ArchiveColumn(ctx context.Context, id uuid.UUID) error {
	if s == nil {
		return errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return errors.New("document template column repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("column id is required")
	}

	col, _ := s.repo.GetByID(ctx, id)

	if err := s.repo.Archive(ctx, id); err != nil {
		return fmt.Errorf("archive document template column: %w", err)
	}

	if col.ID != uuid.Nil {
		mapped := mapColumn(col)

		notification := modulekit.Notification{
			Type:       "DOCUMENT_TEMPLATE_COLUMN_ARCHIVED",
			Title:      "Template column archived",
			Severity:   "warning",
			Message:    "Document template column archived",
			TargetRole: "admin",
			Metadata: mustJSON(map[string]any{
				"column_id":   mapped.ID.String(),
				"sheet_id":    mapped.SheetID.String(),
				"column_key":  mapped.ColumnKey,
				"column_name": mapped.ColumnName,
				"data_type":   mapped.DataType,
			}),
		}

		s.attachAdminEmailDelivery(
			&notification,
			"document-template-column-archived",
			"Document template column archived",
			"A document template column was archived.",
			map[string]any{
				"Name":       s.systemAdminName(),
				"Platform":   s.platformName(),
				"ColumnName": mapped.ColumnName,
				"ColumnKey":  mapped.ColumnKey,
				"DataType":   mapped.DataType,
				"ActionURL":  s.adminTemplatesURL(),
				"Details": fmt.Sprintf(
					"Column ID: %s\nSheet ID: %s\nColumn Key: %s\nColumn Name: %s\nData Type: %s",
					mapped.ID.String(),
					mapped.SheetID.String(),
					mapped.ColumnKey,
					mapped.ColumnName,
					mapped.DataType,
				),
			},
		)

		s.notify(ctx, notification)
	}

	return nil
}

func (s *documentTemplateColumnService) DeleteColumn(ctx context.Context, id uuid.UUID) error {
	if s == nil {
		return errors.New("document template column service is nil")
	}

	if s.repo == nil {
		return errors.New("document template column repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("column id is required")
	}

	col, _ := s.repo.GetByID(ctx, id)

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete document template column: %w", err)
	}

	if col.ID != uuid.Nil {
		mapped := mapColumn(col)

		notification := modulekit.Notification{
			Type:       "DOCUMENT_TEMPLATE_COLUMN_DELETED",
			Title:      "Template column deleted",
			Severity:   "critical",
			Message:    "Document template column deleted",
			TargetRole: "admin",
			Metadata: mustJSON(map[string]any{
				"column_id":   mapped.ID.String(),
				"sheet_id":    mapped.SheetID.String(),
				"column_key":  mapped.ColumnKey,
				"column_name": mapped.ColumnName,
				"data_type":   mapped.DataType,
			}),
		}

		s.attachAdminEmailDelivery(
			&notification,
			"document-template-column-deleted",
			"Document template column deleted",
			"A document template column was deleted.",
			map[string]any{
				"Name":       s.systemAdminName(),
				"Platform":   s.platformName(),
				"ColumnName": mapped.ColumnName,
				"ColumnKey":  mapped.ColumnKey,
				"DataType":   mapped.DataType,
				"ActionURL":  s.adminTemplatesURL(),
				"Details": fmt.Sprintf(
					"Column ID: %s\nSheet ID: %s\nColumn Key: %s\nColumn Name: %s\nData Type: %s",
					mapped.ID.String(),
					mapped.SheetID.String(),
					mapped.ColumnKey,
					mapped.ColumnName,
					mapped.DataType,
				),
			},
		)

		s.notify(ctx, notification)
	}

	return nil
}

/* =========================================================
 * RUNTIME HANDLER
 * ========================================================= */

func (s *documentTemplateColumnService) Handle(
	ctx context.Context,
	col DocumentTemplateColumn,
	value any,
) (*ProcessedColumnValue, error) {
	result := &ProcessedColumnValue{
		ColumnKey: col.ColumnKey,
		RawValue:  value,
		IsValid:   true,
		Value:     value,
	}

	if isEmptyValue(value) {
		if col.Required {
			result.IsValid = false
			result.Error = &ColumnError{
				Code:    "REQUIRED_FIELD",
				Message: col.ColumnName + " is required",
			}
		}

		if col.DefaultValue != nil {
			result.Value = *col.DefaultValue
		}

		return result, nil
	}

	switch strings.ToUpper(strings.TrimSpace(col.DataType)) {
	case "STRING", "TEXT":
		result.Value = fmt.Sprintf("%v", value)

	case "INTEGER":
		v, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprintf("%v", value)), 10, 64)
		if err != nil {
			result.IsValid = false
			result.Error = &ColumnError{
				Code:    "INVALID_INTEGER",
				Message: col.ColumnName + " must be a valid integer",
			}
			return result, nil
		}
		result.Value = v

	case "DECIMAL":
		v, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprintf("%v", value)), 64)
		if err != nil {
			result.IsValid = false
			result.Error = &ColumnError{
				Code:    "INVALID_DECIMAL",
				Message: col.ColumnName + " must be a valid decimal number",
			}
			return result, nil
		}
		result.Value = v

	case "BOOLEAN":
		v, ok := parseBool(value)
		if !ok {
			result.IsValid = false
			result.Error = &ColumnError{
				Code:    "INVALID_BOOLEAN",
				Message: col.ColumnName + " must be a valid boolean value",
			}
			return result, nil
		}
		result.Value = v

	case "ENUM":
		str := strings.TrimSpace(fmt.Sprintf("%v", value))
		valid := false
		for _, v := range col.AllowedValues {
			if strings.EqualFold(strings.TrimSpace(v), str) {
				valid = true
				str = v
				break
			}
		}
		if !valid {
			result.IsValid = false
			result.Error = &ColumnError{
				Code:    "INVALID_ENUM_VALUE",
				Message: col.ColumnName + " has an unsupported value",
			}
			return result, nil
		}
		result.Value = str

	case "DATE", "DATETIME", "TIME", "UUID", "EMAIL", "PHONE", "JSON":
		// Keep these as raw/string for now. Add stricter validators later if needed.
		result.Value = fmt.Sprintf("%v", value)

	default:
		result.IsValid = false
		result.Error = &ColumnError{
			Code:    "INVALID_TYPE",
			Message: "unsupported type",
		}
	}

	return result, nil
}

func validateDataType(t string) error {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "STRING", "TEXT", "INTEGER", "DECIMAL",
		"BOOLEAN", "DATE", "DATETIME", "TIME",
		"ENUM", "UUID", "EMAIL", "PHONE", "JSON":
		return nil
	default:
		return ErrInvalidDataType
	}
}

func mapColumn(c db.DocumentTemplateColumn) *DocumentTemplateColumn {
	return &DocumentTemplateColumn{
		ID:          c.ID,
		SheetID:     c.SheetID,
		ColumnKey:   c.ColumnKey,
		ColumnName:  c.ColumnName,
		DisplayName: toString(c.DisplayName),
		DataType:    string(c.DataType),
		Required:    c.Required,
		IsUnique:    c.IsUnique,

		DefaultValue: nullStringToPtr(c.DefaultValue),

		AllowedValues: mustSliceString(c.AllowedValues),
		Aliases:       mustSliceString(c.Aliases),

		Configuration: mustMap(c.Configuration),
	}
}

func toString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}

	return ""
}

func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}

	v := ns.String
	return &v
}

func mustSliceString(v json.RawMessage) []string {
	if len(v) == 0 {
		return nil
	}

	var arr []string
	_ = json.Unmarshal(v, &arr)

	return arr
}

func mustMap(v json.RawMessage) map[string]any {
	if len(v) == 0 {
		return map[string]any{}
	}

	var m map[string]any
	_ = json.Unmarshal(v, &m)

	if m == nil {
		return map[string]any{}
	}

	return m
}

func toJSON(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage([]byte(`null`))
	}

	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage([]byte(`null`))
	}

	return b
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}

	str, ok := value.(string)
	if !ok {
		return false
	}

	return strings.TrimSpace(str) == ""
}

func parseBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return false, false
		}
	default:
		str := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", value)))
		switch str {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return false, false
		}
	}
}
