package report_scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
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
	ListPortalReports(context.Context, string) ([]PortalReport, error)
	SetScheduleEnabled(context.Context, string, string, bool, bool) (Schedule, error)
	CreateExecution(context.Context, Schedule, string, string, time.Time, ResolvedPeriod, map[string]any) (Execution, error)
	ClaimDueExecution(context.Context, time.Time) (Schedule, Execution, bool, error)
	UpdateExecutionJob(context.Context, string, string, string, string) error
	ClaimGeneratingExecutions(context.Context, int) ([]Execution, error)
	GetExecution(context.Context, string) (Execution, error)
	GetExecutionDetail(context.Context, string, string, bool) (ExecutionDetail, error)
	MarkGenerationAttempt(context.Context, string) error
	ScheduleExecutionRetry(context.Context, string, string, time.Time) error
	ClaimRetryExecution(context.Context, time.Time) (Execution, bool, error)
	ResetExecutionForManualRetry(context.Context, string, string, bool) (Execution, error)
	CreatePendingDelivery(context.Context, string, string, string, string, string) (Delivery, error)
	MarkDeliverySent(context.Context, string) error
	ScheduleDeliveryRetry(context.Context, string, string, time.Time) error
	MarkDeliveryFailed(context.Context, string, string) error
	ClaimDueDelivery(context.Context, time.Time) (Delivery, bool, error)
	GetArtifact(context.Context, string) (Artifact, error)
	ClaimFailureNotification(context.Context, string) (bool, error)
	FinalizeExecutionDeliveryState(context.Context, string) (string, bool, error)
	ResetFailedDeliveries(context.Context, string) (int64, error)
	SchedulerOverview(context.Context, string, bool, HealthContext) (SchedulerOverview, error)
	GetArtifactForUser(context.Context, string, string, bool) (Artifact, Execution, error)
	SetDeliveryAccessToken(context.Context, string, string, time.Time) error
	GetDeliveryByAccessToken(context.Context, string) (Delivery, Artifact, Execution, error)
	MarkDeliveryAccessTokenUsed(context.Context, string) error
	UpdateWorkerHeartbeat(context.Context, string, time.Time, *time.Time, *time.Time, string) error
}

type postgresRepository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &postgresRepository{db: db} }

const scheduleColumns = `id::text, health_bi_report_id, report_name, COALESCE(description,''), frequency,
	COALESCE(cron_expression,''), timezone, period_strategy, parameters, output_format, output_config, timing, health_context, enabled,
	created_by, created_at, updated_at, last_run_at, next_run_at`

func scanSchedule(row interface{ Scan(...any) error }) (Schedule, error) {
	var value Schedule
	var parameters, outputConfig, timing, healthContext []byte
	err := row.Scan(&value.ID, &value.HealthBIReportID, &value.ReportName, &value.Description,
		&value.Frequency, &value.CronExpression, &value.Timezone, &value.PeriodStrategy, &parameters,
		&value.OutputFormat, &outputConfig, &timing, &healthContext, &value.Enabled, &value.CreatedBy, &value.CreatedAt,
		&value.UpdatedAt, &value.LastRunAt, &value.NextRunAt)
	if err != nil {
		return Schedule{}, err
	}
	value.Parameters = map[string]any{}
	_ = json.Unmarshal(parameters, &value.Parameters)
	_ = json.Unmarshal(outputConfig, &value.OutputConfig)
	_ = json.Unmarshal(timing, &value.Timing)
	_ = json.Unmarshal(healthContext, &value.HealthContext)
	return value, nil
}

func (r *postgresRepository) CreateSchedule(ctx context.Context, input CreateScheduleRequest, userID string) (Schedule, error) {
	parameters, _ := json.Marshal(input.Parameters)
	outputConfig, _ := json.Marshal(input.OutputConfig)
	timing, _ := json.Marshal(input.Timing)
	healthContext, _ := json.Marshal(input.HealthContext)
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	var nextRun any
	if enabled {
		computed, err := CalculateNextRun(input.Frequency, input.Timezone, input.Timing, time.Now())
		if err != nil { return Schedule{}, err }
		nextRun = computed
	}
	row := r.db.QueryRowContext(ctx, `INSERT INTO report_schedules
		(health_bi_report_id, report_name, description, frequency, cron_expression, timezone, period_strategy,
		 parameters, output_format, output_config, timing, health_context, enabled, created_by, next_run_at)
		VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),$6,$7,$8::jsonb,$9,$10::jsonb,$11::jsonb,$12::jsonb,$13,$14,$15)
		RETURNING `+scheduleColumns,
		input.HealthBIReportID, input.ReportName, input.Description, input.Frequency, input.CronExpression,
		input.Timezone, input.PeriodStrategy, string(parameters), input.OutputFormat, string(outputConfig),
		string(timing), string(healthContext), enabled, userID, nextRun)
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
	timing, _ := json.Marshal(input.Timing)
	healthContext, _ := json.Marshal(input.HealthContext)
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	var nextRun any
	if enabled {
		computed, err := CalculateNextRun(input.Frequency, input.Timezone, input.Timing, time.Now())
		if err != nil { return Schedule{}, err }
		nextRun = computed
	}
	query := `UPDATE report_schedules SET health_bi_report_id=$2, report_name=$3, description=NULLIF($4,''),
		frequency=$5, cron_expression=NULLIF($6,''), timezone=$7, period_strategy=$8, parameters=$9::jsonb,
		output_format=$10, output_config=$11::jsonb, timing=$12::jsonb, health_context=$13::jsonb, enabled=$14,
		next_run_at=$15, updated_at=now()
		WHERE id=$1::uuid`
	args := []any{id, input.HealthBIReportID, input.ReportName, input.Description, input.Frequency,
		input.CronExpression, input.Timezone, input.PeriodStrategy, string(parameters), input.OutputFormat,
		string(outputConfig), string(timing), string(healthContext), enabled, nextRun}
	if !includeAll {
		query += ` AND created_by=$16`
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
	query := `SELECT ` + executionColumns + ` FROM report_executions e`
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
		value, err := scanExecution(rows)
		if err != nil { return nil, err }
		out = append(out, value)
	}
	return out, rows.Err()
}

