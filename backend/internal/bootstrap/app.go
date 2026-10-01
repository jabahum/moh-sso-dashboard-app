package bootstrap

import (
	"context"

	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/config"
	storepkg "github.com/moh-sso-dashboard/internal/db/sqlc"
	"github.com/moh-sso-dashboard/internal/features/authsession"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/storage"
	"github.com/moh-sso-dashboard/internal/version"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Run() {
	// ==================================================
	// Root Context
	// ==================================================
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ==================================================
	// CONFIG
	// ==================================================
	cfg, err := config.LoadConfig(".")
	if err != nil {
		panic("cannot load config: " + err.Error())
	}

	appLogger := logger.NewLogger()
	appLogger.SetLevel(zerolog.InfoLevel)
	build := version.Get()
	log.Info().
		Str("service", build.Service).
		Str("version", build.Version).
		Str("commit", build.Commit).
		Str("build_time", build.BuildTime).
		Bool("dirty", build.Dirty).
		Str("go_version", build.GoVersion).
		Str("environment", cfg.Environment).
		Msg("starting MOH SSO Dashboard")

	appLogger.Info(
		"notification config loaded",
		"platform_name", cfg.Notification.PlatformName,
		"system_admin_name", cfg.Notification.SystemAdminName,
		"system_admin_email", cfg.Notification.SystemAdminEmail,
		"admin_dashboard_url", cfg.Notification.AdminDashboardURL,
	)

	appLogger.Info(
		"smtp config loaded",
		"smtp_host", cfg.SMTP.Host,
		"smtp_port", cfg.SMTP.Port,
		"smtp_from_email", cfg.SMTP.FromEmail,
		"smtp_from_name", cfg.SMTP.FromName,
	)

	// ==================================================
	// DATABASES
	// ==================================================
	dbs, err := initDatabases(ctx, cfg)
	if err != nil {
		appLogger.Fatal("Failed to initialize databases: ", err)
	}
	if dbs.Primary == nil {
		appLogger.Fatal("Failed to initialize databases: primary database connection is nil")
	}
	defer dbs.Close()

	// ==================================================
	// REDIS
	// ==================================================
	cacheRuntime := initCache(ctx, cfg)
	rdb := cacheRuntime.Redis
	cacheAdapter := cacheRuntime.Cache
	rateLimiter := cacheRuntime.RateLimiter

	// Server-side auth sessions: tokens live in Redis, browsers carry a
	// single opaque cookie.
	authSessions := authsession.NewStore(rdb)

	// ==================================================
	// KEYCLOAK
	// ==================================================
	keycloakClients, err := initKeycloak(cfg, cacheAdapter)
	if err != nil {
		// LOCAL DEV ONLY: with DEV_AUTH_BYPASS=true the portal runs without
		// Keycloak, so a failed admin auth is a warning instead of fatal.
		// Endpoints that call Keycloak directly (user/client admin) will error.
		if middleware.DevAuthBypassEnabled(cfg) {
			appLogger.Warn("DEV_AUTH_BYPASS enabled - starting without Keycloak: ", err)
		} else {
			appLogger.Fatal("Keycloak admin authentication failed: ", err)
		}
	}
	adminKC := keycloakClients.Admin
	webKC := keycloakClients.Web

	// ==================================================
	// STORAGE
	// ==================================================
	fileStorage, err := storage.NewFileStorage(cfg.StorageProvider, cfg)
	if err != nil {
		appLogger.Fatal("Storage initialization failed: ", err)
	}

	storageFactory := storage.NewStorageFactory(cfg)

	// ==================================================
	// STORE
	// ==================================================
	store := storepkg.NewStore(dbs.Primary)

	// ==================================================
	// REPOSITORIES
	// ==================================================
	repos := buildRepositories(cfg, store, adminKC, webKC, appLogger)

	// ==================================================
	// SERVICES
	// ==================================================
	services := buildServices(serviceDependencies{
		Config:        cfg,
		Store:         store,
		Cache:         cacheAdapter,
		CacheClient:   rdb,
		Databases:     dbs,
		Repositories:  repos,
		FileStorage:   fileStorage,
		Logger:        appLogger,
		AdminKeycloak: adminKC,
	})

	if err := runStartupRBACSync(ctx, cfg, dbs.Primary, services, adminKC, appLogger); err != nil {
		if cfg.RBACStartupSyncFailOnError {
			appLogger.Fatal("RBAC startup sync failed: ", err)
		}
		appLogger.Warn("RBAC startup sync failed", "error", err)
	}

	authzResolver := authz.NewCompositeResolver(
		authz.NewDBResolver(dbs.Primary),
		authz.NewStaticResolver(),
	)

	// ==================================================
	// HANDLERS
	// ==================================================
	handlers := buildHandlers(handlerDependencies{
		Config:         cfg,
		Store:          store,
		Cache:          cacheAdapter,
		Databases:      dbs,
		Repositories:   repos,
		Services:       services,
		AuthSessions:   authSessions,
		FileStorage:    fileStorage,
		StorageFactory: storageFactory,
		AdminKeycloak:  adminKC,
		Redis:          rdb,
		AuthzResolver:  authzResolver,
	})

	startBackgroundWorkers(ctx, workerDependencies{
		Config:                         cfg,
		ProcessRepository:              repos.Processes,
		ImportService:                  services.Import,
		FileStorage:                    fileStorage,
		EmailRepository:                repos.Email,
		SMTPService:                    services.SMTP,
		NotificationDeliveryRepository: repos.NotificationDelivery,
		EmailService:                   services.Email,
		SMSService:                     services.SMS,
		AuditService:                   services.Audit,
		Logger:                         appLogger,
		DQADB:                          dbs.DQA,
		DWHDB:                          dbs.DWH,
	})

	services.Notifications.NotifySystemStartup(ctx, cfg.Environment)

	// ==================================================
	// ROUTER
	// ==================================================
	r := router.SetupRouter(router.RouterDependencies{
		Config:         cfg,
		KeycloakClient: webKC,
		Limiter:        rateLimiter,
		AuditService:   services.Audit,
		AuthzResolver:  authzResolver,
		Handlers:       handlers.Router,
		AuthSessions:   authSessions,
	})

	router.RegisterHealthRoutes(r, handlers.Health)

	runHTTPServer(cancel, cfg, appLogger, services.Notifications, r)
}
