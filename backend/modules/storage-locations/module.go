package storagelocations

import (
	"database/sql"
	_ "embed"
	"errors"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// ID is the module's registry identifier.
const ID = "storage-locations"

// Permissions checked by the module's endpoints.
const (
	ReadPermission  = "storage_locations:read"
	WritePermission = "storage_locations:write"
)

//go:embed openapi.json
var openAPIDocument []byte

// Dependencies are supplied by the host. Exactly one of Database or
// Repository is required; Notifier is optional.
type Dependencies struct {
	Database   *sql.DB
	Repository Repository
	Notifier   modulekit.Notifier
	Platform   modulekit.Platform
}

// Module is the mounted HTTP module plus the service other modules consume.
type Module struct {
	*modulekit.HTTPModule
	Service Service
}

// NewModule validates dependencies and builds the module.
func NewModule(deps Dependencies) (*Module, error) {
	repository := deps.Repository
	if (deps.Database == nil) == (repository == nil) {
		return nil, errors.New("storage locations: exactly one of Database or Repository is required")
	}
	if repository == nil {
		repository = NewPostgresRepository(deps.Database)
	}
	service := NewService(repository, deps.Notifier, deps.Platform)
	h := handlers{service: service}
	router, err := modulekit.NewRouter([]modulekit.Endpoint{
		{Method: "POST", Path: "/storage-locations", Permission: WritePermission, Handle: h.create},
		{Method: "GET", Path: "/storage-locations", Permission: ReadPermission, Handle: h.listActive},
		{Method: "GET", Path: "/storage-locations/{id}", Permission: ReadPermission, Handle: h.getByID},
		{Method: "PUT", Path: "/storage-locations/{id}", Permission: WritePermission, Handle: h.update},
		{Method: "DELETE", Path: "/storage-locations/{id}", Permission: WritePermission, Handle: h.delete},
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
