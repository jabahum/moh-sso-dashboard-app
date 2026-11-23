package router

import (
	"context"
	"errors"
	"fmt"
	dataqualitymodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/data-quality"
	documentsmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/documents"
	surveillancemodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance"
	"strings"

	"github.com/gin-gonic/gin"
	kitadapter "github.com/moh-sso-dashboard/internal/adapters/modulekit"
	"github.com/moh-sso-dashboard/internal/api/handler"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/moh-sso-dashboard/module"
	adminunitsmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units"
	announcementmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements"
	auditmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/audit"
	documenttemplatesmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates"
	geojsonmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
	metricsmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
	schedulermodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
	storagelocationsmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/storage-locations"
	visualisermodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/visualiser"
)

// NewModuleRegistry is the distribution's explicit composition root. Legacy
// features enter this inventory only as their registration is migrated.
func NewModuleRegistry(handlers HandlerSet, disabled string, limiter *ratelimit.Limiter, scheduler *schedulermodule.Worker) (*module.Registry, error) {
	enabled := map[string]bool{}
	for _, id := range config.OptionalModuleIDs() {
		enabled[id] = true
	}
	for _, id := range strings.Split(disabled, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, known := enabled[id]; !known {
			return nil, fmt.Errorf("unknown disabled module %q", id)
		}
		enabled[id] = false
	}
	return module.New(module.Definition{
		Info: module.Info{ID: "metrics", Version: metricsmodule.Version(), Enabled: enabled["metrics"]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.Metrics == nil {
				return nil, errors.New("metrics handler is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			modulehttp.Register(routes.Protected, handlers.Metrics)
		},
	}, module.Definition{
		Info: module.Info{ID: "audit", Version: auditmodule.Version(), Enabled: enabled["audit"]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.Audit == nil {
				return nil, errors.New("audit read handler is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			modulehttp.Register(routes.Protected, handlers.Audit)
		},
	}, module.Definition{
		Info: module.Info{ID: "geojson", Version: geojsonmodule.Version(), Enabled: enabled["geojson"]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.GeoJSON == nil {
				return nil, errors.New("geojson handler is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			modulehttp.Register(routes.Protected, handlers.GeoJSON)
		},
	}, module.Definition{
		Info: module.Info{ID: "report-scheduler", Version: schedulermodule.Version(), Enabled: enabled["report-scheduler"]},
		Initialize: func(ctx context.Context) (module.Stop, error) {
			if handlers.ReportScheduler == nil {
				return nil, errors.New("report scheduler handler is required")
			}
			if scheduler == nil {
				return nil, errors.New("report scheduler worker is required")
			}
			return scheduler.Initialize(ctx)
		},
		Register: func(routes module.Routes) {
			RegisterReportScheduler(routes.Public, routes.Protected, handlers.ReportScheduler, limiter)
		},
	}, module.Definition{
		Info: module.Info{ID: "announcements", Version: announcementmodule.Version(), Enabled: enabled["announcements"]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.Announcements == nil {
				return nil, errors.New("announcements handler is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			modulehttp.RegisterSurfaces(routes.Public, routes.Protected, handlers.Announcements)
		},
	}, module.Definition{
		Info: module.Info{ID: "admin-units", Version: adminunitsmodule.Version(), Enabled: enabled["admin-units"]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.AdminUnits == nil {
				return nil, errors.New("administrative units handler is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			modulehttp.Register(routes.Protected, handlers.AdminUnits)
		},
	}, module.Definition{
		Info: module.Info{ID: storagelocationsmodule.ID, Version: storagelocationsmodule.Version(), Enabled: enabled[storagelocationsmodule.ID]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.StorageLocations == nil {
				return nil, errors.New("storage locations module is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			modulehttp.Register(routes.Protected, handlers.StorageLocations)
		},
	}, module.Definition{
		Info: module.Info{ID: visualisermodule.ID, Version: visualisermodule.Version(), Enabled: enabled[visualisermodule.ID]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.Visualiser == nil {
				return nil, errors.New("visualiser module is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			modulehttp.Register(routes.ContextScoped, handlers.Visualiser)
		},
	}, module.Definition{
		Info: module.Info{ID: documenttemplatesmodule.ID, Version: documenttemplatesmodule.Version(), Enabled: enabled[documenttemplatesmodule.ID]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.DocumentTemplates == nil {
				return nil, errors.New("document templates module is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			// Behind the data-statistics system check and health-context
			// selection, where the host registered these routes before.
			modulehttp.Register(routes.ContextScoped, handlers.DocumentTemplates)
		},
	}, module.Definition{
		Info: module.Info{ID: dataqualitymodule.ID, Version: dataqualitymodule.Version(), Enabled: enabled[dataqualitymodule.ID]},
		Initialize: func(ctx context.Context) (module.Stop, error) {
			if handlers.DataQuality == nil {
				return nil, errors.New("data quality module is required")
			}
			return handlers.DataQuality.Initialize(ctx)
		},
		Register: func(routes module.Routes) {
			if routes.ContextScoped == nil {
				return
			}
			// The middleware only names the caller for created_by/reported_by fields.
			modulehttp.Register(routes.ContextScoped.Group("", kitadapter.CallerNameMiddleware()), handlers.DataQuality)
		},
	}, module.Definition{
		Info:     module.Info{ID: documentsmodule.ID, Version: documentsmodule.Version(), Enabled: enabled[documentsmodule.ID]},
		Requires: []string{storagelocationsmodule.ID},
		Initialize: func(ctx context.Context) (module.Stop, error) {
			if handlers.Documents == nil {
				return nil, errors.New("documents module is required")
			}
			// Runs the document processing worker until shutdown.
			return handlers.Documents.Initialize(ctx)
		},
		Register: func(routes module.Routes) {
			modulehttp.Register(routes.ContextScoped, handlers.Documents)
		},
	}, module.Definition{
		Info: module.Info{ID: surveillancemodule.ID, Version: surveillancemodule.Version(), Enabled: enabled[surveillancemodule.ID]},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.Surveillance == nil {
				return nil, errors.New("surveillance module is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			// Context-scoped: data-statistics system and the selected health context.
			modulehttp.Register(routes.ContextScoped, handlers.Surveillance)
		},
	})
}

func registerModuleInventory(protected *gin.RouterGroup, registry *module.Registry) {
	h := &handler.ModuleHandler{Registry: registry}
	protected.GET("/admin/modules", middleware.RequireAdmin(), h.List)
}
