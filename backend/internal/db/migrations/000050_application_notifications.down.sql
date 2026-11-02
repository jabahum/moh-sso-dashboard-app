-- Existing queued notifications remain owned by the original application.
-- Cancel outstanding application deliveries before rolling back consumers.
DROP INDEX IF EXISTS email_outbox_notification_delivery_idx;
DROP TABLE ihp_application_notifications;
