CREATE TABLE ihp_application_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_client_id TEXT NOT NULL REFERENCES ihp_systems(client_id),
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    notification_id UUID REFERENCES notifications(id) ON DELETE SET NULL,
    delivery_id UUID REFERENCES notification_deliveries(id) ON DELETE SET NULL,
    created_by TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(application_client_id, idempotency_key)
);
CREATE INDEX ihp_application_notifications_quota_idx ON ihp_application_notifications(application_client_id, created_at);
CREATE INDEX email_outbox_notification_delivery_idx ON email_outbox ((metadata->>'delivery_id')) WHERE metadata ? 'delivery_id';
