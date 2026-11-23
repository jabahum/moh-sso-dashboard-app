package identityreconcile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// Single-account apply must never grant an account that is already linked for
// the configured issuer (the batch's linked_no_access rule), and preview must
// show revoked identities as revoked instead of proposing reuse.

func TestSingleApplyRefusesAlreadyLinkedAccountWithoutAccess(t *testing.T) {
	ctx := context.Background()
	identity := identitysync.Identity{Issuer: batchIssuer, Subject: "subj-linked"}
	roles := newRoleState() // the person does not hold the access role
	accounts := &linkRecorder{account: identitysync.Account{ID: "acct-1", Email: "linked@example.org", Link: identity}}
	service := Service{
		Accounts:    accounts,
		Directory:   &interleavingServiceDirectory{identity: identity, roles: roles},
		Issuer:      batchIssuer,
		Application: "alerts",
	}
	decision := Decision{Account: accounts.account, Action: Reuse, Identity: identity}
	err := service.Apply(ctx, decision, accounts.account)
	if err == nil {
		t.Fatal("single apply granted access to an account that is already linked without access")
	}
	if !errors.Is(err, ErrAccountAlreadyLinked) || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("err = %v, want ErrAccountAlreadyLinked telling the operator to restore explicitly", err)
	}
	if roles.has(identity.Subject) || accounts.links != 0 {
		t.Fatalf("granted=%v links=%d; a linked account must be left alone", roles.has(identity.Subject), accounts.links)
	}
}

func TestPreviewMarksRevokedAndLinkedAccounts(t *testing.T) {
	ctx := context.Background()
	matched := identitysync.Identity{Issuer: batchIssuer, Subject: "subj-match"}
	revs := &fakeRevocations{subject: map[string]bool{"subj-match": true, "subj-linked-revoked": true}}
	page := []identitysync.Account{
		{ID: "a-revoked-match", Email: "match@example.org"},
		{ID: "b-linked", Email: "linked@example.org", Link: identitysync.Identity{Issuer: batchIssuer, Subject: "subj-linked"}},
		{ID: "c-linked-revoked", Email: "lr@example.org", Link: identitysync.Identity{Issuer: batchIssuer, Subject: "subj-linked-revoked"}},
	}
	service := Service{
		Accounts:    pageAccounts(page),
		Directory:   emailDirectory{"match@example.org": {matched}},
		Issuer:      batchIssuer,
		Application: "alerts",
		Revocations: revs,
	}
	preview, err := service.Preview(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Decision{}
	for _, d := range preview.Decisions {
		got[d.Account.ID] = d
	}
	if d := got["a-revoked-match"]; d.Action != Revoked || d.Identity != matched {
		t.Errorf("revoked match = %+v, want action revoked (never reuse)", d)
	}
	if d := got["b-linked"]; d.Action != Linked || d.Identity.Subject != "subj-linked" {
		t.Errorf("linked account = %+v, want action linked", d)
	}
	if d := got["c-linked-revoked"]; d.Action != Revoked {
		t.Errorf("linked revoked account = %+v, want action revoked", d)
	}
	// A failing revocation check stops the preview instead of proposing reuse.
	revs.err = errors.New("database down")
	if _, err := service.Preview(ctx, "", 10); err == nil {
		t.Fatal("preview proposed matches although the revocation list could not be read")
	}
	// A revoked decision can never be applied.
	revs.err = nil
	if err := service.Apply(ctx, got["a-revoked-match"], page[0]); err == nil {
		t.Fatal("a revoked preview decision was applied")
	}
}

type pageAccounts []identitysync.Account

func (p pageAccounts) ListEligibleAccounts(context.Context, string, int) (identitysync.AccountPage, error) {
	return identitysync.AccountPage{Accounts: append([]identitysync.Account(nil), p...)}, nil
}
func (pageAccounts) LinkIdentity(context.Context, string, identitysync.Identity) error {
	return errors.New("preview must not link")
}

type emailDirectory map[string][]identitysync.Identity

func (d emailDirectory) FindEmail(_ context.Context, email string) ([]identitysync.Identity, error) {
	return d[email], nil
}
func (emailDirectory) EnsureUser(context.Context, identitysync.Account) (identitysync.Identity, error) {
	return identitysync.Identity{}, errors.New("preview must not create users")
}
func (emailDirectory) GrantAccess(context.Context, identitysync.Identity) error {
	return errors.New("preview must not grant")
}
