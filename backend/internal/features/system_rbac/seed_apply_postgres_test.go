package system_rbac

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// seedTestDatabase returns a database in a fresh schema with every up
// migration applied. Set MOH_REGISTRY_TEST_DATABASE_URL to run it.
func seedTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOH_REGISTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MOH_REGISTRY_TEST_DATABASE_URL to run PostgreSQL seed integration tests")
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
	ctx := context.Background()
	schema := "seed_test_" + uuid.NewString()[:8]
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+pq.QuoteIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	files, err := filepath.Glob(filepath.Join("..", "..", "db", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(raw)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(file), err)
		}
	}
	return db
}

type seedTestRole struct {
	DisplayName string
	ServiceOnly bool
}

func systemRoles(t *testing.T, db *sql.DB, clientID string) map[string]seedTestRole {
	t.Helper()
	rows, err := db.Query(`SELECT r.role_name, COALESCE(r.display_name, ''), r.service_only
		FROM ihp_system_roles r JOIN ihp_systems s ON s.id = r.system_id WHERE s.client_id = $1`, clientID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	roles := map[string]seedTestRole{}
	for rows.Next() {
		var name string
		var role seedTestRole
		if err := rows.Scan(&name, &role.DisplayName, &role.ServiceOnly); err != nil {
			t.Fatal(err)
		}
		roles[name] = role
	}
	return roles
}

// A manifest-imported system is left to its manifest at startup, but still
// gets the seed's service-only roles, which a manifest cannot declare and the
// seed's service clients need (clims-identity-sync -> clims/identity_sync).
func TestStartupSeedAddsServiceOnlyRolesToManifestManagedSystems(t *testing.T) {
	db := seedTestDatabase(t)
	ctx := context.Background()
	seed, err := LoadSeedFile(filepath.Join("..", "..", "..", "config", "system-rbac.seed.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	// clims was imported from its manifest with only its access role.
	var climsID string
	if err := db.QueryRow(`INSERT INTO ihp_systems (client_id, display_name, metadata)
		VALUES ('clims', 'CLIMS from manifest', '{"applicationManifest": {"id": "clims"}}'::jsonb) RETURNING id::text`).Scan(&climsID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ihp_system_roles (system_id, role_name, display_name) VALUES ($1::uuid, 'clims_access', 'From manifest')`, climsID); err != nil {
		t.Fatal(err)
	}

	for run := 1; run <= 2; run++ {
		if err := ApplySeed(WithStartupDiscovery(ctx), db, seed); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		roles := systemRoles(t, db, "clims")
		if got := roles["identity_sync"]; !got.ServiceOnly || got.DisplayName != "CLIMS Identity Sync" {
			t.Fatalf("run %d: clims identity_sync = %+v, want the seed's service-only role", run, got)
		}
		// The manifest still owns everything else.
		if got := roles["clims_access"]; got.DisplayName != "From manifest" || got.ServiceOnly {
			t.Fatalf("run %d: clims_access = %+v, want the manifest's role unchanged", run, got)
		}
		if len(roles) != 2 {
			t.Fatalf("run %d: clims roles = %v, want only clims_access and identity_sync", run, roles)
		}
		var name string
		if err := db.QueryRow(`SELECT display_name FROM ihp_systems WHERE client_id = 'clims'`).Scan(&name); err != nil || name != "CLIMS from manifest" {
			t.Fatalf("run %d: clims display name = %q, %v", run, name, err)
		}
	}

	// A system the seed owns still gets its full role set, as before.
	if got := systemRoles(t, db, "poesgo")["identity_sync"]; !got.ServiceOnly {
		t.Fatalf("poesgo identity_sync = %+v", got)
	}
}

func TestStartupSeedLeavesAManifestRoleOfTheSameNameAlone(t *testing.T) {
	db := seedTestDatabase(t)
	seed, err := LoadSeedFile(filepath.Join("..", "..", "..", "config", "system-rbac.seed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var climsID string
	if err := db.QueryRow(`INSERT INTO ihp_systems (client_id, display_name, metadata)
		VALUES ('clims', 'CLIMS', '{"applicationManifest": {"id": "clims"}}'::jsonb) RETURNING id::text`).Scan(&climsID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ihp_system_roles (system_id, role_name, display_name) VALUES ($1::uuid, 'identity_sync', 'Manifest role')`, climsID); err != nil {
		t.Fatal(err)
	}
	if err := ApplySeed(WithStartupDiscovery(context.Background()), db, seed); err != nil {
		t.Fatal(err)
	}
	if got := systemRoles(t, db, "clims")["identity_sync"]; got.ServiceOnly || !strings.EqualFold(got.DisplayName, "Manifest role") {
		t.Fatalf("clims identity_sync = %+v, want the manifest's role unchanged", got)
	}
}
