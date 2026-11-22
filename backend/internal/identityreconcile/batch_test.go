package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

const batchIssuer = "https://sso.example.test/realms/moh"

// memoryBatchRepo mirrors PostgresBatchRepository, including migration 63's
// row constraints, so state-machine bugs fail the unit tests too.
type memoryBatchRepo struct {
	mu   sync.Mutex
	runs map[string]BatchRun
	rows map[string]map[string]BatchRow
}

func newMemoryBatchRepo() *memoryBatchRepo {
	return &memoryBatchRepo{runs: map[string]BatchRun{}, rows: map[string]map[string]BatchRow{}}
}

func checkRowConstraints(r BatchRow) error {
	if r.Decision != DecisionPending && (strings.TrimSpace(r.ReviewedBy) == "" || r.ReviewedAt == nil) {
		return errors.New("violates ihp_identity_sync_rows_reviewer")
	}
	if (r.Proposal == ProposalLinkedNoAccess || r.Proposal == ProposalRevoked) && r.GrantStatus == GrantGranted {
		return errors.New("violates ihp_identity_sync_rows_no_grant")
	}
	if r.LastError != "" && strings.ContainsAny(r.LastError, " :{}\"") {
		return fmt.Errorf("last_error %q is not a stable code", r.LastError)
	}
	return nil
}

func (m *memoryBatchRepo) CreateRun(_ context.Context, run BatchRun, rows []BatchRow) (BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run.ID = uuid.NewString()
	run.CreatedAt, run.UpdatedAt = time.Now(), time.Now()
	m.runs[run.ID] = run
	m.rows[run.ID] = map[string]BatchRow{}
	for _, r := range rows {
		if _, dup := m.rows[run.ID][r.AccountID]; dup {
			return BatchRun{}, errors.New("duplicate account")
		}
		if err := checkRowConstraints(r); err != nil {
			return BatchRun{}, err
		}
		r.ID, r.RunID = uuid.NewString(), run.ID
		m.rows[run.ID][r.AccountID] = r
	}
	return run, nil
}

func (m *memoryBatchRepo) GetRun(_ context.Context, id string) (BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return BatchRun{}, ErrRunNotFound
	}
	return run, nil
}

func (m *memoryBatchRepo) ListRuns(_ context.Context, app string, limit int) ([]BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []BatchRun
	for _, run := range m.runs {
		if run.ApplicationClientID == app {
			out = append(out, run)
		}
	}
	return out, nil
}

func (m *memoryBatchRepo) ListRows(_ context.Context, id string) ([]BatchRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []BatchRow
	for _, r := range m.rows[id] {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out, nil
}

func (m *memoryBatchRepo) TransitionRow(_ context.Context, runID, accountID string, mutate func(BatchRun, *BatchRow) error) (BatchRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[runID]
	if !ok {
		return BatchRow{}, ErrRunNotFound
	}
	row, ok := m.rows[runID][accountID]
	if !ok {
		return BatchRow{}, ErrRowNotFound
	}
	next := row
	if err := mutate(run, &next); err != nil {
		return BatchRow{}, err
	}
	if err := checkRowConstraints(next); err != nil {
		return BatchRow{}, err
	}
	next.UpdatedAt = time.Now()
	m.rows[runID][accountID] = next
	run.UpdatedAt = time.Now()
	m.runs[runID] = run
	return next, nil
}

func (m *memoryBatchRepo) ClaimRun(_ context.Context, runID string, from []RunStatus, staleAfter time.Duration) (BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[runID]
	if !ok {
		return BatchRun{}, ErrRunNotFound
	}
	before := run
	switch {
	case run.Status == RunCancelled:
		return BatchRun{}, ErrRunCancelled
	case run.Status == RunApplying && time.Since(run.UpdatedAt) < staleAfter:
		return BatchRun{}, ErrRunBusy
	}
	allowed := run.Status == RunApplying
	for _, s := range from {
		allowed = allowed || s == run.Status
	}
	if !allowed {
		return BatchRun{}, fmt.Errorf("run is %s", run.Status)
	}
	run.Status, run.UpdatedAt = RunApplying, time.Now()
	m.runs[runID] = run
	return before, nil
}

func (m *memoryBatchRepo) FinishRun(_ context.Context, runID string, status RunStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.runs[runID]
	if run.Status != RunApplying {
		return errors.New("not applying")
	}
	run.Status = status
	m.runs[runID] = run
	return nil
}

// force overwrites a row, bypassing the service (simulating direct edits).
func (m *memoryBatchRepo) force(t *testing.T, runID, accountID string, mutate func(*BatchRow)) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.rows[runID][accountID]
	mutate(&row)
	m.rows[runID][accountID] = row
}

