# Backend Deployment

## Production and development environments

Requires Docker Compose **2.24.4 or newer** for includes, optional development
env files and gateway port resets. Run these as separate stacks, not as a
production base with a development override.

| Entry point | Environment | Purpose |
| --- | --- | --- |
| `docker-compose.yml` | `app.env` | Default production stack; host or remote Nginx |
| `docker-compose.prod.yml` | `app.env` | Explicit production entry point, includes the default stack |
| `docker-compose-nginx.yml` | `app.env` | Same production services with a container gateway; application ports unpublished |
| `docker-compose.dev.yml` | `app.dev.env` | Source builds, Air/Vite reload, local Keycloak/Postgres/Redis/MailHog |
| `docker-compose-without-keycloak.yml` | `app.external-dev.env` | Development with external Keycloak; local IdP services excluded |

Production uses project `moh-sso-dashboard`; development uses
`moh-sso-dashboard-dev` with separate persistent volumes and a managed bridge
network. Existing volumes are never reset automatically. If moving from the old
development setup, stop its containers first and retain its volumes; they remain
under the old project name. Both modes use host ports 3000 and 9000, so change the
bindings before running both on the same host at once.

### Production quick start

```bash
cp app.env.example app.env
# Edit runtime settings, production DB/IdP/warehouse credentials, and release tags.
# Create secrets/db_password.txt, secrets/keycloak_admin_client_secret.txt,
# and secrets/keycloak_web_client_secret.txt using the real credentials.
docker compose --env-file app.env -f docker-compose.prod.yml config --quiet
docker compose --env-file app.env -f docker-compose.prod.yml pull
docker compose --env-file app.env -f docker-compose.prod.yml up -d
```

Use **newly built backend and frontend release images** for these changes.
Backend startup now reads mounted `*_FILE` secrets; frontend startup generates
the public runtime config. Older images do not implement those entrypoints.
Do not edit or overwrite existing private env files when adopting the examples.

Use `--env-file app.env` on every production command so Compose interpolation
and the backend's `env_file` agree. `APP_ENV_FILE` selects the backend runtime
file; if using another name, set it to the same file passed on the CLI.
The CLI env file configures Postgres user/database, Redis authentication, image
tags, private binding addresses and the public API URL. `APP_BIND_ADDRESS` remains
`127.0.0.1` for same-host Nginx; use the remote-proxy section for another machine.

The production backend always uses production/release mode with auth bypass
disabled and port 9000. Nginx terminates public TLS; backend HTTP is internal.
`DB_HOST`, `DB_PORT`, `REDIS_HOST` and `REDIS_PORT` default to the managed services
but can be overridden via the CLI env file. The local Postgres and Redis services
are still started; replacing them entirely requires a deployment override.

Primary DB and both Keycloak client secrets come from Compose secrets and take
precedence over the matching plaintext env values. `DB_PASSWORD` in `app.env`
may remain blank. Secret source paths can be set with `DB_PASSWORD_SECRET_FILE`,
`KEYCLOAK_ADMIN_SECRET_FILE` and `KEYCLOAK_WEB_SECRET_FILE`. Additional backend
secrets support corresponding `*_FILE` paths when mounted via a deployment
override; unreadable or empty mounted secrets fail startup. The production
entrypoint reads restricted secret files as root, then replaces itself with the
server running as `appuser`; the API does not run as root. Keep source files
restricted to the deployment account (for example mode 0600) and mount them
read-only. Development leaves the privilege-drop setting unset so Air can build
the bind-mounted source.

The backend receives the full runtime env file, including SMTP/GM, SMS, storage,
notification workers, RBAC, Health BI, DWH and DQA settings. Frontend receives only
public settings and does not receive database or client credentials. Its generated
`config.js` is served at both `/config.js` and `/portal/config.js` with `no-store`.
Set `API_BASE_URL` to the browser-facing API base, normally `/ssobackend`.
Redis authentication uses the same `REDIS_PASSWORD` in server, client and health
check; leave it empty only for an isolated internal Redis network.

