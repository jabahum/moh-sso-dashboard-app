package router

import (
	"context"
	"errors"
	"github.com/moh-sso-dashboard/module"
	scheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
	"sync/atomic"
	"testing"
	"time"
)

type lifecycleService struct {
	readyErr error
	poll     func(context.Context) error
	cycles   atomic.Int32
}

func (s *lifecycleService) UpdateWorkerHeartbeat(context.Context, string, time.Duration, time.Time, *time.Time, *time.Time, string) error {
	return s.readyErr
}
func (s *lifecycleService) PollGenerating(ctx context.Context, _ int) error {
	s.cycles.Add(1)
	if s.poll != nil {
		return s.poll(ctx)
	}
	return nil
}
func (*lifecycleService) ProcessOneRetry(context.Context) (bool, error)    { return false, nil }
func (*lifecycleService) ProcessOneDue(context.Context) (bool, error)      { return false, nil }
func (*lifecycleService) ProcessOneDelivery(context.Context) (bool, error) { return false, nil }

func waitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach expected lifecycle state")
	}
}

func TestWorkerReadinessFailureNeverClaimsJobs(t *testing.T) {
	failure := errors.New("runtime table unavailable")
	s := &lifecycleService{readyErr: failure}
	stop, err := scheduler.NewWorker(s, time.Hour, nil).Initialize(context.Background())
	if !errors.Is(err, failure) || stop != nil || s.cycles.Load() != 0 {
		t.Fatalf("readiness failure: stop=%v err=%v cycles=%d", stop != nil, err, s.cycles.Load())
	}
	var service *scheduler.Service
	if _, err := scheduler.NewWorker(service, time.Hour, nil).Initialize(context.Background()); err == nil {
		t.Fatal("nil service passed readiness")
	}
	if _, err := scheduler.NewWorker(scheduler.NewService(nil, nil, nil, nil, nil, nil), time.Hour, nil).Initialize(context.Background()); err == nil {
		t.Fatal("missing repository passed readiness")
	}
}

func TestWorkerStopCancelsAndJoinsInFlightCycle(t *testing.T) {
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s := &lifecycleService{poll: func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	}}
	w := scheduler.NewWorker(s, time.Hour, nil)
	stop, err := w.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	waitSignal(t, entered)
	if _, err := w.Initialize(context.Background()); err == nil {
		t.Fatal("duplicate worker start accepted")
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- stop(shutdown) }()
	waitSignal(t, canceled)
	select {
	case err := <-stopped:
		t.Fatalf("shutdown returned before cycle joined: %v", err)
	default:
	}
	close(release)
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-shutdown.Done():
		t.Fatal("shutdown failed to join")
	}
	if err := stop(shutdown); err != nil {
		t.Fatal("repeated cleanup failed:", err)
	}
	if s.cycles.Load() != 1 {
		t.Fatalf("unexpected additional cycles: %d", s.cycles.Load())
	}
}

func TestWorkerShutdownDeadlineAndParentCancellation(t *testing.T) {
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s := &lifecycleService{poll: func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	stop, err := scheduler.NewWorker(s, time.Hour, nil).Initialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waitSignal(t, entered)
	cancel()
	waitSignal(t, canceled)
	expired, expire := context.WithCancel(context.Background())
	expire()
	if err := stop(expired); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown ignored deadline: %v", err)
	}
	close(release)
	joined, finish := context.WithTimeout(context.Background(), 3*time.Second)
	defer finish()
	if err := stop(joined); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledWorkerNeverStarts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &lifecycleService{}
	if _, err := scheduler.NewWorker(s, time.Hour, nil).Initialize(ctx); !errors.Is(err, context.Canceled) || s.cycles.Load() != 0 {
		t.Fatalf("canceled startup: %v", err)
	}
}

type blockedReadiness struct{ lifecycleService }

func (*blockedReadiness) UpdateWorkerHeartbeat(ctx context.Context, _ string, _ time.Duration, _ time.Time, _, _ *time.Time, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestReadinessRespectsStartupDeadline(t *testing.T) {
	s := &blockedReadiness{}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := scheduler.NewWorker(s, time.Hour, nil).Initialize(ctx); !errors.Is(err, context.DeadlineExceeded) || s.cycles.Load() != 0 {
		t.Fatalf("startup deadline ignored: %v", err)
	}
}

func TestRegistryRollbackJoinsSchedulerWorker(t *testing.T) {
	entered, exited := make(chan struct{}), make(chan struct{})
	s := &lifecycleService{poll: func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	}}
	worker := scheduler.NewWorker(s, time.Hour, nil)
	failure := errors.New("later module failed")
	registry, err := module.New(
		module.Definition{Info: module.Info{ID: "report-scheduler", Version: "test", Enabled: true}, Initialize: func(ctx context.Context) (module.Stop, error) { return worker.Initialize(ctx) }},
		module.Definition{Info: module.Info{ID: "later", Version: "test", Enabled: true}, Initialize: func(context.Context) (module.Stop, error) {
			waitSignal(t, entered)
			return nil, failure
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Initialize(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("rollback failure: %v", err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("registry rollback returned before worker exit")
	}
}
