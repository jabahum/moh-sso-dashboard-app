package rbac

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) ListSystems(ctx context.Context) ([]System, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, client_id, display_name, COALESCE(description, ''), COALESCE(icon, ''),
		       COALESCE(launch_url, ''), COALESCE(category, ''), COALESCE(owner_team, ''),
		       COALESCE(owner_name, ''), COALESCE(owner_email, ''), COALESCE(support_url, ''),
		       COALESCE(documentation_url, ''), COALESCE(environment, ''), COALESCE(criticality, ''),
		       COALESCE(metadata->>'navigation', ''),
		       COALESCE(metadata->>'authenticatedLaunchUrl', ''),
		       system_type, display_in_launcher, display_in_sidenav, launch_mode,
		       enabled, sort_order, lifecycle_status, integration_type, contract_version
		FROM ihp_systems
		ORDER BY sort_order, display_name, client_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	systems := make([]System, 0)
	for rows.Next() {
		var system System
		if err := rows.Scan(
			&system.ID,
			&system.ClientID,
			&system.DisplayName,
			&system.Description,
			&system.Icon,
			&system.LaunchURL,
			&system.Category,
			&system.OwnerTeam,
			&system.OwnerName,
			&system.OwnerEmail,
			&system.SupportURL,
			&system.DocumentationURL,
			&system.Environment,
			&system.Criticality,
			&system.Navigation,
			&system.AuthenticatedLaunchURL,
			&system.SystemType,
			&system.DisplayInLauncher,
			&system.DisplayInSideNav,
			&system.LaunchMode,
			&system.Enabled,
			&system.SortOrder,
			&system.LifecycleStatus, &system.IntegrationType, &system.ContractVersion,
		); err != nil {
			return nil, err
		}
		systems = append(systems, system)
	}
	return systems, rows.Err()
}

func (r *postgresRepository) GetSystem(ctx context.Context, clientID string) (SystemDetail, error) {
	record, err := loadSystem(ctx, r.db, clientID, false)
	if err != nil {
		return SystemDetail{}, err
	}
	detail := SystemDetail{System: record}

	accessRoles, err := r.listSystemAccessRoles(ctx, clientID)
	if err != nil {
		return SystemDetail{}, err
	}
	roles, err := r.ListSystemRoles(ctx, clientID)
	if err != nil {
		return SystemDetail{}, err
	}
	detail.AccessRoles = accessRoles
	detail.Roles = roles
	return detail, nil
}

func (r *postgresRepository) UpsertSystem(ctx context.Context, input UpsertSystemInput) (System, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return System{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('ihp.actor_user_id', $1, true)`, input.ActorUserID); err != nil {
		return System{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	metadata, err := json.Marshal(map[string]string{
		"navigation":             input.Navigation,
		"authenticatedLaunchUrl": input.AuthenticatedLaunchURL,
	})
	if err != nil {
		return System{}, err
	}

	var system System
	err = tx.QueryRowContext(ctx, `
		INSERT INTO ihp_systems (
			client_id, display_name, description, icon, launch_url, category, owner_team, owner_name,
			owner_email, support_url, documentation_url, environment, criticality,
			system_type, display_in_launcher, display_in_sidenav, launch_mode, enabled, sort_order, metadata, integration_type, contract_version
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''),
			NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''),
			NULLIF($13, ''), $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		ON CONFLICT (client_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			icon = EXCLUDED.icon,
			launch_url = EXCLUDED.launch_url,
			category = EXCLUDED.category,
			owner_team = EXCLUDED.owner_team,
			owner_name = EXCLUDED.owner_name,
			owner_email = EXCLUDED.owner_email,
			support_url = EXCLUDED.support_url,
			documentation_url = EXCLUDED.documentation_url,
			environment = EXCLUDED.environment,
			criticality = EXCLUDED.criticality,
			system_type = EXCLUDED.system_type,
			display_in_launcher = EXCLUDED.display_in_launcher,
			display_in_sidenav = EXCLUDED.display_in_sidenav,
			launch_mode = EXCLUDED.launch_mode,
			integration_type = EXCLUDED.integration_type,
			contract_version = EXCLUDED.contract_version,
			enabled = $18,
			sort_order = EXCLUDED.sort_order,
			metadata = CASE
				WHEN NULLIF(EXCLUDED.metadata->>'navigation', '') IS NULL
				 AND NULLIF(EXCLUDED.metadata->>'authenticatedLaunchUrl', '') IS NULL THEN ihp_systems.metadata
				ELSE COALESCE(ihp_systems.metadata, '{}'::jsonb) || EXCLUDED.metadata
			END,
			updated_at = now()
        WHERE NOT $23::boolean
		RETURNING id::text, client_id, display_name, COALESCE(description, ''), COALESCE(icon, ''),
		          COALESCE(launch_url, ''), COALESCE(category, ''), COALESCE(owner_team, ''),
		          COALESCE(owner_name, ''), COALESCE(owner_email, ''), COALESCE(support_url, ''),
		          COALESCE(documentation_url, ''), COALESCE(environment, ''), COALESCE(criticality, ''),
		          COALESCE(metadata->>'navigation', ''), COALESCE(metadata->>'authenticatedLaunchUrl', ''), system_type, display_in_launcher,
		          display_in_sidenav, launch_mode, enabled, sort_order, lifecycle_status, integration_type, contract_version
	`, input.ClientID, input.DisplayName, input.Description, input.Icon, input.LaunchURL, input.Category, input.OwnerTeam, input.OwnerName, input.OwnerEmail, input.SupportURL, input.DocumentationURL, input.Environment, input.Criticality, input.SystemType, *input.DisplayInLauncher, *input.DisplayInSideNav, input.LaunchMode, enabled, input.SortOrder, metadata, input.IntegrationType, input.ContractVersion, input.RegisterOnly).Scan(
		&system.ID,
		&system.ClientID,
		&system.DisplayName,
		&system.Description,
		&system.Icon,
		&system.LaunchURL,
		&system.Category,
		&system.OwnerTeam,
		&system.OwnerName,
		&system.OwnerEmail,
		&system.SupportURL,
		&system.DocumentationURL,
		&system.Environment,
		&system.Criticality,
		&system.Navigation,
		&system.AuthenticatedLaunchURL,
		&system.SystemType,
		&system.DisplayInLauncher,
		&system.DisplayInSideNav,
		&system.LaunchMode,
		&system.Enabled,
		&system.SortOrder,
		&system.LifecycleStatus, &system.IntegrationType, &system.ContractVersion,
	)
	if errors.Is(err, sql.ErrNoRows) && input.RegisterOnly {
		return System{}, ErrApplicationExists
	}
	if err != nil {
		return System{}, err
	}
	details, err := json.Marshal(system)
	if err != nil {
		return System{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_rbac_audit_events
 (actor_user_id, action, resource_type, resource_id, system_client_id, details)
 VALUES (NULLIF($1, ''), 'system.upserted', 'system', $2, $3, $4::jsonb)`, input.ActorUserID, system.ID, system.ClientID, string(details))
	if err != nil {
		return System{}, err
	}
	if err := tx.Commit(); err != nil {
		return System{}, err
	}
	return system, nil
}

