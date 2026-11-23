package identityreconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

const (
	revokeIssuer  = "https://sso.example.test/realms/moh"
	revokeSubject = "5f2e0a8c-6a43-4c1e-9f0e-2b8d6f3c1a77"
)

// ---------------------------------------------------------------------------
// In-memory revocation repository (same semantics as PostgresRevocations)
// ---------------------------------------------------------------------------

type memoryRevocations struct {
	mu    sync.Mutex
	rows  []Revocation
	audit []string
}

func (m *memoryRevocations) BeginRevocation(_ context.Context, r Revocation) (Revocation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.rows {
		if row.Active() && row.ApplicationClientID == r.ApplicationClientID && row.Issuer == r.Issuer && row.Subject == r.Subject {
			return row, false, nil
		}
	}
	r.ID = uuid.NewString()
	r.RevokedAt = time.Now()
	r.RoleRemoved, r.ConsentsRevoked, r.SessionsRevoked = StepPending, StepPending, StepPending
	m.rows = append(m.rows, r)
	m.audit = append(m.audit, AuditRevocationRecorded)
	return r, true, nil
}

func (m *memoryRevocations) ActiveRevocation(_ context.Context, app, issuer, subject string) (Revocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.rows {
		if row.Active() && row.ApplicationClientID == app && row.Issuer == issuer && row.Subject == subject {
			return row, nil
		}
	}
	return Revocation{}, ErrRevocationNotFound
}

func (m *memoryRevocations) SaveRevocationAttempt(_ context.Context, r Revocation, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, row := range m.rows {
		if row.ID == r.ID && row.Active() {
			m.rows[i] = r
			m.audit = append(m.audit, AuditRevocationAttempted)
			return nil
		}
	}
	return ErrRevocationNotFound
}

func (m *memoryRevocations) RestoreRevocation(_ context.Context, app, issuer, subject, actor, reason string) (Revocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, row := range m.rows {
		if row.Active() && row.ApplicationClientID == app && row.Issuer == issuer && row.Subject == subject {
			now := time.Now()
			row.RestoredAt, row.RestoredBy, row.RestoreReason = &now, actor, reason
			m.rows[i] = row
			m.audit = append(m.audit, AuditRevocationRestored)
			return row, nil
		}
	}
	return Revocation{}, ErrRevocationNotFound
}

