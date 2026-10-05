# Report Scheduler

Portal module foundation for recurring report generation and delivery.

- Frontend: `frontend/apps/report-scheduler`, mounted at `/apps/dwh/report-scheduler` (normally `/portal/apps/dwh/report-scheduler`).
- Backend: `backend/internal/features/report_scheduler`.
- API: `GET /api/v1/report-scheduler` returns `{ success: true, data: { name: "Report Scheduler", status: "setup", schedulingEnabled: false } }`.
- Access: `data-statistics` system plus `report_scheduler:read` permission.
- Keycloak role: `data-statistics:report-scheduler_access`.

This setup provides the module boundary, protected API, React entry screen, shared session handling, and local/production microfrontend wiring. Schedule persistence, report generation, workers, delivery, and run history are not implemented yet. The screen explicitly shows scheduling as coming soon.

## Development

Run commands from `frontend/`:

```sh
npm run dev:mf:report-scheduler
npm run build -w @moh-sso/report-scheduler
npm run preview:mf:report-scheduler
```

The local microfrontend uses port 4115. Start the shell using its existing development flow. The module uses the portal API and login session; it does not own an OAuth callback.

## Enable access

Report Scheduler is a module of the Data & Statistics registry client. Its navigation entry and client role are included in the development, production, and Helm development realm exports. Realm import files only affect imported realms; existing installations require updating the `data-statistics` client navigation and adding its `report-scheduler_access` role with the same `portal.permissions` attributes.

From `backend/`, validate and apply the RBAC seed to the intended database:

```sh
go run ./cmd/cli system-rbac validate --file config/system-rbac.seed.yaml
go run ./cmd/cli system-rbac seed --file config/system-rbac.seed.yaml
```

Assign `data-statistics:report-scheduler_access` to authorized users or groups in Keycloak, then refresh their session. No users receive this role automatically. Authentication continues through `dashboard-web`.

If the earlier standalone `report-scheduler` client was installed, move its user/group role assignments to `data-statistics:report-scheduler_access`, retire that client in Keycloak, and disable its old system entry in portal RBAC. Apply the updated seed and sync the parent client so the launcher and side navigation use the Data & Statistics module entry. The backend now requires the parent system and module permission; the standalone client role no longer grants access.
