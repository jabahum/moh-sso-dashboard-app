# Deployment

See the [Make command reference](make.md) for every deployment and generation target,
its prerequisites, options, effects and examples.

## Infrastructure layout

All deployment files are under `infra/`:

```text
infra/
  Makefile                 deployment targets, also included by the root Makefile
  compose/                 production, development and gateway Compose files
  docker/backend/          backend Dockerfiles and secret-loading entrypoint
  docker/frontend/         frontend Dockerfiles, runtime config writer and static Nginx
  nginx/                   gateway, host and remote reverse-proxy snippets
  helm/moh-sso/             Kubernetes chart and environment values
  keycloak/                realm fixtures and themes
  env/                     public environment examples; private *.env files are ignored
  secrets/                 private Compose secrets (ignored)
  certs/                   existing local certificate fixtures
  scripts/                 Compose launcher, updater, SSO tooling and deployment checks
```

Run commands below from the repository root. `make -C infra` also exposes the
same deployment targets. Requires Docker Compose 2.24.4+, Python 3 and jq for
contract checks; runtime tests use Docker. Frontend build tools require Node 22.

## Production and development environments

| Mode | Compose entry point under `infra/compose/` | Private environment file |
| --- | --- | --- |
| `prod` | `docker-compose.prod.yml` (includes `docker-compose.yml`) | `infra/env/prod.env` |
| `dev` | `docker-compose.dev.yml` | `infra/env/dev.env`, with checked-in fallback |
| `gateway` | `docker-compose-nginx.yml` | `infra/env/prod.env` |
| `remote` | `docker-compose.prod.yml` | production file plus `infra/env/remote-proxy.env` |
| `external` | `docker-compose-without-keycloak.yml` | `infra/env/external-dev.env` |

Use `make compose-config COMPOSE_ENV=dev`, for example, to validate one mode.
`compose-up`, `compose-down`, `compose-logs`, `compose-ps`, `compose-pull` and
`compose-restart` accept the same `COMPOSE_ENV`. `SERVICE=backend` selects a service
where supported. `ENV_FILE` and `REMOTE_ENV_FILE` select alternative files; paths
are relative to the repository root or absolute.

`infra/scripts/compose.sh` resolves the repository root, passes env files to
Compose without sourcing them as shell code, and supplies absolute runtime-file
paths. Prefer Make/the launcher instead of duplicating raw CLI flags in scripts.
For raw Compose commands, paths in YAML are relative to `infra/compose/`; Docker
build contexts are the repository root. Source files, credentials and generated
build output are filtered by the root `.dockerignore`.

Production keeps project `moh-sso-dashboard`; development keeps
`moh-sso-dashboard-dev`. Moving the files does not rename projects or data volumes.
Existing private root env files are not moved automatically. Copy them to the
new ignored `infra/env/` locations, or use `ENV_FILE=/absolute/old/path.env`.
Do not run development as an override of the production stack. Both use host
ports 3000 and 9000, so customize bindings before running both on one machine.

### Production quick start

```bash
cp infra/env/prod.env.example infra/env/prod.env
# Edit release tags, public URLs, signing key, external databases and credentials.
# Create the real credentials in:
# infra/secrets/db_password.txt
# infra/secrets/keycloak_admin_client_secret.txt
# infra/secrets/keycloak_web_client_secret.txt
chmod 600 infra/env/prod.env infra/secrets/*.txt
make compose-config
make compose-pull
make compose-prod-up
```

Use semantic-version or SHA tags for both `BACKEND_TAG` and `FRONTEND_TAG`.
Compose rejects empty tags; release automation also rejects mutable tags.
Rebuild and publish backend/frontend images after Dockerfile or entrypoint changes.
Image builds now run from the repository root, for example:

```bash
docker build -f infra/docker/backend/Dockerfile --build-arg VERSION=1.2.3 -t backend:1.2.3 .
docker build -f infra/docker/frontend/Dockerfile --build-arg VERSION=1.2.3 -t frontend:1.2.3 .
```

