package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/moh-sso-dashboard/internal/authz"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/modulehttp"
)

func RegisterProtectedRoutes(protected *gin.RouterGroup, deps Dependencies) {
	registerEmailRoutes(protected, deps)

	if deps.Modules == nil {
		registerGeoJSONRoutes(protected, deps)
	}
	registerClientRoutes(protected, deps)
	registerUserRoutes(protected, deps)
	registerSessionRoutes(protected, deps)
	registerNotificationRoutes(protected, deps)

	contextScoped := ContextScopedGroup(protected, deps.HealthContexts)
	registerVisualiserRoutes(contextScoped, deps)

	registerRBACProtectedRoutes(protected, deps)
	registerHealthContextProtectedRoutes(protected, deps)
}

func registerHealthContextProtectedRoutes(protected *gin.RouterGroup, deps Dependencies) {
	healthcontextfeature.RegisterProtectedRoutes(protected, deps.HealthContexts)
}

func registerEmailRoutes(protected *gin.RouterGroup, deps Dependencies) {
	emailfeature.RegisterProtectedRoutes(protected, deps.Email, deps.Limiter)
}

func registerGeoJSONRoutes(protected *gin.RouterGroup, deps Dependencies) {
	if deps.GeoJSON != nil {
		modulehttp.Register(protected, deps.GeoJSON)
	}
}

func registerClientRoutes(protected *gin.RouterGroup, deps Dependencies) {
	clientfeature.RegisterProtectedRoutes(protected, deps.Clients)
}

func registerUserRoutes(protected *gin.RouterGroup, deps Dependencies) {
	userfeature.RegisterProtectedRoutes(protected, deps.Users)
}

func registerSessionRoutes(protected *gin.RouterGroup, deps Dependencies) {
	sessionfeature.RegisterProtectedRoutes(protected, deps.Sessions)
}

func registerRBACProtectedRoutes(protected *gin.RouterGroup, deps Dependencies) {
	rbacfeature.RegisterProtectedRoutes(protected, deps.RBAC)
}

func registerNotificationRoutes(protected *gin.RouterGroup, deps Dependencies) {
	notificationsfeature.RegisterProtectedRoutes(protected, deps.Notifications)
}

// ContextScopedGroup retains the shared system and selected-context boundary.
func ContextScopedGroup(protected *gin.RouterGroup, contexts *healthcontextfeature.Handler) *gin.RouterGroup {
	group := protected.Group("")
	group.Use(middleware.RequireSystem(authz.SystemDataStatistics))
	group.Use(contexts.RequireAccessibleSelection())
	return group
}
