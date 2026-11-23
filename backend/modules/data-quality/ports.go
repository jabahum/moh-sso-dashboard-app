package dataquality

import (
	"context"
	"net/http"
)

// Ports the host supplies. Each declares only the calls this module makes.

// DirectoryGroup is an identity-provider group.
type DirectoryGroup struct {
	ID   string
	Name string
	Path string
}

// DirectoryUser is an identity-provider user.
type DirectoryUser struct {
	ID        string
	Username  string
	Email     string
	FirstName string
	LastName  string
}

// UserDirectory is the host's identity-provider admin client (Keycloak). The
// issue tracker lists groups for assignment pickers, resolves who resolved an
// issue, and finds the resolver's group colleagues to notify.
type UserDirectory interface {
	ListGroups(ctx context.Context) ([]DirectoryGroup, error)
	ListGroupMembers(ctx context.Context, groupID string) ([]DirectoryUser, error)
	// FindUsers searches users by username/email; exact requests an exact match.
	FindUsers(ctx context.Context, query string, exact bool) ([]DirectoryUser, error)
	GetUserGroups(ctx context.Context, userID string) ([]DirectoryGroup, error)
	// GetUser returns a user by ID; a nil user with a nil error means not found.
	GetUser(userID string) (*DirectoryUser, error)
}

// EmailMessage is a ready-to-send issue-tracker email.
type EmailMessage struct {
	To       []string
	Subject  string
	HTMLBody string
	TextBody string
}

// Mailer sends issue-tracker emails. Send uses the dedicated issue-tracker
// SMTP account; Fallback uses the platform email service and is tried only when
// Send fails.
type Mailer interface {
	Send(ctx context.Context, message EmailMessage) error
	Fallback(ctx context.Context, message EmailMessage) error
}

// CallerName returns the verified caller's email, or username, or "" when the
// host has neither. The module then falls back to the caller's subject.
type CallerName func(r *http.Request) string
