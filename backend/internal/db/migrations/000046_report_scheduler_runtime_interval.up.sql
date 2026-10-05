ALTER TABLE report_scheduler_runtime
    ADD COLUMN IF NOT EXISTS worker_interval_seconds INTEGER NOT NULL DEFAULT 30;
