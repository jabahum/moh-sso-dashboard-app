package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/utils"
)

// ----------------------------------------------------
// TYPES
// ----------------------------------------------------

type ClientInfo struct {
	ID                        string            `json:"id"`
	ClientID                  string            `json:"clientId"`
	Name                      string            `json:"name"`
	Description               string            `json:"description"`
	BaseURL                   string            `json:"baseUrl"`
	RootURL                   string            `json:"rootUrl"`
	AdminURL                  string            `json:"adminUrl"`
	Enabled                   bool              `json:"enabled"`
	PublicClient              bool              `json:"publicClient"`
	BearerOnly                bool              `json:"bearerOnly"`
	Protocol                  string            `json:"protocol"`
	Secret                    string            `json:"secret"`
	ClientAuthenticatorType   string            `json:"clientAuthenticatorType"`
	RedirectURIs              []string          `json:"redirectUris"`
	WebOrigins                []string          `json:"webOrigins"`
	StandardFlowEnabled       bool              `json:"standardFlowEnabled"`
	ImplicitFlowEnabled       bool              `json:"implicitFlowEnabled"`
	DirectAccessGrantsEnabled bool              `json:"directAccessGrantsEnabled"`
	ServiceAccountsEnabled    bool              `json:"serviceAccountsEnabled"`
	FullScopeAllowed          bool              `json:"fullScopeAllowed"`
	DefaultClientScopes       []string          `json:"defaultClientScopes"`
	OptionalClientScopes      []string          `json:"optionalClientScopes"`
	Attributes                map[string]string `json:"attributes"`
	ProtocolMappers           []ProtocolMapper  `json:"protocolMappers"`
}

type ProtocolMapper struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	ProtocolMapper string            `json:"protocolMapper"`
	Config         map[string]string `json:"config"`
}

type CreateClientParams struct {
	ClientID                  string            `json:"clientId"`
	Name                      string            `json:"name"`
	Description               string            `json:"description"`
	BaseURL                   string            `json:"baseUrl"`
	RootURL                   string            `json:"rootUrl"`
	RedirectURIs              []string          `json:"redirectUris"`
	WebOrigins                []string          `json:"webOrigins"`
	PublicClient              bool              `json:"publicClient"`
	Protocol                  string            `json:"protocol"`
	StandardFlowEnabled       bool              `json:"standardFlowEnabled"`
	ImplicitFlowEnabled       bool              `json:"implicitFlowEnabled"`
	DirectAccessGrantsEnabled bool              `json:"directAccessGrantsEnabled"`
	ServiceAccountsEnabled    bool              `json:"serviceAccountsEnabled"`
	Enabled                   bool              `json:"enabled"`
	Attributes                map[string]string `json:"attributes"`
}

type KeycloakUser struct {
	ID               string              `json:"id"`
	Username         string              `json:"username"`
	Email            string              `json:"email"`
	FirstName        string              `json:"firstName"`
	LastName         string              `json:"lastName"`
	Enabled          bool                `json:"enabled"`
	EmailVerified    bool                `json:"emailVerified"`
	RequiredActions  []string            `json:"requiredActions"`
	CreatedTimestamp int64               `json:"createdTimestamp"`
	Attributes       map[string][]string `json:"attributes"`
}

type UserInfo struct {
	ID                  string              `json:"id"`
	Username            string              `json:"username"`
	Email               string              `json:"email"`
	DisplayName         string              `json:"displayName"`
	FirstName           string              `json:"firstName"`
	LastName            string              `json:"lastName"`
	Enabled             bool                `json:"enabled"`
	AccountStatus       string              `json:"accountStatus"`
	EmailVerified       bool                `json:"emailVerified"`
	NeverLoggedIn       bool                `json:"neverLoggedIn"`
	RealmRoles          []string            `json:"realmRoles"`
	ClientRoles         map[string][]string `json:"clientRoles"`
	RequiredActions     []string            `json:"requiredActions"`
	FailedLoginAttempts int                 `json:"failedLoginAttempts"`
	TemporarilyLocked   bool                `json:"temporarilyLocked"`
	LastLoginAt         *time.Time          `json:"lastLoginAt"`
	LastLoginIP         string              `json:"lastLoginIp"`
	Source              string              `json:"source"`
	ImportedAt          *time.Time          `json:"importedAt"`
	Notes               string              `json:"notes"`
	CreatedAt           time.Time           `json:"createdAt"`
	CreatedTimestamp    int64               `json:"createdTimestamp"`
	Attributes          map[string][]string `json:"attributes"`
}

type CreateUserRequest struct {
	Username        string              `json:"username"`
	Email           string              `json:"email"`
	FirstName       string              `json:"firstName"`
	LastName        string              `json:"lastName"`
	Enabled         bool                `json:"enabled"`
	EmailVerified   bool                `json:"emailVerified"`
	RequiredActions []string            `json:"requiredActions,omitempty"`
	Attributes      map[string][]string `json:"attributes,omitempty"`
}

type UserRep struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Enabled  bool   `json:"enabled"`
}

type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Composite   bool   `json:"composite"`
	ClientRole  bool   `json:"clientRole"`
}

type UserClientRoleAssignment struct {
	Id         string   `json:"id"`
	ClientID   string   `json:"clientId"`
	ClientName string   `json:"clientName"`
	Roles      []string `json:"roles"`
}

type ClientRoleRep struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ClientRoleRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RoleRep struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type GroupRep struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Path          string              `json:"path"`
	ParentID      string              `json:"parentId,omitempty"`
	SubGroupCount int                 `json:"subGroupCount,omitempty"`
	Attributes    map[string][]string `json:"attributes,omitempty"`
	SubGroups     []GroupRep          `json:"subGroups"`
}

type compositeRoleRepresentation struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClientRole  bool   `json:"clientRole"`
	ContainerID string `json:"containerId"`
}

type ClientScope struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
}

type ClientTemplate string

const (
	WebClient     ClientTemplate = "web"
	ServiceClient ClientTemplate = "service"
	AdminClient   ClientTemplate = "admin"
)

var defaultClientScopes = []string{
	"profile",
	"email",
	"roles",
}

// ----------------------------------------------------
// CLIENT
// ----------------------------------------------------

type KeyAdminClient struct {
	BaseURL      string
	Realm        string
	ClientID     string
	ClientSecret string

	// Optional. Used by execute-actions-email.
	AppClientID string
	RedirectURI string

	httpClient *http.Client

	mu        sync.Mutex
	Token     string
	expiresAt time.Time
}

