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
	CreateArtifact(context.Context, Artifact) (Artifact, error)
	CreateDelivery(context.Context, string, string, string, string, string) error
	ListPortalReports(context.Context, string) ([]PortalReport, error)
}

type postgresRepository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &postgresRepository{db: db} }

const scheduleColumns = `id::text, health_bi_report_id, report_name, COALESCE(description,''), frequency,
	COALESCE(cron_expression,''), timezone, period_strategy, parameters, output_format, output_config, health_context, enabled,
	created_by, created_at, updated_at, last_run_at, next_run_at`

func scanSchedule(row interface{ Scan(...any) error }) (Schedule, error) {
	var value Schedule
	var parameters, outputConfig, healthContext []byte
	err := row.Scan(&value.ID, &value.HealthBIReportID, &value.ReportName, &value.Description,
		&value.Frequency, &value.CronExpression, &value.Timezone, &value.PeriodStrategy, &parameters,
		&value.OutputFormat, &outputConfig, &healthContext, &value.Enabled, &value.CreatedBy, &value.CreatedAt,
		&value.UpdatedAt, &value.LastRunAt, &value.NextRunAt)
	if err != nil {
		return Schedule{}, err
	}
	value.Parameters = map[string]any{}
	_ = json.Unmarshal(parameters, &value.Parameters)
	_ = json.Unmarshal(outputConfig, &value.OutputConfig)
	_ = json.Unmarshal(healthContext, &value.HealthContext)
	return value, nil
}

func (r *postgresRepository) CreateSchedule(ctx context.Context, input CreateScheduleRequest, userID string) (Schedule, error) {
	parameters, _ := json.Marshal(input.Parameters)
	outputConfig, _ := json.Marshal(input.OutputConfig)
	healthContext, _ := json.Marshal(input.HealthContext)
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	row := r.db.QueryRowContext(ctx, `INSERT INTO report_schedules
		(health_bi_report_id, report_name, description, frequency, cron_expression, timezone, period_strategy,
		 parameters, output_format, output_config, health_context, enabled, created_by)
		VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),$6,$7,$8::jsonb,$9,$10::jsonb,$11::jsonb,$12,$13)
		RETURNING `+scheduleColumns,
		input.HealthBIReportID, input.ReportName, input.Description, input.Frequency, input.CronExpression,
		input.Timezone, input.PeriodStrategy, string(parameters), input.OutputFormat, string(outputConfig),
		string(healthContext), enabled, userID)
	value, err := scanSchedule(row)
	if err != nil {
		return Schedule{}, err
	}
	if err := r.replaceRecipients(ctx, value.ID, input.Recipients); err != nil {
		return Schedule{}, err
	}
	value.Recipients, _ = r.listRecipients(ctx, value.ID)
	return value, nil
}

func (r *postgresRepository) ListSchedules(ctx context.Context, userID string, includeAll bool) ([]Schedule, error) {
	query := `SELECT ` + scheduleColumns + ` FROM report_schedules`
	args := []any{}
	if !includeAll {
		query += ` WHERE created_by = $1`
		args = append(args, userID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		value, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		value.Recipients, _ = r.listRecipients(ctx, value.ID)
		out = append(out, value)
	}
	return out, rows.Err()
}

func (r *postgresRepository) GetSchedule(ctx context.Context, id, userID string, includeAll bool) (Schedule, error) {
	query := `SELECT ` + scheduleColumns + ` FROM report_schedules WHERE id = $1::uuid`
	args := []any{id}
	if !includeAll {
		query += ` AND created_by = $2`
		args = append(args, userID)
	}
	value, err := scanSchedule(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Schedule{}, ErrScheduleNotFound
	}
	if err == nil {
		value.Recipients, _ = r.listRecipients(ctx, value.ID)
	}
	return value, err
}

func (r *postgresRepository) UpdateSchedule(ctx context.Context, id string, input UpdateScheduleRequest, userID string, includeAll bool) (Schedule, error) {
	parameters, _ := json.Marshal(input.Parameters)
	outputConfig, _ := json.Marshal(input.OutputConfig)
	healthContext, _ := json.Marshal(input.HealthContext)
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	query := `UPDATE report_schedules SET health_bi_report_id=$2, report_name=$3, description=NULLIF($4,''),
		frequency=$5, cron_expression=NULLIF($6,''), timezone=$7, period_strategy=$8, parameters=$9::jsonb,
		output_format=$10, output_config=$11::jsonb, health_context=$12::jsonb, enabled=$13, updated_at=now()
		WHERE id=$1::uuid`
	args := []any{id, input.HealthBIReportID, input.ReportName, input.Description, input.Frequency,
		input.CronExpression, input.Timezone, input.PeriodStrategy, string(parameters), input.OutputFormat,
		string(outputConfig), string(healthContext), enabled}
	if !includeAll {
		query += ` AND created_by=$14`
		args = append(args, userID)
	}
	query += ` RETURNING ` + scheduleColumns
	value, err := scanSchedule(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Schedule{}, ErrScheduleNotFound
	}
	if err != nil {
		return Schedule{}, err
	}
	if err := r.replaceRecipients(ctx, value.ID, input.Recipients); err != nil {
		return Schedule{}, err
	}
	value.Recipients, _ = r.listRecipients(ctx, value.ID)
	return value, nil
}

func (r *postgresRepository) DeleteSchedule(ctx context.Context, id, userID string, includeAll bool) error {
	query := `DELETE FROM report_schedules WHERE id=$1::uuid`
	args := []any{id}
	if !includeAll {
		query += ` AND created_by=$2`
		args = append(args, userID)
	}
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrScheduleNotFound
	}
	return err
}

