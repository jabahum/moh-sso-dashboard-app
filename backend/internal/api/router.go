package router

import (
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/moh-sso-dashboard/internal/api/handler"
	"github.com/moh-sso-dashboard/internal/api/routes"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/config"
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
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	service "github.com/moh-sso-dashboard/internal/service"
)

type RouterDependencies struct {
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
	Auth                    *authfeature.Handler
	Clients                 *clientfeature.Handler
	Users                   *userfeature.Handler
	Metrics                 *metricsfeature.Handler
	Audit                   *auditfeature.Handler
	Notifications           *notificationsfeature.Handler
	Documents               *documentsfeature.Handler
	DocumentTemplates       *documenttemplatesfeature.Handler
	DocumentTemplateSheets  *documenttemplatesfeature.SheetHandler
	DocumentTemplateColumns *documenttemplatesfeature.ColumnHandler
	StorageLocations        *storagelocationfeature.Handler
	Sessions                *sessionfeature.Handler
	DataQuality             *dataqualityfeature.Handler
	Announcements           *announcementfeature.Handler
	AdminUnits              *adminunitsfeature.Handler
	Visualiser              *visualiserfeature.Handler
	Surveillance            *surveillancefeature.Handler
	GeoJSON                 *geojsonfeature.Handler
	Email                   *emailfeature.Handler
	RBAC                    *rbacfeature.Handler
	ReportScheduler         *reportschedulerfeature.Handler
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
		Limiter:                       deps.Limiter,
		Auth:                          deps.Handlers.Auth,
		Clients:                       deps.Handlers.Clients,
		Users:                         deps.Handlers.Users,
		Metrics:                       deps.Handlers.Metrics,
		Audit:                         deps.Handlers.Audit,
		Notifications:                 deps.Handlers.Notifications,
		Documents:                     deps.Handlers.Documents,
		DocumentTemplates:             deps.Handlers.DocumentTemplates,
		DocumentTemplateSheets:        deps.Handlers.DocumentTemplateSheets,
		DocumentTemplateColumns:       deps.Handlers.DocumentTemplateColumns,
		StorageLocations:              deps.Handlers.StorageLocations,
		Sessions:                      deps.Handlers.Sessions,
		DataQuality:                   deps.Handlers.DataQuality,
		Announcements:                 deps.Handlers.Announcements,
		AdminUnits:                    deps.Handlers.AdminUnits,
		Visualiser:                    deps.Handlers.Visualiser,
		Surveillance:                  deps.Handlers.Surveillance,
		GeoJSON:                       deps.Handlers.GeoJSON,
		Email:                         deps.Handlers.Email,
		RBAC:                          deps.Handlers.RBAC,
		ReportScheduler:               deps.Handlers.ReportScheduler,
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
	routes.RegisterPublicAnnouncementRoutes(api, routeDeps)

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

	routes.RegisterProtectedRoutes(protected, routeDeps)
	routes.RegisterAdminRoutes(protected, routeDeps)

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
