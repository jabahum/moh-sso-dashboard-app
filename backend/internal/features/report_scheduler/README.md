# Report Scheduler backend

The initial module exposes `GET /api/v1/report-scheduler` behind portal authentication, `report-scheduler` system access, and `report_scheduler:read` permission. It uses the standard API response envelope.

Bootstrap constructs the service and handler. Feature routes own the endpoint. The service currently reports `status: setup` and `schedulingEnabled: false`; it does not run jobs or store schedules. Add scheduling persistence and worker execution after defining the report source, recurrence, timezone, delivery, and retry contracts.

See the [frontend module README](../../../../frontend/apps/report-scheduler/README.md) for development and access setup.
