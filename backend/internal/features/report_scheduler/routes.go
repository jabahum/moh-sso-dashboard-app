package report_scheduler

import (
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/ratelimit"
)

func RegisterProtectedRoutes(protected *gin.RouterGroup, handler *Handler) {
	scheduler := protected.Group("/report-scheduler", middleware.RequireSystem(authz.SystemDataStatistics))
	scheduler.GET("", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetModule)
	scheduler.GET("/overview", middleware.RequirePermission(authz.PermissionReportSchedulerHistory), handler.Overview)
	scheduler.GET("/reports", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.ListReports)
	scheduler.GET("/reports/:reportId", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetReport)
	scheduler.GET("/reports/:reportId/parameters", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetReportParameters)
	scheduler.GET("/schedules", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.ListSchedules)
	scheduler.POST("/schedules", middleware.RequirePermission(authz.PermissionReportSchedulerCreate), handler.CreateSchedule)
	scheduler.GET("/schedules/:scheduleId", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.GetSchedule)
	scheduler.PUT("/schedules/:scheduleId", middleware.RequirePermission(authz.PermissionReportSchedulerUpdate), handler.UpdateSchedule)
	scheduler.DELETE("/schedules/:scheduleId", middleware.RequirePermission(authz.PermissionReportSchedulerDelete), handler.DeleteSchedule)
	scheduler.POST("/schedules/:scheduleId/pause", middleware.RequirePermission(authz.PermissionReportSchedulerUpdate), handler.PauseSchedule)
	scheduler.POST("/schedules/:scheduleId/resume", middleware.RequirePermission(authz.PermissionReportSchedulerUpdate), handler.ResumeSchedule)
	scheduler.POST("/schedules/:scheduleId/duplicate", middleware.RequirePermission(authz.PermissionReportSchedulerCreate), handler.DuplicateSchedule)
	scheduler.POST("/schedules/:scheduleId/run", middleware.RequirePermission(authz.PermissionReportSchedulerExecute), handler.RunNow)
	scheduler.GET("/executions", middleware.RequirePermission(authz.PermissionReportSchedulerHistory), handler.ListExecutions)
	scheduler.GET("/executions/:executionId", middleware.RequirePermission(authz.PermissionReportSchedulerHistory), handler.GetExecution)
	scheduler.POST("/executions/:executionId/retry", middleware.RequirePermission(authz.PermissionReportSchedulerExecute), handler.RetryExecution)
	scheduler.POST("/recipients/preview", middleware.RequirePermission(authz.PermissionReportSchedulerCreate), handler.PreviewRecipients)
	scheduler.GET("/portal-reports", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.ListPortalReports)
	scheduler.GET("/artifacts/:artifactId/download", middleware.RequirePermission(authz.PermissionReportSchedulerRead), handler.DownloadArtifact)
}

func RegisterPublicRoutes(api *gin.RouterGroup, handler *Handler, limiter *ratelimit.Limiter) {
	scheduler := api.Group("/report-scheduler/public")
	scheduler.GET("/deliveries/:token/download", ratelimit.MiddlewareForPolicy(limiter, ratelimit.PerIPPolicy("report-delivery-download", 60)), handler.DownloadDeliveryLink)
}
