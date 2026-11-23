package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// GetUserRoleMappings returns the realm and client roles mapped directly to a
// user. GET users/{id} carries no role mappings, so callers that need a user's
// current access must use this instead of reading UserInfo.RealmRoles or
// UserInfo.ClientRoles from GetUser.
func (c *KeyAdminClient) GetUserRoleMappings(ctx context.Context, userID string) ([]string, map[string][]string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil, fmt.Errorf("userID is required")
	}
	res, err := c.GetWithContext(ctx, "users/"+url.PathEscape(userID)+"/role-mappings")
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, nil, fmt.Errorf("get user role mappings failed [%d]: %s", res.StatusCode, string(body))
	}
	var mappings struct {
		RealmMappings  []RoleRep `json:"realmMappings"`
		ClientMappings map[string]struct {
			Client   string    `json:"client"`
			Mappings []RoleRep `json:"mappings"`
		} `json:"clientMappings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&mappings); err != nil {
		return nil, nil, fmt.Errorf("decode user role mappings: %w", err)
	}
	realm := make([]string, 0, len(mappings.RealmMappings))
	for _, role := range mappings.RealmMappings {
		if role.Name != "" {
			realm = append(realm, role.Name)
		}
	}
	sort.Strings(realm)
	clients := make(map[string][]string, len(mappings.ClientMappings))
	for key, client := range mappings.ClientMappings {
		clientID := client.Client
		if clientID == "" {
			clientID = key
		}
		names := make([]string, 0, len(client.Mappings))
		for _, role := range client.Mappings {
			if role.Name != "" {
				names = append(names, role.Name)
			}
		}
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)
		clients[clientID] = names
	}
	return realm, clients, nil
}
