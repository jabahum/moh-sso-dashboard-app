# Platform core boundaries

This page defines the mandatory platform core: which capabilities it owns, the
portable contracts in `backend/core`, dependency rules, startup profiles and
verification. It is for backend developers and operators working on the core or
on optional modules; module packaging and release are covered in
[Modular distribution](modular-distribution.md).

The platform core is the mandatory foundation used by optional applications.
Separately packaged capabilities can still be mandatory. The module inventory
and `DISABLED_MODULES` describe optional compiled applications, not a switch for
authentication, authorization or governance.

## Current implementation: phases 1–4

### Module runtime (phase 1)

`backend/core` owns framework-independent module inventory, composition
validation, initialization, cancellation, rollback and cleanup. It uses only
the standard library. The existing `backend/module` package is a Gin adapter
that delegates lifecycle management to the core and retains route callbacks.
Existing bootstrap wiring and compiled-module APIs remain compatible. The host
inventory view retains the `module.Info` schema name while deriving its fields
from `core.ModuleInfo`, keeping generated client contracts stable.

| Contract | Responsibility |
| --- | --- |
| `core.ModuleInfo` | Existing ID/version/enabled JSON inventory |
| `core.ModuleDefinition` | Explicit dependency order and readiness/initialization callback |
| `core.Stop` | Cooperative cleanup using the caller's shutdown context |
| `core.Runtime`, `core.NewRuntime` | Validate and run the compiled-module lifecycle |

Initialization runs once and skips disabled definitions. Dependencies must
precede consumers; enabled consumers cannot require disabled dependencies.
Failure cancels the shared module context and cleans up partial acquisitions
in reverse order, using the existing independent 15-second rollback context.
Normal shutdown cancels module contexts before reverse cleanup and returns
joined errors. Cleanup is idempotent, including concurrent calls. Callbacks
must honor cancellation/deadlines; the runtime does not forcibly terminate
callbacks that ignore them.

The host still owns routing, credential verification, policy/provider selection,
health-context selection middleware and infrastructure. Phases 2–3 share existing
access predicates and decision rules through portable contracts. The core runtime runs trusted compiled
code; it is not a sandbox or runtime code downloader.

### Identity, sessions and authorization (phase 2)

Phase 2 adds portable `Identity`, `SessionStore[T]`, `AuthorizationResolver`,
`Access` and `SystemAccess` contracts. Identity carries verified subject/role
claims rather than credentials. The generic session payload type belongs to the
trusted host: `authsession.Data` still defines and serializes the token bundle
only in the Redis adapter. The host store implements the core interface and the
auth handler consumes it; the not-found sentinel is shared without changing its
existing error text.

The authentication middleware still selects/verifies credentials in the host,
then supplies `core.Identity` to `authz.NewContextForIdentity`. Existing
constructors remain compatible. Host role normalization, defaults and admin/user
flags remain unchanged. The existing DB/static resolver is adapted through
`core.ResolveAuthorization`; provider errors discard partial permission/system
grants and do not trigger a new fallback. Explicit host static-policy selection
remains available where it already existed.

Permission comparisons remain exact, role matching remains case-insensitive
with whitespace normalization, and system matching retains its previous rules.
The existing host `authz.Context`, `SystemAccess` and `ResolvedAccess` types
retain their schema ownership. Adapters preserve all system metadata and
nil/empty collections.

No OIDC verifier, HTTP/cookie parser, Redis client, token payload, credential
fallback, session TTL algorithm or persistence implementation moved into core.
Authorization contracts do not authenticate a caller: the host must verify
credentials and apply its authentication/context guards first.

### Application governance (phase 3)

Phase 3 adds `Application` and a read-only `ApplicationRegistry` contract.
The RBAC adapter preserves registry identity, launch/navigation metadata,
ownership/support information, lifecycle/integration/version, visibility and
configured access roles. This is an administrative registry view, including
inactive registrations; it is not the caller's authorized launcher inventory.
The host's named HTTP DTOs and schema identities stay unchanged.

