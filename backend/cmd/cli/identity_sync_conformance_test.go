package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter/memory"
)

const conformanceTestIssuer = "https://sso.example.test/realms/moh"

// startConformanceApp serves the core adapter over TLS with synthetic accounts
// and returns its origin and a PEM file trusting its certificate.
func startConformanceApp(t *testing.T) (origin, caFile string) {
	t.Helper()
	store := memory.New(conformanceTestIssuer, identitysync.AccountIDPositiveInteger)
	for _, id := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"} {
		store.Add(id, "", true)
	}
	store.Add("99", "", false)
	good := adapter.Claims{Issuer: conformanceTestIssuer, Audience: []string{"demo"}, AuthorizedParty: "demo-identity-sync",
		Type: "Bearer", ExpiresAt: time.Now().Add(time.Hour), ResourceAccess: map[string][]string{"demo": {"identity_sync"}}}
	h, err := adapter.New(adapter.Config{
		Application: "demo", Issuer: conformanceTestIssuer, SyncClientID: "demo-identity-sync",
		AccountIDFormat: identitysync.AccountIDPositiveInteger, Store: store, ProbeEnabled: true, Probe: store,
		Verifier: adapter.TokenVerifierFunc(func(_ context.Context, raw string) (adapter.Claims, error) {
			if raw == "synthetic-sync-token" {
				return good, nil
			}
			return adapter.Claims{}, errors.New("unverifiable")
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	caFile = filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	return srv.URL, caFile
}

func writeFixtures(t *testing.T, application string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixtures.json")
	raw, _ := json.Marshal(map[string]any{
		"application": application, "issuer": conformanceTestIssuer,
		"linkableAccounts": []string{"2", "8"}, "ineligibleAccount": "99",
	})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runConformanceCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newIdentitySyncConformanceCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestIdentitySyncConformanceCommandPassesAgainstAdapter(t *testing.T) {
	origin, caFile := startConformanceApp(t)
	t.Setenv("IDENTITY_SYNC_TOKEN", "synthetic-sync-token")
	t.Setenv("IDENTITY_SYNC_EXPIRED_TOKEN", "synthetic-expired-token")
	out, err := runConformanceCommand(t, "--origin", origin, "--fixtures", writeFixtures(t, "demo"), "--ca-file", caFile, "--probe")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "PASS  sessions.revoked-session-rejected") || !strings.Contains(out, " 0 failed") ||
		!strings.Contains(out, "SKIP  authorization.user-token") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if strings.Contains(out, "synthetic-sync-token") {
		t.Fatal("output contains a token")
	}
}

func TestIdentitySyncConformanceCommandReportsFailures(t *testing.T) {
	origin, caFile := startConformanceApp(t)
	t.Setenv("IDENTITY_SYNC_TOKEN", "synthetic-sync-token")
	out, err := runConformanceCommand(t, "--base-url", origin+identitySyncBasePath, "--fixtures", writeFixtures(t, "demo"),
		"--ca-file", caFile, "--app", "other-app", "--json")
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("expected failure, got %v", err)
	}
	if !strings.Contains(out, `"id": "descriptor.application"`) || !strings.Contains(out, `"status": "fail"`) {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestIdentitySyncConformanceCommandValidatesInputs(t *testing.T) {
	t.Setenv("IDENTITY_SYNC_TOKEN", "")
	fixtures := writeFixtures(t, "demo")
	cases := [][]string{
		{"--fixtures", fixtures},
		{"--origin", "https://app.example.test", "--base-url", "https://app.example.test/x", "--fixtures", fixtures},
		{"--origin", "https://app.example.test/path", "--fixtures", fixtures},
		{"--origin", "https://app.example.test", "--fixtures", filepath.Join(t.TempDir(), "missing.json")},
		{"--origin", "https://app.example.test", "--fixtures", fixtures},
	}
	for i, args := range cases {
		if _, err := runConformanceCommand(t, args...); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}
