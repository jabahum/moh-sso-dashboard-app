//go:build integration

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	_ "github.com/lib/pq"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

func moduleTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MIGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("run make migrations-check to provision disposable PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	name := fmt.Sprintf("module_migrations_ci_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	database, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.Close()
		if _, err := admin.Exec("DROP DATABASE " + name + " WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	return database
}

func stream(moduleID string, files map[string]string) core.MigrationStream {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return core.MigrationStream{ModuleID: moduleID, Table: core.ModuleMigrationsTable(moduleID), Files: fsys}
}

func appliedVersion(t *testing.T, database *sql.DB, table string) (int64, bool) {
	t.Helper()
	var version int64
	var dirty bool
	if err := database.QueryRow("SELECT version, dirty FROM "+table).Scan(&version, &dirty); err != nil {
		t.Fatalf("read %s: %v", table, err)
	}
	return version, dirty
}

func TestModuleMigrationStreamsArePerModulePostgres(t *testing.T) {
	database := moduleTestDatabase(t)
	ctx := context.Background()
	migrator := ModuleMigrator{DB: database, LockTimeout: 10 * time.Second}

	reports := map[string]string{
		"000001_jobs.up.sql":   "CREATE TABLE reports_jobs (id int PRIMARY KEY);",
		"000001_jobs.down.sql": "DROP TABLE reports_jobs;",
	}
	alerts := map[string]string{
		"000001_rules.up.sql":   "CREATE TABLE alert_hub_rules (id int PRIMARY KEY);",
		"000001_rules.down.sql": "DROP TABLE alert_hub_rules;",
		"000002_seed.up.sql":    "INSERT INTO alert_hub_rules VALUES (1);",
		"000002_seed.down.sql":  "DELETE FROM alert_hub_rules;",
	}
	for _, s := range []core.MigrationStream{stream("reports", reports), stream("alert-hub", alerts)} {
		if err := core.ValidateModuleMigrations(s.ModuleID, s.Files); err != nil {
			t.Fatal(err)
		}
		if err := migrator.Migrate(ctx, s); err != nil {
			t.Fatalf("%s: %v", s.ModuleID, err)
		}
	}
	if v, dirty := appliedVersion(t, database, "module_reports_schema_migrations"); v != 1 || dirty {
		t.Fatalf("reports at %d dirty=%v", v, dirty)
	}
	if v, dirty := appliedVersion(t, database, "module_alert_hub_schema_migrations"); v != 2 || dirty {
		t.Fatalf("alert-hub at %d dirty=%v", v, dirty)
	}
	var platformHistory sql.NullString
	if err := database.QueryRow("SELECT to_regclass('public.schema_migrations')::text").Scan(&platformHistory); err != nil || platformHistory.Valid {
		t.Fatalf("module streams must not touch the platform history table: %v %v", platformHistory, err)
	}

	// Re-running is a no-op, and a new version applies only that version.
	if err := migrator.Migrate(ctx, stream("reports", reports)); err != nil {
		t.Fatal(err)
	}
	reports["000002_owner.up.sql"] = "ALTER TABLE reports_jobs ADD COLUMN owner text;"
	reports["000002_owner.down.sql"] = "ALTER TABLE reports_jobs DROP COLUMN owner;"
	if err := migrator.Migrate(ctx, stream("reports", reports)); err != nil {
		t.Fatal(err)
	}
	if v, _ := appliedVersion(t, database, "module_reports_schema_migrations"); v != 2 {
		t.Fatalf("reports at %d after upgrade", v)
	}
	var rules int
	if err := database.QueryRow("SELECT count(*) FROM alert_hub_rules").Scan(&rules); err != nil || rules != 1 {
		t.Fatalf("the other module's stream changed: %d %v", rules, err)
	}
}

func TestConcurrentReplicasApplyAModuleStreamOncePostgres(t *testing.T) {
	database := moduleTestDatabase(t)
	s := stream("reports", map[string]string{
		// Not idempotent: a second application would fail.
		"000001_jobs.up.sql":   "CREATE TABLE reports_jobs (id int PRIMARY KEY); SELECT pg_sleep(0.3);",
		"000001_jobs.down.sql": "DROP TABLE reports_jobs;",
	})
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ModuleMigrator{DB: database, LockTimeout: 20 * time.Second}.Migrate(context.Background(), s)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("replica failed: %v", err)
		}
	}
	if v, dirty := appliedVersion(t, database, "module_reports_schema_migrations"); v != 1 || dirty {
		t.Fatalf("reports at %d dirty=%v", v, dirty)
	}
}

func TestFailedModuleMigrationStopsStartupPostgres(t *testing.T) {
	database := moduleTestDatabase(t)
	s := stream("reports", map[string]string{
		"000001_bad.up.sql":   "CREATE TABLE reports_jobs (id int); SELECT * FROM reports_missing_table;",
		"000001_bad.down.sql": "DROP TABLE IF EXISTS reports_jobs;",
	})
	migrator := ModuleMigrator{DB: database}
	if err := migrator.Migrate(context.Background(), s); err == nil {
		t.Fatal("a failing migration was reported as applied")
	}
	if _, dirty := appliedVersion(t, database, "module_reports_schema_migrations"); !dirty {
		t.Fatal("a failed migration must leave the stream dirty for an operator to resolve")
	}
	if err := migrator.Migrate(context.Background(), s); err == nil {
		t.Fatal("a dirty stream must keep refusing to start the module")
	}
}
