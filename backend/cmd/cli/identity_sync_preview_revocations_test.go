package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
)

// "identity-sync preview" must use the revocation list (so revoked identities
// are shown as revoked, not proposed for reuse) and fail closed without it.
func TestIdentitySyncPreviewConsultsTheRevocationList(t *testing.T) {
	fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"alerts": alertsIntegration(true)})
	consulted := false
	identitySyncRevocations = func(context.Context, *identitySyncRuntime) (identityreconcile.RevocationChecker, error) {
		consulted = true
		return nil, errors.New("revocation store unavailable")
	}
	_, _, err := runCommand(t, newIdentitySyncCommand(), "preview", "--app", "alerts")
	if !consulted {
		t.Fatal("preview did not open the revocation list")
	}
	if err == nil || !strings.Contains(err.Error(), "revocation") {
		t.Fatalf("preview without the revocation list: err = %v, want a fail-closed error", err)
	}
}
