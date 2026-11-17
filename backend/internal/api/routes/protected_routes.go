package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/moh-sso-dashboard/internal/authz"
	announcementfeature "github.com/moh-sso-dashboard/internal/features/announcements"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	dataqualityfeature "github.com/moh-sso-dashboard/internal/features/data_quality"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	geojsonfeature "github.com/moh-sso-dashboard/internal/features/geojson"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	reportschedulerfeature "github.com/moh-sso-dashboard/internal/features/report_scheduler"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	storagelocationfeature "github.com/moh-sso-dashboard/internal/features/storage_locations"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/internal/middleware"
)

func RegisterProtectedRoutes(protected *gin.RouterGroup, deps Dependencies) {
	registerEmailRoutes(protected, deps)
	registerAnnouncementRoutes(protected, deps)
	if deps.Modules == nil {
		registerGeoJSONRoutes(protected, deps)
	}
	registerClientRoutes(protected, deps)
	registerUserRoutes(protected, deps)
	registerStorageLocationRoutes(protected, deps)
	registerSessionRoutes(protected, deps)
	registerNotificationRoutes(protected, deps)

	contextScoped := protected.Group("")
	contextScoped.Use(middleware.RequireSystem(authz.SystemDataStatistics))
	contextScoped.Use(deps.HealthContexts.RequireAccessibleSelection())
	registerDocumentRoutes(contextScoped, deps)
	registerDataQualityRoutes(contextScoped, deps)
	registerVisualiserRoutes(contextScoped, deps)
	registerSurveillanceRoutes(contextScoped, deps)

	registerRBACProtectedRoutes(protected, deps)
	registerHealthContextProtectedRoutes(protected, deps)
	reportschedulerfeature.RegisterProtectedRoutes(protected, deps.ReportScheduler)
}

func registerHealthContextProtectedRoutes(protected *gin.RouterGroup, deps Dependencies) {
	healthcontextfeature.RegisterProtectedRoutes(protected, deps.HealthContexts)
}

func registerAnnouncementRoutes(protected *gin.RouterGroup, deps Dependencies) {
	announcementfeature.RegisterProtectedRoutes(protected, deps.Announcements)
}

func registerEmailRoutes(protected *gin.RouterGroup, deps Dependencies) {
	emailfeature.RegisterProtectedRoutes(protected, deps.Email, deps.Limiter)
}

func registerGeoJSONRoutes(protected *gin.RouterGroup, deps Dependencies) {
	geojsonfeature.RegisterProtectedRoutes(protected, deps.GeoJSON)
}

func registerClientRoutes(protected *gin.RouterGroup, deps Dependencies) {
	clientfeature.RegisterProtectedRoutes(protected, deps.Clients)
}

func registerUserRoutes(protected *gin.RouterGroup, deps Dependencies) {
	userfeature.RegisterProtectedRoutes(protected, deps.Users)
}

func registerStorageLocationRoutes(protected *gin.RouterGroup, deps Dependencies) {
	storagelocationfeature.RegisterProtectedRoutes(protected, deps.StorageLocations)
}

func registerSessionRoutes(protected *gin.RouterGroup, deps Dependencies) {
	sessionfeature.RegisterProtectedRoutes(protected, deps.Sessions)
}

func registerDataQualityRoutes(protected *gin.RouterGroup, deps Dependencies) {
	dataqualityfeature.RegisterProtectedRoutes(protected, deps.DataQuality)
}

func registerRBACProtectedRoutes(protected *gin.RouterGroup, deps Dependencies) {
	rbacfeature.RegisterProtectedRoutes(protected, deps.RBAC)
}

func registerNotificationRoutes(protected *gin.RouterGroup, deps Dependencies) {
	notificationsfeature.RegisterProtectedRoutes(protected, deps.Notifications)
}
