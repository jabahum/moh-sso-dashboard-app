# Portal authentication and release hardening

## Implemented changes

- `DEV_AUTH_BYPASS` is loaded through the same Viper configuration as other settings. It defaults to false. Only `development`, `dev`, and `local` permit it; staging, production, and test startup reject it. Middleware also checks the effective configuration. Keep it disabled in deployed environments.
- Bootstrap passes the effective configuration into the router. CORS extracts origins from frontend URLs, deduplicates them, rejects unsafe URL forms, and adds localhost defaults only for development. The existing MOH production origin remains supported.
- Refresh returns `503 SESSION_UNAVAILABLE` if Redis cannot read or persist the session. Read outages preserve the browser session without exchanging tokens. Failed writes after token rotation clear the session and legacy token cookies, preserving in-flight OAuth cookies. A successful refresh audit is recorded only after persistence succeeds.
- The shell loads the selected `config.js` through Vite's base path. Hostname no longer overrides the selected microfrontend mode. `npm run config:dev`, `config:local-remote`, and `config:prod` select the runtime explicitly. Production builds continue to use the existing production API URL.
- PR checks include architecture/configuration checks and an independent production frontend build with import-map, staged-runtime, and bundle-budget checks.
- Deployment requires backend readiness, validates frontend release tags, and checks the served shell, runtime config, all portal app/package entrypoints, and shell assets. The verifier rejects missing assets and SPA HTML fallbacks. The existing Node-based frontend image supplies its runtime. This is an asset smoke check, not a browser login test or a public reverse-proxy check.
- Deployment failures, including restart failures, trigger rollback through an exit trap. A previously successful deployment must have recorded its image tags for automatic rollback to be possible.

## Verification

From the repository root:

```sh
node --test scripts/verify-frontend-deployment.test.mjs
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
FRONTEND_ASSET_BASE_URL=/portal FRONTEND_BASE_PATH=/portal npm run build:docker
npm run audit:import-map
npm run runtime:verify
npm run bundle:budget
```

For a running frontend, execute the smoke check from the repository root:

```sh
FRONTEND_SMOKE_URL=http://127.0.0.1:3000/portal/ node scripts/verify-frontend-deployment.mjs
```

Use `npm run config:dev` before returning to local development. Configuration selection writes `frontend/public/config.js`.

Implementation-session verification: the five isolated Node verifier tests passed, and the edited workflows passed YAML parsing and shell syntax checks. Go tests, the frontend build, and browser flows were not executed because the connected workspace did not expose a command runner. CI must pass before release.

## Remaining reliability work

- Decouple remote, DWH, and DQA availability from core startup with domain-specific availability responses and health reporting. This needs coordinated changes to repositories, workers, and readiness checks; do not simply replace failed connections with nil.
- Add multi-instance refresh coordination and test concurrent requests, token rotation, and logout races.
- Add browser tests covering Keycloak login, session expiry, permission denial, deep links, and module mount/unmount behaviour.
- Separate API and worker execution when independent scaling is required, after checking queue claiming, retries, and duplicate-delivery handling.

No database isolation or worker extraction is included in this change.
