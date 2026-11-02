package worker

import (
	"context"
	"errors"
	"time"

	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/model"
	repository "github.com/moh-sso-dashboard/internal/repository/email"
)

type EmailSender interface {
	Send(ctx context.Context, msg model.Message) error
}

type EmailWorker struct {
	heartbeat  func(context.Context)
	repo       repository.EmailRepository
	sender     EmailSender
	pollEvery  time.Duration
	batchSize  int32
	maxRetries int32
	logger     *logger.Logger
}

func NewEmailWorker(
	repo repository.EmailRepository,
	sender EmailSender,
	pollEvery time.Duration,
	batchSize int32,
	maxRetries int32,
	logger *logger.Logger,
) (*EmailWorker, error) {
	if repo == nil {
		return nil, errors.New("email repository is required")
	}

	if sender == nil {
		return nil, errors.New("email sender is required")
	}

	if logger == nil {
		return nil, errors.New("logger is required")
	}

	if pollEvery <= 0 {
		pollEvery = 5 * time.Second
	}

	if batchSize <= 0 {
		batchSize = 20
	}

	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &EmailWorker{
		repo:       repo,
		sender:     sender,
		pollEvery:  pollEvery,
		batchSize:  batchSize,
		maxRetries: maxRetries,
		logger:     logger,
	}, nil
}

func (w *EmailWorker) Start(ctx context.Context) error {
	if w == nil {
		return errors.New("email worker is nil")
	}

	w.logger.Info("email worker started")

	ticker := time.NewTicker(w.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("email worker shutting down")
			return nil

		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *EmailWorker) process(ctx context.Context) {
	defer recoverWorkerPanic(w.logger, "email worker")

	select {
	case <-ctx.Done():
		return
	default:
	}

	items, err := w.repo.ClaimNextBatch(ctx, w.batchSize)
	if err != nil {
		w.logger.Error(
			"email worker failed to claim batch",
			"error", err,
		)
		return
	}

	if w.heartbeat != nil {
		w.heartbeat(ctx)
	}
	if len(items) == 0 {
		return
	}

	w.logger.Info(
		"email worker claimed batch",
		"count", len(items),
	)

	for _, item := range items {
		if w.heartbeat != nil {
			w.heartbeat(ctx)
		}
		itemCtx := platformMessageContext(ctx, item.Message.Metadata)
		logPlatformAttempt(itemCtx, "email_delivery_attempt", item.ID)
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.sender.Send(itemCtx, item.Message); err != nil {
			if item.Message.Metadata["application_id"] != "" {
				err = errors.New("platform email transport failed")
			}
			attempts := item.Attempts + 1

			if attempts >= item.MaxAttempts || attempts >= w.maxRetries {
				if markErr := w.repo.MarkFailed(ctx, item.ID, attempts, err.Error()); markErr != nil {
					w.logger.Error(
						"email worker failed to mark email as failed",
						"email_id", item.ID,
						"attempts", attempts,
						"error", markErr,
					)
					continue
				}

				w.logger.Error(
					"email permanently failed",
					"email_id", item.ID,
					"attempts", attempts,
					"error", err,
				)

				continue
			}

			if markErr := w.repo.MarkRetry(ctx, item.ID, attempts, err.Error()); markErr != nil {
				w.logger.Error(
					"email worker failed to mark email for retry",
					"email_id", item.ID,
					"attempts", attempts,
					"error", markErr,
				)
				continue
			}

			w.logger.Error(
				"email send failed and scheduled for retry",
				"email_id", item.ID,
				"attempts", attempts,
				"error", err,
			)

			continue
		}

		if err := w.repo.MarkSent(ctx, item.ID); err != nil {
			w.logger.Error(
				"email worker failed to mark email as sent",
				"email_id", item.ID,
				"error", err,
			)
			continue
		}

		w.logger.Info(
			"email sent successfully",
			"email_id", item.ID,
		)
	}
}

func (w *EmailWorker) SetHeartbeat(callback func(context.Context)) { w.heartbeat = callback }
