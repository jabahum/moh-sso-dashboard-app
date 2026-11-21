package identityreconcile

import (
	"context"
	"errors"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

type fixture struct {
	identitysync.AccountStore
	account  identitysync.Account
	identity identitysync.Identity
	linked   bool
	created  bool
	grants   int
}

func (f *fixture) ListEligibleAccounts(context.Context, string, int) (identitysync.AccountPage, error) {
	return identitysync.AccountPage{Accounts: []identitysync.Account{f.account}}, nil
}

func (f *fixture) FindEmail(context.Context, string) ([]identitysync.Identity, error) {
	if f.created {
		return []identitysync.Identity{f.identity}, nil
	}
	return nil, nil
}

func (f *fixture) EnsureUser(context.Context, identitysync.Account) (identitysync.Identity, error) {
	f.created = true
	return f.identity, nil
}

func (f *fixture) LinkIdentity(context.Context, string, identitysync.Identity) error {
	f.linked = true
	return nil
}

func (f *fixture) GrantAccess(context.Context, identitysync.Identity) error {
	if !f.linked {
		return errors.New("grant attempted before link")
	}
	f.grants++
	if f.grants == 1 {
		return errors.New("temporary grant failure")
	}
	return nil
}

func TestApplyResumesAfterGrantFailure(t *testing.T) {
	ctx := context.Background()
	f := &fixture{
		account:  identitysync.Account{ID: "account", Email: "account@example.org"},
		identity: identitysync.Identity{Issuer: "https://issuer.example/realm", Subject: "subject"},
	}
	s := Service{Accounts: f, Directory: f, Issuer: f.identity.Issuer}
	preview, err := s.Preview(ctx, "", 1)
	if err != nil || len(preview.Decisions) != 1 || preview.Decisions[0].Action != Create || f.created || f.linked {
		t.Fatalf("expected read-only creation preview, got %+v, %v", preview, err)
	}
	decision := preview.Decisions[0]
	if err := s.Apply(ctx, decision, f.account); err == nil || f.grants != 1 {
		t.Fatalf("expected first grant attempt to fail, got %v", err)
	}
	if err := s.Apply(ctx, decision, f.account); err != nil || f.grants != 2 {
		t.Fatalf("expected retry to complete the grant, got %v", err)
	}
}
