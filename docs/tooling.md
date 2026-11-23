# MOH SSO Dashboard tooling

This page lists the backend CLI commands, frontend checks and verification commands used during development. It is for developers. This project uses lightweight operational and architecture tools instead of a large tooling framework.

See the [Make command reference](make.md) for every deployment and generation target,
its prerequisites, options, effects and examples.

## Backend CLI

Run backend commands from `backend/`.

```sh
go run ./cmd/cli doctor
go run ./cmd/cli config validate
go run ./cmd/cli system-rbac validate --file config/system-rbac.seed.yaml
go run ./cmd/cli system-rbac doctor --file config/system-rbac.seed.yaml
go run ./cmd/cli system-rbac explain --realm-role admin --client-role outbreak-management:viewer
go run ./cmd/cli system-rbac sync-keycloak --draft-file /tmp/system-rbac.seed.yaml
go run ./cmd/cli dev seed --system-rbac
go run ./cmd/cli openapi validate
```

`system-rbac sync-keycloak` defaults to dry-run behavior. Use `--apply` only after reviewing the generated draft.

## Frontend tooling

Run frontend commands from `frontend/`. With `nvm`, use the project Node version first. `nvm use` reads `frontend/.nvmrc` (currently 20.20.1); CI, the release workflows and the frontend Docker images use Node 22.

```sh
source ~/.nvm/nvm.sh
nvm use
```

Useful checks:

```sh
npm run config:validate
npm run architecture:check
npm run mf:doctor
npm run deploy:doctor
npm run version:audit
npm run audit:packages
npm run audit:import-map
npm run audit:publishability
```

Release gates:

```sh
npm run preversion:check
npm run release:check
```

## Verification

Backend:

```sh
GOCACHE=/private/tmp/moh-sso-go-build go test ./...
```

Backend architecture boundary check only:

```sh
cd backend
GOCACHE=/private/tmp/moh-sso-go-build go test ./internal/architecture
```

Backend observability/event checks:

```sh
cd backend
GOCACHE=/private/tmp/moh-sso-go-build go test ./internal/middleware ./internal/events
```

Frontend:

```sh
source ~/.nvm/nvm.sh
nvm use
npm run typecheck
npm run lint
npm run build:shell
```