The application currently requires the upload-management DB (`REMOTE_DB_*`) and
warehouse (`DWH_*`) during startup, as well as its primary DB and Keycloak.
Configure reachable real services; production does not create a mock warehouse.
`DQA_DB_USERNAME`/`DQA_DB_PASSWORD` select the role that writes DQA results, or
fall back to warehouse credentials when the username is empty. `DB_ENABLE_SSL`
is currently shared by primary/upload/warehouse connections, so all selected
databases must support the chosen mode; the bundled Postgres defaults to no TLS.

Local uploads persist in `upload_data` mounted at `/data/uploads`, owned by the
backend image's application user on new volumes. Preserve existing data and
permissions on upgrades. For NFS, mount the host's actual NFS directory at
`NFS_BASE_PATH` using a deployment override; selecting `STORAGE_PROVIDER=nfs`
alone does not mount a network filesystem. S3/MinIO credentials and endpoints
remain runtime settings; no object-storage service is assumed.

### Development quick start

```bash
cp backend/app.env.example app.dev.env
docker compose --env-file app.dev.env -f docker-compose.dev.yml config --quiet
docker compose --env-file app.dev.env -f docker-compose.dev.yml up --build
```

With no private file, development uses `backend/app.env.example` as a complete
fallback. The optional `app.dev.env` is applied afterwards, so custom SMTP,
notification, storage and scheduler settings win. For a differently named file,
set `DEV_ENV_FILE` to that path and pass the same file with `--env-file`.
Explicit topology values keep the local stack consistent: local DB/Redis/IdP,
HTTP port 9000, browser frontend on `http://localhost:3000/portal/`, host-only
cookies and development mode. Auth bypass defaults to false; it is never enabled
by the production Compose file.

The imported local realm fixes the browser callback to
`http://localhost:9000/api/v1/auth/callback`, client IDs to `dashboard-admin` and
`dashboard-web`, and their fixture secrets to the values in the development
example. Changing those secrets in the env file alone does not change imported
Keycloak clients. Existing Keycloak databases retain their current client settings.

All development published ports bind to localhost:

| Service | Host address |
| --- | --- |
| Portal | `http://localhost:3000/portal/` |
| API / Swagger | `http://localhost:9000/swagger/` |
| Keycloak | `http://localhost:8081` |
| MailHog UI / SMTP | `http://localhost:8025` / `localhost:1025` |
| Primary Postgres | `localhost:5436` |
| Upload/warehouse Postgres | `localhost:5433` |
| Module development servers | `localhost:4101` through `localhost:4115` |

Backend and frontend source are bind-mounted. Air and Vite reload source changes;
uploads, Go caches and frontend dependencies use named volumes. Rebuild after
Dockerfile/lockfile changes; refresh the frontend dependency volume when its
installed dependencies no longer match the lockfile, preserving database/upload
volumes. The default frontend command launches the local shell. Set
`FRONTEND_DEV_COMMAND` to `npm run dev:remote:all` to launch module servers.
Development uses local Postgres as an empty upload/warehouse fixture, not a copy
of production datasets. Health BI is optional; configure its local adapter and
service account before using report execution.

For external Keycloak, copy the development example to `app.external-dev.env`,
set its reachable internal/base/browser URLs, realm, client IDs and secrets, then:

```bash
DEV_ENV_FILE=./app.external-dev.env docker compose --env-file app.external-dev.env \
  -f docker-compose-without-keycloak.yml up --build
```

Register the same localhost web callback and origin on that external web client.
The local Keycloak and its database are excluded; API data dependencies remain local.

### Validate environment and container startup changes

```bash
python3 scripts/test-compose.py
python3 scripts/test-compose-runtime.py
python3 scripts/test-nginx.py
```

The first checks environment coverage, Compose service alignment, optional dev
overrides, secret loading and safe frontend config serialization. The runtime
suite builds the actual production runtime stages with tiny fixture build outputs
and starts a disposable Keycloak to check its development readiness probe. It
does not compile the complete frontend/backend or connect to production services.
The Nginx suite checks same-host, remote-IP and container-gateway routing. Tests
use temporary resources and preserve running application stacks.

