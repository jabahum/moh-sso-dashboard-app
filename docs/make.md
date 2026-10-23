# Make command reference

Run these commands from the repository root. `make help` prints the command
index; plain `make` builds the backend. Deployment targets also work with
`make -C infra <target>`. Contract and frontend-generation targets belong to the
root Makefile; use `make -C backend help` for the backend's own commands.

## Requirements

- Compose commands: Docker with Compose 2.24.4 or newer. Starting services also
  requires a running Docker daemon.
- API generation/check/export: Go matching `backend/go.mod`, with Go modules
  available. A running application or production credentials are not required.
- Frontend manifests: Node 22, npm and installed frontend dependencies (`cd
  frontend && npm ci`).
- Infrastructure contracts: Python 3, Docker Compose CLI and jq.
- Runtime/Nginx checks: a running Docker daemon; the Nginx suite also uses OpenSSL.
  Runtime checks may download images/packages to build their disposable fixtures.
- SQL bindings: an installed `sqlc` command.

## Choose a deployment mode

`COMPOSE_ENV` defaults to **prod** for generic `compose-*` commands.

| Value | Topology | Default env files |
| --- | --- | --- |
| `prod` | Application containers; Nginx runs outside Compose | `infra/env/prod.env` |
| `dev` | Source builds with local Keycloak, databases, Redis and MailHog | `infra/env/dev.env`, falling back to `infra/env/dev.env.example` |
| `gateway` | Production plus container Nginx; app ports remain unpublished | `infra/env/prod.env` |
| `remote` | Production reachable from Nginx on another machine | `infra/env/prod.env` plus `infra/env/remote-proxy.env` |
| `external` | Development with external Keycloak | `infra/env/external-dev.env` |

Prepare private files from the corresponding examples and edit them before use:

```bash
cp infra/env/dev.env.example infra/env/dev.env
cp infra/env/prod.env.example infra/env/prod.env
cp infra/env/remote-proxy.env.example infra/env/remote-proxy.env
# For external Keycloak:
cp infra/env/external-dev.env.example infra/env/external-dev.env
```

These are alternatives; only create the files for the mode you use. Production
also needs release tags, reachable data services and real files in `infra/secrets/`.
Remote mode needs the application server's private IP and matching Nginx/firewall
configuration. External mode needs real Keycloak URLs, realm and client credentials.
See the [deployment guide](deployment.md) for that setup.

## Compose commands

Each name below is a separate Make target. The `|` and slash notation sometimes
used in summaries denotes alternatives; do not paste it into a shell command.

| Target | Behavior | Supports `SERVICE` |
| --- | --- | --- |
| `make compose-config` | Resolve and validate the entire selected Compose model; quiet on success. Does not start services or prove live connectivity. | No |
| `make compose-up` | Start/reconcile containers in the background (`up -d`). Does not force a source rebuild. | Yes |
| `make compose-down` | Stop/remove the selected project's containers and network. Keeps named data volumes. | No; affects the whole selected project |
| `make compose-logs` | Follow logs until interrupted. | Yes |
| `make compose-ps` | Show container status for the selected project. | No |
| `make compose-pull` | Download the selected images. Does not restart containers. | Yes |
| `make compose-restart` | Restart existing containers. Does not apply changed env files or recreate containers with new images. | Yes |

Use `compose-up` after an environment or image-tag change to reconcile the
containers, and `compose-pull` first when downloading a production release.
These commands do not deploy Kubernetes or run contract generation.

```bash
make compose-config COMPOSE_ENV=dev
make compose-up COMPOSE_ENV=dev SERVICE=backend
make compose-logs COMPOSE_ENV=dev SERVICE=backend
make compose-ps COMPOSE_ENV=dev
make compose-restart COMPOSE_ENV=dev SERVICE=frontend
make compose-down COMPOSE_ENV=dev
```

### Start shortcuts

| Target | Fixed mode and action |
| --- | --- |
| `make compose-dev-up` | `dev up --build -d`; builds source images and starts local dependencies. |
| `make compose-prod-up` | `prod up -d`; starts the configured production image tags. |
| `make compose-gateway-up` | `gateway up -d`; adds the container gateway. |
| `make compose-remote-up` | `remote up -d`; reads both production and remote settings. |
| `make compose-external-up` | `external up --build -d`; source builds with external Keycloak. |

All five shortcuts accept `SERVICE`. Each selects its own mode, so setting
`COMPOSE_ENV` does not change the shortcut's mode. To stop or inspect that stack,
use a generic command with the matching `COMPOSE_ENV`.

```bash
make compose-dev-up
make compose-logs COMPOSE_ENV=dev
make compose-remote-up
make compose-ps COMPOSE_ENV=remote
```

`make dev` aliases `compose-dev-up`. `make backend-dev` builds/starts the dev API
and its dependencies with attached logs; `make backend-dev-logs` follows API logs.

### Override settings

| Variable | Meaning |
| --- | --- |
| `COMPOSE_ENV` | Mode used by generic Compose targets; default `prod`. |
| `SERVICE` | Optional service for up/logs/pull/restart and start shortcuts; e.g. `backend`, `frontend`, `backend-db`. Up may also start required dependencies. |
| `ENV_FILE` | Replace the mode's main env file. |
| `REMOTE_ENV_FILE` | Replace the extra remote overlay; only remote mode reads it. |

Override paths are absolute or relative to the repository root, including when
running `make -C infra`. Private env files are passed to Compose, never sourced
as shell scripts. Exported shell variables override values in Compose's CLI env
files. Keep the same mode and file overrides on subsequent commands for a stack.

