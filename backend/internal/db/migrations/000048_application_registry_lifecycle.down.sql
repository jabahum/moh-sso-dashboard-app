-- Inactive applications remain disabled after rollback.
DROP TRIGGER IF EXISTS ihp_application_lifecycle_guard ON ihp_systems;
DROP FUNCTION IF EXISTS ihp_enforce_application_lifecycle();
ALTER TABLE ihp_systems DROP COLUMN contract_version, DROP COLUMN integration_type, DROP COLUMN lifecycle_status;
