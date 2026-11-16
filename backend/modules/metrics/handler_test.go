package metrics_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	metrics "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
	"github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics/model"
)

type repository struct {
	metrics.Repository
	err   error
	start time.Time
	end   time.Time
}

func (r *repository) CountUsers(context.Context) (int64, error) { return 42, r.err }

func (r *repository) LoginTrendByDay(_ context.Context, start, end time.Time) ([]model.LoginTrendByDayRow, error) {
	r.start, r.end = start, end
	return []model.LoginTrendByDayRow{{Day: start, SuccessCount: 12, FailureCount: 2}}, r.err
}

func (r *repository) NewUsersInRange(context.Context, time.Time, time.Time) ([]model.User, error) {
	return []model.User{{
		ID: uuid.MustParse("6596c0c5-4446-4caa-9bd0-c7ad991567ba"), Username: "test-user", Email: "test@example.org",
		Enabled: sql.NullBool{Bool: false, Valid: true}, FirstName: sql.NullString{String: "Jeremy", Valid: true},
	}}, r.err
}

func testRouter(repo metrics.Repository, authorize gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("request_id", "module-test-request") })
	metrics.RegisterAdminRoutes(r.Group("/api/v1/admin"), metrics.NewHandler(metrics.NewService(repo)), authorize)
	return r
}

func TestIndependentModuleHTTPContract(t *testing.T) {
	repo := &repository{}
	r := testRouter(repo, func(c *gin.Context) { c.Next() })
	for _, path := range []string{
		"/system/count-users",
		"/system/login-trend-range?start=2026-01-01T00:00:00Z&end=2026-02-01T00:00:00Z",
		"/users/new-range?start=2026-01-01T00:00:00Z&end=2026-02-01T00:00:00Z",
	} {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics"+path, nil))
		if res.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
		var body struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
			Meta    struct {
				RequestID string `json:"requestId"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if !body.Success || body.Meta.RequestID != "module-test-request" {
			t.Fatalf("response envelope changed: %s", res.Body.String())
		}
		switch path {
		case "/system/count-users":
			if string(body.Data) != `{"total_users":42}` {
				t.Fatalf("count payload changed: %s", body.Data)
			}
		case "/system/login-trend-range?start=2026-01-01T00:00:00Z&end=2026-02-01T00:00:00Z":
			if string(body.Data) != `[{"day":"2026-01-01T00:00:00Z","success_count":12,"failure_count":2}]` || repo.start.Month() != time.January || repo.end.Month() != time.February {
				t.Fatalf("range or payload changed: %s", body.Data)
			}
		default:
			if string(body.Data) != `[{"id":"6596c0c5-4446-4caa-9bd0-c7ad991567ba","username":"test-user","first_name":"Jeremy","email":"test@example.org","enabled":false}]` {
				t.Fatalf("nullable user mapping changed: %s", body.Data)
			}
		}
	}
}

func TestIndependentModuleValidationAndErrors(t *testing.T) {
	r := testRouter(&repository{err: errors.New("database failure")}, func(c *gin.Context) { c.Next() })
	for _, tc := range []struct {
		path   string
		status int
		code   string
	}{
		{"/system/count-users", 500, "INTERNAL_ERROR"},
		{"/system/login-trend-range", 400, "VALIDATION_FAILED"},
		{"/system/login-trend-range?start=bad&end=bad", 400, "INVALID_DATE_FORMAT"},
		{"/clients/login-count", 400, "CLIENT_ID NOT FOUND"},
		{"/users/last-login/invalid", 400, "INVALID_UUID"},
		{"/users/client-usage/invalid", 502, "INVALID_UUID"},
	} {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics"+tc.path, nil))
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if res.Code != tc.status || body.Error.Code != tc.code {
			t.Fatalf("%s: expected %d/%s, got %d %s", tc.path, tc.status, tc.code, res.Code, res.Body.String())
		}
	}
}

func TestEveryModuleRouteRequiresHostAuthorization(t *testing.T) {
	r := testRouter(nil, func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) })
	if len(r.Routes()) != 19 {
		t.Fatalf("expected 19 existing metrics routes, got %d", len(r.Routes()))
	}
	for _, route := range r.Routes() {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(route.Method, route.Path, nil))
		if res.Code != http.StatusForbidden {
			t.Fatalf("authorization bypass on %s: %d", route.Path, res.Code)
		}
	}
}

func TestModuleRejectsMissingAuthorizationMiddleware(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("missing authorization accepted")
		}
	}()
	testRouter(nil, nil)
}