func (r *postgresRepository) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id::text, p.permission_key, COALESCE(p.display_name, ''), COALESCE(p.description, ''),
		       COALESCE(p.category, ''), COALESCE(p.status, 'active'),
		       COUNT(DISTINCT srp.system_role_id)::int,
		       COUNT(DISTINCT rrp.realm_role)::int
		FROM ihp_permissions p
		LEFT JOIN ihp_system_role_permissions srp ON srp.permission_id = p.id
		LEFT JOIN ihp_realm_role_permissions rrp ON rrp.permission_id = p.id
		GROUP BY p.id, p.permission_key, p.display_name, p.description, p.category, p.status
		ORDER BY p.category NULLS LAST, p.permission_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]Permission, 0)
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func (r *postgresRepository) ListSystemRoles(ctx context.Context, clientID string) ([]SystemRole, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sr.id::text, sr.system_id::text, s.client_id, sr.role_name,
		       COALESCE(sr.display_name, ''), COALESCE(sr.description, ''), sr.enabled
		FROM ihp_system_roles sr
		JOIN ihp_systems s ON s.id = sr.system_id
		WHERE s.client_id = $1
		ORDER BY sr.role_name
	`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]SystemRole, 0)
	for rows.Next() {
		var role SystemRole
		if err := rows.Scan(
			&role.ID,
			&role.SystemID,
			&role.ClientID,
			&role.Name,
			&role.DisplayName,
			&role.Description,
			&role.Enabled,
		); err != nil {
			return nil, err
		}
		permissions, err := r.listPermissionsForSystemRole(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		role.Permissions = permissions
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *postgresRepository) GetSystemRole(ctx context.Context, roleID string) (SystemRole, error) {
	var role SystemRole
	err := r.db.QueryRowContext(ctx, `
		SELECT sr.id::text, sr.system_id::text, s.client_id, sr.role_name,
		       COALESCE(sr.display_name, ''), COALESCE(sr.description, ''), sr.enabled
		FROM ihp_system_roles sr
		JOIN ihp_systems s ON s.id = sr.system_id
		WHERE sr.id = $1::uuid
	`, roleID).Scan(
		&role.ID,
		&role.SystemID,
		&role.ClientID,
		&role.Name,
		&role.DisplayName,
		&role.Description,
		&role.Enabled,
	)
	if err != nil {
		return SystemRole{}, err
	}
	role.Permissions, err = r.listPermissionsForSystemRole(ctx, role.ID)
	return role, err
}

func (r *postgresRepository) CreateSystemRole(ctx context.Context, clientID string, input RoleInput) (SystemRole, error) {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	var role SystemRole
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ihp_system_roles (system_id, role_name, display_name, description, enabled)
		SELECT id, $2, NULLIF($3, ''), NULLIF($4, ''), $5
		FROM ihp_systems
		WHERE client_id = $1
		RETURNING id::text, system_id::text, $1::text, role_name, COALESCE(display_name, ''), COALESCE(description, ''), enabled
	`, clientID, input.Name, input.DisplayName, input.Description, enabled).Scan(
		&role.ID,
		&role.SystemID,
		&role.ClientID,
		&role.Name,
		&role.DisplayName,
		&role.Description,
		&role.Enabled,
	)
	return role, err
}

