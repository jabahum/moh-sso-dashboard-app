package bootstrap

import (
	"context"
	"strings"

	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/api/handler"
	authfeature "github.com/moh-sso-dashboard/internal/features/auth"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	platformfeature "github.com/moh-sso-dashboard/internal/features/platform"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	db "github.com/moh-sso-dashboard/internal/migrate"
	"github.com/moh-sso-dashboard/internal/service"
)

// Core shares durable notification event recording with auth/audit but supplies
// no delivery queue, publisher, channel provider or administration endpoints.
func buildCoreServices(deps serviceDependencies) services {
	notifications := service.NewNotificationsService(deps.Config, deps.Repositories.Notifications, nil, nil, nil)
	audit := service.NewAuditService(deps.Store, deps.Cache, notifications, deps.Config)
	users := userfeature.NewService(deps.Repositories.Users, notifications, deps.Config)
	users.SetAccessRoleRemovalHook(newIdentitySyncRevocationHook(deps.Config, deps.Databases.Primary, deps.AdminKeycloak))
	rbac := rbacfeature.NewService(deps.Repositories.RBAC, users)
	rbac.SetKeycloakGroupMembershipManager(deps.AdminKeycloak)
	rbac.SetFrontendBaseURL(deps.Config.FrontendBaseURL)
	return services{
		Notifications: notifications, Audit: audit,
		Auth:  service.NewAuthService(deps.Repositories.Auth, deps.CacheClient),
		Users: users, RBAC: rbac,
		Sessions:       sessionfeature.NewService(deps.Repositories.Sessions),
		HealthContexts: healthcontextfeature.NewService(deps.Repositories.HealthContexts),
	}
}
func buildCoreHandlers(deps handlerDependencies) handlers {
	auth := authfeature.NewHandler(deps.Services.Auth, deps.Services.Audit, deps.Services.Notifications, deps.AuthSessions, deps.Config, deps.AuthzResolver)
	auth.SetHealthContextResolver(deps.Services.HealthContexts)
	health := handler.NewHealthHandler(
		func(ctx context.Context) error { return db.PingDB(ctx, deps.Databases.Primary) },
		nil, func(context.Context) error { return deps.AdminKeycloak.Authenticate() }, deps.Redis,
	)
	delivery := false
	return handlers{
		Health: health,
		Router: router.HandlerSet{
			Auth:           auth,
			Sessions:       sessionfeature.NewHandler(deps.Services.Sessions),
			RBAC:           rbacfeature.NewHandler(deps.Services.RBAC),
			HealthContexts: healthcontextfeature.NewHandler(deps.Services.HealthContexts, deps.Services.Audit),
			Platform: &platformfeature.Handler{
				Repository:   &platformfeature.PostgresRepository{DB: deps.Databases.Primary},
				IdentitySync: platformfeature.NewIdentitySyncRegistry(deps.Databases.Primary, deps.Config.KeycloakTokenBaseURL(), deps.Config.KeycloakRealm),
				CoreReady:    health.CheckCore, RequiredWorkers: []string{}, DeliveryEnabled: &delivery,
				Verifier: &platformfeature.Introspector{
					Endpoint: strings.TrimRight(deps.Config.KeycloakTokenBaseURL(), "/") + "/realms/" + deps.Config.KeycloakRealm + "/protocol/openid-connect/token/introspect",
					Issuer:   strings.TrimRight(deps.Config.KeycloakExternalURL, "/") + "/realms/" + deps.Config.KeycloakRealm,
					Audience: deps.Config.PlatformAuthAudience, ClientID: deps.Config.PlatformAuthClientID, Secret: deps.Config.PlatformAuthClientSecret,
				},
			},
		},
	}
}
