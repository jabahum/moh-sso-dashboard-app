package keycloak

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func revocationAdminFixture(t *testing.T, handler http.HandlerFunc) *KeyAdminClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token") {
			_, _ = io.WriteString(w, `{"access_token":"t","expires_in":300}`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return NewAdminClient(server.URL, "moh", "admin", "secret")
}

func TestRevokeUserClientConsentIsIdempotentAndScopedToOneClient(t *testing.T) {
	var paths []string
	status := http.StatusNoContent
	admin := revocationAdminFixture(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.WriteHeader(status)
	})
	ctx := context.Background()
	for _, s := range []int{http.StatusNoContent, http.StatusNotFound} {
		status = s
		if err := admin.RevokeUserClientConsent(ctx, "user-1", "clims"); err != nil {
			t.Fatalf("status %d: %v", s, err)
		}
	}
	status = http.StatusInternalServerError
	if err := admin.RevokeUserClientConsent(ctx, "user-1", "clims"); err == nil {
		t.Fatal("500 accepted")
	}
	for _, p := range paths {
		if p != "DELETE /admin/realms/moh/users/user-1/consents/clims" {
			t.Fatalf("unexpected request %s", p)
		}
	}
}

func TestRemoveUserClientRoleIfPresent(t *testing.T) {
	var deleted int
	admin := revocationAdminFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/admin/realms/moh/clients":
			// A prefix match for another client must not be used.
			_, _ = io.WriteString(w, `[{"id":"other","clientId":"clims-old"},{"id":"uuid-clims","clientId":"clims"}]`)
		case r.URL.Path == "/admin/realms/moh/users/missing/role-mappings/clients/uuid-clims":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/moh/users/user-1/role-mappings/clients/uuid-clims":
			if deleted > 0 {
				_, _ = io.WriteString(w, `[]`)
				return
			}
			_, _ = io.WriteString(w, `[{"id":"r1","name":"clims_access"}]`)
		case r.Method == http.MethodDelete && r.URL.Path == "/admin/realms/moh/users/user-1/role-mappings/clients/uuid-clims":
			body, _ := io.ReadAll(r.Body)
			if string(body) != `[{"id":"r1","name":"clims_access"}]` {
				t.Errorf("delete body = %s", body)
			}
			deleted++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	ctx := context.Background()
	if removed, err := admin.RemoveUserClientRoleIfPresent(ctx, "user-1", "clims", "clims_access"); err != nil || !removed {
		t.Fatalf("first = %v, %v", removed, err)
	}
	if removed, err := admin.RemoveUserClientRoleIfPresent(ctx, "user-1", "clims", "clims_access"); err != nil || removed || deleted != 1 {
		t.Fatalf("repeat = %v, %v (deleted %d)", removed, err, deleted)
	}
	if _, err := admin.RemoveUserClientRoleIfPresent(ctx, "missing", "clims", "clims_access"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing user = %v", err)
	}
}
