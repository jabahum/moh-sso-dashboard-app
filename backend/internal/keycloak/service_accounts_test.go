package keycloak

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestServiceAccountUserID(t *testing.T) {
	admin := revocationAdminFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/admin/realms/moh/clients" && r.URL.Query().Get("clientId") == "clims-identity-sync":
			// A prefix match for another client must not be used.
			_, _ = io.WriteString(w, `[{"id":"other","clientId":"clims-identity-sync-old","serviceAccountsEnabled":true},
				{"id":"uuid-sync","clientId":"clims-identity-sync","serviceAccountsEnabled":true}]`)
		case r.URL.Path == "/admin/realms/moh/clients" && r.URL.Query().Get("clientId") == "clims":
			_, _ = io.WriteString(w, `[{"id":"uuid-clims","clientId":"clims","serviceAccountsEnabled":false}]`)
		case r.URL.Path == "/admin/realms/moh/clients":
			_, _ = io.WriteString(w, `[]`)
		case r.URL.Path == "/admin/realms/moh/clients/uuid-sync/service-account-user":
			_, _ = io.WriteString(w, `{"id":"sa-user-1","username":"service-account-clims-identity-sync"}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	ctx := context.Background()
	id, err := admin.ServiceAccountUserID(ctx, "clims-identity-sync")
	if err != nil || id != "sa-user-1" {
		t.Fatalf("ServiceAccountUserID = %q, %v", id, err)
	}
	for _, client := range []string{"clims", "missing", " "} {
		if _, err := admin.ServiceAccountUserID(ctx, client); !errors.Is(err, ErrServiceAccountNotFound) {
			t.Errorf("%q: err = %v, want ErrServiceAccountNotFound", client, err)
		}
	}
}
