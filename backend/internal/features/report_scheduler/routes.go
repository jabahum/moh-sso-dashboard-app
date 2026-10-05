package report_scheduler

import (
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
)

func RegisterProtectedRoutes(protected *gin.RouterGroup, handler *Handler) {
	scheduler := protected.Group("/report-scheduler", middleware.RequireSystem(authz.SystemDataStatistics))
	scheduler.GET("", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetModule)
}
