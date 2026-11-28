package client

import (
	"net/mail"
	"strings"
)

// Identity is the calling machine client as the portal sees it.
type Identity struct {
	ApplicationID string `json:"applicationId"`
	ClientID      string `json:"clientId"`
	Subject       string `json:"subject"`
	// Scopes are the effective scopes: in the token and granted by the binding.
	Scopes []string `json:"scopes"`
}

// HasScope reports whether scope is effective.
func (i Identity) HasScope(scope string) bool {
	for _, s := range i.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// Readiness is the portal's readiness report.
type Readiness struct {
	Ready   bool           `json:"ready"`
	Workers []WorkerHealth `json:"workers"`
}

// WorkerHealth is one background worker's health.
type WorkerHealth struct {
	Name     string `json:"name"`
	Healthy  bool   `json:"healthy"`
	LastSeen string `json:"lastSeen,omitempty"`
}

// Recipient is a notification's single recipient.
type Recipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// Notification is an email notification to queue. Don't put
// patient-identifiable or other sensitive content in it.
type Notification struct {
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Recipient Recipient `json:"recipient"`
}

func (n Notification) validate() error {
	title, message := strings.TrimSpace(n.Title), strings.TrimSpace(n.Message)
	switch {
	case title == "" || len(title) > 200 || strings.ContainsAny(n.Title, "\r\n"):
		return invalid("the title is required, at most 200 characters, without line breaks")
	case message == "" || len(message) > 10000:
		return invalid("the message is required, at most 10,000 characters")
	case len(n.Recipient.Name) > 200 || strings.ContainsAny(n.Recipient.Name, "\r\n"):
		return invalid("the recipient name is at most 200 characters, without line breaks")
	}
	email := strings.TrimSpace(n.Recipient.Email)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return invalid("the recipient email must be one plain address")
	}
	return nil
}

// NotificationStatus is a notification's delivery status.
type NotificationStatus struct {
	ID            string `json:"id"`
	ApplicationID string `json:"applicationId"`
	Channel       string `json:"channel"`
	// Status is the overall status: DispatchStatus until it is SENT, then the
	// email outbox's status. EXPIRED when the delivery record no longer exists.
	Status string `json:"status"`
	// DispatchStatus is the portal's delivery queue status: PENDING,
	// PROCESSING, RETRY, SENT, FAILED or CANCELLED.
	DispatchStatus   string `json:"dispatchStatus"`
	DispatchAttempts int    `json:"dispatchAttempts"`
	DeliveryAttempts int    `json:"deliveryAttempts"`
	CreatedAt        string `json:"createdAt"`
	// Replayed is true when an earlier request with the same idempotency key
	// already created this notification.
	Replayed bool `json:"replayed"`
}

// UserAccess is a user's current access to one application.
type UserAccess struct {
	ApplicationID string   `json:"applicationId"`
	Subject       string   `json:"subject"`
	Roles         []string `json:"roles"`
	Permissions   []string `json:"permissions"`
}

// HasPermission reports whether the user holds permission in the application.
func (a UserAccess) HasPermission(permission string) bool {
	for _, p := range a.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}
