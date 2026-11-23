package modulekit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

func serve(t *testing.T, router http.Handler, method, target, body string, request *modulekit.Request) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if request != nil {
		req = req.WithContext(modulekit.WithRequest(req.Context(), *request))
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func caller(permissions ...string) *modulekit.Request {
	return &modulekit.Request{
		Authenticated: true, RequestID: "req-1",
		Access: core.RequestAccess{Identity: core.Identity{Subject: "user-1"}, Permissions: permissions, Systems: []string{"stats"}},
	}
}

func TestRouterEnforcesDeclaredAccessWithHostResponses(t *testing.T) {
	router, err := modulekit.NewRouter([]modulekit.Endpoint{
		{Method: "GET", Path: "/items/{id}", Permission: "items:read", Handle: func(c *modulekit.Context) {
			c.OK(http.StatusOK, map[string]string{"id": c.Param("id"), "by": c.Subject()})
		}},
		{Method: "GET", Path: "/public", Public: true, Handle: func(c *modulekit.Context) { c.OK(http.StatusOK, "hi") }},
		{Method: "POST", Path: "/items", AnyPermissions: []string{"items:write", "items:admin"}, Handle: func(c *modulekit.Context) { c.Status(http.StatusNoContent) }},
		{Method: "DELETE", Path: "/items", AllPermissions: []string{"items:write", "items:delete"}, Handle: func(c *modulekit.Context) { c.Status(http.StatusNoContent) }},
		{Method: "GET", Path: "/admin", Admin: true, Handle: func(c *modulekit.Context) { c.Status(http.StatusNoContent) }},
		{Method: "GET", Path: "/system", System: "other", Handle: func(c *modulekit.Context) { c.Status(http.StatusNoContent) }},
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method, target string
		request        *modulekit.Request
		status         int
		body           string
	}{
		{"GET", "/items/7", nil, 401, `{"success":false,"error":{"code":"UNAUTHORIZED","message":"unauthorized"},"meta":{}}`},
		{"GET", "/items/7", caller(), 403, `{"success":false,"error":{"code":"FORBIDDEN","message":"permission required: items:read"},"meta":{"requestId":"req-1"}}`},
		{"GET", "/items/7", caller("items:read"), 200, `{"success":true,"data":{"by":"user-1","id":"7"},"meta":{"requestId":"req-1"}}`},
		{"GET", "/public", nil, 200, `{"success":true,"data":"hi","meta":{}}`},
		{"POST", "/items", caller("items:read"), 403, `{"success":false,"error":{"code":"FORBIDDEN","message":"one of these permissions is required: items:write, items:admin"},"meta":{"requestId":"req-1"}}`},
		{"POST", "/items", caller("items:admin"), 204, ``},
		{"DELETE", "/items", caller("items:write"), 403, `{"success":false,"error":{"code":"FORBIDDEN","message":"missing required permissions: items:delete"},"meta":{"requestId":"req-1"}}`},
		{"GET", "/admin", caller(), 403, `{"success":false,"error":{"code":"FORBIDDEN","message":"admin access required"},"meta":{"requestId":"req-1"}}`},
		{"GET", "/system", caller(), 403, `{"success":false,"error":{"code":"FORBIDDEN","message":"system access required: other"},"meta":{"requestId":"req-1"}}`},
	}
	for _, tc := range cases {
		got := serve(t, router, tc.method, tc.target, "", tc.request)
		if got.Code != tc.status || got.Body.String() != tc.body {
			t.Errorf("%s %s: got %d %s, want %d %s", tc.method, tc.target, got.Code, got.Body.String(), tc.status, tc.body)
		}
	}
}

func TestRouterAppliesHostRateLimiterAfterAuthorization(t *testing.T) {
	calls := 0
	limiter := func(r *http.Request, limit modulekit.RateLimit) (bool, http.Header) {
		calls++
		headers := http.Header{}
		headers.Set("X-RateLimit-Policy", limit.Name)
		return calls == 1, headers
	}
	router, err := modulekit.NewRouter([]modulekit.Endpoint{{
		Method: "POST", Path: "/x", Permission: "x:write",
		RateLimit: &modulekit.RateLimit{Name: "x-write", Limit: 1, Window: time.Minute},
		Handle:    func(c *modulekit.Context) { c.Status(http.StatusAccepted) },
	}}, modulekit.WithLimiter(limiter))
	if err != nil {
		t.Fatal(err)
	}
	if got := serve(t, router, "POST", "/x", "", caller()); got.Code != 403 || calls != 0 {
		t.Fatalf("unauthorized request consumed budget: %d calls=%d", got.Code, calls)
	}
	if got := serve(t, router, "POST", "/x", "", caller("x:write")); got.Code != 202 || got.Header().Get("X-RateLimit-Policy") != "x-write" {
		t.Fatalf("first request: %d %v", got.Code, got.Header())
	}
	got := serve(t, router, "POST", "/x", "", caller("x:write"))
	if got.Code != 429 || !strings.Contains(got.Body.String(), "RATE_LIMIT_EXCEEDED") {
		t.Fatalf("second request: %d %s", got.Code, got.Body.String())
	}
}

func TestRouterRejectsInvalidDeclarations(t *testing.T) {
	handle := func(*modulekit.Context) {}
	for name, endpoints := range map[string][]modulekit.Endpoint{
		"empty":        nil,
		"no handler":   {{Method: "GET", Path: "/a"}},
		"bad path":     {{Method: "GET", Path: "a", Handle: handle}},
		"duplicate":    {{Method: "GET", Path: "/a", Handle: handle}, {Method: "get", Path: "/a", Handle: handle}},
		"public+perms": {{Method: "GET", Path: "/a", Public: true, Permission: "p", Handle: handle}},
	} {
		if _, err := modulekit.NewRouter(endpoints); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestBindJSONAppliesBindingRules(t *testing.T) {
	type nested struct {
		Code string `binding:"required"`
	}
	type payload struct {
		Name   string   `json:"name" binding:"required"`
		Kind   string   `json:"kind" binding:"omitempty,oneof=a b"`
		Tags   []string `json:"tags" binding:"required"`
		Nested *nested  `json:"nested"`
	}
	cases := map[string]bool{
		`{"name":"n","tags":[]}`:                       true,
		`{"name":"n","tags":[],"kind":"b"}`:            true,
		`{"name":"","tags":[]}`:                        false,
		`{"name":"n"}`:                                 false,
		`{"name":"n","tags":[],"kind":"c"}`:            false,
		`{"name":"n","tags":[],"nested":{"Code":""}}`:  false,
		`{"name":"n","tags":[],"nested":{"Code":"x"}}`: true,
		``:         false,
		`not json`: false,
	}
	for body, ok := range cases {
		var target payload
		err := modulekit.BindJSON(httptest.NewRequest("POST", "/", strings.NewReader(body)), &target)
		if (err == nil) != ok {
			t.Errorf("%q: err=%v, want ok=%v", body, err, ok)
		}
	}
	type unsupported struct {
		Email string `binding:"email"`
	}
	if err := modulekit.Validate(unsupported{Email: "x"}); err == nil {
		t.Error("unsupported rule accepted")
	}
}

func TestHTTPModuleRequiresMatchingOpenAPI(t *testing.T) {
	router, err := modulekit.NewRouter([]modulekit.Endpoint{{Method: "GET", Path: "/a/{id}", Permission: "a:read", Handle: func(*modulekit.Context) {}}})
	if err != nil {
		t.Fatal(err)
	}
	good := []byte(`{"paths":{"/a/{id}":{"get":{"tags":["A"]},"parameters":[]}}}`)
	if _, err := modulekit.NewHTTPModule(router, good); err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{`{"paths":{}}`, `{"paths":{"/a/{id}":{"get":{}},"/b":{"post":{}}}}`, `nope`} {
		if _, err := modulekit.NewHTTPModule(router, []byte(document)); err == nil {
			t.Errorf("accepted %s", document)
		}
	}
	module, _ := modulekit.NewHTTPModule(router, good)
	var parsed map[string]any
	if err := json.Unmarshal(module.OpenAPI(), &parsed); err != nil || len(module.Patterns()) != 1 {
		t.Fatal("module did not expose its contract")
	}
}
