package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	"github.com/moh-sso-dashboard/internal/model"
	notificationDeliveryRepository "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
	notificationPreferencesRepository "github.com/moh-sso-dashboard/internal/repository/notification_preferences"
	repository "github.com/moh-sso-dashboard/internal/repository/notifications"
	"github.com/moh-sso-dashboard/internal/utils"
)

const (
	defaultNotificationTargetRole = "admin"

	fallbackPlatformName      = "MOH Integrated Health Portal"
	fallbackAdminDashboardURL = "http://localhost:3000/admin/home"
	fallbackSystemAdminName   = "System Administrator"
	fallbackSystemAdminEmail  = "admin@example.com"
)

type NotificationsService interface {
	Notify(ctx context.Context, notification model.Notification) (*model.Notification, error)
	NotifyLoginFailed(ctx context.Context, clientID, ip, userAgent string, err error)
	NotifySuspiciousLogin(ctx context.Context, ip, userAgent string)
	NotifyTokenRefreshFailed(ctx context.Context, ip, userAgent string)
	NotifyAccountLocked(ctx context.Context, userID uuid.UUID)
	NotifySystemStartup(ctx context.Context, version string)
	NotifySystemShutdown(ctx context.Context, reason string)
	NotifyConfigChanged(ctx context.Context, changedBy string, keys []string)
	NotifyBackupCompleted(ctx context.Context, backupID string, durationSeconds int)
	NotifyBackupFailed(ctx context.Context, backupID string, err error)
	ListNotifications(
		ctx context.Context,
		role string,
		unread *bool,
		limit int32,
		offset int32,
	) ([]model.Notification, error)
	GetNotificationByID(ctx context.Context, notificationID string) (*model.Notification, error)
	MarkNotificationAsRead(ctx context.Context, notificationID string) error
	DeleteNotification(ctx context.Context, notificationID string) error
	DeleteOldNotifications(ctx context.Context) error
	CountNotifications(ctx context.Context, targetRole string) (int64, error)
	CountUnreadNotificationsCount(ctx context.Context, targetRole string) (int64, error)
	ListNotificationDeliveries(ctx context.Context, notificationID uuid.UUID) ([]db.NotificationDelivery, error)
	ListAllNotificationDeliveries(ctx context.Context, filter NotificationDeliveryListFilter) ([]db.NotificationDelivery, int64, error)
	ListNotificationDeliveryMetrics(ctx context.Context) ([]db.ListNotificationDeliveryMetricsRow, error)
	GetNotificationDelivery(ctx context.Context, deliveryID uuid.UUID) (db.NotificationDelivery, error)
	RetryNotificationDelivery(ctx context.Context, deliveryID uuid.UUID) error
	CancelNotificationDelivery(ctx context.Context, deliveryID uuid.UUID) error
	QueueTestSMS(ctx context.Context, to string, message string) (uuid.UUID, error)
	GetNotificationPreferences(ctx context.Context, userID string) (NotificationPreferences, error)
	UpdateNotificationPreferences(ctx context.Context, userID string, input UpdateNotificationPreferencesInput) (NotificationPreferences, error)
}

type NotificationDeliveryListFilter struct {
	Channel string
	Status  string
	Search  string
	Limit   int32
	Offset  int32
}

