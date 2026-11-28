package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	platform "github.com/mohuganda/moh-sso-dashboard/backend/core/platform/client"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/platform/conformance"
	"github.com/spf13/cobra"
)

// newPlatformCommand builds the "platform" parent command.
func newPlatformCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "platform",
		Short: "Check the portal's platform API from an application backend's point of view",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newPlatformConformanceCommand())
	return cmd
}

type platformTokenEnv struct {
	secret, narrow, unbound, wrongAudience, wrongIssuer, user string
}

func newPlatformConformanceCommand() *cobra.Command {
	var (
		portal, tokenURL, clientID, application, scopes, expectScopes, recipient, caFile string
		asJSON, allowHTTP                                                                bool
		timeout                                                                          time.Duration
		env                                                                              platformTokenEnv
	)
	cmd := &cobra.Command{
		Use:   "conformance",
		Short: "Run the platform API conformance suite against a portal with your machine client",
		Long: `Runs the platform API conformance checks against a portal and prints one result
per check: authentication rules, your client's identity and effective scopes,
readiness, the access decision, and notifications when the portal serves them.

It obtains a token for --client-id with the client-credentials grant; the
client secret is read from the environment, never a flag. Optional tokens for
the negative checks (another client, wrong audience, wrong issuer, a user) are
also read from environment variables; a check whose token is not set reports
"skip", never "pass". Notifications are sent only with --notification-recipient,
so use a test mailbox. The revocation checks, which disable bindings and
suspend applications, run only in the portal's own CI environment.`,
		Example: `  export PLATFORM_CLIENT_SECRET=...
  moh-sso platform conformance --portal https://portals.health.go.ug \
    --token-url https://auth.health.go.ug/realms/MoH/protocol/openid-connect/token \
    --client-id alerts-platform --application alerts`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if portal == "" || tokenURL == "" || clientID == "" || application == "" {
				return errors.New("--portal, --token-url, --client-id and --application are required")
			}
			secret := os.Getenv(env.secret)
			if secret == "" {
				return fmt.Errorf("set %s to the client secret of %s", env.secret, clientID)
			}
			httpClient, err := conformanceHTTPClient(caFile)
			if err != nil {
				return err
			}
			requested := splitList(scopes)
			expected := requested
			if expectScopes != "" {
				expected = splitList(expectScopes)
			}
			bound := &platform.ClientCredentials{TokenURL: tokenURL, ClientID: clientID, ClientSecret: secret, Scopes: requested, HTTPClient: httpClient}
			tokens := conformance.Tokens{Bound: conformance.FromClient(bound)}
			for name, target := range map[string]*conformance.TokenSource{
				env.narrow: &tokens.Narrow, env.unbound: &tokens.Unbound, env.wrongAudience: &tokens.WrongAudience,
				env.wrongIssuer: &tokens.WrongIssuer, env.user: &tokens.User,
			} {
				if value := os.Getenv(name); name != "" && value != "" {
					*target = conformance.StaticToken(value)
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			report, err := conformance.Run(ctx, conformance.Config{
				PortalURL: portal, Client: httpClient, Tokens: tokens, AllowHTTP: allowHTTP,
				Expect: conformance.Expect{Application: application, ClientID: clientID, Scopes: expected, NotificationRecipient: recipient},
			})
			if err != nil {
				return err
			}
			if err := writePlatformReport(cmd, report, asJSON); err != nil {
				return err
			}
			if report.Failed() {
				cmd.SilenceUsage = true
				_, fail, _ := report.Counts()
				return fmt.Errorf("platform conformance: %d check(s) failed", fail)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&portal, "portal", "", "Portal origin, for example https://portals.health.go.ug")
	f.StringVar(&tokenURL, "token-url", "", "Keycloak token endpoint of the portal's realm")
	f.StringVar(&clientID, "client-id", "", "Your machine client, bound to --application in the portal")
	f.StringVar(&application, "application", "", "The registered application your client is bound to")
	f.StringVar(&scopes, "scopes", "identity:read,notifications:read,notifications:write", "Scopes to request (comma-separated)")
	f.StringVar(&expectScopes, "expect-scopes", "", "Effective scopes the portal should report (default: --scopes)")
	f.StringVar(&recipient, "notification-recipient", "", "Test mailbox for the notification checks (default: no notification is sent)")
	f.StringVar(&caFile, "ca-file", "", "PEM CA bundle to trust in addition to the system roots")
	f.BoolVar(&asJSON, "json", false, "Print the report as JSON")
	f.BoolVar(&allowHTTP, "allow-http", false, "Accept an http:// portal URL for local runs")
	f.DurationVar(&timeout, "timeout", 2*time.Minute, "Overall time limit")
	f.StringVar(&env.secret, "client-secret-env", "PLATFORM_CLIENT_SECRET", "Env var holding --client-id's secret")
	f.StringVar(&env.narrow, "narrow-token-env", "PLATFORM_NARROW_TOKEN", "Env var holding a bound client's token without notifications:write")
	f.StringVar(&env.unbound, "unbound-token-env", "PLATFORM_UNBOUND_TOKEN", "Env var holding a platform-audience token of a client with no binding")
	f.StringVar(&env.wrongAudience, "wrong-audience-token-env", "PLATFORM_WRONG_AUDIENCE_TOKEN", "Env var holding a realm token without the platform audience")
	f.StringVar(&env.wrongIssuer, "wrong-issuer-token-env", "PLATFORM_WRONG_ISSUER_TOKEN", "Env var holding a token from another realm")
	f.StringVar(&env.user, "user-token-env", "PLATFORM_USER_TOKEN", "Env var holding a user's access token for --application")
	return cmd
}

func splitList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func writePlatformReport(cmd *cobra.Command, report conformance.Report, asJSON bool) error {
	out := cmd.OutOrStdout()
	if asJSON {
		return writeJSON(cmd, report)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range report.Results {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", strings.ToUpper(string(r.Status)), r.ID, r.Detail)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	pass, fail, skip := report.Counts()
	_, err := fmt.Fprintf(out, "\n%d passed, %d failed, %d skipped against %s\n", pass, fail, skip, report.PortalURL)
	return err
}
