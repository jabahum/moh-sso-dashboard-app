package identityreconcile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Exercises migration 63 (up, down, up), its constraints and the batch
// repository, then runs the batch service end to end on PostgreSQL.
func TestIdentitySyncBatchPostgres(t *testing.T) {
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
		"000062_identity_sync_integrations.up.sql", "000063_identity_sync_runs.up.sql",
	} {
		run(name)
	}
	run("000063_identity_sync_runs.down.sql")
	run("000063_identity_sync_runs.up.sql")

	exec(`INSERT INTO ihp_systems (client_id, display_name) VALUES ('alerts', 'Alerts')`)
	exec(`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`)
	exec(`INSERT INTO ihp_application_identities (oauth_client_id, application_client_id, subject, enabled)
		VALUES ('alerts-identity-sync', 'alerts', $1, TRUE)`, uuid.NewString())
	exec(`INSERT INTO ihp_identity_sync_integrations (application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format, enabled)
		VALUES ('alerts', 'https://alerts.example.test', 'alerts-identity-sync', 'alerts_access', 'env:ALERTS_SYNC_SECRET', 'opaque', TRUE)`)

	repo := &PostgresBatchRepository{DB: db}
	if _, err := repo.CreateRun(ctx, BatchRun{ApplicationClientID: "unregistered", Issuer: batchIssuer, Status: RunPreviewed, CreatedBy: "op"}, nil); !errors.Is(err, ErrIntegrationNotFound) {
		t.Fatalf("run for unregistered application: %v", err)
	}

	// Constraint checks on a scratch run.
	var scratch string
	if err := db.QueryRowContext(ctx, `INSERT INTO ihp_identity_sync_runs (application_client_id, issuer, status, created_by)
		VALUES ('alerts', $1, 'previewed', 'op') RETURNING id::text`, batchIssuer).Scan(&scratch); err != nil {
		t.Fatal(err)
	}
	for name, query := range map[string]string{
		"http issuer":               `INSERT INTO ihp_identity_sync_runs (application_client_id, issuer, status, created_by) VALUES ('alerts', 'http://x', 'previewed', 'op')`,
		"unknown run status":        `INSERT INTO ihp_identity_sync_runs (application_client_id, issuer, status, created_by) VALUES ('alerts', 'https://x', 'done', 'op')`,
		"blank creator":             `INSERT INTO ihp_identity_sync_runs (application_client_id, issuer, status, created_by) VALUES ('alerts', 'https://x', 'previewed', ' ')`,
		"unknown proposal":          `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal) VALUES ('` + scratch + `', 'a1', 'grant')`,
		"bad account id":            `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal) VALUES ('` + scratch + `', '-bad id', 'reuse')`,
		"approved without reviewer": `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, decision) VALUES ('` + scratch + `', 'a2', 'reuse', 'approved')`,
		"rejected with blank reviewer": `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, decision, reviewed_by, reviewed_at)
			VALUES ('` + scratch + `', 'a3', 'reuse', 'rejected', ' ', now())`,
		"granted linked_no_access": `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, grant_status) VALUES ('` + scratch + `', 'a4', 'linked_no_access', 'granted')`,
		"granted revoked":          `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, grant_status) VALUES ('` + scratch + `', 'a5', 'revoked', 'granted')`,
		"upstream body as error":   `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, last_error) VALUES ('` + scratch + `', 'a6', 'reuse', '{"error":"boom"}')`,
		"negative attempts":        `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, attempts) VALUES ('` + scratch + `', 'a7', 'reuse', -1)`,
		"long note":                `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, review_note) VALUES ('` + scratch + `', 'a8', 'reuse', repeat('n', 1001))`,
		"unknown link status":      `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, link_status) VALUES ('` + scratch + `', 'a9', 'reuse', 'done')`,
	} {
		if _, err := db.ExecContext(ctx, query); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	exec(`INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal, last_error) VALUES ($1, 'ok-1', 'reuse', 'link_conflict')`, scratch)
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_identity_sync_rows (run_id, account_id, proposal) VALUES ($1, 'ok-1', 'create')`, scratch); err == nil {
		t.Error("duplicate account in a run accepted")
	}
	// Runs keep their integration: deleting it is refused while runs exist.
	if err := (&PostgresRegistry{DB: db}).DeleteIntegration(ctx, "alerts", "op", "retire"); !errors.Is(err, ErrIntegrationInUse) {
		t.Fatalf("delete integration with runs: %v", err)
	}
	exec(`DELETE FROM ihp_identity_sync_runs WHERE id = $1`, scratch)
	var orphans int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM ihp_identity_sync_rows WHERE run_id = $1`, scratch).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("rows not removed with their run: %d, %v", orphans, err)
	}

	// The batch service end to end on PostgreSQL.
	f := newBatchFixture()
	f.svc.Repo = repo
	created, rows, err := f.svc.StartRun(ctx, f.target, "cli:operator")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 || created.Status != RunPreviewed || created.CreatedAt.IsZero() {
		t.Fatalf("run %+v with %d rows", created, len(rows))
	}
	got := byAccount(rows)
	if got["a-reuse"].Proposal != ProposalReuse || got["a-reuse"].ProposedSubject != "subj-reuse" ||
		got["b-create"].UserStatus != UserPending || got["g-noaccess"].GrantStatus != GrantNotRequired || got["e-noemail"].ProposedSubject != "" {
		t.Fatalf("stored rows = %+v", got)
	}
	if runs, err := f.svc.ListRuns(ctx, "alerts", 5); err != nil || len(runs) != 1 || runs[0].ID != created.ID {
		t.Fatalf("list runs = %+v, %v", runs, err)
	}
	if _, err := f.svc.ApproveRows(ctx, created.ID, []string{"g-noaccess"}, "alice", ""); !errors.Is(err, ErrRowNotApprovable) {
		t.Fatalf("approve linked_no_access: %v", err)
	}
	if _, err := f.svc.ApproveRows(ctx, created.ID, []string{"nope"}, "alice", ""); !errors.Is(err, ErrRowNotFound) {
		t.Fatalf("approve missing row: %v", err)
	}
	if _, err := f.svc.ApproveRows(ctx, uuid.NewString(), []string{"a-reuse"}, "alice", ""); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("approve in missing run: %v", err)
	}
	approved, err := f.svc.ApproveRows(ctx, created.ID, []string{"a-reuse", "b-create"}, "alice", "checked")
	if err != nil || approved[0].ReviewedAt == nil || approved[0].ReviewNote != "checked" {
		t.Fatalf("approve = %+v, %v", approved, err)
	}
	if _, err := f.svc.RejectRows(ctx, created.ID, []string{"c-ambiguous"}, "bob", ""); err != nil {
		t.Fatal(err)
	}

	// A concurrent claim is refused; the claim reports the earlier status.
	before, err := repo.ClaimRun(ctx, created.ID, []RunStatus{RunPreviewed}, time.Hour)
	if err != nil || before.Status != RunPreviewed {
		t.Fatalf("claim = %+v, %v", before, err)
	}
	if _, err := repo.ClaimRun(ctx, created.ID, []RunStatus{RunPreviewed}, time.Hour); !errors.Is(err, ErrRunBusy) {
		t.Fatalf("second claim: %v", err)
	}
	if err := repo.FinishRun(ctx, created.ID, RunPreviewed); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRun(ctx, created.ID, RunCompleted); err == nil {
		t.Fatal("finished a run that was not applying")
	}

	f.dir.grantErr = []error{errors.New("grant failed")}
	result, err := f.svc.Apply(ctx, f.target, created.ID)
	if err != nil || result.Status != RunFailed {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	_, rows, err = f.svc.GetRun(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	got = byAccount(rows)
	if r := got["a-reuse"]; r.LinkStatus != LinkLinked || r.GrantStatus != GrantFailed || r.LastError != CodeGrantFailed || r.Attempts != 1 || r.ReviewedBy != "alice" {
		t.Fatalf("a-reuse after apply = %+v", r)
	}
	if r := got["b-create"]; !r.Finished() || r.UserStatus != UserInvited || r.ProposedSubject != "new-b-create" {
		t.Fatalf("b-create after apply = %+v", r)
	}
	links := f.app.linkCalls
	result, err = f.svc.Resume(ctx, f.target, created.ID)
	if err != nil || result.Status != RunCompleted || f.app.linkCalls != links {
		t.Fatalf("resume = %+v, %v, links %d->%d", result, err, links, f.app.linkCalls)
	}
	stored, rows, err := f.svc.GetRun(ctx, created.ID)
	if err != nil || stored.Status != RunCompleted {
		t.Fatalf("run after resume = %+v, %v", stored, err)
	}
	if r := byAccount(rows)["a-reuse"]; !r.Finished() || r.Attempts != 2 || r.LastError != "" {
		t.Fatalf("a-reuse after resume = %+v", r)
	}
	if _, err := f.svc.RejectRows(ctx, created.ID, []string{"a-reuse"}, "bob", ""); !errors.Is(err, ErrRowStarted) {
		t.Fatalf("decision change after apply: %v", err)
	}
	exec(`UPDATE ihp_identity_sync_runs SET status = 'cancelled' WHERE id = $1`, created.ID)
	if _, err := f.svc.Apply(ctx, f.target, created.ID); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("apply cancelled run: %v", err)
	}

	run("000063_identity_sync_runs.down.sql")
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('ihp_identity_sync_rows') IS NOT NULL OR to_regclass('ihp_identity_sync_runs') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("down migration left tables: %v, %v", exists, err)
	}
}
