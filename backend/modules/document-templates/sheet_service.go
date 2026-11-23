package documenttemplates

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates/internal/postgres"
)

var (
	ErrSheetCodeExists = errors.New("sheet code already exists in template")
	ErrSheetNameExists = errors.New("sheet name already exists in template")

	ErrInvalidHeaderRow = errors.New("header row must be >= 1")
	ErrInvalidStartRow  = errors.New("start row must be >= header row")
	ErrSheetNotFound    = errors.New("sheet not found in template")
)

type SheetService interface {
	CreateSheet(ctx context.Context, req CreateSheetRequest) (*DocumentTemplateSheet, error)
	GetSheet(ctx context.Context, id uuid.UUID) (*DocumentTemplateSheet, error)
	GetSheetByCode(ctx context.Context, templateID uuid.UUID, code string) (*DocumentTemplateSheet, error)
	ListSheets(ctx context.Context, templateID uuid.UUID) ([]DocumentTemplateSheet, error)
	ListRequiredSheets(ctx context.Context, templateID uuid.UUID) ([]DocumentTemplateSheet, error)
	UpdateSheet(ctx context.Context, req UpdateSheetRequest) (*DocumentTemplateSheet, error)
	ArchiveSheet(ctx context.Context, id uuid.UUID) error
	DeleteSheet(ctx context.Context, id uuid.UUID) error

	// runtime helpers
	GetSheetRuntime(
		ctx context.Context,
		tpl *TemplateRuntime,
		sheetCode string,
	) (*TemplateSheetRuntime, error)

	ValidateSheetExists(
		ctx context.Context,
		tpl *TemplateRuntime,
		sheetCode string,
	) error
}

type documentTemplateSheetService struct {
	repo SheetRepository
	alerts
}

func NewSheetService(
	repo SheetRepository,
	notifier modulekit.Notifier,
	platform modulekit.Platform,
) SheetService {
	return &documentTemplateSheetService{
		repo:   repo,
		alerts: alerts{notifier: notifier, platform: platform},
	}
}

/* =========================================================
 * CREATE
 * ========================================================= */

func (s *documentTemplateSheetService) CreateSheet(
	ctx context.Context,
	req CreateSheetRequest,
) (*DocumentTemplateSheet, error) {
	if s == nil {
		return nil, errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.TemplateID == uuid.Nil {
		return nil, errors.New("template id is required")
	}

	if req.Code == "" {
		return nil, errors.New("sheet code is required")
	}

	if req.Name == "" {
		return nil, errors.New("sheet name is required")
	}

	exists, err := s.repo.ExistsCode(ctx, req.TemplateID, req.Code)
	if err != nil {
		return nil, fmt.Errorf("check sheet code exists: %w", err)
	}
	if exists {
		return nil, ErrSheetCodeExists
	}

	nameExists, err := s.repo.ExistsName(ctx, req.TemplateID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check sheet name exists: %w", err)
	}
	if nameExists {
		return nil, ErrSheetNameExists
	}

	if req.HeaderRow < 1 {
		return nil, ErrInvalidHeaderRow
	}
	if req.StartRow < req.HeaderRow {
		return nil, ErrInvalidStartRow
	}

	var sheetOrder sql.NullInt32
	if req.SheetOrder != nil {
		sheetOrder = sql.NullInt32{
			Int32: int32(*req.SheetOrder),
			Valid: true,
		}
	}

	configJSON, err := json.Marshal(req.Configuration)
	if err != nil {
		return nil, fmt.Errorf("marshal sheet configuration: %w", err)
	}

	sheet, err := s.repo.Create(ctx, db.CreateDocumentTemplateSheetParams{
		ID:         uuid.New(),
		TemplateID: req.TemplateID,
		Code:       req.Code,
		Name:       req.Name,
		DisplayName: sql.NullString{
			String: req.DisplayName,
			Valid:  req.DisplayName != "",
		},
		Required:   req.Required,
		SheetOrder: sheetOrder,
		HeaderRow:  int32(req.HeaderRow),
		StartRow:   int32(req.StartRow),

		AllowExtraColumns:     req.AllowExtraColumns,
		AllowDuplicateHeaders: req.AllowDuplicateHeaders,

		Configuration: configJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create document template sheet: %w", err)
	}

	mapped := mapSheet(sheet)

	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_SHEET_CREATED",
		Title:      "Template sheet created",
		Severity:   "info",
		Message:    "Document template sheet created",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"sheet_id":                mapped.ID.String(),
			"template_id":             mapped.TemplateID.String(),
			"code":                    mapped.Code,
			"name":                    mapped.Name,
			"display_name":            mapped.DisplayName,
			"required":                mapped.Required,
			"sheet_order":             mapped.SheetOrder,
			"header_row":              mapped.HeaderRow,
			"start_row":               mapped.StartRow,
			"allow_extra_columns":     mapped.AllowExtraColumns,
			"allow_duplicate_headers": mapped.AllowDuplicateHeaders,
		}),
	})

	return mapped, nil
}