func (r *postgresRepository) CreateArtifact(ctx context.Context, artifact Artifact) (Artifact, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO report_artifacts
		(execution_id,file_name,content_type,object_key,external_url,size_bytes)
		VALUES ($1::uuid,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),$6)
		ON CONFLICT (execution_id) DO UPDATE SET file_name=EXCLUDED.file_name,
		content_type=EXCLUDED.content_type, object_key=EXCLUDED.object_key,
		external_url=EXCLUDED.external_url, size_bytes=EXCLUDED.size_bytes
		RETURNING id::text, execution_id::text, file_name, COALESCE(content_type,''),
		          COALESCE(object_key,''), COALESCE(external_url,''), size_bytes, created_at`,
		artifact.ExecutionID, artifact.FileName, artifact.ContentType, artifact.ObjectKey, artifact.ExternalURL, artifact.SizeBytes)
	var out Artifact
	err := row.Scan(&out.ID, &out.ExecutionID, &out.FileName, &out.ContentType, &out.ObjectKey,
		&out.ExternalURL, &out.SizeBytes, &out.CreatedAt)
	return out, err
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

func (r *postgresRepository) SetScheduleEnabled(ctx context.Context, id, userID string, includeAll, enabled bool) (Schedule, error) {
	current, err := r.GetSchedule(ctx, id, userID, includeAll)
	if err != nil { return Schedule{}, err }
	var nextRun any
	if enabled {
		computed, err := CalculateNextRun(current.Frequency, current.Timezone, current.Timing, time.Now())
		if err != nil { return Schedule{}, err }
		nextRun = computed
	}
	query := `UPDATE report_schedules SET enabled=$2, next_run_at=$3, updated_at=now() WHERE id=$1::uuid`
	args := []any{id, enabled, nextRun}
	if !includeAll { query += ` AND created_by=$4`; args = append(args, userID) }
	query += ` RETURNING ` + scheduleColumns
	value, err := scanSchedule(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) { return Schedule{}, ErrScheduleNotFound }
	if err == nil { value.Recipients, _ = r.listRecipients(ctx, value.ID) }
	return value, err
}

func scanExecution(row interface{ Scan(...any) error }) (Execution, error) {
	var value Execution
	var scheduleID sql.NullString
	var parameters, resolvedPeriod []byte
	err := row.Scan(&value.ID, &scheduleID, &value.HealthBIJobID, &value.ReportID, &value.Status,
		&parameters, &value.OutputFormat, &value.TriggerType, &value.TriggeredBy, &value.ErrorMessage,
		&value.StartedAt, &value.FinishedAt, &value.ScheduledFor, &resolvedPeriod,
		&value.GenerationAttempts, &value.MaxGenerationAttempts, &value.NextRetryAt, &value.LastAttemptAt, &value.CreatedAt)
	if err != nil { return Execution{}, err }
	if scheduleID.Valid { value.ScheduleID = &scheduleID.String }
	value.Parameters = map[string]any{}; _ = json.Unmarshal(parameters, &value.Parameters)
	if len(resolvedPeriod) > 0 { var period ResolvedPeriod; if json.Unmarshal(resolvedPeriod, &period) == nil { value.ResolvedPeriod = &period } }
	return value, nil
}

const executionColumns = `id::text, schedule_id::text, COALESCE(health_bi_job_id,''), report_id, status,
	parameters, output_format, trigger_type, COALESCE(triggered_by,''), COALESCE(error_message,''),
	started_at, finished_at, scheduled_for, COALESCE(resolved_period,'{}'::jsonb),
	generation_attempts, max_generation_attempts, next_retry_at, last_attempt_at, created_at`

func (r *postgresRepository) CreateExecution(ctx context.Context, schedule Schedule, triggerType, triggeredBy string, scheduledFor time.Time, period ResolvedPeriod, parameters map[string]any) (Execution, error) {
	paramsJSON, _ := json.Marshal(parameters); periodJSON, _ := json.Marshal(period)
	row := r.db.QueryRowContext(ctx, `INSERT INTO report_executions
		(schedule_id,report_id,status,parameters,output_format,trigger_type,triggered_by,scheduled_for,resolved_period,started_at)
		VALUES ($1::uuid,$2,'queued',$3::jsonb,$4,$5,NULLIF($6,''),$7,$8::jsonb,now()) RETURNING `+executionColumns,
		schedule.ID, schedule.HealthBIReportID, string(paramsJSON), schedule.OutputFormat, triggerType, triggeredBy, scheduledFor, string(periodJSON))
	return scanExecution(row)
}

func (r *postgresRepository) ClaimDueExecution(ctx context.Context, now time.Time) (Schedule, Execution, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil); if err != nil { return Schedule{}, Execution{}, false, err }; defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `SELECT `+scheduleColumns+` FROM report_schedules
		WHERE enabled=true AND next_run_at IS NOT NULL AND next_run_at <= $1 ORDER BY next_run_at
		FOR UPDATE SKIP LOCKED LIMIT 1`, now.UTC())
	schedule, err := scanSchedule(row)
	if errors.Is(err, sql.ErrNoRows) { return Schedule{}, Execution{}, false, nil }
	if err != nil { return Schedule{}, Execution{}, false, err }
	scheduledFor := now.UTC(); if schedule.NextRunAt != nil { scheduledFor = *schedule.NextRunAt }
	period, err := ResolveReportingPeriod(schedule.PeriodStrategy, schedule.Timezone, scheduledFor); if err != nil { return Schedule{}, Execution{}, false, err }
	parameters := buildExecutionParameters(schedule, period)
	paramsJSON, _ := json.Marshal(parameters); periodJSON, _ := json.Marshal(period)
	execRow := tx.QueryRowContext(ctx, `INSERT INTO report_executions
		(schedule_id,report_id,status,parameters,output_format,trigger_type,triggered_by,scheduled_for,resolved_period,started_at)
		VALUES ($1::uuid,$2,'queued',$3::jsonb,$4,'scheduled',$5,$6,$7::jsonb,now()) RETURNING `+executionColumns,
		schedule.ID, schedule.HealthBIReportID, string(paramsJSON), schedule.OutputFormat, schedule.CreatedBy, scheduledFor, string(periodJSON))
	execution, err := scanExecution(execRow); if err != nil { return Schedule{}, Execution{}, false, err }
	nextBase := scheduledFor
	if now.After(nextBase) { nextBase = now }
	nextRun, err := CalculateNextRun(schedule.Frequency, schedule.Timezone, schedule.Timing, nextBase); if err != nil { return Schedule{}, Execution{}, false, err }
	if _, err := tx.ExecContext(ctx, `UPDATE report_schedules SET last_run_at=$2, next_run_at=$3, updated_at=now() WHERE id=$1::uuid`, schedule.ID, scheduledFor, nextRun); err != nil { return Schedule{}, Execution{}, false, err }
	if err := tx.Commit(); err != nil { return Schedule{}, Execution{}, false, err }
	schedule.LastRunAt = &scheduledFor; schedule.NextRunAt = &nextRun; schedule.Recipients, _ = r.listRecipients(ctx, schedule.ID)
	return schedule, execution, true, nil
}

func (r *postgresRepository) UpdateExecutionJob(ctx context.Context, id, jobID, status, errorMessage string) error {
	finished := status == "completed" || status == "failed" || status == "cancelled"
	_, err := r.db.ExecContext(ctx, `UPDATE report_executions SET health_bi_job_id=NULLIF($2,''), status=$3,
		error_message=NULLIF($4,''), poll_claimed_at=NULL,
		finished_at=CASE WHEN $5 THEN now() ELSE finished_at END WHERE id=$1::uuid`, id, jobID, status, errorMessage, finished)
	return err
}

func (r *postgresRepository) ClaimGeneratingExecutions(ctx context.Context, limit int) ([]Execution, error) {
	if limit <= 0 { limit = 50 }
	rows, err := r.db.QueryContext(ctx, `UPDATE report_executions SET status='polling', poll_claimed_at=now()
		WHERE id IN (
			SELECT id FROM report_executions
			WHERE (status='generating' OR (status='polling' AND poll_claimed_at < now() - interval '2 minutes'))
			  AND health_bi_job_id IS NOT NULL
			ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
		) RETURNING `+executionColumns, limit)
	if err != nil { return nil, err }; defer rows.Close(); out := []Execution{}
	for rows.Next() { value, err := scanExecution(rows); if err != nil { return nil, err }; out = append(out, value) }
	return out, rows.Err()
}

func (r *postgresRepository) GetExecution(ctx context.Context, id string) (Execution, error) {
	value, err := scanExecution(r.db.QueryRowContext(ctx, `SELECT `+executionColumns+` FROM report_executions WHERE id=$1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) { return Execution{}, ErrScheduleNotFound }
	return value, err
}

