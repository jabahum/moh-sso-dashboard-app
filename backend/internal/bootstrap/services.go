package bootstrap

import (
	"net/url"
	"strings"

	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
	storepkg "github.com/moh-sso-dashboard/internal/db/sqlc"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	documenttemplatefeature "github.com/moh-sso-dashboard/internal/features/document_templates"
	documentfeature "github.com/moh-sso-dashboard/internal/features/documents"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	storagelocationfeature "github.com/moh-sso-dashboard/internal/features/storage_locations"
	surveillancefeature "github.com/moh-sso-dashboard/internal/features/surveillance"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/internal/keycloak"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/service"
	importsvc "github.com/moh-sso-dashboard/internal/service/import"
	"github.com/moh-sso-dashboard/internal/storage"
	"github.com/moh-sso-dashboard/internal/worker"
	reportschedulerfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
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
	Sessions                sessionfeature.Service
	Documents               *documentfeature.Service
	Clients                 *clientfeature.Service
	Users                   *userfeature.Service
	Diseases                *surveillancefeature.DiseaseService
	EpiWeeks                *surveillancefeature.EpiWeekService
	Locations               *surveillancefeature.LocationService
	Facilities              *surveillancefeature.FacilityService
	FacilityWeeklyMetrics   *surveillancefeature.FacilityWeeklyMetricsService
	WeeklyStatus            *surveillancefeature.WeeklyStatusService
	SurveillanceImport      *surveillancefeature.ImportService
	Alerts                  *surveillancefeature.AlertService
	DocumentTemplates       documenttemplatefeature.Service
	DocumentTemplateSheets  documenttemplatefeature.SheetService
	DocumentTemplateColumns documenttemplatefeature.ColumnService
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

	storageLocationService := storagelocationfeature.NewService(
		deps.Repositories.StorageLocations,
		notificationsService,
		deps.Config,
	)

	sessionService := sessionfeature.NewService(deps.Repositories.Sessions)
	documentService := documentfeature.NewService(
		deps.Repositories.Documents,
		deps.Repositories.Processes,
		notificationsService,
		deps.FileStorage,
		deps.Config,
	)

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

	diseaseService := surveillancefeature.NewDiseaseService(
		deps.Logger,
		deps.Repositories.Surveillance.Diseases,
		notificationsService,
		deps.Config,
	)

	epiWeekService := surveillancefeature.NewEpiWeekService(
		deps.Logger,
		deps.Repositories.Surveillance.EpiWeeks,
		notificationsService,
	)

	locationService := surveillancefeature.NewLocationService(
		deps.Logger,
		deps.Repositories.Surveillance.Regions,
		deps.Repositories.Surveillance.Districts,
		deps.Repositories.Surveillance.SubCounties,
		notificationsService,
		deps.Config,
	)

	facilityService := surveillancefeature.NewFacilityService(
		deps.Logger,
		deps.Repositories.Surveillance.Facilities,
		notificationsService,
	)

	facilityWeeklyMetricsService := surveillancefeature.NewFacilityWeeklyMetricsService(
		deps.Logger,
		deps.Repositories.Surveillance.FacilityMetrics,
		deps.Repositories.Surveillance.Imports,
	)

	weeklyStatusService := surveillancefeature.NewWeeklyStatusService(
		deps.Logger,
		deps.Repositories.Surveillance.WeeklyStatus,
	)

	surveillanceImportService := surveillancefeature.NewImportService(
		deps.Repositories.Surveillance.Imports,
	)

	alertsService := surveillancefeature.NewAlertService(
		deps.Logger,
		deps.Repositories.Surveillance.Alerts,
		deps.Repositories.Surveillance.Imports,
		notificationsService,
		deps.Config,
	)

	documentTemplateService := documenttemplatefeature.NewService(
		deps.Repositories.DocumentTemplates,
		deps.Repositories.DocumentTemplateSheets,
		deps.Repositories.DocumentTemplateColumns,
		notificationsService,
		deps.Config,
	)

	documentTemplateSheetService := documenttemplatefeature.NewSheetService(
		deps.Repositories.DocumentTemplateSheets,
		notificationsService,
		deps.Config,
	)

	documentTemplateColumnService := documenttemplatefeature.NewColumnService(
		deps.Repositories.DocumentTemplateColumns,
		notificationsService,
		deps.Config,
	)

	importService := importsvc.NewService(
		deps.Repositories.Documents,
		deps.Repositories.Processes,
		deps.Repositories.DocumentFiles,
		deps.Repositories.Surveillance.Imports,
		documentTemplateService,
		facilityWeeklyMetricsService,
		weeklyStatusService,
		alertsService,
		deps.FileStorage,
		deps.Databases.Remote,
	)

	rbacService := rbacfeature.NewService(deps.Repositories.RBAC, userService)
	rbacService.SetKeycloakGroupMembershipManager(deps.AdminKeycloak)
	rbacService.SetFrontendBaseURL(deps.Config.FrontendBaseURL)
	healthContextService := healthcontextfeature.NewService(deps.Repositories.HealthContexts)
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
		Sessions:                sessionService,
		Documents:               documentService,
		Clients:                 clientService,
		Users:                   userService,
		Diseases:                diseaseService,
		EpiWeeks:                epiWeekService,
		Locations:               locationService,
		Facilities:              facilityService,
		FacilityWeeklyMetrics:   facilityWeeklyMetricsService,
		WeeklyStatus:            weeklyStatusService,
		SurveillanceImport:      surveillanceImportService,
		Alerts:                  alertsService,
		DocumentTemplates:       documentTemplateService,
		DocumentTemplateSheets:  documentTemplateSheetService,
		DocumentTemplateColumns: documentTemplateColumnService,
		Import:                  importService,
		RBAC:                    rbacService,
		HealthContexts:          healthContextService,
		ReportScheduler:         reportSchedulerService,
	}
}
