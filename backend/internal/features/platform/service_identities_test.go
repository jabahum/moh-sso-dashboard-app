package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

type listingRepository struct {
	testRepository
	app string
}

func (r *listingRepository) ListIdentities(_ context.Context, app string) ([]Identity, error) {
	r.app = app
	return []Identity{{ApplicationID: app, ClientID: "alerts-identity-sync", Subject: "sa-1", Scopes: []string{}, Enabled: true}}, nil
}

type fakeResolver map[string]string

func (f fakeResolver) ServiceAccountUserID(_ context.Context, client string) (string, error) {
	if client == "broken" {
		return "", errors.New("keycloak down: https://kc.internal secret")
	}
	if id, ok := f[client]; ok {
		return id, nil
	}
	return "", keycloak.ErrServiceAccountNotFound
}

func serviceIdentityRouter(resolver ServiceAccountResolver, permissions ...authz.Permission) (*gin.Engine, *listingRepository) {
	repo := &listingRepository{}
	h := &Handler{Repository: repo, Verifier: testVerifier{token: validToken()}}
	if resolver != nil {
		h.ServiceAccounts = resolver
	}
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(authz.ContextKey, authz.Context{UserID: "operator-1", Permissions: permissions})
		c.Set("user_id", "operator-1")
	})
	RegisterRoutes(router.Group("/api/v1"), protected, h, nil)
	return router, repo
}

func get(router *gin.Engine, path string) (int, string) {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Code, w.Body.String()
}

func TestServiceIdentityAdminRoutes(t *testing.T) {
	const base = "/api/v1/admin/platform/applications/alerts/identities"
	router, repo := serviceIdentityRouter(fakeResolver{"alerts-identity-sync": "sa-1"}, opsPermissions...)
	if code, body := get(router, base); code != 200 || !strings.Contains(body, `"clientId":"alerts-identity-sync"`) || repo.app != "alerts" {
		t.Fatalf("list: %d %s (app %q)", code, body, repo.app)
	}
	if code, body := get(router, base+"/alerts-identity-sync/service-account"); code != 200 || !strings.Contains(body, `"subject":"sa-1"`) {
		t.Fatalf("resolve: %d %s", code, body)
	}
	if code, _ := get(router, base+"/unknown-client/service-account"); code != 404 {
		t.Fatalf("unknown client: %d", code)
	}
	if code, body := get(router, base+"/broken/service-account"); code != 503 || strings.Contains(body, "secret") {
		t.Fatalf("keycloak failure: %d %s", code, body)
	}
	unconfigured, _ := serviceIdentityRouter(nil, opsPermissions...)
	if code, _ := get(unconfigured, base+"/alerts-identity-sync/service-account"); code != 503 {
		t.Fatalf("unconfigured: %d", code)
	}
	forbidden, _ := serviceIdentityRouter(fakeResolver{}, "rbac:write")
	for _, path := range []string{base, base + "/alerts-identity-sync/service-account"} {
		if code, _ := get(forbidden, path); code != 403 {
			t.Fatalf("%s without rbac:roles:write: %d", path, code)
		}
	}
}
