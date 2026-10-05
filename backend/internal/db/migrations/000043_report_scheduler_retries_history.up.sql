ALTER TABLE report_executions
    ADD COLUMN IF NOT EXISTS generation_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_generation_attempts INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS failure_notified_at TIMESTAMPTZ;

ALTER TABLE report_deliveries
    ADD COLUMN IF NOT EXISTS max_attempts INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ;

UPDATE report_executions SET next_retry_at=now()
WHERE status='retrying' AND next_retry_at IS NULL;

UPDATE report_deliveries SET next_retry_at=now()
WHERE status IN ('pending','retrying') AND next_retry_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_report_executions_retry_due
    ON report_executions(next_retry_at)
    WHERE status='retrying' AND next_retry_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_deliveries_retry_due
    ON report_deliveries(next_retry_at)
    WHERE status IN ('pending','retrying') AND next_retry_at IS NOT NULL;

WITH ranked_artifacts AS (
    SELECT id,
           FIRST_VALUE(id) OVER (PARTITION BY execution_id ORDER BY created_at, id) AS keeper_id,
           ROW_NUMBER() OVER (PARTITION BY execution_id ORDER BY created_at, id) AS row_number
    FROM report_artifacts
)
UPDATE report_deliveries delivery
SET artifact_id = ranked.keeper_id
FROM ranked_artifacts ranked
WHERE ranked.row_number > 1
  AND delivery.artifact_id = ranked.id;

WITH ranked_artifacts AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY execution_id ORDER BY created_at, id) AS row_number
    FROM report_artifacts
)
DELETE FROM report_artifacts artifact
USING ranked_artifacts ranked
WHERE ranked.row_number > 1
  AND artifact.id = ranked.id;

WITH ranked_deliveries AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY execution_id, artifact_id, recipient_type, recipient_value, delivery_channel
               ORDER BY created_at, id
           ) AS row_number
    FROM report_deliveries
)
DELETE FROM report_deliveries delivery
USING ranked_deliveries ranked
WHERE ranked.row_number > 1
  AND delivery.id = ranked.id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_artifacts_execution
    ON report_artifacts(execution_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_delivery_target
    ON report_deliveries(execution_id, artifact_id, recipient_type, recipient_value, delivery_channel);
