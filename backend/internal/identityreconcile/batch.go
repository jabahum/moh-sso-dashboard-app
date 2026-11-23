package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// Reviewed batch sync (design: docs/architecture/identity-sync.md, Track B).
//
// StartRun walks an application's eligible accounts and stores one proposal
// per account. Operators approve or reject rows (a reviewer is required).
// Apply then works through approved rows that were never attempted, and Resume
// through attempted rows that did not finish. Every row step is idempotent and
// recorded in its own transaction.

// Proposal is what a run suggests for one account.
type Proposal string

const (
	ProposalReuse          Proposal = "reuse"            // one verified, enabled Keycloak user matches the email
	ProposalCreate         Proposal = "create"           // no Keycloak user matches: invite one
	ProposalReview         Proposal = "review"           // ambiguous, unverified or no email
	ProposalLinked         Proposal = "linked"           // linked and holding the access role
	ProposalLinkedNoAccess Proposal = "linked_no_access" // linked but without the access role; never auto-granted
	ProposalRevoked        Proposal = "revoked"          // access was revoked; only an explicit restore grants it again
)

// RowDecision is the reviewer's decision on a row.
type RowDecision string

const (
	DecisionPending  RowDecision = "pending"
	DecisionApproved RowDecision = "approved"
	DecisionRejected RowDecision = "rejected"
)

// RunStatus is the state of a run.
type RunStatus string