func (r *postgresRepository) MarkGenerationAttempt(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE report_executions
		SET generation_attempts=generation_attempts+1, last_attempt_at=now(), next_retry_at=NULL,
		error_message=NULL, finished_at=NULL WHERE id=$1::uuid`, id)
	return err
}

func (r *postgresRepository) ScheduleExecutionRetry(ctx context.Context, id, message string, next time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE report_executions SET status='retrying', error_message=$2,
		next_retry_at=$3, poll_claimed_at=NULL, finished_at=NULL WHERE id=$1::uuid`, id, message, next.UTC())
	return err
}

func (r *postgresRepository) ClaimRetryExecution(ctx context.Context, now time.Time) (Execution, bool, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE report_executions SET status='queued', next_retry_at=NULL
		WHERE id=(SELECT id FROM report_executions WHERE status='retrying' AND next_retry_at <= $1
		ORDER BY next_retry_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+executionColumns, now.UTC())
	value, err := scanExecution(row)
	if errors.Is(err, sql.ErrNoRows) { return Execution{}, false, nil }
	return value, err == nil, err
}

func (r *postgresRepository) ResetExecutionForManualRetry(ctx context.Context, id, userID string, includeAll bool) (Execution, error) {
	query := `UPDATE report_executions SET status='queued', health_bi_job_id=NULL, error_message=NULL,
		next_retry_at=NULL, finished_at=NULL, poll_claimed_at=NULL, generation_attempts=0,
		last_attempt_at=NULL, failure_notified_at=NULL WHERE id=$1::uuid`
	args := []any{id}
	if !includeAll {
		query += ` AND (triggered_by=$2 OR EXISTS (SELECT 1 FROM report_schedules s WHERE s.id=report_executions.schedule_id AND s.created_by=$2))`
		args = append(args, userID)
	}
	query += ` RETURNING ` + executionColumns
	value, err := scanExecution(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) { return Execution{}, ErrScheduleNotFound }
	return value, err
}

