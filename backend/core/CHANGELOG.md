# backend/core changelog

Release tags use the `backend/core/vX.Y.Z` prefix. Below 1.0 a minor version
may contain breaking changes. Check IDs in `identitysync/conformance` are
stable within a minor version.

## v0.2.0 (unreleased)

### Module kit

- `modulekit`: the HTTP contract between the platform host and compiled
  application modules. The host attaches verified request state
  (`WithRequest`: access, request ID, selected health context); modules
  declare `Endpoint`s whose access requirements (`Public`, `Admin`, `System`,
  `Permission`, `AnyPermissions`, `AllPermissions`) and optional `RateLimit`
  the `Router` enforces with the host's exact response bodies. `Context`
  offers request helpers and the platform response envelope; `BindJSON`
  applies `binding:"required|omitempty|oneof=..."` tags.
- `HTTPModule` pairs a router with its OpenAPI document and rejects routes
  and operations that do not match.
- Service contracts modules consume: `Notifier`/`Notification` (platform
  notifications and their deliveries), `Platform` (installation name and
  administrator contact) and `HealthContextDirectory` (read-only
  health-context hierarchy and aliases).

## v0.1.0 (unreleased)

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
