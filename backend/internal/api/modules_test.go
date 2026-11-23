package router

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/moh-sso-dashboard/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/module"
	geojsonfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
	metricsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
)

func TestModuleConfiguration(t *testing.T) {
	if _, err := NewModuleRegistry(HandlerSet{}, "unknown", nil, nil); err == nil {
		t.Fatal("unknown module was accepted")
	}
	r, err := NewModuleRegistry(HandlerSet{}, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Initialize(context.Background()); err == nil {
		t.Fatal("missing enabled handler was accepted")
	}
}

func TestMetricsModuleRoutesAndInventory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, disabled := range []string{"", " metrics "} {
		registry, err := NewModuleRegistry(HandlerSet{Metrics: testMetricsModule(t), Audit: testAuditModule(t, auditReadFixture{}), GeoJSON: testGeoJSONModule(t, "")}, disabled+",report-scheduler,announcements,admin-units,"+extractedModules, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		stop, err := registry.Initialize(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer stop(context.Background())
		for _, identity := range []string{"anonymous", "user", "admin"} {
			r := gin.New()
			protected := r.Group("/api/v1", func(c *gin.Context) {
				if identity != "anonymous" {
					c.Set("user_id", "test-user")
					c.Set("is_admin", identity == "admin")
				}
			}, middleware.RequireAuth())
			registry.RegisterRoutes(module.Routes{Protected: protected, Admin: protected.Group("/admin")})
			registerModuleInventory(protected, registry)
			res := httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/admin/modules", nil))
			want := map[string]int{"anonymous": 401, "user": 403, "admin": 200}[identity]
			if res.Code != want {
				t.Fatalf("identity %s: expected %d, got %d", identity, want, res.Code)
			}
			if identity == "admin" {
				var body response.SuccessResponse[[]module.Info]
				if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if !body.Success || len(body.Data) != len(config.OptionalModuleIDs()) || body.Data[0].Enabled != (disabled == "") {
					t.Fatalf("unexpected inventory: %s", res.Body.String())
				}
			}
			res = httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics/overview", nil))
			if disabled != "" && res.Code != http.StatusNotFound {
				t.Fatalf("disabled metrics route served: %d", res.Code)
			}
			if disabled == "" && res.Code != http.StatusUnauthorized {
				t.Fatalf("metrics permission guard bypassed: %d", res.Code)
			}
		}
	}
}

// Required platform capabilities are not optional business module IDs.
func TestRequiredPlatformCapabilitiesCannotBeDisabledAsModules(t *testing.T) {
	for _, id := range []string{"auth", "authsession", "sessions", "rbac", "system_rbac", "health_context", "platform"} {
		if _, err := NewModuleRegistry(HandlerSet{}, id, nil, nil); err == nil {
			t.Fatalf("required capability %s accepted as a disabled optional module", id)
		}
	}
}

func testGeoJSONModule(t *testing.T, directory string) *geojsonfeature.Module {
	t.Helper()
	item, err := geojsonfeature.NewModule(geojsonfeature.Dependencies{Directory: directory, AuthorizeAny: modulehttp.AuthorizeAny, RequestID: modulehttp.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func testMetricsModule(t *testing.T) *metricsfeature.Module {
	t.Helper()
	m, err := metricsfeature.NewModule(metricsfeature.Dependencies{
		Database: new(sql.DB), RequireAdmin: modulehttp.RequireAdmin, AuthorizeAny: modulehttp.AuthorizeAny, RequestID: modulehttp.RequestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
