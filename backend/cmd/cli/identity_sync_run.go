package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

// "identity-sync run": reviewed batch sync (Track B). Every subcommand calls
// identityreconcile.BatchService, which an admin page can reuse later.

func init() { registerIdentitySyncSubcommand(newIdentitySyncRunCommand) }

// openIdentitySyncBatchRepository opens the run store (migration 63). Tests replace it.
var openIdentitySyncBatchRepository = func(ctx context.Context) (identityreconcile.BatchRepository, func(), error) {
	db, err := openIdentitySyncDB(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("open the dashboard database: %w", err)
	}
	return &identityreconcile.PostgresBatchRepository{DB: db}, func() { db.Close() }, nil
}

// identitySyncRevocations supplies the revocation checker used for proposals
// and before every link and grant. identity_sync_wiring.go replaces this
// default with Track C's revocation list (RevokeService.IsRevoked).
var identitySyncRevocations = func(ctx context.Context, rt *identitySyncRuntime) (identityreconcile.RevocationChecker, error) {
	return identityreconcile.NoRevocations{}, nil
}

// identitySyncBatchTarget builds the connector and Keycloak directory for an
// enabled application. Tests replace it.
var identitySyncBatchTarget = func(ctx context.Context, rt *identitySyncRuntime, app string) (identityreconcile.BatchTarget, error) {
	kc, err := rt.Keycloak()
	if err != nil {
		return identityreconcile.BatchTarget{}, err
	}
	connector, integration, err := rt.Connector(ctx, app)
	if err != nil {
		return identityreconcile.BatchTarget{}, err
	}
	directory := identityreconcile.KeycloakDirectory{
		Admin:       keycloak.NewAdminClient(kc.adminBase, kc.realm, kc.adminID, kc.adminSecret),
		Issuer:      kc.issuer,
		Application: integration.ApplicationClientID,
		AccessRole:  integration.AccessRole,
	}
	return identityreconcile.BatchTarget{Application: integration.ApplicationClientID, Issuer: kc.issuer, Accounts: connector, Directory: directory}, nil
}

// batchSession bundles what the run subcommands need. rt is nil for
// subcommands that only read or record decisions.
type batchSession struct {
	rt      *identitySyncRuntime
	service identityreconcile.BatchService
	close   func()
}

func openBatchSession(ctx context.Context, withRuntime bool) (*batchSession, error) {
	s := &batchSession{close: func() {}}
	var closers []func()
	s.close = func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	repo, closeRepo, err := openIdentitySyncBatchRepository(ctx)
	if err != nil {
		return nil, err
	}
	closers = append(closers, closeRepo)
	s.service = identityreconcile.BatchService{Repo: repo}
	if withRuntime {
		rt, err := openIdentitySyncRuntime(ctx)
		if err != nil {
			s.close()
			return nil, err
		}
		closers = append(closers, rt.Close)
		s.rt = rt
		revocations, err := identitySyncRevocations(ctx, rt)
		if err != nil {
			s.close()
			return nil, fmt.Errorf("open the revocation list: %w", err)
		}
		s.service.Revocations = revocations
	}
	return s, nil
}

func newIdentitySyncRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Reviewed batch sync: start a run, review its rows, then apply or resume it",
		Long: "A run walks every eligible account of a registered application and proposes, per account:\n" +
			"reuse (one verified Keycloak user matches), create (invite a new user), review (ambiguous,\n" +
			"unverified or no email), linked, linked_no_access (never granted here) or revoked.\n" +
			"Only reuse and create rows can be approved, always by a named reviewer. apply processes\n" +
			"approved rows once; resume retries rows that did not finish (an already linked row only\n" +
			"has its access grant retried).",
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newRunStartCommand(), newRunShowCommand(), newRunDecideCommand(true), newRunDecideCommand(false),
		newRunApplyCommand(false), newRunApplyCommand(true))
	return cmd
}

func newRunStartCommand() *cobra.Command {
	var app string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Read all eligible accounts and store a proposal for each (read-only on both sides)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
			defer cancel()
			s, err := openBatchSession(ctx, true)
			if err != nil {
				return err
			}
			defer s.close()
			target, err := identitySyncBatchTarget(ctx, s.rt, app)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), app+" identity links use issuer "+target.Issuer)
			run, rows, err := s.service.StartRun(ctx, target, identitySyncActor())
			if err != nil {
				return err
			}
			return writeJSON(cmd, runSummary(run, rows))
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Application Keycloak client ID (required)")
	_ = cmd.MarkFlagRequired("app")
	return cmd
}

type runSummaryView struct {
	Run    identityreconcile.BatchRun `json:"run"`
	Counts map[string]int             `json:"counts"`
}

