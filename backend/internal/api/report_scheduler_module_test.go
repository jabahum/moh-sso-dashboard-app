package router

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/module"
	scheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

type schedulerModuleFixture struct {
	scheduler.Repository
	heartbeats atomic.Int32
	polls      atomic.Int32
	readyErr   error
}

func (f *schedulerModuleFixture) UpdateWorkerHeartbeat(context.Context, string, time.Duration, time.Time, *time.Time, *time.Time, string) error {
	f.heartbeats.Add(1)
	return f.readyErr
}
func (f *schedulerModuleFixture) ClaimGeneratingExecutions(context.Context, int) ([]scheduler.Execution, error) {
	f.polls.Add(1)
	return nil, nil
}
func (*schedulerModuleFixture) ClaimRetryExecution(context.Context, time.Time) (scheduler.Execution, bool, error) {
	return scheduler.Execution{}, false, nil
}
func (*schedulerModuleFixture) ClaimDueExecution(context.Context, time.Time) (scheduler.Schedule, scheduler.Execution, bool, error) {
	return scheduler.Schedule{}, scheduler.Execution{}, false, nil
}
func (*schedulerModuleFixture) ClaimDueDelivery(context.Context, time.Time) (scheduler.Delivery, bool, error) {
	return scheduler.Delivery{}, false, nil
}
func (*schedulerModuleFixture) GetDeliveryByAccessToken(context.Context, string) (scheduler.Delivery, scheduler.Artifact, scheduler.Execution, error) {
	return scheduler.Delivery{}, scheduler.Artifact{}, scheduler.Execution{}, scheduler.ErrScheduleNotFound
}

func TestSchedulerModuleControlsRoutesAndWorker(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, disabled := range []bool{false, true} {
		disabledIDs := "metrics,audit,geojson,announcements,admin-units," + extractedModules
		if disabled {
			disabledIDs += ", report-scheduler "
		}
		fixture := &schedulerModuleFixture{}
		service := scheduler.NewService(fixture, nil, nil, nil, nil, nil)
		registry, err := NewModuleRegistry(HandlerSet{ReportScheduler: NewReportSchedulerModule(service)}, disabledIDs, nil, scheduler.NewWorker(service, time.Hour, nil))
		if err != nil {
			t.Fatal(err)
		}
		stop, err := registry.Initialize(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, identity := range []string{"anonymous", "no-permission", "reader"} {
			r := gin.New()
			r.Use(middleware.RequestContext())
			public := r.Group("/api/v1")
			protected := r.Group("/api/v1", func(c *gin.Context) {
				if identity == "anonymous" {
					return
				}
				c.Set("user_id", "fixture")
				actor := authz.Context{UserID: "fixture"}
				if identity == "reader" {
					actor.AccessibleSystems = []string{authz.SystemDataStatistics}
					actor.Permissions = []authz.Permission{authz.PermissionReportSchedulerRead}
				}
				c.Set(authz.ContextKey, actor)
			}, middleware.RequireAuth())
			registry.RegisterRoutes(module.Routes{Public: public, Protected: protected, Admin: protected.Group("/admin")})
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/report-scheduler", nil)
			req.Header.Set("X-Request-ID", "scheduler-trace")
			r.ServeHTTP(res, req)
			want := map[string]int{"anonymous": 401, "no-permission": 403, "reader": 200}[identity]
			if disabled {
				want = 404
			}
			if res.Code != want {
				t.Fatalf("disabled=%v identity=%s: got %d want %d", disabled, identity, res.Code, want)
			}
			if !disabled {
				var body struct {
					Meta  struct{ RequestID string }
					Error struct{ Code string }
				}
				if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Meta.RequestID != "scheduler-trace" || res.Header().Get("X-Request-ID") != body.Meta.RequestID {
					t.Fatalf("request ID not propagated: %s", res.Body.String())
				}
				if code := map[string]string{"anonymous": "UNAUTHORIZED", "no-permission": "FORBIDDEN"}[identity]; body.Error.Code != code {
					t.Fatalf("unexpected error code for %s: %s", identity, res.Body.String())
				}
			}
			res = httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/report-scheduler/public/deliveries/invalid/download", nil))
			if res.Code != 404 || (!disabled && !strings.Contains(res.Body.String(), "DELIVERY_LINK_INVALID")) || (disabled && strings.Contains(res.Body.String(), "DELIVERY_LINK_INVALID")) {
				t.Fatalf("public route disablement=%v: %s", disabled, res.Body.String())
			}
		}
		if err := stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if disabled && (fixture.heartbeats.Load() != 0 || fixture.polls.Load() != 0) {
			t.Fatal("disabled scheduler touched persistence")
		}
		if !disabled && fixture.heartbeats.Load() == 0 {
			t.Fatal("enabled scheduler skipped readiness")
		}
	}
}

func TestSchedulerRegistryFailsStartupBeforeClaims(t *testing.T) {
	for _, missing := range []string{"handler", "worker", "persistence"} {
		fixture := &schedulerModuleFixture{readyErr: errors.New("database unavailable")}
		service := scheduler.NewService(fixture, nil, nil, nil, nil, nil)
		handlers := HandlerSet{ReportScheduler: NewReportSchedulerModule(service)}
		worker := scheduler.NewWorker(service, time.Hour, nil)
		if missing == "handler" {
			handlers.ReportScheduler = nil
		}
		if missing == "worker" {
			worker = nil
		}
		registry, err := NewModuleRegistry(handlers, "metrics,audit,geojson,announcements,admin-units,"+extractedModules, nil, worker)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := registry.Initialize(context.Background()); err == nil || fixture.polls.Load() != 0 {
			t.Fatalf("invalid %s accepted: %v", missing, err)
		}
	}
}
