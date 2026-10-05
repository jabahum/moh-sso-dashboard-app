ALTER TABLE report_schedules
    ADD COLUMN IF NOT EXISTS health_context JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS output_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE report_schedule_recipients
    ADD COLUMN IF NOT EXISTS delivery_channel TEXT NOT NULL DEFAULT 'email';

ALTER TABLE report_schedule_recipients
    DROP CONSTRAINT IF EXISTS report_schedule_recipients_schedule_id_recipient_type_recipient_value_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_schedule_recipient_channel
    ON report_schedule_recipients (schedule_id, recipient_type, recipient_value, delivery_channel);

CREATE TABLE IF NOT EXISTS report_artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES report_executions(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,
    content_type TEXT,
    object_key TEXT,
    external_url TEXT,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (COALESCE(NULLIF(object_key, ''), NULLIF(external_url, '')) IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_report_artifacts_execution ON report_artifacts (execution_id, created_at DESC);

ALTER TABLE report_deliveries
    ADD COLUMN IF NOT EXISTS artifact_id UUID REFERENCES report_artifacts(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS delivery_channel TEXT NOT NULL DEFAULT 'email';

CREATE INDEX IF NOT EXISTS idx_report_deliveries_recipient
    ON report_deliveries (recipient_type, recipient_value, delivery_channel, created_at DESC);
