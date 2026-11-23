package bootstrap

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
	kitadapter "github.com/moh-sso-dashboard/internal/adapters/modulekit"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	documentfeature "github.com/moh-sso-dashboard/internal/repository/document_imports"
	processRepo "github.com/moh-sso-dashboard/internal/repository/processes"
	"github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/internal/storage"
	surveillancemodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance"
)

// buildSurveillanceModule wires the Surveillance module to the Observatory,
// the primary geography registry, the health-context hierarchy and the host's
// document pipeline. It returns nil when the Observatory is not configured
// (the core profile), where the module is disabled.
func buildSurveillanceModule(deps serviceDependencies, notifications service.NotificationsService, contexts *healthcontextfeature.Service) *surveillancemodule.Module {
	if deps.Databases.Observatory == nil {
		return nil
	}
	moduleDeps := surveillancemodule.Dependencies{
		Database:      deps.Databases.Observatory,
		HealthContext: healthcontextfeature.ModuleDirectory{Service: contexts},
		Imports: surveillanceImportHost{
			documents: deps.Repositories.Documents,
			processes: deps.Repositories.Processes,
			storage:   deps.FileStorage,
		},
		Notifier: kitadapter.Notifier{Service: notifications},
		Platform: kitadapter.Platform(deps.Config),
		Limiter:  modulehttp.PolicyLimiter(ratelimit.New(deps.CacheClient)),
		Logger:   slog.Default(),
	}
	// Geography writes go through the primary registry and are replicated
	// back, exactly as the host's Observatory store did before extraction.
	if deps.Databases.registry != nil {
		moduleDeps.GeographyPrimary = deps.Databases.Primary
		moduleDeps.ReplicateGeography = deps.Databases.registry.Sync
	}
	if deps.Databases.observatory != nil {
		moduleDeps.Available = deps.Databases.observatory.Ready
	}
	module, err := surveillancemodule.NewModule(moduleDeps)
	if err != nil {
		panic(err)
	}
	return module
}

// surveillanceImportHost adapts the host's documents, file storage and process
// queue to the module's CSV import port.
type surveillanceImportHost struct {
	documents documentfeature.DocumentRepository
	processes processRepo.ProcessRepository
	storage   storage.Storage
}

func (h surveillanceImportHost) GetDocument(ctx context.Context, documentID uuid.UUID) (surveillancemodule.ImportDocument, error) {
	doc, err := h.documents.GetDocument(ctx, documentID)
	if err != nil {
		return surveillancemodule.ImportDocument{}, err
	}
	return surveillancemodule.ImportDocument{ObjectKey: doc.ObjectKey, OriginalFilename: doc.OriginalFilename}, nil
}

func (h surveillanceImportHost) Download(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return h.storage.Download(ctx, objectKey)
}

func (h surveillanceImportHost) UpdateProgress(ctx context.Context, processID uuid.UUID, progress int32, message *string) error {
	return h.processes.UpdateProgress(ctx, processID, progress, message)
}

func (h surveillanceImportHost) QueueBatchProcess(ctx context.Context, documentID, createdBy uuid.UUID) error {
	_, err := h.processes.CreateProcess(ctx, db.CreateProcessParams{
		DocumentID:  documentID,
		ProcessType: string(model.ProcessTypeSurveillanceBatchProcess),
		CreatedBy:   createdBy,
	})
	return err
}
