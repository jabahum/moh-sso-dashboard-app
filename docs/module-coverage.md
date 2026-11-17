# Module coverage

Generated from `docs/module-catalog.json`; update with `python3 infra/scripts/check-module-catalog.py --write`.

`core` stays in the trusted host. `planned` still uses legacy wiring. `registry-hosted` uses the compiled registry but ships with the host version. `published` has independently versioned package artifacts.

Registry integration does not imply independent Go packaging. Planned domains are tracked expansion work; their workers and migrations have not been transferred to the registry.

## Backend

| Feature | Stage | Runtime ID | Scope | Dependencies | Workers | Migration owner | Next work |
| --- | --- | --- | --- | --- | --- | --- | --- |
| admin_units | planned | — | Administrative geography API | health_context, primary-db | none | core | Inject health-context visibility and administrative-geography repository ports before registry migration. |
| announcements | planned | — | Announcements and audience delivery | users, rbac, health_context, email, notifications, blob-storage | none | core | Separate audience, attachment and delivery ports; preserve public/protected/admin route groups and publication behavior. |
| audit | registry-hosted | audit | Audit read, metrics and export APIs only | primary-db, redis, rbac, host-audit-recorder | none | core | Define public cache/repository ports and response contracts before independent packaging. Core event recording remains in the host. |
| auth | core | — | OIDC login, callback and token lifecycle | keycloak, authsession, host-audit-recorder | none | core | Keep authentication in the trusted host; extensions consume authenticated groups and injected identity. |
| authsession | core | — | Opaque browser-session backing store | redis | none | none | Retain with authentication; do not expose stored tokens through module contracts. |
| clients | planned | — | Keycloak application/client administration | keycloak, rbac, host-audit-recorder | none | core | Separate Keycloak provisioning, local registry and audit ports while retaining governance checks. |
| data_quality | planned | — | DQA rules, results and schedules | warehouse, health_context, primary-db | host DQA schedule worker | core and warehouse | Define DQA migration ownership and a joined worker stop/readiness contract before enabling module disablement. |
| document_templates | planned | — | Template, sheet and column APIs | primary-db, blob-storage, host-audit-recorder | none | core | Keep all three handler families together and define template/storage ports and publication permissions. |
| documents | planned | — | Document ingestion, metadata and processing | storage_locations, blob-storage, primary-db, upload-db, notifications | host document processing worker | core and upload database | Move job claims, cancellation, readiness and shutdown with the domain; define storage and processor ports first. |
| email | planned | — | Email administration and recipient resolution | rbac, users, health_context, smtp, primary-db | host email outbox worker | core | Separate recipient resolution and SMTP ports; preserve retry/idempotence and migrate outbox worker ownership. |
| geojson | registry-hosted | geojson | Permission-guarded map asset serving | geojson-assets, rbac | none | none | Define a public asset-reader and authorization contract before independent packaging; preserve the filename allowlist. |
| health_context | core | — | Shared contextual scope and geography policy | primary-db, observatory-db, rbac | host Observatory readiness and geography replication | core and Observatory | Keep shared access policy in the host; modules receive contextual authorization/repository ports. |
| metrics | published | metrics | Dashboard metrics service and guarded APIs | primary-db, rbac | none | core | Use the published v0.1.0 package as the independent-module reference; change its pin only after release verification. |
| notifications | planned | — | In-app, email and SMS delivery orchestration | primary-db, email, sms, host-audit-recorder | host notification email delivery, host notification SMS delivery | core | Define delivery/provider and event ports and transfer all retry workers with bounded cancellation and cleanup. |
| platform | core | — | Platform inventory, introspection and operations API | primary-db, keycloak, rbac | none | core | Retain platform trust and module lifecycle controls in the host. |
| rbac | core | — | Application registry, governance and permissions | primary-db, keycloak, system_rbac | none | core | Keep authoritative permission resolution, manifest review and lifecycle governance in the host. |
| report_scheduler | planned | — | Report schedules, execution and delivery | primary-db, keycloak, health-bi, notifications, host-audit-recorder | host report scheduler worker | core | Next worker-bearing candidate: inject report provider/delivery ports and transfer schedule claims, readiness, retries and joined shutdown. |
| sessions | core | — | Authenticated session management and revocation | keycloak, authsession, rbac | none | core | Retain revocation and session guarantees alongside host authentication. |
| storage_locations | planned | — | Storage configuration and location administration | primary-db, blob-storage, host-audit-recorder | none | core | Define storage-factory/secret references without leaking credentials; preserve document consumers. |
| surveillance | planned | — | Surveillance and outbreak APIs | health_context, primary-db, observatory-db, geojson | none | core and Observatory | Review primary/Observatory repository boundaries and schema ownership before packaging routes. |
| system_rbac | core | — | Seed application and system behavior policy | primary-db, health_context | none | core | Retain bootstrap policy and manifest configuration precedence in the host. |
| users | planned | — | User administration and bulk import APIs | keycloak, primary-db, notifications, host-audit-recorder | none | core | Define identity/import and delivery ports while preserving user synchronization and existing role checks. |
| visualiser | planned | — | Warehouse visualisation and query APIs | warehouse, health_context | none | external warehouse | Inject query and contextual-scope ports; retain warehouse failure/readiness behavior. |

## Frontend

The shell is built from the checkout. This table covers every publishable application and shared package; versions come from the reviewed production selection.

| Package | Kind | Stage | Production version | Backend features |
| --- | --- | --- | --- | --- |
| @moh-sso/announcements | application | published | 1.3.2 | announcements |
| @moh-sso/api | shared | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/audit | application | published | 1.3.2 | audit |
| @moh-sso/auth | shared | published | 1.3.2 | auth, authsession |
| @moh-sso/clients | application | published | 1.3.2 | clients, rbac |
| @moh-sso/config | shared | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/data-validation | application | published | 1.3.2 | data_quality |
| @moh-sso/data-visualizer | application | published | 1.3.2 | visualiser |
| @moh-sso/documents | application | published | 1.3.2 | documents, document_templates, storage_locations |
| @moh-sso/e-services | application | published | 1.3.2 | clients, rbac |
| @moh-sso/email | application | published | 1.3.2 | email |
| @moh-sso/issue-tracker | application | published | 1.3.2 | data_quality |
| @moh-sso/microfrontend | shared | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/rbac | application | published | 1.3.2 | rbac, health_context |
| @moh-sso/report-browser | application | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/report-scheduler | application | published | 0.4.2 | report_scheduler |
| @moh-sso/state | shared | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/surveillance | application | published | 1.3.2 | surveillance, geojson |
| @moh-sso/types | shared | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/ui | shared | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/users | application | published | 1.3.2 | users |
| @moh-sso/utilities | application | published | 1.3.2 | frontend/shared or external integration |
| @moh-sso/utils | shared | published | 1.3.2 | frontend/shared or external integration |
