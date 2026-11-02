package rbac

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
	"github.com/moh-sso-dashboard/internal/authz"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
)

// Uses only an isolated, randomly named schema in an explicitly supplied test database.
func TestApplicationRegistryPostgres(t *testing.T) {
	dsn := os.Getenv("MOH_REGISTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MOH_REGISTRY_TEST_DATABASE_URL to run PostgreSQL migration/lifecycle integration tests")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("MOH_REGISTRY_TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "registry_test_" + uuid.NewString()[:8]
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+pq.QuoteIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, name := range []string{"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql", "000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql"} {
		migration(name)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_systems (client_id, display_name, enabled) VALUES ('legacy-enabled', 'Legacy', true), ('legacy-disabled', 'Disabled', false)`); err != nil {
		t.Fatal(err)
	}
	migration("000048_application_registry_lifecycle.up.sql")
	repo := NewRepository(db)
	for clientID, enabled := range map[string]bool{"legacy-enabled": true, "legacy-disabled": false} {
		detail, err := repo.GetSystem(ctx, clientID)
		if err != nil {
			t.Fatal(err)
		}
		if detail.Enabled != enabled || detail.LifecycleStatus != "active" || detail.IntegrationType != "hosted_module" {
			t.Fatalf("migration changed existing application: %+v", detail)
		}
	}
	service := NewService(repo)
	truth, falsity := true, false
	input := UpsertSystemInput{ClientID: "pilot", DisplayName: "Pilot", OwnerTeam: "Team", OwnerName: "Owner", OwnerEmail: "owner@example.org", SupportURL: "https://example.org/support", Environment: "test", LaunchURL: "https://example.org", SystemType: "external", LaunchMode: "new_tab", Enabled: &truth, DisplayInLauncher: &truth, DisplayInSideNav: &falsity, ActorUserID: "registrar", RegisterOnly: true}
	system, err := service.UpsertSystem(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if system.LifecycleStatus != "registered" || system.Enabled {
		t.Fatalf("registration granted access: %+v", system)
	}
	if _, err := service.UpsertSystem(ctx, input); !errors.Is(err, ErrApplicationExists) {
		t.Fatalf("duplicate registration: %v", err)
	}
	input.RegisterOnly = false
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_system_roles (system_id, role_name) VALUES ($1, 'viewer');`, system.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_system_access_roles (system_id, role_name) VALUES ($1, 'viewer')`, system.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_permissions (permission_key) VALUES ('pilot:read'); INSERT INTO ihp_system_role_permissions SELECT sr.id, p.id FROM ihp_system_roles sr CROSS JOIN ihp_permissions p WHERE sr.role_name = 'viewer' AND p.permission_key = 'pilot:read'`); err != nil {
		t.Fatal(err)
	}
	transition := func(status string) error {
		_, err := service.TransitionSystem(ctx, "pilot", LifecycleTransitionInput{Status: status, Reason: "Integration review"}, lifecycleActor())
		return err
	}
	for _, status := range []string{"configured", "validated", "approved", "active"} {
		if err := transition(status); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
	}
	resolver := authz.NewDBResolver(db)
	assertAccess := func(want bool) {
		t.Helper()
		access, err := resolver.Resolve(ctx, nil, map[string][]string{"pilot": {"viewer"}})
		if err != nil {
			t.Fatal(err)
		}
		if (len(access.Systems) > 0) != want || (len(access.Permissions) > 0) != want {
			t.Fatalf("unexpected access: %+v", access)
		}
	}
	assertAccess(true)
	// Unchanged upserts must preserve active status despite BEFORE INSERT / ON CONFLICT interaction.
	system, err = service.UpsertSystem(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if !system.Enabled || system.LifecycleStatus != "active" {
		t.Fatalf("upsert disabled active application: %+v", system)
	}
	if err := transition("suspended"); err != nil {
		t.Fatal(err)
	}
	assertAccess(false)
	if _, err := service.UpsertSystem(ctx, input); err != nil {
		t.Fatal(err)
	}
	assertAccess(false)
	seed := systemrbac.SeedFile{Systems: []systemrbac.SeedSystem{{ClientID: "pilot", DisplayName: "Pilot", OwnerTeam: input.OwnerTeam, OwnerName: input.OwnerName, OwnerEmail: input.OwnerEmail, SupportURL: input.SupportURL, Environment: input.Environment, LaunchURL: input.LaunchURL, SystemType: "external", LaunchMode: "new_tab", Enabled: &truth, DisplayInLauncher: &truth, DisplayInSideNav: &falsity}}}
	if err := systemrbac.ApplySeed(ctx, db, seed); err != nil {
		t.Fatal(err)
	}
	assertAccess(false)
	if err := transition("active"); !errors.Is(err, ErrLifecycleConflict) {
		t.Fatalf("suspension bypassed approval: %v", err)
	}
	for _, status := range []string{"configured", "validated", "approved", "active"} {
		if err := transition(status); err != nil {
			t.Fatal(err)
		}
	}
	// Sync of identical config must preserve activation too.
	if err := systemrbac.ApplySeed(ctx, db, seed); err != nil {
		t.Fatal(err)
	}
	assertAccess(true)
	input.OwnerTeam = "New owner"
	system, err = service.UpsertSystem(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if system.LifecycleStatus != "configured" || system.Enabled {
		t.Fatalf("configuration edit retained approval: %+v", system)
	}
	assertAccess(false)
	for _, status := range []string{"validated", "approved", "active", "retired"} {
		if err := transition(status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.UpsertSystem(ctx, input); err != nil {
		t.Fatal(err)
	}
	assertAccess(false)
	var events, denials, invalidations int
	err = db.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE action = 'system.lifecycle.denied'), count(*) FILTER (WHERE action = 'system.lifecycle.invalidated') FROM ihp_rbac_audit_events WHERE system_client_id = 'pilot'`).Scan(&events, &denials, &invalidations)
	if err != nil {
		t.Fatal(err)
	}
	if events < 10 || denials == 0 || invalidations == 0 {
		t.Fatalf("missing audit history: %d/%d/%d", events, denials, invalidations)
	}
	// An audit write failure must roll back activation.
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_systems (client_id, display_name) VALUES ('rollback', 'Rollback')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE ihp_rbac_audit_events ADD CONSTRAINT reject_rollback_audit CHECK (system_client_id <> 'rollback')`); err != nil {
		t.Fatal(err)
	}
	_, err = service.TransitionSystem(ctx, "rollback", LifecycleTransitionInput{Status: "configured", Reason: "Audit outage"}, lifecycleActor())
	if err == nil {
		t.Fatal("expected audit failure")
	}
	unchanged, err := repo.GetSystem(ctx, "rollback")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.LifecycleStatus != "registered" {
		t.Fatal("audit failure changed application")
	}
	migration("000048_application_registry_lifecycle.down.sql")
	var enabled bool
	if err := db.QueryRowContext(ctx, `SELECT enabled FROM ihp_systems WHERE client_id = 'pilot'`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("rollback revived retired application")
	}
	migration("000048_application_registry_lifecycle.up.sql")
}