type NotificationPreferences struct {
	UserID          string
	EmailEnabled    bool
	SMSEnabled      bool
	PhoneNumber     string
	PhoneVerified   bool
	QuietHoursStart string
	QuietHoursEnd   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type UpdateNotificationPreferencesInput struct {
	EmailEnabled    *bool
	SMSEnabled      *bool
	PhoneNumber     *string
	QuietHoursStart *string
	QuietHoursEnd   *string
}

type notificationsService struct {
	cfg                         *config.Config
	notificationsRepo           repository.NotificationsRepository
	notificationDeliveryRepo    notificationDeliveryRepository.NotificationDeliveryRepository
	notificationPreferencesRepo notificationPreferencesRepository.NotificationPreferencesRepository
	publisher                   *cache.NotificationPublisher
}

func NewNotificationsService(
	cfg *config.Config,
	notificationsRepo repository.NotificationsRepository,
	notificationDeliveryRepo notificationDeliveryRepository.NotificationDeliveryRepository,
	notificationPreferencesRepo notificationPreferencesRepository.NotificationPreferencesRepository,
	publisher *cache.NotificationPublisher,
) NotificationsService {
	return &notificationsService{
		cfg:                         cfg,
		notificationsRepo:           notificationsRepo,
		notificationDeliveryRepo:    notificationDeliveryRepo,
		notificationPreferencesRepo: notificationPreferencesRepo,
		publisher:                   publisher,
	}
}

func (s *notificationsService) Notify(
	ctx context.Context,
	notification model.Notification,
) (*model.Notification, error) {
	if s == nil {
		return nil, errors.New("notifications service is nil")
	}

	if s.notificationsRepo == nil {
		return nil, errors.New("notifications repository is nil")
	}

	notification.CreatedAt = time.Now()
	notification.Read = false

	n, err := s.notificationsRepo.Notify(ctx, notification)
	if err != nil {
		log.Printf("error creating notification: %v", err)
		return nil, err
	}

	if n == nil {
		return nil, errors.New("created notification is nil")
	}

	if s.publisher != nil {
		go func(saved model.Notification) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("recovered from panic in notification publish goroutine: %v", r)
				}
			}()

			channel := cache.ResolveNotificationChannel(saved)

			if err := s.publisher.Publish(ctx, channel, saved); err != nil {
				log.Printf(
					"notification publish failed (channel=%s, id=%s): %v",
					channel,
					saved.ID,
					err,
				)
			}
		}(*n)
	}

	deliveries := normalizeNotificationDeliveries(notification, *n)

	for _, delivery := range deliveries {
		if err := s.createNotificationDelivery(ctx, n.ID, delivery); err != nil {
			log.Printf(
				"failed to create notification delivery notification_id=%s channel=%s error=%v",
				n.ID,
				delivery.Channel,
				err,
			)

			continue
		}
	}

	return n, nil
}

func (s *notificationsService) ListNotificationDeliveries(
	ctx context.Context,
	notificationID uuid.UUID,
) ([]db.NotificationDelivery, error) {
	if s == nil {
		return nil, errors.New("notifications service is nil")
	}

	if s.notificationDeliveryRepo == nil {
		return nil, errors.New("notification delivery repository is nil")
	}

	return s.notificationDeliveryRepo.ListByNotificationID(ctx, notificationID)
}

