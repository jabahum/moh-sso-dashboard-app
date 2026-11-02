# MOH platform development

All phases continue on `platform/phase-1-application-registry`. Each phase is delivered as a separate commit with a descriptive subject and a body covering the problem, implementation, compatibility/migrations, validation, and remaining limitations. Phases 1–4 are implemented; phase 5 remains planned. No additional phase branches are required.

| Phase | Planned commit subject | Scope |
| --- | --- | --- |
| 1 | `feat(platform): add governed application registry and lifecycle` | Application registry, lifecycle, validation, governance, audit |
| 2 | `feat(platform): define application identity and authorization contracts` | External sign-in, hosted-module identity, scoped machine identities, revocation |
| 3 | `feat(platform): expose application-scoped notification services` | Versioned notifications API, application isolation, idempotency, delivery tracking |
| 4 | `feat(platform): add developer onboarding and integration examples` | Integration guide, external example, hosted starter, validation command |
| 5 | `feat(platform): add application operations and rollout guidance` | Application-aware monitoring, support, compatibility, rollout and rollback |

Commit messages must describe the work actually delivered and identify checks that were not run. Future phases are committed only after their implementation and relevant checks; these subjects do not represent completed work.

## Phase 1: registry and lifecycle

The existing `ihp_systems` registry remains the application catalog. Existing ownership fields (`ownerTeam`, `ownerName`, `ownerEmail`), support and documentation URLs, environment, navigation, and launcher configuration are reused.

New fields:

- `lifecycleStatus`: controlled through the lifecycle endpoint, never the metadata payload.
- `integrationType`: `external` or `hosted_module`; validation requires agreement with `systemType` (`external` or `platform`).
- `contractVersion`: currently `1`, the registry onboarding contract version. This does not select or guarantee an application's business API version.

No new deployment unit, framework requirement, or shared domain database is introduced.

### Lifecycle and permissions

| Current status | Allowed next statuses |
| --- | --- |
| registered | configured, retired |
| configured | validated, retired |
| validated | approved, configured, retired |
| approved | active, configured, retired |
| active | suspended, retired |
| suspended | configured, retired |
| retired | none |

All changes require an authenticated actor with `rbac:write`. Approval and activation additionally require `rbac:roles:write`. These reuse existing governance permissions; no new permission grants or Keycloak roles are automatically assigned. A reason of 1–2000 characters is required. The existing admin route group authentication remains in force.

The lifecycle endpoint checks authorization in the service rather than a permission middleware so denied transition attempts can be audited. Malformed requests fail with 400, missing permissions with 403, invalid state transitions with 409, and unknown systems with 404. The existing admin authentication boundary can reject unauthenticated/non-admin requests before the endpoint; its normal request logging applies.

Validation, approval, and activation check saved configuration for:

- Display name, owner team, owner name, plain owner email address, support URL, and launch URL.
- Environment: `development`, `test`, `staging`, or `production`.
- A supported integration type and contract version.
- Valid platform/external launch and navigation behavior under the existing onboarding contract.
- At least one access role backed by an enabled system role.

This is registry configuration validation. It does not perform remote endpoint probes, verify a Keycloak client's redirect URIs, or certify the application's own authorization implementation. Those integration contracts belong to phase 2.

Suspension requires a new configuration/validation/approval/activation cycle before access can return. Retirement is terminal. The enabled flag remains a separate operational switch for active applications.

### API and administration

Use **Register application** on `/admin/rbac`, enter metadata, save, configure the system's roles/access roles, then advance through the lifecycle panel. Save ownership, launch, environment, navigation, or integration changes before requesting validation. Those changes invalidate existing validation/approval, including for active applications, and return the application to `configured` with access disabled. Edits to suspended or retired applications do not restore access.

Endpoints, relative to `/api/v1/admin/rbac`:

- `POST /systems`: register a new application. Accepts the metadata payload plus `clientId`; returns 201. Duplicate client IDs return 409 without changing the existing registration.
- `PUT /systems/:clientId`: existing compatible metadata upsert; new records also start as `registered` and disabled.
- `GET /systems` and `GET /systems/:clientId`: include lifecycle and integration metadata.
- `POST /systems/:clientId/lifecycle`: submit a transition.
- `GET /audit?systemClientId=pilot`: inspect application governance events.

Example registration (existing authenticated admin session or bearer token required):

```json
{
  "clientId": "pilot",
  "displayName": "Pilot health application",
  "ownerTeam": "Health applications",
  "ownerName": "Application owner",
  "ownerEmail": "owner@example.org",
  "supportUrl": "https://example.org/support",
  "documentationUrl": "https://example.org/integration",
  "environment": "test",
  "systemType": "external",
  "integrationType": "external",
  "contractVersion": "1",
  "launchUrl": "https://example.org/pilot",
  "launchMode": "new_tab",
  "displayInLauncher": true,
  "displayInSideNav": false
}
```

