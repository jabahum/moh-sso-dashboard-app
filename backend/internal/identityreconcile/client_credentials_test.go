package identityreconcile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClientCredentialsMintsAndCachesToken(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		id, secret, ok := r.BasicAuth()
		if r.Method != http.MethodPost || !ok || id != "clims-identity-sync" || secret != "s3cr3t" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "client_credentials" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok-1","expires_in":300,"token_type":"Bearer"}`))
	}))
	defer server.Close()

	source, err := newClientCredentials(server.URL+"/realms/moh/protocol/openid-connect/token", "clims-identity-sync", "s3cr3t", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		token, err := source(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if token != "tok-1" {
			t.Fatalf("token = %q", token)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("token endpoint called %d times, want 1 (cached)", got)
	}
}

func TestClientCredentialsRejectsBadResponses(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"unauthorized":  {http.StatusUnauthorized, `{"error":"unauthorized_client"}`},
		"missing token": {http.StatusOK, `{"expires_in":300,"token_type":"Bearer"}`},
		"no expiry":     {http.StatusOK, `{"access_token":"t","token_type":"Bearer"}`},
		"wrong type":    {http.StatusOK, `{"access_token":"t","expires_in":300,"token_type":"MAC"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			source, err := newClientCredentials(server.URL+"/token", "id", "secret", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := source(context.Background()); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestClientCredentialsValidatesConfig(t *testing.T) {
	for _, tc := range []struct{ endpoint, id, secret string }{
		{"http://keycloak.example/token", "id", "secret"},
		{"https://user:pw@keycloak.example/token", "id", "secret"},
		{"https://keycloak.example/token?x=1", "id", "secret"},
		{"https://keycloak.example/token", "", "secret"},
		{"https://keycloak.example/token", "id", ""},
	} {
		if _, err := ClientCredentials(tc.endpoint, tc.id, tc.secret); err == nil {
			t.Errorf("ClientCredentials(%q, %q, %q) accepted invalid config", tc.endpoint, tc.id, tc.secret)
		}
	}
}
