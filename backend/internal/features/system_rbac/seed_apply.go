package system_rbac

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moh-sso-dashboard/internal/authz"
)

func ApplySeed(ctx context.Context, db *sql.DB, seed SeedFile) error {
	if db == nil {
		return fmt.Errorf("database is required")
	}
	if err := ValidateSeed(seed); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	permissionIDs := map[string]string{}
	for _, permission := range collectPermissions(seed) {
		if permission == "*" {
			continue
		}

		permissionID, err := upsertPermission(ctx, tx, permission)
		if err != nil {
			return err
		}
		permissionIDs[permission] = permissionID
	}

	for _, system := range seed.Systems {
		systemID, err := upsertSystem(ctx, tx, system)
		if err != nil {
			return err
		}

		for _, accessRole := range system.AccessRoles {
			if err := assignSystemAccessRole(ctx, tx, systemID, accessRole); err != nil {
				return err
			}
		}

		for _, role := range system.Roles {
			roleID, err := upsertSystemRole(ctx, tx, systemID, role)
			if err != nil {
				return err
			}

			for _, permission := range role.Permissions {
				if permission == "*" {
					for _, permissionID := range permissionIDs {
						if err := assignSystemRolePermission(ctx, tx, roleID, permissionID); err != nil {
							return err
						}
					}
					continue
				}
				if err := assignSystemRolePermission(ctx, tx, roleID, permissionIDs[permission]); err != nil {
					return err
				}
			}
		}
	}

	for _, role := range seed.RealmRoles {
		realmRole := strings.ToLower(strings.TrimSpace(role.Name))
		for _, permission := range role.Permissions {
			if permission == "*" {
				for _, permissionID := range permissionIDs {
					if err := assignRealmRolePermission(ctx, tx, realmRole, permissionID); err != nil {
						return err
					}
				}
				continue
			}
			if err := assignRealmRolePermission(ctx, tx, realmRole, permissionIDs[permission]); err != nil {
				return err
			}
		}
		for clientID, systemRoles := range role.SystemRoles {
			for _, systemRole := range systemRoles {
				if err := assignRealmRoleSystemRole(ctx, tx, realmRole, clientID, systemRole); err != nil {
					return err
				}
			}
		}
	}

	if err := applySeedGroups(ctx, tx, seed, permissionIDs); err != nil {
		return err
	}

	return tx.Commit()
}

func collectPermissions(seed SeedFile) []string {
	seen := map[string]bool{}
	values := make([]string, 0)

	add := func(permission string) {
		permission = strings.TrimSpace(permission)
		if permission == "" || seen[permission] {
			return
		}
		seen[permission] = true
		values = append(values, permission)
	}
	addAll := func() {
		for _, permission := range authz.AllPermissions {
			add(string(permission))
		}
	}

	for _, system := range seed.Systems {
		for _, role := range system.Roles {
			for _, permission := range role.Permissions {
				if permission == "*" {
					addAll()
					continue
				}
				add(permission)
			}
		}
	}
	for _, role := range seed.RealmRoles {
		for _, permission := range role.Permissions {
			if permission == "*" {
				addAll()
				continue
			}
			add(permission)
		}
	}
	for _, group := range FlattenGroups(seed.Groups) {
		for _, permission := range group.Permissions {
			if permission == "*" {
				addAll()
				continue
			}
			add(permission)
		}
	}

	return values
}

