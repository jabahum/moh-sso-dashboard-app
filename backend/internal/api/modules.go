package router

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/api/handler"
	metricsfeature "github.com/moh-sso-dashboard/internal/features/metrics"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/module"
	metricsmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
)

// NewModuleRegistry is the distribution's explicit composition root. Legacy
// features enter this inventory only as their registration is migrated.
func NewModuleRegistry(handlers HandlerSet, disabled string) (*module.Registry, error) {
	enabled := true
	for _, id := range strings.Split(disabled, ",") {
		switch strings.TrimSpace(id) {
		case "":
		case "metrics":
			enabled = false
		default:
			return nil, fmt.Errorf("unknown disabled module %q", strings.TrimSpace(id))
		}
	}
	return module.New(module.Definition{
		Info: module.Info{ID: "metrics", Version: metricsmodule.Version(), Enabled: enabled},
		Initialize: func(context.Context) (module.Stop, error) {
			if handlers.Metrics == nil {
				return nil, errors.New("metrics handler is required")
			}
			return nil, nil
		},
		Register: func(routes module.Routes) {
			metricsfeature.RegisterAdminRoutes(routes.Admin, handlers.Metrics)
		},
	})
}

func registerModuleInventory(protected *gin.RouterGroup, registry *module.Registry) {
	h := &handler.ModuleHandler{Registry: registry}
	protected.GET("/admin/modules", middleware.RequireAdmin(), h.List)
}
