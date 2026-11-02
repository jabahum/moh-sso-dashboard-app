package platform

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestPlatformPostgres(t *testing.T) {
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
	schema := "platform_test_" + uuid.NewString()[:8]
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
	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"000002_create_notifications_schema.up.sql", "000020_create_email_outbox_schema.up.sql", "000024_create_schema_notification_deliveries.up.sql", "000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql", "000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql", "000048_application_registry_lifecycle.up.sql", "000049_application_identities.up.sql", "000050_application_notifications.up.sql"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	repo := &PostgresRepository{DB: db}
	for _, app := range []string{"pilot", "other", "blocked"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO ihp_systems (client_id,display_name,lifecycle_status,enabled) VALUES ($1,$1,'active',true)`, app); err != nil {
			t.Fatal(err)
		}
	}
	token := validToken()
	token.Subject = uuid.NewString()
	token.Scope = "identity:read notifications:write notifications:read"
	input := IdentityInput{Subject: token.Subject, Scopes: []string{IdentityRead, NotificationsWrite, NotificationsRead}, Enabled: true, Reason: "Integration test"}
	if _, err := repo.SaveIdentity(ctx, "pilot", token.ClientID, "operator", input); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveIdentity(ctx, "other", token.ClientID, "operator", input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("binding reassigned: %v", err)
	}
	p, err := repo.ResolveIdentity(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	wrong := token
	wrong.Subject = uuid.NewString()
	if _, err := repo.ResolveIdentity(ctx, wrong); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong subject: %v", err)
	}
	if _, err := repo.UserAccess(ctx, "pilot", token); !errors.Is(err, ErrForbidden) {
		t.Fatalf("machine used as interactive user: %v", err)
	}
	// Concurrent retries create one notification, delivery, application request and audit.
	var wg sync.WaitGroup
	ids := make(chan string, 10)
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := repo.SubmitNotification(ctx, p, "same-key", validNotification())
			if err != nil {
				errs <- err
				return
			}
			ids <- result.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var id string
	for value := range ids {
		if id != "" && id != value {
			t.Fatal("duplicate queue records")
		}
		id = value
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM notifications WHERE target_role='application:pilot'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("queue count=%d err=%v", count, err)
	}
	changed := validNotification()
	changed.Message = "different"
	if _, err := repo.SubmitNotification(ctx, p, "same-key", changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	if _, err := repo.GetNotification(ctx, "other", id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-app read: %v", err)
	}
	// Track the existing email outbox's actual status rather than the worker handoff.
	var deliveryID, notificationID string
	if err := db.QueryRowContext(ctx, `SELECT delivery_id::text,notification_id::text FROM ihp_application_notifications WHERE id=$1`, id).Scan(&deliveryID, &notificationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE notification_deliveries SET status='SENT' WHERE id=$1`, deliveryID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO email_outbox(to_addresses,subject,status,metadata) VALUES ('[]','Pilot','RETRY',jsonb_build_object('delivery_id',$1::text))`, deliveryID); err != nil {
		t.Fatal(err)
	}
	status, err := repo.GetNotification(ctx, "pilot", id)
	if err != nil || status.Status != "RETRY" {
		t.Fatalf("handoff mistaken for delivery: %+v %v", status, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE email_outbox SET status='SENT'`); err != nil {
		t.Fatal(err)
	}
	status, err = repo.GetNotification(ctx, "pilot", id)
	if err != nil || status.Status != "SENT" {
		t.Fatal(status, err)
	}
	// Suspension gates both authentication and already-authenticated submissions.
	if _, err := db.ExecContext(ctx, `UPDATE ihp_systems SET lifecycle_status='suspended' WHERE client_id='pilot'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ResolveIdentity(ctx, token); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := repo.SubmitNotification(ctx, p, "suspended", validNotification()); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE ihp_systems SET lifecycle_status='active',enabled=true WHERE client_id='pilot'`); err != nil {
		t.Fatal(err)
	}
	input.Enabled = false
	if _, err := repo.SaveIdentity(ctx, "pilot", token.ClientID, "operator", input); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ResolveIdentity(ctx, token); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	input.Enabled = true
	if _, err := repo.SaveIdentity(ctx, "pilot", token.ClientID, "operator", input); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_application_notifications(application_client_id,idempotency_key,request_hash,created_by) SELECT 'pilot','quota-'||i,'hash','test' FROM generate_series(1,59) i`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SubmitNotification(ctx, p, "over-quota", validNotification()); !errors.Is(err, ErrQuota) {
		t.Fatal(err)
	}
	replay, err := repo.SubmitNotification(ctx, p, "same-key", validNotification())
	if err != nil || !replay.Replayed {
		t.Fatalf("quota blocked safe replay: %+v %v", replay, err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM notifications WHERE id=$1`, notificationID); err != nil {
		t.Fatal(err)
	}
	replay, err = repo.SubmitNotification(ctx, p, "same-key", validNotification())
	if err != nil || replay.Status != "EXPIRED" || !replay.Replayed {
		t.Fatalf("cleanup recreated delivery: %+v %v", replay, err)
	}
	// Queue inserts roll back if their required audit cannot be written.
	blocked := token
	blocked.ClientID = "blocked-service"
	blocked.Username = "service-account-blocked-service"
	if _, err := repo.SaveIdentity(ctx, "blocked", blocked.ClientID, "operator", input); err != nil {
		t.Fatal(err)
	}
	bp, err := repo.ResolveIdentity(ctx, blocked)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE ihp_rbac_audit_events ADD CONSTRAINT reject_blocked_queue_audit CHECK (action <> 'application.notification.queued' OR system_client_id <> 'blocked')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SubmitNotification(ctx, bp, "audit-failure", validNotification()); err == nil {
		t.Fatal("audit failure ignored")
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM notifications WHERE target_role='application:blocked'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial queue mutation: %d %v", count, err)
	}
}