type fakeBatchApp struct {
	mu        sync.Mutex
	accounts  []identitysync.Account
	linkErr   map[string][]error // per account, consumed in order
	links     map[string]identitysync.Identity
	linkCalls int
	listCalls int
}

func (a *fakeBatchApp) ListEligibleAccounts(_ context.Context, cursor string, limit int) (identitysync.AccountPage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.listCalls++
	start := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "c%d", &start)
	}
	end := min(start+limit, len(a.accounts))
	page := identitysync.AccountPage{Accounts: append([]identitysync.Account(nil), a.accounts[start:end]...)}
	if end < len(a.accounts) {
		page.NextCursor = fmt.Sprintf("c%d", end)
	}
	return page, nil
}

func (a *fakeBatchApp) LinkIdentity(_ context.Context, id string, identity identitysync.Identity) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.linkCalls++
	if errs := a.linkErr[id]; len(errs) > 0 {
		a.linkErr[id] = errs[1:]
		if errs[0] != nil {
			return errs[0]
		}
	}
	if a.links == nil {
		a.links = map[string]identitysync.Identity{}
	}
	a.links[id] = identity
	return nil
}

type fakeBatchDirectory struct {
	mu         sync.Mutex
	byEmail    map[string][]identitysync.Identity
	access     map[string]bool
	created    map[string]string // account ID -> subject
	ensureErr  []error
	grantErr   []error
	grants     []string
	findCalls  int
	ensures    int
	findErr    error
	accessErrs error
}

func (d *fakeBatchDirectory) FindEmail(_ context.Context, email string) ([]identitysync.Identity, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.findCalls++
	if d.findErr != nil {
		return nil, d.findErr
	}
	return append([]identitysync.Identity(nil), d.byEmail[email]...), nil
}

func (d *fakeBatchDirectory) EnsureUser(_ context.Context, account identitysync.Account) (identitysync.Identity, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ensures++
	if len(d.ensureErr) > 0 {
		err := d.ensureErr[0]
		d.ensureErr = d.ensureErr[1:]
		if err != nil {
			return identitysync.Identity{}, err
		}
	}
	if d.created == nil {
		d.created = map[string]string{}
	}
	subject, ok := d.created[account.ID]
	if !ok {
		subject = "new-" + account.ID
		d.created[account.ID] = subject
		// The new user is unverified, so it matches by email with no identity.
		d.byEmail[account.Email] = []identitysync.Identity{{}}
	}
	return identitysync.Identity{Issuer: batchIssuer, Subject: subject}, nil
}

func (d *fakeBatchDirectory) GrantAccess(_ context.Context, identity identitysync.Identity) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.grantErr) > 0 {
		err := d.grantErr[0]
		d.grantErr = d.grantErr[1:]
		if err != nil {
			return err
		}
	}
	d.grants = append(d.grants, identity.Subject)
	return nil
}

func (d *fakeBatchDirectory) HasAccess(_ context.Context, identity identitysync.Identity) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access[identity.Subject], d.accessErrs
}

type fakeRevocations struct {
	mu      sync.Mutex
	subject map[string]bool
	err     error
}

func (f *fakeRevocations) IsRevoked(_ context.Context, app, issuer, subject string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if app != "alerts" || issuer != batchIssuer {
		return false, errors.New("unexpected revocation scope")
	}
	return f.subject[subject], f.err
}

