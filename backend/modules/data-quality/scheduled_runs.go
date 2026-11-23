package dataquality

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Scheduled runs: a scan queued to execute later. A run started immediately does
// not go through here — it is executed inline and only the result is stored.

const scheduledRunsSchemaDDL = `
CREATE TABLE IF NOT EXISTS dqa.scheduled_runs (
    id           BIGSERIAL PRIMARY KEY,
    table_id     TEXT        NOT NULL,
    scope        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    scheduled_at TIMESTAMPTZ NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','running','done','failed','cancelled')),
    created_by   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    run_id       BIGINT,
    error        TEXT
);
CREATE INDEX IF NOT EXISTS idx_scheduled_runs_due ON dqa.scheduled_runs (scheduled_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_scheduled_runs_table ON dqa.scheduled_runs (table_id, created_at DESC);
`

var errScheduledRunNotFound = errors.New("scheduled run not found")

type ScheduledRun struct {
	ID          int64      `json:"id"`
	TableID     string     `json:"table_id"`
	Scope       RunScope   `json:"scope"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	Status      string     `json:"status"`
	CreatedBy   string     `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	RunID       *int64     `json:"run_id,omitempty"`
	Error       string     `json:"error,omitempty"`
}

const scheduledRunColumns = `id, table_id, scope, scheduled_at, status,
	COALESCE(created_by, ''), created_at, started_at, finished_at, run_id, COALESCE(error, '')`

func scanScheduledRun(row interface{ Scan(...any) error }) (ScheduledRun, error) {
	var s ScheduledRun
	var scopeRaw []byte
	if err := row.Scan(&s.ID, &s.TableID, &scopeRaw, &s.ScheduledAt, &s.Status, &s.CreatedBy,
		&s.CreatedAt, &s.StartedAt, &s.FinishedAt, &s.RunID, &s.Error); err != nil {
		return ScheduledRun{}, err
	}
	if len(scopeRaw) > 0 {
		_ = json.Unmarshal(scopeRaw, &s.Scope)
	}
	return s, nil
}

// ScheduleRun queues a scan. The scope is validated against the table's config
// now so a bad scope fails at scheduling time rather than silently at midnight.
func (s *DQAStore) ScheduleRun(ctx context.Context, tableID string, scope RunScope, at time.Time, createdBy string) (ScheduledRun, error) {
	cfg, found, err := s.GetTable(ctx, tableID)
	if err != nil {
		return ScheduledRun{}, err
	}
	if !found {
		return ScheduledRun{}, fmt.Errorf("table_id %q is not registered", tableID)
	}
	if _, err := scope.Predicate(cfg); err != nil {
		return ScheduledRun{}, err
	}

	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return ScheduledRun{}, err
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO dqa.scheduled_runs (table_id, scope, scheduled_at, created_by)
		VALUES ($1,$2::jsonb,$3,$4)
		RETURNING `+scheduledRunColumns,
		tableID, string(scopeJSON), at.UTC(), nullIfEmpty(createdBy))
	return scanScheduledRun(row)
}

func (s *DQAStore) ListScheduledRuns(ctx context.Context, status string, limit, offset int) ([]ScheduledRun, error) {
	q := `SELECT ` + scheduledRunColumns + ` FROM dqa.scheduled_runs`
	args := []any{}
	if status != "" {
		args = append(args, status)
		q += fmt.Sprintf(` WHERE status = $%d`, len(args))
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(` ORDER BY scheduled_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ScheduledRun{}
	for rows.Next() {
		run, err := scanScheduledRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// CancelScheduledRun cancels a run that has not started.
func (s *DQAStore) CancelScheduledRun(ctx context.Context, id int64) (ScheduledRun, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE dqa.scheduled_runs SET status = 'cancelled', finished_at = now()
		WHERE id = $1 AND status = 'pending'
		RETURNING `+scheduledRunColumns, id)
	run, err := scanScheduledRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ScheduledRun{}, errScheduledRunNotFound
	}
	return run, err
}

// claimDueRun takes one due run, marking it running. SKIP LOCKED keeps two
// backends from claiming the same row.
func (s *DQAStore) claimDueRun(ctx context.Context, now time.Time) (ScheduledRun, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE dqa.scheduled_runs SET status = 'running', started_at = now()
		WHERE id = (
			SELECT id FROM dqa.scheduled_runs
			WHERE status = 'pending' AND scheduled_at <= $1
			ORDER BY scheduled_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1)
		RETURNING `+scheduledRunColumns, now.UTC())
	run, err := scanScheduledRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ScheduledRun{}, false, nil
	}
	if err != nil {
		return ScheduledRun{}, false, err
	}
	return run, true, nil
}

func (s *DQAStore) finishScheduledRun(ctx context.Context, id int64, runID int64, runErr error) error {
	if runErr != nil {
		_, err := s.db.ExecContext(ctx, `
			UPDATE dqa.scheduled_runs SET status = 'failed', finished_at = now(), error = $2
			WHERE id = $1`, id, runErr.Error())
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE dqa.scheduled_runs SET status = 'done', finished_at = now(), run_id = $2
		WHERE id = $1`, id, runID)
	return err
}
