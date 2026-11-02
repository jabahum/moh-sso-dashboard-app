package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/observability"
	emailrepo "github.com/moh-sso-dashboard/internal/repository/email"
	deliveryrepo "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
	"github.com/sqlc-dev/pqtype"
)

type heartbeatDeliveryRepository struct {
	deliveryrepo.NotificationDeliveryRepository
	claimErr error
}

func (r heartbeatDeliveryRepository) ClaimPending(context.Context, string, int32) ([]db.NotificationDelivery, error) {
	return nil, r.claimErr
}
func TestNotificationHeartbeatRequiresSuccessfulQueuePoll(t *testing.T) {
	for _, failed := range []bool{false, true} {
		calls := 0
		repo := heartbeatDeliveryRepository{}
		if failed {
			repo.claimErr = errors.New("queue unavailable")
		}
		worker := &NotificationEmailDeliveryWorker{notificationDelivery: repo, logger: logger.NewLogger(), heartbeat: func(context.Context) { calls++ }}
		err := worker.process(context.Background())
		if failed && (err == nil || calls != 0) {
			t.Fatal("failed queue poll refreshed health")
		}
		if !failed && (err != nil || calls != 1) {
			t.Fatal("successful queue poll did not refresh health")
		}
	}
}
func TestPlatformTraceSurvivesNotificationToEmailConversion(t *testing.T) {
	raw, _ := json.Marshal(map[string]string{"subject": "Private title", "text_body": "Private body", "application_id": "pilot", "request_id": "req-1", "correlation_id": "corr-1"})
	recipient := pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"email":"recipient@example.org"}`), Valid: true}
	msg, err := notificationDeliveryToEmailMessage(db.NotificationDelivery{ID: uuid.New(), NotificationID: uuid.New(), Channel: "email", Recipient: recipient, Payload: pqtype.NullRawMessage{RawMessage: raw, Valid: true}, TemplateName: sql.NullString{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := platformMessageContext(context.Background(), msg.Metadata)
	if observability.ApplicationIDFromContext(ctx) != "pilot" || observability.RequestIDFromContext(ctx) != "req-1" || observability.CorrelationIDFromContext(ctx) != "corr-1" {
		t.Fatal("async trace was lost")
	}
	if msg.Subject != "Private title" || msg.TextBody != "Private body" {
		t.Fatal("trace propagation changed the message")
	}
}

type platformEmailRepository struct {
	emailrepo.EmailRepository
	item      emailrepo.OutboxMessage
	lastError string
	marked    bool
}

func (r *platformEmailRepository) ClaimNextBatch(context.Context, int32) ([]emailrepo.OutboxMessage, error) {
	return []emailrepo.OutboxMessage{r.item}, nil
}
func (r *platformEmailRepository) MarkFailed(_ context.Context, _ string, _ int32, message string) error {
	r.lastError = message
	return nil
}
func (r *platformEmailRepository) MarkSent(context.Context, string) error {
	r.marked = true
	return nil
}

type platformSender struct {
	err                               error
	application, request, correlation string
}

func (s *platformSender) Send(ctx context.Context, _ model.Message) error {
	s.application = observability.ApplicationIDFromContext(ctx)
	s.request = observability.RequestIDFromContext(ctx)
	s.correlation = observability.CorrelationIDFromContext(ctx)
	return s.err
}
func TestEmailWorkerRestoresTraceAndRedactsPlatformFailure(t *testing.T) {
	repo := &platformEmailRepository{item: emailrepo.OutboxMessage{ID: "message", MaxAttempts: 1, Message: model.Message{Metadata: map[string]string{"application_id": "pilot", "request_id": "req-1", "correlation_id": "corr-1"}}}}
	sender := &platformSender{err: errors.New("provider echoed private recipient/content")}
	worker := &EmailWorker{repo: repo, sender: sender, maxRetries: 1, logger: logger.NewLogger()}
	worker.process(context.Background())
	if sender.application != "pilot" || sender.request != "req-1" || sender.correlation != "corr-1" {
		t.Fatal("trace not restored before SMTP")
	}
	if repo.lastError != "platform email transport failed" {
		t.Fatalf("sensitive provider failure stored: %s", repo.lastError)
	}
	sender.err = nil
	worker.process(context.Background())
	if !repo.marked {
		t.Fatal("successful mail not marked sent")
	}
}
