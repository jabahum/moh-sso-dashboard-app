package rbac

import (
	"context"
	"encoding/json"
)

func lifecycleAudit(clientID, actor, action, from string, input LifecycleTransitionInput, outcome string) AuditEvent {
	details, _ := json.Marshal(map[string]string{"from": from, "to": input.Status, "reason": input.Reason, "outcome": outcome})
	return AuditEvent{ActorUserID: actor, Action: action, ResourceType: "system", SystemClientID: clientID, Details: details}
}

// Serialize lifecycle changes with configuration updates and commit their audit together.
func (r *postgresRepository) TransitionSystem(ctx context.Context, clientID string, input LifecycleTransitionInput, actor string, validate func(System, int) error) (System, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return System{}, err
	}
	defer tx.Rollback()
	system, err := loadSystem(ctx, tx, clientID, true)
	if err != nil {
		return System{}, err
	}
	var accessRoles int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM ihp_system_access_roles ar
 JOIN ihp_system_roles sr ON sr.system_id = ar.system_id AND sr.role_name = ar.role_name
 WHERE ar.system_id = $1::uuid AND sr.enabled = TRUE`, system.ID).Scan(&accessRoles)
	if err != nil {
		return System{}, err
	}
	validationErr := validate(system, accessRoles)
	action, outcome := "system.lifecycle.changed", "accepted"
	if validationErr != nil {
		action, outcome = "system.lifecycle.denied", validationErr.Error()
	}
	event := lifecycleAudit(clientID, actor, action, system.LifecycleStatus, input, outcome)
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_rbac_audit_events
 (actor_user_id, action, resource_type, resource_id, system_client_id, details)
 VALUES (NULLIF($1, ''), $2, 'system', $3, $4, $5::jsonb)`, actor, action, system.ID, clientID, string(event.Details))
	if err != nil {
		return System{}, err
	}
	if validationErr == nil {
		_, err = tx.ExecContext(ctx, `UPDATE ihp_systems SET lifecycle_status = $2, enabled = ($2 = 'active'), updated_at = now() WHERE client_id = $1`, clientID, input.Status)
		if err != nil {
			return System{}, err
		}
		system.LifecycleStatus, system.Enabled = input.Status, input.Status == "active"
	}
	if err := tx.Commit(); err != nil {
		return System{}, err
	}
	if validationErr != nil {
		return System{}, validationErr
	}
	return system, nil
}
