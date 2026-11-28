package client

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
	"sync/atomic"
	"testing"
	"time"
)

// fakePortal plays both Keycloak's token endpoint and the portal.
type fakePortal struct {
	t      *testing.T
	server *httptest.Server

	tokens    atomic.Int32 // token requests
	expiresIn int64
	reject    atomic.Int32 // portal answers 401 this many times

	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	handler  func(w http.ResponseWriter, r *http.Request)
}

func newFakePortal(t *testing.T) *fakePortal {
	f := &fakePortal{t: t, expiresIn: 300}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakePortal) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "client_credentials" || r.PostForm.Get("client_id") != "alerts-platform" || r.PostForm.Get("client_secret") != "s3cret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"invalid_client"}`)
			return
		}
		n := f.tokens.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": fmt.Sprintf("machine-%d", n), "token_type": "Bearer", "expires_in": f.expiresIn})
		return
	}
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, r.Clone(context.Background()))
	f.bodies = append(f.bodies, string(body))
	handler := f.handler
	f.mu.Unlock()
	if f.reject.Load() > 0 {
		f.reject.Add(-1)
		writeError(w, 401, "INVALID_IDENTITY", "valid bearer token required")
		return
	}
	handler(w, r)
}

func (f *fakePortal) last() (*http.Request, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1], f.bodies[len(f.bodies)-1]
}

func (f *fakePortal) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data, "meta": map[string]any{"requestId": "req-1"}})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"code": code, "message": message}, "meta": map[string]any{"requestId": "req-err"}})
}