func (f *fakeRevocations) revoke(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subject[subject] = true
}

func verified(subject string) identitysync.Identity {
	return identitysync.Identity{Issuer: batchIssuer, Subject: subject}
}

type batchFixture struct {
	repo   *memoryBatchRepo
	app    *fakeBatchApp
	dir    *fakeBatchDirectory
	revs   *fakeRevocations
	svc    BatchService
	target BatchTarget
}

func newBatchFixture() *batchFixture {
	f := &batchFixture{
		repo: newMemoryBatchRepo(),
		app: &fakeBatchApp{linkErr: map[string][]error{}, accounts: []identitysync.Account{
			{ID: "a-reuse", Email: "reuse@example.org"},
			{ID: "b-create", Email: "create@example.org"},
			{ID: "c-ambiguous", Email: "twice@example.org"},
			{ID: "d-unverified", Email: "unverified@example.org"},
			{ID: "e-noemail", Email: " "},
			{ID: "f-linked", Email: "linked@example.org", Link: verified("subj-linked")},
			{ID: "g-noaccess", Email: "noaccess@example.org", Link: verified("subj-noaccess")},
			{ID: "h-revoked-linked", Email: "rl@example.org", Link: verified("subj-revoked-linked")},
			{ID: "i-revoked-match", Email: "rm@example.org"},
			{ID: "j-other-issuer", Email: "other@example.org", Link: identitysync.Identity{Issuer: "https://elsewhere.test/realms/x", Subject: "s"}},
			{ID: "a-reuse", Email: "reuse@example.org"}, // listed twice during the walk
		}},
		dir: &fakeBatchDirectory{
			byEmail: map[string][]identitysync.Identity{
				"reuse@example.org":      {verified("subj-reuse")},
				"twice@example.org":      {verified("x1"), verified("x2")},
				"unverified@example.org": {{}},
				"rm@example.org":         {verified("subj-revoked-match")},
			},
			access: map[string]bool{"subj-linked": true},
		},
		revs: &fakeRevocations{subject: map[string]bool{"subj-revoked-linked": true, "subj-revoked-match": true}},
	}
	f.svc = BatchService{Repo: f.repo, Revocations: f.revs, PageSize: 3}
	f.target = BatchTarget{Application: "alerts", Issuer: batchIssuer, Accounts: f.app, Directory: f.dir}
	return f
}

func (f *batchFixture) start(t *testing.T) (BatchRun, map[string]BatchRow) {
	t.Helper()
	run, rows, err := f.svc.StartRun(context.Background(), f.target, "cli:operator")
	if err != nil {
		t.Fatal(err)
	}
	return run, byAccount(rows)
}