## Immutable Inputs

Production Compose requires immutable `BACKEND_TAG` and `FRONTEND_TAG` values. Accepted examples:

```env
BACKEND_TAG=1.2.3
BACKEND_TAG=sha-d3062e2
FRONTEND_TAG=1.2.3
FRONTEND_TAG=sha-d3062e2
```

Compose rejects empty image tags. Use explicit semantic-version or SHA tags for production; `latest` is outside the release policy. Backend and frontend tags remain independently managed.

## Compose Production Files

Use `docker-compose-nginx.yml` for the container gateway. It binds to
`127.0.0.1:8080` by default, behind an HTTPS reverse proxy. Set
`NGINX_BIND_ADDRESS` and `NGINX_HTTP_PORT` explicitly if another listener is
required. This replaces the previous public HTTP port 80 default.

The gateway sends `/api/*`, `/swagger`, `/swagger/*`, `/health`, `/health/*` and
`/version` to the backend. It also accepts `/ssobackend/*` and `/backend/*`,
stripping that prefix before forwarding. Other paths go to the frontend.
For example, `/ssobackend/swagger/` serves the API documentation and
`/ssobackend/swagger/openapi.json` serves its specification.

Use `docker-compose.yml` when Nginx runs outside Compose. Application ports bind
to localhost by default. For Nginx on a separate machine, set `APP_BIND_ADDRESS`
to the application server's private IPv4 using the remote deployment settings
below.

### Nginx on a separate machine

```text
Browser --HTTPS--> Nginx machine (example private IP 10.0.0.10)
                     /portal/*     --> App machine 10.0.0.20:3000
                     /ssobackend/* --> App machine 10.0.0.20:9000
```

The IPs below are examples; replace them with the actual private/VPN addresses.
The application server must own `APP_BIND_ADDRESS` on one of its interfaces, and
that address must be reachable from Nginx. Do not use `0.0.0.0`, a public interface
or the Nginx machine's address as the application binding.

**On the application machine:**

```bash
cp .env.remote-proxy.example .env.remote-proxy
# Edit APP_BIND_ADDRESS here; keep runtime settings and release tags in app.env.
docker compose --env-file app.env --env-file .env.remote-proxy -f docker-compose.yml config --quiet
docker compose --env-file app.env --env-file .env.remote-proxy -f docker-compose.yml up -d
```

Use the same `--env-file app.env --env-file .env.remote-proxy` on subsequent pull, update and rollback
commands, including deployment automation. Without it (or the equivalent exported
variable), Compose falls back to `127.0.0.1`. The file is git-ignored. Keep runtime
credentials in `app.env` and the existing secret files; `.env.remote-proxy` holds
Compose interpolation settings. Setting `APP_BIND_ADDRESS` only in the service's
`app.env` does not change Compose port mappings. Shell variables override values
in the CLI env file; clear conflicting exported deployment variables first.

**On the Nginx machine:**

```bash
sudo install -d -m 0755 /etc/nginx/moh-sso
sudo install -m 0644 nginx/host/*.conf /etc/nginx/moh-sso/
# Replace the loopback upstream file with the remote-machine variant.
sudo install -m 0644 nginx/remote/http.conf /etc/nginx/moh-sso/http.conf
# Edit BOTH 10.0.0.20 upstream addresses to match APP_BIND_ADDRESS.
sudoedit /etc/nginx/moh-sso/http.conf
```

Use the same Nginx version and includes described in the host setup below:
include `/etc/nginx/moh-sso/http.conf` once inside `http {}`, and
`/etc/nginx/moh-sso/portal.locations.conf` inside the existing public HTTPS
`server {}`. Choose either the host or remote upstream file, never both; they
define the same upstream names and share `http-common.conf` and route snippets.
For remote deployments, subsequent config updates must reinstall the remote
upstream variant and retain the actual application IP.

