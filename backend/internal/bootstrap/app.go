package bootstrap

import (
	"context"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/config"
	storepkg "github.com/moh-sso-dashboard/internal/db/sqlc"
	"github.com/moh-sso-dashboard/internal/features/authsession"
	"github.com/moh-sso-dashboard/internal/geography"
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
		Str("profile", cfg.PlatformProfile).
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
	// Do not explicitly close pools underneath a worker that failed to join.
	// Process termination still reclaims them after an unsuccessful shutdown.
	resourcesJoined := true
	defer func() {
		if resourcesJoined {
			dbs.Close()
		}
	}()

	// ==================================================
	// REDIS
	// ==================================================
	cacheRuntime := initCache(ctx, cfg)
	rdb := cacheRuntime.Redis
	defer func() {
		if resourcesJoined {
			rdb.Close()
		}
	}()
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
	var fileStorage storage.Storage
	var storageFactory *storage.StorageFactory
	if !cfg.CoreOnly() {
		fileStorage, err = storage.NewFileStorage(cfg.StorageProvider, cfg)
		if err != nil {
			appLogger.Fatal("Storage initialization failed: ", err)
		}
		storageFactory = storage.NewStorageFactory(cfg)
	}

	// ==================================================
	// STORE
	// ==================================================
	store := storepkg.NewStore(dbs.Primary)

	// ==================================================
	// REPOSITORIES
	// ==================================================
	var observatoryStore storepkg.Store
	if dbs.Observatory != nil && dbs.registry != nil {
		observatoryStore = geography.NewStore(storepkg.NewStore(dbs.Observatory), dbs.Primary, dbs.registry.Sync)
	}
	repos := buildRepositories(cfg, store, observatoryStore, adminKC, webKC, appLogger)

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
	workerStatuses := make(map[string]*core.WorkerStatus)
	for _, name := range enabledDeliveryWorkers(cfg) {
		workerStatuses[name] = &core.WorkerStatus{}
	}
	handlers := buildHandlers(handlerDependencies{
		Config:         cfg,
		WorkerStatuses: workerStatuses,
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

	modules, err := router.NewModuleRegistry(handlers.Router, cfg.EffectiveDisabledModules(), rateLimiter, schedulerWorker(cfg, services, appLogger))
	if err != nil {
		panic("invalid module configuration: " + err.Error())
	}
	stopModules, err := modules.Initialize(ctx)
	if err != nil {
		panic("module initialization failed: " + err.Error())
	}
	services.RBAC.ConfigureManifests(cfg.Environment, modules.Inventory())
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := stopModules(shutdownCtx); err != nil {
			if shutdownCtx.Err() != nil {
				resourcesJoined = false
			}
			appLogger.Error("Module shutdown failed: ", err)
		}
	}()

	stopWorkers, err := startBackgroundWorkers(ctx, workerDependencies{
		Statuses:                       workerStatuses,
		PrimaryDB:                      dbs.Primary,
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

	if err != nil {
		panic("worker initialization failed: " + err.Error())
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := stopWorkers(shutdownCtx); err != nil {
			if shutdownCtx.Err() != nil {
				resourcesJoined = false
			}
			appLogger.Error("Worker shutdown failed: ", err)
		}
	}()

	services.Notifications.NotifySystemStartup(ctx, cfg.Environment)

	// ==================================================
	// ROUTER
	// ==================================================
	r := router.SetupRouter(router.RouterDependencies{
		Modules:        modules,
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
