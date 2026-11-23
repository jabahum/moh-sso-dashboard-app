# Getting started

This page gets a developer from a fresh checkout to a running portal, then points to
the guide for the next task. For a map of all documentation, see the
[documentation overview](../documentation/pages/index.md).

## What you are running

| Component | What it is |
| --- | --- |
| Frontend shell | The portal host under `/portal`: layouts, routing, sign-in, app launcher and single-spa orchestration. |
| Frontend apps | Product modules such as users, clients, RBAC, surveillance, documents, audit and email, each an npm package. |
| Frontend packages | Shared UI, API hooks, auth, runtime config, state, types and microfrontend contracts. |
| Go backend | The API server: feature modules, Keycloak integration, RBAC sync, notifications, email, audit, documents and surveillance. |
| Keycloak | The identity source of truth for users, credentials, realm roles, clients and client roles. |
| PostgreSQL | Portal data: RBAC metadata, announcements, audit logs, notifications, email queue and feature data. |
| Redis | Cache, session and rate-limit support where configured. |

## Prerequisites

- Docker with Docker Compose
- Go at the version in `backend/go.mod`
- Node.js at the version in `frontend/.nvmrc` (nvm recommended), and npm
- Python 3 and Chrome or Chromium, only for `make dev` with Health BI

## Choose how to run it

| You want to | Use |
| --- | --- |
| Run everything in containers with live reload (most developers) | [Containers](#run-everything-in-containers) |
| Also run the sibling Health BI project for reports | [Portal with Health BI](#run-the-portal-with-health-bi) |
| Run the backend or frontend directly on your machine | [Native processes](#run-the-backend-or-frontend-natively) |

### Run everything in containers

```bash
cp infra/env/dev.env.example infra/env/dev.env
make compose-dev-up
```

This builds the API and portal from source and starts Keycloak, Redis, PostgreSQL, an
upload/warehouse fixture database and MailHog. Backend changes reload through Air and
frontend changes through Vite.

| Service | Address |
| --- | --- |
| Portal | `http://localhost:3000/portal/` |
| API / Swagger | `http://localhost:9000/swagger/` |
| Keycloak | `http://localhost:8081` |
| MailHog | `http://localhost:8025` |

See the [development quick start](deployment.md#development-quick-start) for the full
port list, external Keycloak, and module servers.

### Run the portal with Health BI

```bash
make dev
```

`make dev` (alias `make sso-up`) starts the SSO infrastructure, the sibling Health BI
checkout and its test database, then the portal. Prerequisites, `make sso-status` and
`make sso-down` are in [start the portal and Health BI together](local-healthbi-startup.md).

### Run the backend or frontend natively

Backend, from `backend/`:

```bash
cp ../infra/env/dev.env.example app.env   # point DB/Redis at localhost and the published ports
go run ./cmd/server
```

Frontend, from `frontend/`:

```bash
nvm use
npm install
npm run config:dev
npm run dev
```

The shell is served under `/portal`. Frontend modes, ports and remote module servers
are described in [frontend development](../frontend/docs/development.md).
`make dev-native` runs both when infrastructure is managed separately.

## Check your changes

```bash
cd backend && go test ./...
cd frontend && npm run typecheck && npm run lint && npm run build:shell
```

Before a release, `npm run release:check` in `frontend/` runs the full frontend
readiness check. The [frontend verification guide](../frontend/docs/verification.md)
and [CI checks](ci-checks.md) list what CI runs, including browser tests.

## Common first-run problems

| Symptom | Fix |
| --- | --- |
| Sign-in succeeds but you are not signed in | Leave `COOKIE_DOMAIN` empty; never use `localhost`. See [cookie domain](configuration.md#cookie-domain-in-local-development). |
| Blank page under `/portal` | The wrong runtime config is active. Run `npm run config:dev` in `frontend/`. See [frontend troubleshooting](../frontend/docs/troubleshooting.md). |
| Vite cannot resolve a microfrontend package | Run `npm run mf:doctor` and `npm run audit:import-map` in `frontend/`. |
| A user has Keycloak roles but sees no apps | Follow [user has role in Keycloak but portal does not show app](rbac-keycloak-sync.md#user-has-role-in-keycloak-but-portal-does-not-show-app). |
| Startup RBAC sync finds no realm export | Set `RBAC_STARTUP_SYNC_REALM_EXPORT_PATH`; live Keycloak sync still runs when enabled. See [startup sync](rbac-keycloak-sync.md#startup-sync). |

## Next steps

| Task | Guide |
| --- | --- |
| Configure environment variables | [Configuration reference](configuration.md) |
| Find a Make target | [Make command reference](make.md) |
| Build a backend module | [Platform core](platform-core.md), then [module development](module-development.md) |
| Build a frontend app | [Adding a new app](../frontend/docs/adding-a-new-app.md) |
| Connect an external application | [Onboarding handbook](platform-onboarding.md) |
| Set up roles and permissions | [RBAC usage](rbac-usage.md) |
| Look up an endpoint | [API reference](api-reference.md) |
| Deploy or release | [Deployment](deployment.md), [releasing](releasing.md) |
