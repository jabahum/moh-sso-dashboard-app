package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/config"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	"github.com/moh-sso-dashboard/internal/model"
	notificationDeliveryRepository "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
	notificationsRepo "github.com/moh-sso-dashboard/internal/repository/notifications"
)

type fakeNotificationDeliveryRepo struct {
	delivery    db.NotificationDelivery
	created     []db.NotificationDelivery
	markRetry   bool
	markCancel  bool
	retryParams db.MarkNotificationDeliveryRetryParams
	cancelledID uuid.UUID
}

func (f *fakeNotificationDeliveryRepo) Create(_ context.Context, arg db.CreateNotificationDeliveryParams) (db.NotificationDelivery, error) {
	delivery := db.NotificationDelivery{
		ID:             uuid.New(),
		NotificationID: arg.NotificationID,
		Channel:        arg.Channel,
		Status:         "",
		Recipient:      arg.Recipient,
		Payload:        arg.Payload,
		Attempts:       0,
		MaxAttempts:    5,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if delivery.Status == "" {
		delivery.Status = string(model.NotificationDeliveryPending)
	}
	if arg.DeliveryStatus.Valid {
		delivery.Status = arg.DeliveryStatus.String
	}
	if maxAttempts, ok := arg.DeliveryMaxAttempts.(sql.NullInt32); ok && maxAttempts.Valid && maxAttempts.Int32 > 0 {
		delivery.MaxAttempts = maxAttempts.Int32
	}
	f.created = append(f.created, delivery)
	return delivery, nil
}

func (f *fakeNotificationDeliveryRepo) ListByNotificationID(context.Context, uuid.UUID) ([]db.NotificationDelivery, error) {
	return nil, nil
}
func (f *fakeNotificationDeliveryRepo) GetByID(context.Context, uuid.UUID) (db.NotificationDelivery, error) {
	return f.delivery, nil
}
func (f *fakeNotificationDeliveryRepo) ClaimPending(context.Context, string, int32) ([]db.NotificationDelivery, error) {
	return nil, nil
}
func (f *fakeNotificationDeliveryRepo) MarkSent(context.Context, uuid.UUID) error { return nil }
func (f *fakeNotificationDeliveryRepo) MarkSentWithProvider(context.Context, notificationDeliveryRepository.MarkSentWithProviderParams) error {
	return nil
}
func (f *fakeNotificationDeliveryRepo) MarkRetry(_ context.Context, arg db.MarkNotificationDeliveryRetryParams) error {
	f.markRetry = true
	f.retryParams = arg
	return nil
}
func (f *fakeNotificationDeliveryRepo) MarkFailed(context.Context, db.MarkNotificationDeliveryFailedParams) error {
	return nil
}
func (f *fakeNotificationDeliveryRepo) Cancel(_ context.Context, id uuid.UUID) error {
	f.markCancel = true
	f.cancelledID = id
	return nil
}
func (f *fakeNotificationDeliveryRepo) CountPendingByChannel(context.Context, string) (int64, error) {
	return 0, nil
}
func (f *fakeNotificationDeliveryRepo) List(context.Context, db.ListNotificationDeliveriesParams) ([]db.NotificationDelivery, error) {
	return nil, nil
}
func (f *fakeNotificationDeliveryRepo) Count(context.Context, db.CountNotificationDeliveriesParams) (int64, error) {
	return 0, nil
}
func (f *fakeNotificationDeliveryRepo) ListMetrics(context.Context) ([]db.ListNotificationDeliveryMetricsRow, error) {
	return nil, nil
}

type fakeNotificationsRepo struct {
	saved []model.Notification
}

func (f *fakeNotificationsRepo) Notify(_ context.Context, notification model.Notification) (*model.Notification, error) {
	if notification.ID == uuid.Nil {
		notification.ID = uuid.New()
	}
	notification.CreatedAt = time.Now()
	f.saved = append(f.saved, notification)
	return &notification, nil
}
func (f *fakeNotificationsRepo) ListNotifications(context.Context, string, *bool, int32, int32) ([]model.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationsRepo) GetNotificationByID(context.Context, uuid.UUID) (*model.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationsRepo) MarkNotificationAsRead(context.Context, uuid.UUID) error { return nil }
func (f *fakeNotificationsRepo) DeleteNotification(context.Context, uuid.UUID) error     { return nil }
func (f *fakeNotificationsRepo) DeleteOldNotifications(context.Context) error            { return nil }
func (f *fakeNotificationsRepo) CountNotifications(context.Context, string) (int64, error) {
	return 0, nil
}
func (f *fakeNotificationsRepo) CountUnreadNotifications(context.Context, string) (int64, error) {
	return 0, nil
}

var _ notificationDeliveryRepository.NotificationDeliveryRepository = (*fakeNotificationDeliveryRepo)(nil)
var _ notificationsRepo.NotificationsRepository = (*fakeNotificationsRepo)(nil)

func TestRetryNotificationDeliveryRejectsSent(t *testing.T) {
	deliveryID := uuid.New()
	repo := &fakeNotificationDeliveryRepo{
		delivery: db.NotificationDelivery{
			ID:     deliveryID,
			Status: string(model.NotificationDeliverySent),
		},
	}
	svc := NewNotificationsService(nil, nil, repo, nil, nil)

	err := svc.RetryNotificationDelivery(context.Background(), deliveryID)
	if err == nil {
		t.Fatal("expected retrying a sent delivery to fail")
	}
	if repo.markRetry {
		t.Fatal("sent delivery should not be marked for retry")
	}
}

func TestCancelNotificationDeliveryRejectsSent(t *testing.T) {
	deliveryID := uuid.New()
	repo := &fakeNotificationDeliveryRepo{
		delivery: db.NotificationDelivery{
			ID:     deliveryID,
			Status: string(model.NotificationDeliverySent),
		},
	}
	svc := NewNotificationsService(nil, nil, repo, nil, nil)

	err := svc.CancelNotificationDelivery(context.Background(), deliveryID)
	if err == nil {
		t.Fatal("expected cancelling a sent delivery to fail")
	}
	if repo.markCancel {
		t.Fatal("sent delivery should not be cancelled")
	}
}

func TestQueueTestSMSCreatesSMSDelivery(t *testing.T) {
	notifications := &fakeNotificationsRepo{}
	deliveries := &fakeNotificationDeliveryRepo{}
	svc := NewNotificationsService(
		&config.Config{SMS: config.SMSConfig{Enabled: true}},
		notifications,
		deliveries,
		nil,
		nil,
	)

	notificationID, err := svc.QueueTestSMS(context.Background(), "+256700000000", "hello")
	if err != nil {
		t.Fatalf("queue test sms: %v", err)
	}
	if notificationID == uuid.Nil {
		t.Fatal("expected notification id")
	}
	if len(notifications.saved) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifications.saved))
	}
	if len(deliveries.created) != 2 {
		t.Fatalf("expected in-app and sms deliveries, got %d", len(deliveries.created))
	}

	var smsDelivery *db.NotificationDelivery
	for i := range deliveries.created {
		if deliveries.created[i].Channel == string(model.NotificationChannelSMS) {
			smsDelivery = &deliveries.created[i]
		}
	}
	if smsDelivery == nil {
		t.Fatal("expected sms delivery")
	}
	if smsDelivery.MaxAttempts != 3 {
		t.Fatalf("expected sms max attempts 3, got %#v", smsDelivery.MaxAttempts)
	}
	for _, delivery := range deliveries.created {
		want := string(model.NotificationDeliveryPending)
		if delivery.Channel == string(model.NotificationChannelInApp) {
			// No worker drains in-app deliveries; the stored notification is the delivery.
			want = string(model.NotificationDeliverySent)
		}
		if delivery.Status != want {
			t.Fatalf("%s delivery created %s, want %s", delivery.Channel, delivery.Status, want)
		}
	}
}

