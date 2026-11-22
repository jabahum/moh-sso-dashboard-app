package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

// identity-sync revoke | restore | revocations (Track C: access removal).

func init() {
	registerIdentitySyncSubcommand(newIdentitySyncRevokeCommand)
	registerIdentitySyncSubcommand(newIdentitySyncRestoreCommand)
	registerIdentitySyncSubcommand(newIdentitySyncRevocationsCommand)
}

// revokeTimeout covers every step, including the connector's retry budget for
// POST /sessions/revoke.
const revokeTimeout = 3 * time.Minute

// openIdentitySyncRevokeService builds the revoke service from the registry
// runtime. Tests replace it.
var openIdentitySyncRevokeService = func(ctx context.Context, rt *identitySyncRuntime) (*identityreconcile.RevokeService, error) {
	kc, err := rt.Keycloak()
	if err != nil {
		return nil, err
	}
	registry, ok := rt.Registry.Repository.(*identityreconcile.PostgresRegistry)
	if !ok || registry.DB == nil {
		return nil, errors.New("identity-sync revocation needs the dashboard database")
	}
	admin := keycloak.NewAdminClient(kc.adminBase, kc.realm, kc.adminID, kc.adminSecret)
	return &identityreconcile.RevokeService{
		Integrations: rt.Registry,
		Sessions:     identityreconcile.RegistrySessionRevokers(rt.Registry),
		Directory:    identityreconcile.KeycloakAccessDirectory{Admin: admin},
		Repository:   &identityreconcile.PostgresRevocations{DB: registry.DB},
		Issuer:       kc.issuer,
	}, nil
}

func withRevokeService(cmd *cobra.Command, run func(context.Context, *identityreconcile.RevokeService) error) error {
	rt, err := openIdentitySyncRuntime(cmd.Context())
	if err != nil {
		return err
	}
	defer rt.Close()
	service, err := openIdentitySyncRevokeService(cmd.Context(), rt)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), revokeTimeout)
	defer cancel()
	return run(ctx, service)
}

// revocationView adds the derived outcome to a revocation for operators.
type revocationView struct {
	identityreconcile.Revocation
	Complete          bool   `json:"complete"`
	SessionsStillOpen bool   `json:"sessionsStillOpen"`
	Note              string `json:"note,omitempty"`
}

func viewRevocation(r identityreconcile.Revocation) revocationView {
	view := revocationView{Revocation: r, Complete: r.Complete(), SessionsStillOpen: r.Active() && r.SessionsStillOpen()}
	switch {
	case !r.Active():
		view.SessionsStillOpen = false
		view.Note = "restored: later syncs may grant access again; access was not re-granted"
	case r.SessionsRevoked == identityreconcile.StepUnsupported:
		view.Note = "the application implements contract 1.0 and cannot end sessions: existing sessions last until their local expiry"
	case !r.Complete():
		view.Note = "incomplete (" + r.LastError + "): run 'identity-sync revocations --resume'"
	}
	return view
}

// reportRevocation prints the revocation and fails when access removal is not
// complete, so a script never mistakes a partial removal for success.
func reportRevocation(cmd *cobra.Command, r identityreconcile.Revocation) error {
	if err := writeJSON(cmd, viewRevocation(r)); err != nil {
		return err
	}
	return revocationOutcome(cmd.ErrOrStderr(), r)
}

func revocationOutcome(w io.Writer, r identityreconcile.Revocation) error {
	fmt.Fprintf(w, "%s access for %s: role %s, consents %s, sessions %s (attempt %d)\n",
		r.ApplicationClientID, r.Subject, r.RoleRemoved, r.ConsentsRevoked, r.SessionsRevoked, r.Attempts)
	switch {
	case !r.Complete():
		return fmt.Errorf("access removal is incomplete (%s); the failed steps are recorded, retry with 'identity-sync revocations --resume --app %s'", r.LastError, r.ApplicationClientID)
	case r.SessionsRevoked == identityreconcile.StepUnsupported:
		return fmt.Errorf("access was removed, but existing %s sessions were NOT ended: the application implements contract 1.0, so they last until their local expiry", r.ApplicationClientID)
	}
	return nil
}

