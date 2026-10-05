DROP INDEX IF EXISTS idx_report_executions_active_jobs;
DROP INDEX IF EXISTS uq_report_executions_schedule_scheduled_for;
ALTER TABLE report_executions DROP CONSTRAINT IF EXISTS report_executions_status_check;
ALTER TABLE report_executions ADD CONSTRAINT report_executions_status_check
    CHECK (status IN ('queued','generating','generated','delivering','completed','failed','cancelled','retrying'));
ALTER TABLE report_executions DROP COLUMN IF EXISTS resolved_period;
ALTER TABLE report_executions DROP COLUMN IF EXISTS scheduled_for;
ALTER TABLE report_executions DROP COLUMN IF EXISTS poll_claimed_at;
ALTER TABLE report_schedules DROP COLUMN IF EXISTS timing;
