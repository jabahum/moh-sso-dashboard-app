package client_test

import (
	"context"
	"errors"
	"log"
	"os"

	platform "github.com/mohuganda/moh-sso-dashboard/backend/core/platform/client"
)

// A backend that sends notifications through the portal and checks a user's
// access before a sensitive operation.
func Example() {
	portal, err := platform.New(platform.Config{
		PortalURL: "https://portals.health.go.ug",
		Tokens: &platform.ClientCredentials{
			TokenURL:     "https://auth.health.go.ug/realms/MoH/protocol/openid-connect/token",
			ClientID:     "alerts-platform",
			ClientSecret: os.Getenv("ALERTS_PLATFORM_SECRET"),
			Scopes:       []string{"identity:read", "notifications:write", "notifications:read"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// At startup: check the binding and scopes end to end.
	if id, err := portal.Identity(ctx); err != nil || !id.HasScope("notifications:write") {
		log.Printf("platform integration not ready: %v", err)
	}

	// The key makes retries safe: the portal sends each notification once.
	status, err := portal.Notify(ctx, "alert-2026-0042:dho", platform.Notification{
		Title:     "New alert in Gulu",
		Message:   "A suspected case was reported.",
		Recipient: platform.Recipient{Email: "dho@example.org", Name: "District Health Officer"},
	})
	switch {
	case errors.Is(err, platform.ErrRateLimited):
		// Try again later.
	case err != nil:
		log.Printf("notification not queued: %v", err)
	default:
		log.Printf("notification %s is %s", status.ID, status.Status)
	}

	// Before a sensitive operation, with the signed-in user's access token.
	userToken := "<the user's access token>"
	access, err := portal.Access(ctx, userToken, "alerts")
	if err != nil || !access.HasPermission("alerts:close") {
		// Fail closed: refuse when the portal can't confirm access.
		return
	}
}