func (f *batchFixture) rows(t *testing.T, runID string) map[string]BatchRow {
	t.Helper()
	rows, err := f.repo.ListRows(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	return byAccount(rows)
}

func byAccount(rows []BatchRow) map[string]BatchRow {
	out := map[string]BatchRow{}
	for _, r := range rows {
		out[r.AccountID] = r
	}
	return out
}

func TestBatchProposals(t *testing.T) {
	f := newBatchFixture()
	run, rows := f.start(t)
	if run.Status != RunPreviewed || run.Issuer != batchIssuer || run.CreatedBy != "cli:operator" {
		t.Fatalf("run = %+v", run)
	}
	if len(rows) != 10 {
		t.Fatalf("got %d rows, want 10 (duplicates collapse)", len(rows))
	}
	want := map[string]struct {
		proposal               Proposal
		subject                string
		user, link, grantState string
	}{
		"a-reuse":          {ProposalReuse, "subj-reuse", UserNotRequired, LinkPending, GrantPending},
		"b-create":         {ProposalCreate, "", UserPending, LinkPending, GrantPending},
		"c-ambiguous":      {ProposalReview, "", UserNotRequired, LinkPending, GrantPending},
		"d-unverified":     {ProposalReview, "", UserNotRequired, LinkPending, GrantPending},
		"e-noemail":        {ProposalReview, "", UserNotRequired, LinkPending, GrantPending},
		"f-linked":         {ProposalLinked, "subj-linked", UserNotRequired, LinkLinked, GrantGranted},
		"g-noaccess":       {ProposalLinkedNoAccess, "subj-noaccess", UserNotRequired, LinkLinked, GrantNotRequired},
		"h-revoked-linked": {ProposalRevoked, "subj-revoked-linked", UserNotRequired, LinkLinked, GrantNotRequired},
		"i-revoked-match":  {ProposalRevoked, "subj-revoked-match", UserNotRequired, LinkPending, GrantNotRequired},
		"j-other-issuer":   {ProposalReview, "", UserNotRequired, LinkPending, GrantPending},
	}
	for id, w := range want {
		r := rows[id]
		if r.Proposal != w.proposal || r.ProposedSubject != w.subject || r.UserStatus != w.user || r.LinkStatus != w.link ||
			r.GrantStatus != w.grantState || r.Decision != DecisionPending || r.Attempts != 0 {
			t.Errorf("%s = %+v, want %+v", id, r, w)
		}
	}
	if f.app.linkCalls != 0 || f.dir.ensures != 0 || len(f.dir.grants) != 0 {
		t.Fatal("starting a run must be read-only")
	}
}

func TestBatchStartRunStoresNothingOnError(t *testing.T) {
	for name, breakIt := range map[string]func(*batchFixture){
		"directory":   func(f *batchFixture) { f.dir.findErr = errors.New("keycloak down") },
		"access":      func(f *batchFixture) { f.dir.accessErrs = errors.New("keycloak down") },
		"revocations": func(f *batchFixture) { f.revs.err = errors.New("db down") },
	} {
		t.Run(name, func(t *testing.T) {
			f := newBatchFixture()
			breakIt(f)
			if _, _, err := f.svc.StartRun(context.Background(), f.target, "cli:operator"); err == nil {
				t.Fatal("expected an error")
			}
			if len(f.repo.runs) != 0 {
				t.Fatal("a failed start stored a run")
			}
		})
	}
	f := newBatchFixture()
	if _, _, err := f.svc.StartRun(context.Background(), f.target, "  "); !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("run without creator: %v", err)
	}
}

func TestBatchApprovalRules(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	for _, reviewer := range []string{"", "   ", "bad\x00name", strings.Repeat("r", 201)} {
		if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, reviewer, ""); !errors.Is(err, ErrReviewerRequired) {
			t.Errorf("reviewer %q: %v", reviewer, err)
		}
		if _, err := f.svc.RejectRows(ctx, run.ID, []string{"a-reuse"}, reviewer, ""); !errors.Is(err, ErrReviewerRequired) {
			t.Errorf("reject reviewer %q: %v", reviewer, err)
		}
	}
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", strings.Repeat("n", 1001)); !errors.Is(err, ErrInvalidNote) {
		t.Errorf("long note: %v", err)
	}
	for _, id := range []string{"c-ambiguous", "f-linked", "g-noaccess", "h-revoked-linked", "i-revoked-match"} {
		if _, err := f.svc.ApproveRows(ctx, run.ID, []string{id}, "alice", ""); !errors.Is(err, ErrRowNotApprovable) {
			t.Errorf("approve %s: %v", id, err)
		}
	}
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"zz-missing"}, "alice", ""); !errors.Is(err, ErrRowNotFound) {
		t.Errorf("missing row: %v", err)
	}
	if _, err := f.svc.ApproveRows(ctx, uuid.NewString(), []string{"a-reuse"}, "alice", ""); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("missing run: %v", err)
	}
	if _, err := f.svc.ApproveRows(ctx, "not-a-uuid", []string{"a-reuse"}, "alice", ""); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("bad run id: %v", err)
	}
	approved, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse", "b-create"}, " alice ", " checked HR record ")
	if err != nil || len(approved) != 2 {
		t.Fatalf("approve: %v", err)
	}
	if r := approved[0]; r.Decision != DecisionApproved || r.ReviewedBy != "alice" || r.ReviewNote != "checked HR record" || r.ReviewedAt == nil {
		t.Fatalf("approved row = %+v", r)
	}
	if _, err := f.svc.RejectRows(ctx, run.ID, []string{"c-ambiguous", "g-noaccess"}, "bob", ""); err != nil {
		t.Fatalf("reject: %v", err)
	}
	// Before apply a decision may change; after apply it may not.
	if _, err := f.svc.RejectRows(ctx, run.ID, []string{"b-create"}, "bob", "changed my mind"); err != nil {
		t.Fatalf("re-decide before apply: %v", err)
	}
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"b-create"}, "bob", ""); err != nil {
		t.Fatalf("re-approve before apply: %v", err)
	}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RejectRows(ctx, run.ID, []string{"a-reuse"}, "bob", ""); !errors.Is(err, ErrRowStarted) {
		t.Fatalf("decision change after apply: %v", err)
	}
	f.repo.runs[run.ID] = BatchRun{ID: run.ID, ApplicationClientID: "alerts", Issuer: batchIssuer, Status: RunCancelled}
	if _, err := f.svc.RejectRows(ctx, run.ID, []string{"d-unverified"}, "bob", ""); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("cancelled run: %v", err)
	}
}