func (s *notificationsService) ListAllNotificationDeliveries(
	ctx context.Context,
	filter NotificationDeliveryListFilter,
) ([]db.NotificationDelivery, int64, error) {
	if s == nil {
		return nil, 0, errors.New("notifications service is nil")
	}
	if s.notificationDeliveryRepo == nil {
		return nil, 0, errors.New("notification delivery repository is nil")
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	channel := sql.NullString{String: strings.TrimSpace(filter.Channel), Valid: strings.TrimSpace(filter.Channel) != ""}
	status := sql.NullString{String: strings.ToUpper(strings.TrimSpace(filter.Status)), Valid: strings.TrimSpace(filter.Status) != ""}
	search := sql.NullString{String: strings.TrimSpace(filter.Search), Valid: strings.TrimSpace(filter.Search) != ""}
	params := db.ListNotificationDeliveriesParams{
		Limit:         filter.Limit,
		Offset:        filter.Offset,
		FilterChannel: channel,
		FilterStatus:  status,
		FilterSearch:  search,
	}
	countParams := db.CountNotificationDeliveriesParams{
		FilterChannel: channel,
		FilterStatus:  status,
		FilterSearch:  search,
	}

	items, err := s.notificationDeliveryRepo.List(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.notificationDeliveryRepo.Count(ctx, countParams)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *notificationsService) ListNotificationDeliveryMetrics(
	ctx context.Context,
) ([]db.ListNotificationDeliveryMetricsRow, error) {
	if s == nil {
		return nil, errors.New("notifications service is nil")
	}
	if s.notificationDeliveryRepo == nil {
		return nil, errors.New("notification delivery repository is nil")
	}

	return s.notificationDeliveryRepo.ListMetrics(ctx)
}

func (s *notificationsService) GetNotificationDelivery(
	ctx context.Context,
	deliveryID uuid.UUID,
) (db.NotificationDelivery, error) {
	if s == nil {
		return db.NotificationDelivery{}, errors.New("notifications service is nil")
	}
	if s.notificationDeliveryRepo == nil {
		return db.NotificationDelivery{}, errors.New("notification delivery repository is nil")
	}

	return s.notificationDeliveryRepo.GetByID(ctx, deliveryID)
}

func (s *notificationsService) RetryNotificationDelivery(
	ctx context.Context,
	deliveryID uuid.UUID,
) error {
	if s == nil {
		return errors.New("notifications service is nil")
	}

	if s.notificationDeliveryRepo == nil {
		return errors.New("notification delivery repository is nil")
	}

	delivery, err := s.notificationDeliveryRepo.GetByID(ctx, deliveryID)
	if err != nil {
		return err
	}

	if strings.EqualFold(delivery.Status, string(model.NotificationDeliverySent)) {
		return errors.New("sent notification deliveries cannot be retried")
	}

	return s.notificationDeliveryRepo.MarkRetry(
		ctx,
		db.MarkNotificationDeliveryRetryParams{
			ID: deliveryID,
			LastError: sql.NullString{
				String: "Manually queued for retry",
				Valid:  true,
			},
			Column3: "0 seconds",
		},
	)
}

func (s *notificationsService) CancelNotificationDelivery(
	ctx context.Context,
	deliveryID uuid.UUID,
) error {
	if s == nil {
		return errors.New("notifications service is nil")
	}
	if s.notificationDeliveryRepo == nil {
		return errors.New("notification delivery repository is nil")
	}

	delivery, err := s.notificationDeliveryRepo.GetByID(ctx, deliveryID)
	if err != nil {
		return err
	}

	if strings.EqualFold(delivery.Status, string(model.NotificationDeliverySent)) {
		return errors.New("sent notification deliveries cannot be cancelled")
	}

	return s.notificationDeliveryRepo.Cancel(ctx, deliveryID)
}

func (s *notificationsService) QueueTestSMS(ctx context.Context, to string, message string) (uuid.UUID, error) {
	to = strings.TrimSpace(to)
	message = strings.TrimSpace(message)
	if to == "" {
		return uuid.Nil, errors.New("sms recipient is required")
	}
	if message == "" {
		return uuid.Nil, errors.New("sms message is required")
	}
	if s == nil || s.cfg == nil || !s.cfg.SMS.Enabled {
		return uuid.Nil, errors.New("sms delivery is disabled")
	}

	notification, err := s.Notify(ctx, model.Notification{
		Type:       "SMS_TEST",
		Title:      "Test SMS",
		Message:    "A test SMS was queued from the notification admin console.",
		Severity:   "INFO",
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]any{
			"channel": "sms",
			"to":      to,
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, "SMS_TEST", "Test SMS", message, "INFO"),
			{
				Channel: model.NotificationChannelSMS,
				Recipient: map[string]any{
					"phone": to,
				},
				Payload: map[string]any{
					"body": message,
				},
				MaxAttempts: 3,
			},
		},
	})
	if err != nil {
		return uuid.Nil, err
	}

	return notification.ID, nil
}

func (s *notificationsService) GetNotificationPreferences(
	ctx context.Context,
	userID string,
) (NotificationPreferences, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return NotificationPreferences{}, errors.New("user id is required")
	}
	if s == nil {
		return NotificationPreferences{}, errors.New("notifications service is nil")
	}
	if s.notificationPreferencesRepo == nil {
		return defaultNotificationPreferences(userID), nil
	}

	preference, err := s.notificationPreferencesRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return defaultNotificationPreferences(userID), nil
		}

		return NotificationPreferences{}, err
	}

	return toNotificationPreferences(preference), nil
}

