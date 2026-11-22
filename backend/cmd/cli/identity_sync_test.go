package main

import (
	"testing"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func baseEnv() map[string]string {
	return map[string]string{
		"KEYCLOAK_BASE_URL":            "https://keycloak.internal:8443/",
		"KEYCLOAK_REALM":               "moh",
		"KEYCLOAK_ADMIN_CLIENT_ID":     "operator",
		"KEYCLOAK_ADMIN_CLIENT_SECRET": "operator-secret",
		"CLIMS_SYNC_ORIGIN":            "https://clims.health.go.ug",
		"CLIMS_SYNC_CLIENT_ID":         "clims-identity-sync",
		"CLIMS_SYNC_CLIENT_SECRET":     "sync-secret",
	}
}

func TestIdentitySyncConfigUsesExternalIssuerAndInternalTokenEndpoint(t *testing.T) {
	env := baseEnv()
	env["KEYCLOAK_EXTERNAL_URL"] = "https://sso.health.go.ug/"
	cfg, err := identitySyncConfigFromEnv("CLIMS", envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.issuer != "https://sso.health.go.ug/realms/moh" {
		t.Errorf("issuer = %q", cfg.issuer)
	}
	if cfg.tokenEndpoint != "https://keycloak.internal:8443/realms/moh/protocol/openid-connect/token" {
		t.Errorf("tokenEndpoint = %q", cfg.tokenEndpoint)
	}
	if cfg.adminBase != "https://keycloak.internal:8443" {
		t.Errorf("adminBase = %q", cfg.adminBase)
	}
	if cfg.origin != "https://clims.health.go.ug" || cfg.syncClientID != "clims-identity-sync" || cfg.syncClientSecret != "sync-secret" {
		t.Errorf("app settings not loaded: %+v", cfg)
	}
}

func TestIdentitySyncConfigFallsBackToBaseURLForIssuer(t *testing.T) {
	cfg, err := identitySyncConfigFromEnv("CLIMS", envFrom(baseEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.issuer != "https://keycloak.internal:8443/realms/moh" {
		t.Errorf("issuer = %q", cfg.issuer)
	}
}

func TestIdentitySyncConfigRejectsInvalidSettings(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"http external URL":   func(e map[string]string) { e["KEYCLOAK_EXTERNAL_URL"] = "http://sso.health.go.ug" },
		"external with query": func(e map[string]string) { e["KEYCLOAK_EXTERNAL_URL"] = "https://sso.health.go.ug?x=1" },
		"http base URL":       func(e map[string]string) { e["KEYCLOAK_BASE_URL"] = "http://keycloak.internal" },
		"missing realm":       func(e map[string]string) { delete(e, "KEYCLOAK_REALM") },
		"realm needs escape":  func(e map[string]string) { e["KEYCLOAK_REALM"] = "moh/x" },
		"missing operator":    func(e map[string]string) { delete(e, "KEYCLOAK_ADMIN_CLIENT_SECRET") },
	} {
		t.Run(name, func(t *testing.T) {
			env := baseEnv()
			mutate(env)
			if _, err := identitySyncConfigFromEnv("CLIMS", envFrom(env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestIdentitySyncTokenSourceSelection(t *testing.T) {
	endpoint := "https://keycloak.internal/realms/moh/protocol/openid-connect/token"
	if _, err := identitySyncToken("CLIMS", endpoint, "", "", ""); err == nil {
		t.Error("expected an error with no credentials")
	}
	if _, err := identitySyncToken("CLIMS", endpoint, "clims-identity-sync", "", "static"); err == nil {
		t.Error("a partial service client must not silently fall back to the static token")
	}
	source, err := identitySyncToken("CLIMS", endpoint, "", "", "static")
	if err != nil {
		t.Fatal(err)
	}
	if token, err := source(t.Context()); err != nil || token != "static" {
		t.Errorf("static token = %q, %v", token, err)
	}
}