func (r *postgresRepository) UpsertSystemRole(ctx context.Context, clientID string, input RoleInput) (SystemRole, error) {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	var role SystemRole
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ihp_system_roles (system_id, role_name, display_name, description, enabled)
		SELECT id, $2, NULLIF($3, ''), NULLIF($4, ''), $5
		FROM ihp_systems
		WHERE client_id = $1
		ON CONFLICT (system_id, role_name) DO UPDATE SET
			display_name = COALESCE(EXCLUDED.display_name, ihp_system_roles.display_name),
			description = COALESCE(EXCLUDED.description, ihp_system_roles.description),
			enabled = EXCLUDED.enabled,
			updated_at = now()
		RETURNING id::text, system_id::text, $1::text, role_name, COALESCE(display_name, ''), COALESCE(description, ''), enabled
	`, clientID, input.Name, input.DisplayName, input.Description, enabled).Scan(
		&role.ID,
		&role.SystemID,
		&role.ClientID,
		&role.Name,
		&role.DisplayName,
		&role.Description,
		&role.Enabled,
	)
	if err != nil {
		return SystemRole{}, err
	}
	role.Permissions, err = r.listPermissionsForSystemRole(ctx, role.ID)
	return role, err
}

func (r *postgresRepository) UpdateSystemRole(ctx context.Context, roleID string, input RoleInput) (SystemRole, error) {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	var role SystemRole
	err := r.db.QueryRowContext(ctx, `
		UPDATE ihp_system_roles sr
		SET role_name = $2,
		    display_name = NULLIF($3, ''),
		    description = NULLIF($4, ''),
		    enabled = $5,
		    updated_at = now()
		FROM ihp_systems s
		WHERE sr.system_id = s.id AND sr.id = $1::uuid
		RETURNING sr.id::text, sr.system_id::text, s.client_id, sr.role_name,
		          COALESCE(sr.display_name, ''), COALESCE(sr.description, ''), sr.enabled
	`, roleID, input.Name, input.DisplayName, input.Description, enabled).Scan(
		&role.ID,
		&role.SystemID,
		&role.ClientID,
		&role.Name,
		&role.DisplayName,
		&role.Description,
		&role.Enabled,
	)
	if err != nil {
		return SystemRole{}, err
	}
	role.Permissions, err = r.listPermissionsForSystemRole(ctx, role.ID)
	return role, err
}

func (r *postgresRepository) DeleteSystemRole(ctx context.Context, roleID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM ihp_system_roles WHERE id = $1::uuid`, roleID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (r *postgresRepository) AssignSystemRolePermission(ctx context.Context, roleID string, permissionKey string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ihp_system_role_permissions (system_role_id, permission_id)
		SELECT $1::uuid, id
		FROM ihp_permissions
		WHERE permission_key = $2
		ON CONFLICT DO NOTHING
	`, roleID, permissionKey)
	return err
}

func (r *postgresRepository) RemoveSystemRolePermission(ctx context.Context, roleID string, permissionKey string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM ihp_system_role_permissions srp
		USING ihp_permissions p
		WHERE srp.permission_id = p.id
		  AND srp.system_role_id = $1::uuid
		  AND p.permission_key = $2
	`, roleID, permissionKey)
	return err
}

func (r *postgresRepository) ListRealmRolePermissions(ctx context.Context) ([]RealmRolePermissionGroup, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT realm_role
		FROM ihp_realm_role_permissions
		GROUP BY realm_role
		ORDER BY realm_role
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]RealmRolePermissionGroup, 0)
	for rows.Next() {
		var group RealmRolePermissionGroup
		if err := rows.Scan(&group.RealmRole); err != nil {
			return nil, err
		}
		permissions, err := r.listPermissionsForRealmRole(ctx, group.RealmRole)
		if err != nil {
			return nil, err
		}
		group.Permissions = permissions
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *postgresRepository) AssignRealmRolePermission(ctx context.Context, realmRole string, permissionKey string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ihp_realm_role_permissions (realm_role, permission_id)
		SELECT $1, id
		FROM ihp_permissions
		WHERE permission_key = $2
		ON CONFLICT DO NOTHING
	`, realmRole, permissionKey)
	return err
}

func (r *postgresRepository) RemoveRealmRolePermission(ctx context.Context, realmRole string, permissionKey string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM ihp_realm_role_permissions rrp
		USING ihp_permissions p
		WHERE rrp.permission_id = p.id
		  AND rrp.realm_role = $1
		  AND p.permission_key = $2
	`, realmRole, permissionKey)
	return err
}

