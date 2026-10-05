package routes

import (
	"github.com/gin-gonic/gin"

	announcementfeature "github.com/moh-sso-dashboard/internal/features/announcements"
	reportschedulerfeature "github.com/moh-sso-dashboard/internal/features/report_scheduler"
)

func RegisterPublicAnnouncementRoutes(api *gin.RouterGroup, deps Dependencies) {
	announcementfeature.RegisterPublicRoutes(api, deps.Announcements)
}

func RegisterPublicReportSchedulerRoutes(api *gin.RouterGroup, deps Dependencies) {
	reportschedulerfeature.RegisterPublicRoutes(api, deps.ReportScheduler, deps.Limiter)
}