func (s *notificationsService) UpdateNotificationPreferences(
	ctx context.Context,
	userID string,
	input UpdateNotificationPreferencesInput,
) (NotificationPreferences, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return NotificationPreferences{}, errors.New("user id is required")
	}
	if s == nil {
		return NotificationPreferences{}, errors.New("notifications service is nil")
	}
	if s.notificationPreferencesRepo == nil {
		return NotificationPreferences{}, errors.New("notification preferences repository is nil")
	}

	current, err := s.GetNotificationPreferences(ctx, userID)
	if err != nil {
		return NotificationPreferences{}, err
	}

	emailEnabled := current.EmailEnabled
	if input.EmailEnabled != nil {
		emailEnabled = *input.EmailEnabled
	}

	smsEnabled := current.SMSEnabled
	if input.SMSEnabled != nil {
		smsEnabled = *input.SMSEnabled
	}

	phoneNumber := strings.TrimSpace(current.PhoneNumber)
	if input.PhoneNumber != nil {
		phoneNumber = strings.TrimSpace(*input.PhoneNumber)
	}

	if phoneNumber != "" {
		defaultCountryCode := "+256"
		if s.cfg != nil {
			defaultCountryCode = s.cfg.SMS.DefaultCountryCode
		}

		normalized, err := NormalizePhoneNumber(phoneNumber, defaultCountryCode)
		if err != nil {
			return NotificationPreferences{}, err
		}
		phoneNumber = normalized
	}

	if smsEnabled && phoneNumber == "" {
		return NotificationPreferences{}, errors.New("phone number is required when SMS notifications are enabled")
	}

	quietHoursStart := strings.TrimSpace(current.QuietHoursStart)
	if input.QuietHoursStart != nil {
		quietHoursStart = strings.TrimSpace(*input.QuietHoursStart)
	}
	if err := validateQuietHour("quiet_hours_start", quietHoursStart); err != nil {
		return NotificationPreferences{}, err
	}

	quietHoursEnd := strings.TrimSpace(current.QuietHoursEnd)
	if input.QuietHoursEnd != nil {
		quietHoursEnd = strings.TrimSpace(*input.QuietHoursEnd)
	}
	if err := validateQuietHour("quiet_hours_end", quietHoursEnd); err != nil {
		return NotificationPreferences{}, err
	}

	preference, err := s.notificationPreferencesRepo.Upsert(
		ctx,
		notificationPreferencesRepository.UpsertNotificationPreferencesParams{
			UserID:          userID,
			EmailEnabled:    emailEnabled,
			SMSEnabled:      smsEnabled,
			PhoneNumber:     sqlNullString(phoneNumber),
			PhoneVerified:   current.PhoneVerified,
			QuietHoursStart: sqlNullString(quietHoursStart),
			QuietHoursEnd:   sqlNullString(quietHoursEnd),
		},
	)
	if err != nil {
		return NotificationPreferences{}, err
	}

	return toNotificationPreferences(preference), nil
}

func (s *notificationsService) NotifyLoginFailed(
	ctx context.Context,
	clientID, ip, userAgent string,
	err error,
) {
	nt := model.LoginFailed

	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	title := nt.Title()
	message := "Authentication failed during login"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"client_id":  clientID,
			"ip":         ip,
			"user_agent": userAgent,
			"error":      errorMessage,
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildAdminAlertEmailDelivery(
				title,
				message,
				fmt.Sprintf(
					"Client ID: %s\nIP: %s\nUser Agent: %s\nError: %s",
					clientID,
					ip,
					userAgent,
					errorMessage,
				),
			),
		},
	})
}

func (s *notificationsService) NotifySuspiciousLogin(
	ctx context.Context,
	ip, userAgent string,
) {
	nt := model.SuspiciousLogin

	title := nt.Title()
	message := "Suspicious login attempt detected"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"ip":         ip,
			"user_agent": userAgent,
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildAdminAlertEmailDelivery(
				title,
				message,
				fmt.Sprintf("IP: %s\nUser Agent: %s", ip, userAgent),
			),
		},
	})
}

func (s *notificationsService) NotifyTokenRefreshFailed(
	ctx context.Context,
	ip, userAgent string,
) {
	nt := model.TokenRefreshFailed

	title := nt.Title()
	message := "Token refresh failed"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"ip":         ip,
			"user_agent": userAgent,
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildAdminAlertEmailDelivery(
				title,
				message,
				fmt.Sprintf("IP: %s\nUser Agent: %s", ip, userAgent),
			),
		},
	})
}

func (s *notificationsService) NotifyAccountLocked(
	ctx context.Context,
	userID uuid.UUID,
) {
	nt := model.AccountLocked

	title := nt.Title()
	message := "User account locked due to repeated login failures"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"user_id": userID.String(),
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildAdminAlertEmailDelivery(
				title,
				message,
				fmt.Sprintf("User ID: %s", userID.String()),
			),
		},
	})
}

func (s *notificationsService) NotifySystemStartup(
	ctx context.Context,
	version string,
) {
	if s == nil || s.notificationsRepo == nil {
		return
	}

	nt := model.SystemStartup

	title := nt.Title()
	message := "SSO service started successfully"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"version": version,
			"time":    time.Now().UTC(),
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildNotificationEmailDelivery(
				title,
				title,
				message,
				fmt.Sprintf("Version: %s\nTime: %s", version, time.Now().UTC().Format(time.RFC3339)),
			),
		},
	})
}

