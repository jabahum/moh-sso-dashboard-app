# Express frontend host

This follows O3's development-host pattern: Express serves the shell, supplies runtime
configuration and an import map, and proxies backend requests. The existing Vite/native
ESM loader and single-spa lifecycle contract remain in use.

## Command helper

From `frontend/`, use `npm run portal -- --help` to discover the common tasks:

```bash
npm run portal -- setup
npm run portal -- build
npm run portal -- serve
npm run portal -- serve users --backend http://127.0.0.1:9000
npm run portal -- serve documents --import-map https://assets.example.com/import-map.json
npm run portal -- modules
npm run portal -- check
```

`build` builds and stages the full frontend. `build users` builds only the Users app;
`preview users` then serves its existing build on the local import-map port. Start the
host in another terminal with an appropriate local/mixed import map to use that preview.
`build shell` builds only the shell for deployments whose modules are hosted elsewhere.
Add `--dry-run` to inspect a command without executing it. Existing host environment
variables remain supported; explicit flags override them. `serve` without an app always
selects the full portal unless the selected profile defines a module. Local paths supplied
as flags resolve from your current directory.
From the repository root, use `node frontend/scripts/portal.mjs` in place of
`npm run portal --`. Node 22+ is required for execution.

## Managed local stack

```bash
npm run portal -- up --profile local
npm run portal -- status
npm run portal -- doctor
npm run portal -- logs backend
npm run portal -- restart backend
npm run portal -- down
```

Start Docker Desktop first. `up` requires a staged frontend (`portal build`) and uses
`docker-compose.dev.yml` plus the tracked `docker-compose.express.yml` overlay. It starts
PostgreSQL, Redis, MailHog and Keycloak, creates the profile's database if absent, registers
the local Express callback on the Keycloak client, starts the backend, waits for strict
backend readiness, then starts Express in Docker and checks its API proxy. The initial
database seed can take several minutes; `--timeout 600` extends each readiness deadline.
Existing databases are never reset and migration versions are never forced.

The `local` profile selects `sso_express_dev`, preserving the older development database.
The backend development image rebuilds the mounted source with Air. Express uses the
small `Dockerfile.host` image and mounts the staged build and import map read-only. Missing
images are built by Compose; use `up --build` or `restart express --build` to rebuild images
after changing host code or dependencies. Backend dependencies must be installed/reachable
by the images' build steps. The current local realm/bootstrap administrator credentials
must match the development Compose settings for automatic callback registration.

`up` refuses to take a port already held by another server. Stop an earlier foreground
`serve` with Ctrl+C (or the old Vite frontend container) before using managed Express.
The helper never kills an unrelated process to free a port. A failed readiness check leaves
started containers available for inspection with `logs` and `doctor`.

`down` uses **stop**, retaining containers, networks and database volumes. It stops only
the managed local service list. `down express` stops just managed Express while keeping
the backend available. `frontend` is an alias for `express` in service commands. Other
service names are `backend`, `keycloak`, `keycloak-db`, `backend-db`, `remote-postgres`,
`redis`, and `mailhog`.

`restart` recreates a service using the selected profile, rather than retaining stale
container environment variables. Backend and Express restarts wait for HTTP readiness.
An Express restart preserves its active standalone module unless the profile sets one.

`status` separates Docker container state from HTTP application readiness, shows published
ports, the active database, and the selected module. `doctor` checks staged assets, resolved
and active database/callback configuration, newer or dirty migrations, port availability,
backend/proxy readiness, and the public Keycloak login form. It exits nonzero on problems.
It does not reset databases, repair dirty migrations, or change identity-provider settings.

## Named profiles

Tracked defaults live in `frontend/portal.profiles.json`. Add personal profiles to the
ignored repository file `.local-sso/profiles.json`, for example:

```json
{
  "training": {
    "port": 3100,
    "publicOrigin": "http://localhost:3100",
    "database": "sso_training_dev",
    "module": "users"
  }
}
```

```bash
npm run portal -- up --profile training
npm run portal -- status --profile training
npm run portal -- restart backend --profile training
```

Profiles inherit the local defaults. Fields include `backend`, `host`, `port`,
`publicOrigin`, `database`, `keycloak`, `realm`, `client`, `module`, `dist`, `importMap`,
`composeFiles`, and `network`. `publicOrigin` is the browser-facing origin used for SSO.
Profile asset paths resolve from `frontend/`; Compose paths resolve from the repository
root. Use `--profiles-file /path/to/profiles.json` for another JSON profile file. Explicit
CLI path flags resolve from the invoking directory.

Foreground `serve`/`dev` honor backend/map environment variables; explicit flags override
them. Managed stack settings come from the profile, with explicit `up` flags overriding
that invocation only. Store persistent port/map/module choices in a named profile so
`status`, `doctor`, and `restart` use the same settings. Profiles select configurations for
the same development Compose stack; they do not create concurrently isolated stacks or
separate backend/Keycloak ports. Local callback registration does not configure an external
identity provider or production deployment.

## Watch one module

```bash
npm run portal -- down express
npm run portal -- dev users --profile local
```

Keep the backend running. `dev` builds the app, starts a Vite build watcher and strict-port
preview server, and starts standalone Express. It copies the selected production/central
map into an ignored `.local-sso/import-map.<profile>.<app>.json`, overriding only the selected
module. Other module/vendor URLs remain intact; the central map is not modified. Remote
map-relative URLs are resolved against their original map URL before copying.

