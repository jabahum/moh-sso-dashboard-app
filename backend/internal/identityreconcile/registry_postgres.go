package identityreconcile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/moh-sso-dashboard/internal/observability"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// ErrIntegrationInUse reports an integration that other records (for example
// sync runs) still reference.
var ErrIntegrationInUse = errors.New("identity sync integration is still referenced")

// PostgresRegistry stores integrations in ihp_identity_sync_integrations
// (migration 62) and audit events in ihp_rbac_audit_events.
type PostgresRegistry struct{ DB *sql.DB }

var _ RegistryRepository = (*PostgresRegistry)(nil)

const integrationColumns = `application_client_id, origin, sync_client_id, access_role, credential_ref,
 account_id_format, COALESCE(contract_version, ''), enabled, COALESCE(updated_by, ''), created_at, updated_at`

type rowScanner interface{ Scan(...any) error }

func scanIntegration(row rowScanner) (Integration, error) {
	var i Integration
	var format string
	err := row.Scan(&i.ApplicationClientID, &i.Origin, &i.SyncClientID, &i.AccessRole, &i.CredentialRef,
		&format, &i.ContractVersion, &i.Enabled, &i.UpdatedBy, &i.CreatedAt, &i.UpdatedAt)
	i.AccountIDFormat = identitysync.AccountIDFormat(format)
	return i, err
}

func (r *PostgresRegistry) GetIntegration(ctx context.Context, application string) (Integration, error) {
	integration, err := scanIntegration(r.DB.QueryRowContext(ctx,
		`SELECT `+integrationColumns+` FROM ihp_identity_sync_integrations WHERE application_client_id = $1`, application))
	if errors.Is(err, sql.ErrNoRows) {
		return Integration{}, ErrIntegrationNotFound
	}
	return integration, err
}

func (r *PostgresRegistry) SaveIntegration(ctx context.Context, integration Integration, actor, reason string) (Integration, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Integration{}, err
	}
	defer tx.Rollback()
	// Lock the application registration to serialize with lifecycle and identity changes.
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT client_id FROM ihp_systems WHERE client_id = $1 FOR UPDATE`, integration.ApplicationClientID).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return Integration{}, ErrIntegrationNotFound
	}
	if err != nil {
		return Integration{}, err
	}
	var version any
	if integration.ContractVersion != "" {
		version = integration.ContractVersion
	}
	saved, err := scanIntegration(tx.QueryRowContext(ctx, `INSERT INTO ihp_identity_sync_integrations
 (application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format, contract_version, enabled, updated_by)
 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
 ON CONFLICT (application_client_id) DO UPDATE SET origin = EXCLUDED.origin, sync_client_id = EXCLUDED.sync_client_id,
 access_role = EXCLUDED.access_role, credential_ref = EXCLUDED.credential_ref, account_id_format = EXCLUDED.account_id_format,
 contract_version = EXCLUDED.contract_version, enabled = EXCLUDED.enabled, updated_by = EXCLUDED.updated_by
 RETURNING `+integrationColumns,
		integration.ApplicationClientID, integration.Origin, integration.SyncClientID, integration.AccessRole,
		integration.CredentialRef, string(integration.AccountIDFormat), version, integration.Enabled, nullable(actor)))
	if err != nil {
		return Integration{}, mapRegistryError(err)
	}
	if err := writeRegistryAudit(ctx, tx, actor, "application.identity_sync.updated", saved.ApplicationClientID, map[string]any{
		"origin": saved.Origin, "syncClientId": saved.SyncClientID, "accessRole": saved.AccessRole,
		"credentialRef": saved.CredentialRef, "accountIdFormat": saved.AccountIDFormat,
		"contractVersion": saved.ContractVersion, "enabled": saved.Enabled, "reason": reason,
	}); err != nil {
		return Integration{}, err
	}
	if err := tx.Commit(); err != nil {
		return Integration{}, err
	}
	return saved, nil
}

func (r *PostgresRegistry) DeleteIntegration(ctx context.Context, application, actor, reason string) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	deleted, err := scanIntegration(tx.QueryRowContext(ctx,
		`DELETE FROM ihp_identity_sync_integrations WHERE application_client_id = $1 RETURNING `+integrationColumns, application))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrIntegrationNotFound
	}
	if err != nil {
		return mapRegistryError(err)
	}
	if err := writeRegistryAudit(ctx, tx, actor, "application.identity_sync.deleted", application, map[string]any{
		"origin": deleted.Origin, "syncClientId": deleted.SyncClientID, "accessRole": deleted.AccessRole,
		"credentialRef": deleted.CredentialRef, "enabled": deleted.Enabled, "reason": reason,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresRegistry) CheckIntegrationBinding(ctx context.Context, integration Integration) error {
	var identityOK, roleOK bool
	err := r.DB.QueryRowContext(ctx, `SELECT
 EXISTS (SELECT 1 FROM ihp_application_identities
   WHERE oauth_client_id = $2 AND application_client_id = $1 AND enabled),
 EXISTS (SELECT 1 FROM ihp_system_access_roles ar JOIN ihp_systems s ON s.id = ar.system_id
   WHERE s.client_id = $1 AND ar.role_name = $3)
 AND NOT EXISTS (SELECT 1 FROM ihp_system_roles sr JOIN ihp_systems s ON s.id = sr.system_id
   WHERE s.client_id = $1 AND sr.role_name = $3 AND sr.service_only)`,
		integration.ApplicationClientID, integration.SyncClientID, integration.AccessRole).Scan(&identityOK, &roleOK)
	if err != nil {
		return err
	}
	if !identityOK {
		return fmt.Errorf("%w: sync client is not an enabled service identity of the application", ErrInvalidIntegration)
	}
	if !roleOK {
		return fmt.Errorf("%w: access role is not a person-facing access role of the application", ErrInvalidIntegration)
	}
	return nil
}

func writeRegistryAudit(ctx context.Context, tx *sql.Tx, actor, action, application string, details map[string]any) error {
	details["requestId"] = observability.RequestIDFromContext(ctx)
	details["correlationId"] = observability.CorrelationIDFromContext(ctx)
	data, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_rbac_audit_events (actor_user_id, action, resource_type, resource_id, system_client_id, details)
 VALUES ($1, $2, 'identity_sync_integration', $3, $3, $4::jsonb)`, nullable(actor), action, application, string(data))
	return err
}

func mapRegistryError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	switch {
	case pqErr.Constraint == "ihp_identity_sync_sync_client":
		return fmt.Errorf("%w: sync client is not an enabled service identity of the application", ErrInvalidIntegration)
	case pqErr.Constraint == "ihp_identity_sync_access_role":
		return fmt.Errorf("%w: access role is not a person-facing access role of the application", ErrInvalidIntegration)
	case pqErr.Code == "23505": // unique_violation: the sync client serves another application
		return fmt.Errorf("%w: sync client is already registered for another application", ErrInvalidIntegration)
	case pqErr.Code == "23503" && pqErr.Table == "ihp_identity_sync_integrations":
		return fmt.Errorf("%w: unknown application or sync client", ErrInvalidIntegration)
	case pqErr.Code == "23503":
		return ErrIntegrationInUse
	case pqErr.Code == "23514": // check_violation from a column CHECK
		return fmt.Errorf("%w: %s", ErrInvalidIntegration, pqErr.Constraint)
	}
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
