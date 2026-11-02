package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrUnauthorized = errors.New("invalid platform identity")
var ErrUnavailable = errors.New("platform identity unavailable")
var ErrForbidden = errors.New("platform access denied")
var ErrInvalid = errors.New("invalid platform request")

const IdentityRead = "identity:read"
const NotificationsWrite = "notifications:write"
const NotificationsRead = "notifications:read"

type Token struct {
	TokenType       string           `json:"token_type"`
	Active          bool             `json:"active"`
	Issuer          string           `json:"iss"`
	Subject         string           `json:"sub"`
	Audience        jwt.ClaimStrings `json:"aud"`
	ClientID        string           `json:"client_id"`
	AuthorizedParty string           `json:"azp"`
	Scope           string           `json:"scope"`
	ExpiresAt       int64            `json:"exp"`
	NotBefore       int64            `json:"nbf"`
	Username        string           `json:"preferred_username"`
	ResourceAccess  map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

type TokenVerifier interface {
	Verify(context.Context, string) (Token, error)
}

type Introspector struct {
	Endpoint, Issuer, Audience, ClientID, Secret string
	HTTP                                         *http.Client
}

// Verify trusts only the authenticated introspection response, never unverified JWT claims.
// Responses and requests are bounded; token/secret/payload are never returned in errors.
func (v *Introspector) Verify(ctx context.Context, raw string) (Token, error) {
	if v == nil || v.Endpoint == "" || v.Issuer == "" || v.Audience == "" || v.ClientID == "" || v.Secret == "" {
		return Token{}, ErrUnavailable
	}
	if raw == "" || len(raw) > 16384 {
		return Token{}, ErrUnauthorized
	}
	client := v.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	// Do not forward tokens or credentials across redirects, including same-host redirects.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.Endpoint, strings.NewReader(url.Values{"token": {raw}, "token_type_hint": {"access_token"}}.Encode()))
	if err != nil {
		return Token{}, ErrUnavailable
	}
	req.SetBasicAuth(v.ClientID, v.Secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := copyClient.Do(req)
	if err != nil {
		return Token{}, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Token{}, ErrUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(body) > 65536 {
		return Token{}, ErrUnavailable
	}
	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return Token{}, ErrUnavailable
	}
	if token.ClientID == "" {
		token.ClientID = token.AuthorizedParty
	}
	if token.AuthorizedParty != "" && token.AuthorizedParty != token.ClientID {
		return Token{}, ErrUnauthorized
	}
	now := time.Now().Unix()
	if !strings.EqualFold(token.TokenType, "Bearer") || !token.Active || token.Subject == "" || token.ClientID == "" || token.Issuer != v.Issuer || token.ExpiresAt <= now || token.NotBefore > now {
		return Token{}, ErrUnauthorized
	}
	for _, aud := range token.Audience {
		if aud == v.Audience {
			return token, nil
		}
	}
	return Token{}, ErrUnauthorized
}

type Principal struct {
	ApplicationID string   `json:"applicationId"`
	ClientID      string   `json:"clientId"`
	Subject       string   `json:"subject"`
	Scopes        []string `json:"scopes"`
}

func (p Principal) HasScope(scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
func intersectScopes(granted []string, token string) []string {
	available := map[string]bool{}
	for _, scope := range strings.Fields(token) {
		available[scope] = true
	}
	scopes := make([]string, 0)
	for _, scope := range granted {
		if available[scope] {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}
