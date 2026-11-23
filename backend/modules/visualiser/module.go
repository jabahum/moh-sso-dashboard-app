package visualiser

import (
	"database/sql"
	_ "embed"
	"errors"
	"log/slog"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// ID is the module's registry identifier.
const ID = "visualiser"

// Permissions accepted by the module's endpoints; any one of them grants access.
const (
	ReportBrowserReadPermission = "report_browser:read"
	DocumentsReadPermission     = "documents:read"
	DataQualityReadPermission   = "data_quality:read"
	SurveillanceReadPermission  = "surveillance:read"
)

// ReadPermissions is the set of permissions, any one of which grants access.
var ReadPermissions = []string{
	ReportBrowserReadPermission,
	DocumentsReadPermission,
	DataQualityReadPermission,
	SurveillanceReadPermission,
}

//go:embed openapi.json
var openAPIDocument []byte

// Dependencies are supplied by the host.
//
// Database is the data-warehouse connection. It may be nil when the warehouse
// is not configured: the endpoints then fail with 500, as before extraction.
// Repository replaces the warehouse queries (tests); at most one of Database
// and Repository may be set. HealthContexts is required; Logger defaults to
// slog.Default().
type Dependencies struct {
	Database       *sql.DB
	Repository     Repository
	HealthContexts modulekit.HealthContextDirectory
	Logger         *slog.Logger
}

// Module is the mounted HTTP module.
type Module struct {
	*modulekit.HTTPModule
	Service Service
}

// NewModule validates dependencies and builds the module.
func NewModule(deps Dependencies) (*Module, error) {
	repository := deps.Repository
	if deps.Database != nil && repository != nil {
		return nil, errors.New("visualiser: set at most one of Database or Repository")
	}
	if deps.HealthContexts == nil {
		return nil, errors.New("visualiser: HealthContexts is required")
	}
	if repository == nil {
		repository = NewPostgresRepository(deps.Database)
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	service := NewService(repository)
	h := handlers{service: service, healthContexts: deps.HealthContexts, logger: logger}
	read := append([]string(nil), ReadPermissions...)
	router, err := modulekit.NewRouter([]modulekit.Endpoint{
		{Method: "GET", Path: "/visualizer/datasets", AnyPermissions: read, Handle: h.getDatasets},
		{Method: "GET", Path: "/visualizer/dataelements", AnyPermissions: read, Handle: h.getDataElements},
		{Method: "POST", Path: "/visualizer/datavalues", AnyPermissions: read, Handle: h.getDataValues},
	})
	if err != nil {
		return nil, err
	}
	httpModule, err := modulekit.NewHTTPModule(router, openAPIDocument)
	if err != nil {
		return nil, err
	}
	return &Module{HTTPModule: httpModule, Service: service}, nil
}