func NewAdminClient(
	baseURL string,
	realm string,
	clientID string,
	clientSecret string,
) *KeyAdminClient {
	return &KeyAdminClient{
		BaseURL:      strings.TrimSuffix(baseURL, "/"),
		Realm:        realm,
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *KeyAdminClient) WithEmailActionRedirect(
	appClientID string,
	redirectURI string,
) *KeyAdminClient {
	c.AppClientID = strings.TrimSpace(appClientID)
	c.RedirectURI = strings.TrimSpace(redirectURI)
	return c
}

// ----------------------------------------------------
// ADMIN AUTH
// ----------------------------------------------------

func (c *KeyAdminClient) Authenticate() error {
	_, err := c.ensureAdminToken(context.Background())
	if err != nil {
		return err
	}

	log.Println("✅ Admin client authenticated successfully")
	return nil
}

func (c *KeyAdminClient) ensureAdminToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.TrimSpace(c.ClientID) == "" {
		return "", fmt.Errorf("client_id is empty")
	}

	if strings.TrimSpace(c.ClientSecret) == "" {
		return "", fmt.Errorf("client_secret is empty")
	}

	// Reuse token if it is still valid for at least 30 seconds.
	if strings.TrimSpace(c.Token) != "" && time.Now().Before(c.expiresAt.Add(-30*time.Second)) {
		return c.Token, nil
	}

	token, expiresAt, err := c.requestAdminToken(ctx)
	if err != nil {
		return "", err
	}

	c.Token = token
	c.expiresAt = expiresAt

	return c.Token, nil
}

func (c *KeyAdminClient) forceRefreshAdminToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Token = ""
	c.expiresAt = time.Time{}

	token, expiresAt, err := c.requestAdminToken(ctx)
	if err != nil {
		return "", err
	}

	c.Token = token
	c.expiresAt = expiresAt

	return c.Token, nil
}

func (c *KeyAdminClient) requestAdminToken(ctx context.Context) (string, time.Time, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", strings.TrimSpace(c.ClientID))
	form.Set("client_secret", strings.TrimSpace(c.ClientSecret))

	tokenURL := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/token",
		strings.TrimRight(c.BaseURL, "/"),
		url.PathEscape(c.Realm),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create admin token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to call admin token endpoint: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf(
			"admin authentication failed [%d]: %s",
			res.StatusCode,
			string(bodyBytes),
		)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse admin token response: %w", err)
	}

	if strings.TrimSpace(body.AccessToken) == "" {
		return "", time.Time{}, fmt.Errorf("admin token response missing access_token")
	}

	if body.ExpiresIn <= 0 {
		body.ExpiresIn = 60
	}

	expiresAt := time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)

	return body.AccessToken, expiresAt, nil
}

// ----------------------------------------------------
// REALM MANAGEMENT
// ----------------------------------------------------
//
// Note: realm create/get endpoints live under /admin, not /admin/realms/{realm}.
// These methods use doRawAdminRequest.

func (c *KeyAdminClient) EnsureRealmExists(realmName string) error {
	realmName = strings.TrimSpace(realmName)
	if realmName == "" {
		return fmt.Errorf("realmName is required")
	}

	res, err := c.doRawAdminRequest(
		context.Background(),
		http.MethodGet,
		"realms/"+url.PathEscape(realmName),
		nil,
		nil,
	)
	if err == nil && res != nil && res.StatusCode == http.StatusOK {
		res.Body.Close()
		return nil
	}

	if res != nil {
		res.Body.Close()
	}

	payload := map[string]any{
		"realm":   realmName,
		"enabled": true,
	}

	res, err = c.doRawAdminRequest(
		context.Background(),
		http.MethodPost,
		"realms",
		payload,
		nil,
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to create realm: %s", string(body))
	}

	return nil
}

// ----------------------------------------------------
// CLIENT MANAGEMENT
// ----------------------------------------------------

func (c *KeyAdminClient) CreateClient(opts CreateClientParams) (string, error) {
	if strings.TrimSpace(opts.ClientID) == "" {
		return "", fmt.Errorf("clientId is required")
	}

	if opts.Protocol == "" {
		opts.Protocol = "openid-connect"
	}

	opts.Enabled = true
	opts.StandardFlowEnabled = true
	opts.ImplicitFlowEnabled = false
	opts.DirectAccessGrantsEnabled = false
	opts.ServiceAccountsEnabled = false

	if opts.RedirectURIs == nil {
		opts.RedirectURIs = []string{}
	}

	if opts.WebOrigins == nil {
		opts.WebOrigins = []string{}
	}

	if opts.Attributes == nil {
		opts.Attributes = map[string]string{}
	}

	opts.Attributes["realm_client"] = "false"
	opts.Attributes["created_by"] = "admin"

	if _, ok := opts.Attributes["ui.icon"]; !ok {
		opts.Attributes["ui.icon"] = "applications"
	}

	if _, ok := opts.Attributes["ui.home"]; !ok {
		opts.Attributes["ui.home"] = "/admin"
	}

	if err := utils.ValidateRedirectURIs(opts.RedirectURIs); err != nil {
		return "", err
	}

	res, err := c.Post("clients", opts)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("failed to create client: %s", string(body))
	}

	clientUUID, err := parseIDFromLocation(res.Header.Get("Location"))
	if err != nil {
		return "", err
	}

	if err := c.EnsureClientBaseline(context.Background(), clientUUID); err != nil {
		return "", err
	}

	return clientUUID, nil
}

func (c *KeyAdminClient) GetClientByClientID(clientID string) (*ClientInfo, error) {
	query := url.Values{}
	query.Set("clientId", clientID)

	res, err := c.Get("clients?" + query.Encode())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"failed checking clientId %q: %s",
			clientID,
			string(body),
		)
	}

	var clients []ClientInfo
	if err := json.NewDecoder(res.Body).Decode(&clients); err != nil {
		return nil, err
	}

	if len(clients) == 0 {
		return nil, nil
	}

	return &clients[0], nil
}

func (c *KeyAdminClient) GetClientByID(id string) (*ClientInfo, error) {
	res, err := c.Get("clients/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get client: %s", string(body))
	}

	var cli ClientInfo
	if err := json.NewDecoder(res.Body).Decode(&cli); err != nil {
		return nil, err
	}

	return &cli, nil
}

func (c *KeyAdminClient) ListClients() ([]ClientInfo, error) {
	res, err := c.Get("clients")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to list clients: %s", string(body))
	}

	var clients []ClientInfo
	if err := json.NewDecoder(res.Body).Decode(&clients); err != nil {
		return nil, err
	}

	return clients, nil
}

func (c *KeyAdminClient) ListClientsWithBaselineCheck() ([]ClientInfo, error) {
	ctx := context.Background()

	clients, err := c.ListClients()
	if err != nil {
		return nil, err
	}

	for _, client := range clients {
		if err := c.EnsureClientBaseline(ctx, client.ID); err != nil {
			return nil, fmt.Errorf(
				"client baseline check (clientId %s) failed: %w",
				client.ClientID,
				err,
			)
		}
	}

	return clients, nil
}

