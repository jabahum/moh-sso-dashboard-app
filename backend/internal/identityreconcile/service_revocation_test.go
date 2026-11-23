package identityreconcile

import (
	"context"
	"errors"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// scriptedRevocations answers IsRevoked from a list of results, one per call.
type scriptedRevocations struct {
	results []bool
	err     error
	calls   []string
}

func (r *scriptedRevocations) IsRevoked(_ context.Context, app, issuer, subject string) (bool, error) {
	r.calls = append(r.calls, app+"|"+issuer+"|"+subject)
	if r.err != nil {
		return false, r.err
	}
	if len(r.calls) > len(r.results) {
		return false, nil
	}
	return r.results[len(r.calls)-1], nil
}

func revocationFixture(t *testing.T) (*fixture, Service, Decision) {
	t.Helper()
	f := &fixture{
		account:  identitysync.Account{ID: "account", Email: "account@example.org"},
		identity: identitysync.Identity{Issuer: "https://issuer.example/realm", Subject: "subject"},
		grants:   1, // the fixture's first grant fails; start past it
	}
	s := Service{Accounts: f, Directory: f, Issuer: f.identity.Issuer, Application: "alerts"}
	preview, err := s.Preview(context.Background(), "", 1)
	if err != nil || len(preview.Decisions) != 1 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	return f, s, preview.Decisions[0]
}

func TestApplyRefusesRevokedIdentityBeforeLink(t *testing.T) {
	f, s, decision := revocationFixture(t)
	revs := &scriptedRevocations{results: []bool{true}}
	s.Revocations = revs
	err := s.Apply(context.Background(), decision, f.account)
	if !errors.Is(err, ErrIdentityRevoked) || f.linked || f.grants != 1 {
		t.Fatalf("expected refusal before link, got err=%v linked=%v grants=%d", err, f.linked, f.grants)
	}
	if len(revs.calls) != 1 || revs.calls[0] != "alerts|https://issuer.example/realm|subject" {
		t.Fatalf("unexpected revocation lookups %v", revs.calls)
	}
}

func TestApplyRefusesGrantWhenRevokedAfterLink(t *testing.T) {
	f, s, decision := revocationFixture(t)
	s.Revocations = &scriptedRevocations{results: []bool{false, true}}
	err := s.Apply(context.Background(), decision, f.account)
	if !errors.Is(err, ErrIdentityRevoked) || !f.linked || f.grants != 1 {
		t.Fatalf("expected link but no grant, got err=%v linked=%v grants=%d", err, f.linked, f.grants)
	}
}

func TestApplyFailsClosedWhenRevocationCheckFails(t *testing.T) {
	f, s, decision := revocationFixture(t)
	s.Revocations = &scriptedRevocations{err: errors.New("database down")}
	if err := s.Apply(context.Background(), decision, f.account); err == nil || f.linked || f.grants != 1 {
		t.Fatalf("expected failure before link, got err=%v linked=%v", err, f.linked)
	}
	s.Revocations, s.Application = &scriptedRevocations{}, ""
	if err := s.Apply(context.Background(), decision, f.account); err == nil || f.linked {
		t.Fatalf("expected failure without an application, got err=%v linked=%v", err, f.linked)
	}
}

func TestApplyGrantsWhenNotRevoked(t *testing.T) {
	f, s, decision := revocationFixture(t)
	revs := &scriptedRevocations{}
	s.Revocations = revs
	if err := s.Apply(context.Background(), decision, f.account); err != nil || !f.linked || f.grants != 2 {
		t.Fatalf("expected link and grant, got err=%v linked=%v grants=%d", err, f.linked, f.grants)
	}
	if len(revs.calls) != 2 {
		t.Fatalf("expected a check before link and before grant, got %v", revs.calls)
	}
}
