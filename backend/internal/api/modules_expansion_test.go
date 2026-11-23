package router

import (
	"context"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/module"
	auditfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/audit"
	geojsonmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
)

type auditReadFixture struct{ auditfeature.Repository }

func TestHostedModuleInitializationRequiresOnlyEnabledHandlers(t *testing.T) {
	for _, disabled := range []string{"metrics,geojson", "metrics,audit", "metrics,audit,geojson"} {
		registry, err := NewModuleRegistry(HandlerSet{}, disabled+",report-scheduler,announcements,admin-units,"+extractedModules, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		stop, err := registry.Initialize(context.Background())
		if disabled == "metrics,audit,geojson" {
			if err != nil {
				t.Fatal(err)
			}
			_ = stop(context.Background())
		} else if err == nil {
			t.Fatalf("missing enabled handler accepted with disabled=%s", disabled)
		}
	}
}

func (auditReadFixture) ListAuditActions(context.Context) (auditfeature.AuditActionsResponse, error) {
	return auditfeature.AuditActionsResponse{}, nil
}

func TestReadModuleRoutesPreservePermissionsAndDisablement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assets := t.TempDir()
	if err := os.WriteFile(filepath.Join(assets, "uga_water.geojson"), []byte(`{"type":"FeatureCollection","features":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, disabled := range []string{"", "audit", "geojson", "audit,geojson"} {
		handlers := HandlerSet{Metrics: testMetricsModule(t), Audit: testAuditModule(t, auditReadFixture{}), GeoJSON: testGeoJSONModule(t, assets)}
		registry, err := NewModuleRegistry(handlers, disabled+",report-scheduler,announcements,admin-units,"+extractedModules, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		stop, err := registry.Initialize(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer stop(context.Background())
		enabled := map[string]bool{}
		for _, info := range registry.Inventory() {
			enabled[info.ID] = info.Enabled
			if info.ID == "geojson" && info.Version != geojsonmodule.Version() {
				t.Fatalf("module %s must report the selected GeoJSON dependency version", info.ID)
			}
		}
		for _, identity := range []string{"anonymous", "no-permission", "audit-reader", "map-reader"} {
			r := gin.New()
			protected := r.Group("/api/v1", func(c *gin.Context) {
				if identity == "anonymous" {
					return
				}
				actor := authz.Context{UserID: "fixture"}
				if identity == "audit-reader" {
					actor.IsAdmin = true
					actor.Permissions = []authz.Permission{authz.PermissionAuditRead}
				}
				if identity == "map-reader" {
					actor.Permissions = []authz.Permission{authz.PermissionSurveillanceRead}
				}
				c.Set("user_id", actor.UserID)
				c.Set(authz.ContextKey, actor)
			}, middleware.RequireAuth())
			registry.RegisterRoutes(module.Routes{Protected: protected, Admin: protected.Group("/admin")})
			for _, endpoint := range []struct{ id, path, allowed string }{
				{"audit", "/api/v1/admin/audit-logs/actions", "audit-reader"},
				{"geojson", "/api/v1/geojson/geo_water", "map-reader"},
			} {
				response := httptest.NewRecorder()
				r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, endpoint.path, nil))
				want := http.StatusForbidden
				if !enabled[endpoint.id] {
					want = http.StatusNotFound
				} else if identity == "anonymous" {
					want = http.StatusUnauthorized
				} else if identity == endpoint.allowed {
					want = http.StatusOK
				}
				if response.Code != want {
					t.Fatalf("disabled=%q identity=%s path=%s: expected %d, got %d", disabled, identity, endpoint.path, want, response.Code)
				}
				if endpoint.id == "geojson" && want == http.StatusOK && response.Header().Get("Content-Type") != "application/geo+json" {
					t.Fatal("GeoJSON media type changed")
				}
			}
		}
	}
}

func testAuditModule(t *testing.T, repository auditfeature.Repository) *auditfeature.Module {
	t.Helper()
	m, err := auditfeature.NewModule(auditfeature.Dependencies{
		Repository: repository, RequireAdmin: modulehttp.RequireAdmin, AuthorizeAny: modulehttp.AuthorizeAny,
		LimitExport: func(*http.Request, string, int, time.Duration) (http.Header, error) { return nil, nil },
		RequestID:   modulehttp.RequestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
