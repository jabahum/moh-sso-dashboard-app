package documenttemplates

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates/internal/postgres"
)

var (
	ErrTemplateCodeExists = errors.New("template code already exists")
)

type Service interface {
	CreateTemplate(ctx context.Context, req CreateTemplateRequest) (*DocumentTemplate, error)

	GetTemplate(ctx context.Context, id uuid.UUID) (*DocumentTemplate, error)

	CreateTemplateStructure(
		ctx context.Context,
		req CreateTemplateStructureRequest,
	) (*TemplateStructure, error)

	GetTemplateRuntime(ctx context.Context, code string) (*TemplateRuntime, error)

	GetTemplateByCode(ctx context.Context, code string) (*DocumentTemplate, error)

	ListTemplates(ctx context.Context) ([]DocumentTemplate, error)

	ListActiveTemplates(ctx context.Context) ([]DocumentTemplate, error)

	UpdateTemplate(ctx context.Context, req UpdateTemplateRequest) (*DocumentTemplate, error)

	DeleteTemplate(ctx context.Context, id uuid.UUID) error

	ArchiveTemplate(ctx context.Context, id uuid.UUID) error

	PublishTemplate(ctx context.Context, id uuid.UUID) error

	GetTemplateStructure(ctx context.Context, code string) (*TemplateStructure, error)

	GetTemplateStructureByDocumentID(
		ctx context.Context,
		documentID uuid.UUID,
	) (*TemplateStructure, error)

	ReplaceStructure(
		ctx context.Context,
		templateID uuid.UUID,
		sheets []CreateTemplateSheetWithColumnsRequest,
	) (*TemplateStructure, error)
}

type documentTemplateService struct {
	documentTemplateRepo Repository
	sheetRepo            SheetRepository
	columnRepo           ColumnRepository
	alerts
}

func NewService(
	documentTemplateRepo Repository,
	sheetRepo SheetRepository,
	columnRepo ColumnRepository,
	notifier modulekit.Notifier,
	platform modulekit.Platform,
) Service {
	return &documentTemplateService{
		documentTemplateRepo: documentTemplateRepo,
		sheetRepo:            sheetRepo,
		columnRepo:           columnRepo,
		alerts:               alerts{notifier: notifier, platform: platform},
	}
}

func (s *documentTemplateService) GetTemplate(
	ctx context.Context,
	id uuid.UUID,
) (*DocumentTemplate, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	if id == uuid.Nil {
		return nil, errors.New("template id is required")
	}

	t, err := s.documentTemplateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get document template: %w", err)
	}

	return mapDocumentTemplate(t), nil
}

func (s *documentTemplateService) CreateTemplate(
	ctx context.Context,
	req CreateTemplateRequest,
) (*DocumentTemplate, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.FileType = strings.TrimSpace(req.FileType)

	if req.Code == "" {
		return nil, errors.New("template code is required")
	}

	if req.Name == "" {
		return nil, errors.New("template name is required")
	}

	if req.FileType == "" {
		return nil, errors.New("template file type is required")
	}

	exists, err := s.documentTemplateRepo.ExistsCode(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("check template code exists: %w", err)
	}

	if exists {
		return nil, ErrTemplateCodeExists
	}

	template, err := s.documentTemplateRepo.Create(ctx, db.CreateDocumentTemplateParams{
		ID:   uuid.New(),
		Code: req.Code,
		Name: req.Name,
		Description: sql.NullString{
			String: req.Description,
			Valid:  req.Description != "",
		},
		FileType:      req.FileType,
		Version:       1,
		IsActive:      false,
		Configuration: toJSON(req.Configuration),
		CreatedBy:     req.CreatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create document template: %w", err)
	}

	out := mapDocumentTemplate(template)

	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_CREATED",
		Title:      "Document template created",
		Severity:   "info",
		Message:    "Document template created",
		TargetRole: "admin",
		UserID:     req.CreatedBy.String(),
		Metadata: mustJSON(map[string]any{
			"template_id": out.ID.String(),
			"code":        out.Code,
			"name":        out.Name,
			"file_type":   out.FileType,
			"version":     out.Version,
			"is_active":   out.IsActive,
			"created_by":  req.CreatedBy.String(),
		}),
	})

	return out, nil
}

func (s *documentTemplateService) GetTemplateRuntime(
	ctx context.Context,
	code string,
) (*TemplateRuntime, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("template code is required")
	}

	t, err := s.documentTemplateRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get template runtime: %w", err)
	}

	return &TemplateRuntime{
		ID:      t.ID,
		Code:    t.Code,
		Version: int(t.Version),
	}, nil
}

