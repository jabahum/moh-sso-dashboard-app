# Building and running the modular distribution

The distribution produces two images: the Go server and the Nginx-hosted portal.
Backend dependencies are pinned in `backend/go.mod` and checked with `go.sum`.
Frontend apps and shared libraries are pinned in `frontend/module-sources.json`;
the production npm lock records transitive dependencies and integrity hashes.
Go and npm fetch packages at image build time. Runtime containers use the compiled
binary and staged bundles. Separately hosted apps use the manifest workflow.

## Requirements

Use Docker with BuildKit and Compose 2.24.4 or newer. Configure the existing
external Keycloak realm, confidential web client, service-account admin client,
roles and callback URL before startup. The backend also requires existing upload
and warehouse databases: configure `REMOTE_DB_*`, `DWH_*` and optional separate
`DQA_DB_*` credentials. The distribution does not populate those integrations.

Distribution mode manages the core and isolated Observatory PostgreSQL databases,
Redis, uploads and the HTTP gateway. Existing `prod`, `remote` and `gateway` modes
retain their topology; use those when Observatory is managed externally. Do not
attach data volumes from another installation without planning its schema upgrade.

## Configure

Copy `infra/env/prod.env.example` to `infra/env/prod.env`. Set public URLs,
Keycloak client IDs and database connections, a unique `DISTRIBUTION_TAG`, and
optionally `DISTRIBUTION_IMAGE_PREFIX`, `DISTRIBUTION_COMMIT` and
`DISTRIBUTION_BUILD_TIME`. Metadata should describe the checkout being built.
Use HTTPS for public URLs; TLS terminates at the public reverse proxy. The
included gateway binds to loopback port 8080 by default and uses the existing
`/portal` and `/ssobackend` routes. Review `infra/nginx/app.conf` and trusted-proxy
settings when connecting your public edge.

Create these private, nonempty files before starting containers:

- `infra/secrets/db_password.txt`
- `infra/secrets/observatory_db_password.txt`
- `infra/secrets/keycloak_admin_client_secret.txt`
- `infra/secrets/keycloak_web_client_secret.txt`

Use actual Keycloak client secrets and unique database passwords. Paths can be
changed with the corresponding `*_SECRET_FILE` settings in the template. Backend
secret files override environment placeholders. Keep secrets out of Git.

## Build and start

```sh
sh infra/scripts/compose.sh distribution config --quiet
sh infra/scripts/compose.sh distribution build backend frontend
sh infra/scripts/compose.sh distribution up -d --no-build --wait
sh infra/scripts/compose.sh distribution ps
```

`ENV_FILE=/absolute/path/settings.env` selects another private environment file.
The launcher passes it to Compose without executing it as shell code. The project
name is `moh-sso-distribution`; container names are not fixed. Use a different
gateway port and project name when running multiple installations on one host.
Project-scoped named volumes retain `backend_data`, `observatory_data`,
`redis_data` and `upload_data` across container replacements.

Core migrations complete before module initialization and serving HTTP. Migration
errors or dirty state prevent startup. Observatory migrations and geography sync
use the existing readiness monitor, which retries until dependencies recover.
Metrics owns no migrations. The backend gets a 60-second stop grace period for
its normal worker and module cleanup.

Published frontend modules use public exports and versioned `single-spa.js`
entries. The staged import map and version manifest record selected versions.
Container startup regenerates only public API/runtime settings. Module pin
changes require rebuilding the image.

## Upgrade and recovery

Back up both databases and uploads before schema upgrades. Select released npm
versions, run `npm run modules:lock` from `frontend`, review the lock, then rebuild
with a new immutable distribution tag. Go updates use `go get` and `go.sum`
review. Keep previous images and environment configuration available.

`down` preserves named volumes; `down -v` destroys them. Reverting images does
not reverse migrations. Restore a verified backup if an older image cannot use
the upgraded schema. Do not automatically force dirty migrations or run down
migrations against production data.

For verified GHCR publication and staging-to-production promotion, see
`infra/server/README.md` and `infra/server/deploy.distribution.json.example`.
Local builds remain useful for development and isolated verification.

## Verification

After building, the following checks use example configuration and disposable
containers. Substitute the image names and tags selected for your build:

```sh
python3 infra/scripts/test-compose.py
python3 infra/scripts/test-migrations.py
python3 infra/scripts/test-distribution-runtime.py \
  --backend-image moh-sso-distribution-backend:0.1.0 \
  --frontend-image moh-sso-distribution-frontend:0.1.0
```

The runtime smoke test starts the actual frontend, both database services and
Redis in a uniquely named project, verifies public configuration and module
versions, recreates the core database container to verify persistence, then
removes its fixture containers and volumes. It checks the backend version command
but does not start the authenticated backend or contact external integrations.
Use the authenticated and transaction checks below for broader coverage.

The authenticated check uses a disposable real Keycloak and fixture users:

```sh
python3 infra/scripts/test-distribution-auth.py \
  --backend-image moh-sso-distribution-backend:0.1.0
python3 infra/scripts/test-cd-runtime.py --mode distribution
```

It checks real tokens and role restrictions, manifest review/activation, audit
idempotence, database and upload backup/restore, shutdown and restart preservation.
The deployment transaction check uses small container fixtures to verify image
identity, failed rollout detection, automatic/operator rollback and persistent
data. These checks create unique projects and remove their fixture resources.
