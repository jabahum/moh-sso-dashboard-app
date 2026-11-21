package bootstrap

import (
	"context"
	"errors"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/log"
)

func TestCoreCompositionOmitsOptionalConstructorsAndRoutes(t *testing.T) {
	cfg := &config.Config{PlatformProfile: config.ProfileCore, FrontendBaseURL: "https://portal.example.org"}
	log := logger.NewLogger()
	repos := buildRepositories(cfg, nil, nil, nil, nil, log)
	if repos.Surveillance != nil || repos.Email != nil || repos.Documents != nil || repos.NotificationDelivery != nil {
		t.Fatal("optional repositories constructed")
	}
	services := buildServices(serviceDependencies{Config: cfg, Repositories: repos, Logger: log})
	if services.RBAC == nil || services.HealthContexts == nil || services.Auth == nil || services.Audit == nil || services.Sessions == nil || services.Notifications == nil {
		t.Fatal("missing core capability")
	}
	if services.SMTP != nil || services.Email != nil || services.SMS != nil || services.Documents != nil || services.ReportScheduler != nil {
		t.Fatal("optional service constructed")
	}
	handlers := buildHandlers(handlerDependencies{Config: cfg, Services: services})
	modules, err := router.NewModuleRegistry(handlers.Router, cfg.EffectiveDisabledModules(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := modules.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop(context.Background())
	r := router.SetupRouter(router.RouterDependencies{Config: cfg, Modules: modules, Handlers: handlers.Router})
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		routes[route.Path] = true
	}
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/admin/modules", "/api/v1/me/health-contexts", "/api/v1/admin/rbac/systems", "/api/v1/platform/readiness"} {
		if !routes[path] {
			t.Fatal("missing core route", path)
		}
	}
	for _, path := range []string{"/api/v1/users", "/api/v1/clients", "/api/v1/documents", "/api/v1/email/send", "/api/v1/admin/audit-logs", "/api/v1/admin/metrics", "/api/v1/platform/notifications"} {
		if routes[path] {
			t.Fatal("optional route remains", path)
		}
	}
	stopWorkers, err := startBackgroundWorkers(context.Background(), workerDependencies{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if err := stopWorkers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if schedulerWorker(cfg, services, nil) != nil {
		t.Fatal("scheduler constructed")
	}
	if !reflect.DeepEqual(enabledDeliveryWorkers(cfg), []string{}) {
		t.Fatal("core expects delivery workers")
	}
}
func TestWorkerStopJoinsAndCanRetryAfterDeadline(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var finished atomic.Bool
	stop := core.StartWorkerGroup(context.Background(), []core.WorkerTask{{Name: "blocked", Run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		<-release
		finished.Store(true)
		return ctx.Err()
	}}}, nil)
	<-started
	deadline, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := stop(deadline); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("stop did not respect deadline", err)
	}
	if finished.Load() {
		t.Fatal("worker exited before release")
	}
	close(release)
	if err := stop(context.Background()); err != nil || !finished.Load() {
		t.Fatal("stop failed to join", err)
	}
	if err := stop(context.Background()); err != nil {
		t.Fatal("stop not idempotent", err)
	}
}
func TestWorkerFailureIsPreservedAtShutdown(t *testing.T) {
	cause := errors.New("worker failed")
	stop := core.StartWorkerGroup(context.Background(), []core.WorkerTask{{Name: "fixture", Run: func(context.Context) error { return cause }}}, nil)
	if err := stop(context.Background()); !errors.Is(err, cause) {
		t.Fatal("worker error lost", err)
	}
}

func TestDisabledAnnouncementsDoNotConstructProviders(t *testing.T) {
	for _, cfg := range []*config.Config{
		{PlatformProfile: config.ProfileCore},
		{PlatformProfile: config.ProfileFull, DisabledModules: "announcements"},
	} {
		if module := buildAnnouncementsModule(handlerDependencies{Config: cfg}); module != nil {
			t.Fatal("disabled module constructed with unavailable provider dependencies")
		}
	}
}
