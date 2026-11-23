package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

const runTestIssuer = "https://sso.example.test/realms/moh"

type runTestApp struct {
	accounts []identitysync.Account
	links    map[string]string
}

func (a *runTestApp) ListEligibleAccounts(context.Context, string, int) (identitysync.AccountPage, error) {
	return identitysync.AccountPage{Accounts: a.accounts}, nil
}

func (a *runTestApp) LinkIdentity(_ context.Context, id string, identity identitysync.Identity) error {
	a.links[id] = identity.Subject
	return nil
}

type runTestDirectory struct {
	grantErrs int
	grants    []string
}

func (d *runTestDirectory) FindEmail(_ context.Context, email string) ([]identitysync.Identity, error) {
	if email == "reuse@example.org" {
		return []identitysync.Identity{{Issuer: runTestIssuer, Subject: "subj-reuse"}}, nil
	}
	if email == "twice@example.org" {
		return []identitysync.Identity{{}, {}}, nil
	}
	return nil, nil
}

func (d *runTestDirectory) EnsureUser(context.Context, identitysync.Account) (identitysync.Identity, error) {
	return identitysync.Identity{}, errors.New("not used")
}

func (d *runTestDirectory) GrantAccess(_ context.Context, identity identitysync.Identity) error {
	if d.grantErrs > 0 {
		d.grantErrs--
		return errors.New("keycloak unavailable")
	}
	d.grants = append(d.grants, identity.Subject)
	return nil
}

func (d *runTestDirectory) HasAccess(context.Context, identitysync.Identity) (bool, error) {
	return false, nil
}

type runTestRevocations struct{ asked int }

func (r *runTestRevocations) IsRevoked(context.Context, string, string, string) (bool, error) {
	r.asked++
	return false, nil
}

func installRunTestDoubles(t *testing.T) (*runTestApp, *runTestDirectory, *runTestRevocations) {
	t.Helper()
	repo := identityreconcile.NewMemoryBatchRepository()
	app := &runTestApp{links: map[string]string{}, accounts: []identitysync.Account{
		{ID: "acct-1", Email: "reuse@example.org"},
		{ID: "acct-2", Email: "twice@example.org"},
	}}
	dir := &runTestDirectory{}
	revs := &runTestRevocations{}
	prevRepo, prevRuntime, prevTarget, prevRevs := openIdentitySyncBatchRepository, openIdentitySyncRuntime, identitySyncBatchTarget, identitySyncRevocations
	t.Cleanup(func() {
		openIdentitySyncBatchRepository, openIdentitySyncRuntime, identitySyncBatchTarget, identitySyncRevocations = prevRepo, prevRuntime, prevTarget, prevRevs
	})
	openIdentitySyncBatchRepository = func(context.Context) (identityreconcile.BatchRepository, func(), error) {
		return repo, func() {}, nil
	}
	openIdentitySyncRuntime = func(context.Context) (*identitySyncRuntime, error) { return &identitySyncRuntime{}, nil }
	identitySyncRevocations = func(context.Context, *identitySyncRuntime) (identityreconcile.RevocationChecker, error) {
		return revs, nil
	}
	identitySyncBatchTarget = func(_ context.Context, _ *identitySyncRuntime, name string) (identityreconcile.BatchTarget, error) {
		if name != "alerts" {
			return identityreconcile.BatchTarget{}, identityreconcile.ErrIntegrationNotFound
		}
		return identityreconcile.BatchTarget{Application: "alerts", Issuer: runTestIssuer, Accounts: app, Directory: dir}, nil
	}
	return app, dir, revs
}

func TestIdentitySyncRunIsAttachedToTheParent(t *testing.T) {
	cmd := newIdentitySyncCommand()
	found, _, err := cmd.Find([]string{"run", "resume"})
	if err != nil || found.Name() != "resume" {
		t.Fatalf("identity-sync run resume not found: %v", err)
	}
	run, _, _ := cmd.Find([]string{"run"})
	names := map[string]bool{}
	for _, sub := range run.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"start", "show", "approve", "reject", "apply", "resume"} {
		if !names[want] {
			t.Errorf("missing run %s", want)
		}
	}
}

