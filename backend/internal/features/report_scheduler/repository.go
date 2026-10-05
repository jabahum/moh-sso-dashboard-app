package report_scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

var ErrScheduleNotFound = errors.New("report schedule not found")

type Repository interface {
	CreateSchedule(context.Context, CreateScheduleRequest, string) (Schedule, error)
	ListSchedules(context.Context, string, bool) ([]Schedule, error)
	GetSchedule(context.Context, string, string, bool) (Schedule, error)
	UpdateSchedule(context.Context, string, UpdateScheduleRequest, string, bool) (Schedule, error)
	DeleteSchedule(context.Context, string, string, bool) error
	ListExecutions(context.Context, string, bool) ([]Execution, error)
}

type postgresRepository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &postgresRepository{db: db} }

const scheduleColumns = `id::text, health_bi_report_id, report_name, COALESCE(description,''), frequency,
	COALESCE(cron_expression,''), timezone, period_strategy, parameters, output_format, enabled,
	created_by, created_at, updated_at, last_run_at, next_run_at`

func scanSchedule(row interface{ Scan(...any) error }) (Schedule, error) {
	var value Schedule
	var parameters []byte
	err := row.Scan(&value.ID, &value.HealthBIReportID, &value.ReportName, &value.Description,
		&value.Frequency, &value.CronExpression, &value.Timezone, &value.PeriodStrategy, &parameters,
		&value.OutputFormat, &value.Enabled, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
		&value.LastRunAt, &value.NextRunAt)
	if err != nil { return Schedule{}, err }
	value.Parameters = map[string]any{}
	if len(parameters) > 0 { _ = json.Unmarshal(parameters, &value.Parameters) }
	return value, nil
}

func (r *postgresRepository) CreateSchedule(ctx context.Context, input CreateScheduleRequest, userID string) (Schedule, error) {
	parameters, err := json.Marshal(input.Parameters)
	if err != nil { return Schedule{}, err }
	enabled := true
	if input.Enabled != nil { enabled = *input.Enabled }
	row := r.db.QueryRowContext(ctx, `INSERT INTO report_schedules
		(health_bi_report_id, report_name, description, frequency, cron_expression, timezone, period_strategy, parameters, output_format, enabled, created_by)
		VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),$6,$7,$8::jsonb,$9,$10,$11)
		RETURNING `+scheduleColumns, input.HealthBIReportID, input.ReportName, input.Description, input.Frequency,
		input.CronExpression, input.Timezone, input.PeriodStrategy, string(parameters), input.OutputFormat, enabled, userID)
	return scanSchedule(row)
}

func (r *postgresRepository) ListSchedules(ctx context.Context, userID string, includeAll bool) ([]Schedule, error) {
	query := `SELECT ` + scheduleColumns + ` FROM report_schedules`
	args := []any{}
	if !includeAll { query += ` WHERE created_by = $1`; args = append(args, userID) }
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() { value, err := scanSchedule(rows); if err != nil { return nil, err }; out = append(out, value) }
	return out, rows.Err()
}

func (r *postgresRepository) GetSchedule(ctx context.Context, id, userID string, includeAll bool) (Schedule, error) {
	query := `SELECT ` + scheduleColumns + ` FROM report_schedules WHERE id = $1::uuid`
	args := []any{id}
	if !includeAll { query += ` AND created_by = $2`; args = append(args, userID) }
	value, err := scanSchedule(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) { return Schedule{}, ErrScheduleNotFound }
	return value, err
}

func (r *postgresRepository) UpdateSchedule(ctx context.Context, id string, input UpdateScheduleRequest, userID string, includeAll bool) (Schedule, error) {
	parameters, err := json.Marshal(input.Parameters)
	if err != nil { return Schedule{}, err }
	enabled := true
	if input.Enabled != nil { enabled = *input.Enabled }
	query := `UPDATE report_schedules SET health_bi_report_id=$2, report_name=$3, description=NULLIF($4,''), frequency=$5,
		cron_expression=NULLIF($6,''), timezone=$7, period_strategy=$8, parameters=$9::jsonb, output_format=$10,
		enabled=$11, updated_at=now() WHERE id=$1::uuid`
	args := []any{id, input.HealthBIReportID, input.ReportName, input.Description, input.Frequency, input.CronExpression,
		input.Timezone, input.PeriodStrategy, string(parameters), input.OutputFormat, enabled}
	if !includeAll { query += ` AND created_by=$12`; args = append(args, userID) }
	query += ` RETURNING ` + scheduleColumns
	value, err := scanSchedule(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) { return Schedule{}, ErrScheduleNotFound }
	return value, err
}

func (r *postgresRepository) DeleteSchedule(ctx context.Context, id, userID string, includeAll bool) error {
	query := `DELETE FROM report_schedules WHERE id=$1::uuid`
	args := []any{id}
	if !includeAll { query += ` AND created_by=$2`; args = append(args, userID) }
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil { return err }
	count, err := result.RowsAffected()
	if err == nil && count == 0 { return ErrScheduleNotFound }
	return err
}

func (r *postgresRepository) ListExecutions(ctx context.Context, userID string, includeAll bool) ([]Execution, error) {
	query := `SELECT e.id::text, e.schedule_id::text, COALESCE(e.health_bi_job_id,''), e.report_id, e.status,
		e.parameters, e.output_format, e.trigger_type, COALESCE(e.triggered_by,''), COALESCE(e.error_message,''),
		e.started_at, e.finished_at, e.created_at FROM report_executions e`
	args := []any{}
	if !includeAll { query += ` WHERE e.triggered_by=$1 OR EXISTS (SELECT 1 FROM report_schedules s WHERE s.id=e.schedule_id AND s.created_by=$1)`; args = append(args, userID) }
	query += ` ORDER BY e.created_at DESC LIMIT 200`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []Execution{}
	for rows.Next() {
		var value Execution; var scheduleID sql.NullString; var parameters []byte
		if err := rows.Scan(&value.ID, &scheduleID, &value.HealthBIJobID, &value.ReportID, &value.Status, &parameters,
			&value.OutputFormat, &value.TriggerType, &value.TriggeredBy, &value.ErrorMessage, &value.StartedAt, &value.FinishedAt, &value.CreatedAt); err != nil { return nil, err }
		if scheduleID.Valid { value.ScheduleID = &scheduleID.String }
		value.Parameters = map[string]any{}; if len(parameters) > 0 { _ = json.Unmarshal(parameters, &value.Parameters) }
		out = append(out, value)
	}
	return out, rows.Err()
}
