package platform

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/moh-sso-dashboard/internal/authz"
	"net/http/httptest"
	"strings"
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

type profileHealthRepository struct {
	operationsTestRepository
	calls int
}

func (r *profileHealthRepository) WorkerHealth(ctx context.Context, stale time.Duration) ([]WorkerHealth, error) {
	r.calls++
	return r.operationsTestRepository.WorkerHealth(ctx, stale)
}
func TestProfileReadinessRequiresOnlyEnabledWorkersAndMandatoryCore(t *testing.T) {
	repo := &profileHealthRepository{operationsTestRepository: operationsTestRepository{
		health: []WorkerHealth{{Name: "email-outbox", Healthy: true}, {Name: "notification-email", Healthy: false}},
	}}
	h := &Handler{Repository: repo, RequiredWorkers: []string{}, CoreReady: func(context.Context) error { return nil }}
	r := gin.New()
	r.GET("/ready", h.GetReadiness)
	request := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/ready", nil))
		return w.Code
	}
	if code := request(); code != 200 || repo.calls != 0 {
		t.Fatal("core-only readiness queried disabled workers", code, repo.calls)
	}
	h.RequiredWorkers = []string{"email-outbox"}
	local := &core.WorkerStatus{}
	local.RecordPoll(time.Now(), errors.New("private provider detail"))
	h.WorkerStatuses = map[string]*core.WorkerStatus{"email-outbox": local}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ready", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"degraded"`) ||
		!strings.Contains(w.Body.String(), `"lastFailure":`) || strings.Contains(w.Body.String(), "private provider detail") {
		t.Fatalf("unexpected readiness with local worker status: %d %s", w.Code, w.Body.String())
	}
	h.RequiredWorkers = []string{"missing-worker"}
	if code := request(); code != 503 {
		t.Fatal("missing enabled worker reported healthy", code)
	}
	h.RequiredWorkers = []string{}
	h.CoreReady = func(context.Context) error { return errors.New("primary down") }
	if code := request(); code != 503 {
		t.Fatal("mandatory failure reported ready", code)
	}
	delivery := false
	h.DeliveryEnabled = &delivery
	routes := gin.New()
	RegisterRoutes(routes.Group("/api/v1"), routes.Group("/api/v1"), h, nil)
	for _, route := range routes.Routes() {
		if strings.Contains(route.Path, "/platform/notifications") {
			t.Fatal("disabled delivery route registered", route.Path)
		}
	}
}
