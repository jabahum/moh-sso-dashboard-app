# Scheduler environment configuration

The portal schedules work; Health BI generates reports and signs artifact URLs.
Each application needs its own configuration. Portal worker credentials belong
only on the portal backend. The artifact signing key belongs only on Health BI.

## Generate private files

For local application services using external Keycloak:

```bash
make scheduler-env COMPOSE_ENV=external
```

This updates scheduler fields in `infra/env/external-dev.env` and generates
`infra/env/scheduler-healthbi.external.env` for the Health BI operator. Both files
are private, ignored by Git/Docker builds and written with mode 0600. Unrelated
application settings and existing worker credentials are retained. Reruns retain
the generated signing key instead of rotating it.

If the mode's default private application env is missing, the generator creates
it from the matching `infra/env/*.env.example`, preserving the template's full
application configuration and comments. For production, run
`make scheduler-env COMPOSE_ENV=prod`; this creates `infra/env/prod.env` and
`infra/env/scheduler-healthbi.prod.env`. Fill the remaining production credentials
and deployment settings before starting services. Existing files are never
replaced with template defaults. A custom `ENV_FILE` must already exist.

The command does not contact Keycloak, register clients, edit a remote Health BI
deployment, or restart containers. A missing worker client secret stays empty;
the portal integration remains disabled until that credential is supplied.

Choose another application base or import the actual Keycloak-issued worker
secret from a private file:

```bash
make scheduler-env COMPOSE_ENV=external \
  HEALTH_BI_URL=https://dashboards.health.go.ug/report-browser \
  SCHEDULER_CLIENT_SECRET_FILE=/private/path/worker-secret.txt
```

Do not pass a secret value on the command line. An explicit secret-file import
copies its value into the private application env and clears any previous
`HEALTH_BI_KEYCLOAK_CLIENT_SECRET_FILE` override, so that override cannot mask the
new credential. Otherwise existing inline/file-backed credentials are preserved.

Options:

| Variable | Meaning |
| --- | --- |
| `COMPOSE_ENV` | Required mode choice in these examples: `external`, `dev`, `prod`, `gateway` or `remote`. Make's generic default remains `prod`. |
| `ENV_FILE` | Existing private application env to update; otherwise the mode's usual file under `infra/env/`, created from its example if missing. Examples are rejected as destinations. |
| `HEALTH_BI_URL` | Backend-reachable Health BI application base, without `?report` or `/api/v1`. Existing nonempty values are kept unless explicitly overridden. |
| `HEALTH_BI_PUBLIC_URL` | Browser-reachable base for Health BI artifact links, if different from the backend address. |
| `HEALTHBI_ENV_FILE` | Companion Health BI env to update; default `infra/env/scheduler-healthbi.<profile>.env`. Production/gateway/remote share profile `prod`. |
| `SCHEDULER_CLIENT_SECRET_FILE` | Optional file containing the existing Keycloak worker client secret. |

For `external`/production, an unset application base defaults to the production
example's Health BI URL. `dev` defaults to `http://host.docker.internal:8080`, with
public links at `http://localhost:8080`. Docker Desktop resolves that host alias;
on Linux, add `host.docker.internal:host-gateway` to the backend's `extra_hosts`,
or supply a reachable service/IP URL. Health BI must run separately.

## Portal backend settings

| Variable | Purpose / default |
| --- | --- |
| `HEALTH_BI_BASE_URL` | Required for integration. Health BI application base; production example uses `https://dashboards.health.go.ug/report-browser`. |
| `HEALTH_BI_KEYCLOAK_CLIENT_ID` | Dedicated worker client; default `report-scheduler-worker`. |
| `HEALTH_BI_KEYCLOAK_CLIENT_SECRET` | Required Keycloak-issued worker credential, or use its existing `_FILE` setting. It cannot be made valid just by generating a random local string. |
| `HEALTH_BI_TIMEOUT` | HTTP timeout; default `30s`. |
| `REPORT_SCHEDULER_WORKER_INTERVAL` | Due-work polling interval; default `30s`. |
| `REPORT_SCHEDULER_DELIVERY_LINK_TTL` | Lifetime of portal delivery links; default `24h`. |

Existing `KEYCLOAK_BASE_URL` and `KEYCLOAK_REALM` determine the token endpoint.
`APP_BASE_URL` must be browser-reachable for portal delivery links; use the
environment's public API base in production, and localhost only for local users.
Email delivery also uses the normal `SMTP_*` settings. Existing primary database,
storage and report-generation dependencies still apply. No separate scheduler
enable flag is needed; the worker starts with the backend and integration readiness
requires a Health BI URL and service-account credentials.

## Health BI settings

| Variable | Purpose |
| --- | --- |
| `KEYCLOAK_URL`, `KEYCLOAK_REALM` | Shared public issuer and realm, derived from the selected portal env. |
| `SCHEDULER_KEYCLOAK_CLIENT_ID` | Authorized worker client; must match the portal's client ID. |
| `SCHEDULER_KEYCLOAK_AUDIENCE` | `data-statistics`. |
| `SCHEDULER_KEYCLOAK_ROLE` | `report_scheduler_service`. |
| `SCHEDULER_ARTIFACT_SIGNING_KEY` | Health BI-only download-signing secret; generated using 48 random bytes when absent. |
| `SCHEDULER_PUBLIC_BASE_URL` | Browser-reachable Health BI base. |
| `BASE_PATH` | Path prefix from that public URL, e.g. `/report-browser`. |

Install the companion values into Health BI's existing environment. It is a
scheduler snippet, not a complete Health BI deployment file: retain Health BI's
browser-client, application-state database and Chromium/PDF configuration.
For an existing Health BI installation, set `HEALTHBI_ENV_FILE` to its existing
private env when running the tool there, or copy its current signing key into
the companion file before generation. Replacing a live signing key invalidates
previously issued artifact links. A legacy `SCHEDULER_SERVICE_API_KEY` is reused
as the signing key if present and at least 32 characters long.

## Keycloak and activation

In the same realm, create confidential client `report-scheduler-worker`, enable
service accounts and disable browser/password flows. Assign only
`data-statistics:report_scheduler_service` to its service account and include
`data-statistics` in its access-token audience. Copy that client's actual secret
into the portal env. Do not use the portal admin or browser-login client secret.

For a fully local Keycloak/Health BI stack, the existing `make sso-up` tool can
provision/reconcile the worker account and retrieve its credential; see
[the scheduler backend contract](../backend/internal/features/report_scheduler/README.md).
For external/production Keycloak, this generator leaves account provisioning to
the realm administrator.

After both applications have their configuration:

```bash
make compose-up COMPOSE_ENV=external SERVICE=backend
```

Restart/recreate Health BI with its installed settings separately. With an
authorized portal session, check `/api/v1/report-scheduler`, then list reports
and run a test report. `healthBiEnabled=true` indicates local configuration
readiness; it does not prove that the remote credential/role/audience is valid.
Use a real report execution to verify generation and delivery end to end.

Run `make scheduler-env-check` for generator tests; `make infra-check` includes
them alongside Compose and CD contracts.