func (r *postgresRepository) ListRealmRoleSystemRoles(ctx context.Context) ([]RealmRoleSystemRole, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT rrsr.realm_role, s.client_id, sr.role_name
		FROM ihp_realm_role_system_roles rrsr
		JOIN ihp_system_roles sr ON sr.id = rrsr.system_role_id
		JOIN ihp_systems s ON s.id = sr.system_id
		WHERE s.enabled = TRUE AND sr.enabled = TRUE
		ORDER BY rrsr.realm_role, s.client_id, sr.role_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]RealmRoleSystemRole, 0)
	for rows.Next() {
		var value RealmRoleSystemRole
		if err := rows.Scan(&value.RealmRole, &value.ClientID, &value.RoleName); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *postgresRepository) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id::text, COALESCE(g.keycloak_group_id, ''), g.path, g.name,
		       COALESCE(g.display_name, ''), COALESCE(g.description, ''), g.enabled,
		       COUNT(DISTINCT gm.user_id)::int
		FROM ihp_rbac_groups g
		LEFT JOIN ihp_rbac_group_members gm ON gm.group_id = g.id
		GROUP BY g.id, g.keycloak_group_id, g.path, g.name, g.display_name, g.description, g.enabled
		ORDER BY g.path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]Group, 0)
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		if err := r.hydrateGroup(ctx, &group); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *postgresRepository) GetGroup(ctx context.Context, groupID string) (Group, error) {
	var group Group
	err := r.db.QueryRowContext(ctx, `
		SELECT g.id::text, COALESCE(g.keycloak_group_id, ''), g.path, g.name,
		       COALESCE(g.display_name, ''), COALESCE(g.description, ''), g.enabled,
		       COUNT(DISTINCT gm.user_id)::int
		FROM ihp_rbac_groups g
		LEFT JOIN ihp_rbac_group_members gm ON gm.group_id = g.id
		WHERE g.id = $1::uuid
		GROUP BY g.id, g.keycloak_group_id, g.path, g.name, g.display_name, g.description, g.enabled
	`, groupID).Scan(
		&group.ID,
		&group.KeycloakGroupID,
		&group.Path,
		&group.Name,
		&group.DisplayName,
		&group.Description,
		&group.Enabled,
		&group.MemberCount,
	)
	if err != nil {
		return Group{}, err
	}
	if err := r.hydrateGroup(ctx, &group); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (r *postgresRepository) ListGroupsForUser(ctx context.Context, userID string, username string, email string) ([]Group, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT g.id::text, COALESCE(g.keycloak_group_id, ''), g.path, g.name,
		       COALESCE(g.display_name, ''), COALESCE(g.description, ''), g.enabled,
		       COUNT(DISTINCT all_members.user_id)::int
		FROM ihp_rbac_groups g
		JOIN ihp_rbac_group_members gm ON gm.group_id = g.id
		LEFT JOIN ihp_rbac_group_members all_members ON all_members.group_id = g.id
		WHERE g.enabled = TRUE
		  AND (
			(NULLIF($1, '') IS NOT NULL AND gm.user_id = $1)
			OR (NULLIF($2, '') IS NOT NULL AND lower(gm.username) = lower($2))
			OR (NULLIF($3, '') IS NOT NULL AND lower(gm.email) = lower($3))
		  )
		GROUP BY g.id, g.keycloak_group_id, g.path, g.name, g.display_name, g.description, g.enabled
		ORDER BY g.path
	`, userID, username, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]Group, 0)
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		if err := r.hydrateGroup(ctx, &group); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *postgresRepository) UpsertGroup(ctx context.Context, input GroupInput) (Group, error) {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	var group Group
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ihp_rbac_groups (
			keycloak_group_id, path, name, display_name, description, enabled
		) VALUES (
			NULLIF($1, ''), $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6
		)
		ON CONFLICT (path) DO UPDATE SET
			keycloak_group_id = COALESCE(EXCLUDED.keycloak_group_id, ihp_rbac_groups.keycloak_group_id),
			name = EXCLUDED.name,
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			enabled = EXCLUDED.enabled,
			updated_at = now()
		RETURNING id::text, COALESCE(keycloak_group_id, ''), path, name,
		          COALESCE(display_name, ''), COALESCE(description, ''), enabled, 0::int
	`, input.KeycloakGroupID, input.Path, input.Name, input.DisplayName, input.Description, enabled).Scan(
		&group.ID,
		&group.KeycloakGroupID,
		&group.Path,
		&group.Name,
		&group.DisplayName,
		&group.Description,
		&group.Enabled,
		&group.MemberCount,
	)
	if err != nil {
		return Group{}, err
	}
	return r.GetGroup(ctx, group.ID)
}

func (r *postgresRepository) ListGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, COALESCE(username, ''), COALESCE(email, '')
		FROM ihp_rbac_group_members
		WHERE group_id = $1::uuid
		ORDER BY COALESCE(username, email, user_id)
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]GroupMember, 0)
	for rows.Next() {
		var member GroupMember
		if err := rows.Scan(&member.UserID, &member.Username, &member.Email); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *postgresRepository) ReplaceGroupMembers(ctx context.Context, groupID string, members []GroupMember) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM ihp_rbac_group_members WHERE group_id = $1::uuid`, groupID); err != nil {
		return err
	}
	for _, member := range members {
		if strings.TrimSpace(member.UserID) == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO ihp_rbac_group_members (group_id, user_id, username, email)
			VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, ''))
			ON CONFLICT (group_id, user_id) DO UPDATE SET
				username = EXCLUDED.username,
				email = EXCLUDED.email
		`, groupID, member.UserID, member.Username, member.Email); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *postgresRepository) AssignGroupPermission(ctx context.Context, groupID string, permissionKey string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ihp_rbac_group_permissions (group_id, permission_id)
		SELECT $1::uuid, id
		FROM ihp_permissions
		WHERE permission_key = $2
		ON CONFLICT DO NOTHING
	`, groupID, permissionKey)
	return err
}

func (r *postgresRepository) RemoveGroupPermission(ctx context.Context, groupID string, permissionKey string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM ihp_rbac_group_permissions gp
		USING ihp_permissions p
		WHERE gp.permission_id = p.id
		  AND gp.group_id = $1::uuid
		  AND p.permission_key = $2
	`, groupID, permissionKey)
	return err
}