func TestIdentitySyncRunCommandsReviewApplyAndResume(t *testing.T) {
	app, dir, revs := installRunTestDoubles(t)
	out, _, err := runCommand(t, newIdentitySyncCommand(), "run", "start", "--app", "alerts")
	if err != nil {
		t.Fatal(err)
	}
	var started runSummaryView
	if err := json.Unmarshal([]byte(out), &started); err != nil {
		t.Fatalf("start output %q: %v", out, err)
	}
	runID := started.Run.ID
	if started.Counts["total"] != 2 || started.Counts["proposal:reuse"] != 1 || started.Counts["proposal:review"] != 1 || revs.asked == 0 {
		t.Fatalf("start summary = %+v (revocation checks %d)", started, revs.asked)
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "start", "--app", "unknown"); err == nil {
		t.Fatal("start for an unregistered application succeeded")
	}

	// A reviewer is required, and review rows cannot be approved.
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "approve", "--run", runID, "--account", "acct-1"); err == nil {
		t.Fatal("approve without --reviewer succeeded")
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "approve", "--run", runID, "--account", "acct-1", "--reviewer", "  "); err == nil {
		t.Fatal("approve with a blank reviewer succeeded")
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "approve", "--run", runID, "--account", "acct-2", "--reviewer", "alice"); !errors.Is(err, identityreconcile.ErrRowNotApprovable) {
		t.Fatalf("approve review row: %v", err)
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "approve", "--run", runID, "--account", "acct-1", "--reviewer", "alice", "--note", "HR confirmed"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "reject", "--run", runID, "--account", "acct-2", "--reviewer", "bob"); err != nil {
		t.Fatal(err)
	}

	out, _, err = runCommand(t, newIdentitySyncCommand(), "run", "show", "--run", runID, "--decision", "approved")
	if err != nil {
		t.Fatal(err)
	}
	var shown struct {
		Rows []identityreconcile.BatchRow `json:"rows"`
	}
	if err := json.Unmarshal([]byte(out), &shown); err != nil || len(shown.Rows) != 1 || shown.Rows[0].ReviewedBy != "alice" || shown.Rows[0].ReviewNote != "HR confirmed" {
		t.Fatalf("show = %s, %v", out, err)
	}

	// The grant fails once: apply reports it, resume finishes it without re-linking.
	dir.grantErrs = 1
	out, _, err = runCommand(t, newIdentitySyncCommand(), "run", "apply", "--run", runID)
	if err == nil || !strings.Contains(err.Error(), "resume") || !strings.Contains(out, `"grant_failed"`) {
		t.Fatalf("apply = %q, %v", out, err)
	}
	if app.links["acct-1"] != "subj-reuse" || len(app.links) != 1 {
		t.Fatalf("links = %v", app.links)
	}
	app.links = map[string]string{}
	out, _, err = runCommand(t, newIdentitySyncCommand(), "run", "resume", "--run", runID)
	if err != nil || !strings.Contains(out, `"completed"`) {
		t.Fatalf("resume = %q, %v", out, err)
	}
	if len(app.links) != 0 || strings.Join(dir.grants, ",") != "subj-reuse" {
		t.Fatalf("resume re-linked (%v) or granted wrongly (%v)", app.links, dir.grants)
	}

	out, _, err = runCommand(t, newIdentitySyncCommand(), "run", "show", "--app", "alerts")
	if err != nil || !strings.Contains(out, runID) {
		t.Fatalf("list runs = %q, %v", out, err)
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "show"); err == nil {
		t.Fatal("show without --run or --app succeeded")
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "run", "apply", "--run", "not-a-run"); !errors.Is(err, identityreconcile.ErrRunNotFound) {
		t.Fatalf("apply unknown run: %v", err)
	}
}

func TestIdentitySyncBatchTargetUsesTheRegistry(t *testing.T) {
	paused := alertsIntegration(false)
	paused.ApplicationClientID = "paused"
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"alerts": alertsIntegration(true), "paused": paused})
	rt, err := openIdentitySyncRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	target, err := identitySyncBatchTarget(context.Background(), rt, "alerts")
	if err != nil {
		t.Fatal(err)
	}
	directory, ok := target.Directory.(identityreconcile.KeycloakDirectory)
	if !ok || target.Application != "alerts" || target.Issuer != "https://sso.example.test/realms/moh" || target.Accounts == nil ||
		directory.AccessRole != "alerts_access" || directory.Application != "alerts" || directory.Issuer != target.Issuer {
		t.Fatalf("target = %+v", target)
	}
	if _, err := identitySyncBatchTarget(context.Background(), rt, "paused"); err == nil {
		t.Fatal("built a batch target for a disabled integration")
	}
}
