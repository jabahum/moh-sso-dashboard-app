# Application platform operations

## Ownership and support

The platform team owns the registry, identity boundary, shared notification API, audit and delivery workers. Each application team owns its OIDC client configuration, backend authorization, domain data, external sessions and support contact. Registry ownership/support URLs are required before activation. Route incidents first by application ID and registry owner, then escalate shared identity/queue/SMTP failures to the platform team.

## Health and application metrics

Existing `/health/live`, `/health/ready` and `/version` keep their existing deployment semantics. The platform adds:

- `GET /api/v1/platform/readiness`, authenticated with an active machine binding and `identity:read`. Returns `ready` plus worker health. HTTP 503 means a required worker is missing/stale; missing identity configuration/dependencies also fail closed. This endpoint requires authentication and should not replace process liveness.
- `GET /api/v1/admin/platform/applications/:clientId/operations`, requiring `rbac:read`. Returns application lifecycle/enabled status, binding counts, request/status counts over 24 hours, oldest outstanding request across all ages, and worker health. It exposes no recipients, message bodies, secrets or provider error payloads.

Worker heartbeats are written only after successful queue claims, including empty polls, and while processing batches. Required workers are `notification-email` and `email-outbox`. Health means a worker recently reached its queue, not that Keycloak/SMTP/mailboxes are healthy. The stale window is the larger of two minutes or three configured notification-worker intervals. Heartbeats aggregate replicas: one functioning worker of each kind is sufficient. A stopped process can retain a healthy heartbeat until the stale window expires.

`statusCounts24Hours` uses the same actual dispatch/email-outbox status calculation as the application status endpoint. Compute a completed-request failure ratio as FAILED/(FAILED+SENT), guarding a zero denominator. Monitor request rate, failed/retry counts and oldest pending age together. Configure alert thresholds against the agreed service objectives and normal application volume; an empty queue is not a failure. Initial operational checks should flag stale workers, growing backlog, persistent retries or any unexpected access-denial spike.

## Request and async correlation

HTTP logs include request ID, correlation ID, application ID (after authenticated binding), machine subject, route, response status and latency. Client-supplied trace IDs accept only 1–128 ASCII letters/digits/dot/underscore/colon/hyphen; invalid values are replaced. An unauthenticated request may have no attributable application ID.

Notification submissions retain request/correlation IDs in the application ledger and audit. They also pass application and trace IDs through the delivery payload and email-outbox metadata. Dispatch and SMTP attempts restore this context and emit identifier-only structured events. SMTP logs redact subjects/provider error text for platform messages, and platform transport failures are stored as safe diagnostics rather than provider-echoed content. Existing non-platform message behavior is preserved.

Use the request ID returned in the API response to locate the request log, `application.notification.queued` audit event, `notification_dispatch_attempt` and `email_delivery_attempt` events. The resource ID connects the attempt to existing delivery/email administration tools. Do not log access tokens, introspection secrets, message bodies or recipients in application-side integration code.

## Deployment and preflight

1. Back up the primary database and verify restore procedures. Read migrations 48–51 and maintain a rollback point.
2. Apply schema migrations before enabling new consumers. Migration 48 preserves existing enabled flags, while new discoveries/imports require activation. Migrations 49–50 add bindings and request ownership/idempotency; migration 51 adds worker heartbeat state.
3. Deploy backend and compatible frontend assets together where the registry UI changes. Existing portal authentication routes remain supported. The new service endpoints fail closed if the dedicated introspection client is missing.
4. Configure Keycloak audience/capability mappers, the introspection secret, service bindings and application ownership. Use a dedicated client without administrative roles. Helm exposes `backend.platformAuth.clientId`, `backend.platformAuth.audience`, and the `backend.secrets.PLATFORM_AUTH_CLIENT_SECRET` secret value; inject real values through your secret-management deployment path rather than committing them. Compose forwards the configured env file; the backend entrypoint also supports `PLATFORM_AUTH_CLIENT_SECRET_FILE` for a mounted secret supplied by an operator override. Verify issuer/external and internal token endpoints match the deployed realm and use protected transport.
5. Confirm the notification/email workers and SMTP configuration. Keep global process liveness independent from SMTP availability. Check platform readiness after at least one queue poll and then inspect per-application operations metrics.
6. Run the read-only integration verifier for the pilot. With explicit approval of the test recipient, submit a stable-key test message and poll its status. Verify launcher sign-in and a protected action with an authorized pilot user. Test a rejected application/role and check the returned request ID in logs.
7. Enable additional applications gradually. Compare failure ratio/backlog against baseline before expanding adoption.

