package identityreconcile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

type sessionRevoker = identitysync.SessionRevoker

type unsupportedSessions struct{}

func (unsupportedSessions) RevokeSessions(context.Context, identitysync.Identity) error {
	return identitysync.ErrUnsupported
}

func identity11() identitysync.Identity {
	return identitysync.Identity{Issuer: testIssuer, Subject: testUUID}
}

// newTestConnectorVersion is newTestConnector with the registry's stored contract version.
func newTestConnectorVersion(t *testing.T, app, stored string, handler http.HandlerFunc) (*HTTPAccounts, *recordedSleeps) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	connector, err := NewHTTPAccounts(HTTPAccountsConfig{
		Application: app, Origin: server.URL, LegacyAccountIDFormat: identitysync.AccountIDPositiveInteger,
		StoredContractVersion: stored,
		Token:                 func(context.Context) (string, error) { return "fixture-token", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	connector.client.Transport = server.Client().Transport
	sleeps := &recordedSleeps{}
	connector.sleep = sleeps.sleep
	connector.jitter = func() float64 { return 0 }
	return connector, sleeps
}

// A registered 1.1 application that answers one GET / with 404 (a proxy
// misroute, a half-finished deploy) must not be treated as 1.0: its sessions
// step must stay retryable, and only an explicit re-register may downgrade.

func TestRevoke11ApplicationAnswering404IsRetriedNotUnsupported(t *testing.T) {
	f := newRevokeFixture(t)
	row := f.registry.rows["alerts"]
	row.ContractVersion = "1.1"
	f.registry.rows["alerts"] = row
	f.app.set(func(a *fakeApp) { a.legacy = true }) // GET / answers 404 this once
	ctx := context.Background()
	revocation, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops@example.test", "left")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.SessionsRevoked != StepFailed || revocation.Complete() {
		t.Fatalf("a 404 from GET / of a registered 1.1 application was taken as 1.0: %+v", revocation)
	}
	if revocation.RoleRemoved != StepDone || revocation.ConsentsRevoked != StepDone {
		t.Fatalf("steps = %+v", revocation)
	}
	f.app.set(func(a *fakeApp) { a.legacy = false })
	resumed, err := f.service.Resume(ctx, "alerts", "cli:ops")
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != 1 || resumed[0].SessionsRevoked != StepDone || !resumed[0].Complete() {
		t.Fatalf("resume = %+v", resumed)
	}
	if len(f.app.snapshot().revoked) != 1 {
		t.Fatal("sessions were not ended on resume")
	}
}

func TestRevokeUnsupportedIsFinalOnlyForA10Registration(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.2"} {
		f := newRevokeFixture(t)
		row := f.registry.rows["alerts"]
		row.ContractVersion = version
		f.registry.rows["alerts"] = row
		// The connector reports unsupported (for example a descriptor without
		// sessions.revoke), but the registry never recorded 1.0.
		f.service.Sessions = func(context.Context, string) (sessionRevoker, error) {
			return unsupportedSessions{}, nil
		}
		revocation, err := f.service.Revoke(context.Background(), "alerts", revokeSubject, "ops", "left")
		if err != nil {
			t.Fatal(err)
		}
		if revocation.SessionsRevoked != StepFailed || revocation.LastError != CodeSessionsUnsupported {
			t.Errorf("stored version %q: sessions = %s (%s), want failed/%s so resume retries", version, revocation.SessionsRevoked, revocation.LastError, CodeSessionsUnsupported)
		}
	}
	f := newRevokeFixture(t)
	row := f.registry.rows["alerts"]
	row.ContractVersion = "1.0"
	f.registry.rows["alerts"] = row
	f.app.set(func(a *fakeApp) { a.legacy = true })
	revocation, err := f.service.Revoke(context.Background(), "alerts", revokeSubject, "ops", "left")
	if err != nil || revocation.SessionsRevoked != StepUnsupported {
		t.Fatalf("1.0 registration: %+v, %v", revocation, err)
	}
}

func TestDescribe404IsAnErrorWhenTheRegistryRecorded11(t *testing.T) {
	for _, stored := range []string{"1.1", "1.3"} {
		connector, _ := newTestConnectorVersion(t, "poesgo", stored, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		if _, err := connector.Describe(context.Background()); err == nil || !strings.Contains(err.Error(), "re-register") {
			t.Fatalf("stored %s: err = %v, want an error naming re-register", stored, err)
		}
		if err := connector.RevokeSessions(context.Background(), identity11()); err == nil || errors.Is(err, identitysync.ErrUnsupported) {
			t.Fatalf("stored %s: RevokeSessions err = %v, want a retryable error, not unsupported", stored, err)
		}
	}
	for _, stored := range []string{"", "1.0"} {
		connector, _ := newTestConnectorVersion(t, "poesgo", stored, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		d, err := connector.Describe(context.Background())
		if err != nil || d.Version != "1.0" {
			t.Fatalf("stored %q: descriptor = %+v, %v; want the 1.0 fallback", stored, d, err)
		}
	}
}

func TestRegistrySaveMayDowngradeA11RegistrationExplicitly(t *testing.T) {
	registry, repo, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // the application now implements 1.0 only
	})
	integration := validIntegration()
	integration.Origin = origin
	integration.ContractVersion = "1.1"
	integration.Enabled = true
	repo.rows = map[string]Integration{"alerts": integration}
	// Use time: a stored 1.1 registration refuses the 404.
	if _, _, err := registry.Connector(context.Background(), "alerts"); err != nil {
		t.Fatal(err)
	}
	connector, _, _ := registry.Connector(context.Background(), "alerts")
	if _, err := connector.Describe(context.Background()); err == nil {
		t.Fatal("use-time describe downgraded a registered 1.1 application")
	}
	// An ordinary enabled save (for example editing another field while the
	// application briefly answers 404) must not downgrade the registration.
	if _, err := registry.Save(context.Background(), integration, "operator", "edit access role"); !errors.Is(err, ErrContractDowngrade) {
		t.Fatalf("plain save = %v, want ErrContractDowngrade", err)
	}
	if got := repo.rows["alerts"].ContractVersion; got != "1.1" {
		t.Fatalf("refused save changed the stored version to %q", got)
	}
	// Only an explicit downgrade reads the descriptor afresh and records 1.0.
	integration.AllowContractDowngrade = true
	saved, err := registry.Save(context.Background(), integration, "operator", "application rolled back to contract 1.0")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContractVersion != "1.0" {
		t.Fatalf("explicit downgrade stored %q, want 1.0", saved.ContractVersion)
	}
}

// Without a stored 1.1 version (a new registration, or one already at 1.0), a
// 1.0 application is accepted without the downgrade flag.
func TestRegistrySaveAcceptsNew10RegistrationWithoutFlag(t *testing.T) {
	registry, repo, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	integration := validIntegration()
	integration.Origin = origin
	integration.Enabled = true
	repo.rows = map[string]Integration{}
	saved, err := registry.Save(context.Background(), integration, "operator", "register a 1.0 application")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContractVersion != "1.0" {
		t.Fatalf("new 1.0 registration stored %q, want 1.0", saved.ContractVersion)
	}
}
