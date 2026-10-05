ALTER TABLE report_deliveries
    ADD COLUMN IF NOT EXISTS access_token_hash TEXT,
    ADD COLUMN IF NOT EXISTS access_token_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS access_token_used_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_deliveries_access_token_hash
    ON report_deliveries(access_token_hash)
    WHERE access_token_hash IS NOT NULL;

CREATE TABLE IF NOT EXISTS report_scheduler_runtime (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    worker_id TEXT,
    last_heartbeat_at TIMESTAMPTZ,
    last_cycle_started_at TIMESTAMPTZ,
    last_cycle_finished_at TIMESTAMPTZ,
    last_cycle_error TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO report_scheduler_runtime(singleton)
VALUES (TRUE)
ON CONFLICT (singleton) DO NOTHING;
