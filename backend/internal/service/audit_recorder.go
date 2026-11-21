package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/utils"
	"github.com/sqlc-dev/pqtype"
)

type AuditService struct {
	store         db.Store
	cache         *cache.RedisCache
	notifications NotificationsService
	cfg           *config.Config
}

func NewAuditService(
	store db.Store,
	cache *cache.RedisCache,
	notifications NotificationsService,
	cfg *config.Config,
) *AuditService {
	return &AuditService{
		store:         store,
		cache:         cache,
		notifications: notifications,
		cfg:           cfg,
	}
}

//
// ----------------------------------------------------
// Internal helpers
// ----------------------------------------------------
//

// Ensures we never violate FK constraints.
// If the user does not exist locally, we downgrade to NULL (system).
func (a *AuditService) safeUserID(
	ctx context.Context,
	userID uuid.NullUUID,
) uuid.NullUUID {
	if a == nil || a.store == nil {
		return uuid.NullUUID{}
	}

	// Anonymous / system event
	if !userID.Valid {
		return userID
	}

	exists, err := a.store.UserExists(ctx, userID.UUID)
	if err != nil || !exists {
		return uuid.NullUUID{}
	}

	return userID
}

func (a *AuditService) write(
	ctx context.Context,
	userID uuid.NullUUID,
	action string,
	metadata map[string]interface{},
) error {
	if a == nil {
		return errors.New("audit service is nil")
	}

	if a.store == nil {
		return errors.New("audit store is nil")
	}

	action = strings.TrimSpace(action)
	if action == "" {
		return errors.New("audit action is required")
	}

	safeID := a.safeUserID(ctx, userID)

	if err := a.store.CreateAuditLog(ctx, db.CreateAuditLogParams{
		UserID: safeID,
		Action: action,
		Metadata: pqtype.NullRawMessage{
			RawMessage: utils.Encode(metadata),
			Valid:      metadata != nil,
		},
	}); err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}

	if a.cache != nil {
		_ = a.cache.DeletePattern(ctx, "audit_logs:*")
		_ = a.cache.DeletePattern(ctx, "audit_actions:*")
		_ = a.cache.DeletePattern(ctx, "audit_metrics:*")
	}

	a.notifyForAuditEvent(ctx, safeID, action, metadata)

	return nil
}

func (a *AuditService) notifyForAuditEvent(
	ctx context.Context,
	userID uuid.NullUUID,
	action string,
	metadata map[string]interface{},
) {
	if a == nil || a.notifications == nil {
		return
	}

	notification := a.auditNotificationFromAction(userID, action, metadata)
	if notification == nil {
		return
	}

	_, _ = a.notifications.Notify(ctx, *notification)
}

