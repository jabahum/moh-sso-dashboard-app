package adapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter/memory"
)

const (
	issuer  = "https://sso.example.test/realms/moh"
	app     = "clims"
	syncCID = "clims-identity-sync"
	acctA   = "11111111-1111-4111-8111-111111111111"
	acctB   = "22222222-2222-4222-8222-222222222222"
	acctX   = "33333333-3333-4333-8333-333333333333"
)

var now = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func goodClaims() adapter.Claims {
	return adapter.Claims{
		Issuer: issuer, Audience: []string{"account", app}, AuthorizedParty: syncCID, Type: "Bearer",
		ExpiresAt: now.Add(time.Minute), ResourceAccess: map[string][]string{app: {"identity_sync"}},
	}
}

// fakeVerifier maps opaque test token strings to claims.
func fakeVerifier(tokens map[string]adapter.Claims) adapter.TokenVerifier {
	return adapter.TokenVerifierFunc(func(_ context.Context, raw string) (adapter.Claims, error) {
		if raw == "unavailable" {
			return adapter.Claims{}, adapter.ErrUnavailable
		}
		c, ok := tokens[raw]
		if !ok {
			return adapter.Claims{}, errors.New("bad signature")
		}
		return c, nil
	})
}

func newHandler(t *testing.T, mutate func(*adapter.Config)) (*adapter.Handler, *memory.Store) {
	t.Helper()
	store := memory.New(issuer, identitysync.AccountIDUUID)
	store.Add(acctA, "a@example.test", true)
	store.Add(acctB, "", true)
	store.Add(acctX, "x@example.test", false)
	cfg := adapter.Config{
		Application: app, Issuer: issuer, SyncClientID: syncCID,
		AccountIDFormat: identitysync.AccountIDUUID, Store: store,
		Verifier: fakeVerifier(map[string]adapter.Claims{"sync": goodClaims()}),
		Probe:    store, Now: func() time.Time { return now },
	}
	if mutate != nil {
		mutate(&cfg)
	}
	h, err := adapter.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return h, store
}

