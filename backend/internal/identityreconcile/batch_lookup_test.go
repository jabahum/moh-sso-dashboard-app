package identityreconcile

import (
	"context"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// An account the re-read walk did not return (a page boundary moved, the
// application briefly hid it) is not proof of ineligibility: the row must stay
// retryable. Only a 404 from PUT /accounts/{id}/identity makes it ineligible.
func TestBatchAccountMissingFromWalkIsRetryable(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	all := f.app.accounts
	f.app.accounts = []identitysync.Account{{ID: "b-create", Email: "create@example.org"}} // a-reuse skipped
	result, err := f.svc.Apply(ctx, f.target, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	row := f.rows(t, run.ID)["a-reuse"]
	if row.LinkStatus != LinkFailed || row.LastError != CodeAccountLookupFailed || row.Terminal() {
		t.Fatalf("skipped account row = %+v, want link failed / %s (retryable)", row, CodeAccountLookupFailed)
	}
	if result.Status != RunFailed || result.Rows[0].Outcome != "failed" {
		t.Fatalf("result = %+v", result)
	}
	// The account is listed again: resume links and grants it.
	f.app.accounts = all
	if _, err := f.svc.Resume(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	if row := f.rows(t, run.ID)["a-reuse"]; !row.Finished() {
		t.Fatalf("resume did not finish the row: %+v", row)
	}
}
