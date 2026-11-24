//go:build integration

package keycloak

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestEnsureServiceClientAgainstKeycloak runs against a disposable Keycloak
// whose realm was imported from infra/keycloak/realm-export.json:
//
//	KEYCLOAK_IT_URL=http://localhost:18080 go test -tags=integration -run AgainstKeycloak ./internal/keycloak/
//
// It deletes clims-identity-sync, recreates it, reads its generated secret
// with ServiceClientSecret, and checks that a client-credentials token carries aud=clims and clims/identity_sync. Never
// point it at a shared realm.
func TestEnsureServiceClientAgainstKeycloak(t *testing.T) {
	baseURL := os.Getenv("KEYCLOAK_IT_URL")
	if baseURL == "" {
		t.Skip("KEYCLOAK_IT_URL is not set")
	}
	realm := envOr("KEYCLOAK_IT_REALM", "moh-realm")
	admin := NewAdminClient(baseURL, realm,
		envOr("KEYCLOAK_IT_ADMIN_CLIENT_ID", "dashboard-admin"),
		envOr("KEYCLOAK_IT_ADMIN_CLIENT_SECRET", "dashboard-admin-secret"))
	ctx := context.Background()
	spec := ServiceClientSpec{
		ClientID: "clims-identity-sync",
		Name:     "clims identity sync",
		Audience: "clims",
		Roles:    []ServiceClientRole{{ClientID: "clims", Role: "identity_sync"}},
	}

	// The imported realm already has the client complete: nothing to do.
	result, err := admin.EnsureServiceClient(ctx, spec)
	if err != nil {
		t.Fatalf("ensure against the imported realm: %v", err)
	}
	if result.Changed() {
		t.Fatalf("imported realm result = %+v, want no changes", result)
	}

	if existing, err := admin.findServiceClientState(ctx, spec.ClientID); err != nil || existing == nil {
		t.Fatalf("find imported client: %v", err)
	} else {
		res, err := admin.DeleteWithContext(ctx, "clients/"+url.PathEscape(existing.ID))
		if err != nil || res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete client: %v %v", err, res)
		}
		res.Body.Close()
	}

	result, err = admin.EnsureServiceClient(ctx, spec)
	if err != nil {
		t.Fatalf("ensure after delete: %v", err)
	}
	if !result.Created || len(result.RolesAssigned) != 1 || len(result.ScopeMappingsAdded) != 1 {
		t.Fatalf("recreate result = %+v", result)
	}
	if again, err := admin.EnsureServiceClient(ctx, spec); err != nil || again.Changed() {
		t.Fatalf("second run = %+v, %v; want no changes", again, err)
	}

	// The secret Keycloak generated is what a keycloak: credential reference
	// reads; the application's own browser client is refused.
	secret, err := admin.ServiceClientSecret(ctx, spec.ClientID)
	if err != nil || secret == "" {
		t.Fatalf("read the generated secret: %v", err)
	}
	if _, err := admin.ServiceClientSecret(ctx, "clims"); !errors.Is(err, ErrNotServiceClient) {
		t.Fatalf("clims secret: err = %v, want ErrNotServiceClient", err)
	}

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {spec.ClientID},
		"client_secret": {secret},
	}
	res, err := http.PostForm(baseURL+"/realms/"+realm+"/protocol/openid-connect/token", form)
	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("token request: HTTP %d", res.StatusCode)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	parts := strings.Split(token.AccessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("access token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode token payload: %v", err)
	}
	var claims struct {
		Azp            string          `json:"azp"`
		Aud            json.RawMessage `json:"aud"`
		ResourceAccess map[string]struct {
			Roles []string `json:"roles"`
		} `json:"resource_access"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var audiences []string
	if err := json.Unmarshal(claims.Aud, &audiences); err != nil {
		var single string
		_ = json.Unmarshal(claims.Aud, &single)
		audiences = []string{single}
	}
	if claims.Azp != spec.ClientID || !slices.Contains(audiences, "clims") {
		t.Errorf("azp = %q, aud = %v; want %s and clims", claims.Azp, audiences, spec.ClientID)
	}
	if !slices.Contains(claims.ResourceAccess["clims"].Roles, "identity_sync") {
		t.Errorf("resource_access = %+v, want clims/identity_sync", claims.ResourceAccess)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