func (r *postgresRepository) AssignGroupRealmRole(ctx context.Context, groupID string, realmRole string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ihp_rbac_group_realm_roles (group_id, realm_role)
		VALUES ($1::uuid, $2)
		ON CONFLICT DO NOTHING
	`, groupID, realmRole)
	return err
}

func (r *postgresRepository) RemoveGroupRealmRole(ctx context.Context, groupID string, realmRole string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM ihp_rbac_group_realm_roles
		WHERE group_id = $1::uuid AND realm_role = $2
	`, groupID, realmRole)
	return err
}

func (r *postgresRepository) AssignGroupSystemRole(ctx context.Context, groupID string, clientID string, roleName string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ihp_rbac_group_system_roles (group_id, system_role_id)
		SELECT $1::uuid, sr.id
		FROM ihp_system_roles sr
		JOIN ihp_systems s ON s.id = sr.system_id
		WHERE s.client_id = $2 AND sr.role_name = $3
		ON CONFLICT DO NOTHING
	`, groupID, clientID, roleName)
	return err
}

func (r *postgresRepository) RemoveGroupSystemRole(ctx context.Context, groupID string, clientID string, roleName string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM ihp_rbac_group_system_roles gr
		USING ihp_system_roles sr, ihp_systems s
		WHERE gr.system_role_id = sr.id
		  AND sr.system_id = s.id
		  AND gr.group_id = $1::uuid
		  AND s.client_id = $2
		  AND sr.role_name = $3
	`, groupID, clientID, roleName)
	return err
}

func (r *postgresRepository) AddSystemAccessRole(ctx context.Context, clientID string, roleName string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ihp_system_access_roles (system_id, role_name)
		SELECT id, $2
		FROM ihp_systems
		WHERE client_id = $1
		ON CONFLICT DO NOTHING
	`, clientID, roleName)
	return err
}

func (r *postgresRepository) RemoveSystemAccessRole(ctx context.Context, clientID string, roleName string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM ihp_system_access_roles sar
		USING ihp_systems s
		WHERE sar.system_id = s.id
		  AND s.client_id = $1
		  AND sar.role_name = $2
	`, clientID, roleName)
	return err
}

func (r *postgresRepository) CountSystemAccessRoles(ctx context.Context, clientID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM ihp_system_access_roles sar
		JOIN ihp_systems s ON s.id = sar.system_id
		WHERE s.client_id = $1
	`, clientID).Scan(&count)
	return count, err
}

func (r *postgresRepository) PermissionExists(ctx context.Context, permissionKey string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM ihp_permissions WHERE permission_key = $1)`, permissionKey).Scan(&exists)
	return exists, err
}

func (r *postgresRepository) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, COALESCE(actor_user_id, ''), action, resource_type, COALESCE(resource_id, ''),
		       COALESCE(system_client_id, ''), COALESCE(role_name, ''), COALESCE(permission_key, ''),
		       details::text, created_at::text
		FROM ihp_rbac_audit_events
		WHERE (NULLIF($2, '') IS NULL OR actor_user_id = $2)
		  AND (NULLIF($3, '') IS NULL OR system_client_id = $3)
		  AND (NULLIF($4, '') IS NULL OR role_name = $4)
		  AND (NULLIF($5, '') IS NULL OR permission_key = $5)
		  AND (NULLIF($6, '') IS NULL OR action ILIKE '%' || $6 || '%')
		  AND (NULLIF($7, '')::timestamptz IS NULL OR created_at >= NULLIF($7, '')::timestamptz)
		  AND (NULLIF($8, '')::timestamptz IS NULL OR created_at <= NULLIF($8, '')::timestamptz)
		ORDER BY created_at DESC
		LIMIT $1
	`, filter.Limit, filter.ActorUserID, filter.SystemClientID, filter.RoleName, filter.PermissionKey, filter.Action, filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]AuditEvent, 0)
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(
			&event.ID,
			&event.ActorUserID,
			&event.Action,
			&event.ResourceType,
			&event.ResourceID,
			&event.SystemClientID,
			&event.RoleName,
			&event.PermissionKey,
			&event.Details,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *postgresRepository) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	details := string(event.Details)
	if strings.TrimSpace(details) == "" {
		details = "{}"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ihp_rbac_audit_events (
			actor_user_id, action, resource_type, resource_id, system_client_id, role_name, permission_key, details
		) VALUES (NULLIF($1, ''), $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), $8::jsonb)
	`, event.ActorUserID, event.Action, event.ResourceType, event.ResourceID, event.SystemClientID, event.RoleName, event.PermissionKey, details)
	return err
}

