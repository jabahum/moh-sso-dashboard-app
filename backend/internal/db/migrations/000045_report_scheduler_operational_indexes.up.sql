CREATE INDEX IF NOT EXISTS idx_report_schedules_owner_enabled_created
    ON report_schedules(created_by, enabled, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_report_schedules_due_enabled
    ON report_schedules(next_run_at, id)
    WHERE enabled = TRUE AND next_run_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_schedules_health_district
    ON report_schedules(lower(COALESCE(health_context->>'district','')), created_at DESC);

CREATE INDEX IF NOT EXISTS idx_report_schedules_health_facility
    ON report_schedules(lower(COALESCE(health_context->>'facility','')), created_at DESC);

CREATE INDEX IF NOT EXISTS idx_report_executions_triggered_created
    ON report_executions(triggered_by, created_at DESC)
    WHERE triggered_by IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_executions_status_retry_due
    ON report_executions(status, next_retry_at, id)
    WHERE status='retrying' AND next_retry_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_executions_polling_due
    ON report_executions(status, poll_claimed_at, created_at, id)
    WHERE status IN ('generating','polling') AND health_bi_job_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_deliveries_work_queue
    ON report_deliveries(status, next_retry_at, last_attempt_at, id)
    WHERE status IN ('pending','retrying','sending');

CREATE INDEX IF NOT EXISTS idx_report_deliveries_portal_inbox
    ON report_deliveries(recipient_value, created_at DESC)
    WHERE delivery_channel='portal' AND recipient_type='user' AND status='sent';
