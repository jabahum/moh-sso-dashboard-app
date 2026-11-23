package identityreconcile

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

func TestResolveCredential(t *testing.T) {
	env := map[string]string{"SYNC_SECRET": "env-value\n", "EMPTY_SECRET": "", "NEWLINE_ONLY": "\n"}
	getenv := func(k string) string { return env[k] }
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	crlf := write("crlf", "file-value\r\n")
	plain := write("plain", "file value")
	empty := write("empty", "\n")
	internal := write("internal", "line1\nline2\n\n")

	for ref, want := range map[string]string{
		"env:SYNC_SECRET": "env-value",
		"file:" + crlf:    "file-value",
		"file:" + plain:   "file value",
		// Only one trailing newline is trimmed.
		"file:" + internal: "line1\nline2\n",
	} {
		got, err := ResolveCredential(ref, getenv, nil)
		if err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", ref, got, err, want)
		}
	}
	for _, ref := range []string{
		"env:EMPTY_SECRET", "env:NEWLINE_ONLY", "env:UNSET_SECRET", "file:" + empty,
		"file:" + filepath.Join(dir, "missing"), "env:lower", "file:relative/path", "vault:x", "SYNC_SECRET", "",
	} {
		if _, err := ResolveCredential(ref, getenv, nil); err == nil {
			t.Errorf("%q resolved", ref)
		}
	}
}

func TestResolveCredentialErrorsNeverContainTheSecret(t *testing.T) {
	const secret = "s3cr3t-value-that-must-not-leak"
	_, err := ResolveCredential("file:/secret/path", nil, func(string) ([]byte, error) {
		return nil, &fs.PathError{Op: "open", Path: "/secret/path", Err: fs.ErrPermission}
	})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}
	// A secret that resolves is returned, never embedded in an error.
	got, err := ResolveCredential("env:X", func(string) string { return secret }, nil)
	if err != nil || got != secret {
		t.Fatal(err)
	}
}

func validIntegration() Integration {
	return Integration{
		ApplicationClientID: "alerts", Origin: "https://alerts.example.test", SyncClientID: "alerts-identity-sync",
		AccessRole: "alerts_access", CredentialRef: "env:ALERTS_SYNC_CLIENT_SECRET", AccountIDFormat: identitysync.AccountIDOpaque,
	}
}

func TestIntegrationValidate(t *testing.T) {
	if err := validIntegration().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Integration){
		"http origin":       func(i *Integration) { i.Origin = "http://alerts.example.test" },
		"origin with path":  func(i *Integration) { i.Origin = "https://alerts.example.test/app" },
		"origin with slash": func(i *Integration) { i.Origin = "https://alerts.example.test/" },
		"origin with query": func(i *Integration) { i.Origin = "https://alerts.example.test?x=1" },
		"origin userinfo":   func(i *Integration) { i.Origin = "https://u:p@alerts.example.test" },
		"origin bad port":   func(i *Integration) { i.Origin = "https://alerts.example.test:70000" },
		"sync is app":       func(i *Integration) { i.SyncClientID = "alerts" },
		"blank role":        func(i *Integration) { i.AccessRole = " " },
		"credential value":  func(i *Integration) { i.CredentialRef = "plain-secret" },
		"relative file":     func(i *Integration) { i.CredentialRef = "file:secret" },
		"format":            func(i *Integration) { i.AccountIDFormat = "email" },
		"application":       func(i *Integration) { i.ApplicationClientID = "../x" },
	} {
		i := validIntegration()
		mutate(&i)
		if err := i.Validate(); !errors.Is(err, ErrInvalidIntegration) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	i := validIntegration()
	i.Origin = "https://alerts.example.test:8443"
	if err := i.Validate(); err != nil {
		t.Errorf("port rejected: %v", err)
	}
}

type fakeRegistryRepo struct {
	rows       map[string]Integration
	bindingErr error
	saves      []string
	deletes    []string
}

func (f *fakeRegistryRepo) GetIntegration(_ context.Context, app string) (Integration, error) {
	row, ok := f.rows[app]
	if !ok {
		return Integration{}, ErrIntegrationNotFound
	}
	return row, nil
}

func (f *fakeRegistryRepo) SaveIntegration(_ context.Context, i Integration, actor, reason string) (Integration, error) {
	if f.rows == nil {
		f.rows = map[string]Integration{}
	}
	i.UpdatedBy = actor
	f.rows[i.ApplicationClientID] = i
	f.saves = append(f.saves, reason)
	return i, nil
}

func (f *fakeRegistryRepo) DeleteIntegration(_ context.Context, app, _, reason string) error {
	if _, ok := f.rows[app]; !ok {
		return ErrIntegrationNotFound
	}
	delete(f.rows, app)
	f.deletes = append(f.deletes, reason)
	return nil
}

func (f *fakeRegistryRepo) CheckIntegrationBinding(context.Context, Integration) error {
	return f.bindingErr
}

