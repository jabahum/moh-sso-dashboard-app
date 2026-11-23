package bootstrap

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/moh-sso-dashboard/internal/config"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

// newIdentitySyncRevokeService builds the identity-sync access-removal service
// (Track C) on the registry, the revocation table and the Keycloak admin client.
// It returns nil when a dependency is missing.
func newIdentitySyncRevokeService(cfg *config.Config, db *sql.DB, admin *keycloak.KeyAdminClient) *identityreconcile.RevokeService {
	if cfg == nil || db == nil || admin == nil || cfg.KeycloakRealm == "" {
		return nil
	}
	realm := url.PathEscape(cfg.KeycloakRealm)
	endpoint := strings.TrimRight(cfg.KeycloakTokenBaseURL(), "/") + "/realms/" + realm + "/protocol/openid-connect/token"
	registry := identityreconcile.NewRegistryService(&identityreconcile.PostgresRegistry{DB: db}, endpoint)
	external := strings.TrimRight(cfg.KeycloakExternalURL, "/")
	if external == "" {
		external = strings.TrimRight(cfg.KeycloakBaseURL, "/")
	}
	return identityreconcile.NewRevokeService(registry, &identityreconcile.PostgresRevocations{DB: db},
		identityreconcile.KeycloakAccessDirectory{Admin: admin}, external+"/realms/"+cfg.KeycloakRealm)
}

// The portal hook also restores an active revocation when an admin grants the
// access role again (users.AccessRoleGrantHook).
var _ userfeature.AccessRoleGrantHook = (*identityreconcile.PortalRevocationHook)(nil)

// newIdentitySyncRevocationHook returns the users-service hook that ends access to
// a registered application (sessions included) when its access role is removed
// through portal RBAC or the users API, or nil when revocation is unavailable.
func newIdentitySyncRevocationHook(cfg *config.Config, db *sql.DB, admin *keycloak.KeyAdminClient) userfeature.AccessRoleRemovalHook {
	service := newIdentitySyncRevokeService(cfg, db, admin)
	if service == nil {
		return nil
	}
	return identityreconcile.NewPortalRevocationHook(service)
}
