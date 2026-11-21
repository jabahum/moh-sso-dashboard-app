# Operating compiled modules

Use [the platform core boundaries](platform-core.md) for mandatory capability
ownership and current bootstrap limitations.

Use [the coverage guide](module-coverage.md) to distinguish packaged modules,
registry-hosted features, planned extraction work and mandatory host capabilities.
Use `GET /api/v1/admin/modules` with an authenticated administrator session to
inspect the modules actually compiled into the running backend.

| Runtime ID | Scope | Version reported | Effect of disablement |
| --- | --- | --- | --- |
| metrics | Dashboard metrics APIs | Selected Go dependency version | Metrics routes are omitted |
| audit | Audit read, metrics and export APIs | Selected Go dependency version | Audit read routes are omitted; core event recording continues |
| geojson | Permission-guarded static map assets | Selected Go dependency version | GeoJSON routes are omitted; map consumers need availability review |
| report-scheduler | Report schedules, execution and delivery | Selected Go dependency version | All scheduler routes, including public downloads, are omitted; worker startup is skipped |
| announcements | Public, audience and administrative announcements, including attachments | Selected Go dependency version | All public, protected and admin announcement routes are omitted; shared delivery workers continue |
| admin-units | Organizational units, facilities and hierarchy lookup | Selected Go dependency version | All nine Administrative Units routes are omitted; Visualizer routes, health-context services and warehouse data remain |

The default empty `DISABLED_MODULES` enables all compiled definitions. Set a
comma-separated list, such as `audit,geojson`, in the backend environment and
restart/recreate the backend through the normal deployment workflow. Whitespace
is trimmed. Unknown IDs fail startup. The registry inventory still lists disabled
definitions with `enabled=false`. Their initialization and registration callbacks
are skipped. Existing core authentication, authorization, session and event
recording services retain their lifecycle.

Review affected application registrations and navigation when changing API
availability. Keep Keycloak roles and application lifecycle approval aligned with
the intended rollout; module configuration does not automatically edit them.
Announcements, the audit read module and map asset module have no background workers to stop.
Announcements disablement takes effect after a backend restart. It preserves
announcement records, audience selections and attachment objects. It does not
cancel notifications already queued in the shared notification outbox; that
worker serves other modules and retains its host lifecycle. Re-enable the module
and restart to restore access to existing records and attachment downloads.
Report Scheduler is pinned to its published Go package and owns its worker through the registry. Its initial persistence
heartbeat must succeed within ten seconds before jobs can be claimed. Startup
fails if scheduler persistence is unavailable. Cleanup cancels and joins the worker
within the host shutdown deadline before database resources close. Disablement
retains schedules, history and pending jobs; re-enabling resumes existing claim,
retry and stale-claim recovery behavior. Previously issued public download links
are unavailable while disabled. Other worker-bearing legacy features remain
tracked as planned.

For version changes, select verified Go/npm releases, review dependency files and
build new images. The frontend pin selection is independent of this backend
availability setting. Database data and migration ownership remain explicit in
the catalogue; disabling an API does not remove its schema or historical data.

For rollout, backup and recovery, use `infra/distribution.md` and
`infra/server/README.md`. Promote the same verified image-pair bundle from staging
to production. Compare application versions, compiled inventory, frontend module
versions, readiness and affected permissions after deployment. Use the documented
backup/compatibility procedure when an image rollback cannot use an upgraded schema.

For local verification, the authenticated distribution fixture exercises real
Keycloak roles, module inventory, API disablement and continued core audit
recording, as well as manifests, backup/restore and restart recovery. It uses
disposable projects and does not contact an existing production identity provider.


Report Scheduler owns its public and protected HTTP routes, OpenAPI contract,
and worker implementation. The dashboard supplies authenticated request
context, rate limiting, and shared services such as email, audit, and
notifications. Apply the dashboard's core database migrations before starting
the scheduler worker.

The dashboard pins the standalone Go module in `backend/go.mod` and records its
checksums in `backend/go.sum`. After changing the pin, rebuild the backend
image, verify the compiled module inventory reports the selected version, and
review scheduler schema compatibility before promoting the image pair. See the
[scheduler backend guide](scheduler-backend.md) for the schema contract.



Audit read APIs select `v0.1.0` from
`github.com/mohuganda/moh-sso-dashboard/backend/modules/audit`, published under `backend/modules/audit/v0.1.0`. Its compiled inventory
version comes from the selected dependency. SQLC queries/mapping, optional Redis
cache, HTTP authorization and export limits remain host adapters. Core event
recording and all audit migrations retain host ownership. Publishing a library
tag does not deploy the running dashboard; rebuild and promote the verified
backend image to consume an updated pin.


GeoJSON selects `v0.2.0` from
`github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson`, published
under `backend/modules/geojson/v0.2.0`. It owns HTTP handling, permission
requirements, filesystem overrides and embedded map snapshots. The host
supplies shared security/trace capabilities and the reusable HTTP registrar;
there is no GeoJSON-specific host adapter or loose image asset directory.
Embedded responses use stable content ETags for conditional reads. Range,
content-type, cache and nosniff behavior remains module-owned. Publishing the
package does not deploy a running installation; select its reviewed Go
pin/checksums and rebuild. Verify releases with
`python3 infra/scripts/test-geojson-release.py`.

Announcements selects `v0.1.0` from
`github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements`, published
under `backend/modules/announcements/v0.1.0`. Its inventory version comes from
the selected dependency. The host retains SQLC/persistence mapping, identity and
storage adapters, HTTP authorization/rate limits, email normalization, shared
notification workers and audit recording. A version change requires reviewing
the Go pin/checksums and rebuilding the backend image. Publishing the module
does not deploy a running installation.


Administrative Units is registry-hosted under `admin-units`. Set
`DISABLED_MODULES=admin-units` and restart/recreate the backend to omit all nine
`/api/v1/visualizer/adminunits/*` endpoints. It owns no workers and does not
modify warehouse data or schemas. Re-enable and restart to restore existing
lookups. Enabled registration retains the same system access, selected health
context and read-permission alternatives as the legacy route path. Other
Visualizer routes retain their registration and guards.

Administrative Units selects `v0.1.0` from
`github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units`, published
under `backend/modules/admin-units/v0.1.0`. Its inventory reports the selected
dependency. Warehouse queries/connections, health-context authorization and HTTP
guards remain host adapters. Version changes require reviewing Go pins/checksums
and rebuilding the backend image; publishing a library does not deploy it.

## Minimal platform profile

`PLATFORM_PROFILE=core` disables all packaged applications and omits legacy
optional routes, providers and workers, while retaining identity/session,
authorization/governance, health-context policy and audit recording.
The default `full` profile preserves the existing distribution.
See [startup profiles](platform-core.md#startup-profiles) for the resource
matrix, readiness rules, shared migration compatibility and the backend-only
`infra/compose/docker-compose.core.yml` entry point.
