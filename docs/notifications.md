# Notifications

This page describes the notification and delivery model, the admin delivery dashboard, its API, and the audit and metrics rules. It is for developers and administrators; operators handling failures should use the [Notification Delivery Runbook](./operations/notification-delivery-runbook.md).

The portal notification system stores a notification record and one or more delivery records.
The notification record is the user-facing message; delivery records track channel-specific work
such as in-app display, email, SMS, or future webhook delivery.

## Delivery channels

- `in_app`: stored for the portal notification center.
- `email`: processed by the notification email delivery worker.
- `sms`: processed by the notification SMS delivery worker.
- `webhook`: reserved for future integrations.

## Delivery statuses

- `PENDING`: queued and ready when `scheduled_at` is due.
- `PROCESSING`: claimed by a worker.
- `SENT`: provider accepted or completed the delivery.
- `FAILED`: exhausted retry attempts.
- `RETRY`: failed once and scheduled for another attempt.
- `CANCELLED`: manually cancelled before completion.

## Admin dashboard

Admins with `notifications:read` can open:

```text
/portal/admin/notifications/deliveries
```

The dashboard supports:

- filtering by channel and status
- searching by recipient JSON, delivery ID, notification ID, or provider message ID
- viewing delivery details and safe provider metadata
- retrying retryable deliveries
- cancelling pending/retry/processing deliveries
- sending a test SMS

Retries and cancellations require `notifications:write`.

## API

```http
GET /api/v1/admin/notifications/deliveries
GET /api/v1/admin/notifications/deliveries/metrics
GET /api/v1/admin/notifications/deliveries/:deliveryID
POST /api/v1/admin/notifications/deliveries/:deliveryID/retry
POST /api/v1/admin/notifications/deliveries/:deliveryID/cancel
POST /api/v1/admin/notifications/test-sms
```

Supported delivery list query parameters:

- `channel`
- `status`
- `search`
- `limit`
- `offset`

Provider metadata is returned only as operational delivery metadata. The API must not expose
secrets, credentials, full provider authentication responses, or message bodies in audit logs.

## Audit

The backend records audit events for:

- manual delivery retry
- manual delivery cancellation
- test SMS queueing
- SMS sent by worker
- SMS retry by worker
- SMS failure by worker
- announcement publish/schedule with SMS queue metadata

Audit metadata intentionally excludes full SMS body content and phone numbers.

## Metrics

The operational metrics endpoint returns low-cardinality delivery aggregates grouped by channel
and provider. It is safe for dashboards because labels are limited to channel, provider, and
status-derived counts.

Do not add phone numbers, user IDs, emails, notification IDs, or provider message IDs as metric labels.

## Related docs

- [SMS Delivery](./sms-delivery.md)
- [Notification Delivery Runbook](./operations/notification-delivery-runbook.md)
