package identityreconcile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// PostgresBatchRepository stores runs in ihp_identity_sync_runs and rows in
// ihp_identity_sync_rows (migration 63).
type PostgresBatchRepository struct{ DB *sql.DB }

var _ BatchRepository = (*PostgresBatchRepository)(nil)

const runColumns = `id::text, application_client_id, issuer, status, created_by, created_at, updated_at`

const rowColumns = `id::text, run_id::text, account_id, email_hint, proposal, COALESCE(proposed_subject, ''), decision,
 COALESCE(reviewed_by, ''), reviewed_at, COALESCE(review_note, ''), user_status, link_status, grant_status, attempts,
 COALESCE(last_error, ''), updated_at`

func scanRun(row rowScanner) (BatchRun, error) {
	var r BatchRun
	var status string
	err := row.Scan(&r.ID, &r.ApplicationClientID, &r.Issuer, &status, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	r.Status = RunStatus(status)
	return r, err
}

func scanBatchRow(row rowScanner) (BatchRow, error) {
	var r BatchRow
	var proposal, decision string
	var reviewedAt sql.NullTime
	err := row.Scan(&r.ID, &r.RunID, &r.AccountID, &r.EmailHint, &proposal, &r.ProposedSubject, &decision,
		&r.ReviewedBy, &reviewedAt, &r.ReviewNote, &r.UserStatus, &r.LinkStatus, &r.GrantStatus, &r.Attempts,
		&r.LastError, &r.UpdatedAt)
	r.Proposal, r.Decision = Proposal(proposal), RowDecision(decision)
	if reviewedAt.Valid {
		t := reviewedAt.Time
		r.ReviewedAt = &t
	}
	return r, err
}

func (p *PostgresBatchRepository) CreateRun(ctx context.Context, run BatchRun, rows []BatchRow) (BatchRun, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchRun{}, err
	}
	defer tx.Rollback()
	stored, err := scanRun(tx.QueryRowContext(ctx, `INSERT INTO ihp_identity_sync_runs (application_client_id, issuer, status, created_by)
 VALUES ($1, $2, $3, $4) RETURNING `+runColumns, run.ApplicationClientID, run.Issuer, string(run.Status), run.CreatedBy))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23503" {
			return BatchRun{}, ErrIntegrationNotFound
		}
		return BatchRun{}, err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO ihp_identity_sync_rows
 (run_id, account_id, email_hint, proposal, proposed_subject, decision, user_status, link_status, grant_status)
 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`)
	if err != nil {
		return BatchRun{}, err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, stored.ID, r.AccountID, r.EmailHint, string(r.Proposal), nullable(r.ProposedSubject),
			string(r.Decision), r.UserStatus, r.LinkStatus, r.GrantStatus); err != nil {
			return BatchRun{}, fmt.Errorf("store row %s: %w", r.AccountID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return BatchRun{}, err
	}
	return stored, nil
}

func (p *PostgresBatchRepository) GetRun(ctx context.Context, runID string) (BatchRun, error) {
	run, err := scanRun(p.DB.QueryRowContext(ctx, `SELECT `+runColumns+` FROM ihp_identity_sync_runs WHERE id = $1::uuid`, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return BatchRun{}, ErrRunNotFound
	}
	return run, err
}

func (p *PostgresBatchRepository) ListRuns(ctx context.Context, application string, limit int) ([]BatchRun, error) {
	rows, err := p.DB.QueryContext(ctx, `SELECT `+runColumns+` FROM ihp_identity_sync_runs
 WHERE application_client_id = $1 ORDER BY created_at DESC, id LIMIT $2`, application, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []BatchRun
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (p *PostgresBatchRepository) ListRows(ctx context.Context, runID string) ([]BatchRow, error) {
	rows, err := p.DB.QueryContext(ctx, `SELECT `+rowColumns+` FROM ihp_identity_sync_rows WHERE run_id = $1::uuid ORDER BY account_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BatchRow
	for rows.Next() {
		row, err := scanBatchRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (p *PostgresBatchRepository) TransitionRow(ctx context.Context, runID, accountID string, mutate func(BatchRun, *BatchRow) error) (BatchRow, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchRow{}, err
	}
	defer tx.Rollback()
	row, err := scanBatchRow(tx.QueryRowContext(ctx, `SELECT `+rowColumns+` FROM ihp_identity_sync_rows
 WHERE run_id = $1::uuid AND account_id = $2 FOR UPDATE`, runID, accountID))
	if errors.Is(err, sql.ErrNoRows) {
		if _, runErr := p.GetRun(ctx, runID); errors.Is(runErr, ErrRunNotFound) {
			return BatchRow{}, ErrRunNotFound
		}
		return BatchRow{}, ErrRowNotFound
	}
	if err != nil {
		return BatchRow{}, err
	}
	run, err := scanRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM ihp_identity_sync_runs WHERE id = $1::uuid`, runID))
	if err != nil {
		return BatchRow{}, err
	}
	next := row
	if err := mutate(run, &next); err != nil {
		return BatchRow{}, err
	}
	// Identity columns never change through a transition.
	next.ID, next.RunID, next.AccountID, next.EmailHint = row.ID, row.RunID, row.AccountID, row.EmailHint
	var reviewedAt any
	if next.ReviewedAt != nil {
		reviewedAt = *next.ReviewedAt
	}
	stored, err := scanBatchRow(tx.QueryRowContext(ctx, `UPDATE ihp_identity_sync_rows SET
 proposal = $2, proposed_subject = $3, decision = $4, reviewed_by = $5, reviewed_at = $6, review_note = $7,
 user_status = $8, link_status = $9, grant_status = $10, attempts = $11, last_error = $12, updated_at = now()
 WHERE id = $1::uuid RETURNING `+rowColumns,
		row.ID, string(next.Proposal), nullable(next.ProposedSubject), string(next.Decision), nullable(next.ReviewedBy),
		reviewedAt, nullable(next.ReviewNote), next.UserStatus, next.LinkStatus, next.GrantStatus, next.Attempts,
		nullable(next.LastError)))
	if err != nil {
		return BatchRow{}, err
	}
	// The run's updated_at doubles as the apply heartbeat (see ClaimRun).
	if _, err := tx.ExecContext(ctx, `UPDATE ihp_identity_sync_runs SET updated_at = now() WHERE id = $1::uuid`, runID); err != nil {
		return BatchRow{}, err
	}
	if err := tx.Commit(); err != nil {
		return BatchRow{}, err
	}
	return stored, nil
}

func (p *PostgresBatchRepository) ClaimRun(ctx context.Context, runID string, from []RunStatus, staleAfter time.Duration) (BatchRun, error) {
	statuses := make([]string, len(from))
	for i, s := range from {
		statuses[i] = string(s)
	}
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchRun{}, err
	}
	defer tx.Rollback()
	var stale bool
	before, err := scanRun(tx.QueryRowContext(ctx, `SELECT `+runColumns+` FROM ihp_identity_sync_runs WHERE id = $1::uuid FOR UPDATE`, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return BatchRun{}, ErrRunNotFound
	}
	if err != nil {
		return BatchRun{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT updated_at < now() - make_interval(secs => $1) FROM ihp_identity_sync_runs WHERE id = $2::uuid`,
		staleAfter.Seconds(), runID).Scan(&stale); err != nil {
		return BatchRun{}, err
	}
	allowed := false
	for _, s := range statuses {
		if string(before.Status) == s {
			allowed = true
		}
	}
	switch {
	case before.Status == RunCancelled:
		return BatchRun{}, ErrRunCancelled
	case before.Status == RunApplying && !stale:
		return BatchRun{}, ErrRunBusy
	case before.Status != RunApplying && !allowed:
		return BatchRun{}, fmt.Errorf("identity sync run is %s", before.Status)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE ihp_identity_sync_runs SET status = 'applying', updated_at = now() WHERE id = $1::uuid`, runID); err != nil {
		return BatchRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return BatchRun{}, err
	}
	return before, nil
}

func (p *PostgresBatchRepository) FinishRun(ctx context.Context, runID string, status RunStatus) error {
	result, err := p.DB.ExecContext(ctx, `UPDATE ihp_identity_sync_runs SET status = $2, updated_at = now()
 WHERE id = $1::uuid AND status = 'applying'`, runID, string(status))
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("identity sync run %s is no longer applying", runID)
	}
	return nil
}
