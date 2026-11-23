package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrNotServiceClient means a client with the requested clientId exists but is
// not a confidential, service-account-only client. EnsureServiceClient never
// reshapes such a client; an administrator has to resolve the conflict.
var ErrNotServiceClient = errors.New("keycloak client exists but is not a service-account-only client")

// ServiceClientRole is one client role granted to a service client's service
// account, for example clims/identity_sync.
type ServiceClientRole struct {
	ClientID string
	Role     string
}

// ServiceClientSpec describes a confidential client that only obtains tokens
// with the client-credentials grant, such as clims-identity-sync. Its shape
// matches the sync clients in infra/keycloak/realm-export.json.
type ServiceClientSpec struct {
	ClientID    string
	Name        string
	Description string
	// Audience is the client ID added to the access token's aud claim.
	Audience string
	// Roles are assigned to the service account and added to the client's
	// dedicated scope, which fullScopeAllowed=false requires for the roles to
	// reach the token.
	Roles []ServiceClientRole
}

// ServiceClientResult reports what EnsureServiceClient changed. A run against a
// client that is already complete reports no changes.
type ServiceClientResult struct {
	ClientID            string   `json:"clientId"`
	Created             bool     `json:"created"`
	AudienceMapperAdded bool     `json:"audienceMapperAdded"`
	RolesAssigned       []string `json:"rolesAssigned,omitempty"`
	ScopeMappingsAdded  []string `json:"scopeMappingsAdded,omitempty"`
}

// Changed reports whether the run created or added anything.
func (r ServiceClientResult) Changed() bool {
	return r.Created || r.AudienceMapperAdded || len(r.RolesAssigned) > 0 || len(r.ScopeMappingsAdded) > 0
}

type serviceClientState struct {
	ID                        string `json:"id"`
	ClientID                  string `json:"clientId"`
	PublicClient              bool   `json:"publicClient"`
	BearerOnly                bool   `json:"bearerOnly"`
	ServiceAccountsEnabled    bool   `json:"serviceAccountsEnabled"`
	StandardFlowEnabled       bool   `json:"standardFlowEnabled"`
	ImplicitFlowEnabled       bool   `json:"implicitFlowEnabled"`
	DirectAccessGrantsEnabled bool   `json:"directAccessGrantsEnabled"`
}

type protocolMapperRep struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	ProtocolMapper string            `json:"protocolMapper"`
	ConsentNeeded  bool              `json:"consentRequired"`
	Config         map[string]string `json:"config"`
}

// EnsureServiceClient creates or completes a service-account-only client. It is
// additive and idempotent:
//
//   - A missing client is created confidential, with only service accounts
//     enabled, fullScopeAllowed off, the roles default scope, an audience mapper
//     and portal attributes that keep it out of the launcher. Keycloak generates
//     its secret; this method never reads or sets one.
//   - An existing client is never reshaped. If it is public, bearer-only, lacks
//     a service account or has a browser or password flow enabled,
//     ErrNotServiceClient is returned and nothing is changed.
//   - A missing audience mapper, service-account role or scope mapping is added.
//     Nothing is removed.
//
// The roles' clients and the roles themselves must already exist.
func (c *KeyAdminClient) EnsureServiceClient(ctx context.Context, spec ServiceClientSpec) (ServiceClientResult, error) {
	spec.ClientID = strings.TrimSpace(spec.ClientID)
	spec.Audience = strings.TrimSpace(spec.Audience)
	result := ServiceClientResult{ClientID: spec.ClientID}
	if spec.ClientID == "" || spec.Audience == "" {
		return result, fmt.Errorf("service client: clientId and audience are required")
	}

	existing, err := c.findServiceClientState(ctx, spec.ClientID)
	if err != nil {
		return result, err
	}

	clientUUID := ""
	if existing == nil {
		clientUUID, err = c.createServiceClient(ctx, spec)
		if err != nil {
			return result, err
		}
		result.Created = true
	} else {
		if existing.PublicClient || existing.BearerOnly || !existing.ServiceAccountsEnabled ||
			existing.StandardFlowEnabled || existing.ImplicitFlowEnabled || existing.DirectAccessGrantsEnabled {
			return result, fmt.Errorf("%w: %s", ErrNotServiceClient, spec.ClientID)
		}
		clientUUID = existing.ID
		added, err := c.ensureAudienceMapper(ctx, clientUUID, spec.Audience)
		if err != nil {
			return result, err
		}
		result.AudienceMapperAdded = added
	}

	if len(spec.Roles) == 0 {
		return result, nil
	}

	userID, err := c.ServiceAccountUserID(ctx, spec.ClientID)
	if err != nil {
		return result, fmt.Errorf("service client %q: %w", spec.ClientID, err)
	}

	for _, ref := range spec.Roles {
		roleClientID := strings.TrimSpace(ref.ClientID)
		roleName := strings.TrimSpace(ref.Role)
		label := roleClientID + "/" + roleName
		roleClientUUID, err := c.exactClientUUID(ctx, roleClientID)
		if err != nil {
			return result, fmt.Errorf("service client %q role %s: %w", spec.ClientID, label, err)
		}
		role, err := c.GetClientRoleByName(ctx, roleClientID, roleClientUUID, roleName)
		if err != nil {
			return result, fmt.Errorf("service client %q role %s: %w", spec.ClientID, label, err)
		}
		payload := []ClientRoleRequest{{ID: role.ID, Name: role.Name}}

		held, err := c.userClientRoleMappings(ctx, userID, roleClientUUID, false)
		if err != nil {
			return result, fmt.Errorf("service client %q role %s: %w", spec.ClientID, label, err)
		}
		if !hasClientRole(held, role.Name) {
			path := fmt.Sprintf("users/%s/role-mappings/clients/%s", url.PathEscape(userID), url.PathEscape(roleClientUUID))
			if err := c.postExpectNoContent(ctx, path, payload, "assign service account role "+label); err != nil {
				return result, err
			}
			result.RolesAssigned = append(result.RolesAssigned, label)
		}

		scopePath := fmt.Sprintf("clients/%s/scope-mappings/clients/%s", url.PathEscape(clientUUID), url.PathEscape(roleClientUUID))
		var scoped []ClientRoleRep
		if err := c.getJSON(ctx, scopePath, &scoped, "read scope mappings"); err != nil {
			return result, err
		}
		if !hasClientRole(scoped, role.Name) {
			if err := c.postExpectNoContent(ctx, scopePath, payload, "add scope mapping "+label); err != nil {
				return result, err
			}
			result.ScopeMappingsAdded = append(result.ScopeMappingsAdded, label)
		}
	}

	return result, nil
}

