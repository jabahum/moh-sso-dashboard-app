package keycloak

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
)

// Client represents the Keycloak API client
// - Web auth/refresh/logout use dashboard-web (authorization_code + refresh_token)
type KCUserInfo struct {
	UserID            string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Name              string `json:"name"`
	District          string `json:"district"`
	Facility          string `json:"facility"`
}

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	IDToken          string `json:"id_token"`
}

type Session struct {
	ID         string            `json:"id"`
	IP         string            `json:"ipAddress"`
	Start      int64             `json:"start"`
	LastAccess int64             `json:"lastAccess"`
	Clients    map[string]string `json:"clients"`
}

type AuthUser struct {
	ID                string               `json:"id"`
	Username          string               `json:"username"`
	Email             string               `json:"email"`
	FirstName         string               `json:"firstName"`
	LastName          string               `json:"lastName"`
	FullName          string               `json:"fullName"`
	IsAdmin           bool                 `json:"isAdmin"`
	IsUser            bool                 `json:"isUser"`
	RealmRoles        []string             `json:"realmRoles"`
	ClientRoles       map[string][]string  `json:"clientRoles"`
	Permissions       []string             `json:"permissions"`
	Systems           []string             `json:"systems"`
	AccessibleSystems []authz.SystemAccess `json:"accessibleSystems"`
	Enabled           bool                 `json:"enabled"`
	EmailVerified     bool                 `json:"emailVerified"`
	LastLoginAt       *time.Time           `json:"lastLoginAt"`
	District          string               `json:"district,omitempty"`
	Facility          string               `json:"facility,omitempty"`
}

func (u *AuthUser) GetAuthorizationFields() (string, []string, map[string][]string) {
	if u == nil {
		return "", nil, nil
	}

	return u.ID, u.RealmRoles, u.ClientRoles
}

func (u *AuthUser) SetAuthorizationAccess(
	permissions []string,
	systems []string,
	accessibleSystems []authz.SystemAccess,
) {
	if u == nil {
		return
	}

	u.Permissions = permissions
	u.Systems = systems
	u.AccessibleSystems = accessibleSystems
}

// ----------------------------------------------------
// CLIENT
// ----------------------------------------------------

type Client struct {
	BaseURL      string
	Realm        string
	ClientID     string
	ClientSecret string
	Token        string
	httpClient   *http.Client
	cache        *cache.RedisCache
	config       *config.Config
}

func NewWebClient(
	baseURL string,
	realm string,
	clientID string,
	clientSecret string,
	cache *cache.RedisCache, // <-- FIXED (pointer)
	config *config.Config,
) *Client {

	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Client{
		BaseURL:      baseURL,
		Realm:        realm,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		cache:        cache,
		config:       config,
	}
}

// ----------------------------------------------------
// WEB AUTH (authorization_code)
// ----------------------------------------------------
func (c *Client) ExchangeCodeForToken(
	code string,
	redirectURI string,
	codeVerifier string,
) (*TokenResponse, error) {

	tokenURL := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/token",
		c.BaseURL,
		c.Realm,
	)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)

	res, err := c.httpClient.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed [%d]: %s", res.StatusCode, string(bodyBytes))
	}

	var tokenRes TokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenRes); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenRes, nil
}

// ----------------------------------------------------
// REFRESH TOKEN (WEB CLIENT)
// ----------------------------------------------------
func (c *Client) AccessToken(refreshToken string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/token",
		c.BaseURL,
		c.Realm,
	)

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)

	res, err := c.httpClient.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("refresh token failed: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token failed [%d]: %s", res.StatusCode, string(bodyBytes))
	}

	var tokenRes TokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenRes); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return &tokenRes, nil
}

