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
// an identity key.
type Account struct {
	ID    string
	Email string
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
