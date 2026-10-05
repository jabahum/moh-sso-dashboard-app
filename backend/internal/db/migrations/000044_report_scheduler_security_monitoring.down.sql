DROP TABLE IF EXISTS report_scheduler_runtime;
DROP INDEX IF EXISTS uq_report_deliveries_access_token_hash;
ALTER TABLE report_deliveries DROP COLUMN IF EXISTS access_token_used_at;
ALTER TABLE report_deliveries DROP COLUMN IF EXISTS access_token_expires_at;
ALTER TABLE report_deliveries DROP COLUMN IF EXISTS access_token_hash;
