package module

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

type countingMigrator struct{ streams []core.MigrationStream }

func (m *countingMigrator) Migrate(_ context.Context, stream core.MigrationStream) error {
	m.streams = append(m.streams, stream)
	return nil
}

func TestRegistryPassesModuleMigrationsToTheHostMigrator(t *testing.T) {
	files := fstest.MapFS{
		"000001_a.up.sql":   &fstest.MapFile{Data: []byte("CREATE TABLE reports_a (id int);")},
		"000001_a.down.sql": &fstest.MapFile{Data: []byte("DROP TABLE reports_a;")},
	}
	newRegistry := func() *Registry {
		registry, err := New(Definition{Info: Info{ID: "reports", Version: "v1", Enabled: true}, Migrations: files})
		if err != nil {
			t.Fatal(err)
		}
		return registry
	}
	if _, err := newRegistry().Initialize(context.Background()); !errors.Is(err, core.ErrMigratorRequired) {
		t.Fatalf("expected ErrMigratorRequired, got %v", err)
	}
	registry := newRegistry()
	migrator := &countingMigrator{}
	registry.UseMigrator(migrator)
	stop, err := registry.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop(context.Background())
	if len(migrator.streams) != 1 || migrator.streams[0].Table != "module_reports_schema_migrations" || len(registry.MigrationStreams()) != 1 {
		t.Fatalf("streams not passed through: %+v", migrator.streams)
	}
}
