package documents

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"log/slog"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// ID is the module's registry identifier.
const ID = "documents"

// Permissions checked by the module's endpoints.
const (
	ReadPermission    = "documents:read"
	WritePermission   = "documents:write"
	ProcessPermission = "documents:process"
)

// Per-user rate limits, unchanged from the host policies of the same names.
var (
	WriteRateLimit   = modulekit.RateLimit{Name: "documents-write", Limit: 30, Window: time.Minute}
	ProcessRateLimit = modulekit.RateLimit{Name: "documents-process", Limit: 10, Window: time.Minute}
)

//go:embed openapi.json
var openAPIDocument []byte

// Dependencies are supplied by the host. Exactly one of Database or
// Repository is required, as are StorageLocations and FileStores.
type Dependencies struct {
	// Database is the primary database (documents, processes, templates).
	Database   *sql.DB
	Repository Repository
	// RemoteDatabase is the upload database holding import.custom_data_files.
	// Without it the data-preview endpoints answer 503 NO_REMOTE_DB.
	RemoteDatabase   *sql.DB
	StorageLocations StorageLocations
	FileStores       FileStores
	// Executor runs claimed processing jobs; Initialize requires it.
	Executor ProcessExecutor
	// PollInterval is the worker's queue polling delay (DefaultPollInterval
	// when zero).
	PollInterval time.Duration
	Notifier     modulekit.Notifier
	Platform     modulekit.Platform
	Limiter      modulekit.Limiter
	Logger       *slog.Logger
}

// Module is the mounted HTTP module plus the service and the processing worker.
type Module struct {
	*modulekit.HTTPModule
	Service *Service

	repository   Repository
	executor     ProcessExecutor
	pollInterval time.Duration
	logger       *slog.Logger
}

// NewModule validates dependencies and builds the module.
func NewModule(deps Dependencies) (*Module, error) {
	repository := deps.Repository
	if (deps.Database == nil) == (repository == nil) {
		return nil, errors.New("documents: exactly one of Database or Repository is required")
	}
	if deps.StorageLocations == nil {
		return nil, errors.New("documents: StorageLocations is required")
	}
	if deps.FileStores == nil {
		return nil, errors.New("documents: FileStores is required")
	}
	if repository == nil {
		repository = NewPostgresRepository(deps.Database)
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	pollInterval := deps.PollInterval
	if pollInterval == 0 {
		pollInterval = DefaultPollInterval
	}
	service := NewService(repository, deps.Notifier, deps.Platform)
	h := handlers{service: service, locations: deps.StorageLocations, stores: deps.FileStores, remoteDB: deps.RemoteDatabase}
	writeLimit, processLimit := WriteRateLimit, ProcessRateLimit
	router, err := modulekit.NewRouter([]modulekit.Endpoint{
		{Method: "GET", Path: "/documents", Permission: ReadPermission, Handle: h.listDocuments},
		{Method: "GET", Path: "/documents/stats", Permission: ReadPermission, Handle: h.getDocumentStats},
		{Method: "POST", Path: "/documents", Permission: WritePermission, RateLimit: &writeLimit, Handle: h.createDocument},
		{Method: "POST", Path: "/documents/scan-structure", Permission: WritePermission, Handle: h.scanStructure},
		{Method: "GET", Path: "/documents/{id}", Permission: ReadPermission, Handle: h.getDocument},
		{Method: "PUT", Path: "/documents/{id}", Permission: WritePermission, Handle: h.editDocument},
		{Method: "DELETE", Path: "/documents/{id}", Permission: WritePermission, Handle: h.deleteDocument},
		{Method: "GET", Path: "/documents/{id}/processes", Permission: ReadPermission, Handle: h.listDocumentProcesses},
		{Method: "POST", Path: "/documents/{id}/reprocess", Permission: ProcessPermission, RateLimit: &processLimit, Handle: h.reprocessDocument},
		{Method: "GET", Path: "/documents/{id}/data-preview", Permission: ReadPermission, Handle: h.dataPreview},
		{Method: "POST", Path: "/documents/{id}/data-preview/export", Permission: ReadPermission, Handle: h.exportDataPreview},
		{Method: "GET", Path: "/documents/{id}/parse-structure", Permission: ReadPermission, Handle: h.parseStructure},
		{Method: "GET", Path: "/documents/files/{id}/view", Permission: ReadPermission, Handle: h.viewDocument},
		{Method: "GET", Path: "/documents/files/{id}/download", Permission: ReadPermission, Handle: h.downloadDocument},
	}, modulekit.WithLimiter(deps.Limiter))
	if err != nil {
		return nil, err
	}
	httpModule, err := modulekit.NewHTTPModule(router, openAPIDocument)
	if err != nil {
		return nil, err
	}
	return &Module{
		HTTPModule: httpModule, Service: service,
		repository: repository, executor: deps.Executor, pollInterval: pollInterval, logger: logger,
	}, nil
}

// Initialize starts the document processing worker. The returned stop
// cancels it and waits for the in-flight job.
func (m *Module) Initialize(ctx context.Context) (core.Stop, error) {
	if m == nil {
		return nil, errors.New("documents module is nil")
	}
	worker, err := NewWorker(m.repository, m.executor, m.pollInterval, m.logger)
	if err != nil {
		return nil, err
	}
	stop, err := worker.Initialize(ctx)
	if err != nil {
		return nil, err
	}
	return stop, nil
}