The backend receives the full runtime environment, covering SMTP/GM, SMS, RBAC,
storage, notification workers, Health BI, DWH and DQA. Infrastructure reads the
same CLI env file for Postgres user/database and Redis authentication. The backend
always uses production/release mode, HTTP port 9000 and disabled auth bypass.
Public TLS terminates at Nginx. Frontend receives public runtime settings only;
its entrypoint writes JSON-escaped `config.js` at root and `/portal/config.js`.
`API_BASE_URL` is normally `/ssobackend`.

The backend entrypoint reads DB and Keycloak client secrets from mounted files,
then execs the server as `appuser`. Matching plaintext values can be blank in the
env file. Secret source paths can be overridden with `DB_PASSWORD_SECRET_FILE`,
`KEYCLOAK_ADMIN_SECRET_FILE`, and `KEYCLOAK_WEB_SECRET_FILE`; relative source paths
are interpreted from `infra/compose/`. Additional secrets support `*_FILE` paths
when mounted through an override. Empty/unreadable mounted secrets fail startup.
Redis server, client and readiness probe use the same `REDIS_PASSWORD`.

The backend needs reachable upload-management (`REMOTE_DB_*`) and warehouse
(`DWH_*`) databases during startup; production does not create mock datasets.
DQA credentials select the warehouse role that writes results, falling back to
DWH credentials when its username is blank. `DB_ENABLE_SSL` currently applies to
all database connections; the bundled primary Postgres defaults to no TLS.
`DB_HOST`, `DB_PORT`, `REDIS_HOST`, and `REDIS_PORT` can target other services,
but bundled DB/Redis containers still start unless excluded by an override.

Uploads persist in `upload_data` at `/data/uploads`. New volumes inherit the
backend image's upload directory ownership. Preserve existing permissions/data.
NFS requires an explicit bind mount of the host's mounted filesystem at
`NFS_BASE_PATH`; setting a provider name alone does not mount NFS. S3/MinIO
endpoints and credentials remain runtime settings.

### Development quick start

```bash
cp infra/env/dev.env.example infra/env/dev.env
make compose-config COMPOSE_ENV=dev
make compose-dev-up
make compose-logs COMPOSE_ENV=dev SERVICE=backend
```

Development works without a private env file using the public fallback. A private
file is loaded after that fallback. Explicit topology values keep service names,
HTTP ports, local SMTP, cookies and Keycloak callbacks consistent. Backend source
uses Air; frontend source uses Vite. Uploads, frontend dependencies and Go caches
persist in named volumes. Rebuild on Dockerfile/lockfile changes, refreshing the
frontend dependency volume if needed without deleting database/upload volumes.

| Service | Local address |
| --- | --- |
| Portal | `http://localhost:3000/portal/` |
| API / Swagger | `http://localhost:9000/swagger/` |
| Keycloak | `http://localhost:8081` |
| MailHog UI / SMTP | `http://localhost:8025` / `localhost:1025` |
| Primary Postgres | `localhost:5436` |
| Upload/warehouse fixture | `localhost:5433` |
| Module servers | ports 4101 through 4115 |

Published development ports bind only to localhost. Keycloak imports
`infra/keycloak/realm-export.json`; its existing database retains existing client
credentials. Updating env secrets alone does not change an imported client.
Default realm/client fixture values are in the dev example. Auth bypass defaults
to false. Use `FRONTEND_DEV_COMMAND='npm run dev:remote:all'` in the env file for
module servers; the default runs the shell with local modules.

For external Keycloak, copy `infra/env/external-dev.env.example` to
`infra/env/external-dev.env`. It uses the production Keycloak URL
`https://auth.health.go.ug` while application URLs stay on localhost. Set the
realm/client IDs to match that server and fill both client secrets privately, then run
`make compose-external-up`. Register localhost callback
`http://localhost:9000/api/v1/auth/callback` and frontend origin
`http://localhost:3000` on that client. The local IdP and its DB are excluded.