Example transition, sent separately for each allowed step:

```json
{
  "status": "configured",
  "reason": "Application owner confirmed the saved registration settings."
}
```

Register/configure the corresponding Keycloak client using [System onboarding](system-onboarding.md). Registry registration does not create Keycloak credentials or deploy the application. Lifecycle fields sent through metadata endpoints cannot override the state machine or impersonate an audit actor.

### Persistence and access enforcement

Migration `000048_application_registry_lifecycle` backfills existing records as `active`, preserving their existing enabled/disabled flags. It derives integration type from system type. New records, including systems discovered by sync or imported from a seed, default to `registered` and remain disabled until activated.

A database trigger enforces disabled status for every non-active application, including legacy seed/sync writes. Metadata upserts and lifecycle transitions commit their audit records in the same transaction. Lifecycle changes lock the system row so they serialize with metadata edits. Invalid state/validation attempts are audited without changing the record. Automatic approval invalidation is also audited; HTTP metadata edits carry their actor, while seed/sync operations may have no human actor.

The database authorization resolver already filters disabled systems from system-role permissions and accessible systems. Effective-access explanations now do the same. An empty database authorization result is authoritative, and a database error fails closed; static fallback cannot revive disabled access. Explicit development/static resolver mode remains available when no database resolver is configured.

These controls govern access resolved by the portal. Existing independently issued Keycloak tokens, external application sessions, and service credentials are not revoked by this phase. External backends must enforce their own authorization; phase 2 will define the identity and revocation integration contract. Portal launcher visibility alone never secures an external application.

### Verification and rollout

Run from `backend/`:

```sh
GOCACHE=/private/tmp/moh-sso-go-build go test ./...
GOCACHE=/private/tmp/moh-sso-go-build go vet ./...
GOCACHE=/private/tmp/moh-sso-go-build go run ./cmd/cli openapi validate
```

Run `make migrations-check` from the repository root to provision disposable PostgreSQL through Docker and run both the existing migration-path tests and the registry integration test. The existing PR migration check runs this command, so database verification is included in CI.

For a database you provision yourself, the PostgreSQL integration test is opt-in. Point it at a disposable test database using `MOH_REGISTRY_TEST_DATABASE_URL` (a `postgres://` or `postgresql://` URL) and run:

```sh
GOCACHE=/private/tmp/moh-sso-go-build go test ./internal/features/rbac -run TestApplicationRegistryPostgres -v
```

It creates and removes its own random schema. It checks migration compatibility, duplicate registration, onboarding, real authorization resolution, suspension, seed/upsert behavior, configuration invalidation, audit rollback, retirement, and down/up migrations. Without the environment variable, the test explicitly skips.

From `frontend/`, run type checking, lint on the changed RBAC files, and the RBAC app build. Use the repository-supported Node version.

Apply migration 48 before starting the new backend. On a fresh installation, a realm administrator with registry and role-management permissions must activate the initial applications after seed/sync. For an existing installation, review ownership/launch configuration and seed sources before the next sync: changes to approved configuration now require renewed onboarding.

For rollback, stop the new backend, roll back migration 48, and deploy the previous backend/frontend. The down migration preserves enabled flags so suspended/retired applications stay disabled immediately after rollback. The previous release's ordinary seed/sync behavior can re-enable them later; retain operational controls during rollback. Lifecycle history remains in the existing audit table, but lifecycle and integration columns are removed.

## Phase 2: identity contracts

Implemented the [application identity contract](platform-identity.md), authenticated Keycloak introspection, application-specific user access decisions, scoped machine bindings, and audited grant revocation. Migration 49 is required before enabling platform endpoints.

## Phase 3: shared notifications

Implemented the [shared notification API](platform-notifications.md) with application ownership, scoped access, atomic queue/audit insertion, idempotency, rolling database quotas and delivery-state tracking through the existing workers. Migration 50 is required. The first adapter supports plain-text email.

## Phase 4: developer onboarding

Added an [independent Python application pilot, reference client, integration verifier, and hosted-module template](../examples/platform/README.md). The onboarding guide covers registration, interactive and service clients, activation, launcher integration, test notifications, suspension/revocation, and verification commands. The examples are opt-in and are not installed in the production shell.

## Remaining phases

Phases 1–4 establish registry governance, identity contracts, shared notifications and developer examples. Platform operations/observability remain phase 5. The migration, registry and platform PostgreSQL integration checks have passed locally against disposable PostgreSQL. Live identity-provider/SMTP setup remains environment-specific.
