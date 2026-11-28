# Platform API for application backends

This page is for teams whose backend lives in another repository (CLIMS, POES,
Alerts Manager and others) and needs to call the portal. It covers everything in one
place: getting a token, being allowed to call, each endpoint, errors, and the rules
that keep integrations simple. The authorization model behind it is defined in the
[application identity and authorization contract](../platform-identity.md). The
machine-readable contract to build and test clients against is
[`backend/core/platform/contract/platform.openapi.yaml`](../../backend/core/platform/contract/platform.openapi.yaml),
generated from the portal's routes and checked in CI.

The portal also calls *your* backend through small contracts it defines, such as
[identity sync](identity-sync-contract.md). That direction is covered by the
[client onboarding guide](client-onboarding.md), not here.

## 1. The rules

1. **Hub and spoke.** Call the portal's platform API for shared needs: who is calling,
   whether a user still has access to your application, sending notifications. Don't
   route calls to another application through the portal; if you need another
   application's data, call that application's own API with the same token model.
2. **One authentication model.** Every call carries a Bearer access token from the
   portal's Keycloak realm. No API keys, shared secrets, cookies, shared databases or
   portal administrator credentials.
3. **Capabilities are granted.** Each endpoint needs a scope. A portal administrator
   grants scopes to your machine client, with a reason; they can be revoked at once.
4. **Fail closed.** If the portal can't answer, refuse the protected operation rather
   than allowing it.
