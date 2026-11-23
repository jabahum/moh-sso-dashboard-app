package bootstrap

import (
	"database/sql"

	"github.com/moh-sso-dashboard/internal/config"
	storepkg "github.com/moh-sso-dashboard/internal/db/sqlc"
	clientRepo "github.com/moh-sso-dashboard/internal/features/clients"
	documentTemplateRepo "github.com/moh-sso-dashboard/internal/features/document_templates"
	documentRepo "github.com/moh-sso-dashboard/internal/features/documents"
	healthcontextRepo "github.com/moh-sso-dashboard/internal/features/health_context"
	rbacRepo "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionRepo "github.com/moh-sso-dashboard/internal/features/sessions"
	surveillanceRepo "github.com/moh-sso-dashboard/internal/features/surveillance"
	userRepo "github.com/moh-sso-dashboard/internal/features/users"
	kcClientPkg "github.com/moh-sso-dashboard/internal/keycloak"
	logger "github.com/moh-sso-dashboard/internal/log"
	authRepo "github.com/moh-sso-dashboard/internal/repository/auth"
	emailRepo "github.com/moh-sso-dashboard/internal/repository/email"
	notificationDeliveryRepo "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
	notificationPreferencesRepo "github.com/moh-sso-dashboard/internal/repository/notification_preferences"
	notificationsRepo "github.com/moh-sso-dashboard/internal/repository/notifications"
	processRepo "github.com/moh-sso-dashboard/internal/repository/processes"
)

type repositories struct {
	Auth                    authRepo.AuthRepository
	Clients                 clientRepo.ClientRepository
	Users                   userRepo.UserRepository
	Notifications           notificationsRepo.NotificationsRepository
	NotificationDelivery    notificationDeliveryRepo.NotificationDeliveryRepository
	NotificationPreferences notificationPreferencesRepo.NotificationPreferencesRepository
	Documents               documentRepo.DocumentRepository
	DocumentFiles           documentRepo.FileRepository
	DocumentTemplates       documentTemplateRepo.DocumentTemplateRepository
	DocumentTemplateColumns documentTemplateRepo.DocumentTemplateColumnRepository
	DocumentTemplateSheets  documentTemplateRepo.DocumentTemplateSheetRepository
	Processes               processRepo.ProcessRepository
	Sessions                sessionRepo.SessionRepository
	Email                   emailRepo.EmailRepository
	Surveillance            *surveillanceRepo.Repositories
	RBAC                    rbacRepo.Repository
	HealthContexts          healthcontextRepo.Repository
}

func buildRepositories(
	cfg *config.Config,
	store storepkg.Store,
	surveillanceStore storepkg.Store,
	adminKC *kcClientPkg.KeyAdminClient,
	webKC *kcClientPkg.Client,
	appLogger *logger.Logger,
) repositories {
	var primaryDB *sql.DB
	if sqlStore, ok := store.(*storepkg.SQLStore); ok && sqlStore != nil {
		primaryDB = sqlStore.DB()
	}

	if cfg.CoreOnly() {
		return repositories{
			Auth:           authRepo.NewAuthRepository(webKC, adminKC, cfg),
			Users:          userRepo.NewUserRepository(adminKC, cfg, store, *appLogger),
			Notifications:  notificationsRepo.NewNotificationsRepository(store, *appLogger),
			Sessions:       sessionRepo.NewSessionRepository(adminKC, cfg, *appLogger),
			RBAC:           rbacRepo.NewRepository(primaryDB),
			HealthContexts: healthcontextRepo.NewRepository(primaryDB),
		}
	}
	healthContexts := healthcontextRepo.NewRepository(primaryDB)
	return repositories{
		Auth:                    authRepo.NewAuthRepository(webKC, adminKC, cfg),
		Clients:                 clientRepo.NewClientRepository(adminKC, cfg, store, *appLogger),
		Users:                   userRepo.NewUserRepository(adminKC, cfg, store, *appLogger),
		Notifications:           notificationsRepo.NewNotificationsRepository(store, *appLogger),
		NotificationDelivery:    notificationDeliveryRepo.NewNotificationDeliveryRepository(store, *appLogger),
		NotificationPreferences: notificationPreferencesRepo.NewNotificationPreferencesRepository(store),
		Documents:               documentRepo.NewDocumentRepository(cfg, store, *appLogger),
		DocumentFiles:           documentRepo.NewFileRepository(),
		DocumentTemplates:       documentTemplateRepo.NewDocumentTemplateRepository(store),
		DocumentTemplateColumns: documentTemplateRepo.NewDocumentTemplateColumnRepository(store),
		DocumentTemplateSheets:  documentTemplateRepo.NewDocumentTemplateSheetRepository(store),
		Processes:               processRepo.NewProcessRepository(cfg, store, *appLogger),
		Sessions:                sessionRepo.NewSessionRepository(adminKC, cfg, *appLogger),
		Email:                   emailRepo.NewEmailRepository(cfg, store, *appLogger),
		Surveillance:            surveillanceRepo.NewRepositories(surveillanceStore),
		RBAC:                    rbacRepo.NewRepository(primaryDB),
		HealthContexts:          healthContexts,
	}
}
