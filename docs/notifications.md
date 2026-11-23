# Notifications

This page describes the notification and delivery model, the admin delivery dashboard, its API, and the audit and metrics rules. It is for developers and administrators; operators handling failures should use the [Notification Delivery Runbook](./operations/notification-delivery-runbook.md).

The portal notification system stores a notification record and one or more delivery records.
The notification record is the user-facing message; delivery records track channel-specific work
such as in-app display, email, SMS, or future webhook delivery.

## Delivery channels

- `in_app`: stored for the portal notification center. No worker processes these records: the
  stored notification is the delivery, so the delivery is created `SENT` (one attempt, `sent_at`
  set) rather than left `PENDING`.
- `email`: processed by the notification email delivery worker, which hands the message to the
  email outbox; the delivery is marked `SENT` with provider `email_queue` and provider status
  `QUEUED` once queued, and the outbox worker sends it later.
- `sms`: processed by the notification SMS delivery worker.
- `webhook`: reserved for future integrations; no worker processes it.

Both delivery workers run only when `NOTIFICATION_DELIVERY_WORKER_ENABLED=true` (the default) and
the backend is not running in core-only mode; the SMS worker additionally requires
`SMS_ENABLED=true`. They poll every `NOTIFICATION_DELIVERY_WORKER_INTERVAL` (default `3s`) and claim
up to `NOTIFICATION_DELIVERY_BATCH_SIZE` (default `20`) due deliveries per channel
(`backend/internal/bootstrap/workers.go`, `backend/internal/config/config.go`).

## Delivery statuses

- `PENDING`: queued and ready when `scheduled_at` is due.
- `PROCESSING`: claimed by a worker, which holds a 15-minute lease on it (`locked_at`).
- `SENT`: provider accepted or completed the delivery (for email: queued to the email outbox).
- `FAILED`: attempt number `max_attempts` (or a later one, after a manual retry) failed, or the
  worker's lease expired during that final attempt.
- `RETRY`: an attempt failed with attempts remaining, or an admin queued a manual retry.
- `CANCELLED`: manually cancelled.

A delivery's `max_attempts` defaults to 5 when the caller does not set one (test SMS uses 3).
`attempts` counts delivery attempts a worker has started: claiming a delivery increments it, and
marking it `RETRY`, `FAILED` or `SENT` does not, so a delivery sent first time shows 1. After a
failed attempt the worker schedules the next one 30 seconds, 2 minutes, then 5 minutes later
(`retryDelay` in `backend/internal/worker/notification_delivery_worker.go`).

A delivery left in `PROCESSING` (for example after a backend crash or restart mid-batch) is
reclaimed once its lease expires, 15 minutes after it was claimed (`ProcessingLease` in
`backend/internal/repository/notification_delivery`). The interrupted attempt counts: the reclaim
starts the next attempt, and a delivery whose lease expired during its final attempt is marked
`FAILED` instead of being reclaimed. Because the worker cannot tell whether the interrupted attempt
reached the provider, a reclaimed delivery can be delivered twice.

## Admin dashboard

Admins with `notifications:read` can open:

```text
/portal/admin/notifications/deliveries
```

The dashboard supports:

- filtering by channel and status
- searching by recipient JSON, delivery ID, notification ID, or provider message ID
- viewing delivery details and safe provider metadata
- retrying `FAILED`, `RETRY`, `PENDING` or `CANCELLED` deliveries
- cancelling `PENDING`, `RETRY` or `PROCESSING` deliveries
- sending a test SMS

Retries, cancellations and test SMS require `notifications:write`.

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
- `search` (matches delivery ID, notification ID, recipient JSON or provider message ID)
- `limit` (default 20)
- `offset` (default 0)

The API is less restrictive than the dashboard buttons: retry and cancel accept any delivery that
is not `SENT`; a `SENT` delivery returns `409 INVALID_DELIVERY_STATE`. A manual retry sets the
delivery to `RETRY`, due immediately, without changing `attempts` (a manual retry is not an
attempt), so a delivery that had already reached `max_attempts` gets exactly one more attempt and
becomes `FAILED` again if it fails. Retry, cancel and test SMS
are also rate limited by the notifications write policy.

Delivery responses include the recipient, template data and payload (so phone numbers, email
addresses and message bodies are visible to `notifications:read` holders) plus provider metadata.
For SMS, the stored provider metadata is limited to provider, message ID and status. Audit logs
must not contain secrets, credentials, full provider authentication responses or message bodies.

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
