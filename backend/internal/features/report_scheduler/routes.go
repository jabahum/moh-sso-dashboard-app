package report_scheduler

import (
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
)

func RegisterProtectedRoutes(protected *gin.RouterGroup, handler *Handler) {
	scheduler := protected.Group("/report-scheduler", middleware.RequireSystem(authz.SystemDataStatistics))
	scheduler.GET("", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetModule)
	scheduler.GET("/reports", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.ListReports)
	scheduler.GET("/reports/:reportId", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetReport)
	scheduler.GET("/reports/:reportId/parameters", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetReportParameters)
	scheduler.POST("/reports/:reportId/generate", middleware.RequirePermission(authz.PermissionReportSchedulerExecute), handler.GenerateReport)
	scheduler.GET("/jobs/:jobId", middleware.RequirePermission(authz.PermissionReportSchedulerHistory), handler.GetJob)
	scheduler.GET("/schedules", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.ListSchedules)
	scheduler.POST("/schedules", middleware.RequirePermission(authz.PermissionReportSchedulerCreate), handler.CreateSchedule)
	scheduler.GET("/schedules/:scheduleId", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetSchedule)
	scheduler.PUT("/schedules/:scheduleId", middleware.RequirePermission(authz.PermissionReportSchedulerUpdate), handler.UpdateSchedule)
	scheduler.DELETE("/schedules/:scheduleId", middleware.RequirePermission(authz.PermissionReportSchedulerDelete), handler.DeleteSchedule)
	scheduler.GET("/executions", middleware.RequirePermission(authz.PermissionReportSchedulerHistory), handler.ListExecutions)
}