func (c *KeyAdminClient) UpdateClient(id string, payload *model.Client) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("client id is required")
	}

	res, err := c.Put("clients/"+url.PathEscape(id), payload)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"failed to update client (status %d): %s",
			res.StatusCode,
			string(body),
		)
	}

	return nil
}

func (c *KeyAdminClient) UpdateClientEnabled(
	ctx context.Context,
	clientID string,
	enabled bool,
) error {
	clientUUID, err := c.resolveClientUUID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("failed to resolve client UUID: %w", err)
	}

	path := "clients/" + url.PathEscape(clientUUID)

	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("get client failed [%d]: %s", res.StatusCode, string(b))
	}

	var kcClient ClientInfo
	if err := json.NewDecoder(res.Body).Decode(&kcClient); err != nil {
		return fmt.Errorf("decode client failed: %w", err)
	}

	kcClient.Enabled = enabled

	putRes, err := c.PutWithContext(ctx, path, kcClient)
	if err != nil {
		return err
	}
	defer putRes.Body.Close()

	if putRes.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(putRes.Body)
		return fmt.Errorf(
			"update client enabled failed [%d]: %s",
			putRes.StatusCode,
			string(b),
		)
	}

	return nil
}

func (c *KeyAdminClient) DeleteClient(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("client id is required")
	}

	res, err := c.Delete("clients/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete client: %s", string(body))
	}

	return nil
}

func (c *KeyAdminClient) UpdateClientPortalAttributes(
	ctx context.Context,
	clientID string,
	attributes map[string]string,
) error {
	client, err := c.GetClientByClientID(strings.TrimSpace(clientID))
	if err != nil {
		return err
	}
	if client == nil || strings.TrimSpace(client.ID) == "" {
		return fmt.Errorf("client %q does not exist", clientID)
	}
	if client.Attributes == nil {
		client.Attributes = map[string]string{}
	}
	for key, value := range attributes {
		client.Attributes[key] = value
	}

	res, err := c.PutWithContext(ctx, "clients/"+url.PathEscape(client.ID), client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("update client portal attributes failed [%d]: %s", res.StatusCode, string(body))
	}
	return nil
}

// ----------------------------------------------------
// USER MANAGEMENT
// ----------------------------------------------------

func (c *KeyAdminClient) CreateUser(user *model.User) (string, error) {
	if user == nil {
		return "", fmt.Errorf("user is required")
	}

	username := strings.TrimSpace(user.Username)
	email := strings.TrimSpace(user.Email)

	if username == "" {
		return "", fmt.Errorf("username is required")
	}

	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	enabled := user.Enabled
	if !enabled {
		enabled = true
	}

	requiredActions := []string{"UPDATE_PASSWORD"}
	if !user.EmailVerified {
		requiredActions = append([]string{"VERIFY_EMAIL"}, requiredActions...)
	}

	payload := CreateUserRequest{
		Username:        username,
		Email:           email,
		FirstName:       strings.TrimSpace(user.FirstName),
		LastName:        strings.TrimSpace(user.LastName),
		Enabled:         enabled,
		EmailVerified:   user.EmailVerified,
		RequiredActions: requiredActions,
		Attributes: map[string][]string{
			"created_by": {"admin"},
			"user_type":  {"human"},
		},
	}

	res, err := c.Post("users", payload)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("failed to create user: %s", string(body))
	}

	kcID, err := parseIDFromLocation(res.Header.Get("Location"))
	if err != nil {
		return "", err
	}

	return kcID, nil
}

func (c *KeyAdminClient) FindUsers(
	ctx context.Context,
	q string,
	exact bool,
) ([]UserRep, error) {
	v := url.Values{}

	if strings.TrimSpace(q) != "" {
		v.Set("search", strings.TrimSpace(q))
	}

	if exact {
		v.Set("exact", "true")
	}

	path := "users"
	if encoded := v.Encode(); encoded != "" {
		path += "?" + encoded
	}

	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"keycloak find users failed: status=%d body=%s",
			res.StatusCode,
			string(b),
		)
	}

	var out []UserRep
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

func (c *KeyAdminClient) ListUsers() ([]UserInfo, error) {
	res, err := c.Get("users")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to list users: %s", string(body))
	}

	var users []UserInfo
	if err := json.NewDecoder(res.Body).Decode(&users); err != nil {
		return nil, err
	}

	return users, nil
}

func (c *KeyAdminClient) GetUserGroups(ctx context.Context, userID string) ([]GroupRep, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	res, err := c.GetWithContext(ctx, "users/"+url.PathEscape(userID)+"/groups")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get user groups: status=%d body=%s", res.StatusCode, string(body))
	}

	var groups []GroupRep
	if err := json.NewDecoder(res.Body).Decode(&groups); err != nil {
		return nil, fmt.Errorf("decode user groups failed: %w", err)
	}

	return groups, nil
}