Keep public DNS pointed at the Nginx machine. TLS certificates and HTTPS
termination remain there. Browser-facing API URLs and Keycloak callback URLs
retain `https://dashboards.health.go.ug/ssobackend`; the private IP is used only
between Nginx and the application server. The existing prefix stripping, Swagger,
SSO, WebSocket, caching and security-header rules apply to both deployment modes.

**Network access:** allow TCP 3000 and 9000 on the application server only from
the Nginx server's actual private/VPN source IP (10.0.0.10 in this example), and
deny other remote sources. Database and Redis ports remain unpublished. Enforce
this with network firewall/security-group ACLs or rules that cover Docker's
forwarded/published traffic. Ordinary host UFW rules alone may not filter Docker
published ports; see [Docker firewall behavior](https://docs.docker.com/engine/network/packet-filtering-firewalls/).
Use private/VPN connectivity for these HTTP upstreams; do not send this traffic
across an untrusted network without transport encryption.

Verify direct application connectivity from the Nginx machine before reload:

```bash
curl -fsS http://10.0.0.20:3000/nginx/health
curl -fsS http://10.0.0.20:9000/health/ready
sudo nginx -t && sudo systemctl reload nginx
curl -fsS https://dashboards.health.go.ug/portal/import-map.json
curl -fsS https://dashboards.health.go.ug/ssobackend/health/ready
```

After firewall setup, also verify that a machine outside the allowed source
cannot reach application ports. The integration suite exercises a separate
Nginx instance reaching application services through another container's IP;
the actual machines' routing and firewall rules must be verified on deployment.

### Nginx installed on the host (no Compose gateway)

Use the ordinary `docker-compose.yml` in this topology:

```text
Browser --HTTPS--> Host Nginx
                     /portal/*     --> 127.0.0.1:3000 (frontend container)
                     /ssobackend/* --> 127.0.0.1:9000 (backend container)
```

The frontend and backend remain containers. The reverse proxy runs as the host's
Nginx service; no Compose Nginx service or port 8080 is needed. Compose already
publishes both application ports on loopback only. Browser-facing SSO URLs remain
the HTTPS URLs shown below.

The ready-to-install snippets live in `nginx/host/`. Use the tested Nginx 1.30.6
production baseline on the host. The snippets use `add_header_inherit merge`
(available since 1.29.3) so location cache headers preserve existing server-level
headers such as HSTS. See the [official header inheritance documentation](https://nginx.org/en/docs/http/ngx_http_headers_module.html#add_header_inherit).

On the deployment server, install the snippets from the checked-out release:

```bash
nginx -v
sudo install -d -m 0755 /etc/nginx/moh-sso
sudo install -m 0644 nginx/host/*.conf /etc/nginx/moh-sso/
```

Add this include **once inside the existing `http {}` block** in the host's
`nginx.conf`, before its site includes:

```nginx
include /etc/nginx/moh-sso/http.conf;
```

Add this include **inside the existing HTTPS `server {}` block** for
`dashboards.health.go.ug`:

```nginx
include /etc/nginx/moh-sso/portal.locations.conf;
```

Replace any existing `/portal`, `/portal/`, `/ssobackend` and `/ssobackend/`
locations with that include to avoid duplicate locations. Keep the server's
certificate settings, HTTPS redirect, security headers, `/` and `/report-browser/`
routes. The snippets only add the portal's paths and do not replace the whole
site or global Nginx configuration. Existing server-level security headers are
inherited; keep the site's security policy configured on that server.

`http.conf` defines loopback upstreams, the WebSocket header map, cache policy and
query-free log format. `proxy.conf` provides proxy timeouts, SSO cookie buffers,
request IDs and sanitized forwarded headers. `portal.locations.conf` preserves
`/portal/` for the frontend and strips `/ssobackend/` before forwarding to Go.
Swagger is available at `https://dashboards.health.go.ug/ssobackend/swagger/`.

Here Nginx terminates TLS itself and forwards `$scheme` and `$remote_addr`.
`nginx/trusted-proxies.conf` belongs to the Compose gateway and is not used.
If the host Nginx itself sits behind a load balancer, configure its real-IP and
scheme trust for that specific load balancer before using these direct-edge
snippets; do not accept arbitrary browser-supplied forwarded headers.

Start/update application containers using the selected immutable release tags:

```bash
docker compose --env-file app.env -f docker-compose.prod.yml up -d
sudo nginx -t && sudo systemctl reload nginx

curl -fsS http://127.0.0.1:3000/nginx/health
curl -fsS http://127.0.0.1:9000/health/ready
curl -fsS https://dashboards.health.go.ug/portal/import-map.json
curl -fsS https://dashboards.health.go.ug/ssobackend/health/ready
```

Replace the example tags with the releases being deployed. Rebuild/publish the
frontend image to activate its improved static caching rules. Host Nginx writes
portal access logs to `/var/log/nginx/moh-sso-access.log`; include that path in the
host's log rotation policy. To test these snippets before installing them, run
`python3 scripts/test-nginx.py`. The test uses disposable containers to emulate
the host's loopback services and actual HTTPS termination; it does not install
or reload the host service.

Both compose files expect:

- `./app.env` for runtime environment
- `./secrets/db_password.txt`
- `./secrets/keycloak_admin_client_secret.txt`
- `./secrets/keycloak_web_client_secret.txt`

Cross-application SSO also requires `AUTH_RETURN_URL_ALLOWED_ORIGINS` to contain exact browser origins, without paths. For the current production host:

```env
FRONTEND_BASE_URL=https://dashboards.health.go.ug/portal
AUTH_RETURN_URL_ALLOWED_ORIGINS=https://dashboards.health.go.ug
LOGIN_URL=https://dashboards.health.go.ug/ssobackend/api/v1/auth/login
KEYCLOAK_REDIRECT_URI=https://dashboards.health.go.ug/ssobackend/api/v1/auth/callback
API_BASE_URL=/ssobackend
```

Do not use Docker or Kubernetes service names in browser-facing values.

### Shared production host routing

The production reverse proxy must keep these path owners distinct:

```text
/                         -> Report Browser
/report-browser/*         -> Report Browser
/portal/*                 -> Integrated Health Portal frontend
/ssobackend/*             -> Integrated Health Portal backend
```

The bare origin redirecting to Report Browser is intentional. Do not configure the
portal frontend base as `https://dashboards.health.go.ug/`, and do not strip
`/ssobackend` from browser-facing login or callback URLs. The repository's simple
`nginx/app.conf` is suitable when this frontend owns the whole host; the shared
production host needs an upstream reverse-proxy configuration that implements the
path table above.

### HTTPS termination and proxy trust

Keep TLS certificates, HTTP-to-HTTPS redirects and HSTS on the existing public
HTTPS proxy. The checked-in container gateway speaks HTTP on the internal hop.
In the public proxy's HTTPS server, forward portal paths without stripping them:

```nginx
# Keep the existing / and /report-browser/ locations unchanged.
location = /portal {
    return 308 /portal/$is_args$args;
}
location /portal/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
location = /ssobackend {
    return 308 /ssobackend/$is_args$args;
}
location /ssobackend/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 50m;
    proxy_read_timeout 120s;
    proxy_buffer_size 32k;
    proxy_buffers 8 32k;
    proxy_busy_buffers_size 64k;
}
```

Configure `nginx/trusted-proxies.conf` with the actual connecting address of the
HTTPS proxy **as seen inside the gateway** (Docker may translate its source).
Add that exact IP/CIDR to both `set_real_ip_from` and the `geo` block. Until it is
configured, the gateway ignores forwarded client addresses and schemes. This
prevents clients from claiming HTTPS or supplying arbitrary client IPs. Never
trust all addresses or an entire shared Docker network. The gateway overwrites
the forwarded chain after resolving the trusted client address.

The example covers the current HTTP API; if a public route requires WebSockets,
the outer proxy must also pass HTTP/1.1 `Upgrade` and `Connection` headers. The
container gateway already supports upgrades. Streaming handlers can set
`X-Accel-Buffering: no` to disable response buffering for their response.

### Frontend runtime and caching

All three frontend Dockerfiles use Nginx 1.30.6. The default published image and
`Dockerfile.npm-modules` keep port 3000; `Dockerfile.nginx` keeps port 80. Helm's
default frontend container port is now 3000, with the Service still on port 80;
set `frontend.service.port=80` if using the alternate `Dockerfile.nginx` image.
The runtime ConfigMap is mounted at both `/config.js` and `/portal/config.js`.
Rebuild and publish backend and frontend images
to activate secret loading and the updated static-server configuration, then deploy its immutable
`FRONTEND_TAG`. Mounting gateway configs does not update an existing frontend
image.

Runtime config, import maps, version manifests and HTML use `no-store`. Stable
microfrontend/package entries and service workers revalidate with `no-cache`.
Only content-hashed files under `assets/` receive a one-year immutable cache
policy. Missing assets return 404 with `no-store`, never the SPA HTML fallback.
Security headers apply to successful responses and errors. API responses through
the gateway use `no-store`; no shared proxy cache is enabled.

The gateway retains the 50 MiB request-body limit, supports large legacy SSO
cookie responses, and refreshes Docker DNS when upstream containers are replaced.
Access logs omit query strings, cookies and authorization headers. Apply the same
log policy on any outer proxy, especially for SSO callback URLs.

### Validate Nginx before deployment

```bash
# Disposable fixtures only; does not start the application stack or database.
python3 scripts/test-nginx.py

# Validate the deployed config, then reload gateway workers after config edits.
docker compose --env-file app.env -f docker-compose-nginx.yml exec nginx nginx -t
docker compose --env-file app.env -f docker-compose-nginx.yml exec nginx nginx -s reload
```

The integration suite checks routing, SPA fallback, cache/security headers,
prefix-aware redirects, SSO cookies, upload limits, WebSocket upgrades, forwarded
header trust, query-free logs and DNS recovery after backend replacement. It
requires Docker, Python 3 and the OpenSSL CLI, creates a unique temporary network and containers,
and removes them on completion. Production containers are not touched.

For syntax validation without real production values:

```bash
APP_ENV_FILE=./app.env.example BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose -f docker-compose-nginx.yml config --quiet
```

For deployment:

```bash
BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose --env-file app.env -f docker-compose-nginx.yml pull

BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose --env-file app.env -f docker-compose-nginx.yml up -d
```

## Automated Deployment

The deployment workflow supports:

- merged backend release PRs and backend release workflow runs using the published backend release tag
- manual deployment of an explicitly supplied immutable backend image tag or `backend/v<version>` tag, which is normalized to the image tag

The remote deployment:

1. Records the previously deployed backend/frontend tags.
2. Restarts using the requested immutable tags.
3. Calls `/version` and verifies expected version and commit.
4. Calls `/health/live`.
5. Records the image digest, actor, timestamp, version, and commit.
6. Restores the previous tags when verification fails.

The deploy workflow now reads the release metadata artifact emitted by the backend release workflow, so production follows the exact published backend release instead of a generic build output. The artifact carries both the Git release tag and the Docker image tag, and deployment uses the immutable image tag while recording the matching release tag in the deployment history.

The server-side `restart` script must honor exported `BACKEND_TAG` and
`FRONTEND_TAG` values and use the production Compose configuration with
`--env-file app.env` (and `--env-file .env.remote-proxy` for the remote topology).
Frontend verification runs in a disposable Node container sharing the frontend's
network namespace because the production frontend image now contains Nginx.

## Manual Verification

```bash
curl -fsS http://localhost:9000/version
curl -fsS http://localhost:9000/health/live
docker image inspect ghcr.io/mohuganda/moh-sso-dashboard-backend:1.2.3
```

The binary metadata, OCI labels, requested tag, and expected Git commit must agree.

## Database Migrations

Review migration compatibility before deployment. Back up the database before irreversible migrations. Application rollback cannot reverse an incompatible database migration automatically.
