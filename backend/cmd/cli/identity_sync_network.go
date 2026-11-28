package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/spf13/cobra"
)

// newIdentitySyncCheckNetworkCommand checks, from where the backend runs, whether
// registered applications' identity-sync endpoints can be reached: DNS, the
// private-address policy, TCP, TLS and an unauthenticated HTTP request. It
// sends no token and needs no sync credentials.
func newIdentitySyncCheckNetworkCommand() *cobra.Command {
	var app, origin string
	cmd := &cobra.Command{
		Use:   "check-network",
		Short: "Check that applications' identity-sync endpoints are reachable from this server",
		Long: "Checks each step the dashboard needs to reach an application's identity-sync endpoint:\n" +
			"DNS, the private-address policy (" + identityreconcile.AllowedPrivateHostsEnv + "), a TCP\n" +
			"connection, the TLS handshake and an HTTP request without a token (401 is the expected answer).\n" +
			"Run it inside the backend container so it uses the same DNS and network as the dashboard.\n\n" +
			"With --app it checks that registered application, with --origin any HTTPS origin (no database\n" +
			"needed), and with neither every registered application. It exits non-zero when any check fails.",
		Example: "  moh-sso identity-sync check-network\n" +
			"  moh-sso identity-sync check-network --app clims\n" +
			"  moh-sso identity-sync check-network --origin https://alerts.health.go.ug",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if app != "" && origin != "" {
				return errors.New("use --app or --origin, not both")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			var reports []identityreconcile.NetworkReport
			if origin != "" {
				reports = []identityreconcile.NetworkReport{identityreconcile.NewRegistryService(nil, "").CheckNetwork(ctx, origin)}
			} else {
				rt, err := openIdentitySyncRuntime(cmd.Context())
				if err != nil {
					return err
				}
				defer rt.Close()
				if reports, err = rt.Registry.CheckIntegrationsNetwork(ctx, app); err != nil {
					return err
				}
			}
			if err := writeJSON(cmd, reports); err != nil {
				return err
			}
			failed := 0
			for _, report := range reports {
				if !report.OK {
					failed++
				}
			}
			if failed > 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("%d of %d network checks failed", failed, len(reports))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Registered application client ID (default: every registered application)")
	cmd.Flags().StringVar(&origin, "origin", "", "Any HTTPS origin to check instead of a registered application")
	return cmd
}