func runSummary(run identityreconcile.BatchRun, rows []identityreconcile.BatchRow) runSummaryView {
	counts := map[string]int{"total": len(rows)}
	for _, row := range rows {
		counts["proposal:"+string(row.Proposal)]++
		counts["decision:"+string(row.Decision)]++
		if row.Finished() && row.Attempts > 0 {
			counts["applied"]++
		}
		if row.LastError != "" {
			counts["error:"+row.LastError]++
		}
	}
	return runSummaryView{Run: run, Counts: counts}
}

func newRunShowCommand() *cobra.Command {
	var runID, app, proposal, decision string
	var summaryOnly bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show a run and its rows (--run), or list an application's recent runs (--app)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (runID == "") == (app == "") {
				return errors.New("pass exactly one of --run or --app")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			s, err := openBatchSession(ctx, false)
			if err != nil {
				return err
			}
			defer s.close()
			if app != "" {
				runs, err := s.service.ListRuns(ctx, app, 20)
				if err != nil {
					return err
				}
				return writeJSON(cmd, struct {
					Runs []identityreconcile.BatchRun `json:"runs"`
				}{runs})
			}
			run, rows, err := s.service.GetRun(ctx, runID)
			if err != nil {
				return err
			}
			view := struct {
				runSummaryView
				Rows []identityreconcile.BatchRow `json:"rows,omitempty"`
			}{runSummaryView: runSummary(run, rows)}
			if !summaryOnly {
				view.Rows = []identityreconcile.BatchRow{}
				for _, row := range rows {
					if (proposal == "" || string(row.Proposal) == proposal) && (decision == "" || string(row.Decision) == decision) {
						view.Rows = append(view.Rows, row)
					}
				}
			}
			return writeJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&runID, "run", "", "Run ID")
	cmd.Flags().StringVar(&app, "app", "", "List this application's recent runs instead")
	cmd.Flags().StringVar(&proposal, "proposal", "", "Only rows with this proposal (reuse, create, review, linked, linked_no_access, revoked)")
	cmd.Flags().StringVar(&decision, "decision", "", "Only rows with this decision (pending, approved, rejected)")
	cmd.Flags().BoolVar(&summaryOnly, "summary", false, "Print counts only")
	return cmd
}

func newRunDecideCommand(approve bool) *cobra.Command {
	var runID, reviewer, note string
	var accounts []string
	use, short := "reject", "Reject rows; rejected rows are never applied"
	if approve {
		use, short = "approve", "Approve reuse or create rows for apply"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long: short + ".\nThe reviewer is recorded on each row with the time and the optional note. A row's decision\n" +
			"can change until apply first touches it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(reviewer) == "" {
				return identityreconcile.ErrReviewerRequired
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			s, err := openBatchSession(ctx, false)
			if err != nil {
				return err
			}
			defer s.close()
			decide := s.service.RejectRows
			if approve {
				decide = s.service.ApproveRows
			}
			rows, err := decide(ctx, runID, accounts, reviewer, note)
			if writeErr := writeJSON(cmd, struct {
				Rows []identityreconcile.BatchRow `json:"rows"`
			}{rows}); writeErr != nil && err == nil {
				err = writeErr
			}
			return err
		},
	}
	cmd.Flags().StringVar(&runID, "run", "", "Run ID (required)")
	cmd.Flags().StringSliceVar(&accounts, "account", nil, "Account ID to decide; repeat or comma-separate (required)")
	cmd.Flags().StringVar(&reviewer, "reviewer", "", "Who reviewed these rows, recorded on each row (required)")
	cmd.Flags().StringVar(&note, "note", "", "Optional review note (at most 1000 characters)")
	for _, name := range []string{"run", "account", "reviewer"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}

func newRunApplyCommand(resume bool) *cobra.Command {
	var runID string
	use, short := "apply", "Apply approved rows that were never attempted (invite, re-check, link, grant)"
	if resume {
		use, short = "resume", "Retry attempted rows that did not finish (linked rows only retry the grant)"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Hour)
			defer cancel()
			s, err := openBatchSession(ctx, true)
			if err != nil {
				return err
			}
			defer s.close()
			run, _, err := s.service.GetRun(ctx, runID)
			if err != nil {
				return err
			}
			target, err := identitySyncBatchTarget(ctx, s.rt, run.ApplicationClientID)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), run.ApplicationClientID+" identity links use issuer "+target.Issuer)
			process := s.service.Apply
			if resume {
				process = s.service.Resume
			}
			result, err := process(ctx, target, runID)
			if result.RunID != "" {
				if writeErr := writeJSON(cmd, result); writeErr != nil && err == nil {
					err = writeErr
				}
			}
			if err == nil && result.Status == identityreconcile.RunFailed {
				return errors.New("some rows did not finish; fix the cause and run 'identity-sync run resume --run " + runID + "'")
			}
			return err
		},
	}
	cmd.Flags().StringVar(&runID, "run", "", "Run ID (required)")
	_ = cmd.MarkFlagRequired("run")
	return cmd
}
