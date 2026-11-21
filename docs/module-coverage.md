# Module coverage

Generated from `docs/module-catalog.json`; update with `python3 infra/scripts/check-module-catalog.py --write`.

`core` stays in the trusted host. `planned` still uses legacy wiring. `registry-hosted` uses the compiled registry but ships with the host version. `published` has independently versioned package artifacts.

Published backend modules own implementation in `backend/modules/<moduleId>`; remaining host HTTP/provider adapters live in `backend/internal/adapters/modules/<feature>`. Published modules must not retain directories in `backend/internal/features`.

Registry integration does not imply independent Go packaging. Planned domains are tracked expansion work; their workers and migrations have not been transferred to the registry.

## Backend

| Feature | Stage | Integration | Runtime ID | Scope | Dependencies | Workers | Migration owner | Next work |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| admin_units | published | self-contained | admin-units | Administrative geography API | health_context, warehouse | none | core | Maintain published HTTP, warehouse queries and context filtering in the module; select reviewed immutable Go versions. |
| announcements | published | self-contained | announcements | Announcements and audience delivery | users, rbac, health_context, email, notifications, blob-storage | none | core | Maintain published HTTP, persistence, audience policy and delivery orchestration in the module; supply shared platform capabilities and select reviewed immutable Go versions. |
| audit | published | self-contained | audit | Audit read, metrics and export APIs only | primary-db, redis, rbac, host-audit-recorder | none | core | Maintain published read HTTP/SQL/cache/export behavior; keep core event recording and migrations in the platform and select reviewed Go versions. |
| auth | core | host | — | OIDC login, callback and token lifecycle | keycloak, authsession, host-audit-recorder | none | core | Keep authentication in the trusted host; extensions consume authenticated groups and injected identity. |
| authsession | core | host | — | Opaque browser-session backing store | redis | none | none | Retain with authentication; do not expose stored tokens through module contracts. |
| clients | planned | host | — | Keycloak application/client administration | keycloak, rbac, host-audit-recorder | none | core | Separate Keycloak provisioning, local registry and audit ports while retaining governance checks. |
| data_quality | planned | host | — | DQA rules, results and schedules | warehouse, health_context, primary-db | host DQA schedule worker | core and warehouse | Define DQA migration ownership and a joined worker stop/readiness contract before enabling module disablement. |
| document_templates | planned | host | — | Template, sheet and column APIs | primary-db, blob-storage, host-audit-recorder | none | core | Keep all three handler families together and define template/storage ports and publication permissions. |
| documents | planned | host | — | Document ingestion, metadata and processing | storage_locations, blob-storage, primary-db, upload-db, notifications | host document processing worker | core and upload database | Move job claims, cancellation, readiness and shutdown with the domain; define storage and processor ports first. |
| email | planned | host | — | Email administration and recipient resolution | rbac, users, health_context, smtp, primary-db | host email outbox worker | core | Separate recipient resolution and SMTP ports; preserve retry/idempotence and migrate outbox worker ownership. |
| geojson | published | self-contained | geojson | Permission-guarded map asset serving | — | none | none | Publish module-owned HTTP/assets changes independently; consume reviewed Go pins through shared platform registration. |
| health_context | core | host | — | Shared contextual scope and geography policy | primary-db, observatory-db, rbac | host Observatory readiness and geography replication | core and Observatory | Keep shared access policy in the host; modules receive contextual authorization/repository ports. |
| metrics | published | self-contained | metrics | Dashboard metrics service and guarded APIs | primary-db, rbac | none | core | Maintain published HTTP, DTOs and PostgreSQL reads in the module; select reviewed immutable Go versions. |
| notifications | planned | host | — | In-app, email and SMS delivery orchestration | primary-db, email, sms, host-audit-recorder | host notification email delivery, host notification SMS delivery | core | Define delivery/provider and event ports and transfer all retry workers with bounded cancellation and cleanup. |
| platform | core | host | — | Platform inventory, introspection and operations API | primary-db, keycloak, rbac | none | core | Retain platform trust and module lifecycle controls in the host. |
| rbac | core | host | — | Application registry, governance and permissions | primary-db, keycloak, system_rbac | none | core | Keep authoritative permission resolution, manifest review and lifecycle governance in the host. |
| report_scheduler | published | self-contained | report-scheduler | Report schedules, execution and delivery | primary-db, keycloak, health-bi, email, users, notifications, blob-storage, warehouse, host-audit-recorder | registry-owned report scheduler worker | core | Maintain module-owned HTTP routes, OpenAPI and worker implementation; supply host capabilities and apply core migrations. |
| sessions | core | host | — | Authenticated session management and revocation | keycloak, authsession, rbac | none | core | Retain revocation and session guarantees alongside host authentication. |
| storage_locations | planned | host | — | Storage configuration and location administration | primary-db, blob-storage, host-audit-recorder | none | core | Define storage-factory/secret references without leaking credentials; preserve document consumers. |
| surveillance | planned | host | — | Surveillance and outbreak APIs | health_context, primary-db, observatory-db, geojson | none | core and Observatory | Review primary/Observatory repository boundaries and schema ownership before packaging routes. |
| system_rbac | core | host | — | Seed application and system behavior policy | primary-db, health_context | none | core | Retain bootstrap policy and manifest configuration precedence in the host. |
| users | planned | host | — | User administration and bulk import APIs | keycloak, primary-db, notifications, host-audit-recorder | none | core | Define identity/import and delivery ports while preserving user synchronization and existing role checks. |
| visualiser | planned | host | — | Warehouse visualisation and query APIs | warehouse, health_context | none | external warehouse | Inject query and contextual-scope ports; retain warehouse failure/readiness behavior. |