func scanDelivery(row interface{ Scan(...any) error }) (Delivery, error) {
	var value Delivery
	var artifactID sql.NullString
	err := row.Scan(&value.ID, &value.ExecutionID, &artifactID, &value.RecipientType, &value.RecipientValue,
		&value.DeliveryChannel, &value.Status, &value.Attempts, &value.MaxAttempts, &value.LastError,
		&value.NextRetryAt, &value.LastAttemptAt, &value.SentAt, &value.CreatedAt, &value.UpdatedAt)
	if artifactID.Valid { value.ArtifactID = artifactID.String }
	return value, err
}

const deliveryColumns = `id::text, execution_id::text, artifact_id::text, recipient_type, recipient_value,
	delivery_channel, status, attempts, max_attempts, COALESCE(last_error,''), next_retry_at, last_attempt_at,
	sent_at, created_at, updated_at`

func (r *postgresRepository) CreatePendingDelivery(ctx context.Context, executionID, artifactID, recipientType, recipientValue, channel string) (Delivery, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO report_deliveries
		(execution_id,artifact_id,recipient_type,recipient_value,delivery_channel,status,next_retry_at)
		VALUES ($1::uuid,$2::uuid,$3,$4,$5,'pending',now())
		ON CONFLICT (execution_id,artifact_id,recipient_type,recipient_value,delivery_channel)
		DO UPDATE SET updated_at=report_deliveries.updated_at RETURNING `+deliveryColumns,
		executionID, artifactID, recipientType, recipientValue, channel)
	return scanDelivery(row)
}

func (r *postgresRepository) MarkDeliverySent(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE report_deliveries SET status='sent', attempts=attempts+1,
		last_attempt_at=now(), sent_at=now(), next_retry_at=NULL, last_error=NULL, updated_at=now() WHERE id=$1::uuid`, id)
	return err
}

func (r *postgresRepository) ScheduleDeliveryRetry(ctx context.Context, id, message string, next time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE report_deliveries SET status='retrying', attempts=attempts+1,
		last_attempt_at=now(), last_error=$2, next_retry_at=$3, updated_at=now() WHERE id=$1::uuid`, id, message, next.UTC())
	return err
}

func (r *postgresRepository) MarkDeliveryFailed(ctx context.Context, id, message string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE report_deliveries SET status='failed', attempts=attempts+1,
		last_attempt_at=now(), last_error=$2, next_retry_at=NULL, updated_at=now() WHERE id=$1::uuid`, id, message)
	return err
}

