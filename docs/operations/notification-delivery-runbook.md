# Notification delivery runbook

This runbook covers operational handling for notification delivery failures across in-app,
email, SMS, and future webhook channels. It is for operators and administrators with access to the delivery dashboard.

## Dashboard

Open:

```text
/portal/admin/notifications/deliveries
```

Required permissions:

- `notifications:read` to view deliveries and metrics
- `notifications:write` to retry, cancel, or send test SMS

## Common checks

1. Filter by `FAILED` and the affected channel.
2. Open the delivery detail panel.
3. Review:
   - delivery ID
   - notification ID
   - channel
   - provider
   - provider message ID
   - last error
   - provider metadata
4. Check backend logs using the delivery ID and notification ID.

Do not paste phone numbers, email addresses, or message bodies into tickets unless explicitly
approved by the incident process.

## Retry a delivery

Use retry when:

- status is `FAILED`, `RETRY`, `PENDING`, or `CANCELLED` (the dashboard disables the button for
  other statuses; the API itself rejects only `SENT`)
- the underlying provider/network issue has been corrected

The dashboard action calls:

```http
POST /api/v1/admin/notifications/deliveries/:deliveryID/retry
```

Retry moves the delivery to `RETRY` and schedules immediate processing. It does not change
`attempts`, which counts only attempts a worker started. If the delivery had already reached
`max_attempts`, it gets exactly one more attempt and returns to `FAILED` if that attempt fails.

## Cancel a delivery

Use cancel when:

- the message should not be delivered
- the delivery is still `PENDING`, `RETRY`, or `PROCESSING` (the dashboard disables the button for
  other statuses; the API itself rejects only `SENT`)

The dashboard action calls:

```http
POST /api/v1/admin/notifications/deliveries/:deliveryID/cancel
```

`SENT` deliveries cannot be retried or cancelled; the API returns `409 INVALID_DELIVERY_STATE`.

## Send a test SMS

Use test SMS after provider configuration changes:

1. Open the admin delivery dashboard.
2. Click **Send test SMS**.
3. Enter a test phone number and message.
4. Confirm a new SMS delivery appears (the test also creates an in-app delivery).
5. Confirm it transitions to `SENT`.

The request is rejected with `503 SMS_DISABLED` when `SMS_ENABLED=false`.

## Stuck processing deliveries

A worker holds a 15-minute lease on each delivery it claims. If the worker dies before finishing
(backend crash or restart mid-batch), the next poll after the lease expires reclaims the delivery
as its next attempt, or marks it `FAILED` with "Processing lease expired during the final attempt"
when that was its last attempt. The interrupted attempt may already have reached the provider, so
a reclaimed delivery can arrive twice.

If a delivery stays in `PROCESSING` for longer than about 15 minutes plus one poll interval:

1. Check if the backend worker is running (reclaim happens only when a worker for that channel
   polls; the SMS worker runs only with `SMS_ENABLED=true`).
2. Check backend logs around the last `locked_at` timestamp.
3. If the delivery must not be reprocessed, cancel it.
4. If the notification should no longer be sent, use cancel.

## Provider outage

1. Confirm provider status externally.
2. Avoid repeated manual retries during a confirmed outage.
3. Let retries accumulate as `RETRY` or `FAILED`.
4. When provider recovers, retry failed deliveries in batches.
5. Confirm delivery metrics improve.

## Metrics to watch

The metrics endpoint groups deliveries by channel and provider and returns:

- total deliveries and per-status counts (pending, processing, sent, failed, retry, cancelled)
- average processing duration (`sent_at` minus `created_at`, over sent deliveries)

Metric labels must remain low-cardinality. Never add phone numbers, emails, user IDs,
notification IDs, or provider message IDs as metric labels.

## Audit events

Expected audit events:

- `notification_delivery.retry`
- `notification_delivery.cancel`
- `notification_delivery.test_sms_queued`
- `notification_delivery.sms_sent`
- `notification_delivery.sms_retry`
- `notification_delivery.sms_failed`
- `ANNOUNCEMENT_PUBLISHED` with SMS queue metadata
- `ANNOUNCEMENT_SCHEDULED` with SMS metadata

Audit records intentionally do not include full SMS body content.
