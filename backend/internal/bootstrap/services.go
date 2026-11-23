package bootstrap

import (
	kitadapter "github.com/moh-sso-dashboard/internal/adapters/modulekit"
	"net/url"
	"strings"

	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
	storepkg "github.com/moh-sso-dashboard/internal/db/sqlc"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/internal/keycloak"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/service"
	importsvc "github.com/moh-sso-dashboard/internal/service/import"
	"github.com/moh-sso-dashboard/internal/storage"
	"github.com/moh-sso-dashboard/internal/worker"
	documenttemplatesmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates"
	reportschedulerfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
	storagelocationfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/storage-locations"
	surveillancemodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance"
	"github.com/redis/go-redis/v9"
)

type services struct {
	Email                   service.EmailService
	EmailFeature            *emailfeature.Service
	SMTP                    worker.EmailSender
	SMS                     service.SMSService
	Notifications           service.NotificationsService
	Auth                    service.AuthService
	Audit                   *service.AuditService
	StorageLocations        storagelocationfeature.Service
	StorageLocationsModule  *storagelocationfeature.Module
	Sessions                sessionfeature.Service
	Clients                 *clientfeature.Service
	Users                   *userfeature.Service
	SurveillanceModule      *surveillancemodule.Module
	DocumentTemplatesModule *documenttemplatesmodule.Module
	Import                  *importsvc.Service
	RBAC                    *rbacfeature.Service
	HealthContexts          *healthcontextfeature.Service
	ReportScheduler         *reportschedulerfeature.Service
}

type serviceDependencies struct {
	Config        *config.Config
	Store         storepkg.Store
	Cache         *cache.RedisCache
	CacheClient   *redis.Client
	Databases     databases
	Repositories  repositories
	FileStorage   storage.Storage
	Logger        *logger.Logger
	AdminKeycloak *keycloak.KeyAdminClient
}

