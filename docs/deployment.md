# Backend Deployment

## Immutable Inputs

Production Compose requires immutable `BACKEND_TAG` and `FRONTEND_TAG` values. Accepted examples:

```env
BACKEND_TAG=1.2.3
BACKEND_TAG=sha-d3062e2
FRONTEND_TAG=1.2.3
FRONTEND_TAG=sha-d3062e2
```

An empty tag or `latest` is not accepted for production. Backend and frontend tags remain independently managed, but both must be explicit.

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

Use `docker-compose.yml` when another reverse proxy already exists on the host and you only want the backend/frontend bound to localhost.

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
Rebuild and publish a new frontend image
to activate the updated static-server configuration, then deploy its immutable
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
docker compose -f docker-compose-nginx.yml exec nginx nginx -t
docker compose -f docker-compose-nginx.yml exec nginx nginx -s reload
```

The integration suite checks routing, SPA fallback, cache/security headers,
prefix-aware redirects, SSO cookies, upload limits, WebSocket upgrades, forwarded
header trust, query-free logs and DNS recovery after backend replacement. It
requires Docker and Python 3, creates a unique temporary network and containers,
and removes them on completion. Production containers are not touched.

For syntax validation without real production values:

```bash
APP_ENV_FILE=./app.env.example BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose -f docker-compose-nginx.yml config --quiet
```

For deployment:

```bash
BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose -f docker-compose-nginx.yml pull

BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose -f docker-compose-nginx.yml up -d
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

The server-side `restart` script must honor exported `BACKEND_TAG` and `FRONTEND_TAG` values and use the production Compose configuration.

## Manual Verification

```bash
curl -fsS http://localhost:9000/version
curl -fsS http://localhost:9000/health/live
docker image inspect ghcr.io/mohuganda/moh-sso-dashboard-backend:1.2.3
```

The binary metadata, OCI labels, requested tag, and expected Git commit must agree.

## Database Migrations

Review migration compatibility before deployment. Back up the database before irreversible migrations. Application rollback cannot reverse an incompatible database migration automatically.
