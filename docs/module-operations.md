# Operating compiled modules

Use [the coverage guide](module-coverage.md) to distinguish packaged modules,
registry-hosted features, planned extraction work and mandatory host capabilities.
Use `GET /api/v1/admin/modules` with an authenticated administrator session to
inspect the modules actually compiled into the running backend.

| Runtime ID | Scope | Version reported | Effect of disablement |
| --- | --- | --- | --- |
| metrics | Dashboard metrics APIs | Selected Go dependency version | Metrics routes are omitted |
| audit | Audit read, metrics and export APIs | Backend build version | Audit read routes are omitted; core event recording continues |
| geojson | Permission-guarded static map assets | Backend build version | GeoJSON routes are omitted; map consumers need availability review |

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
The audit read module and map asset module have no background workers to stop.
Worker-bearing legacy features are tracked as planned and cannot yet be disabled
through this setting.

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