// registryFixture serves a Keycloak token endpoint and an application on one TLS server.
func registryFixture(t *testing.T, app http.HandlerFunc) (*RegistryService, *fakeRegistryRepo, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/moh/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != "alerts-identity-sync" || secret != "fixture-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"minted","expires_in":300,"token_type":"Bearer"}`)
	})
	mux.HandleFunc(syncBase+"/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer minted" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		app(w, r)
	})
	mux.HandleFunc(syncBase, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer minted" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		app(w, r)
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	repo := &fakeRegistryRepo{}
	service := NewRegistryService(repo, server.URL+"/realms/moh/protocol/openid-connect/token")
	service.HTTPClient = server.Client()
	// The TLS test server listens on loopback; allowlist it the way a dev or
	// e2e stack sets IDENTITY_SYNC_ALLOWED_PRIVATE_HOSTS.
	service.AllowedPrivateHosts = []string{"127.0.0.1"}
	service.Getenv = func(k string) string {
		if k == "ALERTS_SYNC_CLIENT_SECRET" {
			return "fixture-secret"
		}
		return ""
	}
	service.Sleeper = func(context.Context, time.Duration) error { return nil }
	return service, repo, server.URL
}

func TestRegistrySaveEnabledProbesDescriptorAndStoresVersion(t *testing.T) {
	service, repo, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == syncBase+"/sessions/revoke" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = io.WriteString(w, descriptorJSON("1.1", "alerts", "opaque", "accounts.list", "identity.link", "sessions.revoke"))
	})
	integration := validIntegration()
	integration.Origin = origin
	integration.Enabled = true
	saved, err := service.Save(context.Background(), integration, "operator", "  onboard alerts  ")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContractVersion != "1.1" || !saved.Enabled || repo.saves[0] != "onboard alerts" {
		t.Fatalf("saved = %+v, reasons = %v", saved, repo.saves)
	}
	connector, got, err := service.Connector(context.Background(), "alerts")
	if err != nil || got.ApplicationClientID != "alerts" {
		t.Fatal(err)
	}
	if err := connector.RevokeSessions(context.Background(), identitysync.Identity{Issuer: testIssuer, Subject: "s"}); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrySaveEnabledAccepts10Application(t *testing.T) {
	service, _, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	integration := validIntegration()
	integration.Origin = origin
	integration.Enabled = true
	saved, err := service.Save(context.Background(), integration, "operator", "legacy app")
	if err != nil || saved.ContractVersion != "1.0" {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
}

func TestRegistrySaveRefusesToEnableUnreachableOrMismatchedApplication(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"unauthorized": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) },
		"other app": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, descriptorJSON("1.1", "clims", "uuid", "accounts.list", "identity.link"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			service, repo, origin := registryFixture(t, handler)
			integration := validIntegration()
			integration.Origin = origin
			integration.Enabled = true
			if _, err := service.Save(context.Background(), integration, "operator", "try"); !errors.Is(err, ErrDescriptorUnavailable) {
				t.Fatal("enabled an application that could not be described")
			}
			if len(repo.saves) != 0 {
				t.Fatal("row written despite failed probe")
			}
		})
	}
}

func TestRegistrySaveDisabledSkipsProbeAndRequiresReason(t *testing.T) {
	service, repo, _ := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("disabled save contacted the application")
	})
	integration := validIntegration()
	if _, err := service.Save(context.Background(), integration, "operator", " "); !errors.Is(err, ErrInvalidIntegration) {
		t.Fatalf("missing reason: %v", err)
	}
	saved, err := service.Save(context.Background(), integration, "operator", "stage")
	if err != nil || saved.Enabled || saved.ContractVersion != "" || len(repo.saves) != 1 {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	if _, _, err := service.Connector(context.Background(), "alerts"); !errors.Is(err, ErrIntegrationDisabled) {
		t.Fatalf("disabled connector: %v", err)
	}
	if _, _, err := service.Connector(context.Background(), "unknown"); !errors.Is(err, ErrIntegrationNotFound) {
		t.Fatalf("unknown connector: %v", err)
	}
	if err := service.Delete(context.Background(), "alerts", "operator", ""); !errors.Is(err, ErrInvalidIntegration) {
		t.Fatalf("delete without reason: %v", err)
	}
	if err := service.Delete(context.Background(), "alerts", "operator", "retire"); err != nil || len(repo.deletes) != 1 {
		t.Fatal(err)
	}
}

func TestRegistryConnectorRechecksBinding(t *testing.T) {
	service, repo, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	integration := validIntegration()
	integration.Origin = origin
	integration.Enabled = true
	repo.rows = map[string]Integration{"alerts": integration}
	repo.bindingErr = ErrInvalidIntegration
	if _, _, err := service.Connector(context.Background(), "alerts"); !errors.Is(err, ErrInvalidIntegration) {
		t.Fatalf("err = %v", err)
	}
	if _, err := service.Save(context.Background(), integration, "operator", "re-enable"); !errors.Is(err, ErrInvalidIntegration) {
		t.Fatalf("save err = %v", err)
	}
}

func TestRegistryConnectorRequiresResolvableCredential(t *testing.T) {
	service, repo, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	integration := validIntegration()
	integration.Origin = origin
	integration.Enabled = true
	integration.CredentialRef = "env:UNSET_SECRET"
	repo.rows = map[string]Integration{"alerts": integration}
	if _, _, err := service.Connector(context.Background(), "alerts"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err = %v", err)
	}
}
