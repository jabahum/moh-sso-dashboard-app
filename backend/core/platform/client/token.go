package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenSource returns a Bearer access token for the portal's platform API.
// Invalidate is called after the portal rejects a token, so the next Token
// call returns a fresh one.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Invalidate()
}

// tokenRefreshMargin is how long before expiry a cached token is replaced.
const tokenRefreshMargin = 30 * time.Second

// ClientCredentials obtains tokens from a Keycloak token endpoint with the
// OAuth 2.0 client-credentials grant and caches each one until shortly
// before it expires. It is safe for concurrent use; concurrent callers share
// one request.
type ClientCredentials struct {
	// TokenURL is the realm's token endpoint, for example
	// https://auth.health.go.ug/realms/MoH/protocol/openid-connect/token.
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes are requested as the space-separated scope parameter.
	Scopes []string
	// HTTPClient sends token requests; nil uses a client with a 10 s timeout
	// that does not follow redirects.
	HTTPClient *http.Client
	// Now overrides the clock (tests).
	Now func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// Token returns the cached token, or requests a new one when there is none or
// it expires within 30 seconds.
func (c *ClientCredentials) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.token != "" && now.Add(tokenRefreshMargin).Before(c.expires) {
		return c.token, nil
	}
	token, lifetime, err := c.fetch(ctx)
	if err != nil {
		return "", err
	}
	c.token, c.expires = token, now.Add(lifetime)
	return token, nil
}

// Invalidate drops the cached token.
func (c *ClientCredentials) Invalidate() {
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
}

func (c *ClientCredentials) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// ErrToken reports a failure to obtain an access token.
var ErrToken = errors.New("platform client: could not obtain an access token")

func (c *ClientCredentials) fetch(ctx context.Context) (string, time.Duration, error) {
	if err := checkEndpoint(c.TokenURL); err != nil {
		return "", 0, fmt.Errorf("%w: token URL: %v", ErrToken, err)
	}
	if c.ClientID == "" || c.ClientSecret == "" {
		return "", 0, fmt.Errorf("%w: client ID and secret are required", ErrToken)
	}
	// client_secret_post: Keycloak accepts it, and it avoids the encoding
	// ambiguity of secrets with special characters in Basic authentication.
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {c.ClientID}, "client_secret": {c.ClientSecret}}
	if len(c.Scopes) > 0 {
		form.Set("scope", strings.Join(c.Scopes, " "))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", ErrToken, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: refuseRedirect}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", ErrToken, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", ErrToken, err)
	}
	if resp.StatusCode != http.StatusOK {
		// Keycloak's error body names the OAuth error; it never contains the secret.
		var oauth struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &oauth)
		if oauth.Error != "" {
			return "", 0, fmt.Errorf("%w: HTTP %d %s", ErrToken, resp.StatusCode, oauth.Error)
		}
		return "", 0, fmt.Errorf("%w: HTTP %d", ErrToken, resp.StatusCode)
	}
	if len(body) > maxResponseBytes {
		return "", 0, fmt.Errorf("%w: token response too large", ErrToken)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return "", 0, fmt.Errorf("%w: malformed token response", ErrToken)
	}
	if token.TokenType != "" && !strings.EqualFold(token.TokenType, "bearer") {
		return "", 0, fmt.Errorf("%w: unexpected token type %q", ErrToken, token.TokenType)
	}
	lifetime := time.Duration(token.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = time.Minute
	}
	return token.AccessToken, lifetime, nil
}

// StaticToken is a TokenSource for a token obtained elsewhere. Invalidate is
// a no-op, so a rejected token is reported instead of retried.
type StaticToken string

// Token returns the token.
func (t StaticToken) Token(context.Context) (string, error) {
	if t == "" {
		return "", fmt.Errorf("%w: empty static token", ErrToken)
	}
	return string(t), nil
}

// Invalidate does nothing.
func (StaticToken) Invalidate() {}