```bash
make compose-config COMPOSE_ENV=dev ENV_FILE=infra/env/team-dev.env
make compose-up COMPOSE_ENV=remote \
  ENV_FILE=/srv/portal/prod.env REMOTE_ENV_FILE=/srv/portal/network.env
make -C infra compose-ps COMPOSE_ENV=dev
```

Production and development use separate Compose projects/volumes but share some
host port numbers by default. Generic `compose-down` acts on the chosen project's
whole stack; it has no `SERVICE` filter and does not pass `-v`.

## Contracts and generated files

Scheduler settings are generated separately with
`make scheduler-env COMPOSE_ENV=external`. It updates the selected private
application env (creating the default from its mode template if missing) and
creates a private Health BI companion file without contacting
Keycloak or restarting services. Existing signing keys and credentials are kept;
missing worker credentials must come from Keycloak. `ENV_FILE`, `HEALTHBI_ENV_FILE`,
`HEALTH_BI_URL`, `HEALTH_BI_PUBLIC_URL` and `SCHEDULER_CLIENT_SECRET_FILE` customize
the operation. See [scheduler configuration](scheduler-configuration.md).
`make scheduler-env-check` tests generation and credential preservation and is
included in `make infra-check`.

| Target | Reads/writes and result |
| --- | --- |
| `make contracts-generate` | Read registered API routes, handler DTOs and `backend/docs/openapi.overrides.yaml`; rewrite `backend/docs/openapi.yaml`. |
| `make contracts-check` | Validate route/schema coverage and generated-file freshness. Fails on stale/invalid contracts; does not rewrite them. |
| `make openapi-generate` | Alias for API contract generation. |
| `make openapi-validate` | Alias for API contract validation. |
| `make openapi-export` | Export the currently embedded spec; default output `/tmp/moh-openapi.yaml`. Does not regenerate a stale source spec. |
| `make frontend-manifests` | Rewrite `frontend/public/import-map.json` and `frontend/public/version-manifest.json` using the frontend generators. |
| `make generate` | Run API contract generation and frontend manifest generation. Does not build applications, deploy services or regenerate SQL bindings. |
| `make sqlc-generate` | Run `sqlc generate` from `backend/` using `backend/sqlc.yaml`; rewrite Go SQL query bindings. |

The backend embeds its OpenAPI resources, so those inputs/output stay in
`backend/docs`. Review and commit intentional changes to generated tracked files.

```bash
make contracts-generate
make contracts-check
make openapi-export OPENAPI_OUTPUT=/tmp/api.yaml
make frontend-manifests
make generate
```

`OPENAPI_OUTPUT` accepts an absolute path or a path relative to the repository
root. Root Make resolves it before delegating to the backend. Go can be selected
with `GO_CMD=/path/to/go`; frontend targets accept `NPM=/path/to/npm`.

## Infrastructure checks

| Target | Scope |
| --- | --- |
| `make infra-check` | CD transaction tests plus Compose/env coverage, service alignment, source/mount paths, launcher modes, safe env-file handling, secret loading and public-config serialization. Does not start the application stack. |
| `make infra-runtime-check` | Build the actual runtime stages with tiny fixture build outputs; check frontend startup config, backend secrets/user/uploads/version arguments and a disposable Keycloak readiness probe. |
| `make infra-nginx-check` | Test gateway, same-host and remote-IP HTTPS routing, SSO, cache/security headers, uploads, WebSocket upgrade, logs, proxy trust and DNS recovery. |
| `make infra-test` | Run all three infrastructure checks. |
| `make workflow-check` | Lint active GitHub workflows and deployment shell scripts; requires actionlint and ShellCheck. |
| `make migrations-check` | Test fresh/previous-schema primary and remote migrations in disposable PostgreSQL; requires Docker and Go. |
| `make cd-runtime-check` | Verify actual container rollout, failed-version detection, rollback, env snapshots and persistent fixture data. Requires Docker and Python. |

Container suites create uniquely named temporary containers, networks and/or
images and clean them up. They preserve running application stacks and data
volumes. Runtime fixtures do not compile the complete apps or verify real remote
network/firewall access. API contract validation is separate: use
`make contracts-check` alongside these checks.

## Continuous delivery and server commands

See the [CD runbook](cd-pipeline.md) for the separate image-build, deployment and
rollback workflows, server bootstrap, and required GitHub/server configuration.

| Target | Behavior |
| --- | --- |
| `make cd-check` | Test bundle validation, image pairing, config snapshots, locking, rollout failure and rollback/recovery. Python only; no server or Docker daemon needed. |
| `make cd-preflight` | Check Docker/Compose and private deployment configuration on the prepared application server. Does not start containers. |
| `make cd-status` | Print the server's recorded current/previous releases. |
| `make cd-verify` | Check the currently recorded release's runtime metadata, modules and configured public HTTPS edge. |
| `make cd-rollback` | Restore and verify the previous recorded server release; changes running containers. |

Server commands default to `DEPLOY_CONFIG=/etc/moh-sso/deploy.json`; override it
for another absolute configuration path. Run them as the configured deployment
account on the application server. On servers without a source checkout, invoke
`python3 /opt/moh-sso/current/infra/scripts/cd_server.py <command>` instead of Make.
For interrupted rollout recovery, use the rollback workflow as documented.

See [CI checks](ci-checks.md) for prerequisites, browser commands, fixture
isolation, failure artifacts and the required-check names for branch protection.

For Kubernetes, local SSO, image building and releases, see
[deployment](deployment.md), [tooling](tooling.md) and `make release-help`.
`make dev-up` and `make prod-up` retain their Kubernetes meaning; Docker Compose
uses the explicit `compose-*-up` names.