func TestBatchApplyStateMachine(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse", "b-create"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	result, err := f.svc.Apply(ctx, f.target, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunCompleted || len(result.Rows) != 2 {
		t.Fatalf("result = %+v", result)
	}
	for _, o := range result.Rows {
		if o.Outcome != "done" {
			t.Errorf("outcome = %+v", o)
		}
	}
	rows := f.rows(t, run.ID)
	reuse, create := rows["a-reuse"], rows["b-create"]
	if !reuse.Finished() || reuse.Attempts != 1 || reuse.LastError != "" || reuse.UserStatus != UserNotRequired {
		t.Errorf("reuse row = %+v", reuse)
	}
	if !create.Finished() || create.UserStatus != UserInvited || create.ProposedSubject != "new-b-create" || create.Attempts != 1 {
		t.Errorf("create row = %+v", create)
	}
	if f.app.links["a-reuse"] != verified("subj-reuse") || f.app.links["b-create"] != verified("new-b-create") {
		t.Errorf("links = %v", f.app.links)
	}
	if len(f.app.links) != 2 || f.dir.ensures != 1 {
		t.Errorf("only approved rows may change: links %v, ensures %d", f.app.links, f.dir.ensures)
	}
	if strings.Join(f.dir.grants, ",") != "subj-reuse,new-b-create" {
		t.Errorf("grants = %v", f.dir.grants)
	}
	run, _ = f.repo.GetRun(ctx, run.ID)
	if run.Status != RunCompleted {
		t.Errorf("run status = %s", run.Status)
	}
	// Applying again processes nothing: every approved row was attempted.
	links, grants := f.app.linkCalls, len(f.dir.grants)
	result, err = f.svc.Apply(ctx, f.target, run.ID)
	if err != nil || len(result.Rows) != 0 || f.app.linkCalls != links || len(f.dir.grants) != grants {
		t.Fatalf("second apply = %+v, %v", result, err)
	}
	// Resuming a finished run also does nothing.
	if result, err = f.svc.Resume(ctx, f.target, run.ID); err != nil || len(result.Rows) != 0 {
		t.Fatalf("resume of finished run = %+v, %v", result, err)
	}
}

func TestBatchApplyRefusesRowsWithoutReviewer(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	// A row marked approved without a reviewer (not possible through the
	// service or migration 63) is still never applied.
	f.repo.force(t, run.ID, "a-reuse", func(r *BatchRow) { r.Decision = DecisionApproved; r.ReviewedBy = " " })
	result, err := f.svc.Apply(ctx, f.target, run.ID)
	if err != nil || len(result.Rows) != 0 || f.app.linkCalls != 0 || len(f.dir.grants) != 0 {
		t.Fatalf("apply = %+v, %v; links %d", result, err, f.app.linkCalls)
	}
}

