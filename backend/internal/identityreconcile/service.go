// Package identityreconcile coordinates reviewed application account links.
package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// Directory is scoped to a single configured Keycloak realm and app access
// role. Implementations must return stable subjects and make EnsureUser and
// GrantAccess idempotent, including when an earlier request timed out.
type Directory interface {
	// FindEmail returns all email matches, including unverified accounts.
	// Only a verified, active account may supply a nonzero Identity.
	FindEmail(ctx context.Context, email string) ([]identitysync.Identity, error)
	// EnsureUser must atomically reject unrelated existing accounts, including
	// email collisions. It may reuse a user it created for this same app and
	// local account ID on a prior attempt.
	EnsureUser(ctx context.Context, account identitysync.Account) (identitysync.Identity, error)
	GrantAccess(ctx context.Context, identity identitysync.Identity) error
}

type Action string

const (
	Review Action = "review"
	Reuse  Action = "reuse"
	Create Action = "create"
)

type Decision struct {
	Account  identitysync.Account
	Action   Action
	Identity identitysync.Identity
}

type PreviewPage struct {
	Decisions  []Decision
	NextCursor string
}

// Accounts is the application-side capability needed for reconciliation.
type Accounts interface {
	ListEligibleAccounts(context.Context, string, int) (identitysync.AccountPage, error)
	LinkIdentity(context.Context, string, identitysync.Identity) error
}

type Service struct {
	Accounts  Accounts
	Directory Directory
	Issuer    string
}

// Preview performs reads only. Directory errors stop the preview instead of
// treating unknown results as permission to create an identity.
func (s Service) Preview(ctx context.Context, cursor string, limit int) (PreviewPage, error) {
	if limit <= 0 {
		return PreviewPage{}, errors.New("page limit must be positive")
	}
	page, err := s.Accounts.ListEligibleAccounts(ctx, cursor, limit)
	if err != nil {
		return PreviewPage{}, fmt.Errorf("list eligible accounts: %w", err)
	}
	result := PreviewPage{NextCursor: page.NextCursor}
	for _, account := range page.Accounts {
		decision, err := s.propose(ctx, account)
		if err != nil {
			return PreviewPage{}, err
		}
		result.Decisions = append(result.Decisions, decision)
	}
	return result, nil
}

func (s Service) propose(ctx context.Context, account identitysync.Account) (Decision, error) {
	decision := Decision{Account: account, Action: Review}
	if account.ID == "" || strings.TrimSpace(account.Email) == "" {
		return decision, nil
	}
	matches, err := s.Directory.FindEmail(ctx, strings.TrimSpace(account.Email))
	if err != nil {
		return Decision{}, fmt.Errorf("look up directory matches: %w", err)
	}
	switch len(matches) {
	case 0:
		decision.Action = Create
	case 1:
		if s.valid(matches[0]) {
			decision.Action = Reuse
			decision.Identity = matches[0]
		}
	}
	return decision, nil
}

// Apply executes one decision from a trusted review record. The host must
// authenticate the reviewer and reload the current eligible account before
// calling Apply. Only the app's configured access role may be granted.
func (s Service) Apply(ctx context.Context, reviewed Decision, current identitysync.Account) error {
	if current != reviewed.Account || (reviewed.Action != Reuse && reviewed.Action != Create) {
		return errors.New("decision requires a fresh review")
	}
	identity := reviewed.Identity
	if reviewed.Action == Create {
		if s.Issuer == "" || current.ID == "" || strings.TrimSpace(current.Email) == "" || identity != (identitysync.Identity{}) {
			return errors.New("invalid account creation decision")
		}
		created, err := s.Directory.EnsureUser(ctx, current)
		if err != nil {
			return fmt.Errorf("ensure directory user: %w", err)
		}
		identity = created
	} else {
		proposed, err := s.propose(ctx, current)
		if err != nil {
			return err
		}
		if proposed != reviewed {
			return errors.New("directory matches changed since review")
		}
	}
	if !s.valid(identity) {
		return errors.New("directory returned an invalid identity")
	}
	if err := s.Accounts.LinkIdentity(ctx, current.ID, identity); err != nil {
		return fmt.Errorf("link local account: %w", err)
	}
	if err := s.Directory.GrantAccess(ctx, identity); err != nil {
		return fmt.Errorf("grant application access after linking: %w", err)
	}
	return nil
}

func (s Service) valid(identity identitysync.Identity) bool {
	return s.Issuer != "" && identity.Issuer == s.Issuer && strings.TrimSpace(identity.Subject) != ""
}
