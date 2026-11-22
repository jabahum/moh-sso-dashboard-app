package conformance_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter/memory"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/conformance"
)

const (
	issuer  = "https://sso.example.test/realms/moh"
	app     = "clims"
	syncCID = "clims-identity-sync"
)

// Synthetic positive-integer accounts: 1-25 eligible, 900 ineligible.
const (
	linkA      = "3"
	linkB      = "17"
	ineligible = "900"
)

func claims() adapter.Claims {
	return adapter.Claims{
		Issuer: issuer, Audience: []string{app}, AuthorizedParty: syncCID, Type: "Bearer",
		ExpiresAt: time.Now().Add(time.Hour), ResourceAccess: map[string][]string{app: {"identity_sync"}},
	}
}

// tokenClaims are the fake realm: opaque token strings and the claims a real
// verifier would return for them. "expired" fails verification.
func tokenClaims() map[string]adapter.Claims {
	wrongAud, user, other, noRole := claims(), claims(), claims(), claims()
	wrongAud.Audience = []string{"account"}
	user.AuthorizedParty = app
	other.AuthorizedParty = "poesgo-identity-sync"
	noRole.ResourceAccess = map[string][]string{app: {"clims_access"}}
	return map[string]adapter.Claims{
		"sync-token": claims(), "wrong-aud-token": wrongAud, "user-token": user,
		"other-client-token": other, "no-role-token": noRole,
	}
}

func verifier(tokens map[string]adapter.Claims) adapter.TokenVerifier {
	return adapter.TokenVerifierFunc(func(_ context.Context, raw string) (adapter.Claims, error) {
		if c, ok := tokens[raw]; ok {
			return c, nil
		}
		return adapter.Claims{}, errors.New("signature does not verify")
	})
}

func allTokens() conformance.Tokens {
	return conformance.Tokens{
		Sync:             conformance.StaticToken("sync-token"),
		ExpiredOrGarbage: conformance.StaticToken("expired-token"),
		WrongAudience:    conformance.StaticToken("wrong-aud-token"),
		User:             conformance.StaticToken("user-token"),
		OtherClient:      conformance.StaticToken("other-client-token"),
		MissingRole:      conformance.StaticToken("no-role-token"),
	}
}

func fixtures() conformance.Fixtures {
	return conformance.Fixtures{
		Application: app, Issuer: issuer,
		LinkableAccounts: []string{linkA, linkB}, IneligibleAccount: ineligible,
		Subjects: []string{"synthetic-subject-1", "synthetic-subject-2", "synthetic-subject-3"},
	}
}

type env struct {
	store    *memory.Store
	appStore adapter.Store
	verifier adapter.TokenVerifier
	wrap     func(http.Handler) http.Handler
	probe    bool
}

func newEnv() *env {
	store := memory.New(issuer, identitysync.AccountIDPositiveInteger)
	for i := 1; i <= 25; i++ {
		store.Add(fmt.Sprint(i), fmt.Sprintf("user%d@example.test", i), true)
	}
	store.Add(ineligible, "disabled@example.test", false)
	return &env{store: store, appStore: store, verifier: verifier(tokenClaims()), probe: true}
}