func (r *postgresRepository) replaceRecipients(ctx context.Context, scheduleID string, recipients []ScheduleRecipient) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM report_schedule_recipients WHERE schedule_id=$1::uuid`, scheduleID); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if _, err := tx.ExecContext(ctx, `INSERT INTO report_schedule_recipients
			(schedule_id, recipient_type, recipient_value, delivery_channel) VALUES ($1::uuid,$2,$3,$4)`,
			scheduleID, recipient.Type, recipient.Value, recipient.DeliveryChannel); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *postgresRepository) listRecipients(ctx context.Context, scheduleID string) ([]ScheduleRecipient, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id::text, recipient_type, recipient_value, delivery_channel
		FROM report_schedule_recipients WHERE schedule_id=$1::uuid ORDER BY created_at`, scheduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ScheduleRecipient{}
	for rows.Next() {
		var value ScheduleRecipient
		if err := rows.Scan(&value.ID, &value.Type, &value.Value, &value.DeliveryChannel); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (r *postgresRepository) ListExecutions(ctx context.Context, userID string, includeAll bool) ([]Execution, error) {
	query := `SELECT e.id::text, e.schedule_id::text, COALESCE(e.health_bi_job_id,''), e.report_id, e.status,
		e.parameters, e.output_format, e.trigger_type, COALESCE(e.triggered_by,''), COALESCE(e.error_message,''),
		e.started_at, e.finished_at, e.created_at FROM report_executions e`
	args := []any{}
	if !includeAll {
		query += ` WHERE e.triggered_by=$1 OR EXISTS
			(SELECT 1 FROM report_schedules s WHERE s.id=e.schedule_id AND s.created_by=$1)`
		args = append(args, userID)
	}
	query += ` ORDER BY e.created_at DESC LIMIT 200`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Execution{}
	for rows.Next() {
		var value Execution
		var scheduleID sql.NullString
		var parameters []byte
		if err := rows.Scan(&value.ID, &scheduleID, &value.HealthBIJobID, &value.ReportID, &value.Status,
			&parameters, &value.OutputFormat, &value.TriggerType, &value.TriggeredBy, &value.ErrorMessage,
			&value.StartedAt, &value.FinishedAt, &value.CreatedAt); err != nil {
			return nil, err
		}
		if scheduleID.Valid {
			value.ScheduleID = &scheduleID.String
		}
		value.Parameters = map[string]any{}
		_ = json.Unmarshal(parameters, &value.Parameters)
		out = append(out, value)
	}
	return out, rows.Err()
}

func (r *postgresRepository) CreateArtifact(ctx context.Context, artifact Artifact) (Artifact, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO report_artifacts
		(execution_id,file_name,content_type,object_key,external_url,size_bytes)
		VALUES ($1::uuid,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),$6)
		RETURNING id::text, execution_id::text, file_name, COALESCE(content_type,''),
		          COALESCE(object_key,''), COALESCE(external_url,''), size_bytes, created_at`,
		artifact.ExecutionID, artifact.FileName, artifact.ContentType, artifact.ObjectKey, artifact.ExternalURL, artifact.SizeBytes)
	var out Artifact
	err := row.Scan(&out.ID, &out.ExecutionID, &out.FileName, &out.ContentType, &out.ObjectKey,
		&out.ExternalURL, &out.SizeBytes, &out.CreatedAt)
	return out, err
}

func (r *postgresRepository) CreateDelivery(ctx context.Context, executionID, artifactID, recipientType, recipientValue, channel string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO report_deliveries
		(execution_id,artifact_id,recipient_type,recipient_value,delivery_channel,status,sent_at)
		VALUES ($1::uuid,NULLIF($2,'')::uuid,$3,$4,$5,
		        CASE WHEN $5='portal' THEN 'sent' ELSE 'pending' END,
		        CASE WHEN $5='portal' THEN now() ELSE NULL END)`,
		executionID, artifactID, recipientType, recipientValue, channel)
	return err
}

func (r *postgresRepository) ListPortalReports(ctx context.Context, userID string) ([]PortalReport, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT d.id::text, e.id::text, e.report_id,
		COALESCE(s.report_name,e.report_id), a.id::text, a.execution_id::text, a.file_name,
		COALESCE(a.content_type,''), COALESCE(a.object_key,''), COALESCE(a.external_url,''),
		a.size_bytes, a.created_at, COALESCE(d.sent_at,d.updated_at,d.created_at)
		FROM report_deliveries d
		JOIN report_executions e ON e.id=d.execution_id
		LEFT JOIN report_schedules s ON s.id=e.schedule_id
		JOIN report_artifacts a ON a.id=d.artifact_id
		WHERE d.delivery_channel='portal' AND d.recipient_type='user'
		  AND d.recipient_value=$1 AND d.status='sent'
		ORDER BY d.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PortalReport{}
	for rows.Next() {
		var value PortalReport
		if err := rows.Scan(&value.DeliveryID, &value.ExecutionID, &value.ReportID, &value.ReportName,
			&value.Artifact.ID, &value.Artifact.ExecutionID, &value.Artifact.FileName, &value.Artifact.ContentType,
			&value.Artifact.ObjectKey, &value.Artifact.ExternalURL, &value.Artifact.SizeBytes,
			&value.Artifact.CreatedAt, &value.DeliveredAt); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}
