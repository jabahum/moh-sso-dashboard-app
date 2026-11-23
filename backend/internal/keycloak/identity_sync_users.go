package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Small, context-aware user helpers used by identity-sync invitations. Their
// errors carry the HTTP status only, never Keycloak response bodies.

var (
	// ErrUserNotFound reports a user ID Keycloak does not know.
	ErrUserNotFound = errors.New("keycloak user not found")
	// ErrUserConflict reports a create rejected because the username or email exists.
	ErrUserConflict = errors.New("keycloak user already exists")
)

// UserSearch selects users by exact email and/or one exact attribute value.
// Max bounds the result (1-100, default 10).
type UserSearch struct {
	Email          string
	AttributeName  string
	AttributeValue string
	Max            int
}

// SearchUsers lists users matching every set criterion with full
// representations (attributes included). Keycloak's attribute and email
// queries are exact; callers should still compare the values they rely on.
func (c *KeyAdminClient) SearchUsers(ctx context.Context, search UserSearch) ([]KeycloakUser, error) {
	query := url.Values{"exact": {"true"}, "briefRepresentation": {"false"}}
	if email := strings.TrimSpace(search.Email); email != "" {
		query.Set("email", email)
	}
	if search.AttributeName != "" {
		if strings.ContainsAny(search.AttributeName, ": ") || strings.ContainsAny(search.AttributeValue, " ") || search.AttributeValue == "" {
			return nil, errors.New("invalid keycloak attribute search")
		}
		query.Set("q", search.AttributeName+":"+search.AttributeValue)
	}
	if query.Get("email") == "" && query.Get("q") == "" {
		return nil, errors.New("keycloak user search needs an email or attribute")
	}
	limit := search.Max
	if limit <= 0 {
		limit = 10
	}
	query.Set("max", strconv.Itoa(min(limit, 100)))
	res, err := c.GetWithContext(ctx, "users?"+query.Encode())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
		return nil, fmt.Errorf("keycloak user search: HTTP %d", res.StatusCode)
	}
	var users []KeycloakUser
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&users); err != nil {
		return nil, fmt.Errorf("decode keycloak user search: %w", err)
	}
	return users, nil
}

// CreateUserFromRequest creates a user exactly as requested (no default roles
// or attributes are added) and returns its ID. A 409 returns ErrUserConflict.
func (c *KeyAdminClient) CreateUserFromRequest(ctx context.Context, req CreateUserRequest) (string, error) {
	res, err := c.PostWithContext(ctx, "users", req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	switch res.StatusCode {
	case http.StatusCreated:
		return parseIDFromLocation(res.Header.Get("Location"))
	case http.StatusConflict:
		return "", ErrUserConflict
	default:
		return "", fmt.Errorf("keycloak create user: HTTP %d", res.StatusCode)
	}
}

// GetUserRepresentation reads one user with its attributes. A 404 returns ErrUserNotFound.
func (c *KeyAdminClient) GetUserRepresentation(ctx context.Context, userID string) (*KeycloakUser, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("userID is required")
	}
	res, err := c.GetWithContext(ctx, "users/"+url.PathEscape(userID))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrUserNotFound
	default:
		return nil, fmt.Errorf("keycloak get user: HTTP %d", res.StatusCode)
	}
	var user KeycloakUser
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode keycloak user: %w", err)
	}
	return &user, nil
}

// DeleteUserWithContext deletes a user. Deleting a missing user succeeds.
func (c *KeyAdminClient) DeleteUserWithContext(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("userID is required")
	}
	res, err := c.DeleteWithContext(ctx, "users/"+url.PathEscape(userID))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("keycloak delete user: HTTP %d", res.StatusCode)
	}
	return nil
}

// GetEffectiveUserClientRoles lists the client roles a user holds directly or
// through composites and groups. A 404 returns ErrUserNotFound.
func (c *KeyAdminClient) GetEffectiveUserClientRoles(ctx context.Context, userID, clientUUID string) ([]ClientRoleRep, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(clientUUID) == "" {
		return nil, errors.New("userID and clientUUID are required")
	}
	res, err := c.GetWithContext(ctx, "users/"+url.PathEscape(userID)+"/role-mappings/clients/"+url.PathEscape(clientUUID)+"/composite")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrUserNotFound
	default:
		return nil, fmt.Errorf("keycloak effective client roles: HTTP %d", res.StatusCode)
	}
	var roles []ClientRoleRep
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&roles); err != nil {
		return nil, fmt.Errorf("decode keycloak client roles: %w", err)
	}
	return roles, nil
}