5. **Additive changes only within `v1`.** See [section 8](#8-versioning-and-compatibility).

## 2. Endpoints at a glance

All paths are under `https://<portal>/api/v1` (production:
`https://portals.health.go.ug/api/v1`).

| Method and path | Caller | Scope | Purpose |
| --- | --- | --- | --- |
| `GET /platform/identity` | Machine client | `identity:read` | Who the portal thinks you are, and your effective scopes |
| `GET /platform/readiness` | Machine client | `identity:read` | Whether the portal and its background workers are healthy |
| `POST /platform/notifications` | Machine client | `notifications:write` | Send an email notification to one recipient |
| `GET /platform/notifications/{id}` | Machine client | `notifications:read` | Delivery status of a notification you sent |
| `GET /platform/applications/{clientId}/access` | A user's token, forwarded | `identity:read` in the user's token | The user's current roles and permissions for your application |

The notification routes exist only when the portal's delivery feature is enabled.

## 3. Before your first call

### 3.1 What the portal operator sets up once

The platform endpoints check every token with Keycloak token introspection. The
portal needs:

| Setting | Purpose |
| --- | --- |
| `PLATFORM_AUTH_CLIENT_ID` / `PLATFORM_AUTH_CLIENT_SECRET` (or `_FILE`) | A dedicated confidential client the portal uses to call introspection |
| `PLATFORM_AUTH_AUDIENCE` | The audience every token must carry. Default `moh-platform-api` |

Without the introspection credentials every platform endpoint answers **503**, while
the rest of the portal keeps working. Details are in
[Platform server setup](../platform-identity.md#platform-server-setup).

In Keycloak, the operator also creates:

- an **audience mapper** that adds the platform audience (`moh-platform-api`) to the
  tokens your clients receive;
- a **client scope for each capability** (`identity:read`, `notifications:read`,
  `notifications:write`) with **Include in token scope** enabled, assigned to the
  clients that may request it.

### 3.2 Your machine client

Create one confidential Keycloak client per backend and purpose, named
`<app>-<purpose>` (for example `alerts-platform`):

- **Service accounts** enabled; standard flow, direct access grants and implicit flow
  disabled.
- No portal or administrator roles.
- The capability client scopes it needs, and the platform audience mapper.

Its service-account user is `service-account-<clientId>`. Note that user's Keycloak
ID; it is the `subject` in the next step.

### 3.3 Bind the client to your application

Your application must already be registered in the portal, **active** and
**enabled**. A portal administrator with `rbac:write` and `rbac:roles:write` binds the
client and grants scopes:

```http
PUT /api/v1/admin/platform/applications/alerts/identities/alerts-platform
Content-Type: application/json

{
  "subject": "<Keycloak ID of service-account-alerts-platform>",
  "scopes": ["identity:read", "notifications:write", "notifications:read"],
  "enabled": true,
  "reason": "CHG-1234: Alerts backend sends notifications"
}
```

- A client can be bound to only one application, and the binding can't be moved.
- To revoke, send the same request with `"enabled": false` (or empty `scopes`). The
  next request fails; nothing is cached.
- Every change and its reason is audited. The registry never stores your secret.

The administrator can list bindings with
`GET /api/v1/admin/platform/applications/{clientId}/identities`.

## 4. Getting a token

Request a token from Keycloak with the client-credentials grant, from your backend
only:

```bash
curl -s https://auth.health.go.ug/realms/MoH/protocol/openid-connect/token \
  -d grant_type=client_credentials \
  -d client_id=alerts-platform \
  --data-urlencode client_secret@/run/secrets/alerts_platform_secret \
  -d 'scope=identity:read notifications:write notifications:read'
```

- **Cache the token** and reuse it until about 30 seconds before `expires_in` runs
  out; don't request one per call.
- **Use the public issuer.** Tokens must carry exactly the portal's issuer
  (`<KEYCLOAK_EXTERNAL_URL>/realms/<realm>`). If you reach Keycloak through an internal
  address, Keycloak must run with a fixed public hostname (`KC_HOSTNAME`).
- **Keep the secret out of browsers, logs and URLs.** Rotate it in Keycloak.

### Which scopes you actually get

Your **effective scopes** are the scopes in the token *and* granted in the binding.
A scope in the token but not granted, or granted but not requested, is not usable.
`GET /platform/identity` shows the result.

## 5. Endpoints

All responses use the portal's standard envelope:

```json
{ "success": true, "data": { … }, "meta": { "requestId": "…" } }
```

Errors:

```json
{ "success": false, "error": { "code": "APPLICATION_ACCESS_DENIED", "message": "application access denied" }, "meta": { "requestId": "…" } }
```

Log the `requestId`; the portal's logs and audits use the same value.

### 5.1 `GET /platform/identity`

Checks your setup end to end. Call it at startup or from a health check.

```bash
curl -s -H "Authorization: Bearer $TOKEN" https://portals.health.go.ug/api/v1/platform/identity
```

```json
{
  "success": true,
  "data": {
    "applicationId": "alerts",
    "clientId": "alerts-platform",
    "subject": "6f0c…",
    "scopes": ["identity:read", "notifications:read", "notifications:write"]
  }
}
```

### 5.2 `GET /platform/readiness`

`200` with `"ready": true` when the portal core and every enabled background worker
are healthy; `503` with the same body shape and `"ready": false` otherwise.

```json
{ "success": true, "data": { "ready": true, "workers": [ { "name": "notification-email", "healthy": true, "lastSeen": "2026-11-28T09:00:00Z" }, { "name": "email-outbox", "healthy": true, "lastSeen": "2026-11-28T09:00:00Z" } ] } }
```

### 5.3 `POST /platform/notifications`

Queues one email notification. Requests are idempotent: send an
`Idempotency-Key` header, unique per notification (1–128 characters, starting with a
letter or digit, then letters, digits, `.`, `_`, `:` or `-`). Reusing the key returns
the original notification instead of sending a second one.

```bash
curl -s -X POST https://portals.health.go.ug/api/v1/platform/notifications \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: alert-2026-0042-notify-dho" \
  -d '{"title":"New alert in Gulu","message":"A new suspected case was reported.","recipient":{"email":"dho@example.org","name":"District Health Officer"}}'
```

| Field | Rules |
| --- | --- |
| `title` | Required, at most 200 characters, no line breaks |
| `message` | Required, at most 10,000 characters |
| `recipient.email` | Required, one plain address (no display name), at most 254 characters |
| `recipient.name` | Optional, at most 200 characters, no line breaks |

The request body may be at most 16 KiB.

Responses:

| Status | Meaning |
| --- | --- |
| `202` | Accepted and queued. `Location` points to `/api/v1/platform/notifications/{id}` |
| `200` | Same `Idempotency-Key` and same body as an earlier request; `"replayed": true` and the original notification |
| `409 IDEMPOTENCY_CONFLICT` | Same key, different body |
| `429 APPLICATION_QUOTA_EXCEEDED` | More than 60 notifications from your application in the last minute. Wait for `Retry-After` (seconds) |

The response body is the notification status (next section).

Don't put patient-identifiable or other sensitive content in notifications; they
travel as email.

### 5.4 `GET /platform/notifications/{id}`

Returns the status of a notification your application sent. Another application's
IDs return `404`.

```json
{
  "success": true,
  "data": {
    "id": "0b6f…",
    "applicationId": "alerts",
    "channel": "email",
    "status": "PENDING",
    "dispatchStatus": "PENDING",
    "dispatchAttempts": 0,
    "deliveryAttempts": 0,
    "createdAt": "2026-11-28T09:00:00Z",
    "replayed": false
  }
}
```

| Field | Meaning |
| --- | --- |
| `dispatchStatus` | The portal's delivery queue: `PENDING`, `PROCESSING`, `RETRY`, `SENT` (handed to the email outbox), `FAILED` or `CANCELLED` |
| `status` | Overall: the same as `dispatchStatus` until it is `SENT`, then the email outbox's status (for example `PENDING` or `SENT`) |
| `dispatchAttempts` / `deliveryAttempts` | Attempts by the delivery queue (up to 5) and by the email outbox |
| `EXPIRED` | Either status, when the delivery record no longer exists (for example after retention clean-up) |

Poll sparingly (for example with backoff, up to a few minutes); delivery is
asynchronous.

### 5.5 `GET /platform/applications/{clientId}/access`

Asks the portal, at the moment of a sensitive operation, whether a signed-in user
still has access to your application and with which roles and permissions. This
enforces portal-side suspension and role changes immediately, without waiting for
the user's tokens to expire.

Send the **user's** access token, not your machine token:

```bash
curl -s -H "Authorization: Bearer $USER_ACCESS_TOKEN" \
  https://portals.health.go.ug/api/v1/platform/applications/alerts/access
```

```json
{
  "success": true,
  "data": {
    "applicationId": "alerts",
    "subject": "ab96ef82-…",
    "roles": ["alerts_access"],
    "permissions": ["alerts:read"]
  }
}
```

The token must be active, carry the platform audience and `identity:read` in its
scope, and have been issued to your application's interactive client (`{clientId}`).
Configure that client's scope mappers to include a non-empty `sub` and the user's
roles. Machine tokens (`service-account-*`), unknown, inactive or unassigned users
are refused with `403`. A realm-wide administrator role does not bypass your
application's access roles.

Treat this as an authorization check, not a login or token exchange, and never
expose it through your own API as an unauthenticated proxy.

## 6. Errors

| Status | Code | Meaning | What to do |
| --- | --- | --- | --- |
| `400` | `INVALID_REQUEST` | Malformed body, path or `Idempotency-Key` | Fix the request; don't retry unchanged |
| `401` | `INVALID_IDENTITY` | Missing, expired or inactive token; wrong issuer or audience | Get a new token once; if it persists, check the client's audience mapper and issuer |
| `403` | `APPLICATION_ACCESS_DENIED` | Not bound, binding disabled, scope not effective, application not active and enabled, or the user has no access | Don't retry; check the binding and scopes with `GET /platform/identity`, or treat the user as denied |
| `404` | `NOT_FOUND` | Unknown resource, or one that belongs to another application | Don't retry |
| `409` | `IDEMPOTENCY_CONFLICT` | Key reused with a different body | Use a new key for a new notification |
| `429` | `APPLICATION_QUOTA_EXCEEDED` | Notification quota exceeded | Wait for `Retry-After` |
| `503` | `PLATFORM_UNAVAILABLE` | Introspection or a portal dependency unavailable, or platform auth not configured | Retry with backoff for reads and idempotent writes; fail closed for authorization |
| `500` | `PLATFORM_ERROR` | Unexpected portal error | Retry with backoff a few times; report the `requestId` |

The platform routes also share a per-IP rate limit (120 requests per minute); keep
tokens cached and avoid tight polling loops.

## 7. Calling the portal well

- **Timeouts:** set a connect and total timeout on every call (for example 2 s and
  10 s). Never wait indefinitely.
- **Retries:** retry only `503`, `500` and network errors, and only for `GET` or for
  `POST /platform/notifications` with the same `Idempotency-Key`. Use exponential
  backoff with jitter, a few attempts at most.
- **Fail closed:** if an access decision can't be obtained, refuse the operation.
- **Don't follow redirects** to other hosts with your token.
- **Network:** the portal must be resolvable and reachable from your backend over
  HTTPS, and your backend must reach Keycloak. Check DNS from the server that runs
  your backend early: internal DNS often holds only some `health.go.ug` names.
- **Logs:** never log tokens or secrets. Log the `requestId` and the HTTP status.

## 8. Versioning and compatibility

- The platform API is versioned in the path (`/api/v1`).
- Within `v1` the portal makes **additive changes only**: new endpoints, new optional
  request fields, new response fields. Ignore response fields you don't know.
- Removing or renaming a field or endpoint, adding a required input, or changing a
  type or meaning is a breaking change. It ships as `v2` alongside `v1`, with a
  deprecation period announced to every application owner before `v1` is removed.
- New capabilities get new scopes; an existing scope never silently gains broader
  meaning.
- The contract's `info.version` follows semantic versioning (`1.x.y` for `/api/v1`).
  CI fails a portal change that would break clients of the current contract. The full
  policy is in the
  [contract README](../../backend/core/platform/contract/README.md).

## 9. Checklist for a new backend

- [ ] Application registered in the portal, active and enabled
- [ ] Portal has `PLATFORM_AUTH_CLIENT_ID` and its secret configured (`GET /platform/identity` doesn't return 503)
- [ ] Machine client `<app>-<purpose>` created with service accounts only, capability scopes and the platform audience mapper
- [ ] Client bound to the application with only the scopes it needs, and a reason
- [ ] Backend caches tokens, sets timeouts, retries only safe calls, and fails closed
- [ ] `GET /platform/identity` returns your application, client and expected scopes
- [ ] For access decisions: your interactive client's tokens carry the platform audience, `identity:read`, `sub` and roles
- [ ] For notifications: idempotency keys are unique per notification and content is not sensitive
- [ ] Secrets stored in your secret mechanism and rotation agreed

## Related

- [Application identity and authorization contract](../platform-identity.md)
- [Onboarding a client application](client-onboarding.md)
- [Identity-sync contract](identity-sync-contract.md)
