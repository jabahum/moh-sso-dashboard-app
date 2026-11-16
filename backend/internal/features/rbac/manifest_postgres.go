package rbac

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/lib/pq"
)

var ErrManifestReview = errors.New("manifest preview is stale or missing")

type ManifestPreview struct {
	Manifest      ApplicationManifest `json:"manifest"`
	Operation     string              `json:"operation"`
	Before        *SystemDetail       `json:"before,omitempty"`
	After         UpsertSystemInput   `json:"after"`
	RolesToCreate []string            `json:"rolesToCreate"`
	AccessRoles   []string            `json:"accessRoles"`
	Warnings      []string            `json:"warnings"`
	ReviewToken   string              `json:"reviewToken"`
}

type manifestRepository interface {
	HasApplicationManifest(context.Context, string) (bool, error)
	ReviewManifest(context.Context, ApplicationManifest, UpsertSystemInput, string, bool, string) (ManifestPreview, error)
	LoadApplicationManifest(context.Context, string) (ApplicationManifest, error)
}

func (r *postgresRepository) HasApplicationManifest(ctx context.Context, clientID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ihp_systems WHERE client_id=$1 AND metadata ? 'applicationManifest')`, clientID).Scan(&exists)
	return exists, err
}

// Review and apply share one planner. Application changes, access gates and
// their audit record commit together. Existing role permissions are preserved.
func (r *postgresRepository) ReviewManifest(ctx context.Context, manifest ApplicationManifest, input UpsertSystemInput, token string, apply bool, actor string) (result ManifestPreview, err error) {
	options := &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
	if apply {
		options = &sql.TxOptions{Isolation: sql.LevelSerializable}
	}
	tx, err := r.db.BeginTx(ctx, options)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	defer func() {
		var postgresError *pq.Error
		if errors.As(err, &postgresError) && (postgresError.Code == "40001" || postgresError.Code == "40P01") {
			err = ErrManifestReview
		}
	}()
	if apply {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "application-manifest:"+manifest.Metadata.ID); err != nil {
			return result, err
		}
	}
	result, err = previewManifestTx(ctx, tx, manifest, input, apply)
	if err != nil {
		return result, err
	}
	if !apply {
		return result, tx.Commit()
	}
	if token == "" || (token != result.ReviewToken && result.Operation != "noop") {
		return result, ErrManifestReview
	}
	if result.Operation == "noop" {
		return result, tx.Commit()
	}
	if result.Before != nil && (result.Before.Enabled || result.Before.LifecycleStatus == "retired") {
		return result, fmt.Errorf("%w: suspend the existing application before changing its manifest; retired applications cannot be updated", ErrInvalidInput)
	}
	input = result.After
	input.ActorUserID = actor
	system, err := upsertSystemTx(ctx, tx, input)
	if err != nil {
		return result, err
	}
	for _, role := range result.RolesToCreate {
		if _, err = tx.ExecContext(ctx, `INSERT INTO ihp_system_roles(system_id,role_name,display_name) VALUES($1::uuid,$2,$2)`, system.ID, role); err != nil {
			return result, err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ihp_system_access_roles WHERE system_id=$1::uuid`, system.ID); err != nil {
		return result, err
	}
	for _, role := range manifest.Spec.Access.Roles {
		if _, err = tx.ExecContext(ctx, `INSERT INTO ihp_system_access_roles(system_id,role_name) VALUES($1::uuid,$2)`, system.ID, role); err != nil {
			return result, err
		}
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE ihp_systems SET metadata=metadata || jsonb_build_object('applicationManifest',$2::jsonb,'navigation',$3::text),updated_at=now() WHERE id=$1::uuid`, system.ID, string(raw), input.Navigation); err != nil {
		return result, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_rbac_audit_events(actor_user_id,action,resource_type,resource_id,system_client_id,details)
 VALUES(NULLIF($1,''),'application.manifest.imported','system',$2,$3,$4::jsonb)`, actor, system.ID, system.ClientID, string(raw))
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func previewManifestTx(ctx context.Context, tx *sql.Tx, manifest ApplicationManifest, input UpsertSystemInput, lock bool) (ManifestPreview, error) {
	result := ManifestPreview{Manifest: manifest, Operation: "create", After: input, RolesToCreate: []string{}, AccessRoles: append([]string(nil), manifest.Spec.Access.Roles...), Warnings: []string{"Registration follows the application lifecycle review before activation.", "Client roles must exist in Keycloak and be assigned through the existing access administration."}}
	current, err := loadSystem(ctx, tx, manifest.Metadata.ID, lock)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	metadata := json.RawMessage(`{}`)
	revision := ""
	roleState := map[string]bool{}
	if err == nil {
		result.Operation = "update"
		result.Before = &SystemDetail{System: current, AccessRoles: []string{}, Roles: []SystemRole{}}
		if err := tx.QueryRowContext(ctx, `SELECT metadata,updated_at::text FROM ihp_systems WHERE id=$1::uuid`, current.ID).Scan(&metadata, &revision); err != nil {
			return result, err
		}
		rows, err := tx.QueryContext(ctx, `SELECT role_name,enabled FROM ihp_system_roles WHERE system_id=$1::uuid ORDER BY role_name`, current.ID)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var role SystemRole
			if err := rows.Scan(&role.Name, &role.Enabled); err != nil {
				rows.Close()
				return result, err
			}
			roleState[role.Name] = role.Enabled
			result.Before.Roles = append(result.Before.Roles, role)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return result, err
		}
		rows.Close()
		rows, err = tx.QueryContext(ctx, `SELECT role_name FROM ihp_system_access_roles WHERE system_id=$1::uuid ORDER BY role_name`, current.ID)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err != nil {
				rows.Close()
				return result, err
			}
			result.Before.AccessRoles = append(result.Before.AccessRoles, role)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return result, err
		}
		rows.Close()
		result.After.AuthenticatedLaunchURL = current.AuthenticatedLaunchURL
		result.After.SortOrder = current.SortOrder
	}
	for _, role := range manifest.Spec.Access.Roles {
		enabled, exists := roleState[role]
		if exists && !enabled {
			return result, fmt.Errorf("%w: access role %q is disabled", ErrInvalidInput, role)
		}
		if !exists {
			result.RolesToCreate = append(result.RolesToCreate, role)
		}
	}
	for _, item := range manifest.Spec.Navigation {
		for _, permission := range item.RequiredPermissions {
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ihp_permissions WHERE permission_key=$1 AND status='active')`, permission).Scan(&exists); err != nil {
				return result, err
			}
			if !exists {
				return result, fmt.Errorf("%w: unknown navigation permission %q", ErrInvalidInput, permission)
			}
		}
	}
	if manifest.Spec.Launch.Mode == "microfrontend" {
		var duplicate bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ihp_systems WHERE client_id<>$1 AND metadata->'applicationManifest'->'spec'->'launch'->>'appName'=$2)`, manifest.Metadata.ID, manifest.Spec.Launch.AppName).Scan(&duplicate); err != nil {
			return result, err
		}
		if duplicate {
			return result, fmt.Errorf("%w: microfrontend appName is already registered", ErrInvalidInput)
		}
		result.Warnings = append(result.Warnings, "The entry URL must serve browser ES modules with CORS and single-spa lifecycle exports.")
	}
	if result.Before != nil && len(result.RolesToCreate) == 0 {
		var stored struct {
			Manifest ApplicationManifest `json:"applicationManifest"`
		}
		if err := json.Unmarshal(metadata, &stored); err != nil {
			return result, err
		}
		desired := append([]string(nil), manifest.Spec.Access.Roles...)
		sort.Strings(desired)
		if reflect.DeepEqual(stored.Manifest, manifest) && reflect.DeepEqual(desired, result.Before.AccessRoles) && manifestSystemMatches(current, result.After) {
			result.Operation = "noop"
			result.After.Enabled = &current.Enabled
		}
	}
	payload, err := json.Marshal(struct {
		Preview  ManifestPreview
		Metadata json.RawMessage
		Revision string
	}{result, metadata, revision})
	if err != nil {
		return result, err
	}
	sum := sha256.Sum256(payload)
	result.ReviewToken = hex.EncodeToString(sum[:])
	return result, nil
}

