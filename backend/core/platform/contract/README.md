# Platform API contract

`platform.openapi.yaml` is the machine-readable contract for the portal's platform
API (`/api/v1/platform/*`): caller identity, readiness, notifications and per-user
access decisions. Application backends in other repositories build and test their
clients against it. The guide for those teams is
[`docs/integrations/platform-api.md`](../../../../docs/integrations/platform-api.md).

## How it is produced

The file is **generated, never edited by hand**. `go run ./cmd/cli openapi generate`
(run from `backend/`, or `make contracts-generate` from the repository root) writes the
full API description `backend/docs/openapi.yaml` from the registered routes and handler
types, then extracts this contract from it:

- only `/api/v1/platform/*` paths;
- the component schemas those paths reference, and their dependencies;
- bearer authentication only;
- no internal `x-go-*` annotations.

Summaries, descriptions, required headers (`x-required-parameters`) and error statuses
the generator can't infer (`x-error-responses`) come from
`backend/docs/openapi.overrides.yaml`.

CI fails when the contract is stale (`openapi validate`) or when a pull request breaks
it (`openapi platform-compat`, below).

## Versioning

`info.version` follows semantic versioning and is set by `PlatformContractVersion` in
`backend/internal/apidocgen/platform.go`. The URL carries the major version
(`/api/v1`).

**Within a major version, changes are additive only.** Allowed:

- new endpoints;
- new optional request fields and parameters;
- new response fields (clients must ignore fields they don't know);
- new error statuses for new failure cases;
- new capability scopes for new endpoints.

Bump the **minor** version for additive changes and the **patch** version for
description-only changes.

**Breaking changes need a new major version**, served at a new path (`/api/v2/...`)
alongside the old one. Breaking means any of:

- removing or renaming an endpoint, operation or success response;
- removing a response field, or making an always-present field optional;
- adding a required parameter, request field or request body, or making an optional
  one required;
- changing the type of a parameter, request field or response field;
- changing what an existing field or scope means, or broadening an existing scope.

## Deprecation

When `v2` ships, `v1` stays available for a deprecation period agreed with every
application owner bound in the portal registry (at least one release cycle, and long
enough for each owner to upgrade). Announce the period before it starts, mark
deprecated operations with `deprecated: true` in the overrides, and remove `v1` only
once no bound application still calls it.

## Checking compatibility

CI compares the contract in a pull request with the default branch's copy:

```sh
cd backend
go run ./cmd/cli openapi platform-compat --base /path/to/base/platform.openapi.yaml
```

It lists each breaking change and exits non-zero unless the major version was raised.
The check covers shapes, not behaviour: a change in meaning without a change in shape
still needs a new major version, and review must catch it.