Core now owns lifecycle status/transition predicates and review-authority
decisions, plus the existing metadata validation rules. The host derives
configuration/review capabilities from its permission catalogue. Approval and
activation require both capabilities; retirement is terminal, and suspension
requires reconfiguration/review before activation. The host still normalizes
requests, validates launch/navigation behavior and enabled access roles, and
serializes lifecycle/configuration changes with audit events in one transaction.
Reviewed manifest import, review tokens and provisioning remain host-owned.

### Contextual authorization (phase 3)

`ContextAuthorization[ID]`, `ContextAccessResolver[ID,E]` and
`ContextDecision[E]` describe combined functional/system/context decisions.
The host supplies its ID and explanation types through generics, keeping UUID
and response-model dependencies outside core. Permission and contextual system
matches remain exact and case-sensitive; absent requirements are optional.
Permission checks precede system checks, which precede scope lookup. Resolver
errors discard partial decisions and preserve their cause. A denied scope
retains its explanation and returns the existing shared forbidden sentinel.

`ContextAssignment[ID]`, `ContextHierarchy[ID]` and
`ResolveContextScope` separate scope decisions from geographic persistence.
Only explicit descendant scope can query ancestry; same-node access requires
no hierarchy query. Assignment ordering, first-match selection, default-National
reason text and group provenance remain unchanged. Adapters reconstruct the
full existing explanation, including requested/assignment nodes and group IDs
and paths. The provider supplies already-effective assignments: expiry, group
membership, defaults and disabled-assignment filtering remain host duties.
The service rejects inactive requested nodes before loading assignments.

Alias resolution and selected-context middleware retain their current
ordering, HTTP errors and legacy handling of requests without a selection.
Observatory replication, warehouse access, provider construction and worker
startup have not changed. Phase 4 introduces the explicit boot profile described below.

## Mandatory capability ownership

| Capability | Existing host owner | Target portable boundary |
| --- | --- | --- |
| Identity and sessions | `auth`, `authsession`, `sessions`, Keycloak/session adapters | Phase 2 verified-claim and generic session-store contracts; token/cookie verification stays in host adapters |
| Authorization | `authz`, `rbac`, `system_rbac` | Phase 2 resolver/access contracts and predicates; policy catalogues and DB/static implementations stay host-owned |
| Application governance | Application registry/manifests in `rbac`, `platform` APIs | Phase 3 registry descriptor, lifecycle/review predicates and metadata checks; manifest import/provisioning remain host-owned |
| Module runtime | `core` with `module` HTTP adapter | Implemented in phase 1 |
| Context authorization | `health_context` | Phase 3 combined-access and assignment/hierarchy contracts; selection middleware and geography sources remain adapters |
| Audit recording | Shared audit service and primary persistence | Record required security/operational events independently of the optional Audit reader |
| Operations and bootstrap | `platform`, `bootstrap`, configuration/logging/readiness | Required capability health, version/request context, resource startup/shutdown |
| Frontend shell/contracts | Shell plus shared auth/API/config/state/types/UI/microfrontend packages | Authenticated context, application loading, navigation and extension contracts |

The current feature catalogue already marks the seven host features
`auth`, `authsession`, `sessions`, `rbac`, `system_rbac`,
`health_context` and `platform` as core. This classification is a host
ownership designation, not a claim that all their implementations are portable.
Those feature IDs cannot be disabled through the optional-module setting.

## Shared contracts versus optional applications

| Area | Required boundary | Optional implementation/application |
| --- | --- | --- |
| Users | Current identity and user-directory contracts | User administration, bulk imports and their UI |
| Clients | Application identity, governed registry and launch/access metadata | Keycloak client provisioning and administration UI |
| Notifications/email | Event/delivery contracts for consumers | Outbox workers, channel providers and administration UI |
| Health context | Assignment and selected-scope policy | Observatory replication and warehouse/geography source adapters |
| Audit/metrics | Required event recording and basic operational telemetry | Audit browsing/export and dashboard analytics |