func (e *env) run(t *testing.T, mutate func(*conformance.Config)) conformance.Report {
	t.Helper()
	h, err := adapter.New(adapter.Config{
		Application: app, Issuer: issuer, SyncClientID: syncCID,
		AccountIDFormat: identitysync.AccountIDPositiveInteger,
		Store:           e.appStore, Verifier: e.verifier,
		ProbeEnabled: e.probe, Probe: e.store,
	})
	if err != nil {
		t.Fatal(err)
	}
	var handler http.Handler = h
	if e.wrap != nil {
		handler = e.wrap(handler)
	}
	mux := http.NewServeMux()
	mux.Handle(adapter.BasePath, handler)
	mux.Handle(adapter.BasePath+"/", handler)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	cfg := conformance.Config{
		BaseURL: srv.URL + adapter.BasePath, Client: srv.Client(),
		Tokens: allTokens(), Fixtures: fixtures(), Probe: e.probe,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	report, err := conformance.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func dump(t *testing.T, report conformance.Report) {
	t.Helper()
	for _, r := range report.Results {
		t.Logf("%-4s %-40s %s", r.Status, r.ID, r.Detail)
	}
}

func TestAdapterPassesEveryCheck(t *testing.T) {
	report := newEnv().run(t, nil)
	pass, fail, skip := report.Counts()
	for _, r := range report.Results {
		if r.Status == conformance.Fail || (r.Status == conformance.Skip && r.ID != "transport.retries") {
			dump(t, report)
			t.Fatalf("%s: %s %s", r.ID, r.Status, r.Detail)
		}
	}
	if report.Failed() || fail != 0 || skip != 1 || pass < 35 {
		t.Fatalf("pass=%d fail=%d skip=%d", pass, fail, skip)
	}
	// Every check of the design's conformance table is present.
	for _, id := range []string{
		"authorization.no-token", "authorization.expired-token", "authorization.wrong-audience",
		"authorization.user-token", "authorization.other-client", "authorization.missing-role",
		"descriptor.shape", "descriptor.application", "descriptor.capabilities",
		"pagination.walk-limit-1", "pagination.walk-limit-7", "pagination.walk-limit-100",
		"pagination.consistent", "pagination.bad-cursor",
		"linking.link", "linking.repeat", "linking.other-subject", "linking.subject-taken",
		"linking.ineligible", "linking.wrong-issuer",
		"sessions.revoke", "sessions.revoke-repeat", "sessions.revoke-unknown", "sessions.revoked-session-rejected",
		"transport.no-store", "transport.no-redirects", "transport.request-body-limit", "transport.response-size",
	} {
		if r, ok := report.Result(id); !ok || r.Status != conformance.Pass {
			t.Errorf("%s: %+v", id, r)
		}
	}
}

func TestGeneratedSubjectsAndUUIDAccounts(t *testing.T) {
	store := memory.New(issuer, identitysync.AccountIDUUID)
	ids := []string{
		"0a000000-0000-4000-8000-000000000001", "0b000000-0000-4000-8000-000000000002",
		"0c000000-0000-4000-8000-000000000003", "ff000000-0000-4000-8000-000000000004",
	}
	for _, id := range ids[:3] {
		store.Add(id, "", true)
	}
	store.Add(ids[3], "", false)
	h, err := adapter.New(adapter.Config{Application: app, Issuer: issuer, SyncClientID: syncCID,
		AccountIDFormat: identitysync.AccountIDUUID, Store: store, Verifier: verifier(tokenClaims())})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(h)
	defer srv.Close()
	report, err := conformance.Run(context.Background(), conformance.Config{
		BaseURL: srv.URL + adapter.BasePath + "/", Client: srv.Client(),
		Tokens: conformance.Tokens{Sync: conformance.StaticToken("sync-token")},
		Fixtures: conformance.Fixtures{Application: app, Issuer: issuer,
			LinkableAccounts: ids[1:3], IneligibleAccount: ids[3]},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed() {
		dump(t, report)
		t.Fatal("unexpected failure")
	}
	if !strings.HasPrefix(store.Link(ids[1]), "conformance-") {
		t.Fatalf("generated subject not used: %q", store.Link(ids[1]))
	}
}

func TestMissingInputsAreSkippedNotPassed(t *testing.T) {
	e := newEnv()
	e.probe = false
	report := e.run(t, func(c *conformance.Config) {
		c.Tokens = conformance.Tokens{Sync: conformance.StaticToken("sync-token")}
	})
	if report.Failed() {
		dump(t, report)
		t.Fatal("unexpected failure")
	}
	for _, id := range []string{
		"authorization.expired-token", "authorization.wrong-audience", "authorization.user-token",
		"authorization.other-client", "authorization.missing-role",
		"sessions.probe-create", "sessions.probe-active", "sessions.probe-unlinked",
		"sessions.revoked-session-rejected", "transport.retries",
	} {
		r, ok := report.Result(id)
		if !ok || r.Status != conformance.Skip || r.Detail == "" {
			t.Errorf("%s: want skip with a reason, got %+v", id, r)
		}
	}
	// Checks that need no extra input still run.
	for _, id := range []string{"authorization.no-token", "authorization.garbage-token", "sessions.revoke"} {
		if r, _ := report.Result(id); r.Status != conformance.Pass {
			t.Errorf("%s: %+v", id, r)
		}
	}
}

// ---- Deliberately broken applications --------------------------------------

type replacingStore struct{ *memory.Store }

func (s replacingStore) LinkIdentity(ctx context.Context, id string, identity identitysync.Identity) error {
	err := s.Store.LinkIdentity(ctx, id, identity)
	if errors.Is(err, identitysync.ErrLinkConflict) {
		s.Unlink(id)
		return s.Store.LinkIdentity(ctx, id, identity)
	}
	return err
}

type conflictAs404Store struct{ *memory.Store }

func (s conflictAs404Store) LinkIdentity(ctx context.Context, id string, identity identitysync.Identity) error {
	if err := s.Store.LinkIdentity(ctx, id, identity); errors.Is(err, identitysync.ErrLinkConflict) {
		return identitysync.ErrAccountNotFound
	} else {
		return err
	}
}

type noopRevokeStore struct{ *memory.Store }

func (noopRevokeStore) RevokeSessions(context.Context, identitysync.Identity) error { return nil }

type unlinkingRevokeStore struct{ *memory.Store }

func (s unlinkingRevokeStore) RevokeSessions(ctx context.Context, identity identitysync.Identity) error {
	page, _ := s.ListEligibleAccounts(ctx, "", 100)
	for _, a := range page.Accounts {
		if a.Link == identity {
			s.Unlink(a.ID)
		}
	}
	return nil
}

// overlappingPagesStore points nextCursor one account too early, so the last
// account of each page is listed again.
type overlappingPagesStore struct{ *memory.Store }

func (s overlappingPagesStore) ListEligibleAccounts(ctx context.Context, cursor string, limit int) (identitysync.AccountPage, error) {
	page, err := s.Store.ListEligibleAccounts(ctx, cursor, limit)
	if err == nil && page.NextCursor != "" && len(page.Accounts) >= 2 {
		page.NextCursor = page.Accounts[len(page.Accounts)-2].ID
	}
	return page, err
}

type headerStrippingWriter struct {
	http.ResponseWriter
}

func (w headerStrippingWriter) WriteHeader(status int) {
	w.Header().Del("Cache-Control")
	w.ResponseWriter.WriteHeader(status)
}

func (w headerStrippingWriter) Write(b []byte) (int, error) {
	w.Header().Del("Cache-Control")
	return w.ResponseWriter.Write(b)
}

func TestBrokenApplicationsFailTheRightChecks(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *env)
		cfg   func(c *conformance.Config)
		fails []string
		pass  []string
	}{
		{
			name:  "replaces existing links",
			setup: func(e *env) { e.appStore = replacingStore{e.store} },
			fails: []string{"linking.other-subject", "linking.visible"},
		},
		{
			name:  "wrong status on conflict",
			setup: func(e *env) { e.appStore = conflictAs404Store{e.store} },
			fails: []string{"linking.other-subject", "linking.subject-taken"},
			pass:  []string{"linking.link", "linking.visible"},
		},
		{
			name: "missing no-store",
			setup: func(e *env) {
				e.wrap = func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						next.ServeHTTP(headerStrippingWriter{w}, r)
					})
				}
			},
			fails: []string{"transport.no-store"},
		},
		{
			name: "unauthenticated access",
			setup: func(e *env) {
				e.wrap = func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Authorization") == "" {
							r.Header.Set("Authorization", "Bearer sync-token")
						}
						next.ServeHTTP(w, r)
					})
				}
			},
			fails: []string{"authorization.no-token"},
			pass:  []string{"authorization.non-bearer", "authorization.user-token"},
		},
		{
			name: "accepts any token",
			setup: func(e *env) {
				e.verifier = adapter.TokenVerifierFunc(func(context.Context, string) (adapter.Claims, error) { return claims(), nil })
			},
			fails: []string{"authorization.garbage-token", "authorization.expired-token", "authorization.wrong-audience",
				"authorization.user-token", "authorization.other-client", "authorization.missing-role"},
			pass: []string{"authorization.no-token"},
		},
		{
			name: "does not check azp",
			setup: func(e *env) {
				inner := verifier(tokenClaims())
				e.verifier = adapter.TokenVerifierFunc(func(ctx context.Context, raw string) (adapter.Claims, error) {
					c, err := inner.VerifyAccessToken(ctx, raw)
					c.AuthorizedParty = syncCID
					return c, err
				})
			},
			fails: []string{"authorization.user-token", "authorization.other-client"},
			pass:  []string{"authorization.missing-role", "authorization.wrong-audience"},
		},
		{
			name:  "revocation does not end sessions",
			setup: func(e *env) { e.appStore = noopRevokeStore{e.store} },
			fails: []string{"sessions.revoked-session-rejected"},
			pass:  []string{"sessions.revoke", "sessions.probe-active"},
		},
		{
			name:  "revocation removes the link",
			setup: func(e *env) { e.appStore = unlinkingRevokeStore{e.store} },
			fails: []string{"sessions.link-kept"},
		},
		{
			name:  "overlapping pages",
			setup: func(e *env) { e.appStore = overlappingPagesStore{e.store} },
			fails: []string{"pagination.walk-limit-7"},
		},
		{
			name: "redirects",
			setup: func(e *env) {
				e.wrap = func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == adapter.BasePath+"/" {
							http.Redirect(w, r, adapter.BasePath, http.StatusPermanentRedirect)
							return
						}
						next.ServeHTTP(w, r)
					})
				}
			},
			fails: []string{"transport.no-redirects", "descriptor.shape"},
		},
		{
			name: "request body limit not enforced",
			setup: func(e *env) {
				e.wrap = func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, _ := io.ReadAll(r.Body)
						r.Body = io.NopCloser(bytes.NewReader(bytes.TrimSpace(body)))
						next.ServeHTTP(w, r)
					})
				}
			},
			fails: []string{"transport.request-body-limit"},
		},
		{
			name: "advertises sessions.revoke without the endpoint",
			setup: func(e *env) {
				e.wrap = func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.HasSuffix(r.URL.Path, "/sessions/revoke") {
							w.Header().Set("Cache-Control", "no-store")
							http.NotFound(w, r)
							return
						}
						next.ServeHTTP(w, r)
					})
				}
			},
			fails: []string{"descriptor.capabilities", "sessions.revoke"},
		},
		{
			name:  "descriptor names another application",
			cfg:   func(c *conformance.Config) { c.Fixtures.Application = "poesgo" },
			fails: []string{"descriptor.application"},
		},
		{
			name: "probe enabled in the run but routes absent",
			setup: func(e *env) {
				e.probe = false
			},
			cfg:   func(c *conformance.Config) { c.Probe = true },
			fails: []string{"sessions.probe-create"},
		},
		{
			name: "unknown error code and echoed token",
			setup: func(e *env) {
				e.wrap = func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Authorization") == "Bearer user-token" {
							w.Header().Set("Cache-Control", "no-store")
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusForbidden)
							_, _ = io.WriteString(w, `{"error":{"code":"forbidden","message":"user-token refused"}}`)
							return
						}
						next.ServeHTTP(w, r)
					})
				}
			},
			fails: []string{"transport.error-bodies"},
			pass:  []string{"authorization.user-token"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			if tc.setup != nil {
				tc.setup(e)
			}
			report := e.run(t, tc.cfg)
			if !report.Failed() {
				dump(t, report)
				t.Fatal("broken application passed the suite")
			}
			for _, id := range tc.fails {
				if r, ok := report.Result(id); !ok || r.Status != conformance.Fail {
					dump(t, report)
					t.Errorf("%s: want fail, got %+v", id, r)
				}
			}
			for _, id := range tc.pass {
				if r, ok := report.Result(id); !ok || r.Status != conformance.Pass {
					dump(t, report)
					t.Errorf("%s: want pass, got %+v", id, r)
				}
			}
		})
	}
}

