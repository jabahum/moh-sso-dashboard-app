package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	logger "github.com/moh-sso-dashboard/internal/log"
	deliveryrepo "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
)

type attemptRecordingDeliveryRepository struct {
	deliveryrepo.NotificationDeliveryRepository
	retries  []db.MarkNotificationDeliveryRetryParams
	failures []db.MarkNotificationDeliveryFailedParams
}

func (r *attemptRecordingDeliveryRepository) MarkRetry(_ context.Context, arg db.MarkNotificationDeliveryRetryParams) error {
	r.retries = append(r.retries, arg)
	return nil
}

func (r *attemptRecordingDeliveryRepository) MarkFailed(_ context.Context, arg db.MarkNotificationDeliveryFailedParams) error {
	r.failures = append(r.failures, arg)
	return nil
}

// Claiming increments attempts, so a claimed item's Attempts is the number of the
// attempt that just failed. The worker must neither add one again (which failed a
// max_attempts=5 delivery after four tries) nor back off by the wrong step.
func TestNotificationDeliveryFailureUsesClaimedAttemptNumber(t *testing.T) {
	cases := []struct {
		attempts, maxAttempts int32
		wantFailed            bool
		wantDelay             string
	}{
		{attempts: 1, maxAttempts: 5, wantDelay: "30s"},
		{attempts: 2, maxAttempts: 5, wantDelay: "2m0s"},
		{attempts: 4, maxAttempts: 5, wantDelay: "5m0s"},
		{attempts: 5, maxAttempts: 5, wantFailed: true},
		// A manual retry of an exhausted delivery buys exactly one more attempt.
		{attempts: 6, maxAttempts: 5, wantFailed: true},
		{attempts: 1, maxAttempts: 1, wantFailed: true},
	}
	for _, tc := range cases {
		item := db.NotificationDelivery{ID: uuid.New(), NotificationID: uuid.New(), Channel: "email", Attempts: tc.attempts, MaxAttempts: tc.maxAttempts}
		if got := deliveryAttemptsExhausted(item); got != tc.wantFailed {
			t.Fatalf("deliveryAttemptsExhausted(%d/%d) = %t", tc.attempts, tc.maxAttempts, got)
		}

		emailRepo := &attemptRecordingDeliveryRepository{}
		email := &NotificationEmailDeliveryWorker{notificationDelivery: emailRepo, logger: logger.NewLogger()}
		smsRepo := &attemptRecordingDeliveryRepository{}
		sms := &NotificationSMSDeliveryWorker{notificationDelivery: smsRepo, logger: logger.NewLogger()}
		cause := errors.New("provider unavailable")
		if err := email.failOrRetry(context.Background(), item, cause); !errors.Is(err, cause) {
			t.Fatalf("email failOrRetry returned %v", err)
		}
		if err := sms.failOrRetry(context.Background(), item, cause); !errors.Is(err, cause) {
			t.Fatalf("sms failOrRetry returned %v", err)
		}
		for name, repo := range map[string]*attemptRecordingDeliveryRepository{"email": emailRepo, "sms": smsRepo} {
			if tc.wantFailed {
				if len(repo.failures) != 1 || len(repo.retries) != 0 || repo.failures[0].ID != item.ID {
					t.Fatalf("%s %d/%d: want failed, got retries=%d failures=%d", name, tc.attempts, tc.maxAttempts, len(repo.retries), len(repo.failures))
				}
				continue
			}
			if len(repo.retries) != 1 || len(repo.failures) != 0 || repo.retries[0].Column3 != tc.wantDelay {
				t.Fatalf("%s %d/%d: want retry after %s, got %+v failures=%d", name, tc.attempts, tc.maxAttempts, tc.wantDelay, repo.retries, len(repo.failures))
			}
		}
	}
}
