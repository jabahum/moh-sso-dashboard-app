package identityreconcile

import (
	"context"
	"errors"
	"testing"

	"github.com/moh-sso-dashboard/internal/accesswarnings"
)

// The portal hook's error is typed, so the portal can show a warning code.
func TestPortalHookErrorCarriesAWarningCode(t *testing.T) {
	f := newRevokeFixture(t)
	f.portalRemovedAccessRole()
	f.app.set(func(a *fakeApp) { a.outage = true })
	err := NewPortalRevocationHook(f.service).ClientRolesRemoved(context.Background(), revokeSubject, "alerts", []string{"alerts_access"}, "admin-1")
	var incomplete *IncompleteRevocationError
	if !errors.As(err, &incomplete) || incomplete.WarningCode() != accesswarnings.CodeRevocationIncomplete ||
		incomplete.Revocation.SessionsRevoked != StepFailed {
		t.Fatalf("err = %#v", err)
	}

	g := newRevokeFixture(t)
	g.portalRemovedAccessRole()
	g.registeredAs10()
	err = NewPortalRevocationHook(g.service).ClientRolesRemoved(context.Background(), revokeSubject, "alerts", []string{"alerts_access"}, "admin-1")
	if !errors.As(err, &incomplete) || incomplete.WarningCode() != accesswarnings.CodeSessionsNotEnded {
		t.Fatalf("1.0 err = %#v", err)
	}
}

// A portal admin re-granting the access role to a person with an active
// revocation records an explicit restore by that admin.
func TestPortalReGrantRestoresActiveRevocation(t *testing.T) {
	ctx := context.Background()
	f := newRevokeFixture(t)
	if _, err := f.service.Revoke(ctx, "alerts", revokeSubject, "ops@example.test", "left"); err != nil {
		t.Fatal(err)
	}
	hook := NewPortalRevocationHook(f.service)
	// Another role: the revocation stays.
	if err := hook.ClientRolesAdded(ctx, revokeSubject, "alerts", []string{"alerts_viewer"}, "admin-2"); err != nil {
		t.Fatal(err)
	}
	if revoked, _ := f.service.IsRevoked(ctx, "alerts", revokeIssuer, revokeSubject); !revoked {
		t.Fatal("granting another role restored the revocation")
	}
	if err := hook.ClientRolesAdded(ctx, revokeSubject, "alerts", []string{"alerts_viewer", "alerts_access"}, "admin-2"); err != nil {
		t.Fatal(err)
	}
	if revoked, _ := f.service.IsRevoked(ctx, "alerts", revokeIssuer, revokeSubject); revoked {
		t.Fatal("the revocation list still says revoked after a portal re-grant")
	}
	row := f.repo.rows[0]
	if row.RestoredBy != "portal:admin-2" || row.RestoreReason != "portal re-grant" || row.RestoredAt == nil {
		t.Fatalf("restore = %+v", row)
	}
	if f.repo.audit[len(f.repo.audit)-1] != AuditRevocationRestored {
		t.Fatalf("audit = %v", f.repo.audit)
	}
	// No active revocation, unregistered application, nil hook: no-ops.
	if err := hook.ClientRolesAdded(ctx, revokeSubject, "alerts", []string{"alerts_access"}, "admin-2"); err != nil {
		t.Fatal(err)
	}
	if err := hook.ClientRolesAdded(ctx, revokeSubject, "clims", []string{"clims_access"}, "admin-2"); err != nil {
		t.Fatal(err)
	}
	var nilHook *PortalRevocationHook
	if err := nilHook.ClientRolesAdded(ctx, revokeSubject, "alerts", []string{"alerts_access"}, ""); err != nil {
		t.Fatal(err)
	}
}
