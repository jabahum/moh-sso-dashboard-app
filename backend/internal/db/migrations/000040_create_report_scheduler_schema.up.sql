CREATE TABLE IF NOT EXISTS report_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    health_bi_report_id TEXT NOT NULL,
    report_name TEXT NOT NULL,
    description TEXT,
    frequency TEXT NOT NULL,
    cron_expression TEXT,
    timezone TEXT NOT NULL DEFAULT 'Africa/Kampala',
    period_strategy TEXT NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_format TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_report_schedules_owner ON report_schedules (created_by, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_report_schedules_due ON report_schedules (next_run_at) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_report_schedules_health_bi_report ON report_schedules (health_bi_report_id);

CREATE TABLE IF NOT EXISTS report_schedule_recipients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL REFERENCES report_schedules(id) ON DELETE CASCADE,
    recipient_type TEXT NOT NULL CHECK (recipient_type IN ('user','group','email')),
    recipient_value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (schedule_id, recipient_type, recipient_value)
);

CREATE TABLE IF NOT EXISTS report_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID REFERENCES report_schedules(id) ON DELETE SET NULL,
    health_bi_job_id TEXT,
    report_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','generating','generated','delivering','completed','failed','cancelled','retrying')),
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_format TEXT NOT NULL,
    trigger_type TEXT NOT NULL DEFAULT 'scheduled' CHECK (trigger_type IN ('scheduled','manual')),
    triggered_by TEXT,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_report_executions_schedule ON report_executions (schedule_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_report_executions_job ON report_executions (health_bi_job_id) WHERE health_bi_job_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_report_executions_status ON report_executions (status, created_at DESC);

CREATE TABLE IF NOT EXISTS report_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES report_executions(id) ON DELETE CASCADE,
    recipient_type TEXT NOT NULL,
    recipient_value TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed','retrying')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_report_deliveries_execution ON report_deliveries (execution_id, created_at);
CREATE INDEX IF NOT EXISTS idx_report_deliveries_status ON report_deliveries (status, created_at);
