package bootstrap

import (
	"context"

	kitadapter "github.com/moh-sso-dashboard/internal/adapters/modulekit"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/moh-sso-dashboard/internal/storage"
	documentsmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/documents"
	storagelocationfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/storage-locations"
)

// buildDocumentsModule wires the Documents module to the host: the primary and
// upload databases, storage locations, file-storage providers, notifications,
// the documents rate-limit policies and the import processors that execute
// claimed processing jobs.
func buildDocumentsModule(deps handlerDependencies) *documentsmodule.Module {
	moduleDeps := documentsmodule.Dependencies{
		Database:         deps.Databases.Primary,
		RemoteDatabase:   deps.Databases.Remote,
		StorageLocations: documentStorageLocations{service: deps.Services.StorageLocations},
		FileStores:       documentFileStores{factory: deps.StorageFactory},
		Notifier:         kitadapter.Notifier{Service: deps.Services.Notifications},
		Platform:         kitadapter.Platform(deps.Config),
		Limiter:          modulehttp.PolicyLimiter(ratelimit.New(deps.Redis)),
	}
	if deps.Services.Import != nil {
		moduleDeps.Executor = deps.Services.Import
	}
	module, err := documentsmodule.NewModule(moduleDeps)
	if err != nil {
		panic(err)
	}
	return module
}

// documentStorageLocations adapts the storage-locations module's Service.
type documentStorageLocations struct {
	service storagelocationfeature.Service
}

func (a documentStorageLocations) GetByID(ctx context.Context, id string) (documentsmodule.Location, error) {
	location, err := a.service.GetByID(ctx, id)
	if err != nil {
		return documentsmodule.Location{}, err
	}
	return documentsmodule.Location{ID: location.ID, Provider: location.Provider, BaseURI: location.BaseUri}, nil
}

// documentFileStores adapts the host storage factory.
type documentFileStores struct{ factory *storage.StorageFactory }

func (a documentFileStores) Get(provider string) (documentsmodule.FileStore, error) {
	return a.factory.Get(provider)
}
