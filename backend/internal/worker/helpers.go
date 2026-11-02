package worker

import (
	"context"
	"github.com/moh-sso-dashboard/internal/observability"
	"github.com/rs/zerolog/log"
	"time"

	logger "github.com/moh-sso-dashboard/internal/log"
)

func recoverWorkerPanic(log *logger.Logger, workerName string) {
	if r := recover(); r != nil {
		if log != nil {
			log.Error(
				"worker panic recovered",
				"worker", workerName,
				"panic", r,
			)
		}
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) {
	if delay <= 0 {
		return
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		return
	}
}

func platformMessageContext(ctx context.Context, metadata map[string]string) context.Context {
	if metadata["application_id"] == "" {
		return ctx
	}
	ctx = observability.WithApplicationID(ctx, metadata["application_id"])
	ctx = observability.WithRequestID(ctx, metadata["request_id"])
	return observability.WithCorrelationID(ctx, metadata["correlation_id"])
}
func logPlatformAttempt(ctx context.Context, event, id string) {
	if observability.ApplicationIDFromContext(ctx) == "" {
		return
	}
	log.Info().Str("application_id", observability.ApplicationIDFromContext(ctx)).Str("request_id", observability.RequestIDFromContext(ctx)).Str("correlation_id", observability.CorrelationIDFromContext(ctx)).Str("resource_id", id).Msg(event)
}
