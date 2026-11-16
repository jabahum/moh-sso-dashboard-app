package rbac

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/module"
)

func manifestFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "examples", "manifests", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestApplicationManifestExamplesAndValidation(t *testing.T) {
	modules := []module.Info{{ID: "metrics", Version: "v0.1.0", Enabled: true}}
	for _, name := range []string{"internal", "external", "microfrontend"} {
		if _, _, err := parseApplicationManifest(manifestFixture(t, name), "development", modules); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	external := manifestFixture(t, "external")
	for name, document := range map[string]string{
		"schema":                strings.Replace(external, ManifestAPIVersion, "unknown/v9", 1),
		"unknown field":         external + "secret: forbidden\n",
		"duplicate field":       external + "kind: Application\n",
		"multiple applications": external + "\n---\n" + external,
		"duplicate role":        strings.Replace(external, "[report_viewer]", "[report_viewer, report_viewer]", 1),
		"environment":           strings.Replace(external, "environment: development", "environment: production", 1),
		"URL":                   strings.Replace(external, "url: https://health-bi.example.org/", "url: javascript:alert(1)", 1),
		"credentials":           strings.Replace(external, "url: https://health-bi.example.org/", "url: https://user:password@health-bi.example.org/", 1),
		"version":               strings.Replace(external, "version: 1.0.0", "version: 01.0.0", 1),
		"oversize":              strings.Repeat("x", maxManifestBytes+1),
	} {
		if _, _, err := parseApplicationManifest(document, "development", modules); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
	internal := manifestFixture(t, "internal")
	if _, _, err := parseApplicationManifest(internal, "development", nil); err == nil {
		t.Fatal("accepted uncompiled module")
	}
	modules[0].Enabled = false
	if _, _, err := parseApplicationManifest(internal, "development", modules); err == nil {
		t.Fatal("accepted disabled module")
	}
	for _, path := range []string{"/portal/../admin/home", "//example.org/app", "/apps/%2e%2e/admin"} {
		if manifestPortalPath(path) {
			t.Errorf("accepted unsafe route %s", path)
		}
	}
	microfrontend := manifestFixture(t, "microfrontend")
	for _, document := range []string{
		strings.Replace(microfrontend, "@moh-extensions/example-reports", "@moh-sso/users", 1),
		strings.ReplaceAll(microfrontend, "/apps/extensions/example-reports", "/apps/other"),
		microfrontend + "  navigation:\n    - id: duplicate\n      label: Duplicate\n      path: /apps/extensions/example-reports\n",
	} {
		if _, _, err := parseApplicationManifest(document, "development", nil); err == nil {
			t.Fatal("accepted invalid microfrontend")
		}
	}
}

func TestApplicationManifestAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, identity := range []string{"reader", "writer", "admin"} {
		service := NewService(nil)
		service.ConfigureManifests("development", nil)
		r := gin.New()
		r.Use(func(c *gin.Context) {
			actor := authz.Context{UserID: "test", IsAdmin: identity == "admin", Permissions: []authz.Permission{authz.PermissionRBACRead}}
			if identity != "reader" {
				actor.Permissions = append(actor.Permissions, authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite)
			}
			c.Set(authz.ContextKey, actor)
		})
		RegisterAdminRoutes(r.Group("/api/v1/admin"), NewHandler(service), nil)
		raw, _ := json.Marshal(ManifestRequest{Document: manifestFixture(t, "external")})
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/admin/rbac/application-manifests/validate", bytes.NewReader(raw)))
		want := http.StatusForbidden
		if identity == "admin" {
			want = http.StatusOK
		}
		if response.Code != want {
			t.Fatalf("%s: expected %d, got %d %s", identity, want, response.Code, response.Body.String())
		}
	}
}
