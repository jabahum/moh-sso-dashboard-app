package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	auditfeature "github.com/moh-sso-dashboard/internal/features/audit"
	geojsonfeature "github.com/moh-sso-dashboard/internal/features/geojson"
	metricsfeature "github.com/moh-sso-dashboard/internal/features/metrics"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/module"
)

func TestModuleConfiguration(t *testing.T) {
	if _, err := NewModuleRegistry(HandlerSet{}, "unknown", nil); err == nil {
		t.Fatal("unknown module was accepted")
	}
	r, err := NewModuleRegistry(HandlerSet{}, "", nil)
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
		registry, err := NewModuleRegistry(HandlerSet{Metrics: &metricsfeature.Handler{}, Audit: &auditfeature.Handler{}, GeoJSON: &geojsonfeature.Handler{}}, disabled, nil)
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
				if !body.Success || len(body.Data) != 3 || body.Data[0].Enabled != (disabled == "") {
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
