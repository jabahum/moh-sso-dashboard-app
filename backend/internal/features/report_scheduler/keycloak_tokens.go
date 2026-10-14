package report_scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Background jobs use a dedicated Keycloak service account, never a saved user token.
type keycloakTokenProvider struct {
	endpoint, clientID, clientSecret string
	client                           *http.Client
	mu                               sync.Mutex
	token                            string
	expiresAt                        time.Time
}

func (p *keycloakTokenProvider) configured() bool {
	return p.endpoint != "" && p.clientID != "" && p.clientSecret != ""
}

func (p *keycloakTokenProvider) get(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.expiresAt.Add(-30*time.Second)) {
		return p.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {p.clientID}, "client_secret": {p.clientSecret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create Health BI Keycloak token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Health BI Keycloak token request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Health BI Keycloak token request returned %s", resp.Status)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Health BI Keycloak token response: %w", err)
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 || !strings.EqualFold(result.TokenType, "Bearer") {
		return "", fmt.Errorf("invalid Health BI Keycloak token response")
	}
	p.token, p.expiresAt = result.AccessToken, time.Now().Add(time.Duration(result.ExpiresIn)*time.Second)
	return p.token, nil
}

func (p *keycloakTokenProvider) invalidate(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token == token {
		p.token = ""
		p.expiresAt = time.Time{}
	}
}
