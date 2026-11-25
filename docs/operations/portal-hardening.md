# Portal authentication and release hardening

This page records the authentication, configuration and deployment hardening applied to the portal, how to verify it, and the reliability work that remains. It is for developers and operators reviewing the portal's release safety.

## Implemented changes

- `DEV_AUTH_BYPASS` is loaded through the same Viper configuration as other settings. It defaults to false. Only `development`, `dev`, and `local` permit it; staging, production, and test startup reject it. Middleware also checks the effective configuration. Keep it disabled in deployed environments.
- Bootstrap passes the effective configuration into the router. CORS extracts origins from frontend URLs, deduplicates them, rejects unsafe URL forms, and adds localhost defaults only for development. The existing MOH production origin remains supported.
- Refresh returns `503 SESSION_UNAVAILABLE` if Redis cannot read or persist the session. Read outages preserve the browser session without exchanging tokens. Failed writes after token rotation clear the session and legacy token cookies, preserving in-flight OAuth cookies. A successful refresh audit is recorded only after persistence succeeds.
- The shell loads the selected `config.js` through Vite's base path. Hostname no longer overrides the selected microfrontend mode. `npm run config:dev`, `config:local-remote`, and `config:prod` select the runtime explicitly. Production builds continue to use the existing production API URL.
- PR checks include architecture/configuration checks and an independent production frontend build with import-map, staged-runtime, and bundle-budget checks.
- Deployment verification (`verify` in `infra/scripts/cd_server.py`) requires backend `/health/ready`, matches the backend `/version` and frontend `/version-manifest.json` version and commit against the deployment manifest, and runs `infra/scripts/verify-frontend-deployment.mjs` against the served shell, runtime config, import map, all portal app/package entrypoints and shell assets. The verifier rejects missing assets and SPA HTML fallbacks; it runs in a separate `node:22-alpine` container on the frontend container's network. When `verify_public_edge` is enabled, the public `/`, `/ssobackend/health/ready`, `/ssobackend/swagger/openapi.json` and `/ssobackend/version` URLs are also checked. None of this is a browser login test.
- A failed rollout is rolled back by the CD transaction in `infra/scripts/cd_server.py`: it restores the previous verified release (image digests plus infrastructure/env snapshot) recorded in `state/deployment.json`. The first managed deployment has no previous release to restore. See [backend rollback](../rollback.md).

## Verification

From the repository root:

```sh
node --test infra/scripts/verify-frontend-deployment.test.mjs
```

From `backend/`:

```sh
go test ./internal/config ./internal/api ./internal/middleware ./internal/features/auth ./internal/features/authsession
go vet ./...
go test ./...
```

From `frontend/`:

```sh
npm run lint
npm run typecheck
npm run config:validate
npm run architecture:check
npm run mf:doctor
npm run rbac:doctor
npm run audit:packages
npm run config:prod
FRONTEND_ASSET_BASE_URL=/ FRONTEND_BASE_PATH=/ npm run build:docker
npm run audit:import-map
npm run runtime:verify
npm run bundle:budget
```

For a running frontend, execute the smoke check from the repository root:

```sh
FRONTEND_SMOKE_URL=http://127.0.0.1:3000/ node infra/scripts/verify-frontend-deployment.mjs
```

Use `npm run config:dev` before returning to local development. Configuration selection writes `frontend/public/config.js`.

CI (see [CI checks](../ci-checks.md)) must pass before release.

## Remaining reliability work

- Decouple remote, DWH, and DQA availability from core startup with domain-specific availability responses and health reporting. This needs coordinated changes to repositories, workers, and readiness checks; do not simply replace failed connections with nil.
- Add multi-instance refresh coordination and test concurrent requests, token rotation, and logout races.
- Add browser tests covering Keycloak login, session expiry, permission denial, deep links, and module mount/unmount behaviour.
- Separate API and worker execution when independent scaling is required, after checking queue claiming, retries, and duplicate-delivery handling.

No database isolation or worker extraction is included in this change.
