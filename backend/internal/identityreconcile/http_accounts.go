package identityreconcile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// AccessToken supplies a short-lived Keycloak service-account access token.
type AccessToken func(context.Context) (string, error)

type HTTPAccounts struct {
	validID func(string) bool
	origin  string
	token   AccessToken
	client  *http.Client
}

var _ Accounts = (*HTTPAccounts)(nil)

func NewPOES(origin string, token AccessToken) (*HTTPAccounts, error) {
	return newHTTPAccounts(origin, token, positiveID)
}

func NewCLIMS(origin string, token AccessToken) (*HTTPAccounts, error) {
	return newHTTPAccounts(origin, token, func(raw string) bool {
		id, err := uuid.Parse(raw)
		return err == nil && id != uuid.Nil && id.String() == raw
	})
}

func newHTTPAccounts(origin string, token AccessToken, validID func(string) bool) (*HTTPAccounts, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		strings.TrimSuffix(origin, "/") != "https://"+u.Host || token == nil {
		return nil, errors.New("Identity sync requires an HTTPS origin and a service access token source")
	}
	return &HTTPAccounts{
		validID: validID,
		origin:  "https://" + u.Host,
		token:   token,
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (p *HTTPAccounts) ListEligibleAccounts(ctx context.Context, cursor string, limit int) (identitysync.AccountPage, error) {
	if limit < 1 || limit > 100 || (cursor != "" && !p.validID(cursor)) {
		return identitysync.AccountPage{}, errors.New("invalid identity sync account page")
	}
	path := "/accounts"
	if cursor != "" {
		path += "?" + url.Values{"after": {cursor}}.Encode()
	}
	resp, err := p.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return identitysync.AccountPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return identitysync.AccountPage{}, fmt.Errorf("list identity sync accounts: HTTP %d", resp.StatusCode)
	}
	var wire struct {
		Accounts   []identitysync.Account
		NextCursor string
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&wire); err != nil {
		return identitysync.AccountPage{}, fmt.Errorf("decode identity sync account page: %w", err)
	}
	if len(wire.Accounts) > 100 || (wire.NextCursor != "" && !p.validID(wire.NextCursor)) {
		return identitysync.AccountPage{}, errors.New("invalid identity sync account response")
	}
	for _, account := range wire.Accounts {
		if !p.validID(account.ID) {
			return identitysync.AccountPage{}, errors.New("invalid identity sync account ID")
		}
	}
	if len(wire.Accounts) > limit {
		wire.Accounts = wire.Accounts[:limit]
		wire.NextCursor = wire.Accounts[limit-1].ID
	}
	return identitysync.AccountPage{Accounts: wire.Accounts, NextCursor: wire.NextCursor}, nil
}

func (p *HTTPAccounts) LinkIdentity(ctx context.Context, accountID string, identity identitysync.Identity) error {
	if !p.validID(accountID) || identity.Issuer == "" || strings.TrimSpace(identity.Subject) == "" {
		return errors.New("invalid application identity link")
	}
	body, err := json.Marshal(map[string]string{"issuer": identity.Issuer, "subject": identity.Subject})
	if err != nil {
		return err
	}
	resp, err := p.request(ctx, http.MethodPut, "/accounts/"+accountID+"/identity", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return identitysync.ErrAccountNotFound
	case http.StatusConflict:
		return identitysync.ErrLinkConflict
	default:
		return fmt.Errorf("link application identity: HTTP %d", resp.StatusCode)
	}
}

func (p *HTTPAccounts) request(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	token, err := p.token(ctx)
	if err != nil {
		return nil, fmt.Errorf("get identity sync service access token: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("empty identity sync service access token")
	}
	req, err := http.NewRequestWithContext(ctx, method, p.origin+"/api/v1/internal/identity-sync"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return p.client.Do(req)
}

func positiveID(raw string) bool {
	id, err := strconv.ParseInt(raw, 10, 64)
	return err == nil && id > 0 && strconv.FormatInt(id, 10) == raw
}
