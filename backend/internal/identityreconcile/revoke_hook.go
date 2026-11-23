package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moh-sso-dashboard/internal/accesswarnings"
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
	if !revocation.Complete() || revocation.SessionsRevoked == StepUnsupported {
		return &IncompleteRevocationError{Revocation: revocation}
	}
	return nil
}

// IncompleteRevocationError is the portal hook's error when the revocation was
// recorded but did not finish (resume retries it) or the application cannot
// end sessions (contract 1.0). WarningCode gives the portal a stable code.
type IncompleteRevocationError struct {
	Revocation Revocation
}

func (e *IncompleteRevocationError) Error() string {
	r := e.Revocation
	if r.Complete() && r.SessionsRevoked == StepUnsupported {
		return fmt.Sprintf("identity-sync revocation %s for %s: the application cannot end existing sessions (contract 1.0); they last until local expiry", r.ID, r.ApplicationClientID)
	}
	return fmt.Sprintf("identity-sync revocation %s for %s is incomplete (%s); run identity-sync revocations --resume", r.ID, r.ApplicationClientID, r.LastError)
}

// WarningCode returns accesswarnings.CodeSessionsNotEnded for a 1.0
// application and accesswarnings.CodeRevocationIncomplete otherwise.
func (e *IncompleteRevocationError) WarningCode() string {
	if e.Revocation.Complete() && e.Revocation.SessionsRevoked == StepUnsupported {
		return accesswarnings.CodeSessionsNotEnded
	}
	return accesswarnings.CodeRevocationIncomplete
}

// PortalReGrantReason is the restore reason recorded when a portal admin grants
// the access role again to a person with an active revocation.
const PortalReGrantReason = "portal re-grant"

// ClientRolesAdded is called after roles were granted to userID on clientID
// through the portal. When the roles include a registered application's access
// role and the person has an active revocation for it, the revocation is
// restored by that admin (reason "portal re-grant", audited), so the revocation
// list matches reality. It never grants anything itself.
func (h *PortalRevocationHook) ClientRolesAdded(ctx context.Context, userID, clientID string, roles []string, actorID string) error {
	if h == nil || h.Service == nil || h.Service.Integrations == nil || h.Service.Repository == nil {
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
		return fmt.Errorf("identity-sync restore for %s: read integration: %w", clientID, err)
	}
	if !containsRole(roles, integration.AccessRole) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	revoked, err := h.Service.IsRevoked(ctx, clientID, h.Service.Issuer, userID)
	if err != nil {
		return fmt.Errorf("identity-sync restore for %s: %w", clientID, err)
	}
	if !revoked {
		return nil
	}
	actor := "portal"
	if actorID = strings.TrimSpace(actorID); actorID != "" {
		actor = "portal:" + actorID
	}
	if _, err := h.Service.Restore(ctx, clientID, userID, actor, PortalReGrantReason); err != nil && !errors.Is(err, ErrRevocationNotFound) {
		return fmt.Errorf("identity-sync restore for %s: %w", clientID, err)
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
