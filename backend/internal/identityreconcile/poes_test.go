package identityreconcile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

func TestPOESAccountConnector(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/internal/identity-sync/accounts":
			if r.URL.Query().Has("after") {
				t.Error("first page must omit the cursor")
			}
			_, _ = w.Write([]byte(`{"accounts":[{"id":"1","email":"one@example.org"},{"id":"2","email":"two@example.org"}],"nextCursor":""}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/internal/identity-sync/accounts/1/identity":
			w.WriteHeader(http.StatusConflict)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	poes, err := NewPOES(server.URL, func(context.Context) (string, error) { return "fixture-token", nil })
	if err != nil {
		t.Fatal(err)
	}
	poes.client.Transport = server.Client().Transport
	ctx := context.Background()
	page, err := poes.ListEligibleAccounts(ctx, "", 1)
	if err != nil || len(page.Accounts) != 1 || page.NextCursor != "1" {
		t.Fatalf("first page: %+v, %v", page, err)
	}
	err = poes.LinkIdentity(ctx, "1", identitysync.Identity{Issuer: "https://issuer.example", Subject: "subject"})
	if !errors.Is(err, identitysync.ErrLinkConflict) {
		t.Fatalf("expected link conflict, got %v", err)
	}
}
