# MOH SSO Dashboard

MOH SSO Dashboard is the Integrated Health Portal for Ministry of Health Uganda. It is a
central, authenticated portal for launching health information systems, with shared
tools for user and client administration, role-based access control (RBAC),
announcements, notifications, email, audit logs, documents, data quality, surveillance
dashboards and reports.

The backend is a feature-first Go modular monolith. The frontend is a single-spa shell
that hosts independently published microfrontend apps.

## Start here

| You are | Read first | Then |
| --- | --- | --- |
| A developer new to the project | [Getting started](docs/getting-started.md) | [Configuration reference](docs/configuration.md), [Make commands](docs/make.md) |
| Connecting an application or system to the portal | [Onboarding handbook](docs/platform-onboarding.md) | [System onboarding](docs/system-onboarding.md), [onboarding record template](docs/templates/application-onboarding-record.md); for CLIMS and POES, [Connected applications](docs/integrations/connected-applications.md) |
| Managing users, roles and access | [RBAC usage](docs/rbac-usage.md) | [RBAC and Keycloak sync](docs/rbac-keycloak-sync.md), [RBAC governance](docs/rbac-governance.md) |
| Deploying or operating the portal | [Deployment](docs/deployment.md) | [Platform runbook](docs/operations/platform-runbook.md), [releasing](docs/releasing.md), [rollback](docs/rollback.md) |
| Looking up an endpoint | [API reference](docs/api-reference.md) | Swagger UI at `/swagger/` on a running backend |

All guides are also published as a documentation site with a sidebar and search; see
[Documentation site](#documentation-site).

## Quick start

```bash
cp infra/env/dev.env.example infra/env/dev.env
make compose-dev-up
```

Open `http://localhost:3000/portal/`. This starts the API, the portal, Keycloak,
PostgreSQL, Redis and MailHog with live reload. For other ways to run the portal,
prerequisites and first-run problems, see [Getting started](docs/getting-started.md).

## How the platform fits together

| Component | Responsibility |
| --- | --- |
| Frontend shell | Hosts layouts, routing, sign-in, the app launcher, import maps and single-spa orchestration. |
| Frontend apps | Product modules such as users, clients, RBAC, surveillance, documents, audit and email, each published to npm. |
| Frontend packages | Shared UI, API hooks, auth helpers, runtime config, state, types, utilities and microfrontend contracts. |
| Go backend | Gin API server with feature modules, Keycloak integration, RBAC sync, notifications, email, audit, documents and surveillance APIs. |
| Keycloak | Identity source of truth for users, credentials, realm roles, clients and client roles. Keycloak clients are the portal's systems. |
| PostgreSQL | Portal data: RBAC metadata, announcements, audit logs, notifications, the email queue and feature data. |
| Redis | Cache, session and rate-limit support where configured. |

Every backend and frontend module, its stage and its dependencies are listed in
[module coverage](docs/module-coverage.md). Architecture guides:
[platform core](docs/platform-core.md), [backend](backend/README.md),
[frontend](frontend/docs/architecture.md) and
[modular distribution](docs/modular-distribution.md).

## Repository layout

```text
backend/        Go API, database schemas, OpenAPI contract and backend tooling
frontend/       Shell, microfrontend apps, shared packages and frontend tooling
infra/          Compose, Dockerfiles, Nginx, Helm, Keycloak, environment files and deployment scripts
docs/           Platform guides (onboarding, identity, operations, releases, API reference)
documentation/  Docusaurus site that publishes docs/ and frontend/docs/
examples/       Application manifests and platform onboarding examples
scripts/        Repository release and versioning tooling
Makefile        App commands, contract generation and the included infra targets
```

Each area has its own README with details for people working in it:

| README | Covers |
| --- | --- |
| [backend/README.md](backend/README.md) | Backend architecture, module boundaries, response rules, migrations and verification. |
| [frontend/README.md](frontend/README.md) | Frontend workspace, development modes, commands and deployment. |
| `frontend/apps/*/README.md`, `frontend/packages/*/README.md` | One app or shared package. |
| [infra/README.md](infra/README.md) | Deployment file layout. |
| [examples/platform/README.md](examples/platform/README.md) | Developer onboarding examples. |

## Documentation site

The [Docusaurus site](documentation/) publishes `docs/` and `frontend/docs/` with a
shared sidebar, page navigation and Mermaid diagrams. Run it with Node.js 20.6 or later.
If needed, copy `infra/env/dev.env.example` to `infra/env/dev.env` first.

```sh
npm --prefix documentation ci
DOCS_URL="$(node --env-file=infra/env/dev.env -p 'process.env.DOCS_URL')" \
  npm --prefix documentation start -- --port 3001
```

Open http://localhost:3001/docs/. Run `npm --prefix documentation run build` to check
internal links and generate the static site in `documentation/build/`. The
Documentation workflow (`.github/workflows/documentation.yml`) also builds the site on
pull requests and `main` pushes that change `docs/`, `frontend/docs/` or
`documentation/`, and uploads it as an artifact. Set `DOCS_URL` and `DOCS_BASE_URL` at build time when serving from a different
host or path.

When you add a guide, put it in `docs/` (or `frontend/docs/` for frontend-only topics)
and add it to the matching category in `documentation/sidebars.js`. Keep existing file
paths stable; they are the published URLs.