## Frontend

The shell is built from the checkout. This table covers every publishable application and shared package; versions come from the reviewed production selection.

| Package | Kind | Stage | Production version | Backend features |
| --- | --- | --- | --- | --- |
| @moh-sso/announcements | application | published | 1.4.0 | announcements |
| @moh-sso/api | shared | published | 1.4.1 | frontend/shared or external integration |
| @moh-sso/audit | application | published | 1.4.0 | audit |
| @moh-sso/auth | shared | published | 1.4.1 | auth, authsession |
| @moh-sso/clients | application | published | 1.4.0 | clients, rbac |
| @moh-sso/config | shared | published | 1.4.1 | frontend/shared or external integration |
| @moh-sso/data-validation | application | published | 1.4.0 | data_quality |
| @moh-sso/data-visualizer | application | published | 1.4.0 | visualiser |
| @moh-sso/documents | application | published | 1.4.0 | documents, document_templates, storage_locations |
| @moh-sso/e-services | application | published | 1.4.0 | clients, rbac |
| @moh-sso/email | application | published | 1.4.0 | email |
| @moh-sso/issue-tracker | application | published | 1.4.0 | data_quality |
| @moh-sso/microfrontend | shared | published | 1.4.1 | frontend/shared or external integration |
| @moh-sso/rbac | application | published | 1.4.0 | rbac, health_context |
| @moh-sso/report-browser | application | published | 1.4.0 | frontend/shared or external integration |
| @moh-sso/report-scheduler | application | published | 0.5.0 | report_scheduler |
| @moh-sso/state | shared | published | 1.4.1 | frontend/shared or external integration |
| @moh-sso/surveillance | application | published | 1.4.0 | surveillance, geojson |
| @moh-sso/types | shared | published | 1.4.1 | frontend/shared or external integration |
| @moh-sso/ui | shared | published | 1.4.1 | frontend/shared or external integration |
| @moh-sso/users | application | published | 1.4.0 | users |
| @moh-sso/utilities | application | published | 1.4.0 | frontend/shared or external integration |
| @moh-sso/utils | shared | published | 1.4.1 | frontend/shared or external integration |
