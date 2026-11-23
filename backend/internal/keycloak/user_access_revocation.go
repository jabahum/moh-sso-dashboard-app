package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Errors returned by the access-revocation helpers. Messages carry status codes
// only, never Keycloak response bodies. ErrUserNotFound is shared with the
// identity-sync user helpers (identity_sync_users.go).
var (
	ErrClientNotFound = errors.New("keycloak client not found")
)

// exactClientUUID resolves a clientId to the client's internal ID, requiring an
// exact clientId match (unlike resolveClientUUID, it never treats a UUID-shaped
// clientId as an internal ID).
func (c *KeyAdminClient) exactClientUUID(ctx context.Context, clientID string) (string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", fmt.Errorf("clientID is required")
	}
	res, err := c.GetWithContext(ctx, "clients?"+url.Values{"clientId": {clientID}}.Encode())
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("look up client %q: HTTP %d", clientID, res.StatusCode)
	}
	var clients []struct {
		ID       string `json:"id"`
		ClientID string `json:"clientId"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&clients); err != nil {
		return "", fmt.Errorf("decode client lookup: %w", err)
	}
	for _, client := range clients {
		if client.ClientID == clientID && client.ID != "" {
			return client.ID, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrClientNotFound, clientID)
}

// userClientRoleMappings reads a user's direct (effective=false) or effective
// (effective=true, including group and composite roles) roles for one client.
func (c *KeyAdminClient) userClientRoleMappings(ctx context.Context, userID, clientUUID string, effective bool) ([]ClientRoleRep, error) {
	path := fmt.Sprintf("users/%s/role-mappings/clients/%s", url.PathEscape(userID), url.PathEscape(clientUUID))
	if effective {
		path += "/composite"
	}
	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrUserNotFound
	default:
		return nil, fmt.Errorf("read user client roles: HTTP %d", res.StatusCode)
	}
	var roles []ClientRoleRep
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&roles); err != nil {
		return nil, fmt.Errorf("decode user client roles: %w", err)
	}
	return roles, nil
}

// RemoveUserClientRoleIfPresent removes one direct client role mapping from a
// user. It is idempotent: removed is false when the user does not hold the role
// directly. A missing user returns ErrUserNotFound and a missing client
// ErrClientNotFound.
func (c *KeyAdminClient) RemoveUserClientRoleIfPresent(ctx context.Context, userID, clientID, roleName string) (removed bool, err error) {
	userID, roleName = strings.TrimSpace(userID), strings.TrimSpace(roleName)
	if userID == "" || roleName == "" {
		return false, fmt.Errorf("userID and role name are required")
	}
	clientUUID, err := c.exactClientUUID(ctx, clientID)
	if err != nil {
		return false, err
	}
	roles, err := c.userClientRoleMappings(ctx, userID, clientUUID, false)
	if err != nil {
		return false, err
	}
	var mapped *ClientRoleRep
	for i := range roles {
		if roles[i].Name == roleName && roles[i].ID != "" {
			mapped = &roles[i]
			break
		}
	}
	if mapped == nil {
		return false, nil
	}
	body, err := json.Marshal([]ClientRoleRequest{{ID: mapped.ID, Name: mapped.Name}})
	if err != nil {
		return false, err
	}
	path := fmt.Sprintf("users/%s/role-mappings/clients/%s", url.PathEscape(userID), url.PathEscape(clientUUID))
	res, err := c.DeleteWithBody(ctx, path, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, ErrUserNotFound
	default:
		return false, fmt.Errorf("remove user client role: HTTP %d", res.StatusCode)
	}
}

// UserHasEffectiveClientRole reports whether a user holds a client role directly,
// through a group or through a composite role. A missing user returns
// ErrUserNotFound.
func (c *KeyAdminClient) UserHasEffectiveClientRole(ctx context.Context, userID, clientID, roleName string) (bool, error) {
	userID, roleName = strings.TrimSpace(userID), strings.TrimSpace(roleName)
	if userID == "" || roleName == "" {
		return false, fmt.Errorf("userID and role name are required")
	}
	clientUUID, err := c.exactClientUUID(ctx, clientID)
	if err != nil {
		return false, err
	}
	roles, err := c.userClientRoleMappings(ctx, userID, clientUUID, true)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role.Name == roleName {
			return true, nil
		}
	}
	return false, nil
}

// RevokeUserClientConsent revokes a user's consent and offline tokens for one
// client (DELETE /users/{id}/consents/{clientId}). Keycloak also ends that
// client's sessions for the user; the user's SSO session and other clients are
// untouched. It is idempotent: a 404 (no consent or offline token, or no such
// user) counts as success.
func (c *KeyAdminClient) RevokeUserClientConsent(ctx context.Context, userID, clientID string) error {
	userID, clientID = strings.TrimSpace(userID), strings.TrimSpace(clientID)
	if userID == "" || clientID == "" {
		return fmt.Errorf("userID and clientID are required")
	}
	path := fmt.Sprintf("users/%s/consents/%s", url.PathEscape(userID), url.PathEscape(clientID))
	res, err := c.DeleteWithContext(ctx, path)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusNoContent, http.StatusOK, http.StatusNotFound:
		return nil
	default:
		return fmt.Errorf("revoke user client consent: HTTP %d", res.StatusCode)
	}
}
