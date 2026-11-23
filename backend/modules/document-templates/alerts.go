package documenttemplates

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// alerts raises the platform notifications the template, sheet and column
// services send. Each service previously carried an identical copy of these
// helpers; their payloads, template names and admin URL are unchanged.
type alerts struct {
	notifier modulekit.Notifier
	platform modulekit.Platform
}

// notify is best effort, as before extraction.
func (a alerts) notify(ctx context.Context, notification modulekit.Notification) {
	if a.notifier == nil {
		return
	}

	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}

	_ = a.notifier.Notify(ctx, notification)
}

func (a alerts) attachAdminEmailDelivery(
	notification *modulekit.Notification,
	templateName string,
	subject string,
	textBody string,
	templateData map[string]any,
) {
	if notification == nil {
		return
	}

	adminEmail := a.systemAdminEmail()
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
		templateData["ActionURL"] = a.adminTemplatesURL()
	}

	notification.Deliveries = []modulekit.NotificationDelivery{
		{
			Channel: modulekit.ChannelInApp,
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
			Channel: modulekit.ChannelEmail,
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

func (a alerts) platformName() string { return a.platform.DisplayName() }

func (a alerts) systemAdminName() string { return a.platform.AdminName() }

func (a alerts) systemAdminEmail() string { return a.platform.AdminEmail() }

// adminTemplatesURL is the dashboard's document-templates page: the dashboard
// URL with "/admin/home" replaced by "/admin/document-templates", or with
// "/document-templates" appended otherwise (modulekit.Platform.AdminPageURL).
func (a alerts) adminTemplatesURL() string { return a.platform.AdminPageURL("document-templates") }

func mustJSON(value any) json.RawMessage {
	body, _ := json.Marshal(value)
	return body
}
