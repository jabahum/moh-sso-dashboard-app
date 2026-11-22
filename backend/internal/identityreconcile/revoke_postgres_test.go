package identityreconcile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Exercises migration 64 (up, down, up), its constraints and PostgresRevocations.
func TestIdentitySyncRevocationsPostgres(t *testing.T) {
	db := registryTestDatabase(t)
	ctx := context.Background()
	run := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "db", "migrations", name))
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
	for _, name := range []string{
		"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql",
		"000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql",
		"000038_add_rbac_groups.up.sql", "000048_application_registry_lifecycle.up.sql",
		"000049_application_identities.up.sql", "000061_service_only_system_roles.up.sql",
		"000062_identity_sync_integrations.up.sql", "000064_identity_sync_revocations.up.sql",
	} {
		run(name)
	}
	run("000064_identity_sync_revocations.down.sql")
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('ihp_identity_sync_revocations') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("down migration left the table: %v %v", exists, err)
	}
	run("000064_identity_sync_revocations.up.sql")

	exec(`INSERT INTO ihp_systems (client_id, display_name) VALUES ('alerts', 'Alerts'), ('clims', 'CLIMS')`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_application_identities (oauth_client_id, application_client_id, subject, enabled) VALUES ('alerts-identity-sync', 'alerts', $1, TRUE)`, uuid.NewString())
	exec(`INSERT INTO ihp_identity_sync_integrations (application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format, enabled)
		VALUES ('alerts', 'https://alerts.example.test', 'alerts-identity-sync', 'alerts_access', 'env:ALERTS_SYNC_CLIENT_SECRET', 'opaque', TRUE)`)

	const issuer = "https://sso.example.test/realms/moh"
	subject := uuid.NewString()
	insert := func(app, subject string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO ihp_identity_sync_revocations (application_client_id, issuer, subject, revoked_by, reason)
			VALUES ($1, $2, $3, 'ops', 'left')`, app, issuer, subject)
		return err
	}
	pqCode := func(err error) string {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			return string(pqErr.Code)
		}
		return ""
	}

	// Constraints.
	if err := insert("clims", subject); pqCode(err) != "23503" {
		t.Errorf("revocation for an application without integration: %v", err)
	}
	if err := insert("alerts", subject); err != nil {
		t.Fatal(err)
	}
	if err := insert("alerts", subject); pqCode(err) != "23505" {
		t.Errorf("second active revocation: %v", err)
	}
	for name, query := range map[string]string{
		"unknown step status": `UPDATE ihp_identity_sync_revocations SET sessions_revoked = 'skipped'`,
		"role status":         `UPDATE ihp_identity_sync_revocations SET role_removed = 'ok'`,
		"consents status":     `UPDATE ihp_identity_sync_revocations SET consents_revoked = ''`,
		"upstream body":       `UPDATE ihp_identity_sync_revocations SET last_error = 'HTTP 503: <html>'`,
		"negative attempts":   `UPDATE ihp_identity_sync_revocations SET attempts = -1`,
		"restore without by":  `UPDATE ihp_identity_sync_revocations SET restored_at = now(), restore_reason = 'x'`,
		"restore no reason":   `UPDATE ihp_identity_sync_revocations SET restored_at = now(), restored_by = 'ops'`,
		"restorer, no time":   `UPDATE ihp_identity_sync_revocations SET restored_by = 'ops', restore_reason = 'x'`,
		"padded subject":      `UPDATE ihp_identity_sync_revocations SET subject = ' x'`,
		"empty reason":        `UPDATE ihp_identity_sync_revocations SET reason = ''`,
		"non-URL issuer":      `UPDATE ihp_identity_sync_revocations SET issuer = 'moh'`,
	} {
		if _, err := db.ExecContext(ctx, query); pqCode(err) != "23514" {
			t.Errorf("%s: err = %v, want check_violation", name, err)
		}
	}
	exec(`UPDATE ihp_identity_sync_revocations SET restored_at = now(), restored_by = 'ops', restore_reason = 'mistake'`)
	if err := insert("alerts", subject); err != nil {
		t.Fatalf("new revocation after restore: %v", err)
	}
	// An integration with revocation history cannot be deleted.
	if err := (&PostgresRegistry{DB: db}).DeleteIntegration(ctx, "alerts", "ops", "retire"); !errors.Is(err, ErrIntegrationInUse) {
		t.Fatalf("delete integration with revocations: %v", err)
	}
	exec(`DELETE FROM ihp_identity_sync_revocations`)

	// Repository round trip.
	repo := &PostgresRevocations{DB: db}
	if _, _, err := repo.BeginRevocation(ctx, Revocation{ApplicationClientID: "clims", Issuer: issuer, Subject: subject, RevokedBy: "ops", Reason: "x"}); !errors.Is(err, ErrIntegrationNotFound) {
		t.Fatalf("begin for unregistered application: %v", err)
	}
	started, created, err := repo.BeginRevocation(ctx, Revocation{ApplicationClientID: "alerts", Issuer: issuer, Subject: subject, RevokedBy: "ops", Reason: "left"})
	if err != nil || !created {
		t.Fatalf("begin = %+v, %v, %v", started, created, err)
	}
	if started.ID == "" || started.RoleRemoved != StepPending || started.SessionsRevoked != StepPending || !started.Active() || started.Attempts != 0 {
		t.Fatalf("started = %+v", started)
	}
	again, created, err := repo.BeginRevocation(ctx, Revocation{ApplicationClientID: "alerts", Issuer: issuer, Subject: subject, RevokedBy: "other", Reason: "again"})
	if err != nil || created || again.ID != started.ID || again.RevokedBy != "ops" {
		t.Fatalf("repeat begin = %+v, %v, %v", again, created, err)
	}

	started.RoleRemoved, started.ConsentsRevoked, started.SessionsRevoked = StepDone, StepDone, StepFailed
	started.Attempts, started.LastError = 1, CodeSessionsAppUnavailable
	if err := repo.SaveRevocationAttempt(ctx, started, "ops"); err != nil {
		t.Fatal(err)
	}
	active, err := repo.ActiveRevocation(ctx, "alerts", issuer, subject)
	if err != nil || active.SessionsRevoked != StepFailed || active.LastError != CodeSessionsAppUnavailable || active.Attempts != 1 {
		t.Fatalf("active = %+v, %v", active, err)
	}
	unfinished, err := repo.ListRevocations(ctx, RevocationFilter{OnlyUnfinished: true})
	if err != nil || len(unfinished) != 1 || unfinished[0].ID != started.ID {
		t.Fatalf("unfinished = %+v, %v", unfinished, err)
	}
	started.SessionsRevoked, started.Attempts, started.LastError = StepUnsupported, 2, ""
	if err := repo.SaveRevocationAttempt(ctx, started, "ops"); err != nil {
		t.Fatal(err)
	}
	if unfinished, err = repo.ListRevocations(ctx, RevocationFilter{Application: "alerts", OnlyUnfinished: true}); err != nil || len(unfinished) != 0 {
		t.Fatalf("unsupported listed as unfinished: %+v, %v", unfinished, err)
	}
	if listed, err := repo.ListRevocations(ctx, RevocationFilter{Application: "alerts"}); err != nil || len(listed) != 1 || listed[0].SessionsRevoked != StepUnsupported || listed[0].LastError != "" {
		t.Fatalf("listed = %+v, %v", listed, err)
	}

	service := &RevokeService{Repository: repo}
	if revoked, err := service.IsRevoked(ctx, "alerts", issuer, subject); err != nil || !revoked {
		t.Fatalf("IsRevoked = %v, %v", revoked, err)
	}
	restored, err := repo.RestoreRevocation(ctx, "alerts", issuer, subject, "lead", "returned")
	if err != nil || restored.Active() || restored.RestoredBy != "lead" || restored.RestoreReason != "returned" {
		t.Fatalf("restored = %+v, %v", restored, err)
	}
	if _, err := repo.RestoreRevocation(ctx, "alerts", issuer, subject, "lead", "again"); !errors.Is(err, ErrRevocationNotFound) {
		t.Fatalf("second restore: %v", err)
	}
	if err := repo.SaveRevocationAttempt(ctx, started, "ops"); !errors.Is(err, ErrRevocationNotFound) {
		t.Fatalf("attempt saved on a restored revocation: %v", err)
	}
	if revoked, err := service.IsRevoked(ctx, "alerts", issuer, subject); err != nil || revoked {
		t.Fatalf("IsRevoked after restore = %v, %v", revoked, err)
	}
	if listed, err := repo.ListRevocations(ctx, RevocationFilter{}); err != nil || len(listed) != 0 {
		t.Fatalf("restored listed as active: %+v, %v", listed, err)
	}
	if listed, err := repo.ListRevocations(ctx, RevocationFilter{IncludeRestored: true, Limit: 10}); err != nil || len(listed) != 1 || listed[0].RestoredAt == nil {
		t.Fatalf("history = %+v, %v", listed, err)
	}

	var recorded, attempted, restoredEvents int
	if err := db.QueryRowContext(ctx, `SELECT
		count(*) FILTER (WHERE action = $1),
		count(*) FILTER (WHERE action = $2),
		count(*) FILTER (WHERE action = $3)
		FROM ihp_rbac_audit_events WHERE resource_type = 'identity_sync_revocation' AND system_client_id = 'alerts'
		AND resource_id = $4 AND details->>'subject' = $5`,
		AuditRevocationRecorded, AuditRevocationAttempted, AuditRevocationRestored, started.ID, subject).Scan(&recorded, &attempted, &restoredEvents); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 || attempted != 2 || restoredEvents != 1 {
		t.Fatalf("audit events: %d recorded, %d attempted, %d restored; want 1, 2, 1", recorded, attempted, restoredEvents)
	}
}
