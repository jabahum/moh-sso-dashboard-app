package dataquality

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// ScheduleWorker executes scans that were queued to run later.
type ScheduleWorker struct {
	store    *DQAStore
	dwhDB    *sql.DB
	interval time.Duration
	log      *slog.Logger
}

// NewScheduleWorker builds the worker. A non-positive interval means one minute.
func NewScheduleWorker(store *DQAStore, dwhDB *sql.DB, interval time.Duration, log *slog.Logger) *ScheduleWorker {
	if interval <= 0 {
		interval = time.Minute
	}
	if log == nil {
		log = slog.Default()
	}
	return &ScheduleWorker{store: store, dwhDB: dwhDB, interval: interval, log: log}
}

func (w *ScheduleWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.drain(ctx)
		}
	}
}

// drain runs every due scan, one at a time, so a backlog cannot start a dozen
// warehouse scans at once.
func (w *ScheduleWorker) drain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		scheduled, found, err := w.store.claimDueRun(ctx, time.Now())
		if err != nil {
			w.log.Error("dqa: claiming a scheduled run failed", "error", err)
			return
		}
		if !found {
			return
		}

		result, runErr := w.store.RunTable(ctx, w.dwhDB, scheduled.TableID, scheduled.Scope, scheduled.CreatedBy)
		var runID int64
		if runErr == nil {
			runID = result.RunID
		} else {
			w.log.Error("dqa: scheduled run failed", "error", runErr)
		}
		if err := w.store.finishScheduledRun(ctx, scheduled.ID, runID, runErr); err != nil {
			w.log.Error("dqa: recording a scheduled run result failed", "error", err)
		}
	}
}