func (r *postgresRepository) ClaimDueDelivery(ctx context.Context, now time.Time) (Delivery, bool, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE report_deliveries SET status='sending', last_attempt_at=now()
		WHERE id=(SELECT delivery.id FROM report_deliveries delivery
		JOIN report_executions execution ON execution.id=delivery.execution_id
		WHERE execution.status='delivering' AND
		((delivery.status IN ('pending','retrying') AND delivery.next_retry_at <= $1)
		 OR (delivery.status='sending' AND delivery.last_attempt_at < now() - interval '2 minutes'))
		ORDER BY COALESCE(delivery.next_retry_at,delivery.last_attempt_at)
		FOR UPDATE OF delivery SKIP LOCKED LIMIT 1)
		RETURNING `+deliveryColumns, now.UTC())
	value, err := scanDelivery(row)
	if errors.Is(err, sql.ErrNoRows) { return Delivery{}, false, nil }
	return value, err == nil, err
}

func (r *postgresRepository) GetArtifact(ctx context.Context, id string) (Artifact, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id::text, execution_id::text, file_name, COALESCE(content_type,''),
		COALESCE(object_key,''), COALESCE(external_url,''), size_bytes, created_at FROM report_artifacts WHERE id=$1::uuid`, id)
	var out Artifact
	err := row.Scan(&out.ID, &out.ExecutionID, &out.FileName, &out.ContentType, &out.ObjectKey, &out.ExternalURL, &out.SizeBytes, &out.CreatedAt)
	return out, err
}

func (r *postgresRepository) GetExecutionDetail(ctx context.Context, id, userID string, includeAll bool) (ExecutionDetail, error) {
	execution, err := r.GetExecution(ctx, id)
	if err != nil { return ExecutionDetail{}, err }
	if !includeAll {
		var allowed bool
		err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM report_executions e LEFT JOIN report_schedules s ON s.id=e.schedule_id WHERE e.id=$1::uuid AND (e.triggered_by=$2 OR s.created_by=$2))`, id, userID).Scan(&allowed)
		if err != nil || !allowed { return ExecutionDetail{}, ErrScheduleNotFound }
	}
	detail := ExecutionDetail{Execution: execution, Artifacts: []Artifact{}, Deliveries: []Delivery{}}
	if execution.ScheduleID != nil { if schedule, err := r.GetSchedule(ctx, *execution.ScheduleID, userID, includeAll); err == nil { detail.Schedule = &schedule } }
	artifactRows, err := r.db.QueryContext(ctx, `SELECT id::text, execution_id::text, file_name, COALESCE(content_type,''), COALESCE(object_key,''), COALESCE(external_url,''), size_bytes, created_at FROM report_artifacts WHERE execution_id=$1::uuid ORDER BY created_at`, id)
	if err != nil { return ExecutionDetail{}, err }
	for artifactRows.Next() { var a Artifact; if err := artifactRows.Scan(&a.ID,&a.ExecutionID,&a.FileName,&a.ContentType,&a.ObjectKey,&a.ExternalURL,&a.SizeBytes,&a.CreatedAt); err != nil { artifactRows.Close(); return ExecutionDetail{}, err }; detail.Artifacts=append(detail.Artifacts,a) }
	artifactRows.Close()
	deliveryRows, err := r.db.QueryContext(ctx, `SELECT `+deliveryColumns+` FROM report_deliveries WHERE execution_id=$1::uuid ORDER BY created_at`, id)
	if err != nil { return ExecutionDetail{}, err }
	for deliveryRows.Next() { d, err := scanDelivery(deliveryRows); if err != nil { deliveryRows.Close(); return ExecutionDetail{}, err }; detail.Deliveries=append(detail.Deliveries,d) }
	deliveryRows.Close()
	return detail, nil
}

func (r *postgresRepository) ClaimFailureNotification(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE report_executions SET failure_notified_at=now()
		WHERE id=$1::uuid AND failure_notified_at IS NULL`, id)
	if err != nil { return false, err }
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (r *postgresRepository) FinalizeExecutionDeliveryState(ctx context.Context, executionID string) (string, bool, error) {
	var total, active, failed int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COUNT(*) FILTER (WHERE status IN ('pending','sending','retrying')),
		COUNT(*) FILTER (WHERE status='failed') FROM report_deliveries WHERE execution_id=$1::uuid`, executionID).Scan(&total,&active,&failed)
	if err != nil { return "", false, err }
	if active > 0 { return "delivering", false, nil }
	status := "completed"
	message := ""
	if failed > 0 { status = "failed"; message = "one or more report deliveries failed permanently" }
	result, err := r.db.ExecContext(ctx, `UPDATE report_executions SET status=$2, error_message=NULLIF($3,''),
		finished_at=now() WHERE id=$1::uuid AND status NOT IN ('completed','failed','cancelled')`, executionID, status, message)
	if err != nil { return "", false, err }
	rows, _ := result.RowsAffected()
	if total == 0 && rows == 0 { return status, false, nil }
	return status, rows > 0, nil
}

func (r *postgresRepository) ResetFailedDeliveries(ctx context.Context, executionID string) (int64, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE report_deliveries
		SET status='retrying', attempts=0, next_retry_at=now(), last_attempt_at=NULL,
		last_error=NULL, updated_at=now()
		WHERE execution_id=$1::uuid AND status='failed'`, executionID)
	if err != nil { return 0, err }
	count, err := result.RowsAffected()
	if err != nil { return 0, err }
	_, err = r.db.ExecContext(ctx, `UPDATE report_executions SET status='delivering', error_message=NULL,
		finished_at=NULL, failure_notified_at=NULL WHERE id=$1::uuid`, executionID)
	return count, err
}

