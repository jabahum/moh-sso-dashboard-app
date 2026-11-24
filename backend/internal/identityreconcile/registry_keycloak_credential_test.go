package identityreconcile

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// fakeClientSecrets stands in for the Keycloak admin client.
type fakeClientSecrets struct {
	secret string
	err    error
	asked  []string
}

func (f *fakeClientSecrets) ServiceClientSecret(_ context.Context, clientID string) (string, error) {
	f.asked = append(f.asked, clientID)
	return f.secret, f.err
}

func keycloakIntegration() Integration {
	integration := validIntegration()
	integration.CredentialRef = "keycloak:alerts-identity-sync"
	return integration
}

func TestKeycloakCredentialReferenceMustNameTheSyncClient(t *testing.T) {
	if err := keycloakIntegration().Validate(); err != nil {
		t.Fatalf("keycloak:<sync client> refused: %v", err)
	}
	for _, ref := range []string{
		"keycloak:alerts",          // the application's own browser client
		"keycloak:dashboard-admin", // any other client
		"keycloak:",                // no client
		"keycloak:-leading-dash",   // not a client ID
		"keycloak:alerts identity", // whitespace
		"keycloak:" + strings.Repeat("a", 129),
	} {
		integration := validIntegration()
		integration.CredentialRef = ref
		if err := integration.Validate(); !errors.Is(err, ErrInvalidIntegration) {
			t.Errorf("%q: err = %v, want ErrInvalidIntegration", ref, err)
		}
	}
}

func TestKeycloakCredentialIsReadFromKeycloakWhenTheConnectorIsBuilt(t *testing.T) {
	service, _, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == syncBase+"/sessions/revoke" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = io.WriteString(w, descriptorJSON("1.1", "alerts", "opaque", "accounts.list", "identity.link", "sessions.revoke"))
	})
	// Nothing in the environment: the secret only comes from Keycloak.
	service.Getenv = func(string) string { return "" }
	secrets := &fakeClientSecrets{secret: "fixture-secret"}
	service.WithClientSecrets(secrets)

	integration := keycloakIntegration()
	integration.Origin = origin
	integration.Enabled = true
	saved, err := service.Save(context.Background(), integration, "operator", "onboard alerts")
	if err != nil {
		t.Fatal(err)
	}
	if saved.CredentialRef != "keycloak:alerts-identity-sync" || saved.ContractVersion != "1.1" {
		t.Fatalf("saved = %+v", saved)
	}
	connector, _, err := service.Connector(context.Background(), "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if err := connector.RevokeSessions(context.Background(), identitysync.Identity{Issuer: testIssuer, Subject: "s"}); err != nil {
		t.Fatal(err)
	}
	// Read afresh for each connector, so a rotated secret is picked up.
	if len(secrets.asked) != 2 || secrets.asked[0] != "alerts-identity-sync" || secrets.asked[1] != "alerts-identity-sync" {
		t.Fatalf("asked Keycloak for %v", secrets.asked)
	}
}

func TestKeycloakCredentialFailures(t *testing.T) {
	const secret = "s3cr3t-value-that-must-not-leak"
	ctx := context.Background()
	t.Run("no admin client", func(t *testing.T) {
		service := NewRegistryService(&fakeRegistryRepo{}, "https://sso.example.test/token")
		if _, err := service.ConnectorFor(ctx, keycloakIntegration()); !errors.Is(err, ErrKeycloakCredentialUnavailable) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("typed nil admin client", func(t *testing.T) {
		var admin *fakeClientSecrets
		service := NewRegistryService(&fakeRegistryRepo{}, "https://sso.example.test/token").WithClientSecrets(admin)
		if _, err := service.ConnectorFor(ctx, keycloakIntegration()); !errors.Is(err, ErrKeycloakCredentialUnavailable) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("keycloak error", func(t *testing.T) {
		cause := errors.New("read client secret of alerts-identity-sync: HTTP 403")
		service := NewRegistryService(&fakeRegistryRepo{}, "https://sso.example.test/token").
			WithClientSecrets(&fakeClientSecrets{err: cause})
		_, err := service.ConnectorFor(ctx, keycloakIntegration())
		if !errors.Is(err, cause) || !strings.Contains(err.Error(), "keycloak:alerts-identity-sync") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("empty secret", func(t *testing.T) {
		service := NewRegistryService(&fakeRegistryRepo{}, "https://sso.example.test/token").
			WithClientSecrets(&fakeClientSecrets{})
		if _, err := service.ConnectorFor(ctx, keycloakIntegration()); err == nil {
			t.Fatal("empty secret accepted")
		}
	})
	t.Run("ResolveCredential alone cannot read keycloak references", func(t *testing.T) {
		_, err := ResolveCredential("keycloak:alerts-identity-sync", func(string) string { return secret }, nil)
		if !errors.Is(err, ErrKeycloakCredentialUnavailable) || strings.Contains(err.Error(), secret) {
			t.Fatalf("err = %v", err)
		}
	})
}
