package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
	repository "github.com/moh-sso-dashboard/internal/repository/metrics"
	"github.com/moh-sso-dashboard/internal/service"
)

type countRepository struct{ repository.MetricsRepository }

func (*countRepository) CountUsers(context.Context) (int64, error) { return 7, nil }

func TestModulePreservesHostPermissionsAndRequestMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, allowed := range []bool{false, true} {
		r := gin.New()
		r.Use(middleware.RequestContext(), func(c *gin.Context) {
			permissions := []authz.Permission{}
			if allowed {
				permissions = append(permissions, authz.PermissionMetricsRead)
			}
			c.Set(authz.ContextKey, authz.Context{UserID: "test-user", Permissions: permissions})
		})
		RegisterAdminRoutes(r.Group("/api/v1/admin"), NewHandler(service.NewMetricsService(&countRepository{})))
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics/system/count-users", nil)
		request.Header.Set("X-Request-ID", "metrics-boundary-test")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, request)
		if allowed {
			if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"total_users":7`) || !strings.Contains(res.Body.String(), `"requestId":"metrics-boundary-test"`) {
				t.Fatalf("host response contract changed: %d %s", res.Code, res.Body.String())
			}
		} else if res.Code != http.StatusForbidden {
			t.Fatalf("metrics permission bypassed: %d", res.Code)
		}
	}
}