func newIdentitySyncRevokeCommand() *cobra.Command {
	var app, subject, reason, revoker string
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Remove a person's access to one application and end its sessions now",
		Long: "Records the revocation, removes the application's access role in Keycloak, revokes the\n" +
			"person's consents and offline tokens for that application's client only (other applications\n" +
			"and the SSO session are untouched) and calls the application's POST /sessions/revoke.\n" +
			"Every step runs even if another fails; failures are recorded and 'revocations --resume'\n" +
			"retries them. Exits non-zero unless every step finished and sessions were ended.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(reason) == "" || strings.TrimSpace(revoker) == "" {
				return errors.New("--reason and --revoker are required")
			}
			return withRevokeService(cmd, func(ctx context.Context, service *identityreconcile.RevokeService) error {
				revocation, err := service.Revoke(ctx, app, subject, revoker, reason)
				if err != nil {
					return err
				}
				return reportRevocation(cmd, revocation)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&app, "app", "", "Application Keycloak client ID (required)")
	f.StringVar(&subject, "subject", "", "Keycloak user ID of the person (required)")
	f.StringVar(&reason, "reason", "", "Why access is removed, recorded in the audit log (required)")
	f.StringVar(&revoker, "revoker", "", "Who is removing access, recorded in the audit log (required)")
	for _, name := range []string{"app", "subject", "reason", "revoker"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}

func newIdentitySyncRestoreCommand() *cobra.Command {
	var app, subject, reason, restorer string
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Clear a person's active revocation for one application (does not grant access)",
		Long: "Ends the active revocation so later reviewed syncs may grant access again. It does not\n" +
			"grant the access role or re-link anything.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(reason) == "" || strings.TrimSpace(restorer) == "" {
				return errors.New("--reason and --restorer are required")
			}
			return withRevokeService(cmd, func(ctx context.Context, service *identityreconcile.RevokeService) error {
				revocation, err := service.Restore(ctx, app, subject, restorer, reason)
				if err != nil {
					return err
				}
				if err := writeJSON(cmd, viewRevocation(revocation)); err != nil {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(), app+" revocation restored; access was not re-granted")
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&app, "app", "", "Application Keycloak client ID (required)")
	f.StringVar(&subject, "subject", "", "Keycloak user ID of the person (required)")
	f.StringVar(&reason, "reason", "", "Why the revocation is cleared, recorded in the audit log (required)")
	f.StringVar(&restorer, "restorer", "", "Who is clearing it, recorded in the audit log (required)")
	for _, name := range []string{"app", "subject", "reason", "restorer"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}

func newIdentitySyncRevocationsCommand() *cobra.Command {
	var app, actor string
	var includeRestored, unfinished, resume bool
	var limit int
	cmd := &cobra.Command{
		Use:   "revocations",
		Short: "List revocations and their step status; --resume retries failed steps",
		Long: "Lists active revocations (add --all for restored ones) with each step's status: pending,\n" +
			"done, failed or unsupported. --resume retries every pending or failed step of active\n" +
			"revocations and exits non-zero if any is still incomplete or cannot end sessions.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if resume && includeRestored {
				return errors.New("--resume works on active revocations only; drop --all")
			}
			return withRevokeService(cmd, func(ctx context.Context, service *identityreconcile.RevokeService) error {
				var revocations []identityreconcile.Revocation
				var err error
				if resume {
					revocations, err = service.Resume(ctx, app, actor)
				} else {
					revocations, err = service.List(ctx, identityreconcile.RevocationFilter{
						Application: app, IncludeRestored: includeRestored, OnlyUnfinished: unfinished, Limit: limit,
					})
				}
				if err != nil {
					return err
				}
				views := make([]revocationView, 0, len(revocations))
				for _, revocation := range revocations {
					views = append(views, viewRevocation(revocation))
				}
				if err := writeJSON(cmd, views); err != nil {
					return err
				}
				if !resume {
					return nil
				}
				var failed int
				for _, revocation := range revocations {
					if revocationOutcome(cmd.ErrOrStderr(), revocation) != nil {
						failed++
					}
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "resumed %d revocation(s)\n", len(revocations))
				if failed > 0 {
					return fmt.Errorf("%d revocation(s) are still incomplete or cannot end sessions", failed)
				}
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&app, "app", "", "Only this application")
	f.BoolVar(&includeRestored, "all", false, "Include restored revocations")
	f.BoolVar(&unfinished, "unfinished", false, "Only active revocations with a pending or failed step")
	f.IntVar(&limit, "limit", 100, "Maximum rows to list (1-1000)")
	f.BoolVar(&resume, "resume", false, "Retry pending and failed steps of active revocations")
	f.StringVar(&actor, "actor", identitySyncActor(), "Who runs the resume, recorded in the audit log")
	return cmd
}