func upsertPermission(ctx context.Context, tx *sql.Tx, permission string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO ihp_permissions (permission_key)
		VALUES ($1)
		ON CONFLICT (permission_key) DO UPDATE SET updated_at = now()
		RETURNING id::text
	`, permission).Scan(&id)

	return id, err
}

func upsertSystem(ctx context.Context, tx *sql.Tx, system SeedSystem) (string, error) {
	system = NormalizeSystemBehavior(system)
	enabled := true
	if system.Enabled != nil {
		enabled = *system.Enabled
	}

	metadata, err := json.Marshal(map[string]string{
		"navigation":             strings.TrimSpace(system.Navigation),
		"authenticatedLaunchUrl": strings.TrimSpace(system.AuthenticatedLaunchURL),
	})
	if err != nil {
		return "", err
	}

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO ihp_systems (
			client_id, display_name, description, icon, launch_url, category, owner_team, owner_name,
			owner_email, support_url, documentation_url, environment, criticality,
			system_type, display_in_launcher, display_in_sidenav, launch_mode, enabled, sort_order, metadata, integration_type
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''),
			NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''),
			NULLIF($13, ''), $14, $15, $16, $17, $18, $19, $20, CASE WHEN $14 = 'platform' THEN 'hosted_module' ELSE 'external' END
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
			enabled = $18,
			sort_order = EXCLUDED.sort_order,
			metadata = CASE
				WHEN NULLIF(EXCLUDED.metadata->>'navigation', '') IS NULL
				 AND NULLIF(EXCLUDED.metadata->>'authenticatedLaunchUrl', '') IS NULL THEN ihp_systems.metadata
				ELSE COALESCE(ihp_systems.metadata, '{}'::jsonb) || EXCLUDED.metadata
			END,
			updated_at = now()
        WHERE NOT $21::boolean
		RETURNING id::text
	`, system.ClientID, system.DisplayName, system.Description, system.Icon, system.LaunchURL, system.Category, system.OwnerTeam, system.OwnerName, system.OwnerEmail, system.SupportURL, system.DocumentationURL, system.Environment, system.Criticality, system.SystemType, *system.DisplayInLauncher, *system.DisplayInSideNav, system.LaunchMode, enabled, system.SortOrder, metadata, IsStartupDiscovery(ctx)).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) && IsStartupDiscovery(ctx) {
		err = tx.QueryRowContext(ctx, `SELECT id::text FROM ihp_systems WHERE client_id=$1`, system.ClientID).Scan(&id)
	}
	return id, err
}

func upsertSystemRole(ctx context.Context, tx *sql.Tx, systemID string, role SeedRole) (string, error) {
	metadata, err := json.Marshal(map[string]any{})
	if err != nil {
		return "", err
	}

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO ihp_system_roles (
			system_id, role_name, display_name, description, enabled, metadata
		) VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, ''), TRUE, $5)
		ON CONFLICT (system_id, role_name) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			enabled = EXCLUDED.enabled,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING id::text
	`, systemID, strings.ToLower(strings.TrimSpace(role.Name)), role.DisplayName, role.Description, metadata).Scan(&id)

	return id, err
}

func assignSystemAccessRole(ctx context.Context, tx *sql.Tx, systemID string, roleName string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ihp_system_access_roles (system_id, role_name)
		VALUES ($1::uuid, $2)
		ON CONFLICT DO NOTHING
	`, systemID, strings.ToLower(strings.TrimSpace(roleName)))

	return err
}

func assignSystemRolePermission(ctx context.Context, tx *sql.Tx, roleID string, permissionID string) error {
	if permissionID == "" {
		return nil
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO ihp_system_role_permissions (system_role_id, permission_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT DO NOTHING
	`, roleID, permissionID)

	return err
}

func assignRealmRolePermission(ctx context.Context, tx *sql.Tx, realmRole string, permissionID string) error {
	if permissionID == "" {
		return nil
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO ihp_realm_role_permissions (realm_role, permission_id)
		VALUES ($1, $2::uuid)
		ON CONFLICT DO NOTHING
	`, realmRole, permissionID)

	return err
}

func assignRealmRoleSystemRole(
	ctx context.Context,
	tx *sql.Tx,
	realmRole string,
	clientID string,
	roleName string,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ihp_realm_role_system_roles (realm_role, system_role_id)
		SELECT $1, sr.id
		FROM ihp_system_roles sr
		JOIN ihp_systems s ON s.id = sr.system_id
		WHERE s.client_id = $2 AND sr.role_name = $3
		ON CONFLICT DO NOTHING
	`, strings.ToLower(strings.TrimSpace(realmRole)), strings.TrimSpace(clientID), strings.ToLower(strings.TrimSpace(roleName)))
	return err
}

func applySeedGroups(ctx context.Context, tx *sql.Tx, seed SeedFile, permissionIDs map[string]string) error {
	groupsByPath := map[string]string{}
	for _, group := range FlattenGroups(seed.Groups) {
		groupID, err := upsertGroup(ctx, tx, group)
		if err != nil {
			return err
		}
		groupPath := NormalizeGroupPath(group.Path)
		groupsByPath[groupPath] = groupID

		if err := replaceGroupAssignments(ctx, tx, groupID); err != nil {
			return err
		}
		for _, roleName := range group.RealmRoles {
			if err := assignGroupRealmRole(ctx, tx, groupID, roleName); err != nil {
				return err
			}
		}
		for clientID, roles := range group.SystemRoles {
			for _, roleName := range roles {
				if err := assignGroupSystemRole(ctx, tx, groupID, clientID, roleName); err != nil {
					return err
				}
			}
		}
		for _, permission := range group.Permissions {
			if permission == "*" {
				for _, permissionID := range permissionIDs {
					if err := assignGroupPermission(ctx, tx, groupID, permissionID); err != nil {
						return err
					}
				}
				continue
			}
			if err := assignGroupPermission(ctx, tx, groupID, permissionIDs[permission]); err != nil {
				return err
			}
		}
	}

	return replaceSeedGroupMembers(ctx, tx, groupsByPath, seed.GroupMemberships)
}

func upsertGroup(ctx context.Context, tx *sql.Tx, group SeedGroup) (string, error) {
	group.Name = strings.TrimSpace(group.Name)
	group.Path = NormalizeGroupPath(firstNonEmpty(group.Path, group.Name))
	metadata, err := json.Marshal(map[string]any{
		"type":       strings.TrimSpace(group.Type),
		"protected":  group.Protected,
		"attributes": group.Attributes,
	})
	if err != nil {
		return "", err
	}

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO ihp_rbac_groups (
			path, name, display_name, description, enabled, metadata
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), TRUE, $5
		)
		ON CONFLICT (path) DO UPDATE SET
			name = EXCLUDED.name,
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			enabled = EXCLUDED.enabled,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING id::text
	`, group.Path, group.Name, group.DisplayName, group.Description, metadata).Scan(&id)

	return id, err
}

