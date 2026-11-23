package identityreconcile

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

// ClientCredentials returns an AccessToken source that mints short-lived tokens for an
// application's dedicated identity-sync Keycloak service account and caches each token
// until shortly before it expires.
func ClientCredentials(tokenEndpoint, clientID, clientSecret string) (AccessToken, error) {
	return newClientCredentials(tokenEndpoint, clientID, clientSecret, &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
}

func newClientCredentials(tokenEndpoint, clientID, clientSecret string, client *http.Client) (AccessToken, error) {
	u, err := url.Parse(tokenEndpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("identity sync token endpoint must be an HTTPS URL")
	}
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("identity sync service client ID and secret are required")
	}
	source := &clientCredentials{
		endpoint:     tokenEndpoint,
		clientID:     clientID,
		clientSecret: clientSecret,
		client:       client,
	}
	return source.get, nil
}

type clientCredentials struct {
	endpoint, clientID, clientSecret string
	client                           *http.Client
	mu                               sync.Mutex
	token                            string
	expiresAt                        time.Time
}

func (c *clientCredentials) get(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expiresAt.Add(-30*time.Second)) {
		return c.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create identity sync token request: %w", err)
	}
	req.SetBasicAuth(url.QueryEscape(c.clientID), url.QueryEscape(c.clientSecret))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("identity sync token request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("identity sync token request returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode identity sync token response: %w", err)
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 || !strings.EqualFold(result.TokenType, "Bearer") {
		return "", errors.New("invalid identity sync token response")
	}
	c.token, c.expiresAt = result.AccessToken, time.Now().Add(time.Duration(result.ExpiresIn)*time.Second)
	return c.token, nil
}
