DROP INDEX IF EXISTS idx_report_deliveries_recipient;
ALTER TABLE report_deliveries DROP COLUMN IF EXISTS delivery_channel;
ALTER TABLE report_deliveries DROP COLUMN IF EXISTS artifact_id;
DROP TABLE IF EXISTS report_artifacts;
DROP INDEX IF EXISTS uq_report_schedule_recipient_channel;
ALTER TABLE report_schedule_recipients DROP COLUMN IF EXISTS delivery_channel;
ALTER TABLE report_schedule_recipients
    ADD CONSTRAINT report_schedule_recipients_schedule_id_recipient_type_recipient_value_key
    UNIQUE (schedule_id, recipient_type, recipient_value);
ALTER TABLE report_schedules DROP COLUMN IF EXISTS output_config;
ALTER TABLE report_schedules DROP COLUMN IF EXISTS health_context;
