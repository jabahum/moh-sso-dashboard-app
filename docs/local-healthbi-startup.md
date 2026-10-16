# Start the portal and Health BI together

`keycloak/realm-export.json` owns the Health BI client definitions. On a fresh
Keycloak database, the existing `--import-realm` startup imports `datawarehouse-report-browser`, the
scheduler worker, its service-account role grant, and its audience/scope mappings.
Matching definitions are included in the production export and Helm dev realm.
No committed worker secret is needed: Keycloak generates it during import.

Run `make dev` (or `make sso-up`) from `moh-sso-dashboard` to start the Docker SSO
infrastructure, prepare Health BI's synthetic PostgreSQL database, start its host
Go/Air server, then start the portal backend and frontend. Normal startup reads
the imported clients and credential; it does not create or update clients. It
verifies a worker token against Health BI's protected API before reporting success.
Docker, Go, Python 3, and local Chrome/Chromium are required.

For a persisted realm created before this change, run once:

```sh
make sso-reconcile-keycloak
make dev
```

The reconciliation command reads the same realm export and applies only the
Health BI clients and service-role mappings. It retains existing credentials,
redirects, user accounts, and realm data. It does not delete the older
`datawarehouse-report-browser` client or its sessions; new Health BI configuration
uses `datawarehouse-report-browser`. No database reset or full realm replacement is required.

Existing development images are reused; missing images are built by Compose.
Use `SSO_BUILD=1 make dev` when Dockerfiles or image dependencies change.

The sibling checkout defaults to `../dwh_visualization`. Override it with
`HEALTHBI_PROJECT_DIR=/path/to/dwh_visualization make dev`. Both checkouts must
contain the unified-startup changes. Health BI's existing `make dev` remains usable
on its own. `make dev-native` retains the portal's former native backend/frontend
command when infrastructure is managed separately. The existing Kubernetes
`make local-up` and `make dev-up` targets retain their previous meanings.

| Component                                  | Identity / address                                                     |
| ------------------------------------------ | ---------------------------------------------------------------------- |
| Portal report-browser module               | `/apps/dwh/dashboards`, permission `report_browser:read`               |
| Parent system and existing report roles    | Keycloak client `data-statistics`                                      |
| Health BI browser login                    | Public PKCE client `datawarehouse-report-browser`                                         |
| Scheduled generation                       | Confidential client `report-scheduler-worker`                          |
| Worker grant                               | `data-statistics:report_scheduler_service`, audience `data-statistics` |
| Portal                                     | `http://localhost:3000`                                                |
| Portal API                                 | `http://localhost:9000`                                                |
| Health BI browser and signed artifact URLs | `http://localhost:8080`                                                |
| Health BI URL from the backend container   | `http://host.docker.internal:8080`                                     |
| Shared Keycloak                            | `http://localhost:8081`, realm `moh-realm`                             |

The report-browser frontend module and the Keycloak browser login client are
separate components. Reusing a confidential portal/worker client in browser code
would expose credentials. The public browser client reads the existing report
roles from `data-statistics`; the worker has no portal user permissions or launcher
entry. Both authentication clients explicitly opt out of the portal launcher;
the existing report-browser module remains the navigation entry.

The realm export is the only canonical client configuration. Health BI's
`tools/keycloak/healthbi-local-client.json` is a standalone example, not another
startup source. The helper uses the local Keycloak bootstrap admin settings only
in memory to read the worker credential (or apply an explicitly requested upgrade).
If the existing admin password was changed, supply `LOCAL_KEYCLOAK_ADMIN_USERNAME`
and `LOCAL_KEYCLOAK_ADMIN_PASSWORD` in the shell. The helper refuses non-loopback
Keycloak addresses and must not be used for production credential distribution.

Generated state lives in ignored `.local-sso/` with private file permissions:
worker environment, Compose override, buildx metadata, managed Health BI PID, and startup log.
The shared worker credential is only passed to the portal backend. Health BI's
artifact signing key is generated once if missing and preserved on subsequent
runs, so previously issued links remain valid. The original root environment files
in either checkout are untouched. Only Health BI's isolated `.local-docker/.env`
receives the aligned local auth settings; unrelated local settings are preserved.

Health BI permits embedding only from `http://localhost:3000` using
`PORTAL_FRAME_ANCESTORS`. Other origins remain blocked by CSP. Leaving the setting
empty preserves an unconfigured deployment's existing reverse-proxy framing
policy. Invalid explicit settings fail closed with `frame-ancestors 'none'` and
`X-Frame-Options: DENY`; the general security-header middleware also denies
framing by default. Configure the reverse proxy consistently if it sets its own
CSP or X-Frame-Options headers.
The portal iframe uses `VITE_HEALTH_BI_BASE_URL` for local development, or runtime
`window.__APP_CONFIG__.HEALTH_BI_BASE_URL` when configured. Its production default
remains `https://dashboards.health.go.ug/report-browser`. For deployed environments,
configure the actual portal origin and Health BI base URL explicitly; this local
helper does not change production credentials, Helm deployments, or warehouse data.

Use `make sso-status` to inspect services and `make sso-down` to stop them. Database
volumes and generated credentials are retained. A Health BI process started by
this helper is stopped with its process group; an already running developer-owned
Health BI server is reused and left running. A startup failure is visible in the
command output; Health BI build/runtime output is in `.local-sso/healthbi.log` for
a managed process, or the original terminal/log for a reused process.

Run `python3 scripts/local-sso.test.py` for bootstrap contracts. Health BI's
`go test ./server -run TestPortalFrameHeaders` verifies framing restrictions.

To exercise a fresh import against the local Keycloak without touching the main
realm, run `TEST_KEYCLOAK_IMPORT=1 python3 scripts/local-sso.test.py`. This creates
a uniquely named temporary realm from the export's Health BI definitions, checks
client-credentials authentication and audience/role claims, then removes only
that test realm.