const (
	RunPreviewed RunStatus = "previewed"
	RunApplying  RunStatus = "applying"
	RunCompleted RunStatus = "completed"
	// RunFailed means some attempted rows did not finish; resume retries them.
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

// Row step statuses (columns user_status, link_status and grant_status).
const (
	UserNotRequired = "not_required"
	UserPending     = "pending"
	UserInvited     = "invited"
	UserFailed      = "failed"

	LinkPending    = "pending"
	LinkLinked     = "linked"
	LinkConflict   = "conflict"
	LinkIneligible = "ineligible"
	LinkFailed     = "failed"

	GrantPending     = "pending"
	GrantGranted     = "granted"
	GrantFailed      = "failed"
	GrantNotRequired = "not_required"
)

// Stable row error codes stored in last_error. Upstream bodies are never stored.
const (
	CodeAccountIneligible     = "account_ineligible"
	CodeAccountChanged        = "account_changed"
	CodeAccountLookupFailed   = "account_lookup_failed"
	CodeDirectoryChanged      = "directory_changed"
	CodeDirectoryLookupFailed = "directory_lookup_failed"
	CodeInviteFailed          = "invite_failed"
	CodeInviteEmailCollision  = "invite_email_collision"
	CodeInviteAmbiguous       = "invite_ambiguous"
	CodeInviteOriginMismatch  = "invite_origin_mismatch"
	CodeInviteUserDisabled    = "invite_user_disabled"
	CodeInviteAttribute       = "invite_attribute_not_stored"
	CodeInviteEmailFailed     = "invite_email_failed"
	CodeInvalidIdentity       = "invalid_identity"
	CodeLinkConflict          = "link_conflict"
	CodeLinkFailed            = "link_failed"
	CodeGrantFailed           = "grant_failed"
	CodeIdentityRevoked       = "identity_revoked"
	CodeRevocationCheckFailed = "revocation_check_failed"
)

var (
	ErrRunNotFound      = errors.New("identity sync run not found")
	ErrRowNotFound      = errors.New("identity sync run has no row for this account")
	ErrRunBusy          = errors.New("identity sync run is being applied by another process")
	ErrRunCancelled     = errors.New("identity sync run is cancelled")
	ErrRunMismatch      = errors.New("identity sync run belongs to another application or issuer")
	ErrReviewerRequired = errors.New("a reviewer identity is required")
	ErrInvalidNote      = errors.New("review note must be at most 1000 characters without control characters")
	ErrRowNotApprovable = errors.New("only reuse and create rows can be approved")
	ErrRowStarted       = errors.New("the row was already applied; its decision can no longer change")
)

// BatchRun is one stored run.
type BatchRun struct {
	ID                  string    `json:"id"`
	ApplicationClientID string    `json:"applicationId"`
	Issuer              string    `json:"issuer"`
	Status              RunStatus `json:"status"`
	CreatedBy           string    `json:"createdBy"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// BatchRow is one account in a run. Empty strings stand for SQL NULL.
type BatchRow struct {
	ID              string      `json:"id"`
	RunID           string      `json:"runId"`
	AccountID       string      `json:"accountId"`
	EmailHint       string      `json:"emailHint"`
	Proposal        Proposal    `json:"proposal"`
	ProposedSubject string      `json:"proposedSubject,omitempty"`
	Decision        RowDecision `json:"decision"`
	ReviewedBy      string      `json:"reviewedBy,omitempty"`
	ReviewedAt      *time.Time  `json:"reviewedAt,omitempty"`
	ReviewNote      string      `json:"reviewNote,omitempty"`
	UserStatus      string      `json:"userStatus"`
	LinkStatus      string      `json:"linkStatus"`
	GrantStatus     string      `json:"grantStatus"`
	Attempts        int         `json:"attempts"`
	LastError       string      `json:"lastError,omitempty"`
	UpdatedAt       time.Time   `json:"updatedAt"`
}

// Finished reports a row whose link and grant are both done.
func (r BatchRow) Finished() bool {
	return r.LinkStatus == LinkLinked && r.GrantStatus == GrantGranted
}

// Terminal reports a row the application refused for good (conflict or ineligible).
func (r BatchRow) Terminal() bool {
	return r.LinkStatus == LinkConflict || r.LinkStatus == LinkIneligible
}

// actionable reports an approved, reviewed reuse/create row with work left.
func (r BatchRow) actionable() bool {
	return r.Decision == DecisionApproved && strings.TrimSpace(r.ReviewedBy) != "" &&
		(r.Proposal == ProposalReuse || r.Proposal == ProposalCreate) && !r.Finished() && !r.Terminal()
}

// BatchRepository stores runs and rows (PostgresBatchRepository in production).
type BatchRepository interface {
	// CreateRun stores a run and its rows in one transaction and returns the
	// stored run. ErrIntegrationNotFound when the application is not registered.
	CreateRun(ctx context.Context, run BatchRun, rows []BatchRow) (BatchRun, error)
	GetRun(ctx context.Context, runID string) (BatchRun, error)
	// ListRuns returns an application's most recent runs first.
	ListRuns(ctx context.Context, application string, limit int) ([]BatchRun, error)
	// ListRows returns a run's rows ordered by account ID.
	ListRows(ctx context.Context, runID string) ([]BatchRow, error)
	// TransitionRow locks one row, passes it with its run to mutate and stores
	// the result, all in one transaction. An error from mutate rolls back.
	TransitionRow(ctx context.Context, runID, accountID string, mutate func(run BatchRun, row *BatchRow) error) (BatchRow, error)
	// ClaimRun moves a run to applying when its status is one of from, or when
	// it is applying but untouched for staleAfter (a crashed apply). It returns
	// the run as it was before the claim; ErrRunBusy or ErrRunCancelled otherwise.
	ClaimRun(ctx context.Context, runID string, from []RunStatus, staleAfter time.Duration) (BatchRun, error)
	// FinishRun moves an applying run to status.
	FinishRun(ctx context.Context, runID string, status RunStatus) error
}

// RevocationChecker reports whether a person's access to an application was
// revoked (Track C, ihp_identity_sync_revocations). A revoked subject is
// proposed as "revoked" and apply never links or grants it.
type RevocationChecker interface {
	IsRevoked(ctx context.Context, application, issuer, subject string) (bool, error)
}

// NoRevocations is the default RevocationChecker: nothing is revoked.
type NoRevocations struct{}

func (NoRevocations) IsRevoked(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// AccessChecker reports whether an identity holds the application's access role.
type AccessChecker interface {
	HasAccess(ctx context.Context, identity identitysync.Identity) (bool, error)
}

// BatchDirectory is the directory a batch needs: matching, invitation,
// granting and an access check for already-linked accounts.
type BatchDirectory interface {
	Directory
	AccessChecker
}

// BatchTarget is one registered application with its connector and directory.
type BatchTarget struct {
	Application string
	Issuer      string
	Accounts    Accounts
	Directory   BatchDirectory
}

func (t BatchTarget) validate() error {
	if t.Application == "" || t.Accounts == nil || t.Directory == nil || !strings.HasPrefix(t.Issuer, "https://") {
		return errors.New("batch sync needs the application, an HTTPS issuer, its connector and a directory")
	}
	return nil
}

// Batch defaults.
const (
	DefaultBatchPageSize   = identitysync.MaxPageSize
	DefaultBatchMaxPages   = 1000
	DefaultBatchStaleAfter = 15 * time.Minute
	maxReviewNote          = 1000
	maxReviewer            = 200
)

// BatchService runs reviewed batch syncs. The zero value of every optional
// field is usable.
type BatchService struct {
	Repo BatchRepository
	// Revocations is consulted for proposals and before every link and grant.
	// Nil means NoRevocations.
	Revocations RevocationChecker
	PageSize    int
	MaxPages    int
	// StaleAfter lets apply take over a run left "applying" by a crashed process.
	StaleAfter time.Duration
}

func (s BatchService) pageSize() int {
	if s.PageSize < 1 || s.PageSize > identitysync.MaxPageSize {
		return DefaultBatchPageSize
	}
	return s.PageSize
}

func (s BatchService) maxPages() int {
	if s.MaxPages < 1 {
		return DefaultBatchMaxPages
	}
	return s.MaxPages
}

func (s BatchService) staleAfter() time.Duration {
	if s.StaleAfter <= 0 {
		return DefaultBatchStaleAfter
	}
	return s.StaleAfter
}

func (s BatchService) revoked(ctx context.Context, t BatchTarget, subject string) (bool, error) {
	if s.Revocations == nil || subject == "" {
		return false, nil
	}
	return s.Revocations.IsRevoked(ctx, t.Application, t.Issuer, subject)
}

// walkAccounts calls visit for every eligible account, page by page.
func (s BatchService) walkAccounts(ctx context.Context, accounts Accounts, visit func(identitysync.Account) (stop bool, err error)) error {
	cursor := ""
	for range s.maxPages() {
		page, err := accounts.ListEligibleAccounts(ctx, cursor, s.pageSize())
		if err != nil {
			return fmt.Errorf("list eligible accounts: %w", err)
		}
		for _, account := range page.Accounts {
			stop, err := visit(account)
			if err != nil || stop {
				return err
			}
		}
		if page.NextCursor == "" {
			return nil
		}
		if page.NextCursor == cursor {
			return errors.New("application returned a repeating cursor")
		}
		cursor = page.NextCursor
	}
	return fmt.Errorf("account listing exceeded %d pages", s.maxPages())
}

// StartRun reads every eligible account (read-only on both sides) and stores
// a run with one proposal per account. Any listing, directory or revocation
// error stops the run before anything is stored: an unknown answer is never
// treated as permission to invite or grant.
func (s BatchService) StartRun(ctx context.Context, t BatchTarget, createdBy string) (BatchRun, []BatchRow, error) {
	if err := t.validate(); err != nil {
		return BatchRun{}, nil, err
	}
	createdBy, err := cleanActor(createdBy)
	if err != nil {
		return BatchRun{}, nil, err
	}
	seen := map[string]bool{}
	var rows []BatchRow
	err = s.walkAccounts(ctx, t.Accounts, func(account identitysync.Account) (bool, error) {
		// The contract allows an account to be listed twice during a walk.
		if seen[account.ID] {
			return false, nil
		}
		seen[account.ID] = true
		row, err := s.propose(ctx, t, account)
		if err != nil {
			return true, err
		}
		rows = append(rows, row)
		return false, nil
	})
	if err != nil {
		return BatchRun{}, nil, err
	}
	run, err := s.Repo.CreateRun(ctx, BatchRun{ApplicationClientID: t.Application, Issuer: t.Issuer, Status: RunPreviewed, CreatedBy: createdBy}, rows)
	if err != nil {
		return BatchRun{}, nil, err
	}
	stored, err := s.Repo.ListRows(ctx, run.ID)
	if err != nil {
		return BatchRun{}, nil, err
	}
	return run, stored, nil
}

// propose classifies one account. It performs reads only.
func (s BatchService) propose(ctx context.Context, t BatchTarget, account identitysync.Account) (BatchRow, error) {
	row := BatchRow{
		AccountID: account.ID, EmailHint: account.Email, Proposal: ProposalReview, Decision: DecisionPending,
		UserStatus: UserNotRequired, LinkStatus: LinkPending, GrantStatus: GrantPending,
	}
	if account.Link != (identitysync.Identity{}) {
		if !identitysync.ValidIdentity(account.Link, t.Issuer) {
			// Linked to another issuer: a person must look at it.
			return row, nil
		}
		row.ProposedSubject = account.Link.Subject
		row.LinkStatus = LinkLinked
		revoked, err := s.revoked(ctx, t, account.Link.Subject)
		if err != nil {
			return BatchRow{}, fmt.Errorf("check revocations: %w", err)
		}
		if revoked {
			row.Proposal, row.GrantStatus = ProposalRevoked, GrantNotRequired
			return row, nil
		}
		has, err := t.Directory.HasAccess(ctx, account.Link)
		if err != nil {
			return BatchRow{}, fmt.Errorf("check application access: %w", err)
		}
		if has {
			row.Proposal, row.GrantStatus = ProposalLinked, GrantGranted
		} else {
			row.Proposal, row.GrantStatus = ProposalLinkedNoAccess, GrantNotRequired
		}
		return row, nil
	}
	email := strings.TrimSpace(account.Email)
	if email == "" {
		return row, nil
	}
	matches, err := t.Directory.FindEmail(ctx, email)
	if err != nil {
		return BatchRow{}, fmt.Errorf("look up directory matches: %w", err)
	}
	switch {
	case len(matches) == 0:
		row.Proposal, row.UserStatus = ProposalCreate, UserPending
	case len(matches) == 1 && identitysync.ValidIdentity(matches[0], t.Issuer):
		row.ProposedSubject = matches[0].Subject
		revoked, err := s.revoked(ctx, t, matches[0].Subject)
		if err != nil {
			return BatchRow{}, fmt.Errorf("check revocations: %w", err)
		}
		if revoked {
			row.Proposal, row.GrantStatus = ProposalRevoked, GrantNotRequired
		} else {
			row.Proposal = ProposalReuse
		}
	}
	return row, nil
}

// GetRun returns a run with its rows.
func (s BatchService) GetRun(ctx context.Context, runID string) (BatchRun, []BatchRow, error) {
	if _, err := uuid.Parse(runID); err != nil {
		return BatchRun{}, nil, ErrRunNotFound
	}
	run, err := s.Repo.GetRun(ctx, runID)
	if err != nil {
		return BatchRun{}, nil, err
	}
	rows, err := s.Repo.ListRows(ctx, runID)
	return run, rows, err
}

// ListRuns returns an application's recent runs.
func (s BatchService) ListRuns(ctx context.Context, application string, limit int) ([]BatchRun, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.Repo.ListRuns(ctx, application, limit)
}

func cleanActor(actor string) (string, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" || len(actor) > maxReviewer || strings.IndexFunc(actor, unicode.IsControl) >= 0 {
		return "", ErrReviewerRequired
	}
	return actor, nil
}

func cleanNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if len([]rune(note)) > maxReviewNote || strings.IndexFunc(note, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0 {
		return "", ErrInvalidNote
	}
	return note, nil
}

// ApproveRows approves reuse and create rows. The reviewer is required.
// Each row changes in its own transaction; on error the rows before it stay approved.
func (s BatchService) ApproveRows(ctx context.Context, runID string, accountIDs []string, reviewer, note string) ([]BatchRow, error) {
	return s.decide(ctx, runID, accountIDs, reviewer, note, DecisionApproved)
}

// RejectRows rejects rows of any proposal. The reviewer is required.
func (s BatchService) RejectRows(ctx context.Context, runID string, accountIDs []string, reviewer, note string) ([]BatchRow, error) {
	return s.decide(ctx, runID, accountIDs, reviewer, note, DecisionRejected)
}

func (s BatchService) decide(ctx context.Context, runID string, accountIDs []string, reviewer, note string, decision RowDecision) ([]BatchRow, error) {
	reviewer, err := cleanActor(reviewer)
	if err != nil {
		return nil, err
	}
	if note, err = cleanNote(note); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(runID); err != nil {
		return nil, ErrRunNotFound
	}
	if len(accountIDs) == 0 {
		return nil, errors.New("select at least one account")
	}
	var decided []BatchRow
	for _, accountID := range accountIDs {
		row, err := s.Repo.TransitionRow(ctx, runID, accountID, func(run BatchRun, row *BatchRow) error {
			if run.Status == RunCancelled {
				return ErrRunCancelled
			}
			if decision == DecisionApproved && row.Proposal != ProposalReuse && row.Proposal != ProposalCreate {
				return ErrRowNotApprovable
			}
			// Once apply has touched a row its decision is part of the record.
			if row.Attempts > 0 || row.UserStatus == UserInvited {
				return ErrRowStarted
			}
			now := time.Now().UTC()
			row.Decision, row.ReviewedBy, row.ReviewedAt, row.ReviewNote = decision, reviewer, &now, note
			return nil
		})
		if err != nil {
			return decided, fmt.Errorf("account %s: %w", accountID, err)
		}
		decided = append(decided, row)
	}
	return decided, nil
}

// RowOutcome summarises what one apply or resume did to a row.
type RowOutcome struct {
	AccountID string `json:"accountId"`
	Outcome   string `json:"outcome"` // done, review, revoked, conflict, ineligible, failed, skipped
	Code      string `json:"code,omitempty"`
}

// ApplyResult is the result of Apply or Resume.
type ApplyResult struct {
	RunID  string       `json:"runId"`
	Status RunStatus    `json:"status"`
	Rows   []RowOutcome `json:"rows"`
}

// Apply processes approved rows that were never attempted.
func (s BatchService) Apply(ctx context.Context, t BatchTarget, runID string) (ApplyResult, error) {
	return s.process(ctx, t, runID, func(r BatchRow) bool { return r.Attempts == 0 })
}

// Resume re-processes attempted rows that did not finish. A row that is
// already linked only has its grant retried; the link is never recreated.
func (s BatchService) Resume(ctx context.Context, t BatchTarget, runID string) (ApplyResult, error) {
	return s.process(ctx, t, runID, func(r BatchRow) bool { return r.Attempts > 0 })
}

func (s BatchService) process(ctx context.Context, t BatchTarget, runID string, selectRow func(BatchRow) bool) (result ApplyResult, err error) {
	if err := t.validate(); err != nil {
		return ApplyResult{}, err
	}
	if _, err := uuid.Parse(runID); err != nil {
		return ApplyResult{}, ErrRunNotFound
	}
	run, err := s.Repo.GetRun(ctx, runID)
	if err != nil {
		return ApplyResult{}, err
	}
	if run.ApplicationClientID != t.Application || run.Issuer != t.Issuer {
		return ApplyResult{}, ErrRunMismatch
	}
	previous, err := s.Repo.ClaimRun(ctx, runID, []RunStatus{RunPreviewed, RunCompleted, RunFailed}, s.staleAfter())
	if err != nil {
		return ApplyResult{}, err
	}
	restore := previous.Status
	if restore == RunApplying {
		restore = RunFailed
	}
	started := false
	defer func() {
		// Use a fresh context so a cancelled command still releases the run.
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		status := restore
		if started {
			status = s.finalStatus(finishCtx, runID)
		}
		if finishErr := s.Repo.FinishRun(finishCtx, runID, status); finishErr != nil && err == nil {
			err = finishErr
		}
		result.Status = status
	}()

	rows, err := s.Repo.ListRows(ctx, runID)
	if err != nil {
		return ApplyResult{RunID: runID}, err
	}
	var selected []BatchRow
	needAccounts := map[string]bool{}
	for _, row := range rows {
		if !row.actionable() || !selectRow(row) {
			continue
		}
		selected = append(selected, row)
		if row.LinkStatus != LinkLinked {
			needAccounts[row.AccountID] = true
		}
	}
	// Re-read the current accounts once for the rows that still need a link.
	current := map[string]identitysync.Account{}
	if len(needAccounts) > 0 {
		err := s.walkAccounts(ctx, t.Accounts, func(account identitysync.Account) (bool, error) {
			if needAccounts[account.ID] {
				current[account.ID] = account
			}
			return len(current) == len(needAccounts), nil
		})
		if err != nil {
			return ApplyResult{RunID: runID}, fmt.Errorf("re-read eligible accounts: %w", err)
		}
	}
	started = true
	result.RunID = runID
	for _, row := range selected {
		account, ok := current[row.AccountID]
		outcome := s.processRow(ctx, t, row, account, ok)
		result.Rows = append(result.Rows, outcome)
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}
	return result, nil
}

// finalStatus is failed while any attempted, approved row has work left.
func (s BatchService) finalStatus(ctx context.Context, runID string) RunStatus {
	rows, err := s.Repo.ListRows(ctx, runID)
	if err != nil {
		return RunFailed
	}
	for _, row := range rows {
		if row.Attempts > 0 && row.actionable() {
			return RunFailed
		}
	}
	return RunCompleted
}

// errStaleRow aborts a transition when the row changed since it was selected.
var errStaleRow = errors.New("row changed since it was selected")

// processRow moves one approved row through invite, directory re-check, link
// and grant. Each step's outcome is stored in its own row transaction, and
// every step is skipped when an earlier attempt already completed it.
func (s BatchService) processRow(ctx context.Context, t BatchTarget, row BatchRow, account identitysync.Account, found bool) RowOutcome {
	out := RowOutcome{AccountID: row.AccountID}
	update := func(mutate func(*BatchRow)) bool {
		stored, err := s.Repo.TransitionRow(ctx, row.RunID, row.AccountID, func(run BatchRun, r *BatchRow) error {
			if run.Status != RunApplying || !r.actionable() {
				return errStaleRow
			}
			mutate(r)
			return nil
		})
		if err != nil {
			out.Outcome, out.Code = "skipped", ""
			return false
		}
		row = stored
		return true
	}
	fail := func(code string, mutate func(*BatchRow)) RowOutcome {
		if update(func(r *BatchRow) { mutate(r); r.LastError = code }) {
			out.Outcome, out.Code = "failed", code
		}
		return out
	}
	// Start the attempt. The row must still be approved by a named reviewer.
	if !update(func(r *BatchRow) { r.Attempts++; r.LastError = "" }) {
		return out
	}

	if row.LinkStatus != LinkLinked {
		if !found {
			// Missing from the walk is not proof of ineligibility (the listing may
			// have skipped it); resume retries. Only a 404 from PUT is final.
			return fail(CodeAccountLookupFailed, func(r *BatchRow) { r.LinkStatus = LinkFailed })
		}
		if account.Email != row.EmailHint {
			return s.backToReview(update, &out, CodeAccountChanged)
		}
		if row.Proposal == ProposalCreate && row.UserStatus != UserInvited {
			identity, err := t.Directory.EnsureUser(ctx, account)
			if err == nil && !identitysync.ValidIdentity(identity, t.Issuer) {
				err = errInvalidIdentity
			}
			if err != nil {
				return fail(inviteCode(err), func(r *BatchRow) { r.UserStatus = UserFailed })
			}
			if !update(func(r *BatchRow) { r.UserStatus, r.ProposedSubject = UserInvited, identity.Subject }) {
				return out
			}
		}
		identity := identitysync.Identity{Issuer: t.Issuer, Subject: row.ProposedSubject}
		if !identitysync.ValidIdentity(identity, t.Issuer) {
			return fail(CodeInvalidIdentity, func(*BatchRow) {})
		}
		if done, o := s.blockRevoked(ctx, t, identity.Subject, update, &out); done {
			return o
		}
		// Re-run the directory lookup: it must still point at the reviewed subject.
		matches, err := t.Directory.FindEmail(ctx, strings.TrimSpace(account.Email))
		if err != nil {
			return fail(CodeDirectoryLookupFailed, func(*BatchRow) {})
		}
		if !directoryStillMatches(row.Proposal, matches, identity) {
			return s.backToReview(update, &out, CodeDirectoryChanged)
		}
		switch {
		case account.Link == identity:
			// A 1.1 application already reports this exact link.
		case account.Link != (identitysync.Identity{}):
			if update(func(r *BatchRow) { r.LinkStatus, r.LastError = LinkConflict, CodeLinkConflict }) {
				out.Outcome, out.Code = "conflict", CodeLinkConflict
			}
			return out
		default:
			err := t.Accounts.LinkIdentity(ctx, account.ID, identity)
			switch {
			case errors.Is(err, identitysync.ErrLinkConflict):
				if update(func(r *BatchRow) { r.LinkStatus, r.LastError = LinkConflict, CodeLinkConflict }) {
					out.Outcome, out.Code = "conflict", CodeLinkConflict
				}
				return out
			case errors.Is(err, identitysync.ErrAccountNotFound):
				if update(func(r *BatchRow) { r.LinkStatus, r.LastError = LinkIneligible, CodeAccountIneligible }) {
					out.Outcome, out.Code = "ineligible", CodeAccountIneligible
				}
				return out
			case err != nil:
				return fail(CodeLinkFailed, func(r *BatchRow) { r.LinkStatus = LinkFailed })
			}
		}
		if !update(func(r *BatchRow) { r.LinkStatus = LinkLinked }) {
			return out
		}
	}

	// Grant. Linked rows reach here directly on resume: only the grant is retried.
	identity := identitysync.Identity{Issuer: t.Issuer, Subject: row.ProposedSubject}
	if !identitysync.ValidIdentity(identity, t.Issuer) {
		return fail(CodeInvalidIdentity, func(r *BatchRow) { r.GrantStatus = GrantFailed })
	}
	// "check revoked -> grant" holds the access lock a revocation takes across
	// "record revocation -> remove role", so the two cannot interleave.
	stopped, granted := false, false
	var grantErr error
	lockErr := withAccessLock(ctx, accessLockerOf(s.Revocations), t.Application, identity.Subject, func(ctx context.Context) error {
		if done, o := s.blockRevoked(ctx, t, identity.Subject, update, &out); done {
			stopped, out = true, o
			return nil
		}
		if grantErr = t.Directory.GrantAccess(ctx, identity); grantErr == nil {
			granted = true
		}
		return nil
	})
	switch {
	case lockErr != nil && !stopped && !granted:
		return fail(CodeRevocationCheckFailed, func(r *BatchRow) { r.GrantStatus = GrantFailed })
	case stopped:
		return out
	case grantErr != nil:
		return fail(CodeGrantFailed, func(r *BatchRow) { r.GrantStatus = GrantFailed })
	}
	// Second line of defence: a revocation written without the lock between the
	// check and the grant is re-asserted now (the role is removed again).
	if enforcer := enforcerOf(s.Revocations); enforcer != nil {
		enforced, err := enforcer.EnforceRevocation(ctx, t.Application, t.Issuer, identity.Subject, "identity-sync run "+row.RunID)
		switch {
		case enforced:
			if update(func(r *BatchRow) {
				r.Proposal, r.GrantStatus, r.LastError = ProposalRevoked, GrantNotRequired, CodeIdentityRevoked
			}) {
				out.Outcome, out.Code = "revoked", CodeIdentityRevoked
			}
			return out
		case err != nil:
			// Unknown whether a revocation appeared: resume re-checks before granting again.
			return fail(CodeRevocationCheckFailed, func(r *BatchRow) { r.GrantStatus = GrantFailed })
		}
	}
	if update(func(r *BatchRow) { r.GrantStatus, r.LastError = GrantGranted, "" }) {
		out.Outcome, out.Code = "done", ""
	}
	return out
}

var errInvalidIdentity = errors.New("directory returned an invalid identity")

// blockRevoked marks the row revoked (never linked or granted) when the
// subject's access was revoked. A failed check also stops the row.
func (s BatchService) blockRevoked(ctx context.Context, t BatchTarget, subject string, update func(func(*BatchRow)) bool, out *RowOutcome) (bool, RowOutcome) {
	revoked, err := s.revoked(ctx, t, subject)
	if err != nil {
		if update(func(r *BatchRow) {
			r.LastError = CodeRevocationCheckFailed
			if r.LinkStatus == LinkLinked {
				r.GrantStatus = GrantFailed
			}
		}) {
			out.Outcome, out.Code = "failed", CodeRevocationCheckFailed
		}
		return true, *out
	}
	if !revoked {
		return false, *out
	}
	if update(func(r *BatchRow) {
		r.Proposal, r.GrantStatus, r.LastError = ProposalRevoked, GrantNotRequired, CodeIdentityRevoked
	}) {
		out.Outcome, out.Code = "revoked", CodeIdentityRevoked
	}
	return true, *out
}

// backToReview returns the row to review: its proposal becomes review and its
// decision pending, so it cannot be applied until a new run re-proposes it.
// The original reviewer stays recorded.
func (s BatchService) backToReview(update func(func(*BatchRow)) bool, out *RowOutcome, code string) RowOutcome {
	if update(func(r *BatchRow) {
		r.Proposal, r.Decision, r.LastError = ProposalReview, DecisionPending, code
	}) {
		out.Outcome, out.Code = "review", code
	}
	return *out
}

// directoryStillMatches re-checks the reviewed match. A reuse row needs the
// same single verified user. A create row needs exactly the user it created:
// unverified until the invitation is completed, then that same subject.
func directoryStillMatches(proposal Proposal, matches []identitysync.Identity, reviewed identitysync.Identity) bool {
	if len(matches) != 1 {
		return false
	}
	if proposal == ProposalCreate && matches[0] == (identitysync.Identity{}) {
		return true
	}
	return matches[0] == reviewed
}

func inviteCode(err error) string {
	switch {
	case errors.Is(err, ErrInviteEmailCollision):
		return CodeInviteEmailCollision
	case errors.Is(err, ErrInviteAmbiguous):
		return CodeInviteAmbiguous
	case errors.Is(err, ErrInviteOriginMismatch):
		return CodeInviteOriginMismatch
	case errors.Is(err, ErrInviteUserDisabled):
		return CodeInviteUserDisabled
	case errors.Is(err, ErrInviteAttributeNotStored):
		return CodeInviteAttribute
	case errors.Is(err, ErrInviteEmailFailed):
		return CodeInviteEmailFailed
	case errors.Is(err, errInvalidIdentity):
		return CodeInvalidIdentity
	default:
		return CodeInviteFailed
	}
}
