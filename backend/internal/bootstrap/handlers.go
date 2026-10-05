package bootstrap

import (
	"context"

	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/api/handler"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
	storepkg "github.com/moh-sso-dashboard/internal/db/sqlc"
	adminunitsfeature "github.com/moh-sso-dashboard/internal/features/admin_units"
	announcementfeature "github.com/moh-sso-dashboard/internal/features/announcements"
	auditfeature "github.com/moh-sso-dashboard/internal/features/audit"
	authfeature "github.com/moh-sso-dashboard/internal/features/auth"
	"github.com/moh-sso-dashboard/internal/features/authsession"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	dataqualityfeature "github.com/moh-sso-dashboard/internal/features/data_quality"
	documenttemplatesfeature "github.com/moh-sso-dashboard/internal/features/document_templates"
	documentsfeature "github.com/moh-sso-dashboard/internal/features/documents"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	geojsonfeature "github.com/moh-sso-dashboard/internal/features/geojson"
	metricsfeature "github.com/moh-sso-dashboard/internal/features/metrics"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	reportschedulerfeature "github.com/moh-sso-dashboard/internal/features/report_scheduler"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	storagelocationfeature "github.com/moh-sso-dashboard/internal/features/storage_locations"
	surveillancefeature "github.com/moh-sso-dashboard/internal/features/surveillance"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	visualiserfeature "github.com/moh-sso-dashboard/internal/features/visualiser"
	"github.com/moh-sso-dashboard/internal/keycloak"
	db "github.com/moh-sso-dashboard/internal/migrate"
	"github.com/moh-sso-dashboard/internal/storage"
	"github.com/redis/go-redis/v9"
)

type handlers struct {
	Router router.HandlerSet
	Health *handler.HealthHandler
}

type handlerDependencies struct {
	Config         *config.Config
	Store          storepkg.Store
	Cache          *cache.RedisCache
	Databases      databases
	Repositories   repositories
	Services       services
	FileStorage    storage.Storage
	StorageFactory *storage.StorageFactory
	AdminKeycloak  *keycloak.KeyAdminClient
	Redis          *redis.Client
	AuthSessions   *authsession.Store
	AuthzResolver  authz.PermissionResolver
}

func buildHandlers(deps handlerDependencies) handlers {
	authHandler := authfeature.NewHandler(
		deps.Services.Auth,
		deps.Services.Audit,
		deps.Services.Notifications,
		deps.AuthSessions,
		deps.Config,
		deps.AuthzResolver,
	)
	clientHandler := clientfeature.NewHandler(deps.Services.Clients, deps.Services.Audit, deps.Cache)
	userHandler := userfeature.NewHandler(deps.Services.Users, deps.Services.Audit, deps.Cache)
	metricsHandler := metricsfeature.NewHandler(deps.Services.Metrics)
	auditHandler := auditfeature.NewHandler(auditfeature.NewService(auditfeature.NewRepository(deps.Store)), deps.Cache)
	notificationsHandler := notificationsfeature.NewHandler(deps.Services.Notifications, deps.Services.Audit)

	documentHandler := documentsfeature.NewHandler(
		deps.Services.Documents,
		deps.Services.Audit,
		deps.Services.StorageLocations,
		deps.FileStorage,
		deps.StorageFactory,
		deps.Databases.Primary,
		deps.Databases.Remote,
	)

	storageLocationHandler := storagelocationfeature.NewHandler(
		deps.Services.StorageLocations,
		deps.Services.Audit,
	)
	sessionHandler := sessionfeature.NewHandler(deps.Services.Sessions)
	dataQualityHandler := dataqualityfeature.NewHandler(deps.Databases.DWH, deps.Databases.DQA, deps.Databases.Primary, deps.AdminKeycloak, deps.Services.Email, deps.Config)
	announcementHandler := announcementfeature.NewHandler(
		deps.Services.Announcements,
		deps.Services.Audit,
	)
	adminunitsHandler := adminunitsfeature.NewHandler(deps.Config, deps.Databases.DWH)
	visualiserHandler := visualiserfeature.NewHandler(deps.Config, deps.Databases.DWH)
	geoJSONHandler := geojsonfeature.NewHandler("./assets/geojson")
	emailHandler := emailfeature.NewHandler(deps.Services.EmailFeature)
	rbacHandler := rbacfeature.NewHandler(deps.Services.RBAC)

	surveillanceHandler := surveillancefeature.NewHandler(
		deps.Services.EpiWeeks,
		deps.Services.Diseases,
		deps.Services.Locations,
		deps.Services.FacilityWeeklyMetrics,
		deps.Services.WeeklyStatus,
		deps.Services.Alerts,
		deps.Services.SurveillanceImport,
	)

	documentTemplateHandler := documenttemplatesfeature.NewHandler(
		deps.Services.DocumentTemplates,
		deps.Services.DocumentTemplateSheets,
		deps.Services.DocumentTemplateColumns,
		deps.Databases.Primary,
		deps.Databases.Remote,
	)
	documentTemplateSheetHandler := documenttemplatesfeature.NewSheetHandler(
		deps.Services.DocumentTemplateSheets,
	)
	documentTemplateColumnHandler := documenttemplatesfeature.NewColumnHandler(
		deps.Services.DocumentTemplateColumns,
	)

	healthHandler := handler.NewHealthHandler(
		func(ctx context.Context) error { return db.PingDB(ctx, deps.Databases.Primary) },
		func(ctx context.Context) error { return db.PingDB(ctx, deps.Databases.Remote) },
		func(ctx context.Context) error { return deps.AdminKeycloak.Authenticate() },
		deps.Redis,
	)

	return handlers{
		Router: router.HandlerSet{
			Auth:                    authHandler,
			Clients:                 clientHandler,
			Users:                   userHandler,
			Metrics:                 metricsHandler,
			Audit:                   auditHandler,
			Notifications:           notificationsHandler,
			Documents:               documentHandler,
			DocumentTemplates:       documentTemplateHandler,
			DocumentTemplateSheets:  documentTemplateSheetHandler,
			DocumentTemplateColumns: documentTemplateColumnHandler,
			StorageLocations:        storageLocationHandler,
			Sessions:                sessionHandler,
			DataQuality:             dataQualityHandler,
			Announcements:           announcementHandler,
			AdminUnits:              adminunitsHandler,
			Visualiser:              visualiserHandler,
			Surveillance:            surveillanceHandler,
			GeoJSON:                 geoJSONHandler,
			Email:                   emailHandler,
			RBAC:                    rbacHandler,
			ReportScheduler:         reportschedulerfeature.NewHandler(reportschedulerfeature.NewService()),
		},
		Health: healthHandler,
	}
}
