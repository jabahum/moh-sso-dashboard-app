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

- status is `FAILED`, `RETRY`, `PENDING`, or `CANCELLED`
- the underlying provider/network issue has been corrected

The dashboard action calls:

```http
POST /api/v1/admin/notifications/deliveries/:deliveryID/retry
```

Retry moves the delivery to `RETRY` and schedules immediate processing.

## Cancel a delivery

Use cancel when:

- the message should not be delivered
- the delivery is still `PENDING`, `RETRY`, or `PROCESSING`

The dashboard action calls:

```http
POST /api/v1/admin/notifications/deliveries/:deliveryID/cancel
```

`SENT` deliveries cannot be cancelled.

## Send a test SMS

Use test SMS after provider configuration changes:

1. Open the admin delivery dashboard.
2. Click **Send test SMS**.
3. Enter a test phone number and message.
4. Confirm a new SMS delivery appears.
5. Confirm it transitions to `SENT`.

## Stuck processing deliveries

If a delivery stays in `PROCESSING`:

1. Check if the backend worker is running.
2. Check backend logs around the last `locked_at` timestamp.
3. If the delivery is safe to reprocess, use retry.
4. If the notification should no longer be sent, use cancel.

## Provider outage

1. Confirm provider status externally.
2. Avoid repeated manual retries during a confirmed outage.
3. Let retries accumulate as `RETRY` or `FAILED`.
4. When provider recovers, retry failed deliveries in batches.
5. Confirm delivery metrics improve.

## Metrics to watch

- total deliveries by channel
- failed SMS deliveries by provider
- retry SMS deliveries by provider
- average processing duration by channel/provider

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