func (c *KeyAdminClient) ListGroups(ctx context.Context) ([]GroupRep, error) {
	roots, err := c.listGroupsPage(ctx, "groups", "failed to list groups")
	if err != nil {
		return nil, err
	}

	groups := make([]GroupRep, 0)
	var walk func(values []GroupRep) error
	walk = func(values []GroupRep) error {
		for _, group := range values {
			children := group.SubGroups
			if len(children) == 0 && group.SubGroupCount > 0 && strings.TrimSpace(group.ID) != "" {
				fetched, err := c.listGroupChildren(ctx, group.ID)
				if err != nil {
					return err
				}
				children = fetched
			}
			group.SubGroups = nil
			groups = append(groups, group)
			if err := walk(children); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(roots); err != nil {
		return nil, err
	}
	return groups, nil
}

func (c *KeyAdminClient) listGroupsPage(ctx context.Context, endpoint string, errorPrefix string) ([]GroupRep, error) {
	const pageSize = 100

	allGroups := make([]GroupRep, 0)
	for first := 0; ; first += pageSize {
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		pagedEndpoint := fmt.Sprintf("%s%sbriefRepresentation=false&first=%d&max=%d", endpoint, separator, first, pageSize)

		res, err := c.GetWithContext(ctx, pagedEndpoint)
		if err != nil {
			return nil, err
		}

		if res.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(res.Body)
			res.Body.Close()
			return nil, fmt.Errorf("%s: %s", errorPrefix, string(body))
		}

		var groups []GroupRep
		if err := json.NewDecoder(res.Body).Decode(&groups); err != nil {
			res.Body.Close()
			return nil, err
		}
		res.Body.Close()

		allGroups = append(allGroups, groups...)
		if len(groups) < pageSize {
			break
		}
	}
	return allGroups, nil
}

func (c *KeyAdminClient) listGroupChildren(ctx context.Context, groupID string) ([]GroupRep, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	return c.listGroupsPage(ctx, "groups/"+url.PathEscape(groupID)+"/children", "failed to list group children")
}

func (c *KeyAdminClient) EnsureGroupPath(
	ctx context.Context,
	groupPath string,
	displayName string,
	description string,
	attributes map[string][]string,
) (GroupRep, error) {
	groupPath = normalizeKeycloakGroupPath(groupPath)
	if groupPath == "" {
		return GroupRep{}, fmt.Errorf("groupPath is required")
	}

	groups, err := c.ListGroups(ctx)
	if err != nil {
		return GroupRep{}, err
	}

	byPath := make(map[string]GroupRep, len(groups))
	for _, group := range groups {
		byPath[normalizeKeycloakGroupPath(group.Path)] = group
	}
	if existing, ok := byPath[groupPath]; ok && strings.TrimSpace(existing.ID) != "" {
		return existing, nil
	}

	segments := strings.Split(strings.Trim(groupPath, "/"), "/")
	parentID := ""
	currentPath := ""
	var current GroupRep
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		currentPath = normalizeKeycloakGroupPath(currentPath + "/" + segment)
		if existing, ok := byPath[currentPath]; ok && strings.TrimSpace(existing.ID) != "" {
			current = existing
			parentID = existing.ID
			continue
		}

		// Creation payloads must omit server-owned fields. Sending id=""
		// makes Keycloak interpret a child creation as an update of that ID.
		payload := struct {
			Name       string              `json:"name"`
			Attributes map[string][]string `json:"attributes,omitempty"`
		}{
			Name: segment,
		}
		if currentPath == groupPath {
			payload.Attributes = copyStringSliceMap(attributes)
			if strings.TrimSpace(displayName) != "" {
				if payload.Attributes == nil {
					payload.Attributes = map[string][]string{}
				}
				payload.Attributes["display_name"] = []string{strings.TrimSpace(displayName)}
			}
			if strings.TrimSpace(description) != "" {
				if payload.Attributes == nil {
					payload.Attributes = map[string][]string{}
				}
				payload.Attributes["description"] = []string{strings.TrimSpace(description)}
			}
		}

		endpoint := "groups"
		if parentID != "" {
			endpoint = "groups/" + url.PathEscape(parentID) + "/children"
		}

		res, err := c.PostWithContext(ctx, endpoint, payload)
		if err != nil {
			return GroupRep{}, err
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusConflict {
			return GroupRep{}, fmt.Errorf("create group %q failed: status=%d body=%s", currentPath, res.StatusCode, string(body))
		}

		groups, err = c.ListGroups(ctx)
		if err != nil {
			return GroupRep{}, err
		}
		byPath = make(map[string]GroupRep, len(groups))
		for _, group := range groups {
			byPath[normalizeKeycloakGroupPath(group.Path)] = group
		}
		existing, ok := byPath[currentPath]
		if !ok || strings.TrimSpace(existing.ID) == "" {
			return GroupRep{}, fmt.Errorf("create group %q failed: created group was not found after refresh", currentPath)
		}
		current = existing
		parentID = existing.ID
	}

	if strings.TrimSpace(current.ID) == "" {
		return GroupRep{}, fmt.Errorf("create group %q failed: group id is empty", groupPath)
	}
	return current, nil
}

func (c *KeyAdminClient) ListGroupMembers(ctx context.Context, groupID string) ([]KeycloakUser, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	res, err := c.GetWithContext(ctx, "groups/"+url.PathEscape(groupID)+"/members")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to list group members: %s", string(body))
	}

	var users []KeycloakUser
	if err := json.NewDecoder(res.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
}

func normalizeKeycloakGroupPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '/' })
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return "/" + strings.Join(cleaned, "/")
}

func copyStringSliceMap(input map[string][]string) map[string][]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string][]string, len(input))
	for key, values := range input {
		copied := make([]string, len(values))
		copy(copied, values)
		out[key] = copied
	}
	return out
}

func (c *KeyAdminClient) AddUserToGroup(ctx context.Context, userID string, groupID string) error {
	userID = strings.TrimSpace(userID)
	groupID = strings.TrimSpace(groupID)
	if userID == "" || groupID == "" {
		return fmt.Errorf("userID and groupID are required")
	}

	res, err := c.PutWithContext(
		ctx,
		"users/"+url.PathEscape(userID)+"/groups/"+url.PathEscape(groupID),
		nil,
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to add user to group: status=%d body=%s", res.StatusCode, string(body))
	}
	return nil
}

func (c *KeyAdminClient) RemoveUserFromGroup(ctx context.Context, userID string, groupID string) error {
	userID = strings.TrimSpace(userID)
	groupID = strings.TrimSpace(groupID)
	if userID == "" || groupID == "" {
		return fmt.Errorf("userID and groupID are required")
	}

	res, err := c.DeleteWithContext(
		ctx,
		"users/"+url.PathEscape(userID)+"/groups/"+url.PathEscape(groupID),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to remove user from group: status=%d body=%s", res.StatusCode, string(body))
	}
	return nil
}

func (c *KeyAdminClient) ListGroupRealmRoles(ctx context.Context, groupID string) ([]RoleRep, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	res, err := c.GetWithContext(ctx, "groups/"+url.PathEscape(groupID)+"/role-mappings/realm")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to list group realm roles: %s", string(body))
	}

	var roles []RoleRep
	if err := json.NewDecoder(res.Body).Decode(&roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func (c *KeyAdminClient) ListGroupClientRoles(ctx context.Context, groupID string, clientID string) ([]ClientRoleRep, error) {
	groupID = strings.TrimSpace(groupID)
	clientID = strings.TrimSpace(clientID)
	if groupID == "" || clientID == "" {
		return nil, fmt.Errorf("groupID and clientID are required")
	}

	clientUUID, err := c.resolveClientUUID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve client UUID: %w", err)
	}

	path := fmt.Sprintf(
		"groups/%s/role-mappings/clients/%s",
		url.PathEscape(groupID),
		url.PathEscape(clientUUID),
	)
	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to list group client roles: %s", string(body))
	}

	var roles []ClientRoleRep
	if err := json.NewDecoder(res.Body).Decode(&roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func (c *KeyAdminClient) GetUser(userID string) (*UserInfo, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	res, err := c.Get("users/" + url.PathEscape(userID))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get user: %s", string(body))
	}

	var user UserInfo
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (c *KeyAdminClient) UpdateUser(user *model.User) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}

	if strings.TrimSpace(user.ID) == "" {
		return fmt.Errorf("missing Keycloak user ID")
	}

	current, err := c.GetUser(user.ID)
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	username := strings.TrimSpace(user.Username)
	if username == "" {
		username = current.Username
	}

	email := strings.TrimSpace(user.Email)
	if email == "" {
		email = current.Email
	}

	firstName := strings.TrimSpace(user.FirstName)
	if firstName == "" {
		firstName = current.FirstName
	}

	lastName := strings.TrimSpace(user.LastName)
	if lastName == "" {
		lastName = current.LastName
	}

	payload := map[string]any{
		"username":        username,
		"email":           email,
		"firstName":       firstName,
		"lastName":        lastName,
		"enabled":         user.Enabled,
		"emailVerified":   user.EmailVerified,
		"requiredActions": current.RequiredActions,
		"attributes":      current.Attributes,
	}

	res, err := c.Put("users/"+url.PathEscape(user.ID), payload)
	if err != nil {
		return fmt.Errorf("keycloak update request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to update user in keycloak: %s", string(body))
	}

	return nil
}