Refresh the browser after a rebuild; this uses build-and-preview rather than HMR. Ctrl+C
stops the watcher, preview, host, and their child processes. A failed development process
stops its siblings. Use `--module-port` to change the preview port. A different Express
`--port` is useful for asset checks, but live SSO requires matching backend/callback settings;
use a matching named profile and backend configuration for that case.

`portal check` now runs command-helper regression tests, host HTTP tests and TypeScript.

## Local setup

Use Node 22 or later. From `frontend/`:

```bash
npm ci
npm run host:install
npm run build:docker
BACKEND_URL=http://127.0.0.1:9000 npm run host:start
```

Open <http://127.0.0.1:3000/portal/>. The backend must be running separately.
The host binds to loopback by default. Set `HOST=0.0.0.0` for container/LAN access.
`PORT` defaults to `3000`.

The shell uses `/portal` for routes and build assets. That base remains fixed.
The default map is `frontend/public/import-map.json`, read on each HTML or map request.
The staged build provides `/mf/*` and `/packages/*`, including chunks and CSS.

## Develop one remote module

Build and preview a module in a separate terminal:

```bash
npm run build:mf:users
npm run preview:mf:users
```

Then run the host with the existing local map (other mapped modules need their own
servers if opened):

```bash
IMPORT_MAP_FILE=public/import-map.local.json npm run host:start
```

This host explicitly allows remote loading on localhost; ordinary Vite development
retains its existing local loading behavior. For mixed local/deployed modules, copy
the production map to an untracked `*.local.json` file, change only the desired entry,
and supply its path through `IMPORT_MAP_FILE`.

## Standalone module

```bash
STANDALONE_MODULE=users BACKEND_URL=http://127.0.0.1:9000 npm run host:start
```

The same shell runs without portal navigation. `/portal/` opens `/portal/admin/users`.
Module deep links retain their canonical paths. The shell checks the session and the
module registry's permission/system requirements before mounting the remote bundle.
`STANDALONE_MODULE` accepts `users` or `@moh-sso/users`; `documents` opens its DWH route.
Only registered module routes are exposed in this mode. Cross-module links may need
the full portal; this mode does not remap modules to `/` or supply missing modules.
Missing import map entries produce HTTP 503, and unknown registered names show an error.

## Shared central map

```bash
IMPORT_MAP_URL=https://assets.example.com/import-map.json \
STANDALONE_MODULE=documents npm run host:start
```

`IMPORT_MAP_URL` takes precedence over `IMPORT_MAP_FILE`. Express fetches it server-side
with a ten-second timeout, resolves relative URLs against that map URL, and embeds the
result before the shell's module script. The JSON endpoint is `/portal/import-map.json`.
Map and HTML responses use `Cache-Control: no-store`. Each new page load reads the
current map; existing tabs retain the modules they already loaded. Publish map changes
atomically and retain previous versioned bundles for rollback. Remote asset servers must
allow cross-origin module requests. A failed map fetch returns 503 rather than stale HTML.

The Express host disables portal PWA registration and serves a retiring service worker
at `/portal/sw.js`, preventing ongoing caching of dynamic host HTML/configuration.
When switching an existing origin from static hosting, clear its old worker/site data
once if it continues serving the previous application.

## Backend and SSO

The public API base is `/backend`; Express strips that prefix and forwards to
`BACKEND_URL`, preserving the rest of the path, query, method, body, and cookies.
For example `/backend/api/v1/auth/me` goes to
`http://127.0.0.1:9000/api/v1/auth/me`. A target deployment prefix is supported, e.g.
`BACKEND_URL=https://example.com/ssobackend`. Cookie domains are removed and cookie
paths become `/` so sessions work on the frontend host. TLS verification remains enabled.

Configure the backend and identity provider for the host's public address. For a local
host at `http://127.0.0.1:3000`, the backend configuration includes:

```dotenv
FRONTEND_BASE_URL=http://127.0.0.1:3000
KEYCLOAK_REDIRECT_URI=http://127.0.0.1:3000/backend/api/v1/auth/callback
```

Register that callback URI in the Keycloak client. Backend cookie settings must suit
the environment (HTTP development versus HTTPS production). Keep login and callback on
the same host so OAuth state/session cookies return through the proxy. The backend
constructs relative `returnTo` destinations against `FRONTEND_BASE_URL`; use a matching
backend configuration per host deployment for standalone domains. Express does not
perform identity validation or replace backend authorization.

## Docker

From `frontend/`:

```bash
docker build -f Dockerfile.express -t moh-frontend-host .
docker run --rm -p 3000:3000 \
  -e BACKEND_URL=http://host.docker.internal:9000 \
  -e STANDALONE_MODULE=users moh-frontend-host
```

The image runs as the `node` user. Supply `IMPORT_MAP_URL` to share a central map across
deployments; otherwise it uses `dist/import-map.json`. `/healthz` reports process health.
`FRONTEND_DIST_DIR` can select another built shell directory (relative paths resolve
from `frontend/`). An external-map deployment can serve a shell-only build as long as
all mapped module assets are hosted elsewhere.

## Verification

```bash
npm run host:test
npm run typecheck
```

Host integration tests cover fresh map injection, ordering, remote URL resolution,
standalone settings, deep links, missing asset responses, map failures, and proxy behavior.
For live SSO, verify login, callback, session retrieval, denied permissions, module
rendering, and refresh on a module deep link against your configured backend/Keycloak.

O3 reference: [development host source](https://github.com/openmrs/openmrs-esm-core/blob/main/packages/tooling/openmrs/src/commands/develop.ts).
