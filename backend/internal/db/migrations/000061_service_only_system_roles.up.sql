-- Service-only system roles belong to Keycloak service accounts (for example the
-- report scheduler and the CLIMS/POES identity-sync reconcilers). They must never be
-- granted to people through the portal: no portal permissions, no access-role
-- membership, no realm-role defaults and no RBAC group grants.
ALTER TABLE ihp_system_roles
    ADD COLUMN service_only BOOLEAN NOT NULL DEFAULT FALSE;

-- Keep a copy of any person-facing links removed below so the down migration can
-- restore them exactly.
CREATE TABLE ihp_service_only_roles_backup (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    snapshot JSONB NOT NULL
);

WITH service_roles AS (
    SELECT sr.id, sr.system_id, sr.role_name
    FROM ihp_system_roles sr
    JOIN ihp_systems s ON s.id = sr.system_id
    WHERE (s.client_id, sr.role_name) IN (
        ('data-statistics', 'report_scheduler_service'),
        ('clims', 'identity_sync'),
        ('poesgo', 'identity_sync')
    )
)
INSERT INTO ihp_service_only_roles_backup (id, snapshot)
SELECT TRUE, jsonb_build_object(
    'roleIds', COALESCE((SELECT jsonb_agg(id::text) FROM service_roles), '[]'::jsonb),
    'permissions', COALESCE((SELECT jsonb_agg(jsonb_build_object('roleId', rp.system_role_id::text, 'permissionId', rp.permission_id::text))
        FROM ihp_system_role_permissions rp WHERE rp.system_role_id IN (SELECT id FROM service_roles)), '[]'::jsonb),
    'realmRoles', COALESCE((SELECT jsonb_agg(jsonb_build_object('roleId', rr.system_role_id::text, 'realmRole', rr.realm_role))
        FROM ihp_realm_role_system_roles rr WHERE rr.system_role_id IN (SELECT id FROM service_roles)), '[]'::jsonb),
    'groups', COALESCE((SELECT jsonb_agg(jsonb_build_object('roleId', gr.system_role_id::text, 'groupId', gr.group_id::text))
        FROM ihp_rbac_group_system_roles gr WHERE gr.system_role_id IN (SELECT id FROM service_roles)), '[]'::jsonb),
    'accessRoles', COALESCE((SELECT jsonb_agg(jsonb_build_object('systemId', ar.system_id::text, 'roleName', ar.role_name))
        FROM ihp_system_access_roles ar JOIN service_roles r ON r.system_id = ar.system_id AND r.role_name = ar.role_name), '[]'::jsonb)
);

UPDATE ihp_system_roles SET service_only = TRUE
WHERE id IN (SELECT jsonb_array_elements_text(snapshot->'roleIds')::uuid FROM ihp_service_only_roles_backup);

DELETE FROM ihp_system_role_permissions WHERE system_role_id IN (SELECT id FROM ihp_system_roles WHERE service_only);
DELETE FROM ihp_realm_role_system_roles WHERE system_role_id IN (SELECT id FROM ihp_system_roles WHERE service_only);
DELETE FROM ihp_rbac_group_system_roles WHERE system_role_id IN (SELECT id FROM ihp_system_roles WHERE service_only);
DELETE FROM ihp_system_access_roles ar USING ihp_system_roles sr
WHERE sr.system_id = ar.system_id AND sr.role_name = ar.role_name AND sr.service_only;

-- Enforce the rule for every writer (seed apply, admin API, manifests, ad-hoc SQL).
CREATE FUNCTION ihp_reject_service_only_role_link() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    blocked BOOLEAN;
BEGIN
    IF TG_TABLE_NAME = 'ihp_system_access_roles' THEN
        SELECT bool_or(service_only) INTO blocked FROM ihp_system_roles
        WHERE system_id = NEW.system_id AND role_name = NEW.role_name;
    ELSE
        SELECT service_only INTO blocked FROM ihp_system_roles WHERE id = NEW.system_role_id;
    END IF;
    IF COALESCE(blocked, FALSE) THEN
        RAISE EXCEPTION 'service-only system roles cannot be granted to people (%)', TG_TABLE_NAME
            USING ERRCODE = 'check_violation', CONSTRAINT = 'ihp_service_only_system_role';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER ihp_service_only_role_permission_guard BEFORE INSERT OR UPDATE ON ihp_system_role_permissions
    FOR EACH ROW EXECUTE FUNCTION ihp_reject_service_only_role_link();
CREATE TRIGGER ihp_service_only_realm_role_guard BEFORE INSERT OR UPDATE ON ihp_realm_role_system_roles
    FOR EACH ROW EXECUTE FUNCTION ihp_reject_service_only_role_link();
CREATE TRIGGER ihp_service_only_group_role_guard BEFORE INSERT OR UPDATE ON ihp_rbac_group_system_roles
    FOR EACH ROW EXECUTE FUNCTION ihp_reject_service_only_role_link();
CREATE TRIGGER ihp_service_only_access_role_guard BEFORE INSERT OR UPDATE ON ihp_system_access_roles
    FOR EACH ROW EXECUTE FUNCTION ihp_reject_service_only_role_link();

-- A role cannot become service-only while it is still granted to people.
CREATE FUNCTION ihp_reject_linked_service_only_role() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.service_only AND (
        EXISTS (SELECT 1 FROM ihp_system_role_permissions WHERE system_role_id = NEW.id) OR
        EXISTS (SELECT 1 FROM ihp_realm_role_system_roles WHERE system_role_id = NEW.id) OR
        EXISTS (SELECT 1 FROM ihp_rbac_group_system_roles WHERE system_role_id = NEW.id) OR
        EXISTS (SELECT 1 FROM ihp_system_access_roles WHERE system_id = NEW.system_id AND role_name = NEW.role_name)
    ) THEN
        RAISE EXCEPTION 'remove permissions, access-role, realm-role and group grants before marking role % service-only', NEW.role_name
            USING ERRCODE = 'check_violation', CONSTRAINT = 'ihp_service_only_system_role';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER ihp_service_only_role_guard BEFORE INSERT OR UPDATE OF service_only, role_name, system_id ON ihp_system_roles
    FOR EACH ROW EXECUTE FUNCTION ihp_reject_linked_service_only_role();
