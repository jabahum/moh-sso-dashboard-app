package documenttemplates

import (
	"database/sql"
	_ "embed"
	"errors"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates/internal/postgres"
)

// ID is the module's registry identifier.
const ID = "document-templates"

// Permissions checked by the module's endpoints.
const (
	ReadPermission    = "document_templates:read"
	WritePermission   = "document_templates:write"
	PublishPermission = "document_templates:publish"
)

//go:embed openapi.json
var openAPIDocument []byte

// Dependencies are supplied by the host. Database (the primary database that
// holds the template tables) and Warehouse (read by the has-data check) are
// required; Notifier is optional.
//
// The routes are context scoped: the host mounts them behind its
// data-statistics system check and health-context selection, as before.
type Dependencies struct {
	Database  *sql.DB
	Warehouse *sql.DB
	Notifier  modulekit.Notifier
	Platform  modulekit.Platform
}

// Module is the mounted HTTP module plus the services other host code
// consumes (the document import reads template structures).
type Module struct {
	*modulekit.HTTPModule
	Service       Service
	SheetService  SheetService
	ColumnService ColumnService
}

type repositories struct {
	templates DocumentTemplateRepository
	sheets    DocumentTemplateSheetRepository
	columns   DocumentTemplateColumnRepository
}

// NewModule validates dependencies and builds the module.
func NewModule(deps Dependencies) (*Module, error) {
	if deps.Database == nil {
		return nil, errors.New("document templates: Database is required")
	}
	queries := db.New(deps.Database)
	return newModule(deps, repositories{
		templates: newDocumentTemplateRepository(queries),
		sheets:    newDocumentTemplateSheetRepository(queries),
		columns:   newDocumentTemplateColumnRepository(queries),
	})
}

func newModule(deps Dependencies, repos repositories) (*Module, error) {
	if deps.Warehouse == nil {
		return nil, errors.New("document templates: Warehouse is required")
	}
	if repos.templates == nil || repos.sheets == nil || repos.columns == nil {
		return nil, errors.New("document templates: repositories are required")
	}
	service := NewService(repos.templates, repos.sheets, repos.columns, deps.Notifier, deps.Platform)
	sheetService := NewSheetService(repos.sheets, deps.Notifier, deps.Platform)
	columnService := NewColumnService(repos.columns, deps.Notifier, deps.Platform)
	h := handlers{service: service, sheetService: sheetService, columnService: columnService, warehouse: deps.Warehouse}

	router, err := modulekit.NewRouter([]modulekit.Endpoint{
		{Method: "GET", Path: "/document-templates", Permission: ReadPermission, Handle: h.listTemplates},
		{Method: "POST", Path: "/document-templates", Permission: WritePermission, Handle: h.createTemplate},
		{Method: "POST", Path: "/document-templates/structure", Permission: WritePermission, Handle: h.createTemplateWithStructure},
		{Method: "GET", Path: "/document-templates/code/{code}/structure", Permission: ReadPermission, Handle: h.getTemplateStructure},
		{Method: "GET", Path: "/document-templates/code/{code}/has-data", Permission: ReadPermission, Handle: h.hasData},
		{Method: "GET", Path: "/document-templates/{id}", Permission: ReadPermission, Handle: h.getTemplate},
		{Method: "PUT", Path: "/document-templates/{id}", Permission: WritePermission, Handle: h.updateTemplate},
		{Method: "DELETE", Path: "/document-templates/{id}", Permission: WritePermission, Handle: h.deleteTemplate},
		{Method: "POST", Path: "/document-templates/{id}/publish", Permission: PublishPermission, Handle: h.publishTemplate},
		{Method: "POST", Path: "/document-templates/{id}/archive", Permission: PublishPermission, Handle: h.archiveTemplate},
		{Method: "GET", Path: "/document-templates/{id}/structure", Permission: ReadPermission, Handle: h.getTemplateStructure},
		{Method: "PUT", Path: "/document-templates/{id}/structure", Permission: WritePermission, Handle: h.replaceStructure},
		{Method: "GET", Path: "/document-templates/{id}/sheets", Permission: ReadPermission, Handle: h.listSheets},
		{Method: "GET", Path: "/document-templates/{id}/sheets/{sheetId}/columns", Permission: ReadPermission, Handle: h.listColumns},
	})
	if err != nil {
		return nil, err
	}
	httpModule, err := modulekit.NewHTTPModule(router, openAPIDocument)
	if err != nil {
		return nil, err
	}
	return &Module{HTTPModule: httpModule, Service: service, SheetService: sheetService, ColumnService: columnService}, nil
}
