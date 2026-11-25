package keycloak

import (
	"strings"
	"testing"
)

func TestOnboardingProfilesAndEnvironmentIsolation(t *testing.T) {
	spec := OnboardingSpec{Version: 1, ClientID: "timesheets", Name: "Timesheets", Owner: "Workforce team", SupportEmail: "support@example.org", Profile: "public-web", Environment: "production", RootURL: "https://app.example.org", RedirectURIs: []string{"https://app.example.org/callback"}, LogoutURIs: []string{"https://app.example.org/login"}, WebOrigins: []string{"https://app.example.org"}}
	client, err := BuildOnboardingClient(spec, "production")
	if err != nil {
		t.Fatal(err)
	}
	if client.Enabled || !client.PublicClient || !client.StandardFlow || client.ServiceAccounts || client.DirectAccess || client.ImplicitFlow || client.Attributes["pkce.code.challenge.method"] != "S256" {
		t.Fatal("unsafe public profile")
	}
	if len(client.ProtocolMappers) != 3 || strings.Join(client.DefaultClientScopes, ",") != "profile,email" {
		t.Fatal("missing identity claims")
	}
	if _, err := BuildOnboardingClient(spec, "staging"); err == nil {
		t.Fatal("cross-environment import accepted without review")
	}
	for _, bad := range []string{"https://app.example.org/*", "https://user:password@app.example.org/callback", "http://app.example.org/callback", "https://app.example.org/callback#fragment"} {
		candidate := spec
		candidate.RedirectURIs = []string{bad}
		if _, err := BuildOnboardingClient(candidate, "production"); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	spec.Profile = "confidential-web"
	client, err = BuildOnboardingClient(spec, "production")
	if err != nil || client.PublicClient || !client.StandardFlow {
		t.Fatal("invalid confidential profile")
	}
	spec.Profile = "service"
	spec.RootURL = ""
	spec.RedirectURIs = nil
	spec.LogoutURIs = nil
	spec.WebOrigins = nil
	client, err = BuildOnboardingClient(spec, "production")
	if err != nil || client.PublicClient || client.StandardFlow || !client.ServiceAccounts || len(client.DefaultClientScopes) > 0 {
		t.Fatalf("invalid service profile: %v", err)
	}
	spec.Launcher = true
	if _, err = BuildOnboardingClient(spec, "production"); err == nil {
		t.Fatal("service launcher accepted")
	}
}

func TestValidateIdentityEnvironmentRealmOverride(t *testing.T) {
	for _, tc := range []struct {
		name, environment, realm, override string
		ok                                 bool
	}{
		{"default production realm", "production", "moh-realm", "", true},
		{"default realm mismatch", "production", "MoH", "", false},
		{"production realm override", "production", "MoH", "MoH", true},
		{"override is trimmed", "production", "MoH", " MoH ", true},
		{"override must match configured realm", "production", "moh-realm", "MoH", false},
		{"override cannot claim another environment's realm", "production", "moh-dev", "moh-dev", false},
		{"dev cannot claim the production realm", "development", "moh-realm", "moh-realm", false},
		{"unknown environment", "qa", "MoH", "MoH", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIdentityEnvironment(tc.environment, tc.realm, tc.override)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidateIdentityEnvironment(%q, %q, %q) = %v, want ok=%v", tc.environment, tc.realm, tc.override, err, tc.ok)
			}
		})
	}
}
