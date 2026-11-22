package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

type memoryRegistry struct {
	rows map[string]identityreconcile.Integration
}

func (m *memoryRegistry) GetIntegration(_ context.Context, app string) (identityreconcile.Integration, error) {
	row, ok := m.rows[app]
	if !ok {
		return identityreconcile.Integration{}, identityreconcile.ErrIntegrationNotFound
	}
	return row, nil
}

func (m *memoryRegistry) SaveIntegration(_ context.Context, i identityreconcile.Integration, actor, _ string) (identityreconcile.Integration, error) {
	i.UpdatedBy = actor
	m.rows[i.ApplicationClientID] = i
	return i, nil
}

func (m *memoryRegistry) DeleteIntegration(_ context.Context, app, _, _ string) error {
	delete(m.rows, app)
	return nil
}

func (m *memoryRegistry) CheckIntegrationBinding(context.Context, identityreconcile.Integration) error {
	return nil
}

// fakeIdentitySyncRuntime serves a Keycloak token endpoint and a 1.1 application
// (with no eligible accounts) on one TLS server and installs a runtime using it.
func fakeIdentitySyncRuntime(t *testing.T, rows map[string]identityreconcile.Integration) (*memoryRegistry, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/moh/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		if _, secret, ok := r.BasicAuth(); !ok || secret != "fixture-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"access_token":"minted","expires_in":300,"token_type":"Bearer"}`)
	})
	mux.HandleFunc("/api/v1/internal/identity-sync", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"contract":"moh-identity-sync","version":"1.1","application":"alerts","accountIdFormat":"opaque","maxPageSize":100,"capabilities":["accounts.list","identity.link","sessions.revoke"]}`)
	})
	mux.HandleFunc("/api/v1/internal/identity-sync/accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") == "" {
			t.Error("1.1 application did not receive limit")
		}
		_, _ = io.WriteString(w, `{"accounts":[],"nextCursor":""}`)
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	for app, row := range rows {
		if row.Origin == "" {
			row.Origin = server.URL
		}
		rows[app] = row
	}
	repo := &memoryRegistry{rows: rows}
	env := map[string]string{
		"KEYCLOAK_BASE_URL": server.URL, "KEYCLOAK_REALM": "moh", "KEYCLOAK_EXTERNAL_URL": "https://sso.example.test",
		"KEYCLOAK_ADMIN_CLIENT_ID": "operator", "KEYCLOAK_ADMIN_CLIENT_SECRET": "operator-secret",
		"ALERTS_SYNC_CLIENT_SECRET": "fixture-secret",
	}
	getenv := func(k string) string { return env[k] }
	previous := openIdentitySyncRuntime
	openIdentitySyncRuntime = func(context.Context) (*identitySyncRuntime, error) {
		registry := identityreconcile.NewRegistryService(repo, server.URL+"/realms/moh/protocol/openid-connect/token")
		registry.HTTPClient = server.Client()
		registry.Getenv = getenv
		registry.Sleeper = func(context.Context, time.Duration) error { return nil }
		return &identitySyncRuntime{Registry: registry, getenv: getenv}, nil
	}
	t.Cleanup(func() { openIdentitySyncRuntime = previous })
	// The fake runtime has no database; nothing is revoked unless a test says so.
	previousRevocations := identitySyncRevocations
	identitySyncRevocations = func(context.Context, *identitySyncRuntime) (identityreconcile.RevocationChecker, error) {
		return identityreconcile.NoRevocations{}, nil
	}
	t.Cleanup(func() { identitySyncRevocations = previousRevocations })
	return repo, server.URL
}

func alertsIntegration(enabled bool) identityreconcile.Integration {
	return identityreconcile.Integration{
		ApplicationClientID: "alerts", SyncClientID: "alerts-identity-sync", AccessRole: "alerts_access",
		CredentialRef: "env:ALERTS_SYNC_CLIENT_SECRET", AccountIDFormat: identitysync.AccountIDOpaque, Enabled: enabled,
	}
}

func runCommand(t *testing.T, cmd *cobra.Command, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	cmd.SetContext(context.Background())
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestIdentitySyncParentCommandHasSubcommandsAndAcceptsRegistrations(t *testing.T) {
	previous := identitySyncSubcommands
	t.Cleanup(func() { identitySyncSubcommands = previous })
	registerIdentitySyncSubcommand(func() *cobra.Command { return &cobra.Command{Use: "run"} })
	cmd := newIdentitySyncCommand()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"register", "describe", "preview", "apply", "run"} {
		if !names[want] {
			t.Errorf("missing subcommand %q (have %v)", want, names)
		}
	}
	cmd.AddCommand(&cobra.Command{Use: "extra"})
	if found, _, err := cmd.Find([]string{"extra"}); err != nil || found.Name() != "extra" {
		t.Fatal("AddCommand on the parent did not work")
	}
}

func TestIdentitySyncRegisterValidatesAndEnables(t *testing.T) {
	repo, origin := fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{})
	base := []string{"register", "--app", "alerts", "--origin", origin, "--sync-client", "alerts-identity-sync",
		"--access-role", "alerts_access", "--credential", "env:ALERTS_SYNC_CLIENT_SECRET"}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), base...); err == nil {
		t.Fatal("register without --reason succeeded")
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), append(base[:3:3], "--origin", "http://alerts.example.test", "--sync-client", "s", "--access-role", "r", "--credential", "env:X", "--reason", "x")...); err == nil {
		t.Fatal("http origin accepted")
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), append(append([]string{}, base...), "--credential", "inline-secret", "--reason", "x")...); err == nil {
		t.Fatal("inline secret accepted as credential reference")
	}
	stdout, _, err := runCommand(t, newIdentitySyncCommand(), append(append([]string{}, base...), "--enable", "--reason", "onboard alerts")...)
	if err != nil {
		t.Fatal(err)
	}
	row := repo.rows["alerts"]
	if !row.Enabled || row.ContractVersion != "1.1" || !strings.HasPrefix(row.UpdatedBy, "cli") || !strings.Contains(stdout, `"contractVersion": "1.1"`) {
		t.Fatalf("row = %+v, stdout = %s", row, stdout)
	}
	if strings.Contains(stdout, "fixture-secret") {
		t.Fatal("secret printed")
	}
}

func TestIdentitySyncDescribeAndPreviewUseTheRegistry(t *testing.T) {
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"alerts": alertsIntegration(true)})
	stdout, _, err := runCommand(t, newIdentitySyncCommand(), "describe", "--app", "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"version": "1.1"`) || !strings.Contains(stdout, `"credentialRef": "env:ALERTS_SYNC_CLIENT_SECRET"`) || strings.Contains(stdout, "fixture-secret") {
		t.Fatalf("describe = %s", stdout)
	}
	stdout, stderr, err := runCommand(t, newIdentitySyncCommand(), "preview", "--app", "alerts", "--limit", "10")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"Decisions":null`) || !strings.Contains(stderr, "https://sso.example.test/realms/moh") {
		t.Fatalf("preview stdout = %s, stderr = %s", stdout, stderr)
	}
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "preview", "--app", "unknown"); !errors.Is(err, identityreconcile.ErrIntegrationNotFound) {
		t.Fatalf("unknown app: %v", err)
	}
}

func TestIdentitySyncPreviewRefusesDisabledIntegration(t *testing.T) {
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"alerts": alertsIntegration(false)})
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "preview", "--app", "alerts"); !errors.Is(err, identityreconcile.ErrIntegrationDisabled) {
		t.Fatalf("err = %v", err)
	}
	// describe still works for a disabled entry.
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "describe", "--app", "alerts"); err != nil {
		t.Fatal(err)
	}
}

func TestIdentitySyncApplyRequiresFlagsAndEligibleAccount(t *testing.T) {
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"alerts": alertsIntegration(true)})
	if _, _, err := runCommand(t, newIdentitySyncCommand(), "apply", "--app", "alerts", "--account", "a"); err == nil {
		t.Fatal("apply without --approved-subject succeeded")
	}
	_, _, err := runCommand(t, newIdentitySyncCommand(), "apply", "--app", "alerts", "--account", "a", "--approved-subject", "kc-1")
	if err == nil || !strings.Contains(err.Error(), "not eligible") {
		t.Fatalf("err = %v", err)
	}
}

func TestDeprecatedAliasFallsBackToEnvironmentWithoutRegistry(t *testing.T) {
	previous := openIdentitySyncRuntime
	t.Cleanup(func() { openIdentitySyncRuntime = previous })
	openIdentitySyncRuntime = func(context.Context) (*identitySyncRuntime, error) {
		return nil, errors.New("no database")
	}
	for _, env := range []string{"KEYCLOAK_BASE_URL", "KEYCLOAK_REALM", "KEYCLOAK_ADMIN_CLIENT_ID", "KEYCLOAK_ADMIN_CLIENT_SECRET"} {
		t.Setenv(env, "")
	}
	_, stderr, err := runCommand(t, newCLIMSSyncCommand())
	if err == nil || !strings.Contains(err.Error(), "configure the HTTPS Keycloak base URL") {
		t.Fatalf("err = %v, want the environment configuration error", err)
	}
	if !strings.Contains(stderr, "clims-identity-sync is deprecated") || !strings.Contains(stderr, "identity-sync preview|apply --app clims") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestDeprecatedAliasUsesEnabledRegistryEntry(t *testing.T) {
	row := alertsIntegration(true)
	row.ApplicationClientID = "poesgo"
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"poesgo": row})
	// The fake application names itself "alerts", so the registry path is
	// proven by the descriptor mismatch it reports (the env path would fail on
	// missing configuration instead).
	_, stderr, err := runCommand(t, newPOESSyncCommand())
	if err == nil || !strings.Contains(err.Error(), "registry expects \"poesgo\"") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stderr, "registry entry for poesgo") || !strings.Contains(stderr, "poes-identity-sync is deprecated") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestDeprecatedAliasIgnoresDisabledRegistryEntry(t *testing.T) {
	row := alertsIntegration(false)
	row.ApplicationClientID = "clims"
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"clims": row})
	for _, env := range []string{"KEYCLOAK_BASE_URL", "KEYCLOAK_REALM"} {
		t.Setenv(env, "")
	}
	_, stderr, err := runCommand(t, newCLIMSSyncCommand())
	if err == nil || !strings.Contains(err.Error(), "configure the HTTPS Keycloak base URL") || strings.Contains(stderr, "registry entry") {
		t.Fatalf("err = %v, stderr = %q", err, stderr)
	}
}

type pagedAccounts struct {
	pages map[string]identitysync.AccountPage
	calls int
}

func (p *pagedAccounts) ListEligibleAccounts(_ context.Context, cursor string, limit int) (identitysync.AccountPage, error) {
	p.calls++
	if limit != identitysync.MaxPageSize {
		return identitysync.AccountPage{}, errors.New("unexpected limit")
	}
	return p.pages[cursor], nil
}

func (p *pagedAccounts) LinkIdentity(context.Context, string, identitysync.Identity) error {
	return nil
}

func TestFindEligibleAccountWalksPages(t *testing.T) {
	accounts := &pagedAccounts{pages: map[string]identitysync.AccountPage{
		"":   {Accounts: []identitysync.Account{{ID: "a"}}, NextCursor: "c1"},
		"c1": {Accounts: []identitysync.Account{{ID: "b", Email: "b@example.org"}}, NextCursor: "c2"},
		"c2": {Accounts: []identitysync.Account{{ID: "c"}}},
	}}
	got, err := findEligibleAccount(context.Background(), accounts, "", "b")
	if err != nil || got.Email != "b@example.org" || accounts.calls != 2 {
		t.Fatalf("got = %+v, calls = %d, err = %v", got, accounts.calls, err)
	}
	if _, err := findEligibleAccount(context.Background(), accounts, "", "z"); err == nil {
		t.Fatal("missing account found")
	}
	loop := &pagedAccounts{pages: map[string]identitysync.AccountPage{"x": {NextCursor: "x"}}}
	if _, err := findEligibleAccount(context.Background(), loop, "x", "z"); err == nil || !strings.Contains(err.Error(), "repeating") {
		t.Fatalf("loop err = %v", err)
	}
}

func TestKeycloakConfigAndTokenEndpointFromEnv(t *testing.T) {
	endpoint, err := tokenEndpointFromEnv(envFrom(baseEnv()))
	if err != nil || endpoint != "https://keycloak.internal:8443/realms/moh/protocol/openid-connect/token" {
		t.Fatalf("endpoint = %q, %v", endpoint, err)
	}
	env := baseEnv()
	env["KEYCLOAK_BASE_URL"] = "http://keycloak.internal"
	if _, err := tokenEndpointFromEnv(envFrom(env)); err == nil {
		t.Fatal("http Keycloak accepted")
	}
	cfg, err := keycloakSyncConfigFromEnv(envFrom(baseEnv()))
	if err != nil || cfg.origin != "" || cfg.syncClientID != "" {
		t.Fatalf("registry config must not read per-app settings: %+v, %v", cfg, err)
	}
}
