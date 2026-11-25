# Configuration reference

The portal is configured in two places:

- **Backend and infrastructure:** environment files under `infra/env/`.
- **Frontend runtime:** a `window.__APP_CONFIG__` script served as `config.js`.

This page explains which file to use and what each group of settings controls. The
example files are the authoritative list of variables and defaults. Copy them; do not
edit the examples with real values.

## Which environment file to use

| File (copy from `*.example`) | Use it for |
| --- | --- |
| `infra/env/dev.env` | Local development with `make compose-dev-up` or `make dev`. Copy to `backend/app.env` instead when running Go outside Docker, and point DB/Redis hosts at `localhost` and the published ports. |
| `infra/env/external-dev.env` | Local services signing in through the production Keycloak server. Other defaults come from `dev.env.example`. |
| `infra/env/prod.env` | Production Compose. The database password and Keycloak admin/web client secrets are read from files (`infra/secrets/*.txt` by default, or the `*_SECRET_FILE` paths); other credentials stay in this file. See [deployment](deployment.md#production-and-development-environments). |
| `infra/env/remote-proxy.env` | Nginx on a separate machine; holds the application's private IP. See [Nginx on a separate machine](deployment.md#nginx-on-a-separate-machine). |
| `infra/env/core.env` | The core-only distribution profile. See [platform core](platform-core.md). |
| `infra/env/observatory.env` | A local Observatory PostgreSQL for surveillance. See [surveillance database](surveillance-database.md). |
| `infra/env/identity.env`, `infra/env/identity-shared.env` | Operator-only Keycloak realm bootstrap. Keep the populated copies outside the repository. See [identity environments](identity-environments.md). |
| `infra/env/frontend.env` | Vite dev-server proxy settings for the visualizer (`VITE_*`). |

Never commit populated copies of these files.

## Backend settings by group

Groups follow the section headings in `infra/env/dev.env.example`.

| Group | Main variables | More detail |
| --- | --- | --- |
| Application runtime | `ENVIRONMENT`, `GIN_MODE`, `SERVER_PORT`, `APP_BASE_URL`, `PLATFORM_PROFILE`, `DISABLED_MODULES`, `DEV_AUTH_BYPASS`, `ALLOW_INSECURE_HTTP` | [Startup profiles](platform-core.md#startup-profiles), [module operations](module-operations.md), [portal hardening](operations/portal-hardening.md) |
| Frontend and browser authentication | `FRONTEND_BASE_URL`, `FRONTEND_REDIRECT_URI`, `AUTH_RETURN_URL_ALLOWED_ORIGINS`, `API_BASE_URL`, `LOGIN_URL`, `COOKIE_DOMAIN`, `DOCS_URL` | [Cookie domain](#cookie-domain-in-local-development), [cross-application SSO](cross-application-sso.md) |
| Keycloak infrastructure | `KEYCLOAK_VERSION`, `KEYCLOAK_DB*`, `KEYCLOAK_ADMIN`, `KEYCLOAK_ADMIN_PASSWORD`, `KEYCLOAK_HOSTNAME` | [Deployment](deployment.md) |
| Keycloak application clients | `KEYCLOAK_EXTERNAL_URL`, `KEYCLOAK_BASE_URL`, `KEYCLOAK_INTERNAL_URL`, `KEYCLOAK_REALM`, `KEYCLOAK_ADMIN_CLIENT_*`, `KEYCLOAK_WEB_CLIENT_*`, `KEYCLOAK_REDIRECT_URI` | [Identity environments](identity-environments.md), [identity contract](platform-identity.md) |
| RBAC startup synchronization | `RBAC_STARTUP_SYNC_ENABLED`, `RBAC_STARTUP_SEED_*`, `RBAC_STARTUP_SYNC_*` | [RBAC and Keycloak sync](rbac-keycloak-sync.md#startup-sync) |
| Backend database | `DB_DRIVER`, `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_ENABLE_SSL` | |
| Redis | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` | |
| Application tokens | `TOKEN_SYMMETRIC_KEY`, `ACCESS_TOKEN_DURATION`, `REFRESH_TOKEN_DURATION` | |
| TLS | `ENABLE_TLS`, `TLS_CERT`, `TLS_KEY`, `TLS_KEYCLOAK_CA_CERT_PATH` | |
| Upload management database | `REMOTE_DB_HOST`, `REMOTE_DB_PORT`, `REMOTE_DB_USER`, `REMOTE_DB_PASSWORD`, `REMOTE_DB_NAME` | |
| Data warehouse / visualizer | `DWH_HOST`, `DWH_PORT`, `DWH_USERNAME`, `DWH_PASSWORD`, `DWH_DB`, `DQA_DB_USERNAME`, `DQA_DB_PASSWORD` | Blank values use the Compose defaults (the upload database); `DQA_DB_*` falls back to the DWH credentials. |
| File storage | `STORAGE_PROVIDER` (`local`, `nfs`, `s3` or `minio`), `LOCAL_BASE_PATH`, `NFS_BASE_PATH`, `S3_*`, `MINIO_*` | [Storage provider](#storage-provider) |
| Primary SMTP | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, `SMTP_FROM_NAME`, `SMTP_CONNECT_TIMEOUT`, `SMTP_SEND_TIMEOUT` | |
| Email attachment limits | `EMAIL_MAX_ATTACHMENTS`, `EMAIL_MAX_ATTACHMENT_BYTES`, `EMAIL_ALLOWED_ATTACHMENT_TYPES` | [Attachment limits](#attachment-limits) |
| Announcement attachment limits | `ANNOUNCEMENT_MAX_ATTACHMENTS`, `ANNOUNCEMENT_MAX_ATTACHMENT_BYTES`, `ANNOUNCEMENT_ALLOWED_ATTACHMENT_TYPES` | [Attachment limits](#attachment-limits) |
| SMS delivery and providers | `SMS_ENABLED`, `SMS_PROVIDER`, `SMS_*`, `AFRICASTALKING_*`, `TWILIO_*` | [SMS delivery](sms-delivery.md) |
| Notification delivery worker and retries | `NOTIFICATION_DELIVERY_WORKER_ENABLED`, `NOTIFICATION_DELIVERY_WORKER_INTERVAL`, `NOTIFICATION_DELIVERY_BATCH_SIZE`, `RETRY_MAX_ATTEMPTS`, `RETRY_BASE_DELAY` | [Notification delivery runbook](operations/notification-delivery-runbook.md) |
| Platform and administrator contact | `PLATFORM_NAME`, `SYSTEM_ADMIN_NAME`, `SYSTEM_ADMIN_EMAIL`, `ADMIN_DASHBOARD_URL` | |
| Report Scheduler / Health BI | `HEALTH_BI_BASE_URL`, `HEALTH_BI_KEYCLOAK_CLIENT_*`, `HEALTH_BI_TIMEOUT`, `REPORT_SCHEDULER_*` | [Scheduler configuration](scheduler-configuration.md) |
| Issue-assignment SMTP (optional) | `SMTP_*_GM` | Same fields as the primary SMTP group, used for issue-tracker assignment mail. |
| Platform service API (optional) | `PLATFORM_AUTH_CLIENT_ID`, `PLATFORM_AUTH_CLIENT_SECRET`, `PLATFORM_AUTH_AUDIENCE` | [Identity contract](platform-identity.md) |
| Observatory database (surveillance) | `OBSERVATORY_DB_*` | [Surveillance database](surveillance-database.md) |

### Cookie domain in local development

Leave `COOKIE_DOMAIN` empty when developing on `localhost`:

```env
COOKIE_DOMAIN=
```

Do not set `COOKIE_DOMAIN=localhost`. Browsers reject or ignore cookies with
`Domain=localhost`, so sign-in appears to succeed but the session cookie is never sent
back.

### Storage provider

The backend validates the provider's settings at startup (`backend/internal/config/config.go`); an empty `STORAGE_PROVIDER` skips the check:

| `STORAGE_PROVIDER` | Required settings |
| --- | --- |
| `local` | `LOCAL_BASE_PATH`, `APP_BASE_URL` |
| `nfs` | `NFS_BASE_PATH`, `APP_BASE_URL` |
| `s3` | `S3_BUCKET`, `S3_REGION` (set `S3_ACCESS_KEY_ID` and `S3_SECRET_ACCESS_KEY` for the bucket; they are not checked at startup) |
| `minio` | `MINIO_ENDPOINT`, `MINIO_BUCKET` (set `MINIO_ACCESS_KEY_ID` and `MINIO_SECRET_ACCESS_KEY`; they are not checked at startup) |

### Attachment limits

If email or announcement attachments are rejected, check the matching
`*_MAX_ATTACHMENTS`, `*_MAX_ATTACHMENT_BYTES` and `*_ALLOWED_ATTACHMENT_TYPES` values.
Email attachment payloads must provide exactly one of `path` or `data_base64`; see the
[API reference](api-reference.md).

## Frontend runtime configuration

The shell reads `window.__APP_CONFIG__` from `config.js` at runtime, so one build can run
in different modes. Select a config from `frontend/`:

```bash
npm run config:dev            # public/config.development.js
npm run config:prod           # public/config.production.js
npm run config:local-remote   # public/config.local-remote.js
```

The keys that decide how microfrontends load:

| Mode | `microfrontendMode` | `singleSpaOrchestration` | `microfrontendMountMode` | Use |
| --- | --- | --- | --- | --- |
| Local / hybrid (`config.development.js`) | `"local"` | `false` | `"hybrid"` | Fast local development with workspace imports. |
| Local remote (`config.local-remote.js`) | `"remote"` | `false` | `"hybrid"` | Workspace shell loading separately served module bundles without single-spa orchestration. |
| Remote / orchestrated (`config.production.js`) | `"remote"` | `true` | `"orchestrated"` | Production-style single-spa orchestration with import maps and separately staged bundles. |

`API_BASE_URL` in the same object sets where the shell sends API requests. A blank page
at `/` usually means the wrong config is active; see
[frontend troubleshooting](../frontend/docs/troubleshooting.md) and
[import maps](../frontend/docs/import-maps.md).