func (a *AuditService) auditNotificationFromAction(
	userID uuid.NullUUID,
	action string,
	metadata map[string]interface{},
) *model.Notification {
	switch action {
	case "auth.login":
		success, _ := metadataBool(metadata, "success")
		if success {
			return nil
		}

		notification := model.Notification{
			Type:       "AUTH_LOGIN_FAILED",
			Title:      "Failed login attempt",
			Severity:   "warning",
			Message:    "A failed login attempt was recorded",
			TargetRole: "admin",
			UserID:     nullUUIDStringValue(userID),
			Metadata: utils.MustJSON(map[string]any{
				"action":     action,
				"user_id":    nullUUIDStringValue(userID),
				"client_id":  metadataString(metadata, "client_id"),
				"ip":         metadataString(metadata, "ip"),
				"user_agent": metadataString(metadata, "user_agent"),
				"country":    metadataString(metadata, "country"),
				"city":       metadataString(metadata, "city"),
				"success":    success,
			}),
		}

		a.attachAdminEmailDelivery(
			&notification,
			"login-failed",
			"Failed login attempt",
			"A failed login attempt was recorded.",
			map[string]any{
				"Name":      a.systemAdminName(),
				"Platform":  a.platformName(),
				"Username":  nullUUIDStringValue(userID),
				"IP":        metadataString(metadata, "ip"),
				"UserAgent": metadataString(metadata, "user_agent"),
				"ActionURL": a.adminDashboardURL(),
				"Details": fmt.Sprintf(
					"Client ID: %s\nCountry: %s\nCity: %s\nUser Agent: %s",
					metadataString(metadata, "client_id"),
					metadataString(metadata, "country"),
					metadataString(metadata, "city"),
					metadataString(metadata, "user_agent"),
				),
			},
		)

		return &notification

	case "auth.refresh_failed":
		notification := model.Notification{
			Type:       "AUTH_TOKEN_REFRESH_FAILED",
			Title:      "Token refresh failed",
			Severity:   "warning",
			Message:    "A token refresh attempt failed",
			TargetRole: "admin",
			UserID:     nullUUIDStringValue(userID),
			Metadata: utils.MustJSON(map[string]any{
				"action":     action,
				"user_id":    nullUUIDStringValue(userID),
				"ip":         metadataString(metadata, "ip"),
				"user_agent": metadataString(metadata, "user_agent"),
				"success":    false,
			}),
		}

		a.attachAdminEmailDelivery(
			&notification,
			"admin-alert",
			"Token refresh failed",
			"A token refresh attempt failed.",
			map[string]any{
				"Name":      a.systemAdminName(),
				"Platform":  a.platformName(),
				"Message":   "A token refresh attempt failed.",
				"ActionURL": a.adminDashboardURL(),
				"Details": fmt.Sprintf(
					"User ID: %s\nIP: %s\nUser Agent: %s",
					nullUUIDStringValue(userID),
					metadataString(metadata, "ip"),
					metadataString(metadata, "user_agent"),
				),
			},
		)

		return &notification

	case "auth.password_reset":
		success, _ := metadataBool(metadata, "success")
		severity := "info"
		title := "Password reset recorded"
		message := "A password reset event was recorded"

		if !success {
			severity = "warning"
			title = "Password reset failed"
			message = "A password reset attempt failed"
		}

		notification := model.Notification{
			Type:       "AUTH_PASSWORD_RESET",
			Title:      title,
			Severity:   severity,
			Message:    message,
			TargetRole: "admin",
			UserID:     nullUUIDStringValue(userID),
			Metadata: utils.MustJSON(map[string]any{
				"action":  action,
				"user_id": nullUUIDStringValue(userID),
				"success": success,
				"ip":      metadataString(metadata, "ip"),
			}),
		}

		if !success {
			a.attachAdminEmailDelivery(
				&notification,
				"admin-alert",
				title,
				message,
				map[string]any{
					"Name":      a.systemAdminName(),
					"Platform":  a.platformName(),
					"Message":   message,
					"ActionURL": a.adminDashboardURL(),
					"Details": fmt.Sprintf(
						"User ID: %s\nIP: %s\nSuccess: %v",
						nullUUIDStringValue(userID),
						metadataString(metadata, "ip"),
						success,
					),
				},
			)
		}

		return &notification

	case "client.create":
		// In-app only. ClientService already handles client lifecycle notifications.
		return &model.Notification{
			Type:       "CLIENT_CREATED",
			Title:      "Client created",
			Severity:   "info",
			Message:    "Client application created",
			TargetRole: "admin",
			UserID:     nullUUIDStringValue(userID),
			ClientID:   metadataString(metadata, "client_id"),
			Metadata: utils.MustJSON(map[string]any{
				"action":     action,
				"admin_id":   nullUUIDStringValue(userID),
				"client_id":  metadataString(metadata, "client_id"),
				"ip":         metadataString(metadata, "ip"),
				"user_agent": metadataString(metadata, "user_agent"),
			}),
		}

	case "client.delete":
		notification := model.Notification{
			Type:       "CLIENT_DELETED",
			Title:      "Client deleted",
			Severity:   "critical",
			Message:    "Client application deleted",
			TargetRole: "admin",
			UserID:     nullUUIDStringValue(userID),
			ClientID:   metadataString(metadata, "client_id"),
			Metadata: utils.MustJSON(map[string]any{
				"action":     action,
				"admin_id":   nullUUIDStringValue(userID),
				"client_id":  metadataString(metadata, "client_id"),
				"ip":         metadataString(metadata, "ip"),
				"user_agent": metadataString(metadata, "user_agent"),
			}),
		}

		a.attachAdminEmailDelivery(
			&notification,
			"client-deleted",
			"Client deleted",
			"Client application deleted.",
			map[string]any{
				"Name":       a.systemAdminName(),
				"Platform":   a.platformName(),
				"ClientID":   metadataString(metadata, "client_id"),
				"ClientName": metadataString(metadata, "client_id"),
				"ActionURL":  a.adminDashboardURL(),
				"Details": fmt.Sprintf(
					"Admin ID: %s\nClient ID: %s\nIP: %s\nUser Agent: %s",
					nullUUIDStringValue(userID),
					metadataString(metadata, "client_id"),
					metadataString(metadata, "ip"),
					metadataString(metadata, "user_agent"),
				),
			},
		)

		return &notification

	default:
		if isSensitiveAdminAction(action) {
			notification := model.Notification{
				Type:       "ADMIN_ACTION_RECORDED",
				Title:      "Admin action recorded",
				Severity:   adminActionSeverity(action),
				Message:    "A sensitive admin action was recorded",
				TargetRole: "admin",
				UserID:     nullUUIDStringValue(userID),
				Metadata: utils.MustJSON(map[string]any{
					"action":   action,
					"admin_id": nullUUIDStringValue(userID),
					"metadata": metadata,
				}),
			}

			if shouldEmailAdminAction(action) {
				a.attachAdminEmailDelivery(
					&notification,
					"admin-alert",
					"Sensitive admin action recorded",
					"A sensitive admin action was recorded.",
					map[string]any{
						"Name":      a.systemAdminName(),
						"Platform":  a.platformName(),
						"Message":   "A sensitive admin action was recorded.",
						"ActionURL": a.adminDashboardURL(),
						"Details": fmt.Sprintf(
							"Action: %s\nAdmin ID: %s\nMetadata: %v",
							action,
							nullUUIDStringValue(userID),
							metadata,
						),
					},
				)
			}

			return &notification
		}

		return nil
	}
}

