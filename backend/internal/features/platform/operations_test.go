package platform

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"net/http/httptest"
	"testing"
	"time"
)

type operationsTestRepository struct {
	testRepository
	health []WorkerHealth
}

func (r *operationsTestRepository) WorkerHealth(context.Context, time.Duration) ([]WorkerHealth, error) {
	return r.health, nil
}
func (r *operationsTestRepository) Operations(_ context.Context, app string) (ApplicationOperations, error) {
	return ApplicationOperations{ApplicationID: app, StatusCounts24Hours: map[string]int{}}, nil
}
func TestReadinessRequiresBothActualWorkersAndOperationsAllowsReadOnlyGovernance(t *testing.T) {
	repo := &operationsTestRepository{testRepository: testRepository{principal: Principal{ApplicationID: "pilot", Scopes: []string{IdentityRead}}}, health: []WorkerHealth{{Name: "email-outbox", Healthy: true}, {Name: "notification-email", Healthy: false}}}
	h := &Handler{Repository: repo, Verifier: testVerifier{token: validToken()}}
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(authz.ContextKey, authz.Context{UserID: "reader", Permissions: []authz.Permission{authz.PermissionRBACRead}})
	})
	RegisterRoutes(router.Group("/api/v1"), protected, h, nil)
	request := func(path string) int {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	if code := request("/api/v1/platform/readiness"); code != 503 {
		t.Fatal("stale worker reported ready", code)
	}
	repo.health[1].Healthy = true
	if code := request("/api/v1/platform/readiness"); code != 200 {
		t.Fatal(code)
	}
	if code := request("/api/v1/admin/platform/applications/pilot/operations"); code != 200 {
		t.Fatal("read-only operator required write privileges", code)
	}
}
