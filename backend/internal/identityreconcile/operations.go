package identityreconcile

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// Operations runs the identity-sync batch and revocation work for the portal's
// admin API. Starting, applying and resuming a run, and resuming revocations,
// can take longer than one HTTP request (the application is walked page by
// page and every row calls Keycloak and the application), so they run as
// background jobs; their state is kept in memory and reported by Jobs. Run and
// revocation state itself is always in Postgres, so a job lost with its process
// leaves nothing hidden: an apply left "applying" is taken over after
// BatchService.StaleAfter and unfinished revocations stay listed.
type Operations struct {
	Batch    BatchService
	Registry *RegistryService
	Revoke   *RevokeService
	// Admin is the Keycloak admin client the directory uses for matching,
	// invitations and grants.
	Admin  *keycloak.KeyAdminClient
	Issuer string
	// JobTimeout bounds one background job. Zero means DefaultJobTimeout.
	JobTimeout time.Duration
	Logger     *slog.Logger

	// spawn starts a job body; tests replace it to run jobs synchronously.
	spawn func(func())
	// target builds the batch target; tests replace it.
	target func(ctx context.Context, application string) (BatchTarget, error)

	mu   sync.Mutex
	jobs map[string][]*Job
}

// DefaultJobTimeout bounds a background job when Operations.JobTimeout is zero.
const DefaultJobTimeout = 30 * time.Minute

// jobsKept is how many jobs per application Jobs reports.
const jobsKept = 10

// JobKind names a background job.
type JobKind string

const (
	JobStartRun          JobKind = "start_run"
	JobApplyRun          JobKind = "apply_run"
	JobResumeRun         JobKind = "resume_run"
	JobResumeRevocations JobKind = "resume_revocations"
)

// JobStatus is a background job's state.
type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

