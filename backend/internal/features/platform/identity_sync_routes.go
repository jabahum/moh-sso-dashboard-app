package platform

import (
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/ratelimit"
)

// registerIdentitySyncRoutes adds the identity-sync registry, run and revocation admin API to the
// admin group, which already requires both rbac:write and rbac:roles:write.
func registerIdentitySyncRoutes(admin *gin.RouterGroup, h *Handler, limiter *ratelimit.Limiter) {
	sensitive := ratelimit.MiddlewareForPolicy(limiter, ratelimit.RBACSensitiveWritePolicy())
	admin.GET("/applications/:clientId/identity-sync", h.GetIdentitySync)
	admin.PUT("/applications/:clientId/identity-sync", sensitive, h.SaveIdentitySync)
	admin.DELETE("/applications/:clientId/identity-sync", sensitive, h.DeleteIdentitySync)
	admin.GET("/applications/:clientId/identity-sync/describe", h.DescribeIdentitySync)
	admin.GET("/applications/:clientId/identity-sync/network-check", sensitive, h.CheckIdentitySyncNetwork)
	admin.GET("/applications/:clientId/identity-sync/jobs", h.ListIdentitySyncJobs)
	admin.GET("/applications/:clientId/identity-sync/runs", h.ListIdentitySyncRuns)
	admin.POST("/applications/:clientId/identity-sync/runs", sensitive, h.StartIdentitySyncRun)
	admin.GET("/applications/:clientId/identity-sync/runs/:runId", h.GetIdentitySyncRun)
	admin.POST("/applications/:clientId/identity-sync/runs/:runId/decisions", sensitive, h.DecideIdentitySyncRows)
	admin.POST("/applications/:clientId/identity-sync/runs/:runId/apply", sensitive, h.ApplyIdentitySyncRun)
	admin.GET("/applications/:clientId/identity-sync/revocations", h.ListIdentitySyncRevocations)
	admin.POST("/applications/:clientId/identity-sync/revocations/resume", sensitive, h.ResumeIdentitySyncRevocations)
	admin.POST("/applications/:clientId/identity-sync/revocations/restore", sensitive, h.RestoreIdentitySyncRevocation)
}