func TestNotifyCreatesDefaultInAppDeliveryAsSent(t *testing.T) {
	deliveries := &fakeNotificationDeliveryRepo{}
	svc := NewNotificationsService(nil, &fakeNotificationsRepo{}, deliveries, nil, nil)

	if _, err := svc.Notify(context.Background(), model.Notification{Type: "TEST", Title: "t", Message: "m", TargetRole: "admin"}); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(deliveries.created) != 1 || deliveries.created[0].Channel != string(model.NotificationChannelInApp) ||
		deliveries.created[0].Status != string(model.NotificationDeliverySent) {
		t.Fatalf("default delivery = %+v, want one SENT in_app delivery", deliveries.created)
	}
}

func TestQueueTestSMSRequiresEnabledSMS(t *testing.T) {
	svc := NewNotificationsService(&config.Config{}, &fakeNotificationsRepo{}, &fakeNotificationDeliveryRepo{}, nil, nil)

	if _, err := svc.QueueTestSMS(context.Background(), "+256700000000", "hello"); err == nil {
		t.Fatal("expected disabled sms to fail")
	}
}

func TestRetryNotificationDeliveryMarksRetry(t *testing.T) {
	deliveryID := uuid.New()
	repo := &fakeNotificationDeliveryRepo{
		delivery: db.NotificationDelivery{
			ID:     deliveryID,
			Status: string(model.NotificationDeliveryFailed),
		},
	}
	svc := NewNotificationsService(nil, nil, repo, nil, nil)

	if err := svc.RetryNotificationDelivery(context.Background(), deliveryID); err != nil {
		t.Fatalf("retry delivery: %v", err)
	}
	if !repo.markRetry {
		t.Fatal("expected delivery to be marked retry")
	}
	if !repo.retryParams.LastError.Valid || repo.retryParams.LastError.String == "" {
		t.Fatal("expected retry reason")
	}
}

func TestCancelNotificationDeliveryMarksCancelled(t *testing.T) {
	deliveryID := uuid.New()
	repo := &fakeNotificationDeliveryRepo{
		delivery: db.NotificationDelivery{
			ID:     deliveryID,
			Status: string(model.NotificationDeliveryPending),
		},
	}
	svc := NewNotificationsService(nil, nil, repo, nil, nil)

	if err := svc.CancelNotificationDelivery(context.Background(), deliveryID); err != nil {
		t.Fatalf("cancel delivery: %v", err)
	}
	if !repo.markCancel || repo.cancelledID != deliveryID {
		t.Fatal("expected delivery to be cancelled")
	}
}
