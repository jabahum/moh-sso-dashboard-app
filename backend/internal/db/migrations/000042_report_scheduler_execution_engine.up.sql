ALTER TABLE report_schedules
    ADD COLUMN IF NOT EXISTS timing JSONB NOT NULL DEFAULT '{"timeOfDay":"08:00"}'::jsonb;

ALTER TABLE report_executions
    ADD COLUMN IF NOT EXISTS scheduled_for TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolved_period JSONB,
    ADD COLUMN IF NOT EXISTS poll_claimed_at TIMESTAMPTZ;

ALTER TABLE report_executions DROP CONSTRAINT IF EXISTS report_executions_status_check;
ALTER TABLE report_executions ADD CONSTRAINT report_executions_status_check
    CHECK (status IN ('queued','generating','polling','generated','delivering','completed','failed','cancelled','retrying'));

UPDATE report_schedules
SET next_run_at = now()
WHERE enabled = TRUE AND next_run_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_executions_schedule_scheduled_for
    ON report_executions(schedule_id, scheduled_for)
    WHERE schedule_id IS NOT NULL AND scheduled_for IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_executions_active_jobs
    ON report_executions(status, created_at)
    WHERE status IN ('queued','generating','polling','generated','delivering','retrying');
