package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/platform/conformance"
)

func TestPlatformConformanceCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/token":
			_ = r.ParseForm()
			if r.PostForm.Get("client_secret") != "fixture-secret" {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"bound","token_type":"Bearer","expires_in":300}`))
		case r.Header.Get("Authorization") != "Bearer bound":
			w.Header().Set("WWW-Authenticate", `Bearer realm="moh-platform"`)
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"INVALID_IDENTITY","message":"m"}}`))
		case r.URL.Path == "/api/v1/platform/identity":
			_, _ = w.Write([]byte(`{"success":true,"data":{"applicationId":"alerts","clientId":"alerts-platform","subject":"sa","scopes":["identity:read"]}}`))
		case r.URL.Path == "/api/v1/platform/readiness":
			_, _ = w.Write([]byte(`{"success":true,"data":{"ready":true,"workers":[]}}`))
		case strings.HasSuffix(r.URL.Path, "/access"):
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"APPLICATION_ACCESS_DENIED","message":"m"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("PLATFORM_CLIENT_SECRET", "fixture-secret")
	args := []string{"conformance", "--portal", server.URL, "--token-url", server.URL + "/token", "--client-id", "alerts-platform",
		"--application", "alerts", "--scopes", "identity:read", "--allow-http", "--json"}
	stdout, _, err := runCommand(t, newPlatformCommand(), args...)
	if err != nil {
		t.Fatalf("err = %v\n%s", err, stdout)
	}
	var report conformance.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || report.Failed() {
		t.Fatalf("report = %s (%v)", stdout, err)
	}
	if res, _ := report.Result(conformance.IDIdentity); res.Status != conformance.Pass {
		t.Fatalf("identity: %+v", res)
	}
	if strings.Contains(stdout, "fixture-secret") {
		t.Fatal("secret printed")
	}

	t.Setenv("PLATFORM_CLIENT_SECRET", "")
	if _, _, err := runCommand(t, newPlatformCommand(), args...); err == nil || !strings.Contains(err.Error(), "PLATFORM_CLIENT_SECRET") {
		t.Fatalf("missing secret: %v", err)
	}
}