func (s *documentTemplateService) GetTemplateByCode(
	ctx context.Context,
	code string,
) (*DocumentTemplate, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("template code is required")
	}

	t, err := s.documentTemplateRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get document template by code: %w", err)
	}

	return mapDocumentTemplate(t), nil
}

func (s *documentTemplateService) ListTemplates(ctx context.Context) ([]DocumentTemplate, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	items, err := s.documentTemplateRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list document templates: %w", err)
	}

	result := make([]DocumentTemplate, 0, len(items))
	for _, t := range items {
		result = append(result, *mapDocumentTemplate(t))
	}

	return result, nil
}

func (s *documentTemplateService) ListActiveTemplates(ctx context.Context) ([]DocumentTemplate, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	items, err := s.documentTemplateRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active document templates: %w", err)
	}

	result := make([]DocumentTemplate, 0, len(items))
	for _, t := range items {
		result = append(result, *mapDocumentTemplate(t))
	}

	return result, nil
}

func (s *documentTemplateService) PublishTemplate(
	ctx context.Context,
	id uuid.UUID,
) error {
	if s == nil {
		return errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return errors.New("document template repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("template id is required")
	}

	t, err := s.documentTemplateRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get template before publish: %w", err)
	}

	versions, err := s.documentTemplateRepo.ListVersions(ctx, t.Code)
	if err != nil {
		return fmt.Errorf("list template versions before publish: %w", err)
	}

	for _, v := range versions {
		_, err := s.documentTemplateRepo.Update(ctx, db.UpdateDocumentTemplateParams{
			ID:            v.ID,
			Name:          v.Name,
			Description:   v.Description,
			FileType:      v.FileType,
			Configuration: v.Configuration,
			IsActive:      false,
		})
		if err != nil {
			return fmt.Errorf("deactivate template version %s: %w", v.ID.String(), err)
		}
	}

	_, err = s.documentTemplateRepo.Update(ctx, db.UpdateDocumentTemplateParams{
		ID:            t.ID,
		Name:          t.Name,
		Description:   t.Description,
		FileType:      t.FileType,
		Configuration: t.Configuration,
		IsActive:      true,
	})
	if err != nil {
		return fmt.Errorf("publish document template: %w", err)
	}

	notification := modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_PUBLISHED",
		Title:      "Document template published",
		Severity:   "info",
		Message:    "Document template published and activated",
		TargetRole: "admin",
		UserID:     t.CreatedBy.String(),
		Metadata: mustJSON(map[string]any{
			"template_id": t.ID.String(),
			"code":        t.Code,
			"name":        t.Name,
			"file_type":   t.FileType,
			"version":     int(t.Version),
			"is_active":   true,
			"created_by":  t.CreatedBy.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"document-template-published",
		"Document template published",
		fmt.Sprintf("Document template %s has been published and activated.", t.Name),
		map[string]any{
			"Name":         s.systemAdminName(),
			"Platform":     s.platformName(),
			"TemplateName": t.Name,
			"TemplateCode": t.Code,
			"FileType":     t.FileType,
			"Version":      int(t.Version),
			"ActionURL":    s.adminTemplatesURL(),
			"Details": fmt.Sprintf(
				"Template ID: %s\nCode: %s\nName: %s\nFile Type: %s\nVersion: %d\nCreated By: %s",
				t.ID.String(),
				t.Code,
				t.Name,
				t.FileType,
				int(t.Version),
				t.CreatedBy.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

func (s *documentTemplateService) ArchiveTemplate(
	ctx context.Context,
	id uuid.UUID,
) error {
	if s == nil {
		return errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return errors.New("document template repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("template id is required")
	}

	t, err := s.documentTemplateRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get template before archive: %w", err)
	}

	if err := s.documentTemplateRepo.Archive(ctx, id); err != nil {
		return fmt.Errorf("archive document template: %w", err)
	}

	notification := modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_ARCHIVED",
		Title:      "Document template archived",
		Severity:   "warning",
		Message:    "Document template archived",
		TargetRole: "admin",
		UserID:     t.CreatedBy.String(),
		Metadata: mustJSON(map[string]any{
			"template_id": t.ID.String(),
			"code":        t.Code,
			"name":        t.Name,
			"file_type":   t.FileType,
			"version":     int(t.Version),
			"is_active":   t.IsActive,
			"created_by":  t.CreatedBy.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"document-template-archived",
		"Document template archived",
		fmt.Sprintf("Document template %s has been archived.", t.Name),
		map[string]any{
			"Name":         s.systemAdminName(),
			"Platform":     s.platformName(),
			"TemplateName": t.Name,
			"TemplateCode": t.Code,
			"FileType":     t.FileType,
			"Version":      int(t.Version),
			"ActionURL":    s.adminTemplatesURL(),
			"Details": fmt.Sprintf(
				"Template ID: %s\nCode: %s\nName: %s\nFile Type: %s\nVersion: %d\nWas Active: %v\nCreated By: %s",
				t.ID.String(),
				t.Code,
				t.Name,
				t.FileType,
				int(t.Version),
				t.IsActive,
				t.CreatedBy.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

func (s *documentTemplateService) GetTemplateStructure(
	ctx context.Context,
	code string,
) (*TemplateStructure, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	if s.sheetRepo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	if s.columnRepo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("template code is required")
	}

	t, err := s.documentTemplateRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get template by code: %w", err)
	}

	sheets, err := s.sheetRepo.List(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("list template sheets: %w", err)
	}

	structure := &TemplateStructure{
		Template: DocumentTemplate{
			ID:            t.ID,
			DocumentID:    nullUUIDPtr(t.DocumentID),
			Code:          t.Code,
			Name:          t.Name,
			Description:   nullStringValue(t.Description),
			FileType:      t.FileType,
			Version:       int(t.Version),
			IsActive:      t.IsActive,
			Configuration: fromJSON(t.Configuration),
			CreatedBy:     t.CreatedBy,
			CreatedAt:     t.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     t.UpdatedAt.Format(time.RFC3339),
			ArchivedAt:    nullTimeStringPtr(t.ArchivedAt),
		},
		Sheets: make([]TemplateSheetStructure, 0, len(sheets)),
	}

	for _, sh := range sheets {
		cols, err := s.columnRepo.List(ctx, sh.ID)
		if err != nil {
			return nil, fmt.Errorf("list template sheet columns: %w", err)
		}

		sheet := TemplateSheetStructure{
			ID:                    sh.ID,
			TemplateID:            sh.TemplateID,
			Code:                  sh.Code,
			Name:                  sh.Name,
			DisplayName:           nullStringValue(sh.DisplayName),
			Required:              sh.Required,
			SheetOrder:            nullInt32Ptr(sh.SheetOrder),
			HeaderRow:             int(sh.HeaderRow),
			StartRow:              int(sh.StartRow),
			AllowExtraColumns:     sh.AllowExtraColumns,
			AllowDuplicateHeaders: sh.AllowDuplicateHeaders,
			Configuration:         fromJSON(sh.Configuration),
			Columns:               make([]TemplateColumnStructure, 0, len(cols)),
		}

		for _, c := range cols {
			sheet.Columns = append(sheet.Columns, TemplateColumnStructure{
				ID:            c.ID,
				SheetID:       c.SheetID,
				ColumnKey:     c.ColumnKey,
				ColumnName:    c.ColumnName,
				DisplayName:   nullStringValue(c.DisplayName),
				DataType:      string(c.DataType),
				Required:      c.Required,
				IsUnique:      c.IsUnique,
				ColumnOrder:   nullInt32Ptr(c.ColumnOrder),
				AllowedValues: fromJSONStringArray(c.AllowedValues),
				Aliases:       fromJSONStringArray(c.Aliases),
				Configuration: fromJSON(c.Configuration),
			})
		}

		structure.Sheets = append(structure.Sheets, sheet)
	}

	return structure, nil
}

func (s *documentTemplateService) GetTemplateStructureByDocumentID(
	ctx context.Context,
	documentID uuid.UUID,
) (*TemplateStructure, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	if documentID == uuid.Nil {
		return nil, errors.New("document id is required")
	}

	t, err := s.documentTemplateRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("get template by document id: %w", err)
	}

	return s.GetTemplateStructure(ctx, t.Code)
}

func (s *documentTemplateService) UpdateTemplate(
	ctx context.Context,
	req UpdateTemplateRequest,
) (*DocumentTemplate, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	if req.ID == uuid.Nil {
		return nil, errors.New("template id is required")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.FileType = strings.TrimSpace(req.FileType)

	if req.Name == "" {
		return nil, errors.New("template name is required")
	}

	if req.FileType == "" {
		return nil, errors.New("template file type is required")
	}

	t, err := s.documentTemplateRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("get template before update: %w", err)
	}

	configJSON, err := json.Marshal(req.Configuration)
	if err != nil {
		return nil, fmt.Errorf("marshal template configuration: %w", err)
	}

	updated, err := s.documentTemplateRepo.Update(ctx, db.UpdateDocumentTemplateParams{
		ID:   t.ID,
		Name: req.Name,
		Description: sql.NullString{
			String: req.Description,
			Valid:  req.Description != "",
		},
		FileType:      req.FileType,
		Configuration: configJSON,
		IsActive:      req.IsActive,
	})
	if err != nil {
		return nil, fmt.Errorf("update document template: %w", err)
	}

	out := mapDocumentTemplate(updated)

	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_UPDATED",
		Title:      "Document template updated",
		Severity:   "info",
		Message:    "Document template updated",
		TargetRole: "admin",
		UserID:     updated.CreatedBy.String(),
		Metadata: mustJSON(map[string]any{
			"template_id": out.ID.String(),
			"code":        out.Code,
			"name":        out.Name,
			"file_type":   out.FileType,
			"version":     out.Version,
			"is_active":   out.IsActive,
			"created_by":  updated.CreatedBy.String(),
		}),
	})

	return out, nil
}

func (s *documentTemplateService) DeleteTemplate(
	ctx context.Context,
	id uuid.UUID,
) error {
	if s == nil {
		return errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return errors.New("document template repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("template id is required")
	}

	t, err := s.documentTemplateRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get template before delete: %w", err)
	}

	versions, err := s.documentTemplateRepo.ListVersions(ctx, t.Code)
	if err != nil {
		return fmt.Errorf("list template versions before delete: %w", err)
	}

	for _, v := range versions {
		err := s.documentTemplateRepo.Delete(ctx, v.ID)
		if err != nil {
			return fmt.Errorf("delete template version %s: %w", v.ID.String(), err)
		}
	}

	notification := modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_DELETED",
		Title:      "Document template deleted",
		Severity:   "critical",
		Message:    "Document template and all its versions deleted",
		TargetRole: "admin",
		UserID:     t.CreatedBy.String(),
		Metadata: mustJSON(map[string]any{
			"template_id":      t.ID.String(),
			"code":             t.Code,
			"name":             t.Name,
			"file_type":        t.FileType,
			"version":          int(t.Version),
			"versions_deleted": len(versions),
			"created_by":       t.CreatedBy.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"document-template-deleted",
		"Document template deleted",
		fmt.Sprintf("Document template %s and all its versions were deleted.", t.Name),
		map[string]any{
			"Name":         s.systemAdminName(),
			"Platform":     s.platformName(),
			"TemplateName": t.Name,
			"TemplateCode": t.Code,
			"FileType":     t.FileType,
			"Version":      int(t.Version),
			"ActionURL":    s.adminTemplatesURL(),
			"Details": fmt.Sprintf(
				"Template ID: %s\nCode: %s\nName: %s\nFile Type: %s\nVersion: %d\nVersions Deleted: %d\nCreated By: %s",
				t.ID.String(),
				t.Code,
				t.Name,
				t.FileType,
				int(t.Version),
				len(versions),
				t.CreatedBy.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

func (s *documentTemplateService) CreateTemplateStructure(
	ctx context.Context,
	req CreateTemplateStructureRequest,
) (*TemplateStructure, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if s.documentTemplateRepo == nil {
		return nil, errors.New("document template repository is nil")
	}

	if s.sheetRepo == nil {
		return nil, errors.New("document template sheet repository is nil")
	}

	if s.columnRepo == nil {
		return nil, errors.New("document template column repository is nil")
	}

	req.Template.Code = strings.TrimSpace(req.Template.Code)
	req.Template.Name = strings.TrimSpace(req.Template.Name)
	req.Template.Description = strings.TrimSpace(req.Template.Description)
	req.Template.FileType = strings.TrimSpace(req.Template.FileType)

	if req.Template.Code == "" {
		return nil, errors.New("template code is required")
	}

	if req.Template.Name == "" {
		return nil, errors.New("template name is required")
	}

	if req.Template.FileType == "" {
		return nil, errors.New("template file type is required")
	}

	exists, err := s.documentTemplateRepo.ExistsCode(ctx, req.Template.Code)
	if err != nil {
		return nil, fmt.Errorf("check template code exists: %w", err)
	}

	if exists {
		return nil, ErrTemplateCodeExists
	}

	template, err := s.documentTemplateRepo.Create(ctx, db.CreateDocumentTemplateParams{
		ID: uuid.New(),
		DocumentID: uuid.NullUUID{
			UUID: func() uuid.UUID {
				if req.Template.DocumentID == nil {
					return uuid.Nil
				}

				return *req.Template.DocumentID
			}(),
			Valid: req.Template.DocumentID != nil,
		},
		Code: req.Template.Code,
		Name: req.Template.Name,
		Description: sql.NullString{
			String: req.Template.Description,
			Valid:  req.Template.Description != "",
		},
		FileType:      req.Template.FileType,
		Version:       1,
		IsActive:      false,
		Configuration: toJSON(req.Template.Configuration),
		CreatedBy:     req.Template.CreatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create template structure template: %w", err)
	}

	sheets := make([]TemplateSheetStructure, 0, len(req.Sheets))

	for _, sheetReq := range req.Sheets {
		sheetInput := sheetReq.Sheet

		sheetInput.TemplateID = template.ID
		sheetInput.Code = strings.TrimSpace(sheetInput.Code)
		sheetInput.Name = strings.TrimSpace(sheetInput.Name)
		sheetInput.DisplayName = strings.TrimSpace(sheetInput.DisplayName)

		if sheetInput.Code == "" {
			return nil, errors.New("sheet code is required")
		}

		if sheetInput.Name == "" {
			return nil, errors.New("sheet name is required")
		}

		sheetOrder := sql.NullInt32{}
		if sheetInput.SheetOrder != nil {
			sheetOrder = sql.NullInt32{
				Int32: int32(*sheetInput.SheetOrder),
				Valid: true,
			}
		}

		sheet, err := s.sheetRepo.Create(ctx, db.CreateDocumentTemplateSheetParams{
			ID:         uuid.New(),
			TemplateID: sheetInput.TemplateID,
			Code:       sheetInput.Code,
			Name:       sheetInput.Name,
			DisplayName: sql.NullString{
				String: sheetInput.DisplayName,
				Valid:  sheetInput.DisplayName != "",
			},
			Required:              sheetInput.Required,
			SheetOrder:            sheetOrder,
			HeaderRow:             int32(sheetInput.HeaderRow),
			StartRow:              int32(sheetInput.StartRow),
			AllowExtraColumns:     sheetInput.AllowExtraColumns,
			AllowDuplicateHeaders: sheetInput.AllowDuplicateHeaders,
			Configuration:         toJSON(sheetInput.Configuration),
		})
		if err != nil {
			return nil, fmt.Errorf("create template structure sheet: %w", err)
		}

		columnStructures := make([]TemplateColumnStructure, 0, len(sheetReq.Columns))

		for _, columnReq := range sheetReq.Columns {
			columnReq.SheetID = sheet.ID
			columnReq.ColumnKey = strings.TrimSpace(columnReq.ColumnKey)
			columnReq.ColumnName = strings.TrimSpace(columnReq.ColumnName)
			columnReq.DisplayName = strings.TrimSpace(columnReq.DisplayName)

			if columnReq.ColumnKey == "" {
				return nil, errors.New("column key is required")
			}

			if columnReq.ColumnName == "" {
				return nil, errors.New("column name is required")
			}

			columnOrder := sql.NullInt32{}
			if columnReq.ColumnOrder != nil {
				columnOrder = sql.NullInt32{
					Int32: int32(*columnReq.ColumnOrder),
					Valid: true,
				}
			}

			column, err := s.columnRepo.Create(ctx, db.CreateDocumentTemplateColumnParams{
				ID:         uuid.New(),
				SheetID:    columnReq.SheetID,
				ColumnKey:  columnReq.ColumnKey,
				ColumnName: columnReq.ColumnName,
				DisplayName: sql.NullString{
					String: columnReq.DisplayName,
					Valid:  columnReq.DisplayName != "",
				},
				DataType:    normalizeColumnDataType(columnReq.DataType),
				Required:    columnReq.Required,
				IsUnique:    columnReq.IsUnique,
				ColumnOrder: columnOrder,
				DefaultValue: sql.NullString{
					String: stringPtrValue(columnReq.DefaultValue),
					Valid:  columnReq.DefaultValue != nil,
				},
				Configuration: toJSON(columnReq.Configuration),
				AllowedValues: toJSON(columnReq.AllowedValues),
				Aliases:       toJSON(columnReq.Aliases),
			})
			if err != nil {
				return nil, fmt.Errorf("create template structure column: %w", err)
			}

			columnStructures = append(columnStructures, TemplateColumnStructure{
				ID:            column.ID,
				SheetID:       column.SheetID,
				ColumnKey:     column.ColumnKey,
				ColumnName:    column.ColumnName,
				DisplayName:   nullStringValue(column.DisplayName),
				DataType:      string(column.DataType),
				Required:      column.Required,
				IsUnique:      column.IsUnique,
				ColumnOrder:   nullInt32Ptr(column.ColumnOrder),
				AllowedValues: fromJSONStringArray(column.AllowedValues),
				Aliases:       fromJSONStringArray(column.Aliases),
				Configuration: fromJSON(column.Configuration),
			})
		}

		sheets = append(sheets, TemplateSheetStructure{
			ID:                    sheet.ID,
			TemplateID:            sheet.TemplateID,
			Code:                  sheet.Code,
			Name:                  sheet.Name,
			DisplayName:           nullStringValue(sheet.DisplayName),
			Required:              sheet.Required,
			SheetOrder:            nullInt32Ptr(sheet.SheetOrder),
			HeaderRow:             int(sheet.HeaderRow),
			StartRow:              int(sheet.StartRow),
			AllowExtraColumns:     sheet.AllowExtraColumns,
			AllowDuplicateHeaders: sheet.AllowDuplicateHeaders,
			Configuration:         fromJSON(sheet.Configuration),
			Columns:               columnStructures,
		})
	}

	structure := &TemplateStructure{
		Template: DocumentTemplate{
			ID:            template.ID,
			DocumentID:    nullUUIDPtr(template.DocumentID),
			Code:          template.Code,
			Name:          template.Name,
			Description:   nullStringValue(template.Description),
			FileType:      template.FileType,
			Version:       int(template.Version),
			IsActive:      template.IsActive,
			Configuration: fromJSON(template.Configuration),
			CreatedBy:     template.CreatedBy,
			CreatedAt:     template.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     template.UpdatedAt.Format(time.RFC3339),
			ArchivedAt:    nullTimeStringPtr(template.ArchivedAt),
		},
		Sheets: sheets,
	}

	columnsCount := countTemplateStructureColumns(sheets)

	notification := modulekit.Notification{
		Type:       "DOCUMENT_TEMPLATE_STRUCTURE_CREATED",
		Title:      "Document template structure created",
		Severity:   "info",
		Message:    "Document template structure created",
		TargetRole: "admin",
		UserID:     template.CreatedBy.String(),
		Metadata: mustJSON(map[string]any{
			"template_id":     template.ID.String(),
			"document_id":     uuidPtrString(nullUUIDPtr(template.DocumentID)),
			"code":            template.Code,
			"name":            template.Name,
			"file_type":       template.FileType,
			"version":         int(template.Version),
			"sheets_count":    len(sheets),
			"columns_count":   columnsCount,
			"created_by":      template.CreatedBy.String(),
			"has_document_id": template.DocumentID.Valid,
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"document-template-structure-created",
		"Document template structure created",
		fmt.Sprintf("Document template structure for %s has been created.", template.Name),
		map[string]any{
			"Name":         s.systemAdminName(),
			"Platform":     s.platformName(),
			"TemplateName": template.Name,
			"TemplateCode": template.Code,
			"FileType":     template.FileType,
			"Version":      int(template.Version),
			"SheetsCount":  len(sheets),
			"ColumnsCount": columnsCount,
			"ActionURL":    s.adminTemplatesURL(),
			"Details": fmt.Sprintf(
				"Template ID: %s\nCode: %s\nName: %s\nFile Type: %s\nVersion: %d\nSheets: %d\nColumns: %d\nCreated By: %s",
				template.ID.String(),
				template.Code,
				template.Name,
				template.FileType,
				int(template.Version),
				len(sheets),
				columnsCount,
				template.CreatedBy.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return structure, nil
}

func (s *documentTemplateService) ReplaceStructure(
	ctx context.Context,
	templateID uuid.UUID,
	sheets []CreateTemplateSheetWithColumnsRequest,
) (*TemplateStructure, error) {
	if s == nil {
		return nil, errors.New("document template service is nil")
	}

	if templateID == uuid.Nil {
		return nil, errors.New("template id is required")
	}

	t, err := s.documentTemplateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template before replace structure: %w", err)
	}

	// Drop all existing sheets (cascade deletes columns via DB FK or loop here)
	existingSheets, err := s.sheetRepo.List(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("list existing sheets: %w", err)
	}

	for _, sh := range existingSheets {
		existingCols, err := s.columnRepo.List(ctx, sh.ID)
		if err != nil {
			return nil, fmt.Errorf("list columns for sheet %s: %w", sh.ID, err)
		}
		for _, col := range existingCols {
			if err := s.columnRepo.Delete(ctx, col.ID); err != nil {
				return nil, fmt.Errorf("delete column %s: %w", col.ID, err)
			}
		}
		if err := s.sheetRepo.Delete(ctx, sh.ID); err != nil {
			return nil, fmt.Errorf("delete sheet %s: %w", sh.ID, err)
		}
	}

	// Recreate sheets and columns from the request
	newSheets := make([]TemplateSheetStructure, 0, len(sheets))

	for _, sheetReq := range sheets {
		sheetInput := sheetReq.Sheet
		sheetInput.TemplateID = templateID
		sheetInput.Code = strings.TrimSpace(sheetInput.Code)
		sheetInput.Name = strings.TrimSpace(sheetInput.Name)
		sheetInput.DisplayName = strings.TrimSpace(sheetInput.DisplayName)

		if sheetInput.Code == "" {
			return nil, errors.New("sheet code is required")
		}

		if sheetInput.Name == "" {
			return nil, errors.New("sheet name is required")
		}

		sheetOrder := sql.NullInt32{}
		if sheetInput.SheetOrder != nil {
			sheetOrder = sql.NullInt32{
				Int32: int32(*sheetInput.SheetOrder),
				Valid: true,
			}
		}

		sheet, err := s.sheetRepo.Create(ctx, db.CreateDocumentTemplateSheetParams{
			ID:         uuid.New(),
			TemplateID: sheetInput.TemplateID,
			Code:       sheetInput.Code,
			Name:       sheetInput.Name,
			DisplayName: sql.NullString{
				String: sheetInput.DisplayName,
				Valid:  sheetInput.DisplayName != "",
			},
			Required:              sheetInput.Required,
			SheetOrder:            sheetOrder,
			HeaderRow:             int32(sheetInput.HeaderRow),
			StartRow:              int32(sheetInput.StartRow),
			AllowExtraColumns:     sheetInput.AllowExtraColumns,
			AllowDuplicateHeaders: sheetInput.AllowDuplicateHeaders,
			Configuration:         toJSON(sheetInput.Configuration),
		})
		if err != nil {
			return nil, fmt.Errorf("create sheet: %w", err)
		}

		columnStructures := make([]TemplateColumnStructure, 0, len(sheetReq.Columns))

		for _, columnReq := range sheetReq.Columns {
			columnReq.SheetID = sheet.ID
			columnReq.ColumnKey = strings.TrimSpace(columnReq.ColumnKey)
			columnReq.ColumnName = strings.TrimSpace(columnReq.ColumnName)
			columnReq.DisplayName = strings.TrimSpace(columnReq.DisplayName)

			if columnReq.ColumnKey == "" {
				return nil, errors.New("column key is required")
			}

			if columnReq.ColumnName == "" {
				return nil, errors.New("column name is required")
			}

			columnOrder := sql.NullInt32{}
			if columnReq.ColumnOrder != nil {
				columnOrder = sql.NullInt32{
					Int32: int32(*columnReq.ColumnOrder),
					Valid: true,
				}
			}

			column, err := s.columnRepo.Create(ctx, db.CreateDocumentTemplateColumnParams{
				ID:         uuid.New(),
				SheetID:    columnReq.SheetID,
				ColumnKey:  columnReq.ColumnKey,
				ColumnName: columnReq.ColumnName,
				DisplayName: sql.NullString{
					String: columnReq.DisplayName,
					Valid:  columnReq.DisplayName != "",
				},
				DataType:    normalizeColumnDataType(columnReq.DataType),
				Required:    columnReq.Required,
				IsUnique:    columnReq.IsUnique,
				ColumnOrder: columnOrder,
				DefaultValue: sql.NullString{
					String: stringPtrValue(columnReq.DefaultValue),
					Valid:  columnReq.DefaultValue != nil,
				},
				Configuration: toJSON(columnReq.Configuration),
				AllowedValues: toJSON(columnReq.AllowedValues),
				Aliases:       toJSON(columnReq.Aliases),
			})
			if err != nil {
				return nil, fmt.Errorf("create column: %w", err)
			}

			columnStructures = append(columnStructures, TemplateColumnStructure{
				ID:            column.ID,
				SheetID:       column.SheetID,
				ColumnKey:     column.ColumnKey,
				ColumnName:    column.ColumnName,
				DisplayName:   nullStringValue(column.DisplayName),
				DataType:      string(column.DataType),
				Required:      column.Required,
				IsUnique:      column.IsUnique,
				ColumnOrder:   nullInt32Ptr(column.ColumnOrder),
				AllowedValues: fromJSONStringArray(column.AllowedValues),
				Aliases:       fromJSONStringArray(column.Aliases),
				Configuration: fromJSON(column.Configuration),
			})
		}

		newSheets = append(newSheets, TemplateSheetStructure{
			ID:                    sheet.ID,
			TemplateID:            sheet.TemplateID,
			Code:                  sheet.Code,
			Name:                  sheet.Name,
			DisplayName:           nullStringValue(sheet.DisplayName),
			Required:              sheet.Required,
			SheetOrder:            nullInt32Ptr(sheet.SheetOrder),
			HeaderRow:             int(sheet.HeaderRow),
			StartRow:              int(sheet.StartRow),
			AllowExtraColumns:     sheet.AllowExtraColumns,
			AllowDuplicateHeaders: sheet.AllowDuplicateHeaders,
			Configuration:         fromJSON(sheet.Configuration),
			Columns:               columnStructures,
		})
	}

	return &TemplateStructure{
		Template: DocumentTemplate{
			ID:            t.ID,
			DocumentID:    nullUUIDPtr(t.DocumentID),
			Code:          t.Code,
			Name:          t.Name,
			Description:   nullStringValue(t.Description),
			FileType:      t.FileType,
			Version:       int(t.Version),
			IsActive:      t.IsActive,
			Configuration: fromJSON(t.Configuration),
			CreatedBy:     t.CreatedBy,
			CreatedAt:     t.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     t.UpdatedAt.Format(time.RFC3339),
			ArchivedAt:    nullTimeStringPtr(t.ArchivedAt),
		},
		Sheets: newSheets,
	}, nil
}

func normalizeColumnDataType(value string) db.DocumentTemplateColumnType {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "TEXT", "STRING", "VARCHAR":
		return db.DocumentTemplateColumnTypeSTRING

	case "NUMBER", "NUMERIC", "DECIMAL", "FLOAT", "DOUBLE":
		return db.DocumentTemplateColumnTypeDECIMAL

	case "INTEGER", "INT":
		return db.DocumentTemplateColumnTypeINTEGER

	case "DATE":
		return db.DocumentTemplateColumnTypeDATE

	case "DATETIME", "TIMESTAMP":
		return db.DocumentTemplateColumnTypeDATETIME

	case "BOOLEAN", "BOOL":
		return db.DocumentTemplateColumnTypeBOOLEAN

	default:
		return db.DocumentTemplateColumnTypeSTRING
	}
}

func mapDocumentTemplate(t db.DocumentTemplate) *DocumentTemplate {
	return &DocumentTemplate{
		ID:            t.ID,
		DocumentID:    nullUUIDPtr(t.DocumentID),
		Code:          t.Code,
		Name:          t.Name,
		Description:   nullStringValue(t.Description),
		FileType:      t.FileType,
		Version:       int(t.Version),
		IsActive:      t.IsActive,
		Configuration: fromJSON(t.Configuration),
		CreatedBy:     t.CreatedBy,
		CreatedAt:     t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     t.UpdatedAt.Format(time.RFC3339),
		ArchivedAt:    nullTimeStringPtr(t.ArchivedAt),
	}
}

func countTemplateStructureColumns(sheets []TemplateSheetStructure) int {
	total := 0
	for _, sheet := range sheets {
		total += len(sheet.Columns)
	}

	return total
}

func uuidPtrString(value *uuid.UUID) string {
	if value == nil {
		return ""
	}

	return value.String()
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

func fromJSON(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{}
	}

	if result == nil {
		return map[string]any{}
	}

	return result
}

func fromJSONStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return []string{}
	}

	var result []string
	if err := json.Unmarshal(raw, &result); err != nil {
		return []string{}
	}

	if result == nil {
		return []string{}
	}

	return result
}

func nullInt32Ptr(value sql.NullInt32) *int {
	if !value.Valid {
		return nil
	}

	v := int(value.Int32)
	return &v
}

func nullUUIDPtr(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}

	return &value.UUID
}

func nullTimeStringPtr(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}

	v := value.Time.Format(time.RFC3339)
	return &v
}