func (a *AuditService) attachAdminEmailDelivery(
	notification *model.Notification,
	templateName string,
	subject string,
	textBody string,
	templateData map[string]any,
) {
	if notification == nil {
		return
	}

	adminEmail := strings.TrimSpace(a.systemAdminEmail())
	if adminEmail == "" {
		return
	}

	if templateData == nil {
		templateData = map[string]any{}
	}

	if _, ok := templateData["Name"]; !ok {
		templateData["Name"] = a.systemAdminName()
	}

	if _, ok := templateData["Platform"]; !ok {
		templateData["Platform"] = a.platformName()
	}

	if _, ok := templateData["ActionURL"]; !ok {
		templateData["ActionURL"] = a.adminDashboardURL()
	}

	notification.Deliveries = []model.NotificationDeliveryRequest{
		{
			Channel: model.NotificationChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		},
		{
			Channel: model.NotificationChannelEmail,
			Recipient: map[string]any{
				"name":  a.systemAdminName(),
				"email": adminEmail,
			},
			TemplateName: templateName,
			TemplateData: templateData,
			Payload: map[string]any{
				"subject":   subject,
				"text_body": textBody,
			},
			MaxAttempts: 5,
		},
	}
}

func isSensitiveAdminAction(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))

	if action == "" {
		return false
	}

	sensitivePrefixes := []string{
		"user.",
		"role.",
		"client.",
		"storage.",
		"document.delete",
		"template.delete",
		"template.archive",
		"surveillance.",
		"system.",
		"backup.",
	}

	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}

	return false
}

func shouldEmailAdminAction(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))

	if action == "" {
		return false
	}

	emailKeywords := []string{
		"delete",
		"disable",
		"archive",
		"failed",
		"failure",
		"role.assign",
		"role.remove",
		"backup.",
		"system.",
		"storage.delete",
		"storage.update",
	}

	for _, keyword := range emailKeywords {
		if strings.Contains(action, keyword) {
			return true
		}
	}

	return false
}

func adminActionSeverity(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))

	switch {
	case strings.Contains(action, "delete"):
		return "critical"
	case strings.Contains(action, "disable"):
		return "warning"
	case strings.Contains(action, "archive"):
		return "warning"
	case strings.Contains(action, "failed"):
		return "warning"
	default:
		return "info"
	}
}

func metadataString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}

	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}

	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func metadataBool(metadata map[string]interface{}, key string) (bool, bool) {
	if metadata == nil {
		return false, false
	}

	value, ok := metadata[key]
	if !ok || value == nil {
		return false, false
	}

	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return false, false
		}
	default:
		switch strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", value))) {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return false, false
		}
	}
}

func nullUUIDStringValue(value uuid.NullUUID) string {
	if !value.Valid {
		return ""
	}

	return value.UUID.String()
}

func (a *AuditService) platformName() string {
	if a != nil && a.cfg != nil && strings.TrimSpace(a.cfg.Notification.PlatformName) != "" {
		return strings.TrimSpace(a.cfg.Notification.PlatformName)
	}

	return "MOH Integrated Health Portal"
}

func (a *AuditService) systemAdminName() string {
	if a != nil && a.cfg != nil && strings.TrimSpace(a.cfg.Notification.SystemAdminName) != "" {
		return strings.TrimSpace(a.cfg.Notification.SystemAdminName)
	}

	return "System Administrator"
}

func (a *AuditService) systemAdminEmail() string {
	if a != nil && a.cfg != nil && strings.TrimSpace(a.cfg.Notification.SystemAdminEmail) != "" {
		return strings.TrimSpace(a.cfg.Notification.SystemAdminEmail)
	}

	return ""
}

