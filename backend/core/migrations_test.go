package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func migrationFiles(files map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

func validReportMigrations() fstest.MapFS {
	return migrationFiles(map[string]string{
		"000001_create_jobs.up.sql":   "CREATE TABLE IF NOT EXISTS report_scheduler_jobs (id uuid PRIMARY KEY);\nCREATE INDEX report_scheduler_jobs_idx ON report_scheduler_jobs (id);",
		"000001_create_jobs.down.sql": "DROP TABLE IF EXISTS report_scheduler_jobs;",
		"000002_add_owner.up.sql":     "-- owner references a platform table, which is allowed\nALTER TABLE report_scheduler_jobs ADD COLUMN owner uuid REFERENCES users(id);",
		"000002_add_owner.down.sql":   "ALTER TABLE report_scheduler_jobs DROP COLUMN owner;",
	})
}

func TestModuleMigrationNames(t *testing.T) {
	if got := ModuleMigrationsTable("report-scheduler"); got != "module_report_scheduler_schema_migrations" {
		t.Fatal(got)
	}
	if got := ModuleTablePrefix("report-scheduler"); got != "report_scheduler_" {
		t.Fatal(got)
	}
}

func TestValidateModuleMigrationsAcceptsOwnedSchema(t *testing.T) {
	if err := ValidateModuleMigrations("report-scheduler", validReportMigrations()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateModuleMigrationsRejectsBadFiles(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"expected NNNNNN_name": migrationFiles(map[string]string{"create.sql": "SELECT 1;"}),
		"no down migration":    migrationFiles(map[string]string{"000001_a.up.sql": "CREATE TABLE report_scheduler_a (id int);"}),
		"no up migration":      migrationFiles(map[string]string{"000001_a.down.sql": "DROP TABLE report_scheduler_a;"}),
		"already used":         migrationFiles(map[string]string{"000001_a.up.sql": "CREATE TABLE report_scheduler_a (id int);", "000001_b.up.sql": "CREATE TABLE report_scheduler_b (id int);", "000001_a.down.sql": "DROP TABLE report_scheduler_a;"}),
		"is empty":             migrationFiles(map[string]string{"000001_a.up.sql": "-- nothing\n", "000001_a.down.sql": "SELECT 1;"}),
		"start at 1":           migrationFiles(map[string]string{"000000_a.up.sql": "CREATE TABLE report_scheduler_a (id int);", "000000_a.down.sql": "DROP TABLE report_scheduler_a;"}),
		"subdirectories":       migrationFiles(map[string]string{"nested/000001_a.up.sql": "SELECT 1;"}),
		"no migrations found":  fstest.MapFS{},
	}
	for want, files := range cases {
		err := ValidateModuleMigrations("report-scheduler", files)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v", want, err)
		}
	}
}

func TestValidateModuleMigrationsRejectsForeignTables(t *testing.T) {
	for _, statement := range []string{
		"CREATE TABLE users_extra (id int);",
		`ALTER TABLE "announcements" ADD COLUMN x int;`,
		"DROP TABLE IF EXISTS audit_logs;",
		"INSERT INTO users (id) VALUES (1);",
		"UPDATE ihp_systems SET enabled = false;",
		"DELETE FROM notifications;",
		"TRUNCATE announcements;",
		"CREATE UNIQUE INDEX IF NOT EXISTS my_idx ON users (email);",
		"CREATE TABLE public.report_scheduler_jobs (id int);",
	} {
		files := migrationFiles(map[string]string{"000001_a.up.sql": statement, "000001_a.down.sql": "SELECT 1;"})
		if err := ValidateModuleMigrations("report-scheduler", files); err == nil {
			t.Errorf("accepted %q", statement)
		}
	}
	// Mentions inside comments are ignored.
	files := migrationFiles(map[string]string{
		"000001_a.up.sql":   "/* DROP TABLE users; */ CREATE TABLE report_scheduler_a (id int); -- ALTER TABLE users",
		"000001_a.down.sql": "DROP TABLE report_scheduler_a;",
	})
	if err := ValidateModuleMigrations("report-scheduler", files); err != nil {
		t.Fatalf("comments must not count: %v", err)
	}
}

type recordingMigrator struct {
	order   *[]string
	streams []MigrationStream
	err     error
}

func (m *recordingMigrator) Migrate(_ context.Context, stream MigrationStream) error {
	*m.order = append(*m.order, "migrate:"+stream.ModuleID)
	m.streams = append(m.streams, stream)
	return m.err
}

func initializer(order *[]string, id string) func(context.Context) (Stop, error) {
	return func(context.Context) (Stop, error) {
		*order = append(*order, "init:"+id)
		return nil, nil
	}
}

func TestRuntimeMigratesEnabledModulesBeforeInitializingAny(t *testing.T) {
	var order []string
	runtime, err := NewRuntime(
		ModuleDefinition{ModuleInfo: ModuleInfo{ID: "plain", Version: "v1", Enabled: true}, Initialize: initializer(&order, "plain")},
		ModuleDefinition{ModuleInfo: ModuleInfo{ID: "report-scheduler", Version: "v1", Enabled: true}, Initialize: initializer(&order, "report-scheduler"), Migrations: validReportMigrations()},
		ModuleDefinition{ModuleInfo: ModuleInfo{ID: "dormant", Version: "v1"}, Migrations: migrationFiles(map[string]string{"000001_a.up.sql": "CREATE TABLE dormant_a (id int);", "000001_a.down.sql": "DROP TABLE dormant_a;"})},
	)
	if err != nil {
		t.Fatal(err)
	}
	migrator := &recordingMigrator{order: &order}
	runtime.UseMigrator(migrator)
	stop, err := runtime.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop(context.Background())
	if got := strings.Join(order, ","); got != "migrate:report-scheduler,init:plain,init:report-scheduler" {
		t.Fatalf("unexpected order: %s", got)
	}
	if migrator.streams[0].Table != "module_report_scheduler_schema_migrations" {
		t.Fatalf("wrong table: %s", migrator.streams[0].Table)
	}
}

func TestRuntimeRefusesModuleMigrationsWithoutMigrator(t *testing.T) {
	var order []string
	runtime, err := NewRuntime(ModuleDefinition{ModuleInfo: ModuleInfo{ID: "report-scheduler", Version: "v1", Enabled: true},
		Initialize: initializer(&order, "report-scheduler"), Migrations: validReportMigrations()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Initialize(context.Background()); !errors.Is(err, ErrMigratorRequired) {
		t.Fatalf("expected ErrMigratorRequired, got %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("a module started without its schema: %v", order)
	}
}

func TestRuntimeStopsWhenAMigrationFails(t *testing.T) {
	var order []string
	runtime, err := NewRuntime(ModuleDefinition{ModuleInfo: ModuleInfo{ID: "report-scheduler", Version: "v1", Enabled: true},
		Initialize: initializer(&order, "report-scheduler"), Migrations: validReportMigrations()})
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("dirty database")
	runtime.UseMigrator(&recordingMigrator{order: &order, err: cause})
	if _, err := runtime.Initialize(context.Background()); !errors.Is(err, cause) {
		t.Fatalf("migration error lost: %v", err)
	}
	if strings.Contains(strings.Join(order, ","), "init:") {
		t.Fatalf("module initialized after a failed migration: %v", order)
	}
}

func TestNewRuntimeRejectsInvalidModuleMigrations(t *testing.T) {
	_, err := NewRuntime(ModuleDefinition{ModuleInfo: ModuleInfo{ID: "report-scheduler", Version: "v1"},
		Migrations: migrationFiles(map[string]string{"000001_a.up.sql": "DROP TABLE users;", "000001_a.down.sql": "SELECT 1;"})})
	if err == nil {
		t.Fatal("invalid migrations accepted at registry construction")
	}
}
