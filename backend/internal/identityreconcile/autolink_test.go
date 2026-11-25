package identityreconcile

import (
	"context"
	"errors"
	"testing"
)

func (f *batchFixture) autoLinker() AutoLinker {
	return AutoLinker{Batch: f.svc, Target: func(context.Context, string) (BatchTarget, error) { return f.target, nil }}
}

func TestAutoLinkLinksReuseAndCreateRowsOnly(t *testing.T) {
	f := newBatchFixture()
	res, err := f.autoLinker().Run(context.Background(), "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != RunCompleted || res.Approved != 2 || res.Outcomes["done"] != 2 {
		t.Fatalf("result = %+v", res)
	}
	if res.Proposals[ProposalReuse] != 1 || res.Proposals[ProposalCreate] != 1 || res.Proposals[ProposalReview] == 0 {
		t.Fatalf("proposals = %+v", res.Proposals)
	}
	if got := f.app.links["a-reuse"]; got != verified("subj-reuse") {
		t.Fatalf("reuse link = %+v", got)
	}
	if got := f.app.links["b-create"]; got != verified("new-b-create") {
		t.Fatalf("create link = %+v", got)
	}
	if len(f.app.links) != 2 {
		t.Fatalf("only reuse and create rows may be linked, got %+v", f.app.links)
	}
	if f.dir.ensures != 1 || len(f.dir.grants) != 2 {
		t.Fatalf("ensures = %d, grants = %v", f.dir.ensures, f.dir.grants)
	}

	run, rows, err := f.svc.GetRun(context.Background(), res.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.CreatedBy != AutoLinkActor {
		t.Fatalf("run created by %q", run.CreatedBy)
	}
	for _, row := range rows {
		approved := row.Decision == DecisionApproved
		if approved != (row.Proposal == ProposalReuse || row.Proposal == ProposalCreate) {
			t.Fatalf("row %s (%s) decision %s", row.AccountID, row.Proposal, row.Decision)
		}
		if approved && (row.ReviewedBy != AutoLinkActor || row.ReviewNote == "") {
			t.Fatalf("row %s reviewer %q note %q", row.AccountID, row.ReviewedBy, row.ReviewNote)
		}
	}
}

func TestAutoLinkIsIdempotent(t *testing.T) {
	f := newBatchFixture()
	if _, err := f.autoLinker().Run(context.Background(), "alerts"); err != nil {
		t.Fatal(err)
	}
	f.dir.access["subj-reuse"], f.dir.access["new-b-create"] = true, true
	// A 1.1 application reports the links it stored in its account listing.
	for i, account := range f.app.accounts {
		if link, ok := f.app.links[account.ID]; ok {
			f.app.accounts[i].Link = link
		}
	}
	linkCalls, ensures := f.app.linkCalls, f.dir.ensures

	res, err := f.autoLinker().Run(context.Background(), "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if res.Approved != 0 || res.Status != RunCompleted || res.Proposals[ProposalLinked] < 2 {
		t.Fatalf("second pass = %+v", res)
	}
	if f.app.linkCalls != linkCalls || f.dir.ensures != ensures {
		t.Fatalf("second pass changed state: links %d→%d, ensures %d→%d", linkCalls, f.app.linkCalls, ensures, f.dir.ensures)
	}
}

func TestAutoLinkResumesItsFailedRun(t *testing.T) {
	f := newBatchFixture()
	f.dir.ensureErr = []error{errors.New("smtp down")}
	first, err := f.autoLinker().Run(context.Background(), "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != RunFailed || first.Outcomes["failed"] != 1 {
		t.Fatalf("first pass = %+v", first)
	}
	if _, linked := f.app.links["b-create"]; linked {
		t.Fatal("create row linked despite the failed invitation")
	}

	second, err := f.autoLinker().Run(context.Background(), "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Resumed) != 1 || second.Resumed[0] != first.RunID {
		t.Fatalf("resumed = %v, want [%s]", second.Resumed, first.RunID)
	}
	if got := f.app.links["b-create"]; got != verified("new-b-create") {
		t.Fatalf("create link after resume = %+v", got)
	}
	run, _, err := f.svc.GetRun(context.Background(), first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != RunCompleted {
		t.Fatalf("resumed run status = %s", run.Status)
	}
}

func TestAutoLinkStopsWhenTargetUnavailable(t *testing.T) {
	f := newBatchFixture()
	linker := AutoLinker{Batch: f.svc, Target: func(context.Context, string) (BatchTarget, error) {
		return BatchTarget{}, errors.New("application unreachable")
	}}
	if _, err := linker.Run(context.Background(), "alerts"); err == nil {
		t.Fatal("expected an error")
	}
	if runs, _ := f.svc.ListRuns(context.Background(), "alerts", 10); len(runs) != 0 {
		t.Fatalf("no run may be stored, got %d", len(runs))
	}
}
