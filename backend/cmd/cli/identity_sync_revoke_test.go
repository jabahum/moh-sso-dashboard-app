package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

const cliRevokeSubject = "5f2e0a8c-6a43-4c1e-9f0e-2b8d6f3c1a77"

type cliRevocations struct {
	rows []identityreconcile.Revocation
}

func (m *cliRevocations) find(app, issuer, subject string) int {
	for i, r := range m.rows {
		if r.Active() && r.ApplicationClientID == app && r.Issuer == issuer && r.Subject == subject {
			return i
		}
	}
	return -1
}

func (m *cliRevocations) BeginRevocation(_ context.Context, r identityreconcile.Revocation) (identityreconcile.Revocation, bool, error) {
	if i := m.find(r.ApplicationClientID, r.Issuer, r.Subject); i >= 0 {
		return m.rows[i], false, nil
	}
	r.ID = "rev-" + r.Subject[:8]
	m.rows = append(m.rows, r)
	return r, true, nil
}

func (m *cliRevocations) ActiveRevocation(_ context.Context, app, issuer, subject string) (identityreconcile.Revocation, error) {
	if i := m.find(app, issuer, subject); i >= 0 {
		return m.rows[i], nil
	}
	return identityreconcile.Revocation{}, identityreconcile.ErrRevocationNotFound
}

func (m *cliRevocations) SaveRevocationAttempt(_ context.Context, r identityreconcile.Revocation, _ string) error {
	for i := range m.rows {
		if m.rows[i].ID == r.ID && m.rows[i].Active() {
			m.rows[i] = r
			return nil
		}
	}
	return identityreconcile.ErrRevocationNotFound
}

func (m *cliRevocations) RestoreRevocation(_ context.Context, app, issuer, subject, actor, reason string) (identityreconcile.Revocation, error) {
	i := m.find(app, issuer, subject)
	if i < 0 {
		return identityreconcile.Revocation{}, identityreconcile.ErrRevocationNotFound
	}
	now := time.Now()
	m.rows[i].RestoredAt, m.rows[i].RestoredBy, m.rows[i].RestoreReason = &now, actor, reason
	return m.rows[i], nil
}

func (m *cliRevocations) ListRevocations(_ context.Context, f identityreconcile.RevocationFilter) ([]identityreconcile.Revocation, error) {
	var out []identityreconcile.Revocation
	for _, r := range m.rows {
		if (f.Application == "" || r.ApplicationClientID == f.Application) && (r.Active() || f.IncludeRestored) && (!f.OnlyUnfinished || !r.Complete()) {
			out = append(out, r)
		}
	}
	return out, nil
}

type cliDirectory struct{ removed, consents int }

func (d *cliDirectory) RemoveAccessRole(context.Context, string, string, string) error {
	d.removed++
	return nil
}
func (d *cliDirectory) RevokeClientConsents(context.Context, string, string) error {
	d.consents++
	return nil
}
func (d *cliDirectory) HasAccessRole(context.Context, string, string, string) (bool, error) {
	return false, nil
}

type cliSessions struct {
	err   error
	calls []identitysync.Identity
}

func (s *cliSessions) RevokeSessions(_ context.Context, identity identitysync.Identity) error {
	s.calls = append(s.calls, identity)
	return s.err
}

func installRevokeService(t *testing.T, sessions *cliSessions) (*cliRevocations, *cliDirectory) {
	t.Helper()
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"alerts": alertsIntegration(true)})
	repo, directory := &cliRevocations{}, &cliDirectory{}
	previous := openIdentitySyncRevokeService
	openIdentitySyncRevokeService = func(_ context.Context, rt *identitySyncRuntime) (*identityreconcile.RevokeService, error) {
		kc, err := rt.Keycloak()
		if err != nil {
			return nil, err
		}
		return &identityreconcile.RevokeService{
			Integrations: rt.Registry,
			Sessions: func(context.Context, string) (identitysync.SessionRevoker, error) {
				return sessions, nil
			},
			Directory: directory, Repository: repo, Issuer: kc.issuer,
		}, nil
	}
	t.Cleanup(func() { openIdentitySyncRevokeService = previous })
	return repo, directory
}

func TestIdentitySyncRevokeSubcommandsAreRegistered(t *testing.T) {
	cmd := newIdentitySyncCommand()
	for _, name := range []string{"revoke", "restore", "revocations"} {
		sub, _, err := cmd.Find([]string{name})
		if err != nil || sub.Name() != name {
			t.Errorf("identity-sync %s not registered: %v", name, err)
		}
	}
	revoke, _, _ := cmd.Find([]string{"revoke"})
	for _, flag := range []string{"app", "subject", "reason", "revoker"} {
		if revoke.Flags().Lookup(flag) == nil {
			t.Errorf("revoke lacks --%s", flag)
		}
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "revoke", "--app", "alerts", "--subject", cliRevokeSubject, "--reason", "left"); err == nil {
		t.Error("revoke ran without --revoker")
	}
}