func (r *postgresRepository) GetArtifactForUser(ctx context.Context, artifactID, userID string, includeAll bool) (Artifact, Execution, error) {
	query := `SELECT a.id::text, a.execution_id::text, a.file_name, COALESCE(a.content_type,''),
		COALESCE(a.object_key,''), COALESCE(a.external_url,''), a.size_bytes, a.created_at,
		e.id::text, e.schedule_id::text, COALESCE(e.health_bi_job_id,''), e.report_id, e.status,
		e.parameters, e.output_format, e.trigger_type, COALESCE(e.triggered_by,''), COALESCE(e.error_message,''),
		e.started_at, e.finished_at, e.scheduled_for, COALESCE(e.resolved_period,'{}'::jsonb),
		e.generation_attempts, e.max_generation_attempts, e.next_retry_at, e.last_attempt_at, e.created_at
		FROM report_artifacts a
		JOIN report_executions e ON e.id=a.execution_id
		LEFT JOIN report_schedules s ON s.id=e.schedule_id
		WHERE a.id=$1::uuid`
	args := []any{artifactID}
	if !includeAll {
		query += ` AND (e.triggered_by=$2 OR s.created_by=$2 OR EXISTS (
			SELECT 1 FROM report_deliveries d WHERE d.execution_id=e.id AND d.artifact_id=a.id
			AND d.recipient_type='user' AND d.recipient_value=$2 AND d.status='sent'))`
		args = append(args, userID)
	}
	row := r.db.QueryRowContext(ctx, query, args...)
	var artifact Artifact
	var execution Execution
	var scheduleID sql.NullString
	var parameters, resolvedPeriod []byte
	err := row.Scan(&artifact.ID,&artifact.ExecutionID,&artifact.FileName,&artifact.ContentType,&artifact.ObjectKey,&artifact.ExternalURL,&artifact.SizeBytes,&artifact.CreatedAt,
		&execution.ID,&scheduleID,&execution.HealthBIJobID,&execution.ReportID,&execution.Status,&parameters,&execution.OutputFormat,&execution.TriggerType,&execution.TriggeredBy,&execution.ErrorMessage,
		&execution.StartedAt,&execution.FinishedAt,&execution.ScheduledFor,&resolvedPeriod,&execution.GenerationAttempts,&execution.MaxGenerationAttempts,&execution.NextRetryAt,&execution.LastAttemptAt,&execution.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return Artifact{}, Execution{}, ErrScheduleNotFound }
	if err != nil { return Artifact{}, Execution{}, err }
	if scheduleID.Valid { execution.ScheduleID=&scheduleID.String }
	execution.Parameters=map[string]any{}; _=json.Unmarshal(parameters,&execution.Parameters)
	if len(resolvedPeriod)>0 { var period ResolvedPeriod; if json.Unmarshal(resolvedPeriod,&period)==nil { execution.ResolvedPeriod=&period } }
	return artifact, execution, nil
}

