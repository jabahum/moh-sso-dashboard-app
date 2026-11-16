package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
	metrics "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
)

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *Handler) {
	metrics.RegisterAdminRoutes(admin, handler, middleware.RequirePermission(authz.PermissionMetricsRead))
}