func (c *KeyAdminClient) DeleteUser(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("invalid user ID")
	}

	res, err := c.Delete("users/" + url.PathEscape(userID))
	if err != nil {
		return fmt.Errorf("keycloak delete request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete user in keycloak: %s", string(body))
	}

	return nil
}

func (c *KeyAdminClient) SetUserEnabled(
	ctx context.Context,
	userID string,
	enabled bool,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	current, err := c.GetUser(userID)
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	payload := map[string]any{
		"username":        current.Username,
		"email":           current.Email,
		"firstName":       current.FirstName,
		"lastName":        current.LastName,
		"enabled":         enabled,
		"emailVerified":   current.EmailVerified,
		"requiredActions": current.RequiredActions,
		"attributes":      current.Attributes,
	}

	res, err := c.PutWithContext(ctx, "users/"+url.PathEscape(userID), payload)
	if err != nil {
		return fmt.Errorf("keycloak set user enabled request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"failed to set user enabled in keycloak [%d]: %s",
			res.StatusCode,
			string(body),
		)
	}

	return nil
}

// ResetUserPassword sends a password reset email using Keycloak required actions.
// It does NOT generate or set a temporary password.
func (c *KeyAdminClient) ResetUserPassword(
	ctx context.Context,
	userID string,
) error {
	return c.SendUserPasswordResetEmail(ctx, userID)
}

func (c *KeyAdminClient) ExecuteActionsEmail(
	ctx context.Context,
	userID string,
	actions []string,
) error {
	userID = strings.TrimSpace(userID)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if len(actions) == 0 {
		return fmt.Errorf("at least one required action is required")
	}

	query := url.Values{}
	query.Set("lifespan", "43200") // 12 hours

	if strings.TrimSpace(c.AppClientID) != "" {
		query.Set("client_id", strings.TrimSpace(c.AppClientID))
	}

	if strings.TrimSpace(c.RedirectURI) != "" {
		query.Set("redirect_uri", strings.TrimSpace(c.RedirectURI))
	}

	path := fmt.Sprintf(
		"users/%s/execute-actions-email?%s",
		url.PathEscape(userID),
		query.Encode(),
	)

	res, err := c.PutWithContext(ctx, path, actions)
	if err != nil {
		return fmt.Errorf("execute actions email request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"execute actions email failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

func (c *KeyAdminClient) SendUserOnboardingEmail(
	ctx context.Context,
	userID string,
) error {
	return c.ExecuteActionsEmail(ctx, userID, []string{
		"VERIFY_EMAIL",
		"UPDATE_PASSWORD",
	})
}

func (c *KeyAdminClient) SendUserVerificationEmail(
	ctx context.Context,
	userID string,
) error {
	return c.ExecuteActionsEmail(ctx, userID, []string{
		"VERIFY_EMAIL",
	})
}

func (c *KeyAdminClient) SendUserPasswordResetEmail(
	ctx context.Context,
	userID string,
) error {
	return c.ExecuteActionsEmail(ctx, userID, []string{
		"UPDATE_PASSWORD",
	})
}

func (c *KeyAdminClient) SetTemporaryPassword(
	ctx context.Context,
	userID string,
	temporaryPassword string,
) error {
	userID = strings.TrimSpace(userID)
	temporaryPassword = strings.TrimSpace(temporaryPassword)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if temporaryPassword == "" {
		return fmt.Errorf("temporaryPassword is required")
	}

	payload := map[string]any{
		"type":      "password",
		"value":     temporaryPassword,
		"temporary": true,
	}

	path := fmt.Sprintf(
		"users/%s/reset-password",
		url.PathEscape(userID),
	)

	res, err := c.PutWithContext(ctx, path, payload)
	if err != nil {
		return fmt.Errorf("set temporary password request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"set temporary password failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

func generateTemporaryPassword() string {
	return utils.RandomString(16) + "!A1"
}

// ----------------------------------------------------
// REALM ROLES
// ----------------------------------------------------

func (c *KeyAdminClient) CreateRealmRole(
	ctx context.Context,
	roleName string,
	description string,
) error {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return fmt.Errorf("roleName is required")
	}

	payload := CreateRoleRequest{
		Name:        roleName,
		Description: description,
		Composite:   false,
		ClientRole:  false,
	}

	res, err := c.PostWithContext(ctx, "roles", payload)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusConflict {
		return nil
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("create realm role failed: %s", string(b))
	}

	return nil
}

func (c *KeyAdminClient) ListRealmRoles(ctx context.Context) ([]RoleRep, error) {
	res, err := c.GetWithContext(ctx, "roles")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("list realm roles failed: %s", string(b))
	}

	var roles []RoleRep
	if err := json.NewDecoder(res.Body).Decode(&roles); err != nil {
		return nil, err
	}

	return roles, nil
}

func (c *KeyAdminClient) EnsureRealmRoleClientRoleComposite(
	ctx context.Context,
	realmRole string,
	clientID string,
	roleName string,
) error {
	realmRole = strings.TrimSpace(realmRole)
	clientID = strings.TrimSpace(clientID)
	roleName = strings.TrimSpace(roleName)
	if realmRole == "" || clientID == "" || roleName == "" {
		return fmt.Errorf("realmRole, clientID, and roleName are required")
	}

	client, err := c.GetClientByClientID(clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return fmt.Errorf("client %q does not exist", clientID)
	}
	role, err := c.GetClientRoleByName(ctx, clientID, client.ID, roleName)
	if err != nil {
		return err
	}

	payload := []compositeRoleRepresentation{{
		ID:          role.ID,
		Name:        role.Name,
		ClientRole:  true,
		ContainerID: client.ID,
	}}
	endpoint := fmt.Sprintf("roles/%s/composites", url.PathEscape(realmRole))
	res, err := c.PostWithContext(ctx, endpoint, payload)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode == http.StatusForbidden {
			return fmt.Errorf(
				"add realm role composite failed [403]: keycloak admin client %q service account needs realm-management realm-admin or manage-realm to assign client role %q/%q to realm role %q: %s",
				c.ClientID,
				clientID,
				roleName,
				realmRole,
				string(body),
			)
		}
		return fmt.Errorf("add realm role composite failed [%d]: %s", res.StatusCode, string(body))
	}
	return nil
}

func (c *KeyAdminClient) GetRealmRoleByName(
	ctx context.Context,
	roleName string,
) (*RoleRep, error) {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return nil, fmt.Errorf("roleName is required")
	}

	res, err := c.GetWithContext(ctx, "roles/"+url.PathEscape(roleName))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"keycloak get role failed: status=%d body=%s",
			res.StatusCode,
			string(b),
		)
	}

	var out RoleRep
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *KeyAdminClient) AddRealmRoleToUser(
	ctx context.Context,
	userID string,
	role RoleRep,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if strings.TrimSpace(role.ID) == "" || strings.TrimSpace(role.Name) == "" {
		return fmt.Errorf("role id and name are required")
	}

	payload := []RoleRep{role}

	path := fmt.Sprintf(
		"users/%s/role-mappings/realm",
		url.PathEscape(userID),
	)

	res, err := c.PostWithContext(ctx, path, payload)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"keycloak add role failed: status=%d body=%s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

func (c *KeyAdminClient) RemoveRealmRoleFromUser(
	ctx context.Context,
	userID string,
	role RoleRep,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if strings.TrimSpace(role.ID) == "" || strings.TrimSpace(role.Name) == "" {
		return fmt.Errorf("role id and name are required")
	}

	path := fmt.Sprintf(
		"users/%s/role-mappings/realm",
		url.PathEscape(userID),
	)

	body, err := json.Marshal([]RoleRep{role})
	if err != nil {
		return err
	}

	res, err := c.DeleteWithBody(ctx, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("remove realm role failed: %s", string(b))
	}

	return nil
}

func (c *KeyAdminClient) BootstrapRealmRoles(ctx context.Context) error {
	roles := []string{
		"admin",
		"user",
		"manager",
	}

	for _, role := range roles {
		if err := c.CreateRealmRole(ctx, role, "system role"); err != nil {
			return err
		}
	}

	return nil
}

// ----------------------------------------------------
// CLIENT ROLES
// ----------------------------------------------------

func (c *KeyAdminClient) CreateClientRole(
	ctx context.Context,
	clientID string,
	req *model.CreateClientRoleRequest,
) error {
	if req == nil {
		return fmt.Errorf("role request is required")
	}

	if strings.TrimSpace(req.Role) == "" {
		return fmt.Errorf("role is required")
	}

	clientUUID, err := c.resolveClientUUID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("failed to resolve client UUID: %w", err)
	}

	path := fmt.Sprintf(
		"clients/%s/roles",
		url.PathEscape(clientUUID),
	)

	payload := map[string]any{
		"name":        strings.TrimSpace(req.Role),
		"description": req.Description,
		"clientRole":  true,
	}

	res, err := c.PostWithContext(ctx, path, payload)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusConflict {
		return nil
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"create client role failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

func (c *KeyAdminClient) GetClientRoleByName(
	ctx context.Context,
	clientID string,
	clientUUID string,
	roleName string,
) (*ClientRoleRep, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientUUID) == "" {
		return nil, fmt.Errorf("clientID and clientUUID are required")
	}

	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return nil, fmt.Errorf("roleName is required")
	}

	path := fmt.Sprintf(
		"clients/%s/roles/%s",
		url.PathEscape(clientUUID),
		url.PathEscape(roleName),
	)

	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"get client role failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	var role ClientRoleRep
	if err := json.NewDecoder(res.Body).Decode(&role); err != nil {
		return nil, err
	}

	return &role, nil
}