func (r *postgresRepository) CreateAccessRequest(ctx context.Context, input AccessRequestInput) (AccessRequest, error) {
	var request AccessRequest
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ihp_access_requests (
			user_id, username, email, system_client_id, requested_role, reason, requested_by
		) VALUES (NULLIF($1, ''), NULLIF($2, ''), NULLIF($3, ''), $4, $5, NULLIF($6, ''), NULLIF($1, ''))
		RETURNING id::text, COALESCE(user_id, ''), COALESCE(username, ''), COALESCE(email, ''), system_client_id,
		          requested_role, COALESCE(reason, ''), status, COALESCE(requested_by, ''), COALESCE(reviewed_by, ''),
		          COALESCE(reviewed_at::text, ''), COALESCE(decision_note, ''), created_at::text, updated_at::text
	`, input.UserID, input.Username, input.Email, input.SystemClientID, input.RequestedRole, input.Reason).Scan(
		&request.ID,
		&request.UserID,
		&request.Username,
		&request.Email,
		&request.SystemClientID,
		&request.RequestedRole,
		&request.Reason,
		&request.Status,
		&request.RequestedBy,
		&request.ReviewedBy,
		&request.ReviewedAt,
		&request.DecisionNote,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	return request, err
}

func (r *postgresRepository) ListAccessRequests(ctx context.Context) ([]AccessRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, COALESCE(user_id, ''), COALESCE(username, ''), COALESCE(email, ''), system_client_id,
		       requested_role, COALESCE(reason, ''), status, COALESCE(requested_by, ''), COALESCE(reviewed_by, ''),
		       COALESCE(reviewed_at::text, ''), COALESCE(decision_note, ''), created_at::text, updated_at::text
		FROM ihp_access_requests
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]AccessRequest, 0)
	for rows.Next() {
		request, err := scanAccessRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *postgresRepository) UpdateAccessRequestStatus(ctx context.Context, id string, status string, note string, reviewer string) (AccessRequest, error) {
	var request AccessRequest
	err := r.db.QueryRowContext(ctx, `
		UPDATE ihp_access_requests
		SET status = $2,
		    decision_note = NULLIF($3, ''),
		    reviewed_by = NULLIF($4, ''),
		    reviewed_at = now(),
		    updated_at = now()
		WHERE id = $1::uuid
		RETURNING id::text, COALESCE(user_id, ''), COALESCE(username, ''), COALESCE(email, ''), system_client_id,
		          requested_role, COALESCE(reason, ''), status, COALESCE(requested_by, ''), COALESCE(reviewed_by, ''),
		          COALESCE(reviewed_at::text, ''), COALESCE(decision_note, ''), created_at::text, updated_at::text
	`, id, status, note, reviewer).Scan(
		&request.ID,
		&request.UserID,
		&request.Username,
		&request.Email,
		&request.SystemClientID,
		&request.RequestedRole,
		&request.Reason,
		&request.Status,
		&request.RequestedBy,
		&request.ReviewedBy,
		&request.ReviewedAt,
		&request.DecisionNote,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	return request, err
}

func (r *postgresRepository) CreateChangeRequest(ctx context.Context, input ChangeRequestInput) (ChangeRequest, error) {
	payload := string(input.Payload)
	if strings.TrimSpace(payload) == "" {
		payload = "{}"
	}
	var request ChangeRequest
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ihp_rbac_change_requests (
			action, resource_type, resource_id, payload, risk_level, reason
		) VALUES ($1, $2, NULLIF($3, ''), $4::jsonb, COALESCE(NULLIF($5, ''), 'medium'), NULLIF($6, ''))
		RETURNING id::text, COALESCE(requested_by, ''), COALESCE(reviewed_by, ''), status, action, resource_type,
		          COALESCE(resource_id, ''), payload::text, risk_level, COALESCE(reason, ''), COALESCE(decision_note, ''),
		          COALESCE(reviewed_at::text, ''), created_at::text, updated_at::text
	`, input.Action, input.ResourceType, input.ResourceID, payload, input.RiskLevel, input.Reason).Scan(
		&request.ID,
		&request.RequestedBy,
		&request.ReviewedBy,
		&request.Status,
		&request.Action,
		&request.ResourceType,
		&request.ResourceID,
		&request.Payload,
		&request.RiskLevel,
		&request.Reason,
		&request.DecisionNote,
		&request.ReviewedAt,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	return request, err
}