func manifestSystemMatches(system System, input UpsertSystemInput) bool {
	return system.DisplayName == input.DisplayName && system.Description == input.Description && system.Icon == input.Icon && system.LaunchURL == input.LaunchURL &&
		system.Category == input.Category && system.OwnerTeam == input.OwnerTeam && system.OwnerName == input.OwnerName && system.OwnerEmail == input.OwnerEmail &&
		system.SupportURL == input.SupportURL && system.Environment == input.Environment && system.Navigation == input.Navigation && system.SystemType == input.SystemType &&
		system.LaunchMode == input.LaunchMode && system.DisplayInLauncher == *input.DisplayInLauncher && system.DisplayInSideNav == *input.DisplayInSideNav && system.IntegrationType == input.IntegrationType && system.ContractVersion == input.ContractVersion
}

func (r *postgresRepository) LoadApplicationManifest(ctx context.Context, clientID string) (ApplicationManifest, error) {
	var manifest ApplicationManifest
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT metadata->'applicationManifest' FROM ihp_systems WHERE client_id=$1 AND enabled AND lifecycle_status='active' AND metadata ? 'applicationManifest'
 AND launch_url=COALESCE(metadata->'applicationManifest'->'spec'->'launch'->>'url',metadata->'applicationManifest'->'spec'->'launch'->>'route')`, clientID).Scan(&raw)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(raw, &manifest)
	return manifest, err
}
