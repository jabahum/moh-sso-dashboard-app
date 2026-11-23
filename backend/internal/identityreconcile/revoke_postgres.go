package identityreconcile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/moh-sso-dashboard/internal/observability"
)

// PostgresRevocations stores revocations in ihp_identity_sync_revocations
// (migration 64) and audit events in ihp_rbac_audit_events.
type PostgresRevocations struct{ DB *sql.DB }

var (
	_ RevocationRepository = (*PostgresRevocations)(nil)
	_ AccessLocker         = (*PostgresRevocations)(nil)
)

// accessLockNamespace is the first key of the two-key advisory lock taken by
// WithAccessLock ("idsync" in ASCII-ish hex), so it cannot collide with
// single-key advisory locks such as the migration lock.
const accessLockNamespace = 0x1d5c

// WithAccessLock holds a transaction-scoped advisory lock on (application,
// subject) while fn runs. Grants (check revoked -> grant) and revocations
// (record revocation -> remove role) take it, so they cannot interleave. The
// key is pg_advisory_xact_lock(namespace, hashtext(application/subject));
// application IDs never contain "/", so the text is unambiguous. A hash
// collision only makes two unrelated identities wait for each other. The lock
// is released when the transaction ends, including when the connection drops.
func (p *PostgresRevocations) WithAccessLock(ctx context.Context, application, subject string, fn func(context.Context) error) error {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("take identity-sync access lock: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1::int, hashtext($2))`, accessLockNamespace, application+"/"+subject); err != nil {
		return fmt.Errorf("take identity-sync access lock: %w", err)
	}
	if err := fn(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

// Audit actions written by access removal.
const (
	AuditRevocationRecorded  = "application.identity_sync.revocation_recorded"
	AuditRevocationAttempted = "application.identity_sync.revocation_attempted"
	AuditRevocationRestored  = "application.identity_sync.revocation_restored"
)

const revocationColumns = `id::text, application_client_id, issuer, subject, revoked_by, reason, revoked_at,
 COALESCE(restored_by, ''), COALESCE(restore_reason, ''), restored_at, role_removed, consents_revoked,
 sessions_revoked, attempts, COALESCE(last_error, ''), updated_at`

func scanRevocation(row rowScanner) (Revocation, error) {
	var r Revocation
	var restoredAt sql.NullTime
	var role, consents, sessions string
	err := row.Scan(&r.ID, &r.ApplicationClientID, &r.Issuer, &r.Subject, &r.RevokedBy, &r.Reason, &r.RevokedAt,
		&r.RestoredBy, &r.RestoreReason, &restoredAt, &role, &consents, &sessions, &r.Attempts, &r.LastError, &r.UpdatedAt)
	if restoredAt.Valid {
		t := restoredAt.Time
		r.RestoredAt = &t
	}
	r.RoleRemoved, r.ConsentsRevoked, r.SessionsRevoked = StepStatus(role), StepStatus(consents), StepStatus(sessions)
	return r, err
}

func (p *PostgresRevocations) BeginRevocation(ctx context.Context, revocation Revocation) (Revocation, bool, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return Revocation{}, false, err
	}
	defer tx.Rollback()
	inserted, err := scanRevocation(tx.QueryRowContext(ctx, `INSERT INTO ihp_identity_sync_revocations
 (application_client_id, issuer, subject, revoked_by, reason)
 VALUES ($1, $2, $3, $4, $5)
 ON CONFLICT (application_client_id, issuer, subject) WHERE restored_at IS NULL DO NOTHING
 RETURNING `+revocationColumns,
		revocation.ApplicationClientID, revocation.Issuer, revocation.Subject, revocation.RevokedBy, revocation.Reason))
	if errors.Is(err, sql.ErrNoRows) {
		existing, err := scanRevocation(tx.QueryRowContext(ctx, `SELECT `+revocationColumns+` FROM ihp_identity_sync_revocations
 WHERE application_client_id = $1 AND issuer = $2 AND subject = $3 AND restored_at IS NULL`,
			revocation.ApplicationClientID, revocation.Issuer, revocation.Subject))
		if err != nil {
			return Revocation{}, false, mapRevocationError(err)
		}
		return existing, false, tx.Commit()
	}
	if err != nil {
		return Revocation{}, false, mapRevocationError(err)
	}
	if err := writeRevocationAudit(ctx, tx, revocation.RevokedBy, AuditRevocationRecorded, inserted, map[string]any{
		"reason": inserted.Reason,
	}); err != nil {
		return Revocation{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Revocation{}, false, err
	}
	return inserted, true, nil
}

func (p *PostgresRevocations) ActiveRevocation(ctx context.Context, application, issuer, subject string) (Revocation, error) {
	revocation, err := scanRevocation(p.DB.QueryRowContext(ctx, `SELECT `+revocationColumns+` FROM ihp_identity_sync_revocations
 WHERE application_client_id = $1 AND issuer = $2 AND subject = $3 AND restored_at IS NULL`, application, issuer, subject))
	if errors.Is(err, sql.ErrNoRows) {
		return Revocation{}, ErrRevocationNotFound
	}
	return revocation, err
}

func (p *PostgresRevocations) SaveRevocationAttempt(ctx context.Context, revocation Revocation, actor string) error {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	saved, err := scanRevocation(tx.QueryRowContext(ctx, `UPDATE ihp_identity_sync_revocations
 SET role_removed = $2, consents_revoked = $3, sessions_revoked = $4, attempts = $5, last_error = $6, updated_at = now()
 WHERE id = $1 AND restored_at IS NULL
 RETURNING `+revocationColumns,
		revocation.ID, string(revocation.RoleRemoved), string(revocation.ConsentsRevoked), string(revocation.SessionsRevoked),
		revocation.Attempts, nullable(revocation.LastError)))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRevocationNotFound
	}
	if err != nil {
		return mapRevocationError(err)
	}
	if err := writeRevocationAudit(ctx, tx, actor, AuditRevocationAttempted, saved, map[string]any{
		"roleRemoved": saved.RoleRemoved, "consentsRevoked": saved.ConsentsRevoked, "sessionsRevoked": saved.SessionsRevoked,
		"attempts": saved.Attempts, "lastError": saved.LastError, "complete": saved.Complete(),
		"sessionsStillOpen": saved.SessionsStillOpen(),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (p *PostgresRevocations) RestoreRevocation(ctx context.Context, application, issuer, subject, actor, reason string) (Revocation, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return Revocation{}, err
	}
	defer tx.Rollback()
	restored, err := scanRevocation(tx.QueryRowContext(ctx, `UPDATE ihp_identity_sync_revocations
 SET restored_by = $4, restore_reason = $5, restored_at = now(), updated_at = now()
 WHERE application_client_id = $1 AND issuer = $2 AND subject = $3 AND restored_at IS NULL
 RETURNING `+revocationColumns, application, issuer, subject, actor, reason))
	if errors.Is(err, sql.ErrNoRows) {
		return Revocation{}, ErrRevocationNotFound
	}
	if err != nil {
		return Revocation{}, mapRevocationError(err)
	}
	if err := writeRevocationAudit(ctx, tx, actor, AuditRevocationRestored, restored, map[string]any{"reason": reason}); err != nil {
		return Revocation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Revocation{}, err
	}
	return restored, nil
}

func (p *PostgresRevocations) ListRevocations(ctx context.Context, filter RevocationFilter) ([]Revocation, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var where []string
	var args []any
	if filter.Application != "" {
		args = append(args, filter.Application)
		where = append(where, fmt.Sprintf("application_client_id = $%d", len(args)))
	}
	if !filter.IncludeRestored || filter.OnlyUnfinished {
		where = append(where, "restored_at IS NULL")
	}
	if filter.OnlyUnfinished {
		where = append(where, `(role_removed IN ('pending','failed') OR consents_revoked IN ('pending','failed') OR sessions_revoked IN ('pending','failed'))`)
	}
	query := `SELECT ` + revocationColumns + ` FROM ihp_identity_sync_revocations`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY revoked_at, id LIMIT $%d", len(args))
	rows, err := p.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Revocation
	for rows.Next() {
		revocation, err := scanRevocation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, revocation)
	}
	return result, rows.Err()
}

func writeRevocationAudit(ctx context.Context, tx *sql.Tx, actor, action string, revocation Revocation, details map[string]any) error {
	details["subject"] = revocation.Subject
	details["issuer"] = revocation.Issuer
	details["at"] = time.Now().UTC().Format(time.RFC3339)
	details["requestId"] = observability.RequestIDFromContext(ctx)
	details["correlationId"] = observability.CorrelationIDFromContext(ctx)
	data, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_rbac_audit_events (actor_user_id, action, resource_type, resource_id, system_client_id, details)
 VALUES ($1, $2, 'identity_sync_revocation', $3, $4, $5::jsonb)`,
		nullable(actor), action, revocation.ID, revocation.ApplicationClientID, string(data))
	return err
}

func mapRevocationError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	switch pqErr.Code {
	case "23503": // foreign_key_violation: no integration for the application
		return ErrIntegrationNotFound
	case "23514": // check_violation
		return fmt.Errorf("%w: %s", ErrInvalidRevocation, pqErr.Constraint)
	}
	return err
}
