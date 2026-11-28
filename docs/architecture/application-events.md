# Application change events: design

**Status: proposed.** For review under issue #398. Nothing here is built yet. The
decision is recorded in [section 10](#10-decision) once reviewed.

This page decides how the portal tells application backends about changes they must
act on, such as a person losing access, without the applications polling every
endpoint and without adding a message broker before one is needed.

## 1. What happens today

The portal notifies an application in exactly one case:

| Change | Where it happens | What the application hears |
| --- | --- | --- |
| A person's `<app>_access` role is removed, for an application with identity sync | `users.Service.clientRolesRemoved` → `identityreconcile.PortalRevocationHook` → `RevokeService.Revoke` | `POST /sessions/revoke` on the application, synchronously in the admin's request |
| The role is granted again | `PortalRevocationHook` restores the revocation row | Nothing |
| A user is disabled or enabled | `users.Service.SetUserEnabled` (Keycloak) | Nothing |
| An application is suspended, retired or reactivated | `rbac` lifecycle transition (`TransitionSystem`) | Nothing. Its platform API calls start failing |
| A person's health-context assignments change | `health_context.ReplaceUserAssignments` / `ReplaceGroupAssignments` | Nothing |

Three weaknesses in the one existing path:

- **It depends on the portal reaching the application.** Connecting CLIMS in production
  failed because the portal server's internal DNS had no record for
  `clims.health.go.ug`. Portal → application is the fragile direction. Applications
  already have to reach the portal for sign-in and the platform API.
- **Failures wait for a person.** The connector retries up to 5 times within the
  request. After that, the step stays `failed` in `ihp_identity_sync_revocations`
  until an operator runs `identity-sync revocations --resume`. No background worker
  retries it.
- **It covers only identity-sync applications and only removal.** Disabling a user,
  for example, doesn't end their sessions in any application.

Useful building blocks already exist:

- Two transactional outboxes (`email_outbox`, `notification_deliveries`) claimed with
  `FOR UPDATE SKIP LOCKED`. The `notification_deliveries.channel` column already allows
  `webhook`, though nothing delivers it.
- Lifecycle changes and revocations write `ihp_rbac_audit_events` in the same
  transaction as the change.
- Workers run under `core.StartWorkerGroup` with heartbeats in
  `ihp_platform_worker_heartbeats`.
- Applications already call the platform API with a bound machine client and
  catalogue scopes ([platform API](../integrations/platform-api.md)).

## 2. Events applications need

| Event type | When | Sent to | Why the application cares | Priority |
| --- | --- | --- | --- | --- |
| `access.revoked` | A person no longer holds `<app>_access` (removed directly, through a group, or by identity-sync revoke) | That application | End their sessions, stop scheduled work on their behalf | 1 |
| `access.restored` | The role is granted again | That application | Re-enable anything suspended on revoke | 1 |
| `user.disabled` | The portal disables a user | Every application where they hold the access role | End sessions everywhere at once | 1 |
| `user.enabled` | The portal re-enables a user | Same | Re-enable | 1 |
| `application.lifecycle_changed` | The application itself is suspended, retired, reactivated or reset to configured | That application | Show a maintenance notice, stop calling the platform API, alert its operators | 2 |
| `health_context.assignments_changed` | A person's health contexts (data scopes) change | Applications where they hold the access role | Refresh cached data scopes | 3 |

Events carry **identifiers, not personal data**: the person's Keycloak issuer and
subject (the same pair identity sync links on), never names or emails. The application
looks up what it needs.

## 3. Options

### A. Pull: an event feed applications read (recommended first step)

The portal writes each event to an append-only table in the same transaction as the
change, where possible. Applications read their own events with the platform API they
already use:

```http
GET /api/v1/platform/events?after=<cursor>&limit=100
Authorization: Bearer <machine token with events:read>
```

- **Network:** only application → portal, which already has to work. No inbound
  firewall rules, DNS records or TLS certificates at the application, and no URLs in
  the portal's registry.
- **Reliability:** if an application is down, its events wait. It catches up from its
  last cursor. Nothing has to be retried by the portal.
- **Latency:** the poll interval, for example 15 to 30 seconds. Acceptable for
  everything above except ending sessions instantly, which identity sync's
  synchronous `POST /sessions/revoke` keeps doing (section 6).
- **Cost to applications:** a small polling loop and a stored cursor. The Go client
  provides both.

### B. Push: signed webhooks fed by an outbox

The portal stores a URL and signing secret per application and a worker POSTs each
event, signed (for example [Standard Webhooks](https://www.standardwebhooks.com/)
HMAC-SHA256 headers), with retries and backoff.

- Lowest latency, and familiar to integrators.
- **Brings back the fragile direction:** each application needs a public or internal
  DNS name the portal can resolve, an open path and a trusted certificate, exactly what
  blocked CLIMS.
- The portal must store and rotate signing secrets, run a delivery worker, and show a
  delivery log with redelivery. Applications must verify signatures, reject replays and
  deduplicate.
- The unused `webhook` channel of `notification_deliveries` is not a good fit: that
  table is shaped around one notification to one person (recipient, payload, provider
  metadata), not machine events to applications.

### C. A message broker (NATS, RabbitMQ, Kafka)

- Fan-out, replay and back-pressure built in.
- A new stateful service to run, secure, back up and monitor, plus broker credentials
  and network access for every application. Not justified for a handful of events per
  minute across a few applications.

### Comparison

| | A. Feed | B. Webhooks | C. Broker |
| --- | --- | --- | --- |
| Network direction | App → portal (already required) | Portal → app (fragile here) | Both → broker |
| New infrastructure | One table, one endpoint | Table, worker, secrets, delivery UI | A broker cluster |
| Missed events | App resumes from its cursor | Portal retries; app deduplicates | Broker retains |
| Latency | Poll interval (seconds) | Sub-second | Sub-second |
| Work for each application | Poll loop and cursor (Go client provides) | Public endpoint, signature checks, dedupe | Broker client and credentials |
| Authentication | Existing machine client and scopes | New per-app signing secrets | New broker credentials |

## 4. Recommended design

**Start with A. Keep identity sync's synchronous revoke for instant session ending.
Add B later only for an application that needs sub-second delivery and can be reached
reliably.** Both share the same event table, so B can be added without changing how
events are produced.

### 4.1 Event table

```sql
CREATE TABLE ihp_application_events (
    sequence              BIGSERIAL PRIMARY KEY,       -- the feed cursor; ordered per application
    event_id              UUID NOT NULL UNIQUE,        -- idempotency key for consumers
    application_client_id TEXT NOT NULL REFERENCES ihp_systems(client_id),
    type                  TEXT NOT NULL,
    subject_issuer        TEXT,                        -- the person, when the event is about one
    subject               TEXT,
    data                  JSONB NOT NULL DEFAULT '{}',
    occurred_at           TIMESTAMPTZ NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON ihp_application_events (application_client_id, sequence);
```

- **One row per receiving application.** A `user.disabled` for someone with access to
  three applications writes three rows, so each feed is a simple indexed range scan.
- **Written in the same transaction as the change** wherever the change is a portal
  database write: lifecycle transitions (next to `system.lifecycle.changed`),
  revocations (next to `revocation_recorded`) and health-context assignments.
- **Keycloak-first changes** (removing a role, disabling a user) write the event right
  after Keycloak succeeds. A crash between the two could lose an event. The existing
  revocation row already narrows this for access removal, and a periodic job can
  reconcile access-role holders later if needed (open question 3).
- **Retention:** 30 days, removed by a daily cleanup task. A cursor older than that gets
  `410 Gone` (section 4.3).

### 4.2 Event envelope

Fields follow [CloudEvents 1.0](https://cloudevents.io/) names so the format is
familiar and a later webhook or broker binding needs no new shape:

```json
{
  "specversion": "1.0",
  "id": "5d0c9a3e-…",
  "sequence": "18452",
  "type": "access.revoked",
  "source": "https://portals.health.go.ug",
  "time": "2026-11-28T09:15:02Z",
  "application": "alerts",
  "subject": { "issuer": "https://auth.health.go.ug/realms/MoH", "sub": "ab96ef82-…" },
  "data": { "role": "alerts_access", "reason": "removed" }
}
```

| Type | `data` |
| --- | --- |
| `access.revoked` | `role`, `reason` (`removed`, `revoked_by_identity_sync`, `user_disabled`) |
| `access.restored` | `role` |
| `user.disabled` / `user.enabled` | none |
| `application.lifecycle_changed` | `from`, `to`: lifecycle statuses (`registered`, `configured`, `validated`, `approved`, `active`, `suspended`, `retired`) |
| `health_context.assignments_changed` | none: call the portal for the current assignments |

Rules for consumers, stated in the contract:

- **At least once.** Deduplicate by `id`.
- **Ordered per application** by `sequence`. Process in order; don't assume gaps mean
  loss (rolled-back transactions leave gaps).
- **Events are hints, the portal is the truth.** For access, re-check with
  `GET /platform/applications/{clientId}/access` when the user next acts. For health
  contexts, fetch the current assignments.
- **Unknown types are ignored**, so new types are additive.

### 4.3 Feed endpoint

`GET /api/v1/platform/events?after=<sequence>&limit=<1..500>`, scope `events:read`
(a new catalogue scope, machine tokens only):

- Returns the caller's application's events with `sequence > after`, oldest first, and
  `next` (the last sequence returned). An empty list means caught up.
- No `after` returns events from the start of retention. To start from now, use
  `GET /api/v1/platform/events/latest`, which returns the current cursor.
- `after` older than retention → `410 Gone` with code `EVENTS_EXPIRED`. The application
  then resynchronizes (for access: treat cached sessions as stale and re-check with the
  access endpoint) and restarts from `latest`.
- Same authentication, application binding and lifecycle rules as every platform
  endpoint. A suspended application can't read its feed, but its events are kept and
  readable after reactivation within retention.
- Each read records the application's cursor and time (`ihp_application_event_cursors`),
  so operators can see lag.

### 4.4 Go client

```go
events, next, err := portal.Events(ctx, cursor, 100)

// or let the client poll, deduplicate and checkpoint:
poller := platform.NewEventPoller(portal, store, 20*time.Second)
err := poller.Run(ctx, func(ctx context.Context, e platform.Event) error {
	switch e.Type {
	case "access.revoked", "user.disabled":
		return sessions.EndAll(ctx, e.Subject.Issuer, e.Subject.Sub)
	}
	return nil // ignore unknown types
})
```

`store` is a small interface (`Load`, `Save` the cursor) the application implements
with its own database, so the cursor advances in the same transaction as its handling
when it wants exactly-once effects.

### 4.5 Admin visibility

The Machine clients tab gains a per-application events panel: subscribed (has
`events:read`), last read, cursor lag (events not yet read) and the last 50 events. A
worker check marks an application **lagging** when it hasn't read for longer than a
threshold while events are waiting, and creates an admin notification, like incomplete
revocations do today.

## 5. Where events are produced

| Event | Producer | Transaction |
| --- | --- | --- |
| `access.revoked` | `users.Service.clientRolesRemoved`, for every application whose access role was removed (not only identity-sync ones); `RevokeService.BeginRevocation` for identity-sync revokes | After Keycloak; with the revocation row when there is one |
| `access.restored` | `users.Service.clientRolesAdded` and `PortalRevocationHook` restore | After Keycloak |
| `user.disabled` / `user.enabled` | `users.Service.SetUserEnabled`, fanned out to applications where the user holds the access role (looked up before disabling) | After Keycloak |
| `application.lifecycle_changed` | `rbac` `TransitionSystem`, and the lifecycle guard's `invalidated` reset | Same transaction |
| `health_context.assignments_changed` | `health_context` `ReplaceUserAssignments` / `ReplaceGroupAssignments` (fanned out from group to members) | Same transaction |

Producers call one function, `events.Record(ctx, tx, []Event)`, so adding an event type
is one call site plus a contract entry.

## 6. Relationship to identity-sync revocation

- **Keep `POST /sessions/revoke`.** It ends sessions within seconds and is already in
  the identity-sync contract. Events add the cases it doesn't cover (other applications,
  disabled users, restores) and a durable record.
- **Close its manual-resume gap separately:** a background worker that retries
  unfinished revocations with backoff (the same claim pattern as the outboxes). This is
  a small, independent follow-up and doesn't need the event feed.
- An identity-sync application that also reads the feed sees `access.revoked` after it
  has already ended the sessions. That's harmless because handling is idempotent.

## 7. Security

- Applications read only their own events; the scope and binding checks are the same as
  every platform endpoint.
- Events hold the issuer and subject, a type and minimal data. No names, emails, tokens
  or health data.
- Access to the feed is revocable by disabling the binding or removing `events:read`,
  like any other scope.
- For webhooks (option B, later): HMAC-SHA256 signatures with a timestamp and event ID
  (Standard Webhooks headers), replay window of 5 minutes, secrets stored as references
  (`keycloak:` / `file:`) like identity-sync credentials, HTTPS only, and the same
  private-address policy and network check as identity sync.

## 8. Contract and compatibility

- The feed is ordinary HTTP, so it goes in the **platform API contract**
  (`backend/core/platform/contract/platform.openapi.yaml`) with the event envelope and
  each `data` shape as schemas. The existing compatibility check then guards it.
- New event types and new optional `data` fields are additive. Changing a type's
  meaning or removing a field needs a new type name (for example `access.revoked.v2`),
  not a silent change.
- AsyncAPI is only worth adding if option B is built.

## 9. Delivery plan

| Step | Work | Depends on |
| --- | --- | --- |
| 1 | Background retry for unfinished identity-sync revocations | None |
| 2 | `ihp_application_events` table, `events.Record`, retention cleanup, cursor table | None |
| 3 | Producers for `access.revoked`, `access.restored`, `user.disabled`, `user.enabled` | 2 |
| 4 | Feed endpoint, `events:read` scope, contract schemas, docs | 2 |
| 5 | Go client `Events` and `EventPoller` | 4 |
| 6 | Producers for `application.lifecycle_changed` and `health_context.assignments_changed` | 2 |
| 7 | Events panel and lag alert in the Machine clients tab | 4 |
| 8 | Webhooks (option B), only if an application needs sub-second delivery | 2, 4 |

Steps 1 and 2 can start in parallel. Steps 2 to 5 deliver the first useful version.

## 10. Decision

*To be recorded after review:* the chosen option, who approved it, the date, and links
to the follow-up issues.

## 11. Open questions

1. **Poll interval and retention.** Are 15 to 30 seconds and 30 days right for MoH
   applications? Retention bounds how long an application can be down and still catch
   up.
2. **Which applications first?** CLIMS and POES already end sessions through identity
   sync. Alerts Manager, which isn't built yet, could adopt the feed from the start.
3. **Keycloak-first changes.** Is a small chance of a lost event after a crash
   acceptable, given that events are hints and access is re-checked? If not, add a
   periodic reconciliation of access-role holders.
4. **Group-driven access changes.** Removing someone from a Keycloak group outside the
   portal changes their access without passing through the portal. Should a periodic
   job detect these, or is "changes made in the portal" enough for now?
5. **Consent.** Should applications need an explicit subscription, or does holding
   `events:read` mean subscribed? This design assumes the scope is the subscription.
