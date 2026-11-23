// Package memory is an in-memory adapter.Store and adapter.SessionProbe for
// tests and local experiments. It follows every Store rule of the
// identity-sync contract: eligibility, stable ascending order, idempotent
// linking that never replaces a link, and immediate session revocation.
package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter"
)

type account struct {
	email    string
	eligible bool
	subject  string // linked subject for the store's issuer, "" when unlinked
}

// Store holds synthetic accounts for one application and issuer.
type Store struct {
	issuer string
	format identitysync.AccountIDFormat

	mu        sync.Mutex
	accounts  map[string]*account
	bySubject map[string]string // subject -> account ID
	sessions  map[string]string // session token -> account ID
}

var (
	_ adapter.Store        = (*Store)(nil)
	_ adapter.SessionProbe = (*Store)(nil)
)

// New returns an empty store for issuer and the account ID format.
func New(issuer string, format identitysync.AccountIDFormat) *Store {
	return &Store{
		issuer:    issuer,
		format:    format,
		accounts:  map[string]*account{},
		bySubject: map[string]string{},
		sessions:  map[string]string{},
	}
}

// Add creates or replaces a synthetic account. It panics on an ID that does not
// match the store's format, because that is a test bug.
func (s *Store) Add(id, email string, eligible bool) {
	if !identitysync.ValidAccountID(s.format, id) {
		panic("memory: invalid account ID " + id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.accounts[id]; ok && old.subject != "" {
		delete(s.bySubject, old.subject)
	}
	s.accounts[id] = &account{email: email, eligible: eligible}
}

// Unlink removes an account's link and ends its sessions. It models the
// application's manual, audited relink operation and is not part of the contract.
func (s *Store) Unlink(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.accounts[id]; ok && a.subject != "" {
		delete(s.bySubject, a.subject)
		a.subject = ""
		s.endSessionsLocked(id)
	}
}

// Link returns the subject linked to an account, or "".
func (s *Store) Link(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.accounts[id]; ok {
		return a.subject
	}
	return ""
}

// ListEligibleAccounts implements adapter.Store. The cursor is the last ID of
// the previous page.
func (s *Store) ListEligibleAccounts(_ context.Context, cursor string, limit int) (identitysync.AccountPage, error) {
	if cursor != "" && !identitysync.ValidAccountID(s.format, cursor) {
		return identitysync.AccountPage{}, adapter.ErrInvalidCursor
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.accounts))
	for id, a := range s.accounts {
		if a.eligible && (cursor == "" || adapter.AccountIDLess(s.format, cursor, id)) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return adapter.AccountIDLess(s.format, ids[i], ids[j]) })
	page := identitysync.AccountPage{Accounts: []identitysync.Account{}}
	for _, id := range ids {
		if len(page.Accounts) == limit {
			page.NextCursor = page.Accounts[limit-1].ID
			break
		}
		a := s.accounts[id]
		account := identitysync.Account{ID: id, Email: a.email}
		if a.subject != "" {
			account.Link = identitysync.Identity{Issuer: s.issuer, Subject: a.subject}
		}
		page.Accounts = append(page.Accounts, account)
	}
	return page, nil
}

// LinkIdentity implements adapter.Store.
func (s *Store) LinkIdentity(_ context.Context, accountID string, identity identitysync.Identity) error {
	if identity.Issuer != s.issuer {
		return identitysync.ErrAccountNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[accountID]
	switch {
	case !ok || !a.eligible:
		return identitysync.ErrAccountNotFound
	case a.subject == identity.Subject:
		return nil
	case a.subject != "":
		return identitysync.ErrLinkConflict
	}
	if _, taken := s.bySubject[identity.Subject]; taken {
		return identitysync.ErrLinkConflict
	}
	a.subject = identity.Subject
	s.bySubject[identity.Subject] = accountID
	return nil
}

// RevokeSessions implements adapter.Store.
func (s *Store) RevokeSessions(_ context.Context, identity identitysync.Identity) error {
	if identity.Issuer != s.issuer {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.bySubject[identity.Subject]; ok {
		s.endSessionsLocked(id)
	}
	return nil
}

// CreateTestSession implements adapter.SessionProbe.
func (s *Store) CreateTestSession(_ context.Context, identity identitysync.Identity) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.bySubject[identity.Subject]
	if identity.Issuer != s.issuer || !ok {
		return "", identitysync.ErrIdentityNotLinked
	}
	if !s.accounts[id].eligible {
		return "", identitysync.ErrAccountNotFound
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	s.sessions[token] = id
	return token, nil
}

// CheckTestSession implements adapter.SessionProbe.
func (s *Store) CheckTestSession(_ context.Context, token string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.sessions[token]
	return ok && s.accounts[id].eligible, nil
}

// Sessions returns the number of live sessions of an account.
func (s *Store) Sessions(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, owner := range s.sessions {
		if owner == id {
			n++
		}
	}
	return n
}

func (s *Store) endSessionsLocked(id string) {
	for token, owner := range s.sessions {
		if owner == id {
			delete(s.sessions, token)
		}
	}
}