func TestIdentitySyncRevokeCommandSucceedsAndEndsSessions(t *testing.T) {
	sessions := &cliSessions{}
	repo, directory := installRevokeService(t, sessions)
	stdout, stderr, err := runCommand(t, newIdentitySyncCommand(), "revoke", "--app", "alerts", "--subject", cliRevokeSubject,
		"--reason", "left the programme", "--revoker", "ops@example.test")
	if err != nil {
		t.Fatalf("revoke: %v\n%s", err, stderr)
	}
	var view revocationView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("output %q: %v", stdout, err)
	}
	if !view.Complete || view.SessionsStillOpen || view.SessionsRevoked != identityreconcile.StepDone || view.Issuer != "https://sso.example.test/realms/moh" {
		t.Fatalf("view = %+v", view)
	}
	if directory.removed != 1 || directory.consents != 1 || len(sessions.calls) != 1 || len(repo.rows) != 1 {
		t.Fatalf("steps not run: %+v %+v %+v", directory, sessions.calls, repo.rows)
	}
	if !strings.Contains(stderr, "sessions done") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestIdentitySyncRevokeCommandFailsOnOutageAndResumeCompletes(t *testing.T) {
	sessions := &cliSessions{err: context.DeadlineExceeded}
	repo, _ := installRevokeService(t, sessions)
	_, stderr, err := runCommand(t, newIdentitySyncCommand(), "revoke", "--app", "alerts", "--subject", cliRevokeSubject,
		"--reason", "left", "--revoker", "ops")
	if err == nil || !strings.Contains(err.Error(), "incomplete") || !strings.Contains(err.Error(), "--resume") {
		t.Fatalf("revoke during outage: err = %v\n%s", err, stderr)
	}
	if repo.rows[0].RoleRemoved != identityreconcile.StepDone || repo.rows[0].SessionsRevoked != identityreconcile.StepFailed {
		t.Fatalf("row = %+v", repo.rows[0])
	}

	// The list shows the failed step.
	stdout, _, err := runCommand(t, newIdentitySyncCommand(), "revocations", "--app", "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"sessionsRevoked": "failed"`) || !strings.Contains(stdout, identityreconcile.CodeSessionsAppUnavailable) {
		t.Fatalf("list = %s", stdout)
	}

	sessions.err = nil
	stdout, stderr, err = runCommand(t, newIdentitySyncCommand(), "revocations", "--resume", "--app", "alerts", "--actor", "cli:ops")
	if err != nil {
		t.Fatalf("resume: %v\n%s", err, stderr)
	}
	if !repo.rows[0].Complete() || repo.rows[0].Attempts != 2 || !strings.Contains(stdout, `"complete": true`) {
		t.Fatalf("after resume: %+v\n%s", repo.rows[0], stdout)
	}
}

func TestIdentitySyncRevokeCommandReports10ResidualWindow(t *testing.T) {
	sessions := &cliSessions{err: identitysync.ErrUnsupported}
	installRevokeService(t, sessions)
	stdout, _, err := runCommand(t, newIdentitySyncCommand(), "revoke", "--app", "alerts", "--subject", cliRevokeSubject,
		"--reason", "left", "--revoker", "ops")
	if err == nil || !strings.Contains(err.Error(), "were NOT ended") {
		t.Fatalf("1.0 revoke must not claim success: %v", err)
	}
	if !strings.Contains(stdout, `"sessionsRevoked": "unsupported"`) || !strings.Contains(stdout, "local expiry") {
		t.Fatalf("stdout = %s", stdout)
	}
}

func TestIdentitySyncRestoreCommand(t *testing.T) {
	repo, directory := installRevokeService(t, &cliSessions{})
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "restore", "--app", "alerts", "--subject", cliRevokeSubject,
		"--reason", "x", "--restorer", "lead"); err == nil {
		t.Fatal("restore without a revocation succeeded")
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "revoke", "--app", "alerts", "--subject", cliRevokeSubject,
		"--reason", "left", "--revoker", "ops"); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runCommand(t, newIdentitySyncCommand(), "restore", "--app", "alerts", "--subject", cliRevokeSubject,
		"--reason", "returned", "--restorer", "lead")
	if err != nil {
		t.Fatal(err)
	}
	if repo.rows[0].Active() || !strings.Contains(stderr, "not re-granted") || !strings.Contains(stdout, `"restoredBy": "lead"`) {
		t.Fatalf("restore: %+v\n%s\n%s", repo.rows[0], stdout, stderr)
	}
	if directory.removed != 1 {
		t.Fatalf("restore touched Keycloak: %+v", directory)
	}
	stdout, _, err = runCommand(t, newIdentitySyncCommand(), "revocations", "--all")
	if err != nil || !strings.Contains(stdout, "restored:") {
		t.Fatalf("history: %v %s", err, stdout)
	}
}
