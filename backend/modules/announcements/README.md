# Announcements Go module

Module path: `github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements`.
Requires Go 1.25.4, `github.com/google/uuid v1.6.0` and `github.com/lib/pq v1.10.9`.

Version `v0.2.0` is the self-contained release under
`backend/modules/announcements/v0.2.0`. It adds standalone PostgreSQL
persistence, HTTP routes and provider composition. Hosts select this immutable
Go dependency with reviewed checksums; integrations upgrade independently.
Local/unversioned consumers report `dev`.

## Public API

| API | Responsibility |
| --- | --- |
| `NewServiceWithSettings`, `Service` | Announcement creation, updates, lifecycle actions, listing and audience/delivery orchestration |
| `NewModule`, `Register`, `Module` | All 26 standard HTTP routes, DTOs, access/rate enforcement and OpenAPI metadata |
| `Repository`, `NewRepository(*sql.DB)` | All 54 persistence operations, package-owned SQL and audience transactions; caller-owned pool |
| `UserDirectory`, `GroupDirectory`, `HealthAudience` | User/group lookup and actor-scoped or stored health audience expansion |
| `AttachmentStorage` | Upload/download streams and deletion; resource ownership stays with the host |
| `NotificationPublisher` | Forward package-owned notification envelopes and delivery requests |
| `EmailAttachmentNormalizer` | Apply the host's attachment validation and normalization policy |
| `ServicePorts`, `Settings` | Explicit dependencies and non-secret branding, URL, SMS and attachment settings |
| `ModulePath`, `Version` | Build dependency identity; local replacements report `dev` |

The repository returns package-owned announcements, attachments, enums and
statistics. Standard-library nullable SQL values and UUID nullable values are
data values, preserving null versus zero semantics without a database driver.
Private SQL records and queries are internal implementation details. No public
SQLC aliases, Gin, Keycloak, Redis, SMTP or dashboard imports are included.

Compose the service using package persistence and the capabilities needed by
the chosen operations (a custom `Repository` remains supported):

```go
repo, err := announcements.NewRepository(primaryPool)
if err != nil {
    return err
}
core := announcements.NewServiceWithSettings(repo, announcements.ServicePorts{
    Users:            users,
    Groups:           groups,
    HealthContexts:   healthAudience,
    Storage:          storage,
    Notifications:    publisher,
    EmailAttachments: normalizer,
}, settings)
```

The package routes invoke verified platform authentication, administrator,
permission and per-user rate capabilities before accessing bodies or feature
dependencies. Direct service consumers remain responsible for authorization.
Health audience resolution keeps actor-scoped selection distinct from stored
expansion.

Notification forwarding remains best effort; a downstream publisher failure
does not turn successful persistence/publication into a failure. Existing
notification labels and queued delivery payloads are preserved. This package
does not own a worker or provide atomic/exactly-once persistence and delivery.
Scheduled delivery execution belongs to the host's notification infrastructure.

### Scheduled publishing

Scheduling an announcement only records `publish_at`; it does not queue audience
email or SMS. Hosts must run the scheduled publisher, otherwise scheduled
announcements never go live:

```go
go module.RunScheduledPublisher(ctx, time.Minute, func(err error) { log.Println(err) })
```

Each tick claims due `SCHEDULED` announcements atomically (`FOR UPDATE SKIP
LOCKED`), marks them `PUBLISHED` and sends their notifications with the audience
and uploaded attachments current at that moment. Several replicas can run it
safely. Because nothing is queued ahead of time, drafting, deleting, archiving,
rescheduling or re-targeting a scheduled announcement needs no cancellation.
Due announcements that have already expired are left unpublished. Direct
service consumers call `Service.RunScheduledPublisher` or `PublishDueScheduled`.

Publishing now (or scheduling with a past time) sets `publish_at` to now when it
was in the future, and refuses announcements whose `expires_at` has passed.
Schedule requests no longer accept request `attachments`; upload them to the
announcement instead.
Hosts must retain their email normalization policy; nonempty email attachments
that require normalization fail closed when the normalizer is absent.

Attachment uploads retain configured count/type/byte limits, checksums, actor
metadata and orphan cleanup on persistence failure. Download streams must be
closed by the caller. Hosts supply settings appropriate to their deployment;
nil settings use the existing defaults rather than host configuration.

## Host ownership

Shared identity and permission verification, blob storage,
configuration/secrets, notification delivery workers, core audit recording and
migrations remain in the dashboard. The module
neither starts infrastructure nor applies migrations. Disabling announcement
routes does not cancel previously queued shared notifications.

`NewRepository` rejects a nil pool, honors request cancellation and preserves
wrapped SQL errors. It does not ping or close the pool. The caller must apply
the existing Announcements schema before using it, including audience and
attachment tables. SMS recipient lookup reads shared notification preferences;
group and health-context mappings reference platform-owned identity/context
rows. These dependencies are explicit schema prerequisites, not owned
infrastructure. All five audience replacements are transactional. Notification
markers and append-only audience snapshots retain existing behavior; they are
not an atomic delivery outbox.

