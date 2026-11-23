# Backend microservice readiness

This page sets the rules that keep backend feature boundaries explicit: allowed imports, feature contracts, dependency direction, event and observability expectations, and the criteria a feature must meet before extraction. It is for backend developers adding or changing features. For how optional features are packaged and released as independently versioned modules, see [Modular distribution](modular-distribution.md).

The backend is intentionally still a modular monolith. Phase 12 makes those module boundaries explicit so individual features can be extracted later without a rushed rewrite.

## Current architecture rule

Feature packages live under:

```text
backend/internal/features/<feature>
```

A feature may import:

- standard library packages
- third-party packages
- shared internal infrastructure packages such as `authz`, `http`, `log`, `storage`, `keycloak`, `service`, and `db/sqlc`
- its own package files

A feature should not import another feature unless the dependency is listed as an approved exception.

## Enforced boundary check

The backend has an architecture test:

```bash
cd backend
GOCACHE=/private/tmp/moh-sso-go-build go test ./internal/architecture
```

It also runs as part of:

```bash
GOCACHE=/private/tmp/moh-sso-go-build go test ./...
```

Approved cross-feature imports today:

| Source feature | Target feature | Reason |
| --- | --- | --- |
| `announcements` | `users` | Announcement audiences resolve Keycloak users through the users repository port. |
| `auth` | `authsession` | Auth owns the auth session backing store as an implementation detail. |
| `documents` | `storage_locations` | Document upload flows validate configured storage locations. |
| `rbac` | `system_rbac` | RBAC sync applies the system RBAC seed format. |

When adding a new cross-feature import, first ask whether the shared concept should become a small shared package or an explicit interface passed through bootstrap. Add a boundary exception only when the coupling is intentional and documented.

## Feature contracts

Each extractable feature should expose stable contracts at the HTTP and service boundary:

- `routes.go`: route ownership
- `dto.go`: request/response contracts
- `mapper.go`: database or external model conversion
- `service.go`: use-case boundary
- `repository.go`: feature-owned persistence port
- `repository_postgres.go`, `repository_keycloak.go`, or similar: adapter implementation

Handlers must return DTOs and safe API errors. They should not return sqlc rows, database models, Keycloak admin payloads, or raw internal error strings.

## Dependency direction

Keep the dependency direction boring:

```text
bootstrap -> feature handlers/services/repositories
feature handler -> feature service/use case
feature service -> feature repository ports + shared services
feature repository adapter -> db/sqlc, Keycloak, storage, or external integration
```

Avoid:

- feature-to-feature service calls
- handlers constructing repositories
- repositories calling handlers or services
- shared packages importing feature packages

## Event and outbox readiness

Do not add a broker yet. Shape business events as explicit in-process events so they can later become outbox rows or messages.

The current in-process event contract lives in:

```text
backend/internal/events
```

Use `events.Publisher` as the dependency boundary when a feature needs to announce something important. Keep event publishing behind constructors/bootstrap so a future outbox or broker adapter can replace the in-memory publisher without changing feature handlers.

Candidate event streams:

| Event | Current behavior | Future boundary |
| --- | --- | --- |
| `announcement.published` | Queues notifications and email deliveries. | Outbox event consumed by notification/email service. |
| `announcement.attachment_added` | Persists attachment metadata and storage object. | File scan/indexing event. |
| `rbac.sync.completed` | Updates local RBAC state and drift reports. | Governance/audit event. |
| `rbac.user_access_changed` | Applies Keycloak/local access changes. | User access projection event. |
| `document.uploaded` | Stores file and starts processing. | Document processing worker event. |
| `notification.delivery_failed` | Delivery worker marks status. | Retry/dead-letter event. |
| `email.delivery_queued` | Email worker sends queued message. | Email service boundary event. |
| `audit.recorded` | Persists audit log. | Central audit service event. |

Recommended future shape:

```go
type Event struct {
    ID            string
    Type          string
    Source        string
    Subject       string
    CorrelationID string
    OccurredAt    time.Time
    Payload       json.RawMessage
}
```

Start with in-process publishing behind an interface. Add a persistent outbox only when retry guarantees and service extraction require it.

## Observability contract

Every extracted service should preserve:

- request ID/correlation ID
- actor/user ID
- system/client ID when applicable
- route/action/module
- audit outcome
- safe structured error code

Avoid using `fmt.Printf` or raw `log.Printf` for normal application events. Prefer the project logger and keep sensitive data out of logs.

Current HTTP observability behavior:

- `X-Request-ID` is accepted from callers or generated per request.
- `X-Correlation-ID` is accepted from callers or defaults to the request ID.
- Both IDs are returned as response headers.
- API response envelopes include `meta.requestId`.
- request logs include request ID, correlation ID, method, path, route, status, latency, IP, and user ID when available.
- audit metadata includes request ID and correlation ID.

When adding async work, pass `context.Context` through so event publishers, logs, and audit records keep the same correlation ID.

## Extraction order

Recommended extraction order when the monolith boundaries are stable:

1. `notifications` and `email`
   - naturally async
   - queue/worker already exists
   - minimal synchronous UI dependency
2. `announcements`
   - clear API, attachment storage, notification/email events
3. `documents`
   - file processing can become worker/service oriented
4. `rbac`
   - strong Keycloak boundary, but security-critical; extract only after contracts are tested
5. `surveillance`
   - largest domain; keep inside monolith until DDD boundaries and reporting/query models are stable

Do not split `auth` until session, cookie, refresh, and Keycloak callback behavior is fully covered by tests.

## Database split risks

The current database is shared. Before extracting a feature:

- identify tables owned by that feature
- identify tables read by other features
- replace direct reads with API/query contracts where needed
- add events/projections for cross-feature read models
- keep migrations owned by the service that owns the table

Avoid splitting the database before table ownership is clear.

## Acceptance for future extraction

A feature is ready to extract when:

- no unapproved feature imports exist
- handlers expose DTOs only
- service dependencies are constructor-injected
- repository ports are owned by the feature
- external integrations are adapters
- important behavior has unit tests
- audit/RBAC behavior is documented
- required events are identified
- deployment/runtime config can be isolated