// ----------------------------------------------------
// LOGOUT (WEB CLIENT)
// ----------------------------------------------------
func (c *Client) LogOut(refreshToken string) error {
	if refreshToken == "" {
		return fmt.Errorf("missing refresh token for logout")
	}

	logoutURL := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/logout",
		c.BaseURL,
		c.Realm,
	)

	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("refresh_token", refreshToken)

	res, err := c.httpClient.PostForm(logoutURL, form)
	if err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("logout failed [%d]: %s", res.StatusCode, string(body))
	}

	return nil
}

func (c *Client) GetLogoutURL(idTokenHint, postLogoutRedirectURI string) string {
	publicLogoutEndpoint := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/logout",
		c.config.KeycloakExternalURL,
		c.Realm,
	)

	params := url.Values{}
	params.Set("client_id", c.ClientID)

	if idTokenHint != "" {
		params.Set("id_token_hint", idTokenHint)
	}

	if postLogoutRedirectURI != "" {
		params.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	}

	return fmt.Sprintf("%s?%s", publicLogoutEndpoint, params.Encode())
}

// ----------------------------------------------------
// TOKEN → FULL USER PROFILE (ADMIN API)
// ----------------------------------------------------
func (c *Client) Me(accessToken string) (*AuthUser, error) {
	ui, err := c.getUserInfo(accessToken)
	if err != nil {
		return nil, err
	}

	if ui.UserID == "" {
		return nil, errors.New("userinfo missing user_id")
	}

	// ----------------------------------------------------
	// Extract roles directly from token claims
	// ----------------------------------------------------

	claims, err := c.parseToken(accessToken)
	if err != nil {
		return nil, err
	}

	realmRoles := []string{}
	clientRoles := map[string][]string{}

	// Realm roles
	if realmAccess, ok := claims["realm_access"].(map[string]any); ok {
		if roles, ok := realmAccess["roles"].([]any); ok {
			for _, r := range roles {
				role := fmt.Sprint(r)
				realmRoles = append(realmRoles, role)
			}
		}
	}

	// Client roles
	if resourceAccess, ok := claims["resource_access"].(map[string]any); ok {
		for clientID, v := range resourceAccess {
			if clientData, ok := v.(map[string]any); ok {
				if roles, ok := clientData["roles"].([]any); ok {
					for _, r := range roles {
						clientRoles[clientID] = append(
							clientRoles[clientID],
							fmt.Sprint(r),
						)
					}
				}
			}
		}
	}

	authContext := authz.NewContext(ui.UserID, realmRoles, clientRoles)

	firstName := ui.GivenName
	lastName := ui.FamilyName

	fullName := strings.TrimSpace(firstName + " " + lastName)
	if fullName == "" {
		fullName = ui.Name
	}

	user := &AuthUser{
		ID:                ui.UserID,
		Username:          ui.PreferredUsername,
		Email:             ui.Email,
		FirstName:         firstName,
		LastName:          lastName,
		FullName:          fullName,
		IsAdmin:           authContext.IsAdmin,
		IsUser:            authContext.IsUser,
		RealmRoles:        authContext.RealmRoles,
		ClientRoles:       authContext.ClientRoles,
		Permissions:       authContext.PermissionStrings(),
		Systems:           authContext.SystemStrings(),
		AccessibleSystems: authContext.SystemAccess(),
		Enabled:           true,
		EmailVerified:     ui.EmailVerified,
		District:          strings.TrimSpace(ui.District),
		Facility:          strings.TrimSpace(ui.Facility),
	}

	return user, nil
}

// ----------------------------------------------------
// UserInfo
// ----------------------------------------------------
func (c *Client) getUserInfo(accessToken string) (*KCUserInfo, error) {
	url := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/userinfo",
		c.BaseURL,
		c.Realm,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("userinfo failed [%d]: %s", res.StatusCode, string(body))
	}

	var ui KCUserInfo
	if err := json.NewDecoder(res.Body).Decode(&ui); err != nil {
		return nil, err
	}

	if ui.UserID == "" {
		return nil, errors.New("userinfo response missing user_id")
	}

	return &ui, nil
}

func (c *Client) parseToken(token string) (map[string]any, error) {

	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	return claims, nil
}