## Verification

From the repository root:

```sh
python3 infra/scripts/test-announcements-package.py
python3 infra/scripts/test-announcements-postgres-package.py
python3 infra/scripts/test-announcements-http-package.py
make -C backend modules-check
```

The verifier copies this package outside the source tree, disables workspaces
and network module downloads, and seeds a private cache with only reviewed
pinned artifacts for UUID and lib/pq. It verifies checksums, the exact dependency
graph,
build/vet/uncached race checks and unchanged dependency metadata. It then
builds, vets and runs `testdata/consumer/main.go` in a separate module using
only public APIs. The consumer exercises lifecycle, notifications, nullable
values, attachment upload/download and repository error propagation.

The PostgreSQL verifier creates a disposable database from a test-only snapshot
of the existing schema. It builds a race-enabled public-API consumer outside
the repository with workspaces/downloads disabled and a private dependency
cache. It exercises all 54 operations, nullable values, audience isolation,
attachment scoping/deletion, append-only snapshots, cancellation and pool
ownership. All five audience replacement transactions are tested for rollback
after partial writes; clients/users use explicit test-only failure triggers.
The module never applies this fixture schema.

The HTTP/provider phase is independently verified. Host integration selects the
reviewed release and supplies shared capabilities at its composition root.
To verify the published dependency with a fresh module cache and no replacement,
run `python3 infra/scripts/test-announcements-release.py`.
This release check requires network access and verifies downloaded checksums,
selected version, source parity and an independent consumer.

## HTTP and capability composition

`NewModule(Dependencies{...})` composes the package-owned repository/service,
`ServicePorts` and HTTP handler. It implements `http.Handler`; `Patterns()`
returns 26 method/path registrations and `OpenAPI()` documents those exact
registrations. `Register` supports any registrar with `Handle(string,
http.Handler)`, including `http.ServeMux`. No Gin or dashboard imports are
required. Routes use `/announcements` and `/admin/announcements`; a dashboard
mounts them below `/api/v1`. Attachment response URLs retain that API prefix.

Provide exactly one database pool or repository. The actor, administrator,
permission and rate capabilities are required; missing callbacks reject
construction. Actor claims must come from verified platform context and contain
the subject UUID, realm roles and client IDs (include platform UUIDs for
UUID-based client audiences). Requests/headers cannot supply identity. Unknown
capability failures return 503; access failures use 401/403. Writes use
`announcements-write` (60/minute), publishing uses `announcements-publish`
(30/minute), and denials return 429 with retry metadata. These required policies
fail closed when enforcement is unavailable.

Audit recording, request IDs and stream-error reporting accept shared platform
capabilities. Audit and notification forwarding retain best-effort behavior.
Storage, user/group lookup, contextual expansion and attachment normalization
are supplied in `ServicePorts`; optional providers remain optional, and
operations requiring missing providers fail. The module owns no delivery worker
and never closes shared providers or the database pool.

Both public feeds expose only live `ALL_USERS` announcements. Signed-in feeds
apply verified user, role, client, group and health-context audience membership
before pagination. Explicit role/client selectors must belong to the caller.
Protected attachment downloads require visibility of the live announcement;
administrator routes use the separate administrator/permission checks.
This closes the old active-feed/me-feed and direct-download visibility gaps.
Missing group/context resolution fails closed.

JSON and multipart request bodies default to a 16 MiB transport limit, adjustable
through `MaxRequestBytes`; existing decoded attachment limits still apply.
Multipart temporary files are removed after handling. Downloads encode filenames
as MIME parameters, close streams and report partial-write failures without
appending an error response. Lifecycle timestamp inputs are normalized to UTC
before binding to the existing timestamp-without-time-zone columns.

The HTTP PostgreSQL consumer uses the same isolated fixture/cache harness and a
race-enabled binary. It verifies all 26 operations, JSON/multipart uploads,
stream downloads/deletion, verified identity/permissions, private-audience
isolation, shared audit, email/SMS payload forwarding and actor-scoped health
preview. Unit tests cover denials before dependencies, exact rate policies,
body bounds, stream errors, metadata inventory and pagination across private
pages. Hosts must verify their pin, shared capability wiring and module disablement
when integrating this release.

Functional providers (`UserDirectoryFuncs`, `GroupDirectoryFuncs`,
`NotificationPublisherFunc`, `EmailAttachmentNormalizerFunc`) allow the
composition root to map shared capability values without defining host feature
adapter types. Missing functions fail explicitly. Direct storage, health-context
and core audit interfaces need no wrapper when the provider already satisfies
the contract. Feature policy, DTOs, database operations and delivery orchestration
remain entirely package-owned.