Announcements, Administrative Units, GeoJSON, Report Scheduler, Visualizer,
Surveillance, DQA, Documents, Document Templates and Storage Locations remain
optional capabilities. The default distribution may ship them enabled.
Packaging and enablement are separate decisions.

## Dependency rules

- Portable core may import the standard library and its own subpackages.
- HTTP/Keycloak/SQL/Redis/SMTP/Observatory adapters depend on core contracts.
- The module HTTP adapter receives feature behavior through composition;
  it does not import host features or published application packages.
- Optional applications depend on the contracts they need. They do not provide
  the mandatory identity or governance authority for the platform.
- Provider credentials, raw session tokens, database connections and route
  objects belong in host adapters, not portable inventory/lifecycle contracts.

Architecture tests enforce the core import boundary and prevent feature imports
in the HTTP module adapter. CI also copies the entire core tree into an isolated
temporary module and checks build/vet/uncached race tests with an empty module
cache, workspaces/downloads disabled and no third-party dependencies.

## Startup profiles

Phase 4 adds `PLATFORM_PROFILE=full|core`. The default is `full`, preserving
the existing full distribution. Unknown profiles and unknown/mandatory IDs in
`DISABLED_MODULES` fail configuration validation before opening connections.
Core selects every optional module as disabled and omits optional feature
routes and constructors. Inventory still reports their versions with
`enabled: false`.

| Resource/capability | Full profile | Core profile |
| --- | --- | --- |
| Primary Postgres, Redis sessions/rate limiting, configured Keycloak | Required | Required |
| Auth, sessions, authoritative RBAC/governance and health-context policy | Enabled | Enabled |
| Audit event recorder | Enabled independently of Audit reader | Enabled |
| Durable internal auth/audit/startup notification records | Enabled | Enabled, without channel queues or Redis publication |
| Remote, DWH, DQA and Observatory pools/replication | Existing legacy-feature dependencies | Not constructed or validated |
| Storage factory and file-storage provider | Constructed for existing document features | Not constructed |
| SMTP/template/queue and SMS providers | Existing delivery wiring | Not constructed or validated |
| Document and DQA workers | Owned by the `documents` and `data-quality` modules; started when the module is enabled | Not constructed or started |
| Delivery workers | Started for existing full-profile settings | Not constructed or started |
| Packaged scheduler worker | Started when its module is enabled | Not constructed |
| Optional HTTP/admin/public/machine notification routes | Existing surface and module switches | Omitted |
| Frontend | Existing full-distribution shell | Separately hosted; core Compose is backend-only |

Core retains the existing user repository/service for governance directory
lookups, but does not register user/client administration routes or run bulk
startup user synchronization. It preserves configured RBAC seed/discovery
settings and existing credential verification; deployed authentication bypass
restrictions still apply. Provider secret files are read only for capabilities
used by the selected profile. Mandatory secret validation remains intact.

Documents, Document Templates, Storage Locations, Data Quality, Visualiser and
Surveillance are now optional modules too, so `DISABLED_MODULES` switches each
of them independently in the full profile: a disabled module registers no
routes and runs no worker. The full profile still opens the Remote, DWH, DQA
and Observatory connections and the storage providers they use, even when the
modules that need them are disabled, and the document import pipeline keeps
its surveillance processors registered.

Both profiles run the same existing primary-database migrations. No schema,
data, module pin or migration history is deleted when profiles change. A
core-to-full switch uses the retained schema/data after the full profile's
provider configuration is supplied. Core is a runtime composition profile in
the same production image, not an independently released image or database
schema. Profile compatibility assumes the same configured primary database and
session store. The standalone core Compose entry point owns its own
`core_postgres`/`core_redis` volumes; it does not copy or automatically reuse
a full installation's volumes. Existing installations must retain their
primary/session endpoints when switching the backend profile.

### Readiness and shutdown

