// Package modulekit adapts host services to the core/modulekit contracts that
// compiled application modules consume.
package modulekit

import (
	"context"
	"errors"

	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/service"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// Notifier delivers module notifications through the host notification service.
type Notifier struct{ Service service.NotificationsService }

var _ modulekit.Notifier = Notifier{}

func (n Notifier) Notify(ctx context.Context, notification modulekit.Notification) error {
	if n.Service == nil {
		return errors.New("notification service is unavailable")
	}
	_, err := n.Service.Notify(ctx, ToModel(notification))
	return err
}

// ToModel converts a module notification to the host model.
func ToModel(notification modulekit.Notification) model.Notification {
	out := model.Notification{
		Type: notification.Type, Title: notification.Title, Message: notification.Message,
		Severity: notification.Severity, TargetRole: notification.TargetRole,
		ClientID: notification.ClientID, UserID: notification.UserID, Metadata: notification.Metadata,
	}
	for _, delivery := range notification.Deliveries {
		converted := model.NotificationDeliveryRequest{
			Channel: model.NotificationChannel(delivery.Channel), Recipient: delivery.Recipient,
			TemplateName: delivery.TemplateName, TemplateData: delivery.TemplateData, Payload: delivery.Payload,
			ScheduledAt: delivery.ScheduledAt, MaxAttempts: delivery.MaxAttempts,
		}
		for _, attachment := range delivery.Attachments {
			converted.Attachments = append(converted.Attachments, model.Attachment{
				FileName: attachment.FileName, ContentType: attachment.ContentType, Path: attachment.Path,
				Data: attachment.Data, Inline: attachment.Inline, ContentID: attachment.ContentID,
			})
		}
		out.Deliveries = append(out.Deliveries, converted)
	}
	return out
}

// Platform returns the installation details modules use in messages.
func Platform(cfg *config.Config) modulekit.Platform {
	if cfg == nil {
		return modulekit.Platform{}
	}
	return modulekit.Platform{
		Name: cfg.Notification.PlatformName, SystemAdminName: cfg.Notification.SystemAdminName,
		SystemAdminEmail: cfg.Notification.SystemAdminEmail, AdminDashboardURL: cfg.Notification.AdminDashboardURL,
	}
}