// Job is one background job. Error is a stable code; ErrorMessage is safe to
// show (it never carries upstream bodies, URLs or secrets).
type Job struct {
	ID           string         `json:"id"`
	Application  string         `json:"applicationId"`
	Kind         JobKind        `json:"kind"`
	RunID        string         `json:"runId,omitempty"`
	Status       JobStatus      `json:"status"`
	Actor        string         `json:"actor"`
	Error        string         `json:"error,omitempty"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
	Summary      map[string]int `json:"summary,omitempty"`
	StartedAt    time.Time      `json:"startedAt"`
	FinishedAt   *time.Time     `json:"finishedAt,omitempty"`
}

// ErrJobRunning is returned when the application already has a running job.
var ErrJobRunning = errors.New("another identity-sync job is running for this application")

func (o *Operations) logger() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return slog.Default()
}

func (o *Operations) timeout() time.Duration {
	if o.JobTimeout > 0 {
		return o.JobTimeout
	}
	return DefaultJobTimeout
}

// Target builds the batch target of an enabled, correctly bound application.
func (o *Operations) Target(ctx context.Context, application string) (BatchTarget, error) {
	if o.target != nil {
		return o.target(ctx, application)
	}
	if o.Registry == nil || o.Admin == nil || !strings.HasPrefix(o.Issuer, "https://") {
		return BatchTarget{}, errors.New("identity sync operations are not configured")
	}
	connector, integration, err := o.Registry.Connector(ctx, application)
	if err != nil {
		return BatchTarget{}, err
	}
	directory := KeycloakDirectory{
		Admin: o.Admin, Issuer: o.Issuer,
		Application: integration.ApplicationClientID, AccessRole: integration.AccessRole,
	}
	return BatchTarget{Application: integration.ApplicationClientID, Issuer: o.Issuer, Accounts: connector, Directory: directory}, nil
}

// Jobs returns an application's recent jobs, newest first.
func (o *Operations) Jobs(application string) []Job {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]Job, 0, len(o.jobs[application]))
	for _, job := range o.jobs[application] {
		copied := *job
		if job.Summary != nil {
			copied.Summary = make(map[string]int, len(job.Summary))
			for k, v := range job.Summary {
				copied.Summary[k] = v
			}
		}
		out = append(out, copied)
	}
	return out
}

// begin records a running job, refusing a second one for the same application.
func (o *Operations) begin(application string, kind JobKind, runID, actor string) (*Job, error) {
	actor, err := cleanActor(actor)
	if err != nil {
		return nil, err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.jobs == nil {
		o.jobs = map[string][]*Job{}
	}
	for _, job := range o.jobs[application] {
		if job.Status == JobRunning {
			return nil, ErrJobRunning
		}
	}
	job := &Job{
		ID: uuid.NewString(), Application: application, Kind: kind, RunID: runID,
		Status: JobRunning, Actor: actor, StartedAt: time.Now().UTC(),
	}
	list := append([]*Job{job}, o.jobs[application]...)
	if len(list) > jobsKept {
		list = list[:jobsKept]
	}
	o.jobs[application] = list
	return job, nil
}

// launch runs body in the background under the job timeout and records its result.
func (o *Operations) launch(job *Job, body func(ctx context.Context) (runID string, summary map[string]int, err error)) Job {
	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), o.timeout())
		defer cancel()
		var (
			runID   string
			summary map[string]int
			err     error
		)
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = errors.New("identity sync job panicked")
					o.logger().Error("identity sync job panicked", "job", job.ID, "kind", job.Kind, "panic", r)
				}
			}()
			runID, summary, err = body(ctx)
		}()
		o.mu.Lock()
		defer o.mu.Unlock()
		now := time.Now().UTC()
		job.FinishedAt = &now
		if runID != "" {
			job.RunID = runID
		}
		job.Summary = summary
		if err != nil {
			job.Status = JobFailed
			job.Error, job.ErrorMessage = JobErrorCode(err)
			o.logger().Warn("identity sync job failed", "job", job.ID, "kind", job.Kind,
				"application", job.Application, "run", job.RunID, "error", err)
			return
		}
		job.Status = JobSucceeded
	}
	o.mu.Lock()
	snapshot := *job
	o.mu.Unlock()
	if o.spawn != nil {
		o.spawn(run)
	} else {
		go run()
	}
	return snapshot
}

// StartRun starts a background run start for an application. The integration
// is checked first, so a disabled or unbound one fails at once.
func (o *Operations) StartRun(ctx context.Context, application, actor string) (Job, error) {
	if _, err := o.Target(ctx, application); err != nil {
		return Job{}, err
	}
	job, err := o.begin(application, JobStartRun, "", actor)
	if err != nil {
		return Job{}, err
	}
	return o.launch(job, func(ctx context.Context) (string, map[string]int, error) {
		target, err := o.Target(ctx, application)
		if err != nil {
			return "", nil, err
		}
		run, rows, err := o.Batch.StartRun(ctx, target, job.Actor)
		if err != nil {
			return "", nil, err
		}
		summary := map[string]int{"total": len(rows)}
		for _, row := range rows {
			summary["proposal:"+string(row.Proposal)]++
		}
		return run.ID, summary, nil
	}), nil
}

// GetRun returns a run and its rows, provided it belongs to the application.
func (o *Operations) GetRun(ctx context.Context, application, runID string) (BatchRun, []BatchRow, error) {
	run, rows, err := o.Batch.GetRun(ctx, runID)
	if err != nil {
		return BatchRun{}, nil, err
	}
	if run.ApplicationClientID != application {
		return BatchRun{}, nil, ErrRunNotFound
	}
	return run, rows, nil
}

// Decide approves or rejects rows of an application's run.
func (o *Operations) Decide(ctx context.Context, application, runID string, accountIDs []string, approve bool, reviewer, note string) ([]BatchRow, error) {
	if _, _, err := o.GetRun(ctx, application, runID); err != nil {
		return nil, err
	}
	if approve {
		return o.Batch.ApproveRows(ctx, runID, accountIDs, reviewer, note)
	}
	return o.Batch.RejectRows(ctx, runID, accountIDs, reviewer, note)
}

// ApplyRun starts a background apply (resume=false) or resume of a run.
func (o *Operations) ApplyRun(ctx context.Context, application, runID, actor string, resume bool) (Job, error) {
	run, _, err := o.GetRun(ctx, application, runID)
	if err != nil {
		return Job{}, err
	}
	if run.Status == RunApplying {
		return Job{}, ErrRunBusy
	}
	if _, err := o.Target(ctx, application); err != nil {
		return Job{}, err
	}
	kind := JobApplyRun
	if resume {
		kind = JobResumeRun
	}
	job, err := o.begin(application, kind, runID, actor)
	if err != nil {
		return Job{}, err
	}
	return o.launch(job, func(ctx context.Context) (string, map[string]int, error) {
		target, err := o.Target(ctx, application)
		if err != nil {
			return runID, nil, err
		}
		var result ApplyResult
		if resume {
			result, err = o.Batch.Resume(ctx, target, runID)
		} else {
			result, err = o.Batch.Apply(ctx, target, runID)
		}
		summary := map[string]int{"rows": len(result.Rows)}
		for _, row := range result.Rows {
			summary["outcome:"+row.Outcome]++
		}
		return runID, summary, err
	}), nil
}

// ListRevocations lists an application's revocations.
func (o *Operations) ListRevocations(ctx context.Context, application string, includeRestored, onlyUnfinished bool, limit int) ([]Revocation, error) {
	if o.Revoke == nil {
		return nil, errors.New("identity sync revocations are not configured")
	}
	return o.Revoke.List(ctx, RevocationFilter{Application: application, IncludeRestored: includeRestored, OnlyUnfinished: onlyUnfinished, Limit: limit})
}

// ResumeRevocations starts a background retry of an application's unfinished revocations.
func (o *Operations) ResumeRevocations(application, actor string) (Job, error) {
	if o.Revoke == nil {
		return Job{}, errors.New("identity sync revocations are not configured")
	}
	job, err := o.begin(application, JobResumeRevocations, "", actor)
	if err != nil {
		return Job{}, err
	}
	return o.launch(job, func(ctx context.Context) (string, map[string]int, error) {
		resumed, err := o.Revoke.Resume(ctx, application, job.Actor)
		summary := map[string]int{"resumed": len(resumed)}
		for _, r := range resumed {
			if r.Complete() {
				summary["complete"]++
			} else {
				summary["incomplete"]++
			}
		}
		return "", summary, err
	}), nil
}

// RestoreRevocation clears a person's active revocation (it never grants access).
func (o *Operations) RestoreRevocation(ctx context.Context, application, subject, actor, reason string) (Revocation, error) {
	if o.Revoke == nil {
		return Revocation{}, errors.New("identity sync revocations are not configured")
	}
	return o.Revoke.Restore(ctx, application, subject, actor, reason)
}

// JobErrorCode maps an error to a stable code and a message that is safe to
// show to an administrator. Unknown errors get a generic message; the full
// error is only logged.
func JobErrorCode(err error) (string, string) {
	known := []struct {
		target error
		code   string
	}{
		{ErrIntegrationNotFound, "integration_not_found"},
		{ErrIntegrationDisabled, "integration_disabled"},
		{ErrInvalidIntegration, "integration_invalid"},
		{ErrDescriptorMissing, "descriptor_missing"},
		{ErrDescriptorUnavailable, "application_unreachable"},
		{ErrContractViolation, "contract_violation"},
		{ErrPrivateOrigin, "private_origin"},
		{ErrRunNotFound, "run_not_found"},
		{ErrRunBusy, "run_busy"},
		{ErrRunCancelled, "run_cancelled"},
		{ErrRunMismatch, "run_mismatch"},
		{ErrJobRunning, "job_running"},
		{ErrReviewerRequired, "reviewer_required"},
		{ErrRevocationNotFound, "revocation_not_found"},
	}
	for _, k := range known {
		if errors.Is(err, k.target) {
			return k.code, k.target.Error()
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "job_timeout", "the job took longer than allowed; resume continues from where it stopped"
	}
	return "job_failed", "the job failed; the server log has the details"
}

// ListRuns returns an application's most recent runs.
func (o *Operations) ListRuns(ctx context.Context, application string, limit int) ([]BatchRun, error) {
	return o.Batch.ListRuns(ctx, application, limit)
}

// describeTimeout keeps a live descriptor read inside one HTTP request.
const describeTimeout = 10 * time.Second

// Describe reads the registered application's live GET / descriptor.
func (o *Operations) Describe(ctx context.Context, application string) (identitysync.Descriptor, error) {
	if o.Registry == nil {
		return identitysync.Descriptor{}, errors.New("identity sync registry is not configured")
	}
	integration, err := o.Registry.Get(ctx, application)
	if err != nil {
		return identitysync.Descriptor{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()
	return o.Registry.Probe(ctx, integration)
}