For CI/local database verification run `make migrations-check`; it provisions disposable PostgreSQL and exercises fresh/previous-schema upgrade paths, registry lifecycle and platform integration checks. Run backend tests/vet/OpenAPI freshness and the Python example tests. Tests never load the application's normal database configuration or create live Keycloak grants.

## Failure handling

| Symptom | Check and action |
| --- | --- |
| 401 | Token type, issuer, platform audience, expiry/not-before and active state. Obtain a fresh access token. Never bypass validation. |
| 403 | Application active/enabled, bound service client/subject, token/registry scope intersection, or user application access role. Inspect governance audit. |
| 409 on submission | Key was used with another normalized payload. Reuse the original payload, or use a new key for a genuinely new operation. |
| 429 | Stop creating new requests until Retry-After. Safe retries retain the original key; do not evade the application quota using more clients. |
| 503 | Check introspection configuration/connectivity, database state, schema version and platform readiness. Fail closed in consumers. |
| Pending/Retry grows | Check worker heartbeats, queue availability, SMTP connection/configuration, bounded attempts and existing admin delivery/email status. |
| Dispatch SENT but message incomplete | Inspect overall API status and email-outbox administration. Dispatch success is only queue handoff. |
| EXPIRED | Historical queue data was cleaned up. Retained idempotency keys prevent re-delivery; do not recreate a key for an old operation. |

New platform delivery rows have a maximum-attempts value of five; current bootstrap workers also impose a three-attempt cap per stage. The effective limit is the smaller cap. Manual retry/cancel controls remain in the existing notification/email administration APIs; manual retries can duplicate an external delivery. Use business identifiers to reconcile before retrying. Never claim exactly-once email transport or mailbox/read receipts.

## Suspension and credential rotation

To stop new work, suspend the application through the registry lifecycle or disable its identity binding. Subsequent access decisions/submissions fail closed. Accepted queue work continues unless separately canceled/drained. Also revoke/rotate Keycloak credentials and end external application sessions when required by the incident. Registry suspension does not remotely erase independently managed sessions or rewrite roles in issued tokens.

Rotate a service client secret in Keycloak and update only that application's backend secret store. Test a fresh token and binding; no registry secret update is needed because secrets are not stored there. To replace the service client/subject, bind the replacement with minimum scopes, verify it, then disable the old binding. Do not move an existing OAuth client binding to another application. Rotate the platform introspection client through the deployment secret mechanism and verify new authenticated requests; no positive authorization cache needs clearing.

## Compatibility, retention and rollback

`/api/v1/platform` is the initial stable service contract. Add optional fields/capabilities compatibly; document changes and give consumers an agreed migration window before removal. Reject unknown scopes/contract versions rather than silently widening access. Hosted modules must continue using the public package/runtime contracts and compatible shell dependency versions.

Application idempotency mappings have no automatic expiry in v1. Retain them for all retry/reconciliation windows and include them in backups. Existing notification/email cleanup can remove transport details without removing the mapping; responses then become EXPIRED. Do not purge mappings while consumers may retry old keys. Set application-owned payload retention according to the actual data governance requirements; operational metrics and logs do not justify retaining message content indefinitely.

For rollback, stop producers, disable bindings as appropriate, reconcile/cancel/drain accepted queue work, and back up ownership/idempotency data. Roll back consumers/backend before dropping schemas they use. Dropping migration 51 only removes heartbeat telemetry; old workers do not depend on it. Dropping migration 50 removes ownership/idempotency mappings but leaves transport records, so preserve a backup and do not resume automatic retries blindly. Drop identity bindings only after consumers stop using them. The registry down migration retains disabled flags and audit history, but older seed/sync behavior can subsequently re-enable systems; supervise those jobs during recovery.
