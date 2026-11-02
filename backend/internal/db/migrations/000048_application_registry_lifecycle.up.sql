-- Existing systems retain their enabled flag and access. New systems require onboarding.
ALTER TABLE ihp_systems
    ADD COLUMN lifecycle_status TEXT NOT NULL DEFAULT 'active'
        CHECK (lifecycle_status IN ('registered', 'configured', 'validated', 'approved', 'active', 'suspended', 'retired')),
    ADD COLUMN integration_type TEXT NOT NULL DEFAULT 'external'
        CHECK (integration_type IN ('external', 'hosted_module')),
    ADD COLUMN contract_version TEXT NOT NULL DEFAULT '1';
UPDATE ihp_systems SET integration_type = CASE WHEN system_type = 'platform' THEN 'hosted_module' ELSE 'external' END;
ALTER TABLE ihp_systems ALTER COLUMN lifecycle_status SET DEFAULT 'registered';

-- Enforce lifecycle even when legacy seed/sync/upsert paths set enabled=true.
CREATE FUNCTION ihp_enforce_application_lifecycle() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF OLD.lifecycle_status IN ('validated', 'approved', 'active') AND
           ROW(NEW.launch_url, NEW.owner_team, NEW.owner_name, NEW.owner_email,
               NEW.support_url, NEW.documentation_url, NEW.environment, NEW.system_type,
               NEW.launch_mode, NEW.integration_type, NEW.contract_version, NEW.metadata)
           IS DISTINCT FROM
           ROW(OLD.launch_url, OLD.owner_team, OLD.owner_name, OLD.owner_email,
               OLD.support_url, OLD.documentation_url, OLD.environment, OLD.system_type,
               OLD.launch_mode, OLD.integration_type, OLD.contract_version, OLD.metadata) THEN
            NEW.lifecycle_status := 'configured';
            INSERT INTO ihp_rbac_audit_events (actor_user_id, action, resource_type, resource_id, system_client_id, details)
            VALUES (NULLIF(current_setting('ihp.actor_user_id', true), ''), 'system.lifecycle.invalidated', 'system', NEW.id::text, NEW.client_id,
                jsonb_build_object('from', OLD.lifecycle_status, 'to', 'configured', 'reason', 'application configuration changed'));

        END IF;
    END IF;
    IF NEW.lifecycle_status <> 'active' THEN
        NEW.enabled := FALSE;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER ihp_application_lifecycle_guard BEFORE INSERT OR UPDATE ON ihp_systems
    FOR EACH ROW EXECUTE FUNCTION ihp_enforce_application_lifecycle();
