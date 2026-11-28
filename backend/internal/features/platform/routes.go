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
	machine.GET("/readiness", requireScope(IdentityRead), h.GetReadiness)
	if h.DeliveryEnabled == nil || *h.DeliveryEnabled {
		machine.POST("/notifications", requireScope(NotificationsWrite), h.SubmitNotification)
		machine.GET("/notifications/:id", requireScope(NotificationsRead), h.GetNotification)
	}
	admin := protected.Group("/admin/platform")
	admin.GET("/applications/:clientId/operations", middleware.RequirePermission(authz.PermissionRBACRead), h.GetOperations)
	admin.GET("/scopes", middleware.RequirePermission(authz.PermissionRBACRead), h.ListScopes)
	admin.Use(middleware.RequireAllPermissions(authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite))
	admin.GET("/applications/:clientId/identities", h.ListIdentities)
	admin.GET("/applications/:clientId/identities/:oauthClientId/service-account", h.GetServiceAccount)
	admin.PUT("/applications/:clientId/identities/:oauthClientId", ratelimit.MiddlewareForPolicy(limiter, ratelimit.RBACSensitiveWritePolicy()), h.SaveIdentity)
	registerIdentitySyncRoutes(admin, h, limiter)
}
