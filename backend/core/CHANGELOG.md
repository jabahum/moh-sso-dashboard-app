# backend/core changelog

Release tags use the `backend/core/vX.Y.Z` prefix. Below 1.0 a minor version
may contain breaking changes. Check IDs in `identitysync/conformance` are
stable within a minor version.

## v0.2.0 (unreleased)

### Platform API conformance

- `platform/conformance`: checks a portal's platform API against its contract
  and authorization rules over real HTTP: authentication (no token, cookie
  only, unverifiable, wrong audience, wrong issuer, unbound client), identity
  and effective scopes, readiness, the access decision, notifications, and
  revocation through caller-supplied controls. Response objects are checked
  against `Shapes`, which a dashboard test keeps identical to the contract;
  passing raw checks are repeated with `platform/client`. Missing inputs skip
  a check with the reason.

### Platform API client

- `platform/client`: a standard-library-only client for the portal's platform
  API (`Identity`, `Readiness`, `Notify`, `Notification`, `Access`).
  `ClientCredentials` obtains Keycloak client-credentials tokens and caches
  them until 30 seconds before expiry, sharing one request between concurrent
  callers; a rejected token is replaced once. Every attempt has a timeout;
  only GETs and idempotency-keyed notifications are retried, on network
  errors, 5xx and 429 with a short `Retry-After`, with exponential backoff and
  jitter. Redirects, plain HTTP outside loopback and responses over 1 MiB are
  refused. Failures are `*Error` values matching `ErrInvalid`,
  `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrConflict`,
  `ErrRateLimited`, `ErrUnavailable`, `ErrUnexpected` or `ErrToken`.

### Platform core

- `Platform`: the standard set of trusted host capabilities offered to every
  compiled module (verified access, `AuthorizeAny`, `RequireAdmin`,
  `RequireSystem`, per-user `Limit`, `RequestID`, `ReportError` and `Audit`),
  with `Require` so a module fails at construction when the host omits a
  capability it needs (`ErrMissingCapability`).

### Module migration streams

- `ModuleDefinition.Migrations` (an `fs.FS` of golang-migrate files) gives a
  module its own schema history. `Runtime.UseMigrator` supplies the host
  `Migrator`; `Runtime.Initialize` applies enabled modules' streams in order
  before initializing any module, and returns `ErrMigratorRequired` if an
  enabled module ships migrations without one. `Runtime.MigrationStreams` lists
  them.
- `ValidateModuleMigrations` checks naming, unique versions, paired up/down
  files and that written tables carry `ModuleTablePrefix`;
  `ModuleMigrationsTable` names each stream's history table.

## v0.1.0 (2026-11-23)

First published version, MIT licensed (`LICENSE`). Standard library only;
requires Go 1.25.

### Identity sync (contract 1.1)

- `identitysync`: contract types and validation (`Descriptor`, `Account`,
  `AccountPage`, `Identity`, `ValidAccountID`, `ValidCursor`,
  `ValidIdentity`), capability constants and the legacy 1.0 descriptor.
- `identitysync/contract/identity-sync.openapi.yaml`: the machine-readable
  contract; `docs/integrations/identity-sync-contract.md` is normative.
- `identitysync/adapter`: a `net/http` handler implementing the whole
  application side (descriptor, paginated accounts, idempotent linking with
  both conflict cases, session revocation, the 401/403 token rules, and the
  test-only session probe behind `IDENTITY_SYNC_TEST_PROBE=1`).
  `identitysync/adapter/memory` is an in-memory reference store.
- `identitysync/conformance`: checks any application over real HTTP for
  authorization, descriptor, pagination, linking, sessions, retry safety
  (`retries.*`: repeated page reads, concurrent duplicate links and revokes,
  `Retry-After` on overload) and transport rules.

### Platform core

- Module inventory, dependency validation and lifecycle contracts; verified
  claim identity, session-store and authorization-resolver contracts;
  application descriptor and lifecycle predicates; contextual authorization
  and scope-assignment contracts; worker status.
