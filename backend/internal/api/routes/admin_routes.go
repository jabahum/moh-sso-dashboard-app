package routes

import (
	"github.com/gin-gonic/gin"

	announcementfeature "github.com/moh-sso-dashboard/internal/features/announcements"
	auditfeature "github.com/moh-sso-dashboard/internal/features/audit"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	metricsfeature "github.com/moh-sso-dashboard/internal/features/metrics"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/module"
)

func RegisterAdminRoutes(protected *gin.RouterGroup, deps Dependencies) {
	admin := protected.Group("/admin")

	registerAdminUserRoutes(admin, deps)
	registerAdminClientRoleRoutes(admin, deps)
	if deps.Modules != nil {
		deps.Modules.RegisterRoutes(module.Routes{Protected: protected, Admin: admin})
	} else {
		registerAdminMetricRoutes(admin, deps)
	}
	registerAdminAuditRoutes(admin, deps)
	registerAdminNotificationRoutes(admin, deps)
	registerAdminAnnouncementRoutes(admin, deps)
	registerAdminRBACRoutes(admin, deps)
	registerAdminHealthContextRoutes(admin, deps)
}

func registerAdminHealthContextRoutes(admin *gin.RouterGroup, deps Dependencies) {
	healthcontextfeature.RegisterAdminRoutes(admin, deps.HealthContexts)
}

func registerAdminUserRoutes(admin *gin.RouterGroup, deps Dependencies) {
	userfeature.RegisterAdminRoutes(admin, deps.Users, deps.Limiter)
}

func registerAdminClientRoleRoutes(admin *gin.RouterGroup, deps Dependencies) {
	clientfeature.RegisterAdminRoutes(admin, deps.Clients)
}

func registerAdminMetricRoutes(admin *gin.RouterGroup, deps Dependencies) {
	metricsfeature.RegisterAdminRoutes(admin, deps.Metrics)
}

func registerAdminAuditRoutes(admin *gin.RouterGroup, deps Dependencies) {
	auditfeature.RegisterAdminRoutes(
		admin,
		deps.Audit,
		deps.Limiter,
	)
}

func registerAdminNotificationRoutes(admin *gin.RouterGroup, deps Dependencies) {
	notificationsfeature.RegisterAdminRoutes(admin, deps.Notifications, deps.Limiter)
}

func registerAdminAnnouncementRoutes(admin *gin.RouterGroup, deps Dependencies) {
	announcementfeature.RegisterAdminRoutes(admin, deps.Announcements, deps.Limiter)
}

func registerAdminRBACRoutes(admin *gin.RouterGroup, deps Dependencies) {
	rbacfeature.RegisterAdminRoutes(admin, deps.RBAC, deps.Limiter)
}