func (r *postgresRepository) SchedulerOverview(ctx context.Context, userID string, includeAll bool, health HealthContext) (SchedulerOverview, error) {
	var out SchedulerOverview
	scopeColumn := ""
	scopeValue := ""
	if includeAll {
		if strings.TrimSpace(health.Facility) != "" { scopeColumn="facility"; scopeValue=strings.TrimSpace(health.Facility) } else if strings.TrimSpace(health.District) != "" { scopeColumn="district"; scopeValue=strings.TrimSpace(health.District) }
	}
	scheduleWhere := ""
	scheduleArgs := []any{}
	if !includeAll { scheduleWhere=" WHERE created_by=$1"; scheduleArgs=append(scheduleArgs,userID) } else if scopeColumn!="" { scheduleWhere=" WHERE lower(COALESCE(health_context->>'"+scopeColumn+"',''))=lower($1)"; scheduleArgs=append(scheduleArgs,scopeValue) }
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE enabled=true) FROM report_schedules`+scheduleWhere, scheduleArgs...).Scan(&out.TotalSchedules,&out.EnabledSchedules); err != nil { return SchedulerOverview{},err }
	executionAccess := ""
	executionArgs := []any{}
	if !includeAll { executionAccess="(e.triggered_by=$1 OR EXISTS (SELECT 1 FROM report_schedules s WHERE s.id=e.schedule_id AND s.created_by=$1))"; executionArgs=append(executionArgs,userID) } else if scopeColumn!="" { executionAccess="EXISTS (SELECT 1 FROM report_schedules s WHERE s.id=e.schedule_id AND lower(COALESCE(s.health_context->>'"+scopeColumn+"',''))=lower($1))"; executionArgs=append(executionArgs,scopeValue) }
	execWhere := "WHERE e.created_at >= now() - interval '24 hours'"
	if executionAccess!="" { execWhere += " AND "+executionAccess }
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE e.status='completed'),COUNT(*) FILTER (WHERE e.status='failed') FROM report_executions e `+execWhere, executionArgs...).Scan(&out.Executions24h,&out.Completed24h,&out.Failed24h); err != nil { return SchedulerOverview{},err }
	retryWhere := "WHERE e.status='retrying'"
	if executionAccess!="" { retryWhere += " AND "+executionAccess }
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM report_executions e `+retryWhere, executionArgs...).Scan(&out.RetryingNow); err != nil { return SchedulerOverview{},err }
	deliveryAccess := ""
	deliveryArgs := []any{}
	if !includeAll { deliveryAccess="EXISTS (SELECT 1 FROM report_executions e LEFT JOIN report_schedules s ON s.id=e.schedule_id WHERE e.id=d.execution_id AND (e.triggered_by=$1 OR s.created_by=$1))"; deliveryArgs=append(deliveryArgs,userID) } else if scopeColumn!="" { deliveryAccess="EXISTS (SELECT 1 FROM report_executions e JOIN report_schedules s ON s.id=e.schedule_id WHERE e.id=d.execution_id AND lower(COALESCE(s.health_context->>'"+scopeColumn+"',''))=lower($1))"; deliveryArgs=append(deliveryArgs,scopeValue) }
	deliveryWhere := "WHERE d.status='failed' AND d.updated_at >= now() - interval '24 hours'"
	if deliveryAccess!="" { deliveryWhere += " AND "+deliveryAccess }
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM report_deliveries d `+deliveryWhere, deliveryArgs...).Scan(&out.DeliveryFailures24h); err != nil { return SchedulerOverview{},err }
	terminal:=out.Completed24h+out.Failed24h; if terminal>0 { out.SuccessRate24h=float64(out.Completed24h)*100/float64(terminal) }
	statusWhere := ""; if executionAccess!="" { statusWhere=" WHERE "+executionAccess }
	rows,err:=r.db.QueryContext(ctx, `SELECT e.status,COUNT(*) FROM report_executions e`+statusWhere+` GROUP BY e.status ORDER BY e.status`, executionArgs...); if err!=nil{return SchedulerOverview{},err}
	for rows.Next(){var v SchedulerStatusCount;if err:=rows.Scan(&v.Status,&v.Count);err!=nil{rows.Close();return SchedulerOverview{},err};out.ExecutionStatuses=append(out.ExecutionStatuses,v)};rows.Close()
	delStatusWhere:=""; if deliveryAccess!="" { delStatusWhere=" WHERE "+deliveryAccess }
	rows,err=r.db.QueryContext(ctx, `SELECT d.status,COUNT(*) FROM report_deliveries d`+delStatusWhere+` GROUP BY d.status ORDER BY d.status`, deliveryArgs...);if err!=nil{return SchedulerOverview{},err}
	for rows.Next(){var v SchedulerStatusCount;if err:=rows.Scan(&v.Status,&v.Count);err!=nil{rows.Close();return SchedulerOverview{},err};out.DeliveryStatuses=append(out.DeliveryStatuses,v)};rows.Close()
	failureWhere:="WHERE e.status='failed'"; if executionAccess!="" { failureWhere += " AND "+executionAccess }
	rows,err=r.db.QueryContext(ctx, `SELECT `+executionColumns+` FROM report_executions e `+failureWhere+` ORDER BY e.created_at DESC LIMIT 5`, executionArgs...);if err!=nil{return SchedulerOverview{},err}
	for rows.Next(){v,scanErr:=scanExecution(rows);if scanErr!=nil{rows.Close();return SchedulerOverview{},scanErr};out.RecentFailures=append(out.RecentFailures,v)};rows.Close()
	var heartbeat sql.NullTime; var cycleErr sql.NullString
	if err:=r.db.QueryRowContext(ctx, `SELECT last_heartbeat_at,last_cycle_error FROM report_scheduler_runtime WHERE singleton=true`).Scan(&heartbeat,&cycleErr);err==nil{if heartbeat.Valid{value:=heartbeat.Time;out.WorkerLastHeartbeatAt=&value;out.WorkerHealthy=time.Since(value)<=2*time.Minute};if cycleErr.Valid{out.WorkerLastCycleError=cycleErr.String}}
	out.GeneratedAt=time.Now().UTC();return out,nil
}

