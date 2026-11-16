package rbac

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/moh-sso-dashboard/internal/authz"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
)

// Uses only an isolated, randomly named schema in an explicitly supplied test database.
func TestApplicationRegistryPostgres(t *testing.T) {
	db := registryTestDatabase(t)
	ctx := context.Background()
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
	for _, status := range []string{"registered", "configured", "validated", "approved", "suspended", "retired"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO ihp_systems (client_id, display_name, lifecycle_status) VALUES ($1, $1, $2)`, "existing-"+status, status); err != nil {
			t.Fatal(err)
		}
	}
	migration("000059_grandfather_existing_client_lifecycle.up.sql")
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
	for _, status := range []string{"registered", "configured", "validated", "approved", "suspended", "retired"} {
		detail, err := repo.GetSystem(ctx, "existing-"+status)
		if err != nil {
			t.Fatal(err)
		}
		if status == "suspended" || status == "retired" {
			if detail.LifecycleStatus != status || detail.Enabled {
				t.Fatalf("reactivated stopped integration: %+v", detail)
			}
		} else if detail.LifecycleStatus != "active" || !detail.Enabled {
			t.Fatalf("existing client still requires onboarding: %+v", detail)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE ihp_systems SET owner_team='Updated legacy owner' WHERE client_id='existing-configured'`); err != nil {
		t.Fatal(err)
	}
	legacy, err := repo.GetSystem(ctx, "existing-configured")
	if err != nil || legacy.LifecycleStatus != "active" || !legacy.Enabled {
		t.Fatalf("legacy edit re-entered onboarding: %+v %v", legacy, err)
	}
	service := NewService(repo)
	truth, falsity := true, false
	input := UpsertSystemInput{ClientID: "pilot", DisplayName: "Pilot", OwnerTeam: "Team", OwnerName: "Owner", OwnerEmail: "owner@example.org", SupportURL: "https://example.org/support", Environment: "test", LaunchURL: "https://example.org", SystemType: "external", LaunchMode: "new_tab", Enabled: &truth, DisplayInLauncher: &truth, DisplayInSideNav: &falsity, ActorUserID: "registrar", RegisterOnly: true}
	adminCreate := input
	adminCreate.ClientID = "admin-update-cannot-bypass"
	adminCreate.RegisterOnly = false
	createdByUpdate, err := service.UpsertSystem(ctx, adminCreate)
	if err != nil || createdByUpdate.LifecycleStatus != "registered" || createdByUpdate.Enabled {
		t.Fatalf("admin upsert bypassed onboarding: %+v %v", createdByUpdate, err)
	}
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
	// Automatic startup discovery must not overwrite active configuration, including
	// a concurrent insert that reaches the repository's ON CONFLICT guard.
	startup := systemrbac.WithStartupDiscovery(ctx)
	changedStartup := input
	changedStartup.OwnerTeam = "Discovered owner"
	changedStartup.LaunchURL = "https://startup.example.org"
	changedStartup.IntegrationType = "external"
	changedStartup.ContractVersion = "1"
	preserved, err := service.UpsertSystem(startup, changedStartup)
	if err != nil || preserved.OwnerTeam != input.OwnerTeam || preserved.LaunchURL != input.LaunchURL || preserved.LifecycleStatus != "active" || !preserved.Enabled {
		t.Fatalf("service startup overwrote configuration: %+v %v", preserved, err)
	}
	preserved, err = repo.UpsertSystem(startup, changedStartup)
	if err != nil || preserved.OwnerTeam != input.OwnerTeam || preserved.LifecycleStatus != "active" || !preserved.Enabled {
		t.Fatalf("repository startup conflict overwrote configuration: %+v %v", preserved, err)
	}
	startupSeed := systemrbac.SeedFile{Systems: []systemrbac.SeedSystem{{ClientID: "pilot", DisplayName: "Discovered title", OwnerTeam: "Seed owner", LaunchURL: "https://seed.example.org", SystemType: "external", LaunchMode: "new_tab", Enabled: &truth, DisplayInLauncher: &truth, DisplayInSideNav: &falsity}}}
	for i := 0; i < 2; i++ {
		if err := systemrbac.ApplySeed(startup, db, startupSeed); err != nil {
			t.Fatal(err)
		}
		preserved, err := repo.GetSystem(ctx, "pilot")
		if err != nil || preserved.OwnerTeam != input.OwnerTeam || preserved.LaunchURL != input.LaunchURL || preserved.LifecycleStatus != "active" || !preserved.Enabled {
			t.Fatalf("seed startup overwrote configuration: %+v %v", preserved, err)
		}
	}
	newInput := changedStartup
	newInput.ClientID = "startup-discovered"
	newInput.RegisterOnly = false
	created, err := service.UpsertSystem(startup, newInput)
	if err != nil || created.LifecycleStatus != "active" || !created.Enabled {
		t.Fatalf("existing discovered client should be operational: %+v %v", created, err)
	}
	profileInput := newInput
	profileInput.ClientID = "onboarding-profile-discovered"
	profileInput.LifecycleManaged = true
	profileInput.ActorUserID = ""
	profile, err := service.UpsertSystem(startup, profileInput)
	if err != nil || profile.LifecycleStatus != "registered" || profile.Enabled {
		t.Fatalf("onboarding profile bypassed review: %+v %v", profile, err)
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
	if _, err := db.ExecContext(ctx, `UPDATE ihp_systems SET lifecycle_managed=false WHERE client_id='pilot'`); err != nil {
		t.Fatal(err)
	}
	var managed bool
	if err := db.QueryRowContext(ctx, `SELECT lifecycle_managed FROM ihp_systems WHERE client_id='pilot'`).Scan(&managed); err != nil || !managed {
		t.Fatalf("managed review was cleared: %v %v", managed, err)
	}
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
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_systems (client_id, display_name, lifecycle_status, lifecycle_managed) VALUES ('rollback', 'Rollback', 'registered', true)`); err != nil {
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

	// Cosmetic navigation migrations preserve access and unrelated configuration.
	t.Run("issue tracker navigation alignment", func(t *testing.T) {
		const navigation = `[{"id":"other","path":"/other","displayInLauncher":true},{"id":"issue-tracking","label":"Issue Tracking","path":"/apps/dwh/issue-tracker","permission":"issue_tracker:read","children":[{"path":"/apps/dwh/issue-tracker/dashboard","displayInLauncher":true},{"path":"/apps/dwh/issue-tracker/issues","displayInLauncher":true}]}]`
		for _, fixture := range []struct {
			client, status string
			enabled        bool
		}{
			{"navigation-active", "active", true}, {"navigation-suspended", "suspended", false},
		} {
			if _, err := db.ExecContext(ctx, `INSERT INTO ihp_systems (client_id, display_name, lifecycle_status, enabled, metadata) VALUES ($1, $1, $2, $3, jsonb_build_object('navigation', $4::text, 'custom', 'preserved'))`, fixture.client, fixture.status, fixture.enabled, navigation); err != nil {
				t.Fatal(err)
			}
		}
		migration("000052_align_issue_tracker_navigation.up.sql")
		var valid bool
		if err := db.QueryRowContext(ctx, `SELECT bool_and(
			(metadata->>'navigation')::jsonb->1->>'label' = 'Issue Tracker'
			AND (metadata->>'navigation')::jsonb->1->>'permission' = 'issue_tracker:read'
			AND (metadata->>'navigation')::jsonb->1->'children'->0->>'displayInLauncher' = 'false'
			AND (metadata->>'navigation')::jsonb->1->'children'->1->>'displayInLauncher' = 'false'
			AND (metadata->>'navigation')::jsonb->0->>'displayInLauncher' = 'true'
			AND metadata->>'custom' = 'preserved'
			AND ((client_id = 'navigation-active' AND lifecycle_status = 'active' AND enabled)
			OR (client_id = 'navigation-suspended' AND lifecycle_status = 'suspended' AND NOT enabled)))
			FROM ihp_systems WHERE client_id LIKE 'navigation-%'`).Scan(&valid); err != nil || !valid {
			t.Fatalf("navigation/access alignment: valid=%v err=%v", valid, err)
		}
		migration("000052_align_issue_tracker_navigation.down.sql")
		if err := db.QueryRowContext(ctx, `SELECT bool_and((metadata->>'navigation')::jsonb = $1::jsonb AND NOT metadata ? 'issueTrackerNavigationBeforeAlignment'
			AND ((client_id = 'navigation-active' AND lifecycle_status = 'active' AND enabled)
			OR (client_id = 'navigation-suspended' AND lifecycle_status = 'suspended' AND NOT enabled)))
			FROM ihp_systems WHERE client_id LIKE 'navigation-%'`, navigation).Scan(&valid); err != nil || !valid {
			t.Fatalf("navigation rollback: valid=%v err=%v", valid, err)
		}
	})
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
