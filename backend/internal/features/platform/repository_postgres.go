package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lib/pq"
)

type PostgresRepository struct{ DB *sql.DB }

func (r *PostgresRepository) ResolveIdentity(ctx context.Context, token Token) (Principal, error) {
	if token.Username != "service-account-"+token.ClientID {
		return Principal{}, ErrForbidden
	}
	var p Principal
	var scopes []string
	err := r.DB.QueryRowContext(ctx, `SELECT i.application_client_id, i.oauth_client_id, i.subject, i.scopes
 FROM ihp_application_identities i JOIN ihp_systems s ON s.client_id = i.application_client_id
 WHERE i.oauth_client_id = $1 AND i.subject = $2 AND i.enabled AND s.enabled AND s.lifecycle_status = 'active'`, token.ClientID, token.Subject).Scan(&p.ApplicationID, &p.ClientID, &p.Subject, pq.Array(&scopes))
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrForbidden
	}
	if err != nil {
		return Principal{}, ErrUnavailable
	}
	p.Scopes = intersectScopes(scopes, token.Scope)
	return p, nil
}
func (r *PostgresRepository) SaveIdentity(ctx context.Context, application, client, actor string, input IdentityInput) (Identity, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, err
	}
	defer tx.Rollback()
	// Lock the parent registration to serialize binding changes with lifecycle changes.
	var existing string
	if err := tx.QueryRowContext(ctx, `SELECT client_id FROM ihp_systems WHERE client_id = $1 FOR UPDATE`, application).Scan(&existing); err != nil {
		return Identity{}, err
	}
	result := Identity{ApplicationID: application, ClientID: client, Subject: input.Subject, Scopes: input.Scopes, Enabled: input.Enabled}
	err = tx.QueryRowContext(ctx, `INSERT INTO ihp_application_identities (application_client_id, oauth_client_id, subject, scopes, enabled)
 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (oauth_client_id) DO UPDATE SET
 subject = EXCLUDED.subject, scopes = EXCLUDED.scopes, enabled = EXCLUDED.enabled, updated_at = now()
 WHERE ihp_application_identities.application_client_id = EXCLUDED.application_client_id
 RETURNING oauth_client_id`, application, client, input.Subject, pq.Array(input.Scopes), input.Enabled).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrForbidden
	}
	if err != nil {
		return Identity{}, err
	}
	details, err := json.Marshal(map[string]any{"clientId": client, "subject": input.Subject, "scopes": input.Scopes, "enabled": input.Enabled, "reason": input.Reason})
	if err != nil {
		return Identity{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_rbac_audit_events (actor_user_id, action, resource_type, resource_id, system_client_id, details)
 VALUES ($1, 'application.identity.updated', 'application_identity', $2, $3, $4::jsonb)`, actor, client, application, string(details))
	if err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, err
	}
	return result, nil
}

func (r *PostgresRepository) UserAccess(ctx context.Context, application string, token Token) (UserAccess, error) {
	// A dedicated bound machine client is never accepted as an interactive user's client.
	if token.ClientID != application {
		return UserAccess{}, ErrForbidden
	}
	var allowed bool
	err := r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ihp_systems s
 WHERE s.client_id = $1 AND s.enabled AND s.lifecycle_status = 'active'
 AND NOT EXISTS(SELECT 1 FROM ihp_application_identities i WHERE i.oauth_client_id = $1 OR i.subject = $2))`, application, token.Subject).Scan(&allowed)
	if err != nil {
		return UserAccess{}, ErrUnavailable
	}
	if !allowed {
		return UserAccess{}, ErrForbidden
	}
	roles := token.ResourceAccess[application].Roles
	rows, err := r.DB.QueryContext(ctx, `SELECT DISTINCT sr.role_name FROM ihp_system_roles sr
 JOIN ihp_systems s ON s.id = sr.system_id JOIN ihp_system_access_roles ar ON ar.system_id = s.id AND ar.role_name = sr.role_name
 WHERE s.client_id = $1 AND sr.enabled AND (sr.role_name = ANY($2::text[]) OR EXISTS(
 SELECT 1 FROM ihp_realm_role_system_roles rr WHERE rr.system_role_id = sr.id AND rr.realm_role = ANY($3::text[])))
 ORDER BY sr.role_name`, application, pq.Array(roles), pq.Array(token.RealmAccess.Roles))
	if err != nil {
		return UserAccess{}, ErrUnavailable
	}
	result := UserAccess{ApplicationID: application, Subject: token.Subject, Roles: []string{}, Permissions: []string{}}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			rows.Close()
			return UserAccess{}, err
		}
		result.Roles = append(result.Roles, role)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return UserAccess{}, err
	}
	if len(result.Roles) == 0 {
		return UserAccess{}, ErrForbidden
	}
	rows, err = r.DB.QueryContext(ctx, `SELECT DISTINCT p.permission_key FROM ihp_permissions p
 JOIN ihp_system_role_permissions rp ON rp.permission_id = p.id JOIN ihp_system_roles sr ON sr.id = rp.system_role_id
 JOIN ihp_systems s ON s.id = sr.system_id WHERE s.client_id = $1 AND sr.enabled
 AND (sr.role_name = ANY($2::text[]) OR EXISTS(SELECT 1 FROM ihp_realm_role_system_roles rr
 WHERE rr.system_role_id = sr.id AND rr.realm_role = ANY($3::text[]))) ORDER BY p.permission_key`, application, pq.Array(roles), pq.Array(token.RealmAccess.Roles))
	if err != nil {
		return UserAccess{}, ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return UserAccess{}, err
		}
		result.Permissions = append(result.Permissions, permission)
	}
	return result, rows.Err()
}
