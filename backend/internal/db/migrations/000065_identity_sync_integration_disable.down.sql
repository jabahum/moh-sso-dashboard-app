-- Restore migration 62's trigger function (binding checked on every write).
CREATE OR REPLACE FUNCTION ihp_check_identity_sync_integration() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM ihp_application_identities
        WHERE oauth_client_id = NEW.sync_client_id
          AND application_client_id = NEW.application_client_id
          AND enabled
    ) THEN
        RAISE EXCEPTION 'sync client % is not an enabled service identity of application %', NEW.sync_client_id, NEW.application_client_id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'ihp_identity_sync_sync_client';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM ihp_system_access_roles ar JOIN ihp_systems s ON s.id = ar.system_id
        WHERE s.client_id = NEW.application_client_id AND ar.role_name = NEW.access_role
    ) OR EXISTS (
        SELECT 1 FROM ihp_system_roles sr JOIN ihp_systems s ON s.id = sr.system_id
        WHERE s.client_id = NEW.application_client_id AND sr.role_name = NEW.access_role AND sr.service_only
    ) THEN
        RAISE EXCEPTION 'role % is not a person-facing access role of application %', NEW.access_role, NEW.application_client_id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'ihp_identity_sync_access_role';
    END IF;
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;
