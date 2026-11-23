package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/model"
	notificationDeliveryRepo "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
	"github.com/moh-sso-dashboard/internal/service"
	"github.com/sqlc-dev/pqtype"
)

type NotificationSMSDeliveryWorker struct {
	notificationDelivery notificationDeliveryRepo.NotificationDeliveryRepository
	smsSvc               service.SMSService
	pollDelay            time.Duration
	batchSize            int32
	maxRetries           int32
	logger               *logger.Logger
	auditService         *service.AuditService
}

func NewNotificationSMSDeliveryWorker(
	notificationDelivery notificationDeliveryRepo.NotificationDeliveryRepository,
	smsSvc service.SMSService,
	pollDelay time.Duration,
	batchSize int32,
	maxRetries int32,
	logger *logger.Logger,
	auditService ...*service.AuditService,
) (*NotificationSMSDeliveryWorker, error) {
	if notificationDelivery == nil {
		return nil, errors.New("notification delivery repository is required")
	}
	if smsSvc == nil {
		return nil, errors.New("sms service is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if pollDelay <= 0 {
		pollDelay = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var audit *service.AuditService
	if len(auditService) > 0 {
		audit = auditService[0]
	}

	return &NotificationSMSDeliveryWorker{
		notificationDelivery: notificationDelivery,
		smsSvc:               smsSvc,
		pollDelay:            pollDelay,
		batchSize:            batchSize,
		maxRetries:           maxRetries,
		logger:               logger,
		auditService:         audit,
	}, nil
}

func (w *NotificationSMSDeliveryWorker) Start(ctx context.Context) error {
	if w == nil {
		return errors.New("notification sms delivery worker is nil")
	}

	w.logger.Info(
		"notification sms delivery worker started",
		"poll_delay", w.pollDelay.String(),
		"batch_size", w.batchSize,
		"max_retries", w.maxRetries,
	)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("notification sms delivery worker stopped")
			return ctx.Err()
		default:
		}

		if err := w.process(ctx); err != nil {
			w.logger.Error("notification sms delivery worker error", "error", err)
		}

		sleepWithContext(ctx, w.pollDelay)
	}
}

func (w *NotificationSMSDeliveryWorker) process(ctx context.Context) error {
	items, err := w.notificationDelivery.ClaimPending(
		ctx,
		string(model.NotificationChannelSMS),
		w.batchSize,
	)
	if err != nil {
		return fmt.Errorf("claim sms notification deliveries: %w", err)
	}

	for _, item := range items {
		if err := w.processOne(ctx, item); err != nil {
			w.logger.Error(
				"failed to process notification sms delivery",
				"error", err,
				"delivery_id", item.ID,
				"notification_id", item.NotificationID,
			)
		}
	}

	return nil
}

func (w *NotificationSMSDeliveryWorker) processOne(ctx context.Context, item db.NotificationDelivery) error {
	msg, err := notificationDeliveryToSMSMessage(item)
	if err != nil {
		return w.failOrRetry(ctx, item, err)
	}

	result, err := w.smsSvc.Send(ctx, msg)
	if err != nil {
		return w.failOrRetry(ctx, item, err)
	}

	if err := w.notificationDelivery.MarkSentWithProvider(
		ctx,
		notificationDeliveryRepo.MarkSentWithProviderParams{
			ID:                item.ID,
			Provider:          sqlNullString(result.Provider),
			ProviderMessageID: sqlNullString(result.MessageID),
			ProviderStatus:    sqlNullString(result.Status),
			ProviderResponse:  smsProviderResponse(result),
		},
	); err != nil {
		return fmt.Errorf("mark notification sms delivery sent: %w", err)
	}

	w.logger.Info(
		"notification sms delivery sent",
		"delivery_id", item.ID,
		"notification_id", item.NotificationID,
		"provider", result.Provider,
		"message_id", result.MessageID,
		"status", result.Status,
	)
	w.auditSMSDelivery(ctx, "notification_delivery.sms_sent", item, map[string]any{
		"status":   "SENT",
		"provider": result.Provider,
	})

	return nil
}