func do(h http.Handler, method, path, token, body string, headers ...string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func identityBody(subject string) string {
	b, _ := json.Marshal(map[string]string{"issuer": issuer, "subject": subject})
	return string(b)
}

func TestAuthorizationSplit(t *testing.T) {
	mod := func(f func(*adapter.Claims)) adapter.Claims { c := goodClaims(); f(&c); return c }
	tokens := map[string]adapter.Claims{
		"sync":       goodClaims(),
		"wrong-iss":  mod(func(c *adapter.Claims) { c.Issuer = "https://other.example.test/realms/moh" }),
		"expired":    mod(func(c *adapter.Claims) { c.ExpiresAt = now.Add(-time.Hour) }),
		"no-exp":     mod(func(c *adapter.Claims) { c.ExpiresAt = time.Time{} }),
		"future-nbf": mod(func(c *adapter.Claims) { c.NotBefore = now.Add(time.Hour) }),
		"wrong-aud":  mod(func(c *adapter.Claims) { c.Audience = []string{"poesgo"} }),
		"id-token":   mod(func(c *adapter.Claims) { c.Type = "ID" }),
		"user":       mod(func(c *adapter.Claims) { c.AuthorizedParty = "clims" }),
		"other":      mod(func(c *adapter.Claims) { c.AuthorizedParty = "poesgo-identity-sync" }),
		"no-role":    mod(func(c *adapter.Claims) { c.ResourceAccess = map[string][]string{app: {"clims_access"}} }),
		"other-role": mod(func(c *adapter.Claims) { c.ResourceAccess = map[string][]string{"poesgo": {"identity_sync"}} }),
	}
	h, _ := newHandler(t, func(c *adapter.Config) { c.Verifier = fakeVerifier(tokens) })
	cases := map[string]int{
		"": 401, "garbage": 401, "wrong-iss": 401, "expired": 401, "no-exp": 401, "future-nbf": 401, "wrong-aud": 401,
		"id-token": 403, "user": 403, "other": 403, "no-role": 403, "other-role": 403,
		"unavailable": 503, "sync": 200,
	}
	for token, want := range cases {
		for _, path := range []string{"/", adapter.BasePath, adapter.BasePath + "/", adapter.BasePath + "/accounts"} {
			w := do(h, http.MethodGet, path, token, "")
			if w.Code != want {
				t.Errorf("token %q %s: status %d, want %d", token, path, w.Code, want)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("token %q %s: missing no-store", token, path)
			}
		}
	}
	// Non-bearer schemes, cookies and duplicate headers are refused.
	for _, header := range []string{"Basic c3luYzpzeW5j", "sync", "Bearer  sync", "Bearer"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", header)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Errorf("Authorization %q: status %d", header, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Add("Authorization", "Bearer sync")
	r.Header.Add("Authorization", "Bearer sync")
	r.AddCookie(&http.Cookie{Name: "session", Value: "x"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Errorf("duplicate Authorization: status %d", w.Code)
	}
	if w := do(h, http.MethodGet, "/", "", "", "Cookie", "session=x"); w.Code != 401 {
		t.Errorf("cookie only: status %d", w.Code)
	}
}

func TestDescriptor(t *testing.T) {
	h, _ := newHandler(t, nil)
	w := do(h, http.MethodGet, "/", "sync", "")
	var d identitysync.Descriptor
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(app); err != nil || d.Version != "1.1" || !d.Supports(identitysync.CapabilityRevokeSessions) {
		t.Fatalf("descriptor %+v: %v", d, err)
	}
	if w := do(h, http.MethodPost, "/", "sync", "{}"); w.Code != 405 || w.Header().Get("Allow") != "GET" {
		t.Fatalf("POST /: %d", w.Code)
	}
}

func TestListAccounts(t *testing.T) {
	h, store := newHandler(t, nil)
	if err := store.LinkIdentity(context.Background(), acctB, identitysync.Identity{Issuer: issuer, Subject: "sub-b"}); err != nil {
		t.Fatal(err)
	}
	w := do(h, http.MethodGet, "/accounts?limit=1", "sync", "")
	if w.Code != 200 || w.Body.String() != `{"accounts":[{"id":"`+acctA+`","email":"a@example.test","link":null}],"nextCursor":"`+acctA+`"}` {
		t.Fatalf("page 1: %d %s", w.Code, w.Body)
	}
	w = do(h, http.MethodGet, "/accounts?limit=1&after="+acctA, "sync", "")
	if w.Code != 200 || w.Body.String() != `{"accounts":[{"id":"`+acctB+`","email":"","link":{"issuer":"`+issuer+`","subject":"sub-b"}}],"nextCursor":""}` {
		t.Fatalf("page 2: %d %s", w.Code, w.Body)
	}
	w = do(h, http.MethodGet, "/accounts?after="+acctB, "sync", "")
	if w.Code != 200 || w.Body.String() != `{"accounts":[],"nextCursor":""}` {
		t.Fatalf("page 3: %d %s", w.Code, w.Body)
	}
	for _, q := range []string{"limit=0", "limit=101", "limit=01", "limit=x", "after=", "after=bad%20cursor", "after=not-a-uuid", "limit=1&limit=2", "after=%zz"} {
		if w := do(h, http.MethodGet, "/accounts?"+q, "sync", ""); w.Code != 400 {
			t.Errorf("%s: status %d", q, w.Code)
		}
	}
}

type badPageStore struct {
	*memory.Store
	page identitysync.AccountPage
}

func (s badPageStore) ListEligibleAccounts(context.Context, string, int) (identitysync.AccountPage, error) {
	return s.page, nil
}

func TestStorePagesThatBreakTheContractAreNotPublished(t *testing.T) {
	cases := map[string]identitysync.AccountPage{
		"bad id":       {Accounts: []identitysync.Account{{ID: "42"}}},
		"out of order": {Accounts: []identitysync.Account{{ID: acctB}, {ID: acctA}}},
		"duplicate":    {Accounts: []identitysync.Account{{ID: acctA}, {ID: acctA}}},
		"other issuer": {Accounts: []identitysync.Account{{ID: acctA, Link: identitysync.Identity{Issuer: "https://x.test", Subject: "s"}}}},
		"bad cursor":   {NextCursor: "bad cursor"},
	}
	for name, page := range cases {
		var reported error
		h, store := newHandler(t, nil)
		h, _ = newHandler(t, func(c *adapter.Config) {
			c.Store = badPageStore{store, page}
			c.OnError = func(_ *http.Request, err error) { reported = err }
		})
		if w := do(h, http.MethodGet, "/accounts", "sync", ""); w.Code != 500 || reported == nil {
			t.Errorf("%s: status %d, reported %v", name, w.Code, reported)
		}
	}
}

func TestLinkIdentity(t *testing.T) {
	h, store := newHandler(t, nil)
	put := func(id, body string) int {
		return do(h, http.MethodPut, "/accounts/"+id+"/identity", "sync", body).Code
	}
	steps := []struct {
		id, body string
		want     int
	}{
		{acctA, identityBody("s1"), 204},
		{acctA, identityBody("s1"), 204},
		{acctA, identityBody("s2"), 409},
		{acctB, identityBody("s1"), 409},
		{acctX, identityBody("s3"), 404},
		{"not-a-uuid", identityBody("s3"), 404},
		{"44444444-4444-4444-8444-444444444444", identityBody("s3"), 404},
		{acctB, `{"issuer":"https://other.test/realms/moh","subject":"s3"}`, 400},
		{acctB, `{"issuer":"` + issuer + `","subject":""}`, 400},
		{acctB, `{"issuer":"` + issuer + `","subject":" s3"}`, 400},
		{acctB, `{"issuer":"` + issuer + `","subject":"` + strings.Repeat("s", 256) + `"}`, 400},
		{acctB, `{"issuer":"` + issuer + `"}`, 400},
		{acctB, `{"issuer":"` + issuer + `","subject":"s3","extra":1}`, 400},
		{acctB, identityBody("s3") + identityBody("s3"), 400},
		{acctB, `[]`, 400},
		{acctB, ``, 400},
		{acctB, `{"issuer":"` + issuer + `","subject":"s3"}` + strings.Repeat(" ", 4096), 400},
	}
	for i, s := range steps {
		if got := put(s.id, s.body); got != s.want {
			t.Errorf("step %d (%s): status %d, want %d", i, s.id, got, s.want)
		}
	}
	if store.Link(acctA) != "s1" || store.Link(acctB) != "" {
		t.Fatalf("links changed: A=%q B=%q", store.Link(acctA), store.Link(acctB))
	}
	if w := do(h, http.MethodGet, "/accounts/"+acctA+"/identity", "sync", ""); w.Code != 405 {
		t.Fatalf("GET identity: %d", w.Code)
	}
	if w := do(h, http.MethodPut, "/accounts/identity", "sync", identityBody("s9")); w.Code != 404 {
		t.Fatalf("/accounts/identity: %d", w.Code)
	}
}

func TestRevokeAndProbe(t *testing.T) {
	h, store := newHandler(t, func(c *adapter.Config) { c.ProbeEnabled = true })
	if err := store.LinkIdentity(context.Background(), acctA, identitysync.Identity{Issuer: issuer, Subject: "s1"}); err != nil {
		t.Fatal(err)
	}
	w := do(h, http.MethodPost, "/test/sessions", "sync", identityBody("s1"))
	if w.Code != 201 {
		t.Fatalf("create session: %d %s", w.Code, w.Body)
	}
	var created struct{ SessionToken string }
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if do(h, http.MethodGet, "/test/sessions/check", "sync", "", "X-Test-Session", created.SessionToken).Code != 200 {
		t.Fatal("new session not active")
	}
	if do(h, http.MethodGet, "/test/sessions/check", "", "", "X-Test-Session", created.SessionToken).Code != 401 {
		t.Fatal("probe accepted a request without a bearer token")
	}
	if do(h, http.MethodGet, "/test/sessions/check", "sync", "").Code != 400 {
		t.Fatal("check without X-Test-Session")
	}
	if do(h, http.MethodPost, "/test/sessions", "sync", identityBody("unlinked")).Code != 404 {
		t.Fatal("session for unlinked identity")
	}
	for i := 0; i < 2; i++ {
		if c := do(h, http.MethodPost, "/sessions/revoke", "sync", identityBody("s1")).Code; c != 204 {
			t.Fatalf("revoke %d: %d", i, c)
		}
	}
	if do(h, http.MethodGet, "/test/sessions/check", "sync", "", "X-Test-Session", created.SessionToken).Code != 401 {
		t.Fatal("session survived revocation")
	}
	if store.Link(acctA) != "s1" {
		t.Fatal("revocation removed the link")
	}
	if do(h, http.MethodPost, "/sessions/revoke", "sync", identityBody("unknown")).Code != 204 {
		t.Fatal("revoke unknown subject")
	}
	if do(h, http.MethodPost, "/sessions/revoke", "sync", `{"issuer":"https://x.test/realms/moh","subject":"s1"}`).Code != 400 {
		t.Fatal("revoke wrong issuer")
	}
}

func TestProbeRoutesDoNotExistUnlessEnabled(t *testing.T) {
	h, _ := newHandler(t, nil)
	if c := do(h, http.MethodPost, "/test/sessions", "sync", identityBody("s1")).Code; c != 404 {
		t.Fatalf("probe create without flag: %d", c)
	}
	if c := do(h, http.MethodGet, "/test/sessions/check", "sync", "", "X-Test-Session", "x").Code; c != 404 {
		t.Fatalf("probe check without flag: %d", c)
	}
	if adapter.ProbeEnabledFromEnv(func(string) string { return "true" }) || !adapter.ProbeEnabledFromEnv(func(string) string { return "1" }) {
		t.Fatal("ProbeEnabledFromEnv must accept exactly 1")
	}
}

type errStore struct {
	*memory.Store
	err error
}

func (s errStore) LinkIdentity(context.Context, string, identitysync.Identity) error { return s.err }

func TestStoreErrorsMapToRetryableStatuses(t *testing.T) {
	for err, want := range map[error]int{adapter.ErrRateLimited: 429, adapter.ErrUnavailable: 503, errors.New("db down"): 500} {
		h, store := newHandler(t, nil)
		h, _ = newHandler(t, func(c *adapter.Config) { c.Store = errStore{store, err} })
		w := do(h, http.MethodPut, "/accounts/"+acctA+"/identity", "sync", identityBody("s1"))
		if w.Code != want || (want != 500 && w.Header().Get("Retry-After") == "") {
			t.Errorf("%v: status %d", err, w.Code)
		}
		if strings.Contains(w.Body.String(), "db down") {
			t.Errorf("internal error leaked: %s", w.Body)
		}
	}
}

func TestNewValidatesConfig(t *testing.T) {
	store := memory.New(issuer, identitysync.AccountIDUUID)
	base := adapter.Config{Application: app, Issuer: issuer, SyncClientID: syncCID, AccountIDFormat: identitysync.AccountIDUUID,
		Store: store, Verifier: fakeVerifier(nil)}
	bad := []func(*adapter.Config){
		func(c *adapter.Config) { c.Application = "" },
		func(c *adapter.Config) { c.Issuer = "http://sso.example.test/realms/moh" },
		func(c *adapter.Config) { c.SyncClientID = app },
		func(c *adapter.Config) { c.AccountIDFormat = "email" },
		func(c *adapter.Config) { c.MaxPageSize = 101 },
		func(c *adapter.Config) { c.Store = nil },
		func(c *adapter.Config) { c.Verifier = nil },
		func(c *adapter.Config) { c.ProbeEnabled = true },
	}
	for i, mutate := range bad {
		cfg := base
		mutate(&cfg)
		if _, err := adapter.New(cfg); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
	if _, err := adapter.New(base); err != nil {
		t.Fatal(err)
	}
}

func TestMaxPageSizeCapsPages(t *testing.T) {
	h, _ := newHandler(t, func(c *adapter.Config) { c.MaxPageSize = 1 })
	w := do(h, http.MethodGet, "/accounts?limit=100", "sync", "")
	var page struct{ Accounts []json.RawMessage }
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if w.Code != 200 || len(page.Accounts) != 1 {
		t.Fatalf("status %d, %d accounts", w.Code, len(page.Accounts))
	}
}

func TestPositiveIntegerOrder(t *testing.T) {
	if !adapter.AccountIDLess(identitysync.AccountIDPositiveInteger, "9", "10") || adapter.AccountIDLess(identitysync.AccountIDOpaque, "9", "10") {
		t.Fatal("ordering")
	}
}
