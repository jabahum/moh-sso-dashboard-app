# Report Scheduler

Portal module foundation for recurring report generation and delivery.

- Frontend: `frontend/apps/report-scheduler`, mounted at `/apps/report-scheduler` (normally `/portal/apps/report-scheduler`).
- Backend: `backend/internal/features/report_scheduler`.
- API: `GET /api/v1/report-scheduler` returns `{ success: true, data: { name: "Report Scheduler", status: "setup", schedulingEnabled: false } }`.
- Access: `report-scheduler` system plus `report_scheduler:read` permission.
- Keycloak role: `report-scheduler_access`.

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

The registry client and role are included in the development, production, and Helm development realm exports. Realm import files only affect imported realms; existing Keycloak installations require creating the same client and role through the established administration flow.

From `backend/`, validate and apply the RBAC seed to the intended database:

```sh
go run ./cmd/cli system-rbac validate --file config/system-rbac.seed.yaml
go run ./cmd/cli system-rbac seed --file config/system-rbac.seed.yaml
```

Assign `report-scheduler_access` to authorized users or groups in Keycloak, then refresh their session. No users receive this role automatically. This is a registry client, with all login and service-account flows disabled. Authentication continues through `dashboard-web`.
