package identityreconcile

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// registryTestDatabase opens an isolated, randomly named schema in the test
// database named by MOH_REGISTRY_TEST_DATABASE_URL (copied from the rbac package).
func registryTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOH_REGISTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MOH_REGISTRY_TEST_DATABASE_URL (or run make migrations-check) for PostgreSQL integration tests")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("MOH_REGISTRY_TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := "identity_sync_test_" + uuid.NewString()[:8]
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+pq.QuoteIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// Exercises migration 62 (up, down, up), its triggers and the registry repository.
func TestIdentitySyncRegistryPostgres(t *testing.T) {
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
	for _, name := range []string{
		"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql",
		"000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql",
		"000038_add_rbac_groups.up.sql", "000048_application_registry_lifecycle.up.sql",
		"000049_application_identities.up.sql", "000061_service_only_system_roles.up.sql",
		"000062_identity_sync_integrations.up.sql",
	} {
		run(name)
	}
	run("000062_identity_sync_integrations.down.sql")
	run("000062_identity_sync_integrations.up.sql")

	exec(`INSERT INTO ihp_systems (client_id, display_name) VALUES ('alerts', 'Alerts'), ('clims', 'CLIMS')`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, r FROM ihp_systems, unnest(ARRAY['alerts_access', 'alerts_viewer']) r WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name, service_only) SELECT id, 'identity_sync', TRUE FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, 'clims_access' FROM ihp_systems WHERE client_id = 'clims'`)
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'clims_access' FROM ihp_systems WHERE client_id = 'clims'`)
	exec(`INSERT INTO ihp_application_identities (oauth_client_id, application_client_id, subject, enabled) VALUES
		('alerts-identity-sync', 'alerts', $1, TRUE),
		('alerts-disabled-sync', 'alerts', $2, FALSE),
		('clims-identity-sync', 'clims', $3, TRUE)`, uuid.NewString(), uuid.NewString(), uuid.NewString())

	insert := func(app, syncClient, role string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO ihp_identity_sync_integrations
			(application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format)
			VALUES ($1, 'https://app.example.test', $2, $3, 'env:APP_SYNC_SECRET', 'opaque')`, app, syncClient, role)
		return err
	}
	assertConstraint := func(name string, err error, constraint string) {
		t.Helper()
		var pqErr *pq.Error
		if !errors.As(err, &pqErr) || pqErr.Constraint != constraint {
			t.Errorf("%s: err = %v, want %s violation", name, err, constraint)
		}
	}
	assertConstraint("disabled sync client", insert("alerts", "alerts-disabled-sync", "alerts_access"), "ihp_identity_sync_sync_client")
	assertConstraint("another application's sync client", insert("alerts", "clims-identity-sync", "alerts_access"), "ihp_identity_sync_sync_client")
	assertConstraint("unknown sync client", insert("alerts", "missing-sync", "alerts_access"), "ihp_identity_sync_sync_client")
	assertConstraint("role that is not an access role", insert("alerts", "alerts-identity-sync", "alerts_viewer"), "ihp_identity_sync_access_role")
	assertConstraint("service-only role", insert("alerts", "alerts-identity-sync", "identity_sync"), "ihp_identity_sync_access_role")
	assertConstraint("another application's access role", insert("alerts", "alerts-identity-sync", "clims_access"), "ihp_identity_sync_access_role")
	if err := insert("alerts", "alerts-identity-sync", "alerts_access"); err != nil {
		t.Fatalf("valid integration rejected: %v", err)
	}
	// The trigger also guards updates.
	_, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET access_role = 'alerts_viewer' WHERE application_client_id = 'alerts'`)
	assertConstraint("update to non-access role", err, "ihp_identity_sync_access_role")
	_, err = db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET sync_client_id = 'alerts-disabled-sync' WHERE application_client_id = 'alerts'`)
	assertConstraint("update to disabled sync client", err, "ihp_identity_sync_sync_client")
	// Column checks.
	for name, query := range map[string]string{
		"origin with path":   `UPDATE ihp_identity_sync_integrations SET origin = 'https://app.example.test/x'`,
		"http origin":        `UPDATE ihp_identity_sync_integrations SET origin = 'http://app.example.test'`,
		"inline secret":      `UPDATE ihp_identity_sync_integrations SET credential_ref = 'secret-value'`,
		"unknown id format":  `UPDATE ihp_identity_sync_integrations SET account_id_format = 'email'`,
		"bad version":        `UPDATE ihp_identity_sync_integrations SET contract_version = '2.0'`,
		"relative file path": `UPDATE ihp_identity_sync_integrations SET credential_ref = 'file:relative'`,
	} {
		if _, err := db.ExecContext(ctx, query); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	exec(`DELETE FROM ihp_identity_sync_integrations`)

	// Repository round trip with audit events in the same transaction.
	repo := &PostgresRegistry{DB: db}
	integration := Integration{
		ApplicationClientID: "alerts", Origin: "https://alerts.example.test:8443", SyncClientID: "alerts-identity-sync",
		AccessRole: "alerts_access", CredentialRef: "file:/run/secrets/alerts-sync", AccountIDFormat: identitysync.AccountIDUUID,
		ContractVersion: "1.1", Enabled: true,
	}
	saved, err := repo.SaveIntegration(ctx, integration, "operator-1", "onboard alerts")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContractVersion != "1.1" || saved.UpdatedBy != "operator-1" || saved.CreatedAt.IsZero() || !saved.Enabled {
		t.Fatalf("saved = %+v", saved)
	}
	got, err := repo.GetIntegration(ctx, "alerts")
	if err != nil || got.Origin != integration.Origin || got.AccountIDFormat != identitysync.AccountIDUUID || got.CredentialRef != integration.CredentialRef {
		t.Fatalf("got = %+v, %v", got, err)
	}
	if err := repo.CheckIntegrationBinding(ctx, got); err != nil {
		t.Fatalf("binding: %v", err)
	}
	integration.ContractVersion = ""
	integration.Enabled = false
	if saved, err = repo.SaveIntegration(ctx, integration, "operator-2", "pause"); err != nil || saved.ContractVersion != "" || saved.Enabled {
		t.Fatalf("update = %+v, %v", saved, err)
	}
	bad := integration
	bad.AccessRole = "identity_sync"
	if _, err := repo.SaveIntegration(ctx, bad, "operator", "bad"); !errors.Is(err, ErrInvalidIntegration) {
		t.Fatalf("service-only role through repository: %v", err)
	}
	other := integration
	other.ApplicationClientID = "clims"
	other.AccessRole = "clims_access"
	if _, err := repo.SaveIntegration(ctx, other, "operator", "bad"); !errors.Is(err, ErrInvalidIntegration) {
		t.Fatalf("sync client of another application through repository: %v", err)
	}
	unknown := integration
	unknown.ApplicationClientID = "unregistered"
	if _, err := repo.SaveIntegration(ctx, unknown, "operator", "bad"); !errors.Is(err, ErrIntegrationNotFound) {
		t.Fatalf("unregistered application: %v", err)
	}

	// Disabling the service identity afterwards is caught at use time.
	exec(`UPDATE ihp_application_identities SET enabled = FALSE WHERE oauth_client_id = 'alerts-identity-sync'`)
	if err := repo.CheckIntegrationBinding(ctx, got); !errors.Is(err, ErrInvalidIntegration) {
		t.Fatalf("binding after identity disabled: %v", err)
	}
	exec(`UPDATE ihp_application_identities SET enabled = TRUE WHERE oauth_client_id = 'alerts-identity-sync'`)

	if err := repo.DeleteIntegration(ctx, "alerts", "operator-3", "retire"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteIntegration(ctx, "alerts", "operator-3", "again"); !errors.Is(err, ErrIntegrationNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := repo.GetIntegration(ctx, "alerts"); !errors.Is(err, ErrIntegrationNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
	var updates, deletes int
	if err := db.QueryRowContext(ctx, `SELECT
		count(*) FILTER (WHERE action = 'application.identity_sync.updated'),
		count(*) FILTER (WHERE action = 'application.identity_sync.deleted')
		FROM ihp_rbac_audit_events WHERE resource_type = 'identity_sync_integration' AND system_client_id = 'alerts'
		AND details->>'reason' <> ''`).Scan(&updates, &deletes); err != nil {
		t.Fatal(err)
	}
	if updates != 2 || deletes != 1 {
		t.Fatalf("audit events: %d updates, %d deletes; want 2 and 1 (failed saves must not be audited)", updates, deletes)
	}
}

// Exercises migration 66: keycloak:<sync client> references are stored only
// for the integration's own sync client, and rolling back refuses while one
// is still in use.
func TestIdentitySyncKeycloakCredentialMigrationPostgres(t *testing.T) {
	db := registryTestDatabase(t)
	ctx := context.Background()
	run := func(name string) error {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.ExecContext(ctx, string(data))
		return err
	}
	for _, name := range []string{
		"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql",
		"000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql",
		"000038_add_rbac_groups.up.sql", "000048_application_registry_lifecycle.up.sql",
		"000049_application_identities.up.sql", "000061_service_only_system_roles.up.sql",
		"000062_identity_sync_integrations.up.sql", "000065_identity_sync_integration_disable.up.sql",
		"000066_identity_sync_keycloak_credential.up.sql",
	} {
		if err := run(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, query := range []string{
		`INSERT INTO ihp_systems (client_id, display_name) VALUES ('clims', 'CLIMS')`,
		`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, 'clims_access' FROM ihp_systems WHERE client_id = 'clims'`,
		`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'clims_access' FROM ihp_systems WHERE client_id = 'clims'`,
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_application_identities (oauth_client_id, application_client_id, subject, enabled)
		VALUES ('clims-identity-sync', 'clims', $1, TRUE)`, uuid.NewString()); err != nil {
		t.Fatal(err)
	}

	repo := &PostgresRegistry{DB: db}
	integration := Integration{
		ApplicationClientID: "clims", Origin: "https://clims.example.test", SyncClientID: "clims-identity-sync",
		AccessRole: "clims_access", CredentialRef: "keycloak:clims-identity-sync", AccountIDFormat: identitysync.AccountIDUUID,
	}
	if _, err := repo.SaveIntegration(ctx, integration, "operator", "keycloak credential"); err != nil {
		t.Fatalf("keycloak:<own sync client> rejected: %v", err)
	}
	got, err := repo.GetIntegration(ctx, "clims")
	if err != nil || got.CredentialRef != "keycloak:clims-identity-sync" {
		t.Fatalf("got = %+v, %v", got, err)
	}
	for _, ref := range []string{"keycloak:clims", "keycloak:dashboard-admin", "keycloak:", "keycloak"} {
		if _, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET credential_ref = $1`, ref); err == nil {
			t.Errorf("%q accepted", ref)
		}
	}
	// env: and file: still work.
	for _, ref := range []string{"env:CLIMS_SYNC_CLIENT_SECRET", "file:/run/secrets/clims-sync"} {
		if _, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET credential_ref = $1`, ref); err != nil {
			t.Errorf("%q rejected: %v", ref, err)
		}
	}

	if _, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET credential_ref = 'keycloak:clims-identity-sync'`); err != nil {
		t.Fatal(err)
	}
	if err := run("000066_identity_sync_keycloak_credential.down.sql"); err == nil {
		t.Fatal("rollback succeeded while a keycloak: reference is in use")
	}
	if _, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET credential_ref = 'env:CLIMS_SYNC_CLIENT_SECRET'`); err != nil {
		t.Fatal(err)
	}
	if err := run("000066_identity_sync_keycloak_credential.down.sql"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE ihp_identity_sync_integrations SET credential_ref = 'keycloak:clims-identity-sync'`); err == nil {
		t.Fatal("keycloak: accepted after rollback")
	}
	if err := run("000066_identity_sync_keycloak_credential.up.sql"); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
}
