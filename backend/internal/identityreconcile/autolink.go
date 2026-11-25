package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// AutoLinkActor is the creator and reviewer recorded on runs the auto-linker
// starts and approves, so they are distinguishable from runs a person reviewed.
const AutoLinkActor = "system:auto-link"

// autoLinkNote is the review note stored on every row the auto-linker approves.
const autoLinkNote = "approved automatically (IDENTITY_SYNC_AUTO_LINK_APPS)"

// resumeRunsLimit bounds how many recent runs are checked for unfinished
// auto-link work before a new run starts.
const resumeRunsLimit = 20

// AutoLinker links an application's accounts to Keycloak without a person
// reviewing them. It runs the same batch machinery as a reviewed run (proposal,
// invitation, directory re-check, link, grant, revocation checks, audit fields)
// but approves every reuse and create row itself.
//
// Rows proposed as review (no email, ambiguous, unverified, linked under
// another issuer), linked_no_access and revoked are never approved: the batch
// service refuses them, and they need a person.
type AutoLinker struct {
	Batch BatchService
	// Target builds the connector and Keycloak directory for an application.
	Target func(ctx context.Context, application string) (BatchTarget, error)
	Logger *slog.Logger
}

// AutoLinkResult summarises one auto-link pass for an application.
type AutoLinkResult struct {
	Application string           `json:"application"`
	RunID       string           `json:"runId"`
	Status      RunStatus        `json:"status"`
	Proposals   map[Proposal]int `json:"proposals"`
	Approved    int              `json:"approved"`
	Outcomes    map[string]int   `json:"outcomes"`
	Resumed     []string         `json:"resumed,omitempty"`
}

func (a AutoLinker) logger() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}

// Run resumes unfinished auto-link runs for the application, then starts a new
// run, approves its reuse and create rows as AutoLinkActor and applies it.
// Starting the run is read-only, so a listing or directory failure stops
// before anything is invited, linked or granted.
func (a AutoLinker) Run(ctx context.Context, application string) (AutoLinkResult, error) {
	result := AutoLinkResult{Application: application, Proposals: map[Proposal]int{}, Outcomes: map[string]int{}}
	if a.Target == nil {
		return result, errors.New("auto-link target is not configured")
	}
	target, err := a.Target(ctx, application)
	if err != nil {
		return result, fmt.Errorf("open %s: %w", application, err)
	}

	// Retry rows an earlier pass could not finish (for example an invitation
	// email that failed, or the application being briefly unavailable).
	runs, err := a.Batch.ListRuns(ctx, application, resumeRunsLimit)
	if err != nil {
		return result, fmt.Errorf("list earlier runs: %w", err)
	}
	for _, run := range runs {
		if run.CreatedBy != AutoLinkActor || run.Status != RunFailed || run.Issuer != target.Issuer {
			continue
		}
		resumed, err := a.Batch.Resume(ctx, target, run.ID)
		if err != nil {
			a.logger().Warn("identity sync auto-link: resume failed", "application", application, "run", run.ID, "error", err)
			continue
		}
		result.Resumed = append(result.Resumed, run.ID)
		for _, row := range resumed.Rows {
			result.Outcomes[row.Outcome]++
		}
	}

	run, rows, err := a.Batch.StartRun(ctx, target, AutoLinkActor)
	if err != nil {
		return result, fmt.Errorf("start run: %w", err)
	}
	result.RunID = run.ID
	var approve []string
	for _, row := range rows {
		result.Proposals[row.Proposal]++
		if (row.Proposal == ProposalReuse || row.Proposal == ProposalCreate) && row.Decision == DecisionPending {
			approve = append(approve, row.AccountID)
		}
	}
	if len(approve) > 0 {
		decided, err := a.Batch.ApproveRows(ctx, run.ID, approve, AutoLinkActor, autoLinkNote)
		result.Approved = len(decided)
		if err != nil {
			// Rows approved so far are still applied below; the rest wait for the next pass.
			a.logger().Warn("identity sync auto-link: approval stopped early", "application", application, "run", run.ID, "approved", len(decided), "error", err)
		}
	}
	// Apply even with nothing approved, so the run is recorded as completed.
	applied, err := a.Batch.Apply(ctx, target, run.ID)
	result.Status = applied.Status
	for _, row := range applied.Rows {
		result.Outcomes[row.Outcome]++
	}
	if err != nil {
		return result, fmt.Errorf("apply run %s: %w", run.ID, err)
	}
	return result, nil
}
