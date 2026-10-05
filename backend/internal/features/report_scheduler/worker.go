package report_scheduler

import (
	"context"
	"errors"
	"time"
)

type Worker struct {
	service  *Service
	interval time.Duration
	log      func(args ...any)
}

func NewWorker(service *Service, interval time.Duration, log func(args ...any)) *Worker {
	if interval <= 0 { interval = 30 * time.Second }
	return &Worker{service: service, interval: interval, log: log}
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
	if err := w.service.PollGenerating(ctx, 50); err != nil { w.logf("report scheduler: poll Health BI jobs failed: ", err) }
	for {
		if ctx.Err() != nil { return }
		found, err := w.service.ProcessOneRetry(ctx)
		if err != nil { w.logf("report scheduler: processing generation retry failed: ", err); return }
		if !found { break }
	}
	for {
		if ctx.Err() != nil { return }
		found, err := w.service.ProcessOneDue(ctx)
		if err != nil { w.logf("report scheduler: processing due schedule failed: ", err); return }
		if !found { break }
	}
	for {
		if ctx.Err() != nil { return }
		found, err := w.service.ProcessOneDelivery(ctx)
		if err != nil { w.logf("report scheduler: processing delivery failed: ", err); return }
		if !found { return }
	}
}

func (w *Worker) logf(args ...any) { if w.log != nil { w.log(args...) } }