func buildServices(deps serviceDependencies) services {
	if deps.Config.CoreOnly() {
		return buildCoreServices(deps)
	}

	templateManager, err := service.NewTemplateManager(deps.Logger)
	if err != nil {
		deps.Logger.Fatal("Failed to initialize email template manager: ", err)
	}

	if err := service.RegisterDefaultTemplates(templateManager); err != nil {
		deps.Logger.Fatal("Failed to register default email templates: ", err)
	}

	deps.Logger.Info("Default email templates registered")

	smtpService, err := service.NewSMTPService(deps.Config, templateManager, deps.Logger)
	if err != nil {
		deps.Logger.Fatal("Failed to initialize SMTP service: ", err)
	}

	queueService, err := service.NewQueueService(deps.Repositories.Email, deps.Logger)
	if err != nil {
		deps.Logger.Fatal("Failed to initialize email queue service: ", err)
	}

	emailService, err := service.NewEmailService(smtpService, queueService, templateManager, deps.Config)
	if err != nil {
		deps.Logger.Fatal("Failed to initialize email application service: ", err)
	}

	deps.Logger.Info("Email application service initialized")

	smsService, err := service.NewSMSService(deps.Config, deps.Logger)
	if err != nil {
		deps.Logger.Fatal("Failed to initialize SMS service: ", err)
	}

	if deps.Config.SMS.Enabled {
		deps.Logger.Info("SMS service initialized", "provider", deps.Config.SMS.Provider)
	} else {
		deps.Logger.Info("SMS service initialized in disabled mode")
	}

	emailFeatureService := emailfeature.NewService(emailService, deps.Repositories.Email)
	emailFeatureService.SetRBACRepository(deps.Repositories.RBAC)
	emailFeatureService.SetUserRepository(deps.Repositories.Users)

	reportSchedulerService := reportschedulerfeature.NewService(
		reportschedulerfeature.NewRepository(deps.Databases.Primary),
		reportschedulerfeature.NewKeycloakHealthBIClient(
			deps.Config.HealthBIBaseURL,
			strings.TrimRight(deps.Config.KeycloakBaseURL, "/")+"/realms/"+url.PathEscape(deps.Config.KeycloakRealm)+"/protocol/openid-connect/token",
			deps.Config.HealthBIKeycloakClientID,
			deps.Config.HealthBIKeycloakClientSecret,
			deps.Config.HealthBITimeout,
		),
		reportEmailAdapter{emailFeatureService},
		reportUserAdapter{deps.Repositories.Users},
		deps.FileStorage,
		reportschedulerfeature.NewPostgresHealthHierarchy(deps.Databases.DWH),
	)

	publisher := cache.NewNotificationPublisher(deps.CacheClient)
	notificationsService := service.NewNotificationsService(
		deps.Config,
		deps.Repositories.Notifications,
		deps.Repositories.NotificationDelivery,
		deps.Repositories.NotificationPreferences,
		publisher,
	)

	authService := service.NewAuthService(deps.Repositories.Auth, deps.CacheClient)
	auditService := service.NewAuditService(
		deps.Store,
		deps.Cache,
		notificationsService,
		deps.Config,
	)
	reportSchedulerService.SetNotifications(reportNotificationAdapter{notificationsService})
	reportSchedulerService.SetAudit(moduleAudit{recorder: auditService})
	reportSchedulerService.SetPublicBaseURL(deps.Config.AppBaseURL)
	reportSchedulerService.SetDeliveryLinkTTL(deps.Config.ReportSchedulerDeliveryLinkTTL)
	reportSchedulerService.SetEmailAttachmentLimit(deps.Config.Email.MaxAttachmentBytes)

	storageLocationsModule, err := storagelocationfeature.NewModule(storagelocationfeature.Dependencies{
		Database: deps.Databases.Primary,
		Notifier: kitadapter.Notifier{Service: notificationsService},
		Platform: kitadapter.Platform(deps.Config),
	})
	if err != nil {
		panic(err)
	}
	storageLocationService := storageLocationsModule.Service

	sessionService := sessionfeature.NewService(deps.Repositories.Sessions)
	clientService := clientfeature.NewService(
		deps.Repositories.Clients,
		notificationsService,
		deps.Config,
	)

	userService := userfeature.NewService(
		deps.Repositories.Users,
		notificationsService,
		deps.Config,
	)

	documentTemplatesModule := buildDocumentTemplatesModule(deps, notificationsService)

	healthContextService := healthcontextfeature.NewService(deps.Repositories.HealthContexts)
	surveillanceModule := buildSurveillanceModule(deps, notificationsService, healthContextService)
	var surveillanceJobs importsvc.SurveillanceJobs
	if surveillanceModule != nil {
		surveillanceJobs = surveillanceModule
	}

	importService := importsvc.NewService(
		deps.Repositories.Documents,
		deps.Repositories.Processes,
		deps.Repositories.DocumentFiles,
		documentTemplatesModule.Service,
		surveillanceJobs,
		deps.FileStorage,
		deps.Databases.Remote,
	)

	rbacService := rbacfeature.NewService(deps.Repositories.RBAC, userService)
	userService.SetServiceOnlyRoleChecker(rbacService)
	userService.SetAccessRoleRemovalHook(newIdentitySyncRevocationHook(deps.Config, deps.Databases.Primary, deps.AdminKeycloak))
	rbacService.SetKeycloakGroupMembershipManager(deps.AdminKeycloak)
	rbacService.SetFrontendBaseURL(deps.Config.FrontendBaseURL)
	emailFeatureService.SetHealthContextService(healthContextService)

	return services{
		Email:                   emailService,
		EmailFeature:            emailFeatureService,
		SMTP:                    smtpService,
		SMS:                     smsService,
		Notifications:           notificationsService,
		Auth:                    authService,
		Audit:                   auditService,
		StorageLocations:        storageLocationService,
		StorageLocationsModule:  storageLocationsModule,
		Sessions:                sessionService,
		Clients:                 clientService,
		Users:                   userService,
		SurveillanceModule:      surveillanceModule,
		DocumentTemplatesModule: documentTemplatesModule,
		Import:                  importService,
		RBAC:                    rbacService,
		HealthContexts:          healthContextService,
		ReportScheduler:         reportSchedulerService,
	}
}
