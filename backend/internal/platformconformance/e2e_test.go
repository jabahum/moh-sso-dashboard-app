package platformconformance

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	platform "github.com/mohuganda/moh-sso-dashboard/backend/core/platform/client"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/platform/conformance"
)

// Values from infra/compose/platform-conformance (fake CI fixtures).
const (
	application = "conformance-app"
	boundClient = "conformance-bound"
)

func clientToken(keycloak, realm, id, secret string, scopes ...string) conformance.TokenSource {
	return conformance.FromClient(&platform.ClientCredentials{
		TokenURL: keycloak + "/realms/" + realm + "/protocol/openid-connect/token",
		ClientID: id, ClientSecret: secret, Scopes: scopes,
	})
}

func TestPlatformConformanceE2E(t *testing.T) {
	portal := os.Getenv("PLATFORM_CONFORMANCE_PORTAL")
	if portal == "" {
		t.Skip("PLATFORM_CONFORMANCE_PORTAL is not set; run infra/scripts/test-platform-conformance.sh")
	}
	keycloak := os.Getenv("PLATFORM_CONFORMANCE_KEYCLOAK")
	db, err := sql.Open("postgres", os.Getenv("PLATFORM_CONFORMANCE_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("database: %v", err)
	}

	scopes := []string{"identity:read", "notifications:read", "notifications:write"}
	report, err := conformance.Run(ctx, conformance.Config{
		PortalURL: portal,
		AllowHTTP: true,
		Tokens: conformance.Tokens{
			Bound:         clientToken(keycloak, "moh-test", boundClient, "ci-fake-bound-secret", scopes...),
			Narrow:        clientToken(keycloak, "moh-test", "conformance-narrow", "ci-fake-narrow-secret", "identity:read", "notifications:read"),
			Unbound:       clientToken(keycloak, "moh-test", "conformance-unbound", "ci-fake-unbound-secret", scopes...),
			WrongAudience: clientToken(keycloak, "moh-test", "conformance-noaudience", "ci-fake-noaudience-secret", scopes...),
			WrongIssuer:   clientToken(keycloak, "conformance-other", "conformance-other-issuer", "ci-fake-other-secret", scopes...),
		},
		Expect: conformance.Expect{Application: application, ClientID: boundClient, Scopes: scopes},
		Controls: conformance.Controls{
			SetBindingEnabled: func(ctx context.Context, enabled bool) error {
				_, err := db.ExecContext(ctx, `UPDATE ihp_application_identities SET enabled = $1, updated_at = now() WHERE oauth_client_id = $2`, enabled, boundClient)
				return err
			},
			SetApplicationActive: func(ctx context.Context, active bool) error {
				status := "suspended"
				if active {
					status = "active"
				}
				_, err := db.ExecContext(ctx, `UPDATE ihp_systems SET lifecycle_status = $1, enabled = $2, updated_at = now() WHERE client_id = $3`, status, active, application)
				return err
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, res := range report.Results {
		t.Logf("%-4s %-40s %s", res.Status, res.ID, res.Detail)
	}
	pass, fail, skip := report.Counts()
	t.Logf("%d passed, %d failed, %d skipped", pass, fail, skip)
	if report.Failed() {
		t.Fatal("platform API conformance failed")
	}
	// The core profile serves no notifications and the stack has no user token;
	// everything else must actually run.
	for _, res := range report.Results {
		switch res.ID {
		case conformance.IDAccessUser, conformance.IDNotifyValidation, conformance.IDNotifySubmit,
			conformance.IDNotifyConflict, conformance.IDNotifyStatus, conformance.IDNotifyOther, conformance.IDNarrowRefused:
		default:
			if res.Status != conformance.Pass {
				t.Errorf("%s was %s (%s); the CI stack must exercise it", res.ID, res.Status, res.Detail)
			}
		}
	}
}
