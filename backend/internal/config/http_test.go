package config

import (
	"github.com/spf13/viper"
	"strings"
	"testing"
)

func privateHTTPConfig() *Config {
	return &Config{
		Environment:                 "production",
		FrontendBaseURL:             "http://localhost:8080/portal",
		FrontendRedirectURI:         "http://localhost:8080/portal",
		LoginURL:                    "http://localhost:8080/ssobackend/api/v1/auth/login",
		AuthReturnURLAllowedOrigins: "http://localhost:8080",
		KeycloakExternalURL:         "https://auth.example.org",
		KeycloakBaseURL:             "https://auth.example.org",
		KeycloakRealm:               "test",
		KeycloakAdminClientID:       "admin",
		KeycloakWebClientID:         "web",
		KeycloakRedirectURI:         "http://localhost:8080/ssobackend/api/v1/auth/callback",
	}
}

func TestProductionHTTPRequiresExplicitOptIn(t *testing.T) {
	for _, environment := range []string{"production", "prod"} {
		t.Run(environment, func(t *testing.T) {
			c := privateHTTPConfig()
			c.Environment = environment
			for name, check := range map[string]func(*Config) error{"frontend": validateFrontendConfig, "keycloak": validateKeycloakConfig} {
				if err := check(c); err == nil || !strings.Contains(err.Error(), "https://") {
					t.Fatalf("%s: expected HTTPS requirement, got %v", name, err)
				}
			}
			c.AllowInsecureHTTP = true
			if err := validateFrontendConfig(c); err != nil {
				t.Fatal(err)
			}
			if err := validateKeycloakConfig(c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrivateHTTPPreservesIdentityProviderAndURLValidation(t *testing.T) {
	c := privateHTTPConfig()
	c.AllowInsecureHTTP = true
	c.KeycloakExternalURL = "http://auth.example.org"
	if err := validateKeycloakConfig(c); err == nil {
		t.Fatal("external Keycloak must still require HTTPS")
	}
	c = privateHTTPConfig()
	c.AllowInsecureHTTP = true
	c.FrontendBaseURL = "file:///portal"
	if err := validateFrontendConfig(c); err == nil {
		t.Fatal("invalid application URL must still be rejected")
	}
	c = privateHTTPConfig()
	c.AllowInsecureHTTP = true
	c.DevAuthBypass = true
	if err := validateConfig(c); err == nil || !strings.Contains(err.Error(), "DEV_AUTH_BYPASS") {
		t.Fatalf("authentication bypass must remain prohibited, got %v", err)
	}
}

func TestCookieSecurityForPrivateHTTP(t *testing.T) {
	for _, environment := range []string{"production", "staging"} {
		c := privateHTTPConfig()
		c.Environment = environment
		if !c.CookieSecure() {
			t.Fatal("deployed cookies must be Secure by default")
		}
		c.AllowInsecureHTTP = true
		if c.CookieSecure() {
			t.Fatal("explicit HTTP access must allow cookies over HTTP")
		}
		c.FrontendBaseURL = "https://dashboard.example.org/portal"
		if !c.CookieSecure() {
			t.Fatal("HTTPS must retain Secure cookies with opt-in enabled")
		}
	}
}

func TestPrivateHTTPEnvironmentBinding(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setDefaults()
	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if c.AllowInsecureHTTP {
		t.Fatal("HTTP must be disabled by default")
	}
	t.Setenv("ALLOW_INSECURE_HTTP", "true")
	for key, env := range envBindings() {
		if err := viper.BindEnv(key, env); err != nil {
			t.Fatal(err)
		}
	}
	if err := viper.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if !c.AllowInsecureHTTP {
		t.Fatal("environment opt-in must reach runtime config")
	}
}