func (c *KeyAdminClient) GetUserClientRoles(
	ctx context.Context,
	userID string,
	clientID string,
	clientUUID string,
) ([]ClientRoleRep, error) {
	userID = strings.TrimSpace(userID)
	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	if clientID == "" || clientUUID == "" {
		return nil, fmt.Errorf("clientID and clientUUID are required")
	}

	path := fmt.Sprintf(
		"users/%s/role-mappings/clients/%s",
		url.PathEscape(userID),
		url.PathEscape(clientUUID),
	)

	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"get user client roles failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	var roles []ClientRoleRep
	if err := json.NewDecoder(res.Body).Decode(&roles); err != nil {
		return nil, err
	}

	return roles, nil
}

func (c *KeyAdminClient) AssignClientRolesToUser(
	ctx context.Context,
	userID string,
	clientID string,
	clientUUID string,
	roles []string,
) error {
	userID = strings.TrimSpace(userID)
	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if clientID == "" || clientUUID == "" {
		return fmt.Errorf("clientID and clientUUID are required")
	}

	if len(roles) == 0 {
		return nil
	}

	payload := make([]ClientRoleRequest, 0, len(roles))

	for _, roleName := range roles {
		roleName = strings.TrimSpace(roleName)
		if roleName == "" {
			continue
		}

		role, err := c.GetClientRoleByName(ctx, clientID, clientUUID, roleName)
		if err != nil {
			return fmt.Errorf(
				"failed to resolve role %q for client %q (uuid: %q): %w",
				roleName,
				clientID,
				clientUUID,
				err,
			)
		}

		payload = append(payload, ClientRoleRequest{
			ID:   role.ID,
			Name: role.Name,
		})
	}

	if len(payload) == 0 {
		return nil
	}

	path := fmt.Sprintf(
		"users/%s/role-mappings/clients/%s",
		url.PathEscape(userID),
		url.PathEscape(clientUUID),
	)

	res, err := c.PostWithContext(ctx, path, payload)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"assign client roles failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

func (c *KeyAdminClient) RemoveClientRoleFromUser(
	ctx context.Context,
	userID string,
	clientUUID string,
	role ClientRoleRep,
) error {
	userID = strings.TrimSpace(userID)
	clientUUID = strings.TrimSpace(clientUUID)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if clientUUID == "" {
		return fmt.Errorf("clientUUID is required")
	}

	if strings.TrimSpace(role.ID) == "" || strings.TrimSpace(role.Name) == "" {
		return fmt.Errorf("role id and name are required")
	}

	path := fmt.Sprintf(
		"users/%s/role-mappings/clients/%s",
		url.PathEscape(userID),
		url.PathEscape(clientUUID),
	)

	body, err := json.Marshal([]ClientRoleRep{role})
	if err != nil {
		return err
	}

	res, err := c.DeleteWithBody(ctx, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("remove client role failed: %s", string(b))
	}

	return nil
}

func (c *KeyAdminClient) RemoveClientRolesFromUser(
	ctx context.Context,
	userID string,
	clientID string,
	clientUUID string,
	roles []string,
) error {
	userID = strings.TrimSpace(userID)
	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if clientID == "" {
		return fmt.Errorf("clientID is required")
	}

	if clientUUID == "" {
		resolvedUUID, err := c.resolveClientUUID(ctx, clientID)
		if err != nil {
			return fmt.Errorf("failed to resolve client UUID: %w", err)
		}
		clientUUID = resolvedUUID
	}

	if len(roles) == 0 {
		return nil
	}

	payload := make([]ClientRoleRep, 0, len(roles))

	for _, roleName := range roles {
		roleName = strings.TrimSpace(roleName)
		if roleName == "" {
			continue
		}

		role, err := c.GetClientRoleByName(ctx, clientID, clientUUID, roleName)
		if err != nil {
			return fmt.Errorf(
				"failed to resolve role %q for client %q: %w",
				roleName,
				clientID,
				err,
			)
		}

		payload = append(payload, *role)
	}

	if len(payload) == 0 {
		return nil
	}

	path := fmt.Sprintf(
		"users/%s/role-mappings/clients/%s",
		url.PathEscape(userID),
		url.PathEscape(clientUUID),
	)

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	res, err := c.DeleteWithBody(ctx, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"remove client roles failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

func (c *KeyAdminClient) ListClientRoles(
	ctx context.Context,
	clientID string,
) ([]ClientRoleRep, error) {
	clientUUID, err := c.resolveClientUUID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve client UUID: %w", err)
	}

	path := fmt.Sprintf(
		"clients/%s/roles",
		url.PathEscape(clientUUID),
	)

	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"list client roles failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	var roles []ClientRoleRep
	if err := json.NewDecoder(res.Body).Decode(&roles); err != nil {
		return nil, err
	}

	return roles, nil
}

func (c *KeyAdminClient) DeleteClientRole(
	ctx context.Context,
	clientID string,
	role string,
) error {
	clientUUID, err := c.resolveClientUUID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("failed to resolve client UUID: %w", err)
	}

	role = strings.TrimSpace(role)
	if role == "" {
		return fmt.Errorf("role is required")
	}

	path := fmt.Sprintf(
		"clients/%s/roles/%s",
		url.PathEscape(clientUUID),
		url.PathEscape(role),
	)

	res, err := c.DeleteWithContext(ctx, path)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"delete client role failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

func (c *KeyAdminClient) resolveClientUUID(
	ctx context.Context,
	clientID string,
) (string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", fmt.Errorf("resolveClientUUID: clientID is required")
	}

	if _, err := uuid.Parse(clientID); err == nil {
		return clientID, nil
	}

	path := "clients?clientId=" + url.QueryEscape(clientID)

	res, err := c.GetWithContext(ctx, path)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"resolveClientUUID: failed [%d]: %s",
			res.StatusCode,
			string(body),
		)
	}

	var clients []struct {
		ID       string `json:"id"`
		ClientID string `json:"clientId"`
	}

	if err := json.Unmarshal(body, &clients); err != nil {
		return "", err
	}

	if len(clients) == 0 {
		return "", fmt.Errorf("client not found in keycloak: %s", clientID)
	}

	return clients[0].ID, nil
}

