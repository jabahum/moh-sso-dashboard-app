package notification_delivery

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	logger "github.com/moh-sso-dashboard/internal/log"
)

// TestNotificationDeliveryPostgres exercises the delivery queue queries against
// real PostgreSQL in a throwaway schema: claim-time attempt counting, retries
// that leave the counter alone, reclaiming expired PROCESSING leases, failing
// leases that expired on the final attempt, and in-app deliveries that are
// complete on creation (plus migration 66 settling ones left PENDING).
func TestNotificationDeliveryPostgres(t *testing.T) {
	dsn := os.Getenv("MOH_REGISTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("run make migrations-check for disposable PostgreSQL")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("test database must be a PostgreSQL URL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "delivery_test_" + uuid.NewString()[:8]
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
	conn, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	migrate := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, name := range []string{"000002_create_notifications_schema.up.sql", "000024_create_schema_notification_deliveries.up.sql", "000035_add_notification_delivery_provider_metadata.up.sql"} {
		migrate(name)
	}

	var notificationID uuid.UUID
	if err := conn.QueryRowContext(ctx, `INSERT INTO notifications (type,title,message,target_role) VALUES ('TEST','t','m','admin') RETURNING id`).Scan(&notificationID); err != nil {
		t.Fatal(err)
	}
	// Rows written before migration 66: an in-app delivery stuck PENDING and one
	// an operator cancelled (which must stay cancelled).
	var stuckInApp, cancelledInApp uuid.UUID
	if err := conn.QueryRowContext(ctx, `INSERT INTO notification_deliveries (notification_id,channel,max_attempts) VALUES ($1,'in_app',1) RETURNING id`, notificationID).Scan(&stuckInApp); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `INSERT INTO notification_deliveries (notification_id,channel,status,max_attempts) VALUES ($1,'in_app','CANCELLED',1) RETURNING id`, notificationID).Scan(&cancelledInApp); err != nil {
		t.Fatal(err)
	}
	migrate("000066_settle_in_app_notification_deliveries.up.sql")

	store := db.NewStore(conn)
	repo := NewNotificationDeliveryRepository(store, *logger.NewLogger())
	get := func(id uuid.UUID) db.NotificationDelivery {
		t.Helper()
		row, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	if row := get(stuckInApp); row.Status != "SENT" || !row.SentAt.Valid || row.Attempts != 1 {
		t.Fatalf("migration 66 left stuck in-app delivery %+v", row)
	}
	if row := get(cancelledInApp); row.Status != "CANCELLED" || row.SentAt.Valid {
		t.Fatalf("migration 66 touched a cancelled in-app delivery %+v", row)
	}

	create := func(channel, status string, maxAttempts int32) db.NotificationDelivery {
		t.Helper()
		row, err := repo.Create(ctx, db.CreateNotificationDeliveryParams{
			NotificationID:      notificationID,
			Channel:             channel,
			DeliveryStatus:      sql.NullString{String: status, Valid: status != ""},
			DeliveryMaxAttempts: sql.NullInt32{Int32: maxAttempts, Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	inApp := create("in_app", "SENT", 1)
	if inApp.Status != "SENT" || !inApp.SentAt.Valid || inApp.Attempts != 1 {
		t.Fatalf("in-app delivery created %+v, want SENT with sent_at", inApp)
	}
	queued := create("email", "PENDING", 2)
	if queued.Status != "PENDING" || queued.SentAt.Valid || queued.Attempts != 0 {
		t.Fatalf("email delivery created %+v, want PENDING without attempts", queued)
	}
	if defaulted := create("sms", "", 2); defaulted.Status != "PENDING" || defaulted.Attempts != 0 {
		t.Fatalf("delivery without explicit status created %+v", defaulted)
	}

	claim := func(want ...uuid.UUID) []db.NotificationDelivery {
		t.Helper()
		rows, err := repo.ClaimPending(ctx, "email", 10)
		if err != nil {
			t.Fatal(err)
		}
		got := map[uuid.UUID]bool{}
		for _, row := range rows {
			got[row.ID] = true
		}
		if len(rows) != len(want) {
			t.Fatalf("claimed %d deliveries, want %d", len(rows), len(want))
		}
		for _, id := range want {
			if !got[id] {
				t.Fatalf("delivery %s was not claimed", id)
			}
		}
		return rows
	}
	expireLease := func(id uuid.UUID) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, `UPDATE notification_deliveries SET locked_at = now() - interval '1 hour' WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}

	// Claiming starts attempt 1.
	if rows := claim(queued.ID); rows[0].Status != "PROCESSING" || rows[0].Attempts != 1 || !rows[0].LockedAt.Valid {
		t.Fatalf("claimed delivery %+v", rows[0])
	}
	// A live lease is not reclaimed.
	claim()

	// A worker retry leaves the counter alone; the next claim starts attempt 2.
	if err := repo.MarkRetry(ctx, db.MarkNotificationDeliveryRetryParams{ID: queued.ID, LastError: sql.NullString{String: "boom", Valid: true}, Column3: "0 seconds"}); err != nil {
		t.Fatal(err)
	}
	if row := get(queued.ID); row.Status != "RETRY" || row.Attempts != 1 || row.LockedAt.Valid {
		t.Fatalf("retried delivery %+v, want RETRY with attempts 1", row)
	}
	if rows := claim(queued.ID); rows[0].Attempts != 2 {
		t.Fatalf("second claim attempts = %d, want 2", rows[0].Attempts)
	}

	// The worker crashed on the final attempt: the expired lease is not reclaimed
	// past max_attempts but failed.
	expireLease(queued.ID)
	claim()
	if row := get(queued.ID); row.Status != "FAILED" || row.Attempts != 2 || row.LockedAt.Valid || !row.LastError.Valid {
		t.Fatalf("exhausted stale delivery %+v, want FAILED with attempts 2", row)
	}

	// A manual retry is not an attempt: it buys exactly one more claim.
	if err := repo.MarkRetry(ctx, db.MarkNotificationDeliveryRetryParams{ID: queued.ID, LastError: sql.NullString{String: "Manually queued for retry", Valid: true}, Column3: "0 seconds"}); err != nil {
		t.Fatal(err)
	}
	if row := get(queued.ID); row.Status != "RETRY" || row.Attempts != 2 {
		t.Fatalf("manually retried delivery %+v, want RETRY with attempts 2", row)
	}
	if rows := claim(queued.ID); rows[0].Attempts != 3 {
		t.Fatalf("claim after manual retry attempts = %d, want 3", rows[0].Attempts)
	}
	if err := repo.MarkFailed(ctx, db.MarkNotificationDeliveryFailedParams{ID: queued.ID, LastError: sql.NullString{String: "boom", Valid: true}}); err != nil {
		t.Fatal(err)
	}
	if row := get(queued.ID); row.Status != "FAILED" || row.Attempts != 3 {
		t.Fatalf("failed delivery %+v, want attempts unchanged at 3", row)
	}

	// A worker crashed with attempts left: the expired lease is reclaimed as the
	// next attempt.
	crashed := create("email", "PENDING", 5)
	claim(crashed.ID)
	expireLease(crashed.ID)
	if rows := claim(crashed.ID); rows[0].Status != "PROCESSING" || rows[0].Attempts != 2 {
		t.Fatalf("reclaimed delivery %+v, want PROCESSING attempt 2", rows[0])
	}
	if err := repo.MarkSent(ctx, crashed.ID); err != nil {
		t.Fatal(err)
	}
	if row := get(crashed.ID); row.Status != "SENT" || row.Attempts != 2 || row.LockedAt.Valid {
		t.Fatalf("sent delivery %+v", row)
	}

	// Other channels are untouched by an email claim, and in-app rows are never
	// claimable.
	var sms int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM notification_deliveries WHERE channel='sms' AND status='PENDING' AND attempts=0`).Scan(&sms); err != nil || sms != 1 {
		t.Fatalf("sms deliveries changed: %d, %v", sms, err)
	}
	if rows, err := repo.ClaimPending(ctx, "in_app", 10); err != nil || len(rows) != 0 {
		t.Fatalf("in-app claim = %d rows, %v", len(rows), err)
	}
}
