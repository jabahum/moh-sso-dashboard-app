# Swagger / OpenAPI

The running backend serves self-hosted Swagger UI at **`/swagger/`**:

- Direct backend: <http://localhost:9000/swagger/>
- When using the Express host: <http://localhost:3000/backend/swagger/>
- YAML contract: `/swagger/openapi.yaml`
- JSON contract: `/swagger/openapi.json`

The UI, specification and Swagger UI 5.33.1 assets are embedded in the Go binary.
Docker images do not need a separate documentation directory or runtime CDN access.
Relative asset/spec URLs and the relative OpenAPI server URL retain reverse-proxy
prefixes such as `/backend`.

## Authenticate and try requests

Use **Authorize → bearerAuth** with an existing access token, or sign into the portal
first and open Swagger UI on that same origin. The API's HttpOnly `sso_session` cookie
is sent by the browser; Swagger UI cannot set it through the cookie authorization
input. The UI does not persist bearer authorization across reloads and does not send
the specification to Swagger's external validator. API authentication, permissions,
system/group checks and rate limiting remain unchanged.

`auth/me` and `auth/refresh` use their existing cookie-based authentication contracts;
the specification does not claim that a bearer token alone can refresh a browser session.

Open login/launch routes through a browser for the redirect flow; the Swagger fetch
client cannot complete an interactive Keycloak sign-in. Upload operations offer file
selectors for multipart bodies. Attachment uploads support both multipart files and
JSON/base64. Download/view/export operations document binary responses.

## Generate and validate

Run from `backend/`:

```bash
make openapi-generate
make openapi-validate
go test ./internal/apidocgen ./docs
```

Equivalent CLI commands:

```bash
go run ./cmd/cli openapi generate
go run ./cmd/cli openapi validate
go run ./cmd/cli openapi export --output /tmp/moh-openapi.yaml
```

Generation needs the source checkout and its Go dependencies, but no running database,
Redis, Keycloak, or environment secrets. Export uses the specification embedded when
the CLI binary was compiled; rebuild the binary after regenerating documentation.

`internal/apidocgen` builds the real Gin route inventory with inert handlers, then reads
typed handler ASTs using Go's package loader. It follows helpers accepting a Gin context
to discover shared query/pagination bindings. Generated contracts include:

- Every registered HTTP method and path, including all path parameters.
- JSON bodies and response DTOs with JSON field names and binding constraints.
- Query/form/header parameters, multipart uploads and binary response types.
- Success/error envelopes, actual static status codes, and redirects.
- Public route exceptions, session/bearer authentication, and explicit route permissions.

Dynamic maps and raw JSON remain open schemas. Parameter validation or conditional
business rules that are not expressed in Go binding tags may need supplemental metadata.
Permission extensions list explicit route middleware requirements; they are not a
replacement for inherited system/group authorization enforced by the server.

Edit routes/DTOs/handlers; do not edit `openapi.yaml` directly. You can regenerate
locally, or let PR CI generate it automatically. Curated
summaries and descriptions live in `openapi.overrides.yaml`. Its optional
`x-required-form-fields` and `x-form-properties` metadata capture conditional/manual form
contracts, such as the required document storage location. Other operation contracts
are derived from code rather than copied from the older starter specification.

`TestCompleteContractIsCurrent` regenerates and compares the committed document during
the normal `go test ./...` CI checks. Missing routes, stale DTOs, unresolved references,
missing path parameters, and duplicate operation IDs fail validation. The generated
specification has also been independently checked with `openapi-spec-validator`.

Swagger UI assets come from the [official v5.33.1 distribution](https://github.com/swagger-api/swagger-ui/tree/v5.33.1/dist).
Both the project license and bundled third-party notices are included under `swagger/`.
For UI configuration, see [Swagger's configuration documentation](https://swagger.io/docs/open-source-tools/swagger-ui/usage/configuration/).

## Automatic updates in CI

The Update OpenAPI contract workflow regenerates `docs/openapi.yaml` and runs
validation and generator tests. PR Quality Checks still require a committed,
current document and run the full backend test suite.
If a same-repository PR changes the generated document, a separate job commits
only that file back to its branch after generation checks pass. This includes
route/DTO/override changes and updates to `backend/VERSION`. Generation runs with
read-only repository permissions; the write job copies the validated artifact
without running PR code. It uses a normal fast-forward push and refuses to update
a branch that has changed since generation.

The bot explicitly dispatches PR Quality Checks on its new commit because pushes
using `GITHUB_TOKEN` do not automatically trigger another PR workflow. The dispatched
quality run requires no contract drift. Bot pushes do not rerun the update workflow,
so it does not create a commit loop. Repository settings must allow Actions to write contents
and dispatch workflows; branch rules are respected and push failures fail CI.

Fork PRs receive a `generated-openapi-<run-id>` artifact containing `openapi.yaml`.
Download it and commit it to `backend/docs/openapi.yaml`, or generate it locally.
CI does not push to forks. Builds and manual runs also reject uncommitted drift,
so release artifacts cannot silently ship a contract different from the source.
