package report_scheduler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		{"permission without system", &authz.Context{Permissions: []authz.Permission{authz.PermissionReportSchedulerRead}}, http.StatusForbidden},
		{"system without permission", &authz.Context{AccessibleSystems: []string{authz.SystemDataStatistics}}, http.StatusForbidden},
		{"assigned parent client role", &allowed, http.StatusOK},
		{"parent access without module permission", contextPtr(authz.NewContext("user-2", nil, map[string][]string{authz.SystemDataStatistics: {authz.DataStatisticsAccess}})), http.StatusForbidden},
		{"retired standalone client", contextPtr(authz.NewContext("user-3", nil, map[string][]string{"report-scheduler": {authz.ReportSchedulerAccess}})), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			group := router.Group("/api/v1", func(c *gin.Context) {
				if tt.auth != nil {
					c.Set(authz.ContextKey, *tt.auth)
				}
			})
			RegisterProtectedRoutes(group, NewHandler(NewService()))
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/report-scheduler", nil))
			if res.Code != tt.status {
				t.Fatalf("expected %d, got %d: %s", tt.status, res.Code, res.Body.String())
			}
			if tt.status == http.StatusOK {
				var body struct {
					Success bool           `json:"success"`
					Data    ModuleResponse `json:"data"`
				}
				if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if !body.Success || body.Data.SchedulingEnabled || body.Data.Status != "setup" {
					t.Fatalf("unexpected module response: %+v", body)
				}
			}
		})
	}
}

func contextPtr(ctx authz.Context) *authz.Context { return &ctx }