/* =========================================================
 * READ
 * ========================================================= */

func (s *documentTemplateSheetService) GetSheet(
	ctx context.Context,
	id uuid.UUID,
) (*DocumentTemplateSheet, error) {
	if s == nil {
		return nil, errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	if id == uuid.Nil {
		return nil, errors.New("sheet id is required")
	}

	sheet, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get document template sheet: %w", err)
	}

	return mapSheet(sheet), nil
}

func (s *documentTemplateSheetService) GetSheetByCode(
	ctx context.Context,
	templateID uuid.UUID,
	code string,
) (*DocumentTemplateSheet, error) {
	if s == nil {
		return nil, errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	if templateID == uuid.Nil {
		return nil, errors.New("template id is required")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("sheet code is required")
	}

	sheet, err := s.repo.GetByCode(ctx, templateID, code)
	if err != nil {
		return nil, fmt.Errorf("get document template sheet by code: %w", err)
	}

	return mapSheet(sheet), nil
}

func (s *documentTemplateSheetService) ListSheets(
	ctx context.Context,
	templateID uuid.UUID,
) ([]DocumentTemplateSheet, error) {
	if s == nil {
		return nil, errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	if templateID == uuid.Nil {
		return nil, errors.New("template id is required")
	}

	items, err := s.repo.List(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("list document template sheets: %w", err)
	}

	result := make([]DocumentTemplateSheet, 0, len(items))
	for _, it := range items {
		result = append(result, *mapSheet(it))
	}

	return result, nil
}

func (s *documentTemplateSheetService) ListRequiredSheets(
	ctx context.Context,
	templateID uuid.UUID,
) ([]DocumentTemplateSheet, error) {
	if s == nil {
		return nil, errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	if templateID == uuid.Nil {
		return nil, errors.New("template id is required")
	}

	items, err := s.repo.ListRequired(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("list required document template sheets: %w", err)
	}

	result := make([]DocumentTemplateSheet, 0, len(items))
	for _, it := range items {
		result = append(result, *mapSheet(it))
	}

	return result, nil
}

/* =========================================================
 * UPDATE
 * ========================================================= */

func (s *documentTemplateSheetService) UpdateSheet(
	ctx context.Context,
	req UpdateSheetRequest,
) (*DocumentTemplateSheet, error) {
	if s == nil {
		return nil, errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.ID == uuid.Nil {
		return nil, errors.New("sheet id is required")
	}

	if req.Name == "" {
		return nil, errors.New("sheet name is required")
	}

	if req.HeaderRow < 1 {
		return nil, ErrInvalidHeaderRow
	}
	if req.StartRow < req.HeaderRow {
		return nil, ErrInvalidStartRow
	}

	var sheetOrder sql.NullInt32
	if req.SheetOrder != nil {
		sheetOrder = sql.NullInt32{
			Int32: int32(*req.SheetOrder),
			Valid: true,
		}
	}

	configJSON, err := json.Marshal(req.Configuration)
	if err != nil {
		return nil, fmt.Errorf("marshal sheet configuration: %w", err)
	}

	sheet, err := s.repo.Update(ctx, db.UpdateDocumentTemplateSheetParams{
		ID:   req.ID,
		Name: req.Name,

		DisplayName: sql.NullString{
			String: req.DisplayName,
			Valid:  req.DisplayName != "",
		},

		Required:   req.Required,
		SheetOrder: sheetOrder,

		HeaderRow: int32(req.HeaderRow),
		StartRow:  int32(req.StartRow),

		AllowExtraColumns:     req.AllowExtraColumns,
		AllowDuplicateHeaders: req.AllowDuplicateHeaders,

		Configuration: configJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("update document template sheet: %w", err)
	}

	mapped := mapSheet(sheet)

	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_SHEET_UPDATED",
		Title:      "Template sheet updated",
		Severity:   "info",
		Message:    "Document template sheet updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"sheet_id":                mapped.ID.String(),
			"template_id":             mapped.TemplateID.String(),
			"code":                    mapped.Code,
			"name":                    mapped.Name,
			"display_name":            mapped.DisplayName,
			"required":                mapped.Required,
			"sheet_order":             mapped.SheetOrder,
			"header_row":              mapped.HeaderRow,
			"start_row":               mapped.StartRow,
			"allow_extra_columns":     mapped.AllowExtraColumns,
			"allow_duplicate_headers": mapped.AllowDuplicateHeaders,
		}),
	})

	return mapped, nil
}

/* =========================================================
 * DELETE / ARCHIVE
 * ========================================================= */

func (s *documentTemplateSheetService) ArchiveSheet(
	ctx context.Context,
	id uuid.UUID,
) error {
	if s == nil {
		return errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return errors.New("document template sheet repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("sheet id is required")
	}

	sheet, _ := s.repo.GetByID(ctx, id)

	if err := s.repo.Archive(ctx, id); err != nil {
		return fmt.Errorf("archive document template sheet: %w", err)
	}

	if sheet.ID != uuid.Nil {
		mapped := mapSheet(sheet)

		notification := modulekit.Notification{
			Type:       "DOCUMENT_TEMPLATE_SHEET_ARCHIVED",
			Title:      "Template sheet archived",
			Severity:   "warning",
			Message:    "Document template sheet archived",
			TargetRole: "admin",
			Metadata: mustJSON(map[string]any{
				"sheet_id":    mapped.ID.String(),
				"template_id": mapped.TemplateID.String(),
				"code":        mapped.Code,
				"name":        mapped.Name,
				"required":    mapped.Required,
			}),
		}

		s.attachAdminEmailDelivery(
			&notification,
			"document-template-sheet-archived",
			"Document template sheet archived",
			"A document template sheet was archived.",
			map[string]any{
				"Name":       s.systemAdminName(),
				"Platform":   s.platformName(),
				"SheetName":  mapped.Name,
				"SheetCode":  mapped.Code,
				"TemplateID": mapped.TemplateID.String(),
				"Required":   mapped.Required,
				"ActionURL":  s.adminTemplatesURL(),
				"Details": fmt.Sprintf(
					"Sheet ID: %s\nTemplate ID: %s\nCode: %s\nName: %s\nRequired: %v\nHeader Row: %d\nStart Row: %d",
					mapped.ID.String(),
					mapped.TemplateID.String(),
					mapped.Code,
					mapped.Name,
					mapped.Required,
					mapped.HeaderRow,
					mapped.StartRow,
				),
			},
		)

		s.notify(ctx, notification)
	}

	return nil
}

func (s *documentTemplateSheetService) DeleteSheet(
	ctx context.Context,
	id uuid.UUID,
) error {
	if s == nil {
		return errors.New("document template sheet service is nil")
	}

	if s.repo == nil {
		return errors.New("document template sheet repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("sheet id is required")
	}

	sheet, _ := s.repo.GetByID(ctx, id)

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete document template sheet: %w", err)
	}

	if sheet.ID != uuid.Nil {
		mapped := mapSheet(sheet)

		notification := modulekit.Notification{
			Type:       "DOCUMENT_TEMPLATE_SHEET_DELETED",
			Title:      "Template sheet deleted",
			Severity:   "critical",
			Message:    "Document template sheet deleted",
			TargetRole: "admin",
			Metadata: mustJSON(map[string]any{
				"sheet_id":    mapped.ID.String(),
				"template_id": mapped.TemplateID.String(),
				"code":        mapped.Code,
				"name":        mapped.Name,
				"required":    mapped.Required,
			}),
		}

		s.attachAdminEmailDelivery(
			&notification,
			"document-template-sheet-deleted",
			"Document template sheet deleted",
			"A document template sheet was deleted.",
			map[string]any{
				"Name":       s.systemAdminName(),
				"Platform":   s.platformName(),
				"SheetName":  mapped.Name,
				"SheetCode":  mapped.Code,
				"TemplateID": mapped.TemplateID.String(),
				"Required":   mapped.Required,
				"ActionURL":  s.adminTemplatesURL(),
				"Details": fmt.Sprintf(
					"Sheet ID: %s\nTemplate ID: %s\nCode: %s\nName: %s\nRequired: %v\nHeader Row: %d\nStart Row: %d",
					mapped.ID.String(),
					mapped.TemplateID.String(),
					mapped.Code,
					mapped.Name,
					mapped.Required,
					mapped.HeaderRow,
					mapped.StartRow,
				),
			},
		)

		s.notify(ctx, notification)
	}

	return nil
}

/* =========================================================
 * RUNTIME HELPERS
 * ========================================================= */

func (s *documentTemplateSheetService) GetSheetRuntime(
	ctx context.Context,
	tpl *TemplateRuntime,
	sheetCode string,
) (*TemplateSheetRuntime, error) {
	sheetCode = strings.TrimSpace(sheetCode)

	if tpl == nil || tpl.Sheets == nil {
		return nil, ErrSheetNotFound
	}

	sheet, exists := tpl.Sheets[sheetCode]
	if !exists {
		return nil, ErrSheetNotFound
	}

	return sheet, nil
}

func (s *documentTemplateSheetService) ValidateSheetExists(
	ctx context.Context,
	tpl *TemplateRuntime,
	sheetCode string,
) error {
	sheetCode = strings.TrimSpace(sheetCode)

	if tpl == nil || tpl.Sheets == nil {
		return ErrSheetNotFound
	}

	if _, exists := tpl.Sheets[sheetCode]; !exists {
		return ErrSheetNotFound
	}

	return nil
}

/* =========================================================
 * MAPPER / HELPERS
 * ========================================================= */

func mapSheet(s db.DocumentTemplateSheet) *DocumentTemplateSheet {
	var display string
	if s.DisplayName.Valid {
		display = s.DisplayName.String
	}

	var order *int
	if s.SheetOrder.Valid {
		v := int(s.SheetOrder.Int32)
		order = &v
	}

	var config map[string]any
	if len(s.Configuration) > 0 {
		_ = json.Unmarshal(s.Configuration, &config)
	}
	if config == nil {
		config = map[string]any{}
	}

	return &DocumentTemplateSheet{
		ID:                    s.ID,
		TemplateID:            s.TemplateID,
		Code:                  s.Code,
		Name:                  s.Name,
		DisplayName:           display,
		Required:              s.Required,
		SheetOrder:            order,
		HeaderRow:             int(s.HeaderRow),
		StartRow:              int(s.StartRow),
		AllowExtraColumns:     s.AllowExtraColumns,
		AllowDuplicateHeaders: s.AllowDuplicateHeaders,
		Configuration:         config,
	}
}
