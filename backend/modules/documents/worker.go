package documents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// DefaultPollInterval is the delay between queue polls.
const DefaultPollInterval = 3 * time.Second

// Worker claims queued document processing jobs one at a time and hands each
// to the host's ProcessExecutor. Claims use FOR UPDATE SKIP LOCKED, so several
// replicas can poll the same queue.
type Worker struct {
	repo      Repository
	executor  ProcessExecutor
	pollDelay time.Duration
	logger    *slog.Logger
}

// NewWorker builds the worker. pollDelay <= 0 selects five seconds.
func NewWorker(repo Repository, executor ProcessExecutor, pollDelay time.Duration, logger *slog.Logger) (*Worker, error) {
	if repo == nil {
		return nil, errors.New("process repository is required")
	}
	if executor == nil {
		return nil, errors.New("document process executor is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if pollDelay <= 0 {
		pollDelay = 5 * time.Second
	}
	return &Worker{repo: repo, executor: executor, pollDelay: pollDelay, logger: logger}, nil
}

// Initialize starts the worker and returns a cleanup that cancels it and
// waits for the in-flight job to return.
func (w *Worker) Initialize(ctx context.Context) (func(context.Context) error, error) {
	if w == nil {
		return nil, errors.New("document worker is nil")
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = w.Start(workerCtx)
	}()
	return func(shutdownCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		default:
		}
		select {
		case <-done:
			return nil
		case <-shutdownCtx.Done():
			return shutdownCtx.Err()
		}
	}, nil
}

// Start polls until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	if w == nil {
		return errors.New("document worker is nil")
	}

	w.logger.Info("document worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("document worker shutting down")
			return nil

		default:
			if err := w.processNext(ctx); err != nil {
				w.logger.Error("document worker processing failed", "error", err)
			}

			sleepWithContext(ctx, w.pollDelay)
		}
	}
}

func (w *Worker) processNext(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("worker panic recovered", "worker", "document worker", "panic", r)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	proc, err := w.repo.ClaimNextPendingProcess(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return fmt.Errorf("claim next pending process: %w", err)
	}

	w.logger.Info("processing document job", "process_id", proc.ID, "process_type", proc.ProcessType)

	if err := w.executor.Execute(ctx, proc.ID); err != nil {
		return fmt.Errorf("execute document job %s: %w", proc.ID, err)
	}

	w.logger.Info("document job completed", "process_id", proc.ID)

	return nil
}

func sleepWithContext(ctx context.Context, delay time.Duration) {
	if delay <= 0 {
		return
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
