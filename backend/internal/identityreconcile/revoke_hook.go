package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DefaultHookTimeout bounds the synchronous revocation a portal role removal
// triggers. It covers the connector's full retry budget for one call.
const DefaultHookTimeout = 2 * time.Minute

// PortalRevocationHook runs the revoke flow when portal RBAC or the users API
// removes a registered application's access role from a person. It implements
// users.AccessRoleRemovalHook.
type PortalRevocationHook struct {
	Service *RevokeService
	Timeout time.Duration
}

// NewPortalRevocationHook returns a hook backed by service.
func NewPortalRevocationHook(service *RevokeService) *PortalRevocationHook {
	return &PortalRevocationHook{Service: service, Timeout: DefaultHookTimeout}
}

// ClientRolesRemoved is called after roles were removed from userID (a Keycloak
// user ID) on clientID. It revokes only when clientID has an enabled
// identity-sync integration, the removed roles include its access role, and the
// person no longer holds that role through a group or composite role. The role
// removal has already happened; an error here means the revocation is
// incomplete (its status and audit trail say which step failed).
func (h *PortalRevocationHook) ClientRolesRemoved(ctx context.Context, userID, clientID string, roles []string, actorID string) error {
	if h == nil || h.Service == nil || h.Service.Integrations == nil {
		return nil
	}
	clientID = strings.TrimSpace(clientID)
	if !applicationIDPattern.MatchString(clientID) {
		return nil
	}
	integration, err := h.Service.Integrations.Get(ctx, clientID)
	if errors.Is(err, ErrIntegrationNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("identity-sync revocation for %s: read integration: %w", clientID, err)
	}
	if !integration.Enabled || !containsRole(roles, integration.AccessRole) {
		return nil
	}
	// Finish the revocation even if the admin's HTTP request is cancelled.
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = DefaultHookTimeout
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if h.Service.Directory != nil {
		still, err := h.Service.Directory.HasAccessRole(ctx, userID, clientID, integration.AccessRole)
		if err == nil && still {
			// Access continues through a group or composite role; nothing was removed.
			return nil
		}
	}
	actor := "portal"
	if actorID = strings.TrimSpace(actorID); actorID != "" {
		actor = "portal:" + actorID
	}
	revocation, err := h.Service.Revoke(ctx, clientID, userID, actor, "access role "+integration.AccessRole+" removed through the portal")
	if err != nil {
		return fmt.Errorf("identity-sync revocation for %s: %w", clientID, err)
	}
	if !revocation.Complete() {
		return fmt.Errorf("identity-sync revocation %s for %s is incomplete (%s); run identity-sync revocations --resume", revocation.ID, clientID, revocation.LastError)
	}
	if revocation.SessionsRevoked == StepUnsupported {
		return fmt.Errorf("identity-sync revocation %s for %s: the application cannot end existing sessions (contract 1.0); they last until local expiry", revocation.ID, clientID)
	}
	return nil
}

func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if strings.TrimSpace(r) == role {
			return true
		}
	}
	return false
}
