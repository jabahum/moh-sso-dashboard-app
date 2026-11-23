-- In-app deliveries have no worker: the stored notification row is the
-- delivery. They were created PENDING and never left that state. New in-app
-- deliveries are created SENT; settle the ones already stuck in a queue state.
UPDATE notification_deliveries
SET
    status = 'SENT',
    sent_at = COALESCE(sent_at, created_at),
    attempts = GREATEST(attempts, 1),
    locked_at = NULL,
    updated_at = now()
WHERE channel = 'in_app'
  AND status IN ('PENDING', 'RETRY', 'PROCESSING');
