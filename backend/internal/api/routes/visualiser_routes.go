package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/moh-sso-dashboard/internal/modulehttp"
)

// registerVisualiserRoutes mounts the legacy administrative-unit routes when
// no module registry is used. The visualiser endpoints themselves are served by
// the visualiser module, registered on module.Routes.ContextScoped.
func registerVisualiserRoutes(protected *gin.RouterGroup, deps Dependencies) {
	if deps.Modules == nil && deps.AdminUnits != nil {
		modulehttp.Register(protected, deps.AdminUnits)
	}
}