// ----------------------------------------------------
// CLIENT SCOPES / BASELINE
// ----------------------------------------------------

func (c *KeyAdminClient) attachDefaultClientScopes(
	ctx context.Context,
	clientUUID string,
) error {
	scopes, err := c.ListClientScopes(ctx)
	if err != nil {
		return err
	}

	scopeByName := map[string]string{}
	for _, scope := range scopes {
		scopeByName[scope.Name] = scope.ID
	}

	for _, name := range defaultClientScopes {
		scopeID, ok := scopeByName[name]
		if !ok {
			return fmt.Errorf("required client scope %q not found", name)
		}

		res, err := c.PutWithContext(
			ctx,
			fmt.Sprintf(
				"clients/%s/default-client-scopes/%s",
				url.PathEscape(clientUUID),
				url.PathEscape(scopeID),
			),
			nil,
		)
		if err != nil {
			return err
		}

		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"failed to attach default client scope %q: %s",
				name,
				string(body),
			)
		}
	}

	return nil
}

func (c *KeyAdminClient) ListClientScopes(ctx context.Context) ([]ClientScope, error) {
	res, err := c.GetWithContext(ctx, "client-scopes")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"failed to list client scopes: %s",
			string(body),
		)
	}

	var scopes []ClientScope
	if err := json.NewDecoder(res.Body).Decode(&scopes); err != nil {
		return nil, err
	}

	return scopes, nil
}

func (c *KeyAdminClient) EnsureClientScopes(
	ctx context.Context,
	required []string,
) (map[string]string, error) {
	existing, err := c.ListClientScopes(ctx)
	if err != nil {
		return nil, err
	}

	scopeByName := map[string]string{}
	for _, scope := range existing {
		scopeByName[scope.Name] = scope.ID
	}

	for _, name := range required {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if _, ok := scopeByName[name]; ok {
			continue
		}

		payload := map[string]any{
			"name":     name,
			"protocol": "openid-connect",
		}

		res, err := c.PostWithContext(ctx, "client-scopes", payload)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusConflict {
			return nil, fmt.Errorf(
				"failed to create client scope %q: %s",
				name,
				string(body),
			)
		}

		existing, err = c.ListClientScopes(ctx)
		if err != nil {
			return nil, err
		}

		for _, scope := range existing {
			scopeByName[scope.Name] = scope.ID
		}
	}

	return scopeByName, nil
}

func (c *KeyAdminClient) DetectMissingClientScopes(
	ctx context.Context,
	clientUUID string,
	required map[string]string,
) ([]string, error) {
	res, err := c.GetWithContext(
		ctx,
		fmt.Sprintf(
			"clients/%s/default-client-scopes",
			url.PathEscape(clientUUID),
		),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to fetch client scopes: %s", string(body))
	}

	var attached []ClientScope
	if err := json.NewDecoder(res.Body).Decode(&attached); err != nil {
		return nil, err
	}

	attachedNames := map[string]bool{}
	for _, scope := range attached {
		attachedNames[scope.Name] = true
	}

	var missing []string
	for name := range required {
		if !attachedNames[name] {
			missing = append(missing, name)
		}
	}

	return missing, nil
}

func ApplyClientTemplate(
	opts *CreateClientParams,
	template ClientTemplate,
) {
	switch template {
	case WebClient:
		opts.PublicClient = false
		opts.StandardFlowEnabled = true
		opts.DirectAccessGrantsEnabled = false
		opts.ServiceAccountsEnabled = false

	case ServiceClient:
		opts.PublicClient = false
		opts.StandardFlowEnabled = false
		opts.DirectAccessGrantsEnabled = true
		opts.ServiceAccountsEnabled = true

	case AdminClient:
		opts.PublicClient = false
		opts.StandardFlowEnabled = true
		opts.DirectAccessGrantsEnabled = true
		opts.ServiceAccountsEnabled = true
	}
}

