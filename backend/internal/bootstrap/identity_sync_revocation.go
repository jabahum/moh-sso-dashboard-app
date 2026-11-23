package bootstrap

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/moh-sso-dashboard/internal/config"
	platformfeature "github.com/moh-sso-dashboard/internal/features/platform"
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
	return identityreconcile.NewRevokeService(identitySyncRegistry(cfg, db), &identityreconcile.PostgresRevocations{DB: db},
		identityreconcile.KeycloakAccessDirectory{Admin: admin}, identitySyncIssuer(cfg))
}

// identitySyncRegistry is the Postgres registry; sync clients mint tokens at
// the realm's token endpoint.
func identitySyncRegistry(cfg *config.Config, db *sql.DB) *identityreconcile.RegistryService {
	realm := url.PathEscape(cfg.KeycloakRealm)
	endpoint := strings.TrimRight(cfg.KeycloakTokenBaseURL(), "/") + "/realms/" + realm + "/protocol/openid-connect/token"
	return identityreconcile.NewRegistryService(&identityreconcile.PostgresRegistry{DB: db}, endpoint)
}

// identitySyncIssuer is the public issuer written into application links.
func identitySyncIssuer(cfg *config.Config) string {
	external := strings.TrimRight(cfg.KeycloakExternalURL, "/")
	if external == "" {
		external = strings.TrimRight(cfg.KeycloakBaseURL, "/")
	}
	return external + "/realms/" + cfg.KeycloakRealm
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

// newIdentitySyncOperations builds the portal's identity-sync run and
// revocation operations, or nil when a dependency is missing. Runs use the
// same revocation list (and its per-subject access lock) as the portal hook.
func newIdentitySyncOperations(cfg *config.Config, db *sql.DB, admin *keycloak.KeyAdminClient) *identityreconcile.Operations {
	revoke := newIdentitySyncRevokeService(cfg, db, admin)
	if revoke == nil {
		return nil
	}
	return &identityreconcile.Operations{
		Batch:    identityreconcile.BatchService{Repo: &identityreconcile.PostgresBatchRepository{DB: db}, Revocations: revoke},
		Registry: identitySyncRegistry(cfg, db),
		Revoke:   revoke,
		Admin:    admin,
		Issuer:   identitySyncIssuer(cfg),
	}
}

// identitySyncOperationsHandler returns the operations as the platform handler's
// interface, keeping it nil (503) when they cannot be built.
func identitySyncOperationsHandler(cfg *config.Config, db *sql.DB, admin *keycloak.KeyAdminClient) platformfeature.IdentitySyncOperations {
	if ops := newIdentitySyncOperations(cfg, db, admin); ops != nil {
		return ops
	}
	return nil
}