func replaceGroupAssignments(ctx context.Context, tx *sql.Tx, groupID string) error {
	for _, statement := range []string{
		`DELETE FROM ihp_rbac_group_realm_roles WHERE group_id = $1::uuid`,
		`DELETE FROM ihp_rbac_group_system_roles WHERE group_id = $1::uuid`,
		`DELETE FROM ihp_rbac_group_permissions WHERE group_id = $1::uuid`,
	} {
		if _, err := tx.ExecContext(ctx, statement, groupID); err != nil {
			return err
		}
	}
	return nil
}

func replaceSeedGroupMembers(ctx context.Context, tx *sql.Tx, groupsByPath map[string]string, memberships []SeedGroupMembership) error {
	for _, groupID := range groupsByPath {
		if _, err := tx.ExecContext(ctx, `DELETE FROM ihp_rbac_group_members WHERE group_id = $1::uuid`, groupID); err != nil {
			return err
		}
	}
	for _, membership := range memberships {
		memberID := firstNonEmpty(membership.UserID, membership.Username, membership.Email)
		if memberID == "" {
			continue
		}
		for _, path := range membership.Groups {
			groupID := groupsByPath[NormalizeGroupPath(path)]
			if groupID == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO ihp_rbac_group_members (group_id, user_id, username, email)
				VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, ''))
				ON CONFLICT (group_id, user_id) DO UPDATE SET
					username = EXCLUDED.username,
					email = EXCLUDED.email
			`, groupID, memberID, membership.Username, membership.Email); err != nil {
				return err
			}
		}
	}
	return nil
}

func assignGroupPermission(ctx context.Context, tx *sql.Tx, groupID string, permissionID string) error {
	if permissionID == "" {
		return nil
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO ihp_rbac_group_permissions (group_id, permission_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT DO NOTHING
	`, groupID, permissionID)

	return err
}

func assignGroupRealmRole(ctx context.Context, tx *sql.Tx, groupID string, realmRole string) error {
	realmRole = strings.ToLower(strings.TrimSpace(realmRole))
	if realmRole == "" {
		return nil
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO ihp_rbac_group_realm_roles (group_id, realm_role)
		VALUES ($1::uuid, $2)
		ON CONFLICT DO NOTHING
	`, groupID, realmRole)

	return err
}

func assignGroupSystemRole(ctx context.Context, tx *sql.Tx, groupID string, clientID string, roleName string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ihp_rbac_group_system_roles (group_id, system_role_id)
		SELECT $1::uuid, sr.id
		FROM ihp_system_roles sr
		JOIN ihp_systems s ON s.id = sr.system_id
		WHERE s.client_id = $2 AND sr.role_name = $3
		ON CONFLICT DO NOTHING
	`, groupID, strings.TrimSpace(clientID), strings.ToLower(strings.TrimSpace(roleName)))
	return err
}
