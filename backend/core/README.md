# Portable platform core

This host-integrated package owns the standard-library-only inventory,
dependency validation and lifecycle contracts used by compiled modules.
See [the platform core boundaries](../../docs/platform-core.md) for required
capabilities, host adapters and the remaining minimal-boot work.

The existing `backend/module` Gin adapter delegates to this runtime.
Core capabilities are composed by the host and are not optional module IDs.

Run `python3 infra/scripts/test-platform-core.py` from the repository root
to verify the whole core outside the host tree with an empty dependency cache
and workspaces/downloads disabled. The check builds, vets and runs uncached
race tests. This package is not independently published or version-pinned yet.


Phase 2 adds verified-claim identity, generic host session-store and
authorization-resolver/access contracts. Permission/role/system predicates are
shared without relocating application policy catalogues. Host authentication,
cookie/token parsing, Redis payload types, provider selection and static
defaults remain outside this package. `ResolveAuthorization` discards partial
grants on provider error; it never authenticates credentials or invents grants.

Phase 3 adds a governed application descriptor/read-only registry port,
lifecycle transition and review-authority predicates, and metadata validation.
Host adapters retain reviewed manifests, provisioning, launch-behavior checks,
HTTP DTOs, transactional audit and access-role persistence.

Generic contextual-authorization, scope-assignment and hierarchy contracts
preserve exact functional/system checks, ordered direct/descendant selection
and failure propagation. Host ID/explanation types remain type parameters;
core imports no UUID, geographic provider or database packages. Effective
assignment loading, alias resolution and selected-context middleware remain
host responsibilities. Core-only startup is deferred.

## External Go consumers

The module path is `github.com/mohuganda/moh-sso-dashboard/backend/core`.
Release tags for this nested module use the `backend/core/vX.Y.Z` prefix.
The dashboard currently uses a local replacement so it builds against the
same source tree as the module.

The `identitysync.AccountStore` contract lets a host expose eligible accounts
and persist OIDC identity links keyed by issuer and subject. The host owns
authorization, storage, and roles. Repeating an existing link is safe, and
conflicting links must fail atomically.
