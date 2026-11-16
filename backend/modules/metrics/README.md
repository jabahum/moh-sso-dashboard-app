# Metrics Go module

This module provides the dashboard metrics service, HTTP handlers, response
types and repository port. It has its own `go.mod` and can be consumed from a
different Go repository. SQL access, authentication, permission resolution,
audit logging and application lifecycle are supplied by the host.

## Use

```sh
go get github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics@v0.1.0
```

Implement `metrics.Repository` using the public records in `metrics/model`.
Construct the service with `metrics.NewService(repository)` and the handler
with `metrics.NewHandler(service)`. Register routes with
`metrics.RegisterAdminRoutes(adminGroup, handler, authorizationMiddleware)`.
The authorization middleware is required. The host should also apply its
authentication, audit and rate limiting middleware to the parent group.

The supplied admin group receives the `/metrics` prefix and all 19 existing
endpoints. Success and error envelopes preserve the portal API format. Hosts
can set the Gin key `request_id` to include `meta.requestId` in responses.

`metrics.Version()` reports the version selected by the consuming binary.
Local filesystem and workspace replacements report `dev`.

## Database ownership

Metrics reads the host's users, client, sessions and audit data through its
repository port. The module owns no database tables or migrations. In the
SSO-DASHBOARD distribution, core migrations run before module initialization.
The existing SQL adapter translates generated query records into module-owned
records without changing database schema or migration history.

## Develop and verify

Run within this directory.

```sh
GOWORK=off go test -race ./...
GOWORK=off go vet ./...
```

For local dashboard integration, run from `backend`.

```sh
go work init . ./modules/metrics
go test ./...
```

If a workspace already exists, add the module using
`go work use ./modules/metrics`. The distribution dependency remains pinned in
`backend/go.mod`. Use `GOWORK=off` to test the published artifact.

## Publish a version

Versions use repository tags with the module directory prefix. For example,
`backend/modules/metrics/v0.1.0` publishes version `v0.1.0` at this module path.
Commit and verify the module source before creating the tag. Verify a clean
consumer can download it before updating the distribution dependency.

The repository can host several independently versioned Go modules. Each module
must keep public APIs independent of other repositories' `internal` packages.
