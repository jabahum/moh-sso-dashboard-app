package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/rs/zerolog/log"
)

// ModuleMigrator applies compiled modules' own migration streams to the
// primary database. Each stream records its versions in its own table
// (core.ModuleMigrationsTable), so golang-migrate's advisory lock is scoped to
// that module: replicas starting together serialize per module, and a module's
// history never mixes with the platform's schema_migrations.
type ModuleMigrator struct {
	DB *sql.DB
	// LockTimeout bounds waiting for another replica migrating the same module.
	LockTimeout time.Duration
}

func (m ModuleMigrator) Migrate(ctx context.Context, stream core.MigrationStream) error {
	if m.DB == nil {
		return errors.New("module migrator has no database")
	}
	if stream.Table != core.ModuleMigrationsTable(stream.ModuleID) {
		return fmt.Errorf("module %q: unexpected migrations table %q", stream.ModuleID, stream.Table)
	}
	source, err := iofs.New(stream.Files, ".")
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}
	conn, err := m.DB.Conn(ctx)
	if err != nil {
		_ = source.Close()
		return err
	}
	driver, err := postgres.WithConnection(ctx, conn, &postgres.Config{MigrationsTable: stream.Table, StatementTimeout: 30 * time.Second})
	if err != nil {
		_ = source.Close()
		_ = conn.Close()
		return err
	}
	runner, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		_ = source.Close()
		_ = conn.Close()
		return err
	}
	defer runner.Close() // closes the source and the dedicated connection
	runner.LockTimeout = m.LockTimeout
	if runner.LockTimeout <= 0 {
		runner.LockTimeout = 30 * time.Second
	}
	if err := runner.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	version, dirty, err := runner.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return err
	}
	log.Info().Str("module", stream.ModuleID).Uint("version", version).Bool("dirty", dirty).Msg("Module migrations applied")
	return nil
}
