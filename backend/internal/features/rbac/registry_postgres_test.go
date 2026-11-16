package rbac

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func registryTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
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
	t.Cleanup(func() { admin.Close() })
	schema := "registry_test_" + uuid.NewString()[:8]
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
