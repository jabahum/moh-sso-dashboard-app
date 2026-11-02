package platform

import (
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/ratelimit"
)

func RegisterRoutes(api, protected *gin.RouterGroup, h *Handler, limiter *ratelimit.Limiter) {
	if h == nil {
		return
	}
	platform := api.Group("/platform")
	platform.Use(ratelimit.MiddlewareForPolicy(limiter, ratelimit.PerIPPolicy("platform-auth", 120)))
	platform.Use(h.AuthenticateToken())
	platform.GET("/applications/:clientId/access", h.GetUserAccess)
	machine := platform.Group("")
	machine.Use(h.AuthenticateMachine())
	machine.GET("/identity", requireScope(IdentityRead), h.GetIdentity)
	admin := protected.Group("/admin/platform")
	admin.Use(middleware.RequireAllPermissions(authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite))
	admin.PUT("/applications/:clientId/identities/:oauthClientId", ratelimit.MiddlewareForPolicy(limiter, ratelimit.RBACSensitiveWritePolicy()), h.SaveIdentity)
}
