package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// MigrationStream is one module's own schema history. Its versions are
// recorded in Table, separate from the platform's history and from every
// other module, so modules can be added, upgraded and disabled independently.
type MigrationStream struct {
	ModuleID string
	// Table records applied versions. It is derived from ModuleID.
	Table string
	// Files holds NNNNNN_name.up.sql and NNNNNN_name.down.sql at its root.
	Files fs.FS
}

// Migrator applies a stream's pending up migrations. The host implements it
// with its database driver; it must hold a lock scoped to stream.Table.
type Migrator interface {
	Migrate(context.Context, MigrationStream) error
}

// ErrMigratorRequired is returned by Runtime.Initialize when an enabled module
// ships migrations but the host has not supplied a Migrator.
var ErrMigratorRequired = errors.New("module migrations require a host migrator")

var migrationName = regexp.MustCompile(`^([0-9]{1,18})_([a-z0-9_]+)\.(up|down)\.sql$`)

// ModuleMigrationsTable returns the table that records a module's applied
// migrations, for example "module_report_scheduler_schema_migrations".
func ModuleMigrationsTable(moduleID string) string {
	return "module_" + moduleTableID(moduleID) + "_schema_migrations"
}

// ModuleTablePrefix is the prefix every table a module's migrations create or
// change must carry, for example "report_scheduler_".
func ModuleTablePrefix(moduleID string) string {
	return moduleTableID(moduleID) + "_"
}

func moduleTableID(moduleID string) string { return strings.ReplaceAll(moduleID, "-", "_") }

// Statements whose target table must belong to the module. This is a guard
// against the common mistakes (writing to platform or other modules' tables),
// not a SQL parser: reviewers still own the migration contents.
var ownedTableStatements = regexp.MustCompile(`(?i)\b(?:create\s+(?:unlogged\s+)?table(?:\s+if\s+not\s+exists)?|alter\s+table(?:\s+if\s+exists)?(?:\s+only)?|drop\s+table(?:\s+if\s+exists)?|truncate(?:\s+table)?(?:\s+only)?|insert\s+into|update(?:\s+only)?|delete\s+from|create\s+(?:unique\s+)?index(?:\s+concurrently)?(?:\s+if\s+not\s+exists)?(?:\s+[a-z0-9_"]+)?\s+on(?:\s+only)?)\s+("?[a-z0-9_.]+"?)`)

var sqlComments = regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/`)

// ValidateModuleMigrations checks a module's migration files: only correctly
// named .sql files at the root, unique versions, a down file for every up file
// and the reverse, non-empty up files, and every created, altered, dropped,
// truncated, indexed or written table prefixed with ModuleTablePrefix and not
// schema-qualified.
func ValidateModuleMigrations(moduleID string, files fs.FS) error {
	if !validID.MatchString(moduleID) {
		return fmt.Errorf("invalid module ID %q", moduleID)
	}
	if files == nil {
		return fmt.Errorf("module %q: migrations are nil", moduleID)
	}
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("module %q: read migrations: %w", moduleID, err)
	}
	type pair struct{ up, down string }
	versions := map[uint64]*pair{}
	prefix := ModuleTablePrefix(moduleID)
	var problems []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			problems = append(problems, name+": subdirectories are not allowed")
			continue
		}
		match := migrationName.FindStringSubmatch(name)
		if match == nil {
			problems = append(problems, name+": expected NNNNNN_name.up.sql or NNNNNN_name.down.sql")
			continue
		}
		version, _ := strconv.ParseUint(match[1], 10, 64)
		if version == 0 {
			problems = append(problems, name+": versions start at 1")
			continue
		}
		p := versions[version]
		if p == nil {
			p = &pair{}
			versions[version] = p
		}
		slot := &p.up
		if match[3] == "down" {
			slot = &p.down
		}
		if *slot != "" {
			problems = append(problems, fmt.Sprintf("%s: version %d is already used by %s", name, version, *slot))
			continue
		}
		*slot = name
		body, err := fs.ReadFile(files, name)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		statements := sqlComments.ReplaceAllString(string(body), " ")
		if match[3] == "up" && strings.TrimSpace(statements) == "" {
			problems = append(problems, name+": up migration is empty")
		}
		for _, target := range ownedTableStatements.FindAllStringSubmatch(statements, -1) {
			table := strings.ToLower(strings.Trim(target[1], `"`))
			if strings.Contains(table, ".") {
				problems = append(problems, fmt.Sprintf("%s: %q is schema-qualified; module tables live in the platform schema", name, table))
			} else if !strings.HasPrefix(table, prefix) {
				problems = append(problems, fmt.Sprintf("%s: table %q is not owned by module %q (expected prefix %q)", name, table, moduleID, prefix))
			}
		}
	}
	if len(versions) == 0 && len(problems) == 0 {
		problems = append(problems, "no migrations found")
	}
	ordered := make([]uint64, 0, len(versions))
	for version := range versions {
		ordered = append(ordered, version)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, version := range ordered {
		p := versions[version]
		if p.up == "" {
			problems = append(problems, fmt.Sprintf("version %d has a down migration but no up migration", version))
		}
		if p.down == "" {
			problems = append(problems, fmt.Sprintf("version %d has an up migration but no down migration", version))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("module %q migrations: %s", moduleID, strings.Join(problems, "; "))
	}
	return nil
}
