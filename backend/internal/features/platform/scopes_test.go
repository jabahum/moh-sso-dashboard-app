package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
)

func TestScopeCatalogueIsValidAndCoversTheScopeConstants(t *testing.T) {
	for _, name := range []string{IdentityRead, NotificationsRead, NotificationsWrite} {
		if !KnownScope(name) || !MachineScope(name) {
			t.Errorf("%s is missing from scopes.json", name)
		}
	}
	if KnownScope("admin:everything") || MachineScope("admin:everything") {
		t.Fatal("unknown scope reported as known")
	}
	for name, mutate := range map[string]string{
		"bad version":      `{"version":2,"scopes":[{"name":"a:b","description":"d","tokens":["machine"],"endpoints":["GET /api/v1/platform/x"]}]}`,
		"no scopes":        `{"version":1,"scopes":[]}`,
		"bad name":         `{"version":1,"scopes":[{"name":"Admin","description":"d","tokens":["machine"],"endpoints":["GET /api/v1/platform/x"]}]}`,
		"duplicate":        `{"version":1,"scopes":[{"name":"a:b","description":"d","tokens":["machine"],"endpoints":["GET /api/v1/platform/x"]},{"name":"a:b","description":"d","tokens":["machine"],"endpoints":["GET /api/v1/platform/x"]}]}`,
		"no description":   `{"version":1,"scopes":[{"name":"a:b","description":" ","tokens":["machine"],"endpoints":["GET /api/v1/platform/x"]}]}`,
		"bad token type":   `{"version":1,"scopes":[{"name":"a:b","description":"d","tokens":["robot"],"endpoints":["GET /api/v1/platform/x"]}]}`,
		"non-platform URL": `{"version":1,"scopes":[{"name":"a:b","description":"d","tokens":["machine"],"endpoints":["GET /api/v1/admin/x"]}]}`,
	} {
		if _, err := parseScopeCatalogue([]byte(mutate)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

var ginParam = regexp.MustCompile(`:([A-Za-z]+)`)

// Every platform route is listed under a scope, and every listed endpoint exists.
func TestScopeCatalogueMatchesThePlatformRoutes(t *testing.T) {
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), router.Group("/api/v1"), &Handler{Repository: &testRepository{}, Verifier: testVerifier{token: validToken()}}, nil)
	listed := map[string]bool{}
	for _, scope := range Scopes() {
		for _, endpoint := range scope.Endpoints {
			listed[endpoint] = true
		}
	}
	registered := map[string]bool{}
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/platform/") {
			endpoint := route.Method + " " + ginParam.ReplaceAllString(route.Path, "{$1}")
			registered[endpoint] = true
			if !listed[endpoint] {
				t.Errorf("%s is not listed under any scope in scopes.json", endpoint)
			}
		}
	}
	for endpoint := range listed {
		if !registered[endpoint] {
			t.Errorf("scopes.json lists %s, which is not a platform route", endpoint)
		}
	}
}

// Each machine endpoint refuses a principal without its scope and accepts one with it.
func TestMachineEndpointsRequireTheirCatalogueScope(t *testing.T) {
	repo := &testRepository{}
	router := gin.New()
	router.Use(gin.Recovery()) // handlers past the scope check may reach the unimplemented fake repository
	RegisterRoutes(router.Group("/api/v1"), router.Group("/api/v1"), &Handler{Repository: repo, Verifier: testVerifier{token: validToken()}}, nil)
	call := func(endpoint string) int {
		method, path, _ := strings.Cut(endpoint, " ")
		path = strings.NewReplacer("{id}", "00000000-0000-4000-8000-000000000001", "{clientId}", "pilot").Replace(path)
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	for _, scope := range Scopes() {
		for _, endpoint := range scope.Endpoints {
			if strings.Contains(endpoint, "/applications/") {
				continue // the access decision uses the user's token, covered elsewhere
			}
			var others []string
			for _, name := range scopeNames() {
				if name != scope.Name {
					others = append(others, name)
				}
			}
			repo.principal = Principal{ApplicationID: "pilot", Subject: "subject", Scopes: others}
			if code := call(endpoint); code != http.StatusForbidden {
				t.Errorf("%s without %s: %d, want 403", endpoint, scope.Name, code)
			}
			repo.principal.Scopes = []string{scope.Name}
			if code := call(endpoint); code == http.StatusForbidden || code == http.StatusUnauthorized {
				t.Errorf("%s with %s: %d", endpoint, scope.Name, code)
			}
		}
	}
}

func scopeNames() []string {
	var names []string
	for _, scope := range Scopes() {
		names = append(names, scope.Name)
	}
	return names
}

type savingRepository struct {
	testRepository
	saved []IdentityInput
}

func (r *savingRepository) SaveIdentity(_ context.Context, app, client, _ string, input IdentityInput) (Identity, error) {
	r.saved = append(r.saved, input)
	return Identity{ApplicationID: app, ClientID: client, Subject: input.Subject, Scopes: input.Scopes, Enabled: input.Enabled}, nil
}

func TestBindingAcceptsOnlyCatalogueScopes(t *testing.T) {
	repo := &savingRepository{}
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(authz.ContextKey, authz.Context{UserID: "operator-1", Permissions: opsPermissions})
		c.Set("user_id", "operator-1")
	})
	RegisterRoutes(router.Group("/api/v1"), protected, &Handler{Repository: repo, Verifier: testVerifier{token: validToken()}}, nil)
	bind := func(scopes ...string) (int, string) {
		body, _ := json.Marshal(map[string]any{"subject": "00000000-0000-4000-8000-000000000001", "scopes": scopes, "enabled": true, "reason": "CHG-1: alerts backend"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/platform/applications/alerts/identities/alerts-platform", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	if code, body := bind(scopeNames()...); code != 200 || len(repo.saved) != 1 {
		t.Fatalf("every catalogue scope: %d %s", code, body)
	}
	code, body := bind(IdentityRead, "admin:everything")
	if code != 400 || !strings.Contains(body, "UNKNOWN_SCOPE") || !strings.Contains(body, `\"admin:everything\"`) || !strings.Contains(body, NotificationsWrite) || len(repo.saved) != 1 {
		t.Fatalf("unknown scope: %d %s", code, body)
	}
	if code, _ := bind(IdentityRead, IdentityRead); code != 400 || len(repo.saved) != 1 {
		t.Fatalf("duplicate scope: %d", code)
	}
}

func TestListScopes(t *testing.T) {
	router, _ := serviceIdentityRouter(nil, authz.PermissionRBACRead)
	code, body := get(router, "/api/v1/admin/platform/scopes")
	var envelope struct {
		Data []Scope `json:"data"`
	}
	if code != 200 || json.Unmarshal([]byte(body), &envelope) != nil || len(envelope.Data) != len(Scopes()) || envelope.Data[0].Description == "" {
		t.Fatalf("%d %s", code, body)
	}
	forbidden, _ := serviceIdentityRouter(nil)
	if code, _ := get(forbidden, "/api/v1/admin/platform/scopes"); code != 403 {
		t.Fatalf("without rbac:read: %d", code)
	}
}
