package identityreconcile

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// Revoke-versus-grant race (finding: a revocation that runs during an apply
// could be undone by that apply's grant). These tests run a real RevokeService
// against an in-memory revocation store whose access lock behaves like
// PostgresRevocations' advisory lock.

// roleState models whether each subject holds the application's access role.
type roleState struct {
	mu   sync.Mutex
	held map[string]bool
}

func newRoleState() *roleState { return &roleState{held: map[string]bool{}} }

func (r *roleState) set(subject string, held bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.held[subject] = held
}

func (r *roleState) has(subject string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.held[subject]
}

// roleAccessDirectory is the Keycloak side of revocation over roleState.
type roleAccessDirectory struct{ roles *roleState }

func (d roleAccessDirectory) RemoveAccessRole(_ context.Context, subject, _, _ string) error {
	d.roles.set(subject, false)
	return nil
}
func (roleAccessDirectory) RevokeClientConsents(context.Context, string, string) error { return nil }
func (d roleAccessDirectory) HasAccessRole(_ context.Context, subject, _, _ string) (bool, error) {
	return d.roles.has(subject), nil
}

type staticIntegrations map[string]Integration

func (s staticIntegrations) Get(_ context.Context, application string) (Integration, error) {
	integration, ok := s[application]
	if !ok {
		return Integration{}, ErrIntegrationNotFound
	}
	return integration, nil
}

type okSessions struct{}

func (okSessions) RevokeSessions(context.Context, identitysync.Identity) error { return nil }

// lockingRevocations is memoryRevocations plus a per-(application, subject)
// mutex with the same contract as PostgresRevocations.WithAccessLock.
type lockingRevocations struct {
	memoryRevocations
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (l *lockingRevocations) WithAccessLock(ctx context.Context, application, subject string, fn func(context.Context) error) error {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = map[string]*sync.Mutex{}
	}
	key := application + "/" + subject
	lock, ok := l.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		l.locks[key] = lock
	}
	l.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	return fn(ctx)
}

func raceRevokeService(issuer string, roles *roleState, repo RevocationRepository) *RevokeService {
	return &RevokeService{
		Integrations: staticIntegrations{"alerts": {ApplicationClientID: "alerts", AccessRole: "alerts_access", Enabled: true}},
		Sessions: func(context.Context, string) (identitysync.SessionRevoker, error) {
			return okSessions{}, nil
		},
		Directory:  roleAccessDirectory{roles: roles},
		Repository: repo,
		Issuer:     issuer,
	}
}

// interleaveRevoke starts a revoke of subject from inside the grant (after the
// revocation check passed, before the role is assigned). An unserialised
// revoke finishes before the grant lands; a serialised one waits for it.
func interleaveRevoke(t *testing.T, revoker *RevokeService, subject string) (beforeGrant func(), wait func() Revocation) {
	t.Helper()
	finished := make(chan struct{})
	var once sync.Once
	var result Revocation
	var resultErr error
	beforeGrant = func() {
		once.Do(func() {
			go func() {
				defer close(finished)
				result, resultErr = revoker.Revoke(context.Background(), "alerts", subject, "ops@example.test", "left the programme")
			}()
			select {
			case <-finished:
			case <-time.After(300 * time.Millisecond):
			}
		})
	}
	wait = func() Revocation {
		t.Helper()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Fatal("the interleaved revoke never finished")
		}
		if resultErr != nil {
			t.Fatalf("interleaved revoke: %v", resultErr)
		}
		return result
	}
	return beforeGrant, wait
}

// interleavingBatchDirectory runs beforeGrant inside GrantAccess.
type interleavingBatchDirectory struct {
	*fakeBatchDirectory
	roles       *roleState
	beforeGrant func()
}

func (d *interleavingBatchDirectory) GrantAccess(ctx context.Context, identity identitysync.Identity) error {
	if d.beforeGrant != nil {
		d.beforeGrant()
	}
	if err := d.fakeBatchDirectory.GrantAccess(ctx, identity); err != nil {
		return err
	}
	d.roles.set(identity.Subject, true)
	return nil
}

func TestBatchGrantIsSerialisedWithRevocation(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	roles := newRoleState()
	revoker := raceRevokeService(batchIssuer, roles, &lockingRevocations{})
	f.svc.Revocations = revoker
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	beforeGrant, wait := interleaveRevoke(t, revoker, "subj-reuse")
	f.target.Directory = &interleavingBatchDirectory{fakeBatchDirectory: f.dir, roles: roles, beforeGrant: beforeGrant}
	if _, err := f.svc.Apply(ctx, f.target, run.ID); err != nil {
		t.Fatal(err)
	}
	revocation := wait()
	if roles.has("subj-reuse") {
		t.Fatal("the apply's grant undid a revocation that ran during the apply: the person still holds the access role")
	}
	if !revocation.Complete() || revocation.RoleRemoved != StepDone {
		t.Fatalf("revocation = %+v", revocation)
	}
	if active, err := revoker.IsRevoked(ctx, "alerts", batchIssuer, "subj-reuse"); err != nil || !active {
		t.Fatalf("revocation not active: %v, %v", active, err)
	}
}

// interleavingServiceDirectory is the single-apply directory with a hook in GrantAccess.
type interleavingServiceDirectory struct {
	identity    identitysync.Identity
	roles       *roleState
	beforeGrant func()
}

