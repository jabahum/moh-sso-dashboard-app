package platform

import (
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/ratelimit"
)

// registerIdentitySyncRoutes adds the identity-sync registry admin API to the
// admin group, which already requires both rbac:write and rbac:roles:write.
func registerIdentitySyncRoutes(admin *gin.RouterGroup, h *Handler, limiter *ratelimit.Limiter) {
	sensitive := ratelimit.MiddlewareForPolicy(limiter, ratelimit.RBACSensitiveWritePolicy())
	admin.GET("/applications/:clientId/identity-sync", h.GetIdentitySync)
	admin.PUT("/applications/:clientId/identity-sync", sensitive, h.SaveIdentitySync)
	admin.DELETE("/applications/:clientId/identity-sync", sensitive, h.DeleteIdentitySync)
}