func (s *notificationsService) NotifySystemShutdown(
	ctx context.Context,
	reason string,
) {
	nt := model.SystemShutdown

	title := nt.Title()
	message := "SSO service shutting down"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"reason": reason,
			"time":   time.Now().UTC(),
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildAdminAlertEmailDelivery(
				title,
				message,
				fmt.Sprintf("Reason: %s\nTime: %s", reason, time.Now().UTC().Format(time.RFC3339)),
			),
		},
	})
}

func (s *notificationsService) NotifyConfigChanged(
	ctx context.Context,
	changedBy string,
	keys []string,
) {
	nt := model.ConfigChanged

	title := nt.Title()
	message := "System configuration updated"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"changed_by": changedBy,
			"keys":       keys,
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildNotificationEmailDelivery(
				title,
				title,
				message,
				fmt.Sprintf("Changed by: %s\nKeys: %s", changedBy, strings.Join(keys, ", ")),
			),
		},
	})
}

func (s *notificationsService) NotifyBackupCompleted(
	ctx context.Context,
	backupID string,
	durationSeconds int,
) {
	nt := model.BackupCompleted

	title := nt.Title()
	message := "Database backup completed successfully"

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"backup_id": backupID,
			"duration":  durationSeconds,
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildNotificationEmailDelivery(
				title,
				title,
				message,
				fmt.Sprintf("Backup ID: %s\nDuration: %d seconds", backupID, durationSeconds),
			),
		},
	})
}

func (s *notificationsService) NotifyBackupFailed(
	ctx context.Context,
	backupID string,
	err error,
) {
	nt := model.BackupFailed

	title := nt.Title()
	message := "Database backup failed"

	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	_, _ = s.Notify(ctx, model.Notification{
		Type:       string(nt),
		Title:      title,
		Severity:   nt.Severity(),
		Message:    message,
		TargetRole: defaultNotificationTargetRole,
		Metadata: utils.MustJSON(map[string]interface{}{
			"backup_id": backupID,
			"error":     errorMessage,
		}),
		Deliveries: []model.NotificationDeliveryRequest{
			buildInAppDelivery(defaultNotificationTargetRole, string(nt), title, message, nt.Severity()),
			s.buildAdminAlertEmailDelivery(
				title,
				message,
				fmt.Sprintf("Backup ID: %s\nError: %s", backupID, errorMessage),
			),
		},
	})
}

func (s *notificationsService) ListNotifications(
	ctx context.Context,
	role string,
	unread *bool,
	limit int32,
	offset int32,
) ([]model.Notification, error) {
	if s == nil || s.notificationsRepo == nil {
		return nil, errors.New("notifications service or repository is nil")
	}

	if limit <= 0 {
		limit = 20
	}

	if offset < 0 {
		offset = 0
	}

	return s.notificationsRepo.ListNotifications(ctx, role, unread, offset, limit)
}

func (s *notificationsService) GetNotificationByID(
	ctx context.Context,
	notificationID string,
) (*model.Notification, error) {
	if s == nil || s.notificationsRepo == nil {
		return nil, errors.New("notifications service or repository is nil")
	}

	id, err := uuid.Parse(notificationID)
	if err != nil {
		return nil, fmt.Errorf("invalid notification id: %w", err)
	}

	return s.notificationsRepo.GetNotificationByID(ctx, id)
}

func (s *notificationsService) MarkNotificationAsRead(
	ctx context.Context,
	notificationID string,
) error {
	if s == nil || s.notificationsRepo == nil {
		return errors.New("notifications service or repository is nil")
	}

	id, err := uuid.Parse(notificationID)
	if err != nil {
		return err
	}

	return s.notificationsRepo.MarkNotificationAsRead(ctx, id)
}

func (s *notificationsService) DeleteNotification(
	ctx context.Context,
	notificationID string,
) error {
	if s == nil || s.notificationsRepo == nil {
		return errors.New("notifications service or repository is nil")
	}

	id, err := uuid.Parse(notificationID)
	if err != nil {
		return err
	}

	return s.notificationsRepo.DeleteNotification(ctx, id)
}

func (s *notificationsService) DeleteOldNotifications(ctx context.Context) error {
	if s == nil || s.notificationsRepo == nil {
		return errors.New("notifications service or repository is nil")
	}

	return s.notificationsRepo.DeleteOldNotifications(ctx)
}

