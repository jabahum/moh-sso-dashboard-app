DROP TRIGGER IF EXISTS ihp_service_only_role_guard ON ihp_system_roles;
DROP TRIGGER IF EXISTS ihp_service_only_access_role_guard ON ihp_system_access_roles;
DROP TRIGGER IF EXISTS ihp_service_only_group_role_guard ON ihp_rbac_group_system_roles;
DROP TRIGGER IF EXISTS ihp_service_only_realm_role_guard ON ihp_realm_role_system_roles;
DROP TRIGGER IF EXISTS ihp_service_only_role_permission_guard ON ihp_system_role_permissions;
DROP FUNCTION IF EXISTS ihp_reject_linked_service_only_role();
DROP FUNCTION IF EXISTS ihp_reject_service_only_role_link();

-- Restore person-facing links that the up migration removed.
INSERT INTO ihp_system_role_permissions (system_role_id, permission_id)
SELECT (link->>'roleId')::uuid, (link->>'permissionId')::uuid
FROM ihp_service_only_roles_backup b, jsonb_array_elements(b.snapshot->'permissions') link
WHERE EXISTS (SELECT 1 FROM ihp_system_roles WHERE id = (link->>'roleId')::uuid)
  AND EXISTS (SELECT 1 FROM ihp_permissions WHERE id = (link->>'permissionId')::uuid)
ON CONFLICT DO NOTHING;

INSERT INTO ihp_realm_role_system_roles (realm_role, system_role_id)
SELECT link->>'realmRole', (link->>'roleId')::uuid
FROM ihp_service_only_roles_backup b, jsonb_array_elements(b.snapshot->'realmRoles') link
WHERE EXISTS (SELECT 1 FROM ihp_system_roles WHERE id = (link->>'roleId')::uuid)
ON CONFLICT DO NOTHING;

INSERT INTO ihp_rbac_group_system_roles (group_id, system_role_id)
SELECT (link->>'groupId')::uuid, (link->>'roleId')::uuid
FROM ihp_service_only_roles_backup b, jsonb_array_elements(b.snapshot->'groups') link
WHERE EXISTS (SELECT 1 FROM ihp_system_roles WHERE id = (link->>'roleId')::uuid)
  AND EXISTS (SELECT 1 FROM ihp_rbac_groups WHERE id = (link->>'groupId')::uuid)
ON CONFLICT DO NOTHING;

INSERT INTO ihp_system_access_roles (system_id, role_name)
SELECT (link->>'systemId')::uuid, link->>'roleName'
FROM ihp_service_only_roles_backup b, jsonb_array_elements(b.snapshot->'accessRoles') link
WHERE EXISTS (SELECT 1 FROM ihp_systems WHERE id = (link->>'systemId')::uuid)
ON CONFLICT DO NOTHING;

DROP TABLE IF EXISTS ihp_service_only_roles_backup;

ALTER TABLE ihp_system_roles DROP COLUMN IF EXISTS service_only;
