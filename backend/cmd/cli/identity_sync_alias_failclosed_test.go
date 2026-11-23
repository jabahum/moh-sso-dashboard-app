package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
)

// The deprecated aliases fall back to <APP>_SYNC_* only when no registry is
// configured or the application has no registry row. When the registry is
// configured but cannot be opened, or the application has a row and the
// revocation list cannot be opened, they fail closed instead of running
// without revocation checks.

func clearKeycloakEnv(t *testing.T) {
	t.Helper()
	for _, env := range []string{"KEYCLOAK_BASE_URL", "KEYCLOAK_REALM", "KEYCLOAK_ADMIN_CLIENT_ID", "KEYCLOAK_ADMIN_CLIENT_SECRET"} {
		t.Setenv(env, "")
	}
}

func TestDeprecatedAliasFailsClosedWhenTheRegistryCannotBeOpened(t *testing.T) {
	previous := openIdentitySyncRuntime
	t.Cleanup(func() { openIdentitySyncRuntime = previous })
	openIdentitySyncRuntime = func(context.Context) (*identitySyncRuntime, error) {
		return nil, errors.New("dial tcp 10.0.0.5:5432: connection refused")
	}
	clearKeycloakEnv(t)
	_, _, err := runCommand(t, newCLIMSSyncCommand())
	if err == nil || strings.Contains(err.Error(), "configure the HTTPS Keycloak base URL") || !strings.Contains(err.Error(), "registry") {
		t.Fatalf("err = %v; want a fail-closed registry error, not the environment fallback", err)
	}
}

func TestDeprecatedAliasFailsClosedWhenRevocationsCannotBeOpened(t *testing.T) {
	for name, enabled := range map[string]bool{"enabled row": true, "disabled row": false} {
		t.Run(name, func(t *testing.T) {
			row := alertsIntegration(enabled)
			row.ApplicationClientID = "clims"
			fakeIdentitySyncRuntime(t, map[string]identityreconcile.Integration{"clims": row})
			identitySyncRevocations = func(context.Context, *identitySyncRuntime) (identityreconcile.RevocationChecker, error) {
				return nil, errors.New("revocation store unavailable")
			}
			clearKeycloakEnv(t)
			_, _, err := runCommand(t, newCLIMSSyncCommand())
			if err == nil || !strings.Contains(err.Error(), "revocation") {
				t.Fatalf("err = %v; want a fail-closed revocation error", err)
			}
		})
	}
}
