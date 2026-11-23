// Package identitysync defines the boundary between identity reconciliation and
// the application that owns the local accounts.
package identitysync

import (
	"context"
	"errors"
)

// Identity is the stable OIDC account key. The issuer must come from a trusted
// provider and the subject from that provider's verified identity.
type Identity struct {
	Issuer  string
	Subject string
}

// Account describes an eligible local account. Email is a matching hint, not
// an identity key. Link is the account's current link for the application's
// configured issuer (contract 1.1); the zero Identity means not linked or a 1.0
// application that does not report links. Account stays comparable with ==.
type Account struct {
	ID    string
	Email string
	Link  Identity
}

// AccountPage contains eligible accounts and an opaque continuation cursor.
// An empty NextCursor marks the last page.
type AccountPage struct {
	Accounts   []Account
	NextCursor string
}

var (
	ErrAccountNotFound   = errors.New("eligible local account not found")
	ErrIdentityNotLinked = errors.New("identity is not linked")
	ErrLinkConflict      = errors.New("identity link conflicts with an existing account link")
	// ErrUnsupported reports an operation the application's contract version lacks.
	ErrUnsupported = errors.New("operation not supported by the application's identity-sync contract version")
)

// AccountStore is scoped to one application. The application owns account
// eligibility, authorization, persistence and all local role assignments.
type AccountStore interface {
	// ListEligibleAccounts starts at the first page when cursor is empty.
	ListEligibleAccounts(ctx context.Context, cursor string, limit int) (AccountPage, error)

	// FindLinkedAccount returns ErrIdentityNotLinked when no link exists and
	// ErrAccountNotFound when its account is no longer eligible.
	FindLinkedAccount(ctx context.Context, identity Identity) (Account, error)

	// LinkIdentity atomically checks eligibility and creates a link. Repeating
	// the same link succeeds. A different account for the same issuer/subject,
	// or a different subject for the same account/issuer, returns ErrLinkConflict.
	LinkIdentity(ctx context.Context, accountID string, identity Identity) error
}

// SessionRevoker ends every local session of the account linked to identity
// (contract 1.1). It succeeds when the identity is unlinked or has no sessions.
type SessionRevoker interface {
	RevokeSessions(ctx context.Context, identity Identity) error
}