func (a *AuditService) adminDashboardURL() string {
	if a != nil && a.cfg != nil && strings.TrimSpace(a.cfg.Notification.AdminDashboardURL) != "" {
		return strings.TrimSpace(a.cfg.Notification.AdminDashboardURL)
	}

	return "http://localhost:3000/admin/home"
}

//
// ----------------------------------------------------
// Public API (handlers use ONLY these)
// ----------------------------------------------------
//

// Generic logger (fallback)
func (a *AuditService) Log(
	ctx context.Context,
	userID uuid.NullUUID,
	action string,
	metadata map[string]interface{},
) error {
	return a.write(ctx, userID, action, metadata)
}

// --------------------
// AUTH EVENTS
// --------------------

func (a *AuditService) LoginInitiated(
	ctx context.Context,
	ip string,
	userAgent string,
	clientID string,
) error {
	return a.write(ctx, uuid.NullUUID{}, "auth.login_initiated", map[string]interface{}{
		"ip":         strings.TrimSpace(ip),
		"user_agent": strings.TrimSpace(userAgent),
		"client_id":  strings.TrimSpace(clientID),
	})
}

func (a *AuditService) LoginResult(
	ctx context.Context,
	userID uuid.NullUUID,
	success bool,
	clientID string,
	ip string,
	userAgent string,
	country string,
	city string,
) error {
	return a.write(ctx, userID, "auth.login", map[string]interface{}{
		"success":    success,
		"client_id":  strings.TrimSpace(clientID),
		"ip":         strings.TrimSpace(ip),
		"user_agent": strings.TrimSpace(userAgent),
		"country":    strings.TrimSpace(country),
		"city":       strings.TrimSpace(city),
	})
}

func (a *AuditService) TokenRefresh(
	ctx context.Context,
	userID uuid.NullUUID,
	success bool,
	ip string,
	userAgent string,
) error {
	action := "auth.refresh_success"
	if !success {
		action = "auth.refresh_failed"
	}

	return a.write(ctx, userID, action, map[string]interface{}{
		"success":    success,
		"ip":         strings.TrimSpace(ip),
		"user_agent": strings.TrimSpace(userAgent),
	})
}

func (a *AuditService) Logout(
	ctx context.Context,
	userID uuid.NullUUID,
	ip string,
	userAgent string,
) error {
	return a.write(ctx, userID, "auth.logout", map[string]interface{}{
		"ip":         strings.TrimSpace(ip),
		"user_agent": strings.TrimSpace(userAgent),
	})
}

func (a *AuditService) PasswordReset(
	ctx context.Context,
	userID uuid.NullUUID,
	success bool,
	ip string,
) error {
	return a.write(ctx, userID, "auth.password_reset", map[string]interface{}{
		"success": success,
		"ip":      strings.TrimSpace(ip),
	})
}

// --------------------
// CLIENT / ADMIN EVENTS
// --------------------

func (a *AuditService) ClientCreated(
	ctx context.Context,
	adminID uuid.NullUUID,
	clientID string,
	ip string,
	userAgent string,
) error {
	return a.write(ctx, adminID, "client.create", map[string]interface{}{
		"client_id":  strings.TrimSpace(clientID),
		"ip":         strings.TrimSpace(ip),
		"user_agent": strings.TrimSpace(userAgent),
	})
}

func (a *AuditService) ClientDeleted(
	ctx context.Context,
	adminID uuid.NullUUID,
	clientID string,
	ip string,
	userAgent string,
) error {
	return a.write(ctx, adminID, "client.delete", map[string]interface{}{
		"client_id":  strings.TrimSpace(clientID),
		"ip":         strings.TrimSpace(ip),
		"user_agent": strings.TrimSpace(userAgent),
	})
}

func (a *AuditService) AdminAction(
	ctx context.Context,
	adminID uuid.NullUUID,
	action string, // e.g. user.disable, role.assign
	metadata map[string]interface{},
) error {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}

	return a.write(ctx, adminID, strings.TrimSpace(action), metadata)
}

// RecordAudit persists a portable audit event using the existing host recorder.
func (a *AuditService) RecordAudit(ctx context.Context, event core.AuditEvent) error {
	var actor uuid.NullUUID
	if event.Subject != "" {
		id, err := uuid.Parse(event.Subject)
		if err != nil {
			return fmt.Errorf("invalid audit subject: %w", err)
		}
		actor = uuid.NullUUID{UUID: id, Valid: true}
	}
	return a.write(ctx, actor, event.Action, event.Metadata)
}
