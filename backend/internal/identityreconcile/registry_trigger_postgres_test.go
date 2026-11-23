package identityreconcile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Disabling an integration must stay possible after its sync client was
// disabled or its access role removed (migration 65); enabling it, or changing
// its binding, must still be checked.
func TestIdentitySyncIntegrationDisableAfterBindingBreaksPostgres(t *testing.T) {
	db := registryTestDatabase(t)
	ctx := context.Background()
	run := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	migrations := []string{
		"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql",
		"000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql",
		"000038_add_rbac_groups.up.sql", "000048_application_registry_lifecycle.up.sql",
		"000049_application_identities.up.sql", "000061_service_only_system_roles.up.sql",
		"000062_identity_sync_integrations.up.sql", "000065_identity_sync_integration_disable.up.sql",
	}
	for _, name := range migrations {
		run(name)
	}
	exec(`INSERT INTO ihp_systems (client_id, display_name) VALUES ('alerts', 'Alerts')`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, r FROM ihp_systems, unnest(ARRAY['alerts_access', 'alerts_viewer']) r WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_application_identities (oauth_client_id, application_client_id, subject, enabled) VALUES
		('alerts-identity-sync', 'alerts', $1, TRUE), ('alerts-other-sync', 'alerts', $2, FALSE)`, uuid.NewString(), uuid.NewString())
	exec(`INSERT INTO ihp_identity_sync_integrations (application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format, enabled)
		VALUES ('alerts', 'https://alerts.example.test', 'alerts-identity-sync', 'alerts_access', 'env:ALERTS_SYNC_SECRET', 'opaque', TRUE)`)

	// The service identity is disabled (for example after a credential leak):
	// the operator must still be able to disable the integration.
	exec(`UPDATE ihp_application_identities SET enabled = FALSE WHERE oauth_client_id = 'alerts-identity-sync'`)
	if _, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET enabled = FALSE, updated_by = 'ops' WHERE application_client_id = 'alerts'`); err != nil {
		t.Fatalf("disabling an integration whose sync client is disabled was refused: %v", err)
	}
	// Re-enabling with the broken binding is still refused.
	_, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET enabled = TRUE WHERE application_client_id = 'alerts'`)
	assertPQConstraint(t, "re-enable with disabled sync client", err, "ihp_identity_sync_sync_client")
	// Changing the binding while disabled is still checked.
	_, err = db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET sync_client_id = 'alerts-other-sync' WHERE application_client_id = 'alerts'`)
	assertPQConstraint(t, "disabled row moved to a disabled sync client", err, "ihp_identity_sync_sync_client")
	exec(`UPDATE ihp_application_identities SET enabled = TRUE WHERE oauth_client_id = 'alerts-identity-sync'`)

	// The access role stops being an access role: disabling still works.
	exec(`UPDATE ihp_identity_sync_integrations SET enabled = TRUE WHERE application_client_id = 'alerts'`)
	exec(`DELETE FROM ihp_system_access_roles WHERE role_name = 'alerts_access'`)
	if _, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET enabled = FALSE WHERE application_client_id = 'alerts'`); err != nil {
		t.Fatalf("disabling an integration whose access role was removed was refused: %v", err)
	}
	_, err = db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET access_role = 'alerts_viewer' WHERE application_client_id = 'alerts'`)
	assertPQConstraint(t, "disabled row moved to a non-access role", err, "ihp_identity_sync_access_role")
	// A disabled insert with a broken binding is still refused.
	exec(`DELETE FROM ihp_identity_sync_integrations`)
	_, err = db.ExecContext(ctx, `INSERT INTO ihp_identity_sync_integrations (application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format, enabled)
		VALUES ('alerts', 'https://alerts.example.test', 'alerts-identity-sync', 'alerts_access', 'env:ALERTS_SYNC_SECRET', 'opaque', FALSE)`)
	assertPQConstraint(t, "disabled insert with non-access role", err, "ihp_identity_sync_access_role")

	// Down restores migration 62's function: disabling a broken binding is refused again.
	run("000065_identity_sync_integration_disable.down.sql")
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_identity_sync_integrations (application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format, enabled)
		VALUES ('alerts', 'https://alerts.example.test', 'alerts-identity-sync', 'alerts_access', 'env:ALERTS_SYNC_SECRET', 'opaque', TRUE)`)
	exec(`UPDATE ihp_application_identities SET enabled = FALSE WHERE oauth_client_id = 'alerts-identity-sync'`)
	_, err = db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET enabled = FALSE WHERE application_client_id = 'alerts'`)
	assertPQConstraint(t, "after down migration", err, "ihp_identity_sync_sync_client")
	run("000065_identity_sync_integration_disable.up.sql")
	exec(`UPDATE ihp_identity_sync_integrations SET enabled = FALSE WHERE application_client_id = 'alerts'`)
}

func assertPQConstraint(t *testing.T, name string, err error, constraint string) {
	t.Helper()
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Constraint != constraint {
		t.Errorf("%s: err = %v, want %s violation", name, err, constraint)
	}
}