func (f *fakePortal) client(t *testing.T, mutate ...func(*Config)) *Client {
	t.Helper()
	cfg := Config{
		PortalURL: f.server.URL,
		Tokens:    &ClientCredentials{TokenURL: f.server.URL + "/token", ClientID: "alerts-platform", ClientSecret: "s3cret", Scopes: []string{"identity:read", "notifications:write"}},
	}
	for _, m := range mutate {
		m(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

func TestIdentityUsesACachedMachineToken(t *testing.T) {
	f := newFakePortal(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		writeData(w, 200, map[string]any{"applicationId": "alerts", "clientId": "alerts-platform", "subject": "sa-1", "scopes": []string{"identity:read"}})
	}
	c := f.client(t)
	for range 3 {
		id, err := c.Identity(context.Background())
		if err != nil || id.ApplicationID != "alerts" || !id.HasScope("identity:read") || id.HasScope("notifications:write") {
			t.Fatalf("identity = %+v, %v", id, err)
		}
	}
	if f.tokens.Load() != 1 {
		t.Fatalf("token requests = %d, want 1", f.tokens.Load())
	}
	req, _ := f.last()
	if req.Method != http.MethodGet || req.URL.Path != "/api/v1/platform/identity" || req.Header.Get("Authorization") != "Bearer machine-1" || !strings.Contains(req.Header.Get("User-Agent"), "moh-platform-client-go") {
		t.Fatalf("request = %s %s %v", req.Method, req.URL.Path, req.Header)
	}
}

func TestClientCredentialsRefreshesNearExpiryAndOnRejection(t *testing.T) {
	f := newFakePortal(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) { writeData(w, 200, map[string]any{"applicationId": "alerts"}) }
	now := time.Unix(1_800_000_000, 0)
	tokens := &ClientCredentials{TokenURL: f.server.URL + "/token", ClientID: "alerts-platform", ClientSecret: "s3cret", Now: func() time.Time { return now }}
	c := f.client(t, func(cfg *Config) { cfg.Tokens = tokens })
	if _, err := c.Identity(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(269 * time.Second) // 31 s left: still cached
	if _, err := c.Identity(context.Background()); err != nil || f.tokens.Load() != 1 {
		t.Fatalf("refreshed too early: %d %v", f.tokens.Load(), err)
	}
	now = now.Add(2 * time.Second) // 29 s left: refresh
	if _, err := c.Identity(context.Background()); err != nil || f.tokens.Load() != 2 {
		t.Fatalf("did not refresh near expiry: %d %v", f.tokens.Load(), err)
	}
	f.reject.Store(1) // the portal rejects the cached token once
	if _, err := c.Identity(context.Background()); err != nil || f.tokens.Load() != 3 {
		t.Fatalf("did not refresh after a 401: %d %v", f.tokens.Load(), err)
	}
	f.reject.Store(5) // persistent 401: one refresh, then the error
	_, err := c.Identity(context.Background())
	var apiErr *Error
	if !errors.Is(err, ErrUnauthorized) || !errors.As(err, &apiErr) || apiErr.Code != "INVALID_IDENTITY" || apiErr.RequestID != "req-err" || f.tokens.Load() != 4 {
		t.Fatalf("err = %v, tokens = %d", err, f.tokens.Load())
	}
}

func TestClientCredentialsConcurrentCallersShareOneRequest(t *testing.T) {
	f := newFakePortal(t)
	tokens := &ClientCredentials{TokenURL: f.server.URL + "/token", ClientID: "alerts-platform", ClientSecret: "s3cret"}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tokens.Token(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if f.tokens.Load() != 1 {
		t.Fatalf("token requests = %d, want 1", f.tokens.Load())
	}
}

func TestClientCredentialsErrors(t *testing.T) {
	f := newFakePortal(t)
	for name, tokens := range map[string]*ClientCredentials{
		"wrong secret":   {TokenURL: f.server.URL + "/token", ClientID: "alerts-platform", ClientSecret: "nope"},
		"missing secret": {TokenURL: f.server.URL + "/token", ClientID: "alerts-platform"},
		"plain http":     {TokenURL: "http://auth.example.org/token", ClientID: "alerts-platform", ClientSecret: "s3cret"},
	} {
		_, err := tokens.Token(context.Background())
		if !errors.Is(err, ErrToken) || strings.Contains(fmt.Sprint(err), "s3cret") || strings.Contains(fmt.Sprint(err), "nope") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := (&ClientCredentials{TokenURL: f.server.URL + "/token", ClientID: "alerts-platform", ClientSecret: "nope"}).Token(context.Background()); !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("OAuth error not reported: %v", err)
	}
}

func TestNotify(t *testing.T) {
	f := newFakePortal(t)
	replay := false
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusAccepted
		if replay {
			status = http.StatusOK
		}
		writeData(w, status, map[string]any{"id": "n-1", "applicationId": "alerts", "channel": "email", "status": "PENDING", "dispatchStatus": "PENDING", "replayed": replay})
	}
	c := f.client(t)
	n := Notification{Title: "New alert in Gulu", Message: "A suspected case was reported.", Recipient: Recipient{Email: "dho@example.org", Name: "DHO"}}
	got, err := c.Notify(context.Background(), "alert-42:notify", n)
	if err != nil || got.ID != "n-1" || got.Status != "PENDING" || got.Replayed {
		t.Fatalf("notify = %+v, %v", got, err)
	}
	req, body := f.last()
	var sent Notification
	if req.Method != http.MethodPost || req.URL.Path != "/api/v1/platform/notifications" || req.Header.Get("Idempotency-Key") != "alert-42:notify" ||
		req.Header.Get("Content-Type") != "application/json" || json.Unmarshal([]byte(body), &sent) != nil || sent != n {
		t.Fatalf("request = %s %s %v %s", req.Method, req.URL.Path, req.Header, body)
	}
	replay = true
	if got, err := c.Notify(context.Background(), "alert-42:notify", n); err != nil || !got.Replayed {
		t.Fatalf("replay = %+v, %v", got, err)
	}
}

func TestNotifyValidatesBeforeSending(t *testing.T) {
	f := newFakePortal(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) { t.Error("an invalid notification was sent") }
	c := f.client(t)
	good := Notification{Title: "t", Message: "m", Recipient: Recipient{Email: "a@example.org"}}
	for name, tc := range map[string]struct {
		key string
		n   Notification
	}{
		"empty key":          {"", good},
		"key with a space":   {"bad key", good},
		"key too long":       {strings.Repeat("k", 129), good},
		"no title":           {"k", Notification{Message: "m", Recipient: good.Recipient}},
		"title line break":   {"k", Notification{Title: "a\nb", Message: "m", Recipient: good.Recipient}},
		"long title":         {"k", Notification{Title: strings.Repeat("t", 201), Message: "m", Recipient: good.Recipient}},
		"long message":       {"k", Notification{Title: "t", Message: strings.Repeat("m", 10001), Recipient: good.Recipient}},
		"display-name email": {"k", Notification{Title: "t", Message: "m", Recipient: Recipient{Email: "DHO <a@example.org>"}}},
		"no email":           {"k", Notification{Title: "t", Message: "m"}},
	} {
		if _, err := c.Notify(context.Background(), tc.key, tc.n); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRetriesSafeRequestsOnly(t *testing.T) {
	f := newFakePortal(t)
	var calls atomic.Int32
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			writeError(w, 503, "PLATFORM_UNAVAILABLE", "platform dependency unavailable")
			return
		}
		writeData(w, 200, map[string]any{"applicationId": "alerts"})
	}
	c := f.client(t)
	if _, err := c.Identity(context.Background()); err != nil || calls.Load() != 3 {
		t.Fatalf("calls = %d, err = %v", calls.Load(), err)
	}

	calls.Store(-10) // always 503
	_, err := c.Identity(context.Background())
	if !errors.Is(err, ErrUnavailable) || calls.Load() != -7 {
		t.Fatalf("gave up after %d calls: %v", calls.Load()+10, err)
	}

	for _, status := range []int{400, 403, 404, 409} {
		calls.Store(0)
		f.handler = func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			writeError(w, status, "X", "no")
		}
		_, err := c.Identity(context.Background())
		if calls.Load() != 1 || err == nil {
			t.Errorf("HTTP %d retried %d times: %v", status, calls.Load(), err)
		}
	}
}

func TestRateLimitHonoursShortRetryAfterOnly(t *testing.T) {
	f := newFakePortal(t)
	var calls atomic.Int32
	retryAfter := "1"
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", retryAfter)
			writeError(w, 429, "APPLICATION_QUOTA_EXCEEDED", "application notification quota exceeded")
			return
		}
		writeData(w, 202, map[string]any{"id": "n-1"})
	}
	c := f.client(t)
	var slept []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	n := Notification{Title: "t", Message: "m", Recipient: Recipient{Email: "a@example.org"}}
	if _, err := c.Notify(context.Background(), "k1", n); err != nil || len(slept) != 1 || slept[0] < time.Second {
		t.Fatalf("err = %v, slept = %v", err, slept)
	}
	calls.Store(0)
	retryAfter = "60"
	_, err := c.Notify(context.Background(), "k2", n)
	var apiErr *Error
	if !errors.Is(err, ErrRateLimited) || !errors.As(err, &apiErr) || apiErr.RetryAfter != time.Minute || calls.Load() != 1 {
		t.Fatalf("long Retry-After: %v (%d calls)", err, calls.Load())
	}
}

func TestReadinessReportsNotReadyWithoutError(t *testing.T) {
	f := newFakePortal(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		writeData(w, 503, map[string]any{"ready": false, "workers": []any{map[string]any{"name": "email-outbox", "healthy": false}}})
	}
	r, err := f.client(t).Readiness(context.Background())
	if err != nil || r.Ready || len(r.Workers) != 1 || r.Workers[0].Name != "email-outbox" {
		t.Fatalf("readiness = %+v, %v", r, err)
	}
}

func TestAccessUsesTheUsersToken(t *testing.T) {
	f := newFakePortal(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		writeData(w, 200, map[string]any{"applicationId": "alerts", "subject": "u-1", "roles": []string{"alerts_access"}, "permissions": []string{"alerts:read"}})
	}
	c := f.client(t, func(cfg *Config) { cfg.Tokens = nil })
	access, err := c.Access(context.Background(), "user-token", "alerts")
	if err != nil || !access.HasPermission("alerts:read") || access.HasPermission("alerts:write") {
		t.Fatalf("access = %+v, %v", access, err)
	}
	req, _ := f.last()
	if req.URL.Path != "/api/v1/platform/applications/alerts/access" || req.Header.Get("Authorization") != "Bearer user-token" || f.tokens.Load() != 0 {
		t.Fatalf("request = %s %v, machine tokens = %d", req.URL.Path, req.Header, f.tokens.Load())
	}
	f.reject.Store(5)
	if _, err := c.Access(context.Background(), "user-token", "alerts"); !errors.Is(err, ErrUnauthorized) || f.count() != 2 {
		t.Fatalf("a rejected user token was retried: %v (%d requests)", err, f.count())
	}
	if _, err := c.Access(context.Background(), "", "alerts"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty user token: %v", err)
	}
	if _, err := c.Identity(context.Background()); !errors.Is(err, ErrToken) {
		t.Fatalf("machine call without a token source: %v", err)
	}
}

func TestRefusesRedirectsOversizedAndMalformedResponses(t *testing.T) {
	f := newFakePortal(t)
	c := f.client(t)
	var calls atomic.Int32
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "https://elsewhere.example.org/steal", http.StatusFound)
	}
	if _, err := c.Identity(context.Background()); !errors.Is(err, ErrUnexpected) || calls.Load() != 1 {
		t.Fatalf("redirect: %v (%d calls)", err, calls.Load())
	}
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"success":true,"data":"`+strings.Repeat("x", maxResponseBytes)+`"}`)
	}
	if _, err := c.Identity(context.Background()); !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("oversized: %v", err)
	}
	f.handler = func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `not json`) }
	if _, err := c.Identity(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("malformed: %v", err)
	}
}

func TestTimeoutIsReportedAsUnavailable(t *testing.T) {
	f := newFakePortal(t)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}
	c := f.client(t, func(cfg *Config) { cfg.Timeout = 50 * time.Millisecond; cfg.MaxAttempts = 2 })
	start := time.Now()
	_, err := c.Identity(context.Background())
	var apiErr *Error
	if !errors.Is(err, ErrUnavailable) || !errors.As(err, &apiErr) || apiErr.Code != "NETWORK_ERROR" || !strings.Contains(apiErr.Message, "in time") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout not applied")
	}
}

func TestNewValidatesThePortalURL(t *testing.T) {
	for _, raw := range []string{"", "portals.health.go.ug", "http://portals.health.go.ug", "https://portals.health.go.ug/api", "https://user:pw@portals.health.go.ug"} {
		if _, err := New(Config{PortalURL: raw}); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"https://portals.health.go.ug", "https://portals.health.go.ug/", "http://127.0.0.1:8080", "http://localhost:3000"} {
		if _, err := New(Config{PortalURL: raw}); err != nil {
			t.Errorf("refused %q: %v", raw, err)
		}
	}
}

func TestErrorTextHasNoToken(t *testing.T) {
	f := newFakePortal(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) { writeError(w, 403, "APPLICATION_ACCESS_DENIED", "application access denied") }
	_, err := f.client(t).Identity(context.Background())
	if !errors.Is(err, ErrForbidden) || strings.Contains(err.Error(), "machine-") || !strings.Contains(err.Error(), "APPLICATION_ACCESS_DENIED") {
		t.Fatalf("err = %v", err)
	}
}