func TestAlreadyLinkedFixturesBlockLinking(t *testing.T) {
	e := newEnv()
	if err := e.store.LinkIdentity(context.Background(), linkA, identitysync.Identity{Issuer: issuer, Subject: "left-over"}); err != nil {
		t.Fatal(err)
	}
	report := e.run(t, nil)
	if r, _ := report.Result("pagination.fixtures"); r.Status != conformance.Fail || !strings.Contains(r.Detail, "already linked") {
		t.Fatalf("fixtures: %+v", r)
	}
	if r, _ := report.Result("linking.link"); r.Status != conformance.Skip {
		t.Fatalf("linking.link: %+v", r)
	}
	if e.store.Link(linkA) != "left-over" {
		t.Fatal("suite changed a pre-existing link")
	}
}

func TestRunRejectsUnusableConfig(t *testing.T) {
	good := conformance.Config{BaseURL: "https://app.example.test" + adapter.BasePath, Tokens: allTokens(), Fixtures: fixtures()}
	bad := []func(*conformance.Config){
		func(c *conformance.Config) { c.BaseURL = "http://app.example.test" },
		func(c *conformance.Config) { c.BaseURL = "https://app.example.test/?x=1" },
		func(c *conformance.Config) { c.Tokens.Sync = nil },
		func(c *conformance.Config) { c.Fixtures.Application = "" },
		func(c *conformance.Config) { c.Fixtures.LinkableAccounts = []string{"1"} },
		func(c *conformance.Config) { c.Fixtures.LinkableAccounts = []string{"1", "1"} },
		func(c *conformance.Config) { c.Fixtures.IneligibleAccount = linkA },
		func(c *conformance.Config) { c.Fixtures.Subjects = []string{"a", "a", "b"} },
		func(c *conformance.Config) { c.Fixtures.Subjects = []string{"a", "b"} },
	}
	for i, mutate := range bad {
		cfg := good
		cfg.Fixtures.LinkableAccounts = append([]string(nil), good.Fixtures.LinkableAccounts...)
		mutate(&cfg)
		if _, err := conformance.Run(context.Background(), cfg); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}