func (s *notificationsService) CountUnreadNotificationsCount(
	ctx context.Context,
	targetRole string,
) (int64, error) {
	if s == nil || s.notificationsRepo == nil {
		return 0, errors.New("notifications service or repository is nil")
	}

	return s.notificationsRepo.CountUnreadNotifications(ctx, targetRole)
}

func (s *notificationsService) CountNotifications(
	ctx context.Context,
	targetRole string,
) (int64, error) {
	if s == nil || s.notificationsRepo == nil {
		return 0, errors.New("notifications service or repository is nil")
	}

	return s.notificationsRepo.CountNotifications(ctx, targetRole)
}

func normalizeNotificationDeliveries(
	input model.Notification,
	saved model.Notification,
) []model.NotificationDeliveryRequest {
	if len(input.Deliveries) > 0 {
		return input.Deliveries
	}

	return []model.NotificationDeliveryRequest{
		buildInAppDelivery(
			saved.TargetRole,
			saved.Type,
			saved.Title,
			saved.Message,
			saved.Severity,
		),
	}
}

func (s *notificationsService) createNotificationDelivery(
	ctx context.Context,
	notificationID uuid.UUID,
	delivery model.NotificationDeliveryRequest,
) error {
	if s.notificationDeliveryRepo == nil {
		return nil
	}

	if delivery.Channel == "" {
		return errors.New("notification delivery channel is required")
	}

	recipient, err := marshalMapToNullRawMessage(delivery.Recipient)
	if err != nil {
		return fmt.Errorf("marshal delivery recipient: %w", err)
	}

	templateData, err := marshalMapToNullRawMessage(delivery.TemplateData)
	if err != nil {
		return fmt.Errorf("marshal delivery template data: %w", err)
	}

	payloadMap := delivery.Payload
	if len(delivery.Attachments) > 0 {
		if payloadMap == nil {
			payloadMap = map[string]any{}
		}
		payloadMap["attachments"] = delivery.Attachments
	}

	payload, err := marshalMapToNullRawMessage(payloadMap)
	if err != nil {
		return fmt.Errorf("marshal delivery payload: %w", err)
	}

	maxAttempts := delivery.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	// No worker processes in-app deliveries: the notification row Notify has
	// just stored is what the portal notification center shows, so the delivery
	// is complete on creation. Creating it PENDING left it pending forever.
	status := model.NotificationDeliveryPending
	if delivery.Channel == model.NotificationChannelInApp {
		status = model.NotificationDeliverySent
	}

	_, err = s.notificationDeliveryRepo.Create(
		ctx,
		db.CreateNotificationDeliveryParams{
			NotificationID: notificationID,
			Channel:        string(delivery.Channel),
			Recipient:      recipient,
			TemplateName: sql.NullString{
				String: strings.TrimSpace(delivery.TemplateName),
				Valid:  strings.TrimSpace(delivery.TemplateName) != "",
			},
			TemplateData: templateData,
			Payload:      payload,
			ScheduledAt: sql.NullTime{
				Time:  derefTime(delivery.ScheduledAt),
				Valid: delivery.ScheduledAt != nil,
			},
			DeliveryStatus: sql.NullString{
				String: string(status),
				Valid:  true,
			},
			DeliveryMaxAttempts: sql.NullInt32{
				Int32: maxAttempts,
				Valid: true,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("create notification delivery: %w", err)
	}

	return nil
}

func buildInAppDelivery(
	targetRole string,
	notificationType string,
	title string,
	message string,
	severity string,
) model.NotificationDeliveryRequest {
	return model.NotificationDeliveryRequest{
		Channel: model.NotificationChannelInApp,
		Recipient: map[string]any{
			"target_role": targetRole,
		},
		Payload: map[string]any{
			"type":      notificationType,
			"title":     title,
			"message":   message,
			"severity":  severity,
			"createdAt": time.Now().UTC(),
		},
		MaxAttempts: 1,
	}
}

func (s *notificationsService) buildNotificationEmailDelivery(
	subject string,
	heading string,
	message string,
	details string,
) model.NotificationDeliveryRequest {
	templateData := map[string]any{
		"Subject":     subject,
		"Heading":     heading,
		"Message":     message,
		"ActionURL":   s.adminDashboardURL(),
		"ActionLabel": "Open Dashboard",
		"Platform":    s.platformName(),
	}

	if strings.TrimSpace(details) != "" {
		templateData["Message"] = fmt.Sprintf("%s\n\n%s", message, details)
	}

	return model.NotificationDeliveryRequest{
		Channel: model.NotificationChannelEmail,
		Recipient: map[string]any{
			"name":  s.systemAdminName(),
			"email": s.systemAdminEmail(),
		},
		TemplateName: "notification",
		TemplateData: templateData,
		Payload: map[string]any{
			"subject":   subject,
			"text_body": plainTextWithDetails(message, details),
		},
		MaxAttempts: 5,
	}
}

func (s *notificationsService) buildAdminAlertEmailDelivery(
	subject string,
	message string,
	details string,
) model.NotificationDeliveryRequest {
	return model.NotificationDeliveryRequest{
		Channel: model.NotificationChannelEmail,
		Recipient: map[string]any{
			"name":  s.systemAdminName(),
			"email": s.systemAdminEmail(),
		},
		TemplateName: "admin-alert",
		TemplateData: map[string]any{
			"Platform":  s.platformName(),
			"Message":   message,
			"Details":   details,
			"ActionURL": s.adminDashboardURL(),
		},
		Payload: map[string]any{
			"subject":   subject,
			"text_body": plainTextWithDetails(message, details),
		},
		MaxAttempts: 5,
	}
}

func (s *notificationsService) platformName() string {
	if s != nil && s.cfg != nil {
		value := strings.TrimSpace(s.cfg.Notification.PlatformName)
		if value != "" {
			return value
		}
	}

	return fallbackPlatformName
}

func (s *notificationsService) systemAdminName() string {
	if s != nil && s.cfg != nil {
		value := strings.TrimSpace(s.cfg.Notification.SystemAdminName)
		if value != "" {
			return value
		}
	}

	return fallbackSystemAdminName
}

func (s *notificationsService) systemAdminEmail() string {
	if s != nil && s.cfg != nil {
		value := strings.TrimSpace(s.cfg.Notification.SystemAdminEmail)
		if value != "" {
			return value
		}
	}

	return fallbackSystemAdminEmail
}

func (s *notificationsService) adminDashboardURL() string {
	if s != nil && s.cfg != nil {
		value := strings.TrimSpace(s.cfg.Notification.AdminDashboardURL)
		if value != "" {
			return value
		}
	}

	return fallbackAdminDashboardURL
}

func plainTextWithDetails(message string, details string) string {
	message = strings.TrimSpace(message)
	details = strings.TrimSpace(details)

	if details == "" {
		return message
	}

	return fmt.Sprintf("%s\n\n%s", message, details)
}

func marshalMapToNullRawMessage(value map[string]any) (pqtype.NullRawMessage, error) {
	if value == nil {
		return pqtype.NullRawMessage{
			Valid: false,
		}, nil
	}

	b, err := json.Marshal(value)
	if err != nil {
		return pqtype.NullRawMessage{}, err
	}

	return pqtype.NullRawMessage{
		RawMessage: b,
		Valid:      true,
	}, nil
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}

	return *value
}

func defaultNotificationPreferences(userID string) NotificationPreferences {
	now := time.Now().UTC()

	return NotificationPreferences{
		UserID:       userID,
		EmailEnabled: true,
		SMSEnabled:   false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func toNotificationPreferences(
	preference notificationPreferencesRepository.NotificationPreference,
) NotificationPreferences {
	return NotificationPreferences{
		UserID:          preference.UserID,
		EmailEnabled:    preference.EmailEnabled,
		SMSEnabled:      preference.SMSEnabled,
		PhoneNumber:     nullStringToString(preference.PhoneNumber),
		PhoneVerified:   preference.PhoneVerified,
		QuietHoursStart: nullStringToString(preference.QuietHoursStart),
		QuietHoursEnd:   nullStringToString(preference.QuietHoursEnd),
		CreatedAt:       preference.CreatedAt,
		UpdatedAt:       preference.UpdatedAt,
	}
}

func validateQuietHour(name string, value string) error {
	if value == "" {
		return nil
	}

	if _, err := time.Parse("15:04", value); err != nil {
		return fmt.Errorf("%s must use HH:MM format", name)
	}

	return nil
}

func sqlNullString(value string) sql.NullString {
	value = strings.TrimSpace(value)

	return sql.NullString{
		String: value,
		Valid:  value != "",
	}
}

func nullStringToString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}
