# Shared notification API v1

This first shared notification adapter delivers plain-text email through the existing notification-delivery and email-outbox workers. It does not expose arbitrary templates, attachments, HTML, SMS, webhooks, or broadcast role audiences. Existing admin notification features keep their current behavior.

## Submit

`POST /api/v1/platform/notifications`

Use a bound machine access token with `notifications:write` in both its Keycloak scope and registry grant. Send `Idempotency-Key` (1–128 letters/digits/dot/underscore/colon/hyphen, beginning with a letter/digit) and JSON:

```json
{
  "title": "Weekly report available",
  "message": "The test report is ready for review.",
  "recipient": {"email": "recipient@example.org", "name": "Report reviewer"}
}
```

The body is limited to 16 KiB, the title to 200 bytes, message to 10,000 bytes, name to 200 bytes, and email to 254 bytes. Titles/names cannot contain newlines; email must be a plain address. Leading/trailing whitespace is normalized. The authenticated binding determines application ownership; no caller-supplied application ID is used.

The first submission returns 202 with the status resource and a Location header. Repeating the same application/key and normalized payload returns 200, the same ID, and `replayed:true`; a different payload returns 409. Different applications can independently use the same key. Up to 60 new requests per application per rolling minute are accepted; excess returns 429 and Retry-After. Replays do not consume new-request quota. Database locks serialize the quota and idempotency decisions across backend replicas, independently of Redis availability.

The application request, notification, delivery queue row and audit event commit together. An audit/queue failure rolls back everything. Submission rechecks current lifecycle, identity binding and write grant under transaction locks, so an already-authenticated request cannot enqueue after a completed suspension/revocation.

## Inspect

`GET /api/v1/platform/notifications/:id`

Requires `notifications:read`. Responses expose identifiers, channel, status, dispatch status, attempts and creation time, without message/recipient/provider-error details. A request owned by another application returns 404. Operators retain their existing admin delivery tooling.

`status` follows the existing delivery queue until email handoff, then follows the email outbox. `dispatchStatus:SENT` means handed to the email queue; it is not a claim that SMTP sent the message. `status:SENT` reflects the email worker's completed send, not a mailbox delivery/read receipt. `dispatchAttempts` and `deliveryAttempts` report the two retry stages separately. Existing workers enforce bounded attempts, retry scheduling and failed-state handling. New rows cap attempts at five, while current bootstrap workers impose a three-attempt cap; the smaller cap applies at each stage.

If existing cleanup removes delivery/outbox data, status becomes `EXPIRED`. The application request/idempotency mapping remains, and replay never recreates the message. There is no automatic expiry of idempotency keys in v1; operators must preserve these mappings for the required retry lifetime.

API retries do not duplicate queue inserts. The underlying worker/SMTP pipeline remains at-least-once: a worker crash after an external send or queue handoff can lead to duplicate transport delivery. Consumers must not treat email as an exactly-once business transaction.

## Operations and migration

Apply migration 50 after identity migration 49. Enable the existing notification-delivery and email workers and configure SMTP before expecting delivery progress. Suspension prevents new submissions; previously accepted queue entries continue unless an operator cancels them through existing delivery controls. Never place patient details, tokens or secrets in operational audit fields.

The database integration tests cover concurrent replay, changed payload conflict, cross-application lookup, real outbox state, lifecycle/binding revocation, quota, retention and audit rollback. Run `make migrations-check` with Docker; this also runs in the existing migration CI job.

Before rollback, stop producers and cancel/drain outstanding application deliveries, then roll back API consumers and migration 50. Queue records survive rollback, but the application ownership/idempotency table does not; preserve a database backup if rollback must retain those guarantees.