func (d *interleavingServiceDirectory) FindEmail(context.Context, string) ([]identitysync.Identity, error) {
	return []identitysync.Identity{d.identity}, nil
}
func (d *interleavingServiceDirectory) EnsureUser(context.Context, identitysync.Account) (identitysync.Identity, error) {
	return d.identity, nil
}
func (d *interleavingServiceDirectory) GrantAccess(_ context.Context, identity identitysync.Identity) error {
	if d.beforeGrant != nil {
		d.beforeGrant()
	}
	d.roles.set(identity.Subject, true)
	return nil
}

type linkRecorder struct {
	account identitysync.Account
	links   int
}

func (a *linkRecorder) ListEligibleAccounts(context.Context, string, int) (identitysync.AccountPage, error) {
	return identitysync.AccountPage{Accounts: []identitysync.Account{a.account}}, nil
}
func (a *linkRecorder) LinkIdentity(context.Context, string, identitysync.Identity) error {
	a.links++
	return nil
}

func TestSingleApplyGrantIsSerialisedWithRevocation(t *testing.T) {
	ctx := context.Background()
	identity := identitysync.Identity{Issuer: batchIssuer, Subject: "subj-single"}
	roles := newRoleState()
	revoker := raceRevokeService(batchIssuer, roles, &lockingRevocations{})
	beforeGrant, wait := interleaveRevoke(t, revoker, identity.Subject)
	accounts := &linkRecorder{account: identitysync.Account{ID: "acct-1", Email: "single@example.org"}}
	service := Service{
		Accounts:    accounts,
		Directory:   &interleavingServiceDirectory{identity: identity, roles: roles, beforeGrant: beforeGrant},
		Issuer:      batchIssuer,
		Application: "alerts",
		Revocations: revoker,
	}
	decision := Decision{Account: accounts.account, Action: Reuse, Identity: identity}
	_ = service.Apply(ctx, decision, accounts.account) // either outcome is fine; the end state is what matters
	wait()
	if roles.has(identity.Subject) {
		t.Fatal("single apply's grant undid a revocation that ran during the apply")
	}
}

// bypassRevocations flips to revoked during the grant without taking the access
// lock (a revocation written by a path that does not serialise). The re-check
// after the grant must catch it and remove the role again.
func TestBatchRecheckAfterGrantRemovesRoleForLateRevocation(t *testing.T) {
	ctx := context.Background()
	f := newBatchFixture()
	roles := newRoleState()
	repo := &memoryRevocations{} // no access lock at all
	revoker := raceRevokeService(batchIssuer, roles, repo)
	f.svc.Revocations = revoker
	run, _ := f.start(t)
	if _, err := f.svc.ApproveRows(ctx, run.ID, []string{"a-reuse"}, "alice", ""); err != nil {
		t.Fatal(err)
	}
	// Record the revocation directly in the store (no role removal, no lock),
	// as if written between the check and the grant.
	f.target.Directory = &interleavingBatchDirectory{fakeBatchDirectory: f.dir, roles: roles, beforeGrant: func() {
		if _, _, err := repo.BeginRevocation(ctx, Revocation{ApplicationClientID: "alerts", Issuer: batchIssuer, Subject: "subj-reuse",
			RevokedBy: "ops", Reason: "late", RoleRemoved: StepDone, ConsentsRevoked: StepDone, SessionsRevoked: StepDone}); err != nil {
			t.Error(err)
		}
	}}
	result, err := f.svc.Apply(ctx, f.target, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if roles.has("subj-reuse") {
		t.Fatal("a revocation recorded during the grant left the access role in place")
	}
	row := f.rows(t, run.ID)["a-reuse"]
	if row.Proposal != ProposalRevoked || row.GrantStatus != GrantNotRequired || row.LastError != CodeIdentityRevoked {
		t.Fatalf("row = %+v", row)
	}
	if len(result.Rows) != 1 || result.Rows[0].Outcome != "revoked" {
		t.Fatalf("result = %+v", result)
	}
	active, err := repo.ActiveRevocation(ctx, "alerts", batchIssuer, "subj-reuse")
	if err != nil || active.RoleRemoved != StepDone || active.Attempts == 0 {
		t.Fatalf("revocation after re-assert = %+v, %v", active, err)
	}
}

func TestEnforceRevocationLeavesRolePendingWithoutDirectory(t *testing.T) {
	ctx := context.Background()
	repo := &memoryRevocations{}
	if _, _, err := repo.BeginRevocation(ctx, Revocation{ApplicationClientID: "alerts", Issuer: batchIssuer, Subject: "s1", RevokedBy: "ops", Reason: "r"}); err != nil {
		t.Fatal(err)
	}
	rev, _ := repo.ActiveRevocation(ctx, "alerts", batchIssuer, "s1")
	rev.RoleRemoved, rev.ConsentsRevoked, rev.SessionsRevoked = StepDone, StepDone, StepDone
	if err := repo.SaveRevocationAttempt(ctx, rev, "ops"); err != nil {
		t.Fatal(err)
	}
	// A checker-only service (no directory) can still record that the role must be removed again.
	service := &RevokeService{Repository: repo}
	enforced, err := service.EnforceRevocation(ctx, "alerts", batchIssuer, "s1", "identity-sync apply")
	if !enforced || err == nil {
		t.Fatalf("enforced = %v, err = %v; want true with an error saying removal is pending", enforced, err)
	}
	after, _ := repo.ActiveRevocation(ctx, "alerts", batchIssuer, "s1")
	if after.RoleRemoved != StepPending {
		t.Fatalf("role_removed = %s, want pending so resume removes it", after.RoleRemoved)
	}
	if enforced, err := service.EnforceRevocation(ctx, "alerts", batchIssuer, "nobody", "x"); enforced || err != nil {
		t.Fatalf("no revocation: enforced = %v, err = %v", enforced, err)
	}
}
