package rbac

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lib/pq"
)

// Exercises migration 61 (data cleanup, down/up round trip) and its triggers on an
// isolated, randomly named schema in an explicitly supplied test database.
func TestServiceOnlyRolesPostgres(t *testing.T) {
	db := registryTestDatabase(t)
	ctx := context.Background()
	run := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
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
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for _, name := range []string{
		"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql",
		"000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql",
		"000038_add_rbac_groups.up.sql",
	} {
		run(name)
	}

	// Pre-migration state: the scheduler service role was (wrongly) granted to people.
	exec(`INSERT INTO ihp_systems (client_id, display_name) VALUES ('data-statistics', 'Data'), ('clims', 'CLIMS')`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name)
	      SELECT id, r FROM ihp_systems, unnest(ARRAY['report_scheduler_service', 'report_viewer']) r WHERE client_id = 'data-statistics'`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, 'identity_sync' FROM ihp_systems WHERE client_id = 'clims'`)
	exec(`INSERT INTO ihp_permissions (permission_key) VALUES ('portal:access')`)
	exec(`INSERT INTO ihp_rbac_groups (path, name) VALUES ('/ops', 'ops')`)
	serviceRole := `(SELECT sr.id FROM ihp_system_roles sr JOIN ihp_systems s ON s.id = sr.system_id
	                 WHERE s.client_id = 'data-statistics' AND sr.role_name = 'report_scheduler_service')`
	viewerRole := `(SELECT sr.id FROM ihp_system_roles sr JOIN ihp_systems s ON s.id = sr.system_id
	                WHERE s.client_id = 'data-statistics' AND sr.role_name = 'report_viewer')`
	links := func(role string) int {
		return count(`SELECT
			(SELECT count(*) FROM ihp_system_role_permissions WHERE system_role_id = ` + role + `) +
			(SELECT count(*) FROM ihp_realm_role_system_roles WHERE system_role_id = ` + role + `) +
			(SELECT count(*) FROM ihp_rbac_group_system_roles WHERE system_role_id = ` + role + `) +
			(SELECT count(*) FROM ihp_system_access_roles ar JOIN ihp_system_roles sr ON sr.system_id = ar.system_id AND sr.role_name = ar.role_name
			  WHERE sr.id = ` + role + `)`)
	}
	exec(`INSERT INTO ihp_system_role_permissions (system_role_id, permission_id) SELECT ` + serviceRole + `, id FROM ihp_permissions`)
	exec(`INSERT INTO ihp_realm_role_system_roles (realm_role, system_role_id) VALUES ('user', ` + serviceRole + `)`)
	exec(`INSERT INTO ihp_rbac_group_system_roles (group_id, system_role_id) SELECT id, ` + serviceRole + ` FROM ihp_rbac_groups`)
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'report_scheduler_service' FROM ihp_systems WHERE client_id = 'data-statistics'`)
	exec(`INSERT INTO ihp_system_role_permissions (system_role_id, permission_id) SELECT ` + viewerRole + `, id FROM ihp_permissions`)
	if got := links(serviceRole); got != 4 {
		t.Fatalf("fixture links = %d, want 4", got)
	}

	run("000061_service_only_system_roles.up.sql")
	if got := count(`SELECT count(*) FROM ihp_system_roles WHERE service_only`); got != 2 {
		t.Fatalf("service-only roles = %d, want 2 (report_scheduler_service, clims/identity_sync)", got)
	}
	if got := links(serviceRole); got != 0 {
		t.Fatalf("person-facing links left on a service-only role: %d", got)
	}
	if got := links(viewerRole); got != 1 {
		t.Fatalf("ordinary role links changed: %d", got)
	}

	// Down restores exactly the removed links; up again removes them again.
	run("000061_service_only_system_roles.down.sql")
	if got := links(serviceRole); got != 4 {
		t.Fatalf("down migration restored %d links, want 4", got)
	}
	run("000061_service_only_system_roles.up.sql")
	if got := links(serviceRole); got != 0 {
		t.Fatalf("second up left %d links", got)
	}

	assertBlocked := func(name, query string) {
		t.Helper()
		_, err := db.ExecContext(ctx, query)
		var pqErr *pq.Error
		if !errors.As(err, &pqErr) || pqErr.Constraint != "ihp_service_only_system_role" {
			t.Errorf("%s: err = %v, want ihp_service_only_system_role violation", name, err)
		}
	}
	assertBlocked("permission", `INSERT INTO ihp_system_role_permissions (system_role_id, permission_id) SELECT `+serviceRole+`, id FROM ihp_permissions`)
	assertBlocked("realm role", `INSERT INTO ihp_realm_role_system_roles (realm_role, system_role_id) VALUES ('user', `+serviceRole+`)`)
	assertBlocked("group", `INSERT INTO ihp_rbac_group_system_roles (group_id, system_role_id) SELECT id, `+serviceRole+` FROM ihp_rbac_groups`)
	assertBlocked("access role", `INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'report_scheduler_service' FROM ihp_systems WHERE client_id = 'data-statistics'`)
	assertBlocked("mark linked role", `UPDATE ihp_system_roles SET service_only = TRUE WHERE id = `+viewerRole)
	// An access role named before its role row exists still blocks a service-only insert.
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'future_role' FROM ihp_systems WHERE client_id = 'clims'`)
	assertBlocked("insert service-only role already named as access role", `INSERT INTO ihp_system_roles (system_id, role_name, service_only)
		SELECT id, 'future_role', TRUE FROM ihp_systems WHERE client_id = 'clims'`)
	exec(`DELETE FROM ihp_system_access_roles WHERE role_name = 'future_role'`)

	// Ordinary writes still work, and a role with no grants can become service-only.
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'report_viewer' FROM ihp_systems WHERE client_id = 'data-statistics'`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, 'new_service' FROM ihp_systems WHERE client_id = 'clims'`)
	exec(`UPDATE ihp_system_roles SET service_only = TRUE WHERE role_name = 'new_service'`)

	// The repository surfaces the flag, and the handler maps the trigger error.
	repo := NewRepository(db)
	roles, err := repo.ListSystemRoles(ctx, "clims")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if !role.ServiceOnly {
			t.Errorf("clims role %q not reported as service-only", role.Name)
		}
	}
	err = repo.AddSystemAccessRole(ctx, "clims", "identity_sync")
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Constraint != "ihp_service_only_system_role" {
		t.Fatalf("repository access-role insert = %v, want trigger violation", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatal("trigger violation must not look like not-found")
	}
}