func (w *NotificationSMSDeliveryWorker) failOrRetry(
	ctx context.Context,
	item db.NotificationDelivery,
	cause error,
) error {
	if deliveryAttemptsExhausted(item) {
		if err := w.notificationDelivery.MarkFailed(
			ctx,
			db.MarkNotificationDeliveryFailedParams{
				ID: item.ID,
				LastError: sql.NullString{
					String: cause.Error(),
					Valid:  true,
				},
			},
		); err != nil {
			return fmt.Errorf("mark notification sms delivery failed: %w", err)
		}
		w.auditSMSDelivery(ctx, "notification_delivery.sms_failed", item, map[string]any{
			"status": "FAILED",
		})

		return cause
	}

	delay := retryDelay(item.Attempts)
	if err := w.notificationDelivery.MarkRetry(
		ctx,
		db.MarkNotificationDeliveryRetryParams{
			ID: item.ID,
			LastError: sql.NullString{
				String: cause.Error(),
				Valid:  true,
			},
			Column3: delay.String(),
		},
	); err != nil {
		return fmt.Errorf("mark notification sms delivery retry: %w", err)
	}
	w.auditSMSDelivery(ctx, "notification_delivery.sms_retry", item, map[string]any{
		"status":        "RETRY",
		"next_attempt":  item.Attempts,
		"retry_delay_s": int64(delay.Seconds()),
	})

	return cause
}

func (w *NotificationSMSDeliveryWorker) auditSMSDelivery(
	ctx context.Context,
	action string,
	item db.NotificationDelivery,
	metadata map[string]any,
) {
	if w == nil || w.auditService == nil {
		return
	}

	event := map[string]any{
		"delivery_id":     item.ID.String(),
		"notification_id": item.NotificationID.String(),
		"channel":         item.Channel,
		"previous_status": item.Status,
	}
	for key, value := range metadata {
		event[key] = value
	}

	_ = w.auditService.Log(ctx, uuid.NullUUID{}, action, event)
}

type smsRecipient struct {
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	PhoneNumber string `json:"phone_number"`
	To          string `json:"to"`
}

type smsPayload struct {
	Body    string `json:"body"`
	Message string `json:"message"`
}

func notificationDeliveryToSMSMessage(item db.NotificationDelivery) (service.SMSMessage, error) {
	var recipient smsRecipient
	if err := unmarshalNullRawMessage(item.Recipient, &recipient); err != nil {
		return service.SMSMessage{}, fmt.Errorf("decode recipient: %w", err)
	}

	to := firstNonEmpty(recipient.Phone, recipient.PhoneNumber, recipient.To)
	if strings.TrimSpace(to) == "" {
		return service.SMSMessage{}, errors.New("sms recipient phone is required")
	}

	var payload smsPayload
	if err := unmarshalNullRawMessage(item.Payload, &payload); err != nil {
		return service.SMSMessage{}, fmt.Errorf("decode payload: %w", err)
	}

	body := firstNonEmpty(payload.Body, payload.Message)
	if strings.TrimSpace(body) == "" {
		return service.SMSMessage{}, errors.New("sms body is required")
	}

	return service.SMSMessage{
		To:   to,
		Body: body,
		Metadata: map[string]string{
			"source":           "notification-delivery-worker",
			"notification_id":  item.NotificationID.String(),
			"delivery_id":      item.ID.String(),
			"delivery_channel": item.Channel,
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func smsProviderResponse(result service.SMSResult) pqtype.NullRawMessage {
	value := map[string]any{
		"provider":   result.Provider,
		"message_id": result.MessageID,
		"status":     result.Status,
	}

	b, err := json.Marshal(value)
	if err != nil {
		return pqtype.NullRawMessage{Valid: false}
	}

	return pqtype.NullRawMessage{
		RawMessage: b,
		Valid:      true,
	}
}

func sqlNullString(value string) sql.NullString {
	value = strings.TrimSpace(value)

	return sql.NullString{
		String: value,
		Valid:  value != "",
	}
}