func (r *postgresRepository) ListChangeRequests(ctx context.Context) ([]ChangeRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, COALESCE(requested_by, ''), COALESCE(reviewed_by, ''), status, action, resource_type,
		       COALESCE(resource_id, ''), payload::text, risk_level, COALESCE(reason, ''), COALESCE(decision_note, ''),
		       COALESCE(reviewed_at::text, ''), created_at::text, updated_at::text
		FROM ihp_rbac_change_requests
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]ChangeRequest, 0)
	for rows.Next() {
		request, err := scanChangeRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *postgresRepository) UpdateChangeRequestStatus(ctx context.Context, id string, status string, note string, reviewer string) (ChangeRequest, error) {
	var request ChangeRequest
	err := r.db.QueryRowContext(ctx, `
		UPDATE ihp_rbac_change_requests
		SET status = $2,
		    decision_note = NULLIF($3, ''),
		    reviewed_by = NULLIF($4, ''),
		    reviewed_at = now(),
		    updated_at = now()
		WHERE id = $1::uuid
		RETURNING id::text, COALESCE(requested_by, ''), COALESCE(reviewed_by, ''), status, action, resource_type,
		          COALESCE(resource_id, ''), payload::text, risk_level, COALESCE(reason, ''), COALESCE(decision_note, ''),
		          COALESCE(reviewed_at::text, ''), created_at::text, updated_at::text
	`, id, status, note, reviewer).Scan(
		&request.ID,
		&request.RequestedBy,
		&request.ReviewedBy,
		&request.Status,
		&request.Action,
		&request.ResourceType,
		&request.ResourceID,
		&request.Payload,
		&request.RiskLevel,
		&request.Reason,
		&request.DecisionNote,
		&request.ReviewedAt,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	return request, err
}

func (r *postgresRepository) UpdatePermissionMetadata(ctx context.Context, permissionKey string, input PermissionMetadataInput) (Permission, error) {
	var permission Permission
	err := r.db.QueryRowContext(ctx, `
		UPDATE ihp_permissions
		SET display_name = NULLIF($2, ''),
		    description = NULLIF($3, ''),
		    category = NULLIF($4, ''),
		    status = COALESCE(NULLIF($5, ''), 'active'),
		    updated_at = now()
		WHERE permission_key = $1
		RETURNING id::text, permission_key, COALESCE(display_name, ''), COALESCE(description, ''), COALESCE(category, ''), COALESCE(status, 'active'),
		          (SELECT COUNT(DISTINCT system_role_id)::int FROM ihp_system_role_permissions WHERE permission_id = ihp_permissions.id),
		          (SELECT COUNT(DISTINCT realm_role)::int FROM ihp_realm_role_permissions WHERE permission_id = ihp_permissions.id)
	`, permissionKey, input.DisplayName, input.Description, input.Category, input.Status).Scan(
		&permission.ID,
		&permission.Key,
		&permission.DisplayName,
		&permission.Description,
		&permission.Category,
		&permission.Status,
		&permission.SystemRoleUsageCount,
		&permission.RealmRoleUsageCount,
	)
	return permission, err
}

func (r *postgresRepository) listSystemAccessRoles(ctx context.Context, clientID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sar.role_name
		FROM ihp_system_access_roles sar
		JOIN ihp_systems s ON s.id = sar.system_id
		WHERE s.client_id = $1
		ORDER BY sar.role_name
	`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *postgresRepository) listPermissionsForSystemRole(ctx context.Context, roleID string) ([]Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id::text, p.permission_key, COALESCE(p.display_name, ''), COALESCE(p.description, ''), COALESCE(p.category, ''), COALESCE(p.status, 'active'),
		       COUNT(DISTINCT srp_all.system_role_id)::int,
		       COUNT(DISTINCT rrp.realm_role)::int
		FROM ihp_system_role_permissions srp
		JOIN ihp_permissions p ON p.id = srp.permission_id
		LEFT JOIN ihp_system_role_permissions srp_all ON srp_all.permission_id = p.id
		LEFT JOIN ihp_realm_role_permissions rrp ON rrp.permission_id = p.id
		WHERE srp.system_role_id = $1::uuid
		GROUP BY p.id, p.permission_key, p.display_name, p.description, p.category, p.status
		ORDER BY p.permission_key
	`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPermissions(rows)
}

func (r *postgresRepository) listPermissionsForRealmRole(ctx context.Context, realmRole string) ([]Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id::text, p.permission_key, COALESCE(p.display_name, ''), COALESCE(p.description, ''), COALESCE(p.category, ''), COALESCE(p.status, 'active'),
		       COUNT(DISTINCT srp.system_role_id)::int,
		       COUNT(DISTINCT rrp_all.realm_role)::int
		FROM ihp_realm_role_permissions rrp
		JOIN ihp_permissions p ON p.id = rrp.permission_id
		LEFT JOIN ihp_system_role_permissions srp ON srp.permission_id = p.id
		LEFT JOIN ihp_realm_role_permissions rrp_all ON rrp_all.permission_id = p.id
		WHERE rrp.realm_role = $1
		GROUP BY p.id, p.permission_key, p.display_name, p.description, p.category, p.status
		ORDER BY p.permission_key
	`, realmRole)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPermissions(rows)
}

func (r *postgresRepository) hydrateGroup(ctx context.Context, group *Group) error {
	if group == nil || strings.TrimSpace(group.ID) == "" {
		return nil
	}
	realmRoles, err := r.listRealmRolesForGroup(ctx, group.ID)
	if err != nil {
		return err
	}
	systemRoles, err := r.listSystemRolesForGroup(ctx, group.ID)
	if err != nil {
		return err
	}
	permissions, err := r.listPermissionsForGroup(ctx, group.ID)
	if err != nil {
		return err
	}
	group.RealmRoles = realmRoles
	group.SystemRoles = systemRoles
	group.Permissions = permissions
	return nil
}

func (r *postgresRepository) listRealmRolesForGroup(ctx context.Context, groupID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT realm_role
		FROM ihp_rbac_group_realm_roles
		WHERE group_id = $1::uuid
		ORDER BY realm_role
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *postgresRepository) listSystemRolesForGroup(ctx context.Context, groupID string) ([]GroupSystemRole, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sr.id::text, s.client_id, sr.role_name, COALESCE(sr.display_name, '')
		FROM ihp_rbac_group_system_roles gr
		JOIN ihp_system_roles sr ON sr.id = gr.system_role_id
		JOIN ihp_systems s ON s.id = sr.system_id
		WHERE gr.group_id = $1::uuid
		ORDER BY s.client_id, sr.role_name
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]GroupSystemRole, 0)
	for rows.Next() {
		var role GroupSystemRole
		if err := rows.Scan(&role.RoleID, &role.ClientID, &role.RoleName, &role.DisplayName); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *postgresRepository) listPermissionsForGroup(ctx context.Context, groupID string) ([]Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id::text, p.permission_key, COALESCE(p.display_name, ''), COALESCE(p.description, ''), COALESCE(p.category, ''), COALESCE(p.status, 'active'),
		       COUNT(DISTINCT srp.system_role_id)::int,
		       COUNT(DISTINCT rrp.realm_role)::int
		FROM ihp_rbac_group_permissions gp
		JOIN ihp_permissions p ON p.id = gp.permission_id
		LEFT JOIN ihp_system_role_permissions srp ON srp.permission_id = p.id
		LEFT JOIN ihp_realm_role_permissions rrp ON rrp.permission_id = p.id
		WHERE gp.group_id = $1::uuid
		GROUP BY p.id, p.permission_key, p.display_name, p.description, p.category, p.status
		ORDER BY p.permission_key
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPermissions(rows)
}

type permissionScanner interface {
	Scan(dest ...any) error
}

type accessRequestScanner interface {
	Scan(dest ...any) error
}

type changeRequestScanner interface {
	Scan(dest ...any) error
}

func scanPermission(row permissionScanner) (Permission, error) {
	var permission Permission
	err := row.Scan(
		&permission.ID,
		&permission.Key,
		&permission.DisplayName,
		&permission.Description,
		&permission.Category,
		&permission.Status,
		&permission.SystemRoleUsageCount,
		&permission.RealmRoleUsageCount,
	)
	return permission, err
}

func scanGroup(row permissionScanner) (Group, error) {
	var group Group
	err := row.Scan(
		&group.ID,
		&group.KeycloakGroupID,
		&group.Path,
		&group.Name,
		&group.DisplayName,
		&group.Description,
		&group.Enabled,
		&group.MemberCount,
	)
	return group, err
}

func scanPermissions(rows *sql.Rows) ([]Permission, error) {
	permissions := make([]Permission, 0)
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func scanAccessRequest(row accessRequestScanner) (AccessRequest, error) {
	var request AccessRequest
	err := row.Scan(
		&request.ID,
		&request.UserID,
		&request.Username,
		&request.Email,
		&request.SystemClientID,
		&request.RequestedRole,
		&request.Reason,
		&request.Status,
		&request.RequestedBy,
		&request.ReviewedBy,
		&request.ReviewedAt,
		&request.DecisionNote,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	return request, err
}

func scanChangeRequest(row changeRequestScanner) (ChangeRequest, error) {
	var request ChangeRequest
	err := row.Scan(
		&request.ID,
		&request.RequestedBy,
		&request.ReviewedBy,
		&request.Status,
		&request.Action,
		&request.ResourceType,
		&request.ResourceID,
		&request.Payload,
		&request.RiskLevel,
		&request.Reason,
		&request.DecisionNote,
		&request.ReviewedAt,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	return request, err
}

func normalizeRoleName(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func notFoundError(entity string) error {
	return fmt.Errorf("%s not found: %w", entity, sql.ErrNoRows)
}

type systemRowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadSystem(ctx context.Context, db systemRowQuerier, clientID string, lock bool) (System, error) {
	query := `
		SELECT id::text, client_id, display_name, COALESCE(description, ''), COALESCE(icon, ''),
		       COALESCE(launch_url, ''), COALESCE(category, ''), COALESCE(owner_team, ''),
		       COALESCE(owner_name, ''), COALESCE(owner_email, ''), COALESCE(support_url, ''),
		       COALESCE(documentation_url, ''), COALESCE(environment, ''), COALESCE(criticality, ''),
		       COALESCE(metadata->>'navigation', ''),
		       COALESCE(metadata->>'authenticatedLaunchUrl', ''),
		       system_type, display_in_launcher, display_in_sidenav, launch_mode,
		       enabled, sort_order, lifecycle_status, integration_type, contract_version
		FROM ihp_systems
		WHERE client_id = $1
	`
	if lock {
		query += " FOR UPDATE"
	}
	var system System
	err := db.QueryRowContext(ctx, query, clientID).Scan(
		&system.ID,
		&system.ClientID,
		&system.DisplayName,
		&system.Description,
		&system.Icon,
		&system.LaunchURL,
		&system.Category,
		&system.OwnerTeam,
		&system.OwnerName,
		&system.OwnerEmail,
		&system.SupportURL,
		&system.DocumentationURL,
		&system.Environment,
		&system.Criticality,
		&system.Navigation,
		&system.AuthenticatedLaunchURL,
		&system.SystemType,
		&system.DisplayInLauncher,
		&system.DisplayInSideNav,
		&system.LaunchMode,
		&system.Enabled,
		&system.SortOrder,
		&system.LifecycleStatus, &system.IntegrationType, &system.ContractVersion,
	)
	return system, err
}