func (m *memoryRevocations) ListRevocations(_ context.Context, f RevocationFilter) ([]Revocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Revocation
	for _, row := range m.rows {
		if f.Application != "" && row.ApplicationClientID != f.Application {
			continue
		}
		if (!f.IncludeRestored || f.OnlyUnfinished) && !row.Active() {
			continue
		}
		if f.OnlyUnfinished && row.Complete() {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Fake Keycloak admin API
// ---------------------------------------------------------------------------

type revokeFakeKeycloak struct {
	t  *testing.T
	mu sync.Mutex
	// direct and inherited client roles of each user on the "alerts" client.
	direct    map[string][]string
	inherited map[string][]string
	failRoles bool
	requests  []string
}

func (k *revokeFakeKeycloak) count(prefix string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	n := 0
	for _, r := range k.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (k *revokeFakeKeycloak) roles(user string) []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]string(nil), k.direct[user]...)
}

func (k *revokeFakeKeycloak) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if r.URL.Path == "/realms/moh/protocol/openid-connect/token" {
		_, _ = io.WriteString(w, `{"access_token":"admin-token","expires_in":300}`)
		return
	}
	k.requests = append(k.requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer admin-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	const admin = "/admin/realms/moh/"
	path := strings.TrimPrefix(r.URL.Path, admin)
	parts := strings.Split(path, "/")
	switch {
	case r.Method == http.MethodGet && path == "clients":
		id := r.URL.Query().Get("clientId")
		_ = json.NewEncoder(w).Encode([]map[string]string{{"id": "uuid-" + id, "clientId": id}})
	case len(parts) >= 5 && parts[0] == "users" && parts[2] == "role-mappings" && parts[3] == "clients":
		user, client := parts[1], parts[4]
		if client != "uuid-alerts" {
			k.t.Errorf("role mapping request for another client: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, ok := k.direct[user]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch {
		case r.Method == http.MethodGet && len(parts) == 6 && parts[5] == "composite":
			writeRoles(w, append(append([]string(nil), k.direct[user]...), k.inherited[user]...))
		case r.Method == http.MethodGet && len(parts) == 5:
			writeRoles(w, k.direct[user])
		case r.Method == http.MethodDelete && len(parts) == 5:
			if k.failRoles {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var body []keycloak.ClientRoleRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			for _, role := range body {
				if role.ID != "role-"+role.Name {
					k.t.Errorf("role removed with wrong id: %+v", role)
				}
				k.direct[user] = remove(k.direct[user], role.Name)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			k.t.Errorf("unexpected role mapping request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	case r.Method == http.MethodDelete && len(parts) == 4 && parts[0] == "users" && parts[2] == "consents":
		if parts[3] != "alerts" {
			k.t.Errorf("consents revoked for another client: %s", parts[3])
		}
		if k.failRoles {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		k.t.Errorf("unexpected Keycloak admin request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeRoles(w http.ResponseWriter, names []string) {
	roles := make([]keycloak.ClientRoleRep, 0, len(names))
	for _, name := range names {
		roles = append(roles, keycloak.ClientRoleRep{ID: "role-" + name, Name: name})
	}
	_ = json.NewEncoder(w).Encode(roles)
}

func remove(values []string, value string) []string {
	out := values[:0:0]
	for _, v := range values {
		if v != value {
			out = append(out, v)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Fake application (contract 1.1 or 1.0) behind Track A's registry connector
// ---------------------------------------------------------------------------

type fakeApp struct {
	mu       sync.Mutex
	legacy   bool // 1.0: GET / returns 404
	outage   bool // POST /sessions/revoke returns 503
	rejected bool // POST /sessions/revoke returns 400
	revoked  []identitysync.Identity
	calls    int
}

func (a *fakeApp) handler(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == syncBase:
		if a.legacy {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, descriptorJSON("1.1", "alerts", "opaque", "accounts.list", "identity.link", "sessions.revoke"))
	case r.Method == http.MethodPost && r.URL.Path == syncBase+"/sessions/revoke":
		a.calls++
		if a.outage {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if a.rejected {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var body struct{ Issuer, Subject string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		a.revoked = append(a.revoked, identitysync.Identity{Issuer: body.Issuer, Subject: body.Subject})
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (a *fakeApp) set(f func(*fakeApp)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f(a)
}

type appCalls struct {
	calls   int
	revoked []identitysync.Identity
}

func (a *fakeApp) snapshot() appCalls {
	a.mu.Lock()
	defer a.mu.Unlock()
	return appCalls{calls: a.calls, revoked: append([]identitysync.Identity(nil), a.revoked...)}
}

func (k *revokeFakeKeycloak) update(f func(*revokeFakeKeycloak)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	f(k)
}

type revokeFixture struct {
	service  *RevokeService
	repo     *memoryRevocations
	registry *fakeRegistryRepo
	kc       *revokeFakeKeycloak
	app      *fakeApp
}

func newRevokeFixture(t *testing.T) *revokeFixture {
	t.Helper()
	app := &fakeApp{}
	registry, registryRepo, origin := registryFixture(t, app.handler)
	integration := validIntegration()
	integration.Origin = origin
	integration.Enabled = true
	registryRepo.rows = map[string]Integration{"alerts": integration}

	kc := &revokeFakeKeycloak{t: t,
		direct:    map[string][]string{revokeSubject: {"alerts_access", "alerts_viewer"}},
		inherited: map[string][]string{},
	}
	kcServer := httptest.NewServer(kc)
	t.Cleanup(kcServer.Close)
	admin := keycloak.NewAdminClient(kcServer.URL, "moh", "dashboard-admin", "admin-secret")

	repo := &memoryRevocations{}
	service := NewRevokeService(registry, repo, KeycloakAccessDirectory{Admin: admin}, revokeIssuer+"/")
	return &revokeFixture{service: service, repo: repo, registry: registryRepo, kc: kc, app: app}
}

// assertNoGlobalLogout checks that only per-client Keycloak endpoints were used:
// no user logout, no session deletion and no other client.
func (f *revokeFixture) assertNoGlobalLogout(t *testing.T) {
	t.Helper()
	f.kc.mu.Lock()
	defer f.kc.mu.Unlock()
	for _, r := range f.kc.requests {
		if strings.Contains(r, "/logout") || strings.Contains(r, "/sessions") || strings.Contains(r, "/offline-sessions") {
			t.Errorf("global logout or session call: %s", r)
		}
	}
}

func TestRevokeFullSuccessEndsSessionsImmediately(t *testing.T) {
	f := newRevokeFixture(t)
	ctx := context.Background()
	revocation, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops@example.test", "left the programme")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.RoleRemoved != StepDone || revocation.ConsentsRevoked != StepDone || revocation.SessionsRevoked != StepDone {
		t.Fatalf("steps = %+v", revocation)
	}
	if !revocation.Complete() || revocation.SessionsStillOpen() || revocation.LastError != "" || revocation.Attempts != 1 {
		t.Fatalf("outcome = %+v", revocation)
	}
	if got := f.kc.roles(revokeSubject); len(got) != 1 || got[0] != "alerts_viewer" {
		t.Fatalf("roles after revoke = %v, want only the access role removed", got)
	}
	if f.kc.count("DELETE /admin/realms/moh/users/"+revokeSubject+"/consents/alerts") != 1 {
		t.Fatalf("consents not revoked: %v", f.kc.requests)
	}
	if len(f.app.snapshot().revoked) != 1 || f.app.snapshot().revoked[0] != (identitysync.Identity{Issuer: revokeIssuer, Subject: revokeSubject}) {
		t.Fatalf("application sessions revoke = %+v", f.app.snapshot().revoked)
	}
	f.assertNoGlobalLogout(t)
	revoked, err := f.service.IsRevoked(ctx, "alerts", revokeIssuer, revokeSubject)
	if err != nil || !revoked {
		t.Fatalf("IsRevoked = %v, %v", revoked, err)
	}
	if other, _ := f.service.IsRevoked(ctx, "clims", revokeIssuer, revokeSubject); other {
		t.Fatal("revocation leaked to another application")
	}
	if strings.Join(f.repo.audit, ",") != AuditRevocationRecorded+","+AuditRevocationAttempted {
		t.Fatalf("audit = %v", f.repo.audit)
	}
}

func TestRevokeAppOutageStillRemovesRoleAndResumeCompletes(t *testing.T) {
	f := newRevokeFixture(t)
	f.app.set(func(a *fakeApp) { a.outage = true })
	ctx := context.Background()
	revocation, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops@example.test", "left the programme")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.RoleRemoved != StepDone || revocation.ConsentsRevoked != StepDone {
		t.Fatalf("an application outage blocked access removal: %+v", revocation)
	}
	if revocation.SessionsRevoked != StepFailed || revocation.LastError != CodeSessionsAppUnavailable || revocation.Complete() {
		t.Fatalf("outage not recorded: %+v", revocation)
	}
	if f.app.snapshot().calls != maxAttempts {
		t.Fatalf("sessions/revoke attempts = %d, want the connector's %d", f.app.snapshot().calls, maxAttempts)
	}
	if roles := f.kc.roles(revokeSubject); len(roles) != 1 {
		t.Fatalf("access role kept during outage: %v", roles)
	}

	f.app.set(func(a *fakeApp) { a.outage = false })
	resumed, err := f.service.Resume(ctx, "alerts", "cli:ops")
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != 1 || resumed[0].SessionsRevoked != StepDone || !resumed[0].Complete() || resumed[0].Attempts != 2 || resumed[0].LastError != "" {
		t.Fatalf("resume = %+v", resumed)
	}
	if f.kc.count("DELETE /admin/realms/moh/users/"+revokeSubject+"/role-mappings") != 1 ||
		f.kc.count("DELETE /admin/realms/moh/users/"+revokeSubject+"/consents") != 1 {
		t.Fatalf("resume repeated finished steps: %v", f.kc.requests)
	}
	if len(f.app.snapshot().revoked) != 1 {
		t.Fatalf("sessions revoked = %v", f.app.snapshot().revoked)
	}
	// Nothing left to resume.
	again, err := f.service.Resume(ctx, "", "cli:ops")
	if err != nil || len(again) != 0 {
		t.Fatalf("second resume = %v, %v", again, err)
	}
}

func TestRevokeKeycloakFailureStillEndsApplicationSessions(t *testing.T) {
	f := newRevokeFixture(t)
	f.kc.update(func(k *revokeFakeKeycloak) { k.failRoles = true })
	revocation, err := f.service.Revoke(context.Background(), "alerts", revokeSubject, "ops@example.test", "incident")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.RoleRemoved != StepFailed || revocation.ConsentsRevoked != StepFailed || revocation.SessionsRevoked != StepDone {
		t.Fatalf("steps = %+v", revocation)
	}
	if revocation.LastError != CodeRoleRemovalFailed {
		t.Fatalf("last error = %q", revocation.LastError)
	}
	f.kc.mu.Lock()
	f.kc.failRoles = false
	f.kc.mu.Unlock()
	resumed, err := f.service.Resume(context.Background(), "alerts", "cli:ops")
	if err != nil || len(resumed) != 1 || !resumed[0].Complete() {
		t.Fatalf("resume = %+v, %v", resumed, err)
	}
	if len(f.app.snapshot().revoked) != 1 {
		t.Fatalf("finished sessions step re-run: %v", f.app.snapshot().revoked)
	}
}

func TestRevoke10ApplicationReportsUnsupportedSessions(t *testing.T) {
	f := newRevokeFixture(t)
	f.registeredAs10()
	revocation, err := f.service.Revoke(context.Background(), "alerts", revokeSubject, "ops@example.test", "left")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.RoleRemoved != StepDone || revocation.ConsentsRevoked != StepDone || revocation.SessionsRevoked != StepUnsupported {
		t.Fatalf("steps = %+v", revocation)
	}
	if !revocation.Complete() || !revocation.SessionsStillOpen() {
		t.Fatalf("1.0 outcome must be complete with sessions still open: %+v", revocation)
	}
	if f.app.snapshot().calls != 0 {
		t.Fatal("POST /sessions/revoke sent to a 1.0 application")
	}
	// Resume does not retry an unsupported step.
	if resumed, err := f.service.Resume(context.Background(), "", "cli:ops"); err != nil || len(resumed) != 0 {
		t.Fatalf("resume = %v, %v", resumed, err)
	}
}

func TestRevokeIsIdempotent(t *testing.T) {
	f := newRevokeFixture(t)
	ctx := context.Background()
	first, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops@example.test", "left")
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.service.Revoke(ctx, "alerts", revokeSubject, "other@example.test", "left again")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || len(f.repo.rows) != 1 {
		t.Fatalf("repeat created another revocation: %v", f.repo.rows)
	}
	if !second.Complete() || second.Attempts != 2 || second.RevokedBy != "ops@example.test" || second.Reason != "left" {
		t.Fatalf("repeat = %+v", second)
	}
	// The role was already gone, so it was not deleted twice.
	if f.kc.count("DELETE /admin/realms/moh/users/"+revokeSubject+"/role-mappings") != 1 {
		t.Fatalf("role deleted again: %v", f.kc.requests)
	}
	// A role granted again by hand is removed by a repeated revoke.
	f.kc.mu.Lock()
	f.kc.direct[revokeSubject] = append(f.kc.direct[revokeSubject], "alerts_access")
	f.kc.mu.Unlock()
	if _, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops@example.test", "re-assert"); err != nil {
		t.Fatal(err)
	}
	if roles := f.kc.roles(revokeSubject); len(roles) != 1 || roles[0] != "alerts_viewer" {
		t.Fatalf("roles = %v", roles)
	}
}

func TestRestoreClearsRevocationWithoutGrantingAccess(t *testing.T) {
	f := newRevokeFixture(t)
	ctx := context.Background()
	if _, err := f.service.Restore(ctx, "alerts", revokeSubject, "ops", "mistake"); !errors.Is(err, ErrRevocationNotFound) {
		t.Fatalf("restore without revocation = %v", err)
	}
	first, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops", "left")
	if err != nil {
		t.Fatal(err)
	}
	requests := f.kc.count("")
	restored, err := f.service.Restore(ctx, "alerts", revokeSubject, "lead@example.test", "returned to post")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Active() || restored.RestoredBy != "lead@example.test" || restored.RestoreReason != "returned to post" {
		t.Fatalf("restored = %+v", restored)
	}
	if f.kc.count("") != requests {
		t.Fatalf("restore called Keycloak: %v", f.kc.requests)
	}
	if roles := f.kc.roles(revokeSubject); len(roles) != 1 || roles[0] != "alerts_viewer" {
		t.Fatalf("restore re-granted access: %v", roles)
	}
	if revoked, err := f.service.IsRevoked(ctx, "alerts", revokeIssuer, revokeSubject); err != nil || revoked {
		t.Fatalf("IsRevoked after restore = %v, %v", revoked, err)
	}
	if _, err := f.service.Restore(ctx, "alerts", revokeSubject, "ops", "again"); !errors.Is(err, ErrRevocationNotFound) {
		t.Fatalf("second restore = %v", err)
	}
	// A later revoke starts a new revocation.
	again, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops", "left again")
	if err != nil || again.ID == first.ID || len(f.repo.rows) != 2 {
		t.Fatalf("revoke after restore = %+v, %v", again, err)
	}
}

func TestRevokeRecordsInheritedAccessRoleAsFailed(t *testing.T) {
	f := newRevokeFixture(t)
	f.kc.update(func(k *revokeFakeKeycloak) { k.inherited[revokeSubject] = []string{"alerts_access"} })
	revocation, err := f.service.Revoke(context.Background(), "alerts", revokeSubject, "ops", "left")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.RoleRemoved != StepFailed || revocation.LastError != CodeAccessRoleInherited {
		t.Fatalf("inherited role not reported: %+v", revocation)
	}
	if revocation.SessionsRevoked != StepDone {
		t.Fatalf("other steps must still run: %+v", revocation)
	}
}

func TestRevokeDeletedKeycloakUserCountsAsRemoved(t *testing.T) {
	f := newRevokeFixture(t)
	gone := "0b6f7c1e-0000-4000-8000-000000000001"
	revocation, err := f.service.Revoke(context.Background(), "alerts", gone, "ops", "user deleted")
	if err != nil {
		t.Fatal(err)
	}
	if !revocation.Complete() || revocation.RoleRemoved != StepDone || revocation.SessionsRevoked != StepDone {
		t.Fatalf("revocation = %+v", revocation)
	}
}

func TestRevokeSessionFailureCodes(t *testing.T) {
	f := newRevokeFixture(t)
	f.app.set(func(a *fakeApp) { a.rejected = true })
	revocation, err := f.service.Revoke(context.Background(), "alerts", revokeSubject, "ops", "left")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.SessionsRevoked != StepFailed || revocation.LastError != CodeSessionsAppRejected || f.app.snapshot().calls != 1 {
		t.Fatalf("rejection = %+v (calls %d)", revocation, f.app.snapshot().calls)
	}

	// A disabled integration still loses access in Keycloak; sessions are recorded as failed.
	g := newRevokeFixture(t)
	row := g.registry.rows["alerts"]
	row.Enabled = false
	g.registry.rows["alerts"] = row
	revocation, err = g.service.Revoke(context.Background(), "alerts", revokeSubject, "ops", "left")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.RoleRemoved != StepDone || revocation.SessionsRevoked != StepFailed || revocation.LastError != CodeIntegrationDisabled {
		t.Fatalf("disabled = %+v", revocation)
	}

	// A broken sync client binding is recorded, never hidden.
	h := newRevokeFixture(t)
	h.registry.bindingErr = fmt.Errorf("%w: sync client disabled", ErrInvalidIntegration)
	revocation, err = h.service.Revoke(context.Background(), "alerts", revokeSubject, "ops", "left")
	if err != nil {
		t.Fatal(err)
	}
	if revocation.RoleRemoved != StepDone || revocation.LastError != CodeConnectorUnavailable {
		t.Fatalf("binding = %+v", revocation)
	}
}

func TestRevokeValidatesInput(t *testing.T) {
	f := newRevokeFixture(t)
	ctx := context.Background()
	for name, args := range map[string][4]string{
		"application": {"../x", revokeSubject, "ops", "r"},
		"subject":     {"alerts", " padded ", "ops", "r"},
		"empty":       {"alerts", "", "ops", "r"},
		"revoker":     {"alerts", revokeSubject, " ", "r"},
		"reason":      {"alerts", revokeSubject, "ops", ""},
	} {
		if _, err := f.service.Revoke(ctx, args[0], args[1], args[2], args[3]); !errors.Is(err, ErrInvalidRevocation) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := f.service.Revoke(ctx, "unregistered", revokeSubject, "ops", "r"); !errors.Is(err, ErrIntegrationNotFound) {
		t.Errorf("unregistered application: err = %v", err)
	}
	if len(f.repo.rows) != 0 || f.kc.count("") != 0 {
		t.Fatal("invalid input reached the repository or Keycloak")
	}
	var unconfigured *RevokeService
	if _, err := unconfigured.Revoke(ctx, "alerts", revokeSubject, "ops", "r"); err == nil {
		t.Fatal("nil service revoked")
	}
}

// ---------------------------------------------------------------------------
// Portal hook
// ---------------------------------------------------------------------------

func TestPortalHookFiresOnlyForEnabledIntegrationAccessRole(t *testing.T) {
	ctx := context.Background()

	f := newRevokeFixture(t)
	hook := NewPortalRevocationHook(f.service)
	// Another role of the same application: no revocation.
	if err := hook.ClientRolesRemoved(ctx, revokeSubject, "alerts", []string{"alerts_viewer"}, "admin-1"); err != nil {
		t.Fatal(err)
	}
	// An application without an identity-sync integration: no revocation.
	if err := hook.ClientRolesRemoved(ctx, revokeSubject, "clims", []string{"clims_access"}, "admin-1"); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.rows) != 0 || f.kc.count("") != 0 {
		t.Fatalf("hook fired: rows=%v requests=%v", f.repo.rows, f.kc.requests)
	}

	// The access role still held through a group: nothing was removed, no revocation.
	f.kc.update(func(k *revokeFakeKeycloak) { k.inherited[revokeSubject] = []string{"alerts_access"} })
	if err := hook.ClientRolesRemoved(ctx, revokeSubject, "alerts", []string{"alerts_access"}, "admin-1"); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.rows) != 0 {
		t.Fatal("hook revoked while access continues through a group")
	}
	f.kc.update(func(k *revokeFakeKeycloak) { k.inherited[revokeSubject] = nil })

	// The access role of an enabled integration: revoked, sessions ended.
	f.kc.update(func(k *revokeFakeKeycloak) {
		k.direct[revokeSubject] = remove(k.direct[revokeSubject], "alerts_access")
	}) // the portal already removed it
	if err := hook.ClientRolesRemoved(ctx, revokeSubject, "alerts", []string{"alerts_viewer", "alerts_access"}, "admin-1"); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.rows) != 1 || f.repo.rows[0].RevokedBy != "portal:admin-1" || !f.repo.rows[0].Complete() {
		t.Fatalf("hook revocation = %+v", f.repo.rows)
	}
	if len(f.app.snapshot().revoked) != 1 {
		t.Fatal("hook did not end application sessions")
	}
	f.assertNoGlobalLogout(t)

	// A disabled integration: no revocation.
	g := newRevokeFixture(t)
	row := g.registry.rows["alerts"]
	row.Enabled = false
	g.registry.rows["alerts"] = row
	if err := NewPortalRevocationHook(g.service).ClientRolesRemoved(ctx, revokeSubject, "alerts", []string{"alerts_access"}, "admin-1"); err != nil {
		t.Fatal(err)
	}
	if len(g.repo.rows) != 0 {
		t.Fatal("hook fired for a disabled integration")
	}
}

func TestPortalHookReportsIncompleteRevocation(t *testing.T) {
	f := newRevokeFixture(t)
	f.portalRemovedAccessRole()
	f.app.set(func(a *fakeApp) { a.outage = true })
	err := NewPortalRevocationHook(f.service).ClientRolesRemoved(context.Background(), revokeSubject, "alerts", []string{"alerts_access"}, "admin-1")
	if err == nil || !strings.Contains(err.Error(), CodeSessionsAppUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if len(f.repo.rows) != 1 || f.repo.rows[0].SessionsRevoked != StepFailed {
		t.Fatalf("failure not recorded: %+v", f.repo.rows)
	}

	g := newRevokeFixture(t)
	g.portalRemovedAccessRole()
	g.registeredAs10()
	err = NewPortalRevocationHook(g.service).ClientRolesRemoved(context.Background(), revokeSubject, "alerts", []string{"alerts_access"}, "")
	if err == nil || !strings.Contains(err.Error(), "cannot end existing sessions") {
		t.Fatalf("1.0 err = %v", err)
	}
	if g.repo.rows[0].RevokedBy != "portal" {
		t.Fatalf("actor = %q", g.repo.rows[0].RevokedBy)
	}

	var nilHook *PortalRevocationHook
	if err := nilHook.ClientRolesRemoved(context.Background(), revokeSubject, "alerts", []string{"alerts_access"}, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPortalHookSurvivesCancelledRequest(t *testing.T) {
	f := newRevokeFixture(t)
	f.portalRemovedAccessRole()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewPortalRevocationHook(f.service).ClientRolesRemoved(ctx, revokeSubject, "alerts", []string{"alerts_access"}, "admin-1"); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.rows) != 1 || !f.repo.rows[0].Complete() {
		t.Fatalf("revocation = %+v", f.repo.rows)
	}
}

// registeredAs10 makes the fixture a 1.0 application as registration records
// it: GET / answers 404 and the registry row's contract version is "1.0".
func (f *revokeFixture) registeredAs10() {
	f.app.set(func(a *fakeApp) { a.legacy = true })
	row := f.registry.rows["alerts"]
	row.ContractVersion = "1.0"
	f.registry.rows["alerts"] = row
}

// portalRemovedAccessRole simulates the portal's own role removal, which runs
// before the hook is called.
func (f *revokeFixture) portalRemovedAccessRole() {
	f.kc.update(func(k *revokeFakeKeycloak) {
		k.direct[revokeSubject] = remove(k.direct[revokeSubject], "alerts_access")
	})
}
