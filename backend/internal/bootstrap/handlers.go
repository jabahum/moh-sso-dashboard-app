package bootstrap

import (
	"context"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/api/handler"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
	storepkg "github.com/moh-sso-dashboard/internal/db/sqlc"
	authfeature "github.com/moh-sso-dashboard/internal/features/auth"
	"github.com/moh-sso-dashboard/internal/features/authsession"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	dataqualityfeature "github.com/moh-sso-dashboard/internal/features/data_quality"
	documenttemplatesfeature "github.com/moh-sso-dashboard/internal/features/document_templates"
	documentsfeature "github.com/moh-sso-dashboard/internal/features/documents"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	platformfeature "github.com/moh-sso-dashboard/internal/features/platform"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	storagelocationfeature "github.com/moh-sso-dashboard/internal/features/storage_locations"
	surveillancefeature "github.com/moh-sso-dashboard/internal/features/surveillance"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	visualiserfeature "github.com/moh-sso-dashboard/internal/features/visualiser"
	"github.com/moh-sso-dashboard/internal/keycloak"
	db "github.com/moh-sso-dashboard/internal/migrate"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/moh-sso-dashboard/internal/storage"
	adminunitsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units"
	auditfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/audit"
	geojsonfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
	metricsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
	"github.com/redis/go-redis/v9"
)

type handlers struct {
	Router router.HandlerSet
	Health *handler.HealthHandler
}

type handlerDependencies struct {
	WorkerStatuses map[string]*core.WorkerStatus
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
	if deps.Config.CoreOnly() {
		return buildCoreHandlers(deps)
	}
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
	var metricsHandler *metricsfeature.Module
	if deps.Config.ModuleEnabled("metrics") {
		var err error
		metricsHandler, err = metricsfeature.NewModule(metricsfeature.Dependencies{
			Database: deps.Databases.Primary, RequireAdmin: modulehttp.RequireAdmin,
			AuthorizeAny: modulehttp.AuthorizeAny, RequestID: modulehttp.RequestID,
		})
		if err != nil {
			panic(err)
		}
	}
	// Preserve the optional-cache behavior when converting a concrete pointer to
	// the feature's interface: a typed nil pointer must not become a non-nil port.
	var auditCache auditfeature.Cache
	if deps.Cache != nil {
		auditCache = deps.Cache
	}
	var auditHandler *auditfeature.Module
	if deps.Config.ModuleEnabled("audit") {
		var err error
		auditHandler, err = auditfeature.NewModule(auditfeature.Dependencies{
			Database: deps.Databases.Primary, Cache: auditCache,
			RequireAdmin: modulehttp.RequireAdmin, AuthorizeAny: modulehttp.AuthorizeAny,
			LimitExport: modulehttp.PerUserLimit(ratelimit.New(deps.Redis)),
			RequestID:   modulehttp.RequestID, StreamError: modulehttp.StreamError,
		})
		if err != nil {
			panic(err)
		}
	}
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
	announcementHandler := buildAnnouncementsModule(deps)

	var adminunitsHandler *adminunitsfeature.Module
	if deps.Config.ModuleEnabled("admin-units") {
		var err error
		adminunitsHandler, err = adminunitsfeature.NewModule(adminunitsfeature.Dependencies{
			Database:      deps.Databases.DWH,
			RequireSystem: modulehttp.RequireSystem, AuthorizeAny: modulehttp.AuthorizeAny,
			SelectContext:   deps.Services.HealthContexts.SelectContextHTTP,
			ContextNodeCode: deps.Services.HealthContexts.NodeCode,
			ContextAliases:  deps.Services.HealthContexts.AliasIDs,
			RequestID:       modulehttp.RequestID,
		})
		if err != nil {
			panic(err)
		}
	}
	visualiserHandler := visualiserfeature.NewHandler(
		deps.Config,
		deps.Databases.DWH,
		deps.Services.HealthContexts,
	)
	var geoJSONHandler *geojsonfeature.Module
	if deps.Config.ModuleEnabled("geojson") {
		var err error
		geoJSONHandler, err = geojsonfeature.NewModule(geojsonfeature.Dependencies{AuthorizeAny: modulehttp.AuthorizeAny, RequestID: modulehttp.RequestID})
		if err != nil {
			panic(err)
		}
	}
	emailHandler := emailfeature.NewHandler(deps.Services.EmailFeature)
	rbacHandler := rbacfeature.NewHandler(deps.Services.RBAC)
	healthContextHandler := healthcontextfeature.NewHandler(
		deps.Services.HealthContexts,
		deps.Services.Audit,
	)
	authHandler.SetHealthContextResolver(deps.Services.HealthContexts)

	surveillanceHandler := surveillancefeature.NewHandler(
		deps.Services.EpiWeeks,
		deps.Services.Diseases,
		deps.Services.Locations,
		deps.Services.FacilityWeeklyMetrics,
		deps.Services.WeeklyStatus,
		deps.Services.Alerts,
		deps.Services.SurveillanceImport,
		deps.Services.HealthContexts,
	)
	if deps.Databases.observatory != nil {
		surveillanceHandler.SetAvailabilityCheck(deps.Databases.observatory.Ready)
	}

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
		func(ctx context.Context) error {
			return db.PingDB(ctx, deps.Databases.Primary)
		},
		func(ctx context.Context) error { return db.PingDB(ctx, deps.Databases.Remote) },
		func(ctx context.Context) error { return deps.AdminKeycloak.Authenticate() },
		deps.Redis,
	)

	if deps.Databases.observatory != nil {
		healthHandler.ObservatoryReady = deps.Databases.observatory.Ready
	}

	return handlers{
		Router: router.HandlerSet{
			Platform: &platformfeature.Handler{
				WorkerStatuses:   deps.WorkerStatuses,
				WorkerStaleAfter: 3 * deps.Config.NotificationDelivery.WorkerInterval,
				CoreReady:        healthHandler.CheckCore, RequiredWorkers: enabledDeliveryWorkers(deps.Config),
				Repository: &platformfeature.PostgresRepository{DB: deps.Databases.Primary},
				Verifier: &platformfeature.Introspector{
					Endpoint: strings.TrimRight(deps.Config.KeycloakTokenBaseURL(), "/") + "/realms/" + deps.Config.KeycloakRealm + "/protocol/openid-connect/token/introspect",
					Issuer:   strings.TrimRight(deps.Config.KeycloakExternalURL, "/") + "/realms/" + deps.Config.KeycloakRealm,
					Audience: deps.Config.PlatformAuthAudience, ClientID: deps.Config.PlatformAuthClientID, Secret: deps.Config.PlatformAuthClientSecret,
				},
			},
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
			HealthContexts:          healthContextHandler,
			ReportScheduler:         router.NewReportSchedulerModule(deps.Services.ReportScheduler),
		},
		Health: healthHandler,
	}
}
