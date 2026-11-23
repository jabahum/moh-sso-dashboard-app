package modulekit

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Notification channels understood by the platform delivery workers.
const (
	ChannelInApp   = "in_app"
	ChannelEmail   = "email"
	ChannelSMS     = "sms"
	ChannelWebhook = "webhook"
)

// Notification is a platform notification raised by a module. The host stores
// it and queues each delivery; with no deliveries the host creates its default
// in-app delivery.
type Notification struct {
	Type       string
	Title      string
	Message    string
	Severity   string // info | warning | critical
	TargetRole string // admin | super_admin | ...
	ClientID   string
	UserID     string
	Metadata   json.RawMessage
	Deliveries []NotificationDelivery
}

// NotificationDelivery requests delivery over one channel.
type NotificationDelivery struct {
	Channel      string
	Recipient    map[string]any
	TemplateName string
	TemplateData map[string]any
	Payload      map[string]any
	Attachments  []NotificationAttachment
	ScheduledAt  *time.Time
	MaxAttempts  int32
}

// NotificationAttachment is a file attached to an email delivery. Supply
// either Path or Data.
type NotificationAttachment struct {
	FileName    string
	ContentType string
	Path        string
	Data        []byte
	Inline      bool
	ContentID   string
}

// Notifier is the host's notification service. Modules treat delivery as best
// effort unless their own contract says otherwise.
type Notifier interface {
	Notify(ctx context.Context, notification Notification) error
}

// Platform describes the installation for messages modules send to people.
type Platform struct {
	Name              string
	SystemAdminName   string
	SystemAdminEmail  string
	AdminDashboardURL string
}

// DisplayName returns the platform name, or the default portal name.
func (p Platform) DisplayName() string {
	if name := strings.TrimSpace(p.Name); name != "" {
		return name
	}
	return "MOH Integrated Health Portal"
}

// AdminName returns the system administrator's name, or a generic title.
func (p Platform) AdminName() string {
	if name := strings.TrimSpace(p.SystemAdminName); name != "" {
		return name
	}
	return "System Administrator"
}

// AdminEmail returns the system administrator's address, or "".
func (p Platform) AdminEmail() string { return strings.TrimSpace(p.SystemAdminEmail) }

// DashboardURL returns the admin dashboard URL, or the local development default.
func (p Platform) DashboardURL() string {
	if url := strings.TrimSpace(p.AdminDashboardURL); url != "" {
		return url
	}
	return "http://localhost:3000/admin/home"
}

// AdminPageURL returns the URL of an admin page such as "storage-locations",
// resolved against the dashboard URL the same way the host does.
func (p Platform) AdminPageURL(page string) string {
	base := strings.TrimRight(p.DashboardURL(), "/")
	page = strings.Trim(page, "/")
	if strings.HasSuffix(base, "/admin/home") {
		return strings.TrimSuffix(base, "/admin/home") + "/admin/" + page
	}
	return base + "/" + page
}