func TestBatchApplyDirectoryChangeReturnsRowToReview(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	f.dir.byEmail["reuse@example.org"] = []identitysync.Identity{verified("someone-else")}
	result, err := f.svc.Apply(ctx, f.target, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	row := f.rows(t, run.ID)["a-reuse"]
	if row.Proposal != ProposalReview || row.Decision != DecisionPending || row.LastError != CodeDirectoryChanged ||
		row.ReviewedBy != "alice" || row.LinkStatus != LinkPending {
		t.Fatalf("row = %+v", row)
	}
	if f.app.linkCalls != 0 || len(f.dir.grants) != 0 || result.Rows[0].Outcome != "review" || result.Status != RunCompleted {
		t.Fatalf("result = %+v, links %d", result, f.app.linkCalls)
	}
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", ""); !errors.Is(err, ErrRowNotApprovable) {
		t.Fatalf("a row sent back to review was re-approved: %v", err)
	}
}

func TestBatchApplyAccountChecks(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse", "b-create"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	// a-reuse left the eligible list; b-create's email changed in the application.
	f.app.accounts = []identitysync.Account{{ID: "b-create", Email: "changed@example.org"}}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	rows := f.rows(t, run.ID)
	if r := rows["a-reuse"]; r.LinkStatus != LinkIneligible || r.LastError != CodeAccountIneligible {
		t.Errorf("missing account row = %+v", r)
	}
	if r := rows["b-create"]; r.Proposal != ProposalReview || r.LastError != CodeAccountChanged || r.UserStatus != UserPending {
		t.Errorf("changed account row = %+v", r)
	}
	if f.dir.ensures != 0 || f.app.linkCalls != 0 {
		t.Fatal("no invitation or link may happen for a changed or missing account")
	}
}