func (r *postgresRepository) SetDeliveryAccessToken(ctx context.Context, deliveryID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE report_deliveries
		SET access_token_hash=$2, access_token_expires_at=$3, access_token_used_at=NULL, updated_at=now()
		WHERE id=$1::uuid`, deliveryID, tokenHash, expiresAt.UTC())
	return err
}

func (r *postgresRepository) GetDeliveryByAccessToken(ctx context.Context, tokenHash string) (Delivery, Artifact, Execution, error) {
	row := r.db.QueryRowContext(ctx, `SELECT
		d.id::text,d.execution_id::text,d.artifact_id::text,d.recipient_type,d.recipient_value,d.delivery_channel,d.status,
		d.attempts,d.max_attempts,COALESCE(d.last_error,''),d.next_retry_at,d.last_attempt_at,d.sent_at,d.created_at,d.updated_at,
		a.id::text,a.execution_id::text,a.file_name,COALESCE(a.content_type,''),COALESCE(a.object_key,''),COALESCE(a.external_url,''),a.size_bytes,a.created_at,
		e.id::text,e.schedule_id::text,COALESCE(e.health_bi_job_id,''),e.report_id,e.status,e.parameters,e.output_format,e.trigger_type,
		COALESCE(e.triggered_by,''),COALESCE(e.error_message,''),e.started_at,e.finished_at,e.scheduled_for,COALESCE(e.resolved_period,'{}'::jsonb),
		e.generation_attempts,e.max_generation_attempts,e.next_retry_at,e.last_attempt_at,e.created_at
		FROM report_deliveries d
		JOIN report_artifacts a ON a.id=d.artifact_id
		JOIN report_executions e ON e.id=d.execution_id
		WHERE d.access_token_hash=$1 AND d.access_token_expires_at > now() AND d.status='sent'`, tokenHash)
	var delivery Delivery
	var artifact Artifact
	var execution Execution
	var artifactID, scheduleID sql.NullString
	var parameters,resolvedPeriod []byte
	err := row.Scan(
		&delivery.ID,&delivery.ExecutionID,&artifactID,&delivery.RecipientType,&delivery.RecipientValue,&delivery.DeliveryChannel,&delivery.Status,
		&delivery.Attempts,&delivery.MaxAttempts,&delivery.LastError,&delivery.NextRetryAt,&delivery.LastAttemptAt,&delivery.SentAt,&delivery.CreatedAt,&delivery.UpdatedAt,
		&artifact.ID,&artifact.ExecutionID,&artifact.FileName,&artifact.ContentType,&artifact.ObjectKey,&artifact.ExternalURL,&artifact.SizeBytes,&artifact.CreatedAt,
		&execution.ID,&scheduleID,&execution.HealthBIJobID,&execution.ReportID,&execution.Status,&parameters,&execution.OutputFormat,&execution.TriggerType,
		&execution.TriggeredBy,&execution.ErrorMessage,&execution.StartedAt,&execution.FinishedAt,&execution.ScheduledFor,&resolvedPeriod,
		&execution.GenerationAttempts,&execution.MaxGenerationAttempts,&execution.NextRetryAt,&execution.LastAttemptAt,&execution.CreatedAt,
	)
	if errors.Is(err,sql.ErrNoRows) { return Delivery{},Artifact{},Execution{},ErrScheduleNotFound }
	if err != nil { return Delivery{},Artifact{},Execution{},err }
	if artifactID.Valid { delivery.ArtifactID=artifactID.String }
	if scheduleID.Valid { execution.ScheduleID=&scheduleID.String }
	execution.Parameters=map[string]any{}; _=json.Unmarshal(parameters,&execution.Parameters)
	if len(resolvedPeriod)>0 { var period ResolvedPeriod; if json.Unmarshal(resolvedPeriod,&period)==nil { execution.ResolvedPeriod=&period } }
	return delivery,artifact,execution,nil
}

func (r *postgresRepository) MarkDeliveryAccessTokenUsed(ctx context.Context, deliveryID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE report_deliveries SET access_token_used_at=now(),updated_at=now() WHERE id=$1::uuid`, deliveryID)
	return err
}

func (r *postgresRepository) UpdateWorkerHeartbeat(ctx context.Context, workerID string, heartbeat time.Time, cycleStarted, cycleFinished *time.Time, cycleErr string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO report_scheduler_runtime
		(singleton,worker_id,last_heartbeat_at,last_cycle_started_at,last_cycle_finished_at,last_cycle_error,updated_at)
		VALUES (true,NULLIF($1,''),$2,$3,$4,NULLIF($5,''),now())
		ON CONFLICT(singleton) DO UPDATE SET
		worker_id=EXCLUDED.worker_id,last_heartbeat_at=EXCLUDED.last_heartbeat_at,
		last_cycle_started_at=COALESCE(EXCLUDED.last_cycle_started_at,report_scheduler_runtime.last_cycle_started_at),
		last_cycle_finished_at=COALESCE(EXCLUDED.last_cycle_finished_at,report_scheduler_runtime.last_cycle_finished_at),
		last_cycle_error=EXCLUDED.last_cycle_error,updated_at=now()`, workerID, heartbeat.UTC(), cycleStarted, cycleFinished, cycleErr)
	return err
}
