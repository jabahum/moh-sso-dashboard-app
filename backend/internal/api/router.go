package router

import (
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/docs"

	"github.com/moh-sso-dashboard/internal/api/handler"
	"github.com/moh-sso-dashboard/internal/api/routes"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/config"
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
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	service "github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/module"
	adminunitsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units"
	announcementfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements"
	auditfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/audit"
	geojsonfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
	metricsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
	reportschedulerfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

type RouterDependencies struct {
	Modules        *module.Registry
	Config         *config.Config
	KeycloakClient *keycloak.Client
	Limiter        *ratelimit.Limiter
	AuditService   *service.AuditService
	AuthSessions   *authsession.Store
	AuthzResolver  authz.PermissionResolver
	Handlers       HandlerSet
	RateLimits     RateLimits
}

type HandlerSet struct {
	Platform                *platformfeature.Handler
	Auth                    *authfeature.Handler
	Clients                 *clientfeature.Handler
	Users                   *userfeature.Handler
	Metrics                 *metricsfeature.Module
	Audit                   *auditfeature.Module
	Notifications           *notificationsfeature.Handler
	Documents               *documentsfeature.Handler
	DocumentTemplates       *documenttemplatesfeature.Handler
	DocumentTemplateSheets  *documenttemplatesfeature.SheetHandler
	DocumentTemplateColumns *documenttemplatesfeature.ColumnHandler
	StorageLocations        *storagelocationfeature.Handler
	Sessions                *sessionfeature.Handler
	DataQuality             *dataqualityfeature.Handler
	Announcements           *announcementfeature.Module
	AdminUnits              *adminunitsfeature.Module
	Visualiser              *visualiserfeature.Handler
	Surveillance            *surveillancefeature.Handler
	GeoJSON                 *geojsonfeature.Module
	Email                   *emailfeature.Handler
	RBAC                    *rbacfeature.Handler
	HealthContexts          *healthcontextfeature.Handler
	ReportScheduler         *reportschedulerfeature.HTTPModule
}

type RateLimits struct {
	AuthenticatedPerMinute int
	AuthLoginPerMinute     int
	AuthCallbackPerMinute  int
	AuthSessionPerMinute   int
}

func SetupRouter(deps RouterDependencies) *gin.Engine {
	rateLimits := deps.RateLimits.withDefaults()

	r := gin.New()
	r.Use(middleware.RequestContext())
	r.Use(middleware.RequestLogger())
	r.Use(gin.Recovery())
	r.Use(cors.New(corsConfig(deps.Config)))

	routeDeps := routes.Dependencies{
		Modules:                 deps.Modules,
		Limiter:                 deps.Limiter,
		Auth:                    deps.Handlers.Auth,
		Clients:                 deps.Handlers.Clients,
		Users:                   deps.Handlers.Users,
		Metrics:                 deps.Handlers.Metrics,
		Audit:                   deps.Handlers.Audit,
		Notifications:           deps.Handlers.Notifications,
		Documents:               deps.Handlers.Documents,
		DocumentTemplates:       deps.Handlers.DocumentTemplates,
		DocumentTemplateSheets:  deps.Handlers.DocumentTemplateSheets,
		DocumentTemplateColumns: deps.Handlers.DocumentTemplateColumns,
		StorageLocations:        deps.Handlers.StorageLocations,
		Sessions:                deps.Handlers.Sessions,
		DataQuality:             deps.Handlers.DataQuality,
		AdminUnits:              deps.Handlers.AdminUnits,
		Visualiser:              deps.Handlers.Visualiser,
		Surveillance:            deps.Handlers.Surveillance,
		GeoJSON:                 deps.Handlers.GeoJSON,
		Email:                   deps.Handlers.Email,
		RBAC:                    deps.Handlers.RBAC,
		HealthContexts:          deps.Handlers.HealthContexts,

		AuthenticatedRateLimitPerMin:  rateLimits.AuthenticatedPerMinute,
		AuthLoginRateLimitPerMin:      rateLimits.AuthLoginPerMinute,
		AuthCallbackRateLimitPerMin:   rateLimits.AuthCallbackPerMinute,
		AuthSessionRateLimitPerMinute: rateLimits.AuthSessionPerMinute,
	}

	// LOCAL DEV ONLY: DEV_AUTH_BYPASS=true serves a synthetic admin session
	// and skips the Keycloak redirect entirely.
	if middleware.DevAuthBypassEnabled(deps.Config) {
		r.Use(middleware.DevAuthBypassRoutes())
	}

	api := r.Group("/api/v1")
	routes.RegisterAuthRoutes(api, routeDeps)

	protected := api.Group("")
	protected.Use(middleware.ExtractAuthContext(
		deps.KeycloakClient,
		deps.AuthSessions,
		deps.AuthzResolver,
		deps.Config,
	))
	protected.Use(middleware.RequireAuth())
	protected.Use(middleware.AuditMiddleware(deps.AuditService))
	protected.Use(ratelimit.MiddlewareForPolicy(
		deps.Limiter,
		ratelimit.AuthenticatedDefaultPolicy(routeDeps.AuthenticatedRateLimitPerMin),
	))

	if deps.Modules == nil && deps.Handlers.Announcements != nil && (deps.Config == nil || !deps.Config.CoreOnly()) {
		modulehttp.RegisterSurfaces(api, protected, deps.Handlers.Announcements)
	}
	platformfeature.RegisterRoutes(api, protected, deps.Handlers.Platform, deps.Limiter)
	if deps.Handlers.RBAC != nil {
		protected.GET("/applications/:clientId/manifest", deps.Handlers.RBAC.GetApplicationManifestRuntime)
	}
	if deps.Config != nil && deps.Config.CoreOnly() {
		sessionfeature.RegisterProtectedRoutes(protected, deps.Handlers.Sessions)
		rbacfeature.RegisterProtectedRoutes(protected, deps.Handlers.RBAC)
		rbacfeature.RegisterAdminRoutes(protected.Group("/admin"), deps.Handlers.RBAC, deps.Limiter)
		healthcontextfeature.RegisterProtectedRoutes(protected, deps.Handlers.HealthContexts)
		healthcontextfeature.RegisterAdminRoutes(protected.Group("/admin"), deps.Handlers.HealthContexts)
	} else {
		routes.RegisterProtectedRoutes(protected, routeDeps)
		if deps.Modules == nil {
			RegisterReportScheduler(api, protected, deps.Handlers.ReportScheduler, routeDeps.Limiter)
		}
		routes.RegisterAdminRoutes(protected, routeDeps)
	}
	if deps.Modules != nil {
		deps.Modules.RegisterRoutes(module.Routes{Public: api, Protected: protected, Admin: protected.Group("/admin")})
		registerModuleInventory(protected, deps.Modules)
	}
	docs.Register(r)

	return r
}

func RegisterHealthRoutes(r *gin.Engine, healthHandler *handler.HealthHandler) {
	r.GET("/health/live", healthHandler.HandleLive)
	r.GET("/health/ready", healthHandler.HandleReady)
	r.GET("/health", healthHandler.HandleHealth)
	r.GET("/version", healthHandler.HandleVersion)
}

func (limits RateLimits) withDefaults() RateLimits {
	if limits.AuthenticatedPerMinute == 0 {
		limits.AuthenticatedPerMinute = 300
	}
	if limits.AuthLoginPerMinute == 0 {
		limits.AuthLoginPerMinute = 20
	}
	if limits.AuthCallbackPerMinute == 0 {
		limits.AuthCallbackPerMinute = 30
	}
	if limits.AuthSessionPerMinute == 0 {
		limits.AuthSessionPerMinute = 60
	}

	return limits
}

func corsConfig(cfg *config.Config) cors.Config {
	origins := buildAllowedOrigins(cfg)

	return cors.Config{
		AllowOrigins: origins,
		AllowMethods: []string{
			httpMethodGet,
			httpMethodPost,
			httpMethodPut,
			httpMethodPatch,
			httpMethodDelete,
			httpMethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"X-CSRF-Token",
			"X-Request-ID",
			"X-Correlation-ID",
			"X-Health-Context-ID",
			"Cache-Control",
			"Pragma",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
			"X-Request-ID",
			"X-Correlation-ID",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}

func buildAllowedOrigins(cfg *config.Config) []string {
	seen := map[string]bool{}
	origins := make([]string, 0)

	add := func(origin string) {
		parsed, err := url.Parse(strings.TrimSpace(origin))
		if err != nil || parsed.Hostname() == "" || parsed.User != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			strings.Contains(parsed.Host, "*") {
			return
		}
		origin = strings.ToLower(parsed.Scheme + "://" + parsed.Host)

		if seen[origin] {
			return
		}

		seen[origin] = true
		origins = append(origins, origin)
	}

	// Local origins are defaults only in development, never deployed environments.
	if cfg == nil || cfg.IsDevelopment() {
		add("http://localhost:3000")
		add("http://localhost:5173")
		add("http://127.0.0.1:3000")
		add("http://127.0.0.1:5173")
	}

	if cfg != nil {
		add(cfg.FrontendBaseURL)
		add(cfg.FrontendRedirectURI)

		// Your production frontend.
		if cfg.IsProduction() || cfg.IsStaging() {
			add("https://dashboards.health.go.ug")
		}
	}

	return origins
}

const (
	httpMethodGet     = "GET"
	httpMethodPost    = "POST"
	httpMethodPut     = "PUT"
	httpMethodPatch   = "PATCH"
	httpMethodDelete  = "DELETE"
	httpMethodOptions = "OPTIONS"
)
