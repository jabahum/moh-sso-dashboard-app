package router

import (
	"context"
	"encoding/json"
	scheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
)

func TestModuleAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	allowed := authz.NewContext("user-1", nil, map[string][]string{authz.SystemDataStatistics: {authz.ReportSchedulerAccess}})
	tests := []struct {
		name   string
		auth   *authz.Context
		status int
	}{
		{"unauthenticated", nil, http.StatusUnauthorized},
		{"permission without system", &authz.Context{UserID: "user-1", Permissions: []authz.Permission{authz.PermissionReportSchedulerRead}}, http.StatusForbidden},
		{"system without permission", &authz.Context{UserID: "user-1", AccessibleSystems: []string{authz.SystemDataStatistics}}, http.StatusForbidden},
		{"assigned parent client role", &allowed, http.StatusOK},
		{"parent access without module permission", contextPtr(authz.NewContext("user-2", nil, map[string][]string{authz.SystemDataStatistics: {authz.DataStatisticsAccess}})), http.StatusForbidden},
		{"retired standalone client", contextPtr(authz.NewContext("user-3", nil, map[string][]string{"report-scheduler": {authz.ReportSchedulerAccess}})), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			group := router.Group("/api/v1", func(c *gin.Context) {
				if tt.auth != nil {
					c.Set("health_context", map[string]string{"district": "Kampala"})
					c.Set(authz.ContextKey, *tt.auth)
				}
			})
			registerProtectedSchedulerTestRoutes(group, NewReportSchedulerModule(scheduler.NewService(nil, nil, nil, nil, nil, nil)))
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/report-scheduler", nil))
			if res.Code != tt.status {
				t.Fatalf("expected %d, got %d: %s", tt.status, res.Code, res.Body.String())
			}
			if tt.status == http.StatusOK {
				var body struct {
					Success bool                     `json:"success"`
					Data    scheduler.ModuleResponse `json:"data"`
				}
				if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if !body.Success || body.Data.HealthContext.District != "Kampala" || body.Data.SchedulingEnabled || body.Data.HealthBIEnabled || body.Data.Status != "ready" {
					t.Fatalf("unexpected module response: %+v", body)
				}
			}
		})
	}
}

func contextPtr(ctx authz.Context) *authz.Context { return &ctx }

func TestParseListOptionsValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name         string
		target       string
		allowEnabled bool
		wantOK       bool
	}{
		{name: "default", target: "/items", allowEnabled: true, wantOK: true},
		{name: "max limit", target: "/items?limit=200", allowEnabled: true, wantOK: true},
		{name: "limit too high", target: "/items?limit=201", allowEnabled: true, wantOK: false},
		{name: "invalid enabled", target: "/items?enabled=maybe", allowEnabled: true, wantOK: false},
		{name: "valid execution status", target: "/items?status=failed", allowEnabled: false, wantOK: true},
		{name: "invalid execution status", target: "/items?status=unknown", allowEnabled: false, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			group, router := schedulerTestRouter()
			registerProtectedSchedulerTestRoutes(group, NewReportSchedulerModule(scheduler.NewService(schedulerListRepository{}, nil, nil, nil, nil, nil)))
			path := "/schedules"
			if !tt.allowEnabled {
				path = "/executions"
			}
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/report-scheduler"+strings.Replace(tt.target, "/items", path, 1), nil))
			ok := res.Code == http.StatusOK
			if ok != tt.wantOK || (!tt.wantOK && res.Code != http.StatusBadRequest) {
				t.Fatalf("expected ok=%v, got %v; body=%s", tt.wantOK, ok, res.Body.String())
			}
		})
	}
}

func schedulerTestRouter() (*gin.RouterGroup, *gin.Engine) {
	router := gin.New()
	group := router.Group("/api/v1", func(c *gin.Context) {
		c.Set(authz.ContextKey, authz.NewContext("user-1", nil, map[string][]string{
			authz.SystemDataStatistics: {authz.ReportSchedulerAccess},
		}))
	})
	return group, router
}

type schedulerListRepository struct{ scheduler.Repository }

func (schedulerListRepository) ListSchedules(context.Context, string, bool, scheduler.HealthContext, scheduler.ListOptions) ([]scheduler.Schedule, error) {
	return nil, nil
}

func (schedulerListRepository) ListExecutions(context.Context, string, bool, scheduler.HealthContext, scheduler.ListOptions) ([]scheduler.Execution, error) {
	return nil, nil
}

func registerProtectedSchedulerTestRoutes(group *gin.RouterGroup, handler *scheduler.HTTPModule) {
	RegisterReportScheduler(nil, group, handler, nil)
}
