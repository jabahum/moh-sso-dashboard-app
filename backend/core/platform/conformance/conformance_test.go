package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakePortal implements the platform API rules for a handful of fixed tokens.
type fakePortal struct {
	mu            sync.Mutex
	bindingOn     bool
	appActive     bool
	notifications bool
	keys          map[string]string // idempotency key -> body
	breakage      string            // a deliberate defect, see the tests
}

var allScopes = []string{"identity:read", "notifications:read", "notifications:write"}

func (f *fakePortal) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	reply := func(status int, data any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data, "meta": map[string]any{"requestId": "r"}})
	}
	fail := func(status int, code string) {
		if status == 401 && f.breakage != "no-challenge" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="moh-platform"`)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"code": code, "message": "m"}, "meta": map[string]any{}})
	}
	if r.URL.Path == "/api/v1/platform/notifications" && !f.notifications || strings.HasPrefix(r.URL.Path, "/api/v1/platform/notifications/") && !f.notifications {
		http.NotFound(w, r)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var scopes []string
	switch token {
	case "bound":
		scopes = allScopes
	case "narrow":
		scopes = []string{"identity:read", "notifications:read"}
	case "unbound":
		fail(403, "APPLICATION_ACCESS_DENIED")
		return
	case "user":
		if strings.HasSuffix(r.URL.Path, "/access") {
			reply(200, map[string]any{"applicationId": "alerts", "subject": "u-1", "roles": []string{"alerts_access"}, "permissions": []string{}})
			return
		}
		fail(403, "APPLICATION_ACCESS_DENIED")
		return
	case "wrong-audience":
		if f.breakage == "accepts-wrong-audience" {
			scopes = allScopes
			break
		}
		fail(401, "INVALID_IDENTITY")
		return
	default:
		fail(401, "INVALID_IDENTITY")
		return
	}
	if strings.HasSuffix(r.URL.Path, "/access") {
		fail(403, "APPLICATION_ACCESS_DENIED")
		return
	}
	if !f.bindingOn && f.breakage != "ignores-binding" || !f.appActive {
		fail(403, "APPLICATION_ACCESS_DENIED")
		return
	}
	has := func(s string) bool {
		for _, x := range scopes {
			if x == s {
				return true
			}
		}
		return false
	}
	switch {
	case r.URL.Path == "/api/v1/platform/identity":
		data := map[string]any{"applicationId": "alerts", "clientId": "alerts-platform", "subject": "sa-1", "scopes": scopes}
		if f.breakage == "missing-subject" {
			delete(data, "subject")
		}
		reply(200, data)
	case r.URL.Path == "/api/v1/platform/readiness":
		reply(200, map[string]any{"ready": true, "workers": []any{map[string]any{"name": "email-outbox", "healthy": true}}})
	case r.URL.Path == "/api/v1/platform/notifications" && r.Method == "POST":
		if !has("notifications:write") {
			fail(403, "APPLICATION_ACCESS_DENIED")
			return
		}
		body, _ := io.ReadAll(r.Body)
		key := r.Header.Get("Idempotency-Key")
		var n struct {
			Title, Message string
		}
		if key == "" || json.Unmarshal(body, &n) != nil || n.Message == "" || strings.Contains(n.Title, "\n") {
			fail(400, "INVALID_REQUEST")
			return
		}
		status := map[string]any{"id": "11111111-1111-4111-8111-111111111111", "applicationId": "alerts", "channel": "email", "status": "PENDING",
			"dispatchStatus": "PENDING", "dispatchAttempts": 0, "deliveryAttempts": 0, "createdAt": "2026-11-28T00:00:00Z"}
		w.Header().Set("Location", "/api/v1/platform/notifications/"+status["id"].(string))
		if previous, ok := f.keys[key]; ok {
			if previous != string(body) {
				fail(409, "IDEMPOTENCY_CONFLICT")
				return
			}
			status["replayed"] = true
			reply(200, status)
			return
		}
		f.keys[key] = string(body)
		status["replayed"] = false
		reply(202, status)
	case strings.HasPrefix(r.URL.Path, "/api/v1/platform/notifications/"):
		if strings.HasSuffix(r.URL.Path, "11111111-1111-4111-8111-111111111111") {
			reply(200, map[string]any{"id": "11111111-1111-4111-8111-111111111111", "applicationId": "alerts", "channel": "email", "status": "PENDING",
				"dispatchStatus": "PENDING", "dispatchAttempts": 0, "deliveryAttempts": 0, "createdAt": "2026-11-28T00:00:00Z", "replayed": false})
			return
		}
		fail(404, "NOT_FOUND")
	default:
		http.NotFound(w, r)
	}
}

func run(t *testing.T, f *fakePortal, mutate ...func(*Config)) Report {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	cfg := Config{
		PortalURL: server.URL,
		AllowHTTP: true,
		Tokens: Tokens{
			Bound: StaticToken("bound"), Narrow: StaticToken("narrow"), Unbound: StaticToken("unbound"),
			WrongAudience: StaticToken("wrong-audience"), WrongIssuer: StaticToken("other-issuer"), User: StaticToken("user"),
		},
		Expect: Expect{Application: "alerts", ClientID: "alerts-platform", Scopes: allScopes, NotificationRecipient: "inbox@conformance.test"},
		Controls: Controls{
			SetBindingEnabled:    func(_ context.Context, on bool) error { f.mu.Lock(); f.bindingOn = on; f.mu.Unlock(); return nil },
			SetApplicationActive: func(_ context.Context, on bool) error { f.mu.Lock(); f.appActive = on; f.mu.Unlock(); return nil },
		},
	}
	for _, m := range mutate {
		m(&cfg)
	}
	report, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func newFake() *fakePortal {
	return &fakePortal{bindingOn: true, appActive: true, notifications: true, keys: map[string]string{}}
}

func statusOf(t *testing.T, r Report, id string) Result {
	t.Helper()
	res, ok := r.Result(id)
	if !ok {
		t.Fatalf("no result %s", id)
	}
	return res
}

func TestConformingPortalPassesEveryCheck(t *testing.T) {
	report := run(t, newFake())
	pass, fail, skip := report.Counts()
	if fail != 0 || skip != 0 || pass != 21 {
		for _, res := range report.Results {
			if res.Status != Pass {
				t.Logf("%s %s: %s", res.ID, res.Status, res.Detail)
			}
		}
		t.Fatalf("pass=%d fail=%d skip=%d", pass, fail, skip)
	}
	ids := map[string]bool{}
	for _, res := range report.Results {
		if ids[res.ID] {
			t.Fatalf("duplicate check ID %s", res.ID)
		}
		ids[res.ID] = true
	}
}

func TestMissingInputsAreSkippedNotPassed(t *testing.T) {
	f := newFake()
	f.notifications = false
	report := run(t, f, func(c *Config) {
		c.Tokens = Tokens{Bound: c.Tokens.Bound}
		c.Controls = Controls{}
		c.Expect.NotificationRecipient = ""
	})
	if report.Failed() {
		t.Fatalf("failed: %+v", report.Results)
	}
	for _, id := range []string{IDWrongAudience, IDWrongIssuer, IDUnbound, IDNarrowScopes, IDAccessUser, IDNotifyValidation, IDNotifySubmit, IDRevokeBinding, IDRevokeSuspended} {
		if res := statusOf(t, report, id); res.Status != Skip || res.Detail == "" {
			t.Errorf("%s: %+v, want skipped with a reason", id, res)
		}
	}
	if statusOf(t, report, IDNotifyValidation).Detail != "the portal does not serve notifications (delivery disabled)" {
		t.Fatal("notifications not reported as unserved")
	}
}

func TestDefectsFailTheRightCheck(t *testing.T) {
	for _, tc := range []struct{ breakage, id string }{
		{"accepts-wrong-audience", IDWrongAudience},
		{"no-challenge", IDNoToken},
		{"missing-subject", IDIdentity},
		{"ignores-binding", IDRevokeBinding},
	} {
		f := newFake()
		f.breakage = tc.breakage
		report := run(t, f)
		if res := statusOf(t, report, tc.id); res.Status != Fail {
			t.Errorf("%s: %s is %s, want fail", tc.breakage, tc.id, res.Status)
		}
	}
}

func TestRevocationControlsAreRestoredAndErrorsReported(t *testing.T) {
	f := newFake()
	report := run(t, f, func(c *Config) {
		c.Controls.SetApplicationActive = func(context.Context, bool) error { return errors.New("database unavailable") }
	})
	if res := statusOf(t, report, IDRevokeSuspended); res.Status != Fail || !strings.Contains(res.Detail, "database unavailable") {
		t.Fatalf("%+v", res)
	}
	if !f.bindingOn || !f.appActive {
		t.Fatal("the portal was left changed")
	}
}

func TestRunRejectsUnusableConfig(t *testing.T) {
	for name, cfg := range map[string]Config{
		"http without AllowHTTP": {PortalURL: "http://portal.test", Tokens: Tokens{Bound: StaticToken("x")}, Expect: Expect{Application: "a", ClientID: "c"}},
		"path":                   {PortalURL: "https://portal.test/api", Tokens: Tokens{Bound: StaticToken("x")}, Expect: Expect{Application: "a", ClientID: "c"}},
		"no bound token":         {PortalURL: "https://portal.test", Expect: Expect{Application: "a", ClientID: "c"}},
		"no expectations":        {PortalURL: "https://portal.test", Tokens: Tokens{Bound: StaticToken("x")}},
	} {
		if _, err := Run(context.Background(), cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestShapeProblems(t *testing.T) {
	ok := map[string]any{"applicationId": "a", "clientId": "c", "subject": "s", "scopes": []any{"identity:read"}, "extra": 1.0}
	if p := ShapeProblems("platform.Principal", ok); len(p) != 0 {
		t.Fatalf("unknown fields must be allowed: %v", p)
	}
	bad := map[string]any{"applicationId": 7.0, "clientId": "c", "scopes": nil}
	if p := ShapeProblems("platform.Principal", bad); len(p) != 3 {
		t.Fatalf("problems = %v, want wrong type, missing and null", p)
	}
	if p := ShapeProblems("platform.NotificationStatus", map[string]any{"dispatchAttempts": 1.5}); !strings.Contains(strings.Join(p, ";"), "number, want integer") {
		t.Fatalf("%v", p)
	}
}