`make sso-up`, `sso-down`, `sso-status` and `sso-reconcile-keycloak` use the relocated
local SSO tool for the separate Health BI checkout. Runtime state stays in
`.local-sso/` at the repository root. Health BI is optional for ordinary portal
development; configure its adapter/service account before executing reports.

### Nginx installed on the host (no Compose gateway)

Use `prod` mode, binding applications to `127.0.0.1:3000` and `127.0.0.1:9000`.
Install the snippets on the Nginx server:

```bash
sudo install -d -m 0755 /etc/nginx/moh-sso
sudo install -m 0644 infra/nginx/host/*.conf /etc/nginx/moh-sso/
```

Include `/etc/nginx/moh-sso/http.conf` **once inside `http {}`**, before site
includes. Include `/etc/nginx/moh-sso/portal.locations.conf` **inside the existing
HTTPS `server {}`**. Replace existing portal/backend locations to avoid duplicates.
Keep certificates, HTTPS redirects, HSTS and other security headers on that server.
The snippets require Nginx 1.29.3+ for [header inheritance](https://nginx.org/en/docs/http/ngx_http_headers_module.html#add_header_inherit), tested with 1.30.6.

Shared public host routing stays:

```text
/ and /report-browser/* -> existing Report Browser owner
/portal/*              -> portal frontend, preserving the prefix
/ssobackend/*           -> Go backend, stripping that prefix internally
```

Swagger is `/ssobackend/swagger/`, with the spec at
`/ssobackend/swagger/openapi.json`. Stable module URLs revalidate; runtime config,
import maps and HTML use no-store. Content-hashed static assets use immutable
caching, and missing code assets return 404. API traffic is not cached. Proxy
rules handle 50 MiB request bodies, large SSO cookie responses and WebSockets.
Access logs omit query strings and credentials; rotate
`/var/log/nginx/moh-sso-access.log` on the host.

### Nginx on a separate machine

```text
Browser --HTTPS--> Nginx machine 10.0.0.10
                     /portal/*     --> App machine 10.0.0.20:3000
                     /ssobackend/* --> App machine 10.0.0.20:9000
```

Both IPs are examples. On the application machine:

```bash
cp infra/env/remote-proxy.env.example infra/env/remote-proxy.env
# Edit APP_BIND_ADDRESS to the application's real private/VPN IPv4.
make compose-config COMPOSE_ENV=remote
make compose-remote-up
```

The remote overlay contains network settings only; release tags and credentials
stay in the production env file. Use `COMPOSE_ENV=remote` for subsequent pull,
restart and rollback commands. The application host must own the binding IP.

On the Nginx machine install the host snippets, then replace only its upstream file:

```bash
sudo install -m 0644 infra/nginx/remote/http.conf /etc/nginx/moh-sso/http.conf
sudoedit /etc/nginx/moh-sso/http.conf
# Replace BOTH example application IPs with APP_BIND_ADDRESS.
curl -fsS http://10.0.0.20:3000/nginx/health
curl -fsS http://10.0.0.20:9000/health/ready
sudo nginx -t && sudo systemctl reload nginx
```

Use either the loopback or remote upstream file, never both. Public DNS points to
Nginx; public API and SSO URLs keep the HTTPS domain, not the private IP. Firewall
application ports 3000/9000 to the Nginx machine's actual private source address.
Use ACLs/security groups or Docker-aware filtering; plain UFW rules can be
bypassed by [published Docker traffic](https://docs.docker.com/engine/network/packet-filtering-firewalls/).
Verify another source cannot reach those ports. Use a private/VPN network for
HTTP upstreams; encrypt transport across untrusted networks. Tests emulate the
separate addresses; verify actual routing/firewalls on deployment.

### Container gateway

`make compose-gateway-up` uses the same production service definitions while
removing application host port mappings. Only Nginx is published, defaulting to
`127.0.0.1:8080`; `NGINX_BIND_ADDRESS` and `NGINX_HTTP_PORT` override that listener.
It uses Docker DNS internally and recovers when upstream container IPs change.
The outer HTTPS proxy forwards portal/backend paths unchanged to port 8080.

Configure `infra/nginx/trusted-proxies.conf` with the exact connecting proxy IP in
both `set_real_ip_from` and the `geo` block. Trust nothing by default; never trust
all addresses or the entire shared Docker network. Nginx on the public edge uses
its own `$scheme` and `$remote_addr`; gateway trust config is only for the extra
container hop. If the public edge is itself behind a load balancer, configure its
trust separately. Preserve WebSocket headers on all proxy hops.

For reloads, use the launcher rather than a different Compose project:

```bash
sh infra/scripts/compose.sh gateway exec nginx nginx -t
sh infra/scripts/compose.sh gateway exec nginx nginx -s reload
```

## Kubernetes

The chart lives at `infra/helm/moh-sso`. Existing root targets delegate to the
shared infrastructure Makefile: `make helm-lint`, `helm-template-local`,
`helm-template-dev`, and `helm-template-prod BACKEND_TAG=... FRONTEND_TAG=...`.
`local-up`, `dev-up`, and `prod-up` retain their Kubernetes meaning; use the
`compose-*-up` names for Docker Compose.

## Contracts and checks

```bash
make contracts-generate        # regenerate backend/docs/openapi.yaml
make contracts-check           # check route/schema coverage and freshness
make openapi-export OPENAPI_OUTPUT=/tmp/moh-openapi.yaml
make frontend-manifests        # regenerate import map and version manifest
make generate                  # API contracts plus frontend manifests
make sqlc-generate             # Go SQL bindings; requires installed sqlc
make infra-check               # Compose/env/path/launcher contracts
make infra-runtime-check       # real runtime stages with tiny build fixtures
make infra-nginx-check         # gateway, host and remote-IP HTTPS routing
make infra-test                # all infrastructure checks
```

OpenAPI inputs/output remain in `backend/`, including `go:embed` resources. Root
Make delegates with the correct backend working directory. Native backend Make
commands still work (`make -C backend contracts-check`). CI runs contract
freshness, moved infrastructure tests, Helm checks and application tests. Runtime
fixtures validate secret loading, non-root uploads, public config and the actual
Keycloak readiness probe without compiling or starting the full application.

## Updates, rollback and migrations

For GitHub-driven delivery, follow the [CD and server setup runbook](cd-pipeline.md).
The build publishes a matched image pair and infrastructure bundle. A separate
workflow deploys staging or promotes that same build to production. Server-local
`/etc/moh-sso/deploy.json` selects `prod`, `remote` or `gateway`; private settings
remain on the server. Rollout verifies exact image digests, runtime versions,
modules and the public HTTPS edge, and restores the previous recorded release on
failure. No repository checkout is needed on the server.

For a manual tag-based update from a checked-out release, run
`COMPOSE_ENV=prod sh infra/scripts/update.sh` (or `remote`/`gateway`). It validates,
pulls selected tags and starts services without resetting volumes or Git state.
It reuses Docker registry login, or reads `GHCR_TOKEN_FILE` when supplied. It does
not prune images. Keep the chosen Compose mode/env paths consistent between
manual commands. This updater does not create the CD system's verified release
history; use the CD workflows for managed promotion and rollback.

Back up data and review migration compatibility before rollout. An application
rollback does not reverse incompatible database migrations. Keep public SSO values
aligned, for example:

```env
FRONTEND_BASE_URL=https://dashboards.health.go.ug/portal
AUTH_RETURN_URL_ALLOWED_ORIGINS=https://dashboards.health.go.ug
LOGIN_URL=https://dashboards.health.go.ug/ssobackend/api/v1/auth/login
KEYCLOAK_REDIRECT_URI=https://dashboards.health.go.ug/ssobackend/api/v1/auth/callback
API_BASE_URL=/ssobackend
```