func TestBatchResumeRetriesOnlyTheGrant(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	f.dir.grantErr = []error{errors.New("HTTP 503 <html>upstream body</html>")}
	result, err := f.svc.Apply(ctx, f.target, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	row := f.rows(t, run.ID)["a-reuse"]
	if result.Status != RunFailed || row.LinkStatus != LinkLinked || row.GrantStatus != GrantFailed || row.LastError != CodeGrantFailed {
		t.Fatalf("after failed grant: result %+v, row %+v", result, row)
	}
	links, finds, lists := f.app.linkCalls, f.dir.findCalls, f.app.listCalls
	// Even if the directory now disagrees, resume does not re-link or re-check.
	f.dir.byEmail["reuse@example.org"] = nil
	result, err = f.svc.Resume(ctx, f.target, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	row = f.rows(t, run.ID)["a-reuse"]
	if result.Status != RunCompleted || !row.Finished() || row.Attempts != 2 || row.LastError != "" {
		t.Fatalf("after resume: result %+v, row %+v", result, row)
	}
	if f.app.linkCalls != links || f.dir.findCalls != finds || f.app.listCalls != lists {
		t.Fatalf("resume re-linked or re-read: links %d->%d, finds %d->%d, lists %d->%d",
			links, f.app.linkCalls, finds, f.dir.findCalls, lists, f.app.listCalls)
	}
	if strings.Join(f.dir.grants, ",") != "subj-reuse" {
		t.Fatalf("grants = %v", f.dir.grants)
	}
}

func TestBatchResumeAfterLinkAndInviteFailures(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse", "b-create"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	f.app.linkErr["a-reuse"] = []error{errors.New("giving up after 5 attempts: HTTP 503")}
	f.dir.ensureErr = []error{ErrInviteEmailFailed}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	rows := f.rows(t, run.ID)
	if r := rows["a-reuse"]; r.LinkStatus != LinkFailed || r.LastError != CodeLinkFailed || r.GrantStatus != GrantPending {
		t.Errorf("link failure row = %+v", r)
	}
	if r := rows["b-create"]; r.UserStatus != UserFailed || r.LastError != CodeInviteEmailFailed || r.LinkStatus != LinkPending {
		t.Errorf("invite failure row = %+v", r)
	}
	// Apply does not pick up attempted rows; resume does.
	if result, err := f.svc.Apply(ctx, f.target, run.ID); err != nil || len(result.Rows) != 0 {
		t.Fatalf("apply re-processed attempted rows: %+v, %v", result, err)
	}
	result, err := f.svc.Resume(ctx, f.target, run.ID)
	if err != nil || result.Status != RunCompleted {
		t.Fatalf("resume = %+v, %v", result, err)
	}
	rows = f.rows(t, run.ID)
	if !rows["a-reuse"].Finished() || !rows["b-create"].Finished() || rows["b-create"].UserStatus != UserInvited {
		t.Fatalf("rows after resume = %+v", rows)
	}
	if f.dir.ensures != 2 || len(f.dir.created) != 1 {
		t.Fatalf("ensure calls %d, users %d", f.dir.ensures, len(f.dir.created))
	}
}

func TestBatchNeverGrantsLinkedNoAccessOrRevoked(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	// Even if these rows were marked approved behind the service's back,
	// apply and resume skip them.
	for _, id := range []string{"g-noaccess", "h-revoked-linked", "i-revoked-match", "f-linked"} {
		f.repo.force(t, run.ID, id, func(r *BatchRow) {
			now := time.Now()
			r.Decision, r.ReviewedBy, r.ReviewedAt = DecisionApproved, "mallory", &now
		})
	}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	f.repo.force(t, run.ID, "g-noaccess", func(r *BatchRow) { r.Attempts = 1; r.GrantStatus = GrantFailed })
	if _, err := f.svc.Resume(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.dir.grants) != 0 || f.app.linkCalls != 0 {
		t.Fatalf("grants %v, links %d", f.dir.grants, f.app.linkCalls)
	}

	// Revoked between approval and apply: neither linked nor granted.
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse", "b-create"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	f.revs.revoke("subj-reuse")
	f.dir.grantErr = []error{errors.New("grant failed")}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	rows := f.rows(t, run.ID)
	if r := rows["a-reuse"]; r.Proposal != ProposalRevoked || r.GrantStatus != GrantNotRequired || r.LinkStatus != LinkPending || r.LastError != CodeIdentityRevoked {
		t.Fatalf("revoked before apply = %+v", r)
	}
	// b-create linked, then its grant failed; it is revoked before resume.
	if r := rows["b-create"]; r.LinkStatus != LinkLinked || r.GrantStatus != GrantFailed {
		t.Fatalf("b-create = %+v", r)
	}
	f.revs.revoke("new-b-create")
	if _, err := f.svc.Resume(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	if r := f.rows(t, run.ID)["b-create"]; r.Proposal != ProposalRevoked || r.GrantStatus != GrantNotRequired || r.LastError != CodeIdentityRevoked {
		t.Fatalf("revoked before resume = %+v", r)
	}
	if len(f.dir.grants) != 0 {
		t.Fatalf("revoked identities were granted: %v", f.dir.grants)
	}
	// A failing revocation check also blocks the grant.
	f2 := newBatchFixture()
	run2, _ := f2.start(t)
	if _, err := f2.svc.ApproveRows(ctx, run2.ID, []string{"a-reuse"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	f2.revs.err = errors.New("db down")
	if _, err := f2.svc.Apply(ctx, f2.target, run2.ID); err != nil {
		t.Fatal(err)
	}
	if r := f2.rows(t, run2.ID)["a-reuse"]; r.LastError != CodeRevocationCheckFailed || r.LinkStatus != LinkPending || len(f2.dir.grants) != 0 {
		t.Fatalf("failed revocation check = %+v", r)
	}
}

func TestBatchLinkConflictAndIneligibleAreFinal(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	f.app.accounts = append(f.app.accounts, identitysync.Account{ID: "k-gone", Email: "gone@example.org"})
	f.dir.byEmail["gone@example.org"] = []identitysync.Identity{verified("subj-gone")}
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse", "k-gone"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	f.app.linkErr["a-reuse"] = []error{identitysync.ErrLinkConflict}
	f.app.linkErr["k-gone"] = []error{identitysync.ErrAccountNotFound}
	result, err := f.svc.Apply(ctx, f.target, run.ID)
	if err != nil || result.Status != RunCompleted {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	rows := f.rows(t, run.ID)
	if rows["a-reuse"].LinkStatus != LinkConflict || rows["a-reuse"].LastError != CodeLinkConflict {
		t.Errorf("conflict row = %+v", rows["a-reuse"])
	}
	if rows["k-gone"].LinkStatus != LinkIneligible {
		t.Errorf("ineligible row = %+v", rows["k-gone"])
	}
	calls := f.app.linkCalls
	if result, err := f.svc.Resume(ctx, f.target, run.ID); err != nil || len(result.Rows) != 0 || f.app.linkCalls != calls {
		t.Fatalf("resume retried final rows: %+v, %v", result, err)
	}
	if len(f.dir.grants) != 0 {
		t.Fatalf("grants = %v", f.dir.grants)
	}
}

func TestBatchAlreadyLinkedAccountSkipsPut(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse", "b-create"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	// An earlier attempt linked a-reuse before crashing; the application now
	// reports that exact link, and another subject on b-create.
	f.app.accounts[0].Link = verified("subj-reuse")
	f.app.accounts[1].Link = verified("intruder")
	f.app.accounts = f.app.accounts[:2]
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	rows := f.rows(t, run.ID)
	if !rows["a-reuse"].Finished() || rows["b-create"].LinkStatus != LinkConflict || f.app.linkCalls != 0 {
		t.Fatalf("rows = %+v, links %d", rows, f.app.linkCalls)
	}
}

func TestBatchRunClaimAndMismatch(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	run, _ := f.start(t)
	other := f.target
	other.Issuer = "https://other.example.test/realms/moh"
	if _, err := f.svc.Apply(ctx, other, run.ID); !errors.Is(err, ErrRunMismatch) {
		t.Fatalf("issuer mismatch: %v", err)
	}
	if _, err := f.repo.ClaimRun(ctx, run.ID, []RunStatus{RunPreviewed}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); !errors.Is(err, ErrRunBusy) {
		t.Fatalf("concurrent apply: %v", err)
	}
	// A stale "applying" run (crashed process) can be taken over.
	f.svc.StaleAfter = time.Nanosecond
	time.Sleep(time.Millisecond)
	if result, err := f.svc.Apply(ctx, f.target, run.ID); err != nil || result.Status != RunCompleted {
		t.Fatalf("takeover = %+v, %v", result, err)
	}
	// A failing account walk restores the previous status.
	f.svc.StaleAfter = 0
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	f.target.Accounts = failingAccounts{}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err == nil {
		t.Fatal("expected the account walk error")
	}
	if got, _ := f.repo.GetRun(ctx, run.ID); got.Status != RunCompleted {
		t.Fatalf("status after aborted apply = %s", got.Status)
	}
	if r := f.rows(t, run.ID)["a-reuse"]; r.Attempts != 0 {
		t.Fatalf("aborted apply counted an attempt: %+v", r)
	}
}

type failingAccounts struct{}

func (failingAccounts) ListEligibleAccounts(context.Context, string, int) (identitysync.AccountPage, error) {
	return identitysync.AccountPage{}, errors.New("unavailable")
}

func (failingAccounts) LinkIdentity(context.Context, string, identitysync.Identity) error {
	return errors.New("unavailable")
}

func TestNoRevocationsDefault(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	f.svc.Revocations = nil
	_, rows := f.start(t)
	if rows["h-revoked-linked"].Proposal != ProposalLinkedNoAccess || rows["i-revoked-match"].Proposal != ProposalReuse {
		t.Fatalf("nil checker must report nothing revoked: %+v", rows)
	}
	if revoked, err := (NoRevocations{}).IsRevoked(ctx, "a", "b", "c"); revoked || err != nil {
		t.Fatal("NoRevocations reported a revocation")
	}
}