func (c *KeyAdminClient) findServiceClientState(ctx context.Context, clientID string) (*serviceClientState, error) {
	var clients []serviceClientState
	if err := c.getJSON(ctx, "clients?"+url.Values{"clientId": {clientID}}.Encode(), &clients, "look up client "+clientID); err != nil {
		return nil, err
	}
	for i := range clients {
		if clients[i].ClientID == clientID && clients[i].ID != "" {
			return &clients[i], nil
		}
	}
	return nil, nil
}

func (c *KeyAdminClient) createServiceClient(ctx context.Context, spec ServiceClientSpec) (string, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = spec.ClientID
	}
	representation := map[string]any{
		"clientId":                  spec.ClientID,
		"name":                      name,
		"description":               strings.TrimSpace(spec.Description),
		"enabled":                   true,
		"protocol":                  "openid-connect",
		"publicClient":              false,
		"clientAuthenticatorType":   "client-secret",
		"standardFlowEnabled":       false,
		"implicitFlowEnabled":       false,
		"directAccessGrantsEnabled": false,
		"serviceAccountsEnabled":    true,
		"fullScopeAllowed":          false,
		"redirectUris":              []string{},
		"webOrigins":                []string{},
		"defaultClientScopes":       []string{"roles"},
		"optionalClientScopes":      []string{},
		"attributes": map[string]string{
			"portal.system":        "false",
			"ui.displayInLauncher": "false",
			"ui.displayInSideNav":  "false",
		},
		"protocolMappers": []protocolMapperRep{audienceMapper(spec.Audience)},
	}
	res, err := c.PostWithContext(ctx, "clients", representation)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create service client %q: HTTP %d", spec.ClientID, res.StatusCode)
	}
	return parseIDFromLocation(res.Header.Get("Location"))
}

func audienceMapper(audience string) protocolMapperRep {
	return protocolMapperRep{
		Name:           audience + "-audience",
		Protocol:       "openid-connect",
		ProtocolMapper: "oidc-audience-mapper",
		Config: map[string]string{
			"included.client.audience": audience,
			"id.token.claim":           "false",
			"access.token.claim":       "true",
		},
	}
}

func (c *KeyAdminClient) ensureAudienceMapper(ctx context.Context, clientUUID, audience string) (bool, error) {
	path := "clients/" + url.PathEscape(clientUUID) + "/protocol-mappers/models"
	var mappers []protocolMapperRep
	if err := c.getJSON(ctx, path, &mappers, "read protocol mappers"); err != nil {
		return false, err
	}
	for _, mapper := range mappers {
		if mapper.ProtocolMapper == "oidc-audience-mapper" &&
			mapper.Config["included.client.audience"] == audience &&
			mapper.Config["access.token.claim"] == "true" {
			return false, nil
		}
	}
	res, err := c.PostWithContext(ctx, path, audienceMapper(audience))
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		return false, fmt.Errorf("add audience mapper: HTTP %d", res.StatusCode)
	}
	return true, nil
}

func (c *KeyAdminClient) getJSON(ctx context.Context, path string, out any, what string) error {
	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", what, res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}
	return nil
}

func (c *KeyAdminClient) postExpectNoContent(ctx context.Context, path string, body any, what string) error {
	res, err := c.PostWithContext(ctx, path, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("%s: HTTP %d", what, res.StatusCode)
	}
	return nil
}

func hasClientRole(roles []ClientRoleRep, name string) bool {
	for _, role := range roles {
		if role.Name == name {
			return true
		}
	}
	return false
}