`/health/ready` checks mandatory primary persistence, Redis and Keycloak.
Optional Remote database or Observatory outages remain visible as degradation
in `/health`; disabled Remote persistence is reported as `disabled`. They do
not fail mandatory-core readiness. The machine platform readiness endpoint
also checks core dependencies and only the delivery workers enabled in the
selected profile/settings. Its existing worker array is empty in core.
Operations expose health for enabled delivery workers; absent enabled
heartbeats still fail feature readiness. Disabled machine delivery endpoints
are not registered in core.

Legacy workers are prepared before any are launched. Startup constructor
failure cannot leave partially running workers. Shutdown cancels and joins the
worker group, then stops packaged module workers, before closing Redis/database
pools. Stop callbacks support idempotence and retry after a caller deadline.
If a worker fails to join within the shutdown budget, shutdown logs the failure
and does not explicitly close pools underneath that worker; process termination
reclaims them. The HTTP server stops accepting/drains requests before the final
join callbacks run.

### Running the minimal backend

Copy `infra/env/core.env.example` to `infra/env/core.env`, configure your real
Keycloak and separately hosted frontend URLs, and create the five files under
`infra/secrets` referenced by `infra/compose/docker-compose.core.yml`:
`db_password.txt`, `keycloak_admin_secret.txt`, `keycloak_web_secret.txt`,
`redis_password.txt` and `token_symmetric_key.txt`. Use a 32-character token
key. Environment variables ending in `_SECRET_FILE` in the Compose model allow
operator-selected paths. Operator env/secret files are ignored by Git.

```sh
docker compose --env-file infra/env/core.env -f infra/compose/docker-compose.core.yml config --quiet
docker compose --env-file infra/env/core.env -f infra/compose/docker-compose.core.yml up -d --build
```

This entry point starts only primary Postgres, Redis and the backend. The
backend listens on loopback port 9000 by default (`CORE_BACKEND_PORT` overrides
the host port). Put the existing operator-managed TLS reverse proxy in front
of it. No warehouse/Observatory/SMTP/SMS/storage service or volume is required.
The current full-distribution Compose entry point remains unchanged.

## Next phases

1. Align frontend shell/extension contracts and verify an optional application
   against the core, then decide independent release/version boundaries.
   `backend/core` is not yet released independently: `backend/go.mod` requires
   it as `v0.0.0` through a local `replace ... => ./core` directive, and no
   `backend/core/v*` tag exists in this checkout. `backend/core/README.md` reserves that tag prefix
   for future releases.

Document, template and storage-location extraction remains deferred.

## Verification

```sh
python3 infra/scripts/test-platform-core.py
go -C backend test -race ./core ./module ./internal/architecture ./internal/authz ./internal/features/rbac ./internal/features/health_context ./internal/api ./internal/bootstrap
make -C backend modules-check contracts-check
python3 infra/scripts/test-distribution-auth.py --backend-image <candidate> --profile core
python3 infra/scripts/test-distribution-auth.py --backend-image <candidate> --profile full
```

## Self-contained HTTP modules

All published modules (Administrative Units, Announcements, Audit, GeoJSON,
Metrics and Report Scheduler) and the modules extracted onto `core/modulekit`
(Storage Locations, Visualiser, Document Templates, Data Quality, Documents and
Surveillance) are self-contained in `docs/module-catalog.json`:
each package owns its HTTP handlers, DTOs, OpenAPI metadata and any persistence, and
no host adapter tree remains (`backend/internal/adapters/modules` holds only a
README). `NewModuleRegistry` in `backend/internal/api/modules.go` mounts all of
them through the generic `internal/modulehttp` registrar: `Register` for
protected-only modules and `RegisterSurfaces` for modules with public and
protected routes (Announcements, and Report Scheduler via
`RegisterReportScheduler` in `backend/internal/api/report_scheduler.go`, which
first adds a per-IP rate limit to the public delivery-download group). The
module owns route dispatch and authorization. The registrar bridges verified Gin
access to the module's standard HTTP request context and owns no application
behavior. Both full and core-only profiles apply the same module enablement
rules.