func (c *KeyAdminClient) CreateClientWithTemplate(
	ctx context.Context,
	opts CreateClientParams,
	template ClientTemplate,
) (string, error) {
	ApplyClientTemplate(&opts, template)

	if err := utils.ValidateRedirectURIs(opts.RedirectURIs); err != nil {
		return "", err
	}

	requiredScopes := []string{"profile", "email", "roles"}

	scopeIDs, err := c.EnsureClientScopes(ctx, requiredScopes)
	if err != nil {
		return "", err
	}

	clientUUID, err := c.CreateClient(opts)
	if err != nil {
		return "", err
	}

	missing, err := c.DetectMissingClientScopes(ctx, clientUUID, scopeIDs)
	if err != nil {
		return "", err
	}

	if len(missing) > 0 {
		if err := c.FixClientScopeDrift(ctx, clientUUID, scopeIDs, missing); err != nil {
			return "", err
		}
	}

	return clientUUID, nil
}

func (c *KeyAdminClient) FixClientScopeDrift(
	ctx context.Context,
	clientUUID string,
	scopeIDs map[string]string,
	missing []string,
) error {
	for _, name := range missing {
		scopeID, ok := scopeIDs[name]
		if !ok || scopeID == "" {
			return fmt.Errorf("cannot fix drift: scope %q not found", name)
		}

		res, err := c.PutWithContext(
			ctx,
			fmt.Sprintf(
				"clients/%s/default-client-scopes/%s",
				url.PathEscape(clientUUID),
				url.PathEscape(scopeID),
			),
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to attach client scope %q: %w", name, err)
		}

		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"failed to attach client scope %q: %s",
				name,
				string(body),
			)
		}
	}

	return nil
}

func (c *KeyAdminClient) EnsureClientBaseline(
	ctx context.Context,
	clientUUID string,
) error {
	scopeIDs, err := c.EnsureClientScopes(
		ctx,
		[]string{"profile", "email", "roles"},
	)
	if err != nil {
		return err
	}

	missing, err := c.DetectMissingClientScopes(ctx, clientUUID, scopeIDs)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		if err := c.FixClientScopeDrift(ctx, clientUUID, scopeIDs, missing); err != nil {
			return err
		}
	}

	return nil
}

// ----------------------------------------------------
// SESSION MANAGEMENT
// ----------------------------------------------------

func (c *KeyAdminClient) GetUserSessions(userID string) ([]Session, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	path := fmt.Sprintf(
		"users/%s/sessions",
		url.PathEscape(userID),
	)

	res, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list sessions: %s", string(body))
	}

	var sessions []Session
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (c *KeyAdminClient) LogoutSession(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("sessionID is required")
	}

	path := fmt.Sprintf(
		"sessions/%s",
		url.PathEscape(sessionID),
	)

	res, err := c.Delete(path)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf(
			"delete session failed [%d]: %s",
			res.StatusCode,
			string(b),
		)
	}

	return nil
}

// ----------------------------------------------------
// HTTP HELPERS
// ----------------------------------------------------

func (c *KeyAdminClient) doRequest(
	ctx context.Context,
	method string,
	path string,
	body any,
	rawBody io.Reader,
) (*http.Response, error) {
	adminURL := fmt.Sprintf(
		"%s/admin/realms/%s/%s",
		strings.TrimRight(c.BaseURL, "/"),
		url.PathEscape(c.Realm),
		strings.TrimPrefix(path, "/"),
	)

	return c.doAuthenticatedRequest(ctx, method, adminURL, body, rawBody)
}

func (c *KeyAdminClient) doRawAdminRequest(
	ctx context.Context,
	method string,
	path string,
	body any,
	rawBody io.Reader,
) (*http.Response, error) {
	adminURL := fmt.Sprintf(
		"%s/admin/%s",
		strings.TrimRight(c.BaseURL, "/"),
		strings.TrimPrefix(path, "/"),
	)

	return c.doAuthenticatedRequest(ctx, method, adminURL, body, rawBody)
}

func (c *KeyAdminClient) doAuthenticatedRequest(
	ctx context.Context,
	method string,
	fullURL string,
	body any,
	rawBody io.Reader,
) (*http.Response, error) {
	bodyBytes, err := buildRequestBodyBytes(body, rawBody)
	if err != nil {
		return nil, err
	}

	token, err := c.ensureAdminToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("keycloak admin client authentication failed: %w", err)
	}

	res, err := c.executeAdminRequest(ctx, method, fullURL, bodyBytes, token)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusUnauthorized {
		return res, nil
	}

	// Token may have expired or been invalidated.
	// Close the failed response and retry once using a freshly requested token.
	_ = res.Body.Close()

	token, err = c.forceRefreshAdminToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("keycloak admin token refresh failed after 401: %w", err)
	}

	return c.executeAdminRequest(ctx, method, fullURL, bodyBytes, token)
}

func (c *KeyAdminClient) executeAdminRequest(
	ctx context.Context,
	method string,
	fullURL string,
	bodyBytes []byte,
	token string,
) (*http.Response, error) {
	var reqBody io.Reader
	if bodyBytes != nil {
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func buildRequestBodyBytes(body any, rawBody io.Reader) ([]byte, error) {
	if rawBody != nil {
		bodyBytes, err := io.ReadAll(rawBody)
		if err != nil {
			return nil, err
		}

		return bodyBytes, nil
	}

	if body == nil {
		return nil, nil
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return bodyBytes, nil
}

func (c *KeyAdminClient) Get(path string) (*http.Response, error) {
	return c.GetWithContext(context.Background(), path)
}

func (c *KeyAdminClient) Post(path string, body any) (*http.Response, error) {
	return c.PostWithContext(context.Background(), path, body)
}

func (c *KeyAdminClient) Put(path string, body any) (*http.Response, error) {
	return c.PutWithContext(context.Background(), path, body)
}

func (c *KeyAdminClient) Delete(path string) (*http.Response, error) {
	return c.DeleteWithContext(context.Background(), path)
}

func (c *KeyAdminClient) GetWithContext(
	ctx context.Context,
	path string,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil, nil)
}

func (c *KeyAdminClient) PostWithContext(
	ctx context.Context,
	path string,
	body any,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPost, path, body, nil)
}

func (c *KeyAdminClient) PutWithContext(
	ctx context.Context,
	path string,
	body any,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPut, path, body, nil)
}

func (c *KeyAdminClient) DeleteWithContext(
	ctx context.Context,
	path string,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil)
}

func (c *KeyAdminClient) DeleteWithBody(
	ctx context.Context,
	path string,
	body io.Reader,
) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodDelete, path, nil, body)
}

func parseIDFromLocation(location string) (string, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		return "", fmt.Errorf("no Location header returned by Keycloak")
	}

	parts := strings.Split(strings.TrimRight(location, "/"), "/")
	id := strings.TrimSpace(parts[len(parts)-1])

	if id == "" {
		return "", fmt.Errorf("failed to parse ID from Location header")
	}

	return id, nil
}
