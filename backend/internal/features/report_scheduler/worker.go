package report_scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Worker struct {
	service  *Service
	interval time.Duration
	log      func(args ...any)
	workerID string
}

func NewWorker(service *Service, interval time.Duration, log func(args ...any)) *Worker {
	if interval <= 0 { interval = 30 * time.Second }
	return &Worker{service: service, interval: interval, log: log, workerID: uuid.NewString()}
}

func (w *Worker) Start(ctx context.Context) error {
	if w == nil || w.service == nil { return errors.New("report scheduler worker service is required") }
	// Process immediately on startup so schedules due during a restart do not
	// wait for the first polling interval.
	w.process(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return ctx.Err()
		case <-ticker.C: w.process(ctx)
		}
	}
}

func (w *Worker) process(ctx context.Context) {
	if ctx.Err() != nil { return }
	started := time.Now().UTC()
	_ = w.service.UpdateWorkerHeartbeat(ctx, w.workerID, started, &started, nil, "")
	cycleErr := ""
	defer func() {
		finished := time.Now().UTC()
		_ = w.service.UpdateWorkerHeartbeat(ctx, w.workerID, finished, nil, &finished, cycleErr)
	}()
	if err := w.service.PollGenerating(ctx, 50); err != nil { cycleErr = err.Error(); w.logf("report scheduler: poll Health BI jobs failed: ", err) }
	for processed := 0; processed < 100; processed++ {
		if ctx.Err() != nil { return }
		found, err := w.service.ProcessOneRetry(ctx)
		if err != nil { cycleErr = err.Error(); w.logf("report scheduler: processing generation retry failed: ", err); return }
		if !found { break }
	}
	for processed := 0; processed < 100; processed++ {
		if ctx.Err() != nil { return }
		found, err := w.service.ProcessOneDue(ctx)
		if err != nil { cycleErr = err.Error(); w.logf("report scheduler: processing due schedule failed: ", err); return }
		if !found { break }
	}
	for processed := 0; processed < 200; processed++ {
		if ctx.Err() != nil { return }
		found, err := w.service.ProcessOneDelivery(ctx)
		if err != nil { cycleErr = err.Error(); w.logf("report scheduler: processing delivery failed: ", err); return }
		if !found { return }
	}
}

func (w *Worker) logf(args ...any) { if w.log != nil { w.log(args...) } }
