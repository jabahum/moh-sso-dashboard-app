package identityreconcile

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// AccessToken supplies a short-lived Keycloak service-account access token.
type AccessToken func(context.Context) (string, error)

// Transport limits from the identity-sync contract.
const (
	contractBasePath    = "/api/v1/internal/identity-sync"
	maxResponseBytes    = 256 << 10
	requestTimeout      = 15 * time.Second
	maxAttempts         = 5
	retryBaseDelay      = 500 * time.Millisecond
	retryMaxDelay       = 10 * time.Second
	maxRetryAfterHonour = 60 * time.Second
)

// Sleeper waits for d or until ctx ends. Tests replace it to avoid real delays.
type Sleeper func(ctx context.Context, d time.Duration) error

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// HTTPAccountsConfig configures a connector for one registered application.
type HTTPAccountsConfig struct {
	// Application is the application's Keycloak client ID. A 1.1 descriptor
	// must name exactly this application.
	Application string
	// Origin is the registered HTTPS origin (scheme and host[:port] only).
	Origin string
	// LegacyAccountIDFormat is the registry's account ID format, used when the
	// application implements contract 1.0 and has no descriptor.
	LegacyAccountIDFormat identitysync.AccountIDFormat
	Token                 AccessToken
}

// HTTPAccounts is the dashboard's client for the identity-sync application
// contract. It supports 1.1 and falls back to 1.0 when GET / returns 404.
type HTTPAccounts struct {
	application  string
	legacyFormat identitysync.AccountIDFormat
	origin       string
	token        AccessToken
	client       *http.Client
	// sleep and jitter are injectable so retry tests run without real delays.
	sleep  Sleeper
	jitter func() float64

	mu         sync.Mutex
	descriptor *identitysync.Descriptor
}

var (
	_ Accounts                    = (*HTTPAccounts)(nil)
	_ identitysync.SessionRevoker = (*HTTPAccounts)(nil)
)

// ErrContractViolation reports a response that does not follow the contract.
var ErrContractViolation = errors.New("identity sync response violates the contract")

// NewHTTPAccounts builds a connector for one registered application.
func NewHTTPAccounts(cfg HTTPAccountsConfig) (*HTTPAccounts, error) {
	origin, ok := normalizeOrigin(cfg.Origin)
	if !ok || cfg.Token == nil {
		return nil, errors.New("Identity sync requires an HTTPS origin and a service access token source")
	}
	if cfg.Application == "" {
		return nil, errors.New("identity sync connector requires the application client ID")
	}
	switch cfg.LegacyAccountIDFormat {
	case identitysync.AccountIDUUID, identitysync.AccountIDPositiveInteger, identitysync.AccountIDOpaque:
	default:
		return nil, fmt.Errorf("unknown account ID format %q", cfg.LegacyAccountIDFormat)
	}
	return &HTTPAccounts{
		application:  cfg.Application,
		legacyFormat: cfg.LegacyAccountIDFormat,
		origin:       origin,
		token:        cfg.Token,
		client: &http.Client{
			Timeout: requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		sleep:  sleepContext,
		jitter: rand.Float64,
	}, nil
}

// NewPOES builds the deprecated, env-configured POES connector (positive-integer IDs).
func NewPOES(origin string, token AccessToken) (*HTTPAccounts, error) {
	return NewHTTPAccounts(HTTPAccountsConfig{Application: "poesgo", Origin: origin, LegacyAccountIDFormat: identitysync.AccountIDPositiveInteger, Token: token})
}

// NewCLIMS builds the deprecated, env-configured CLIMS connector (UUID IDs).
func NewCLIMS(origin string, token AccessToken) (*HTTPAccounts, error) {
	return NewHTTPAccounts(HTTPAccountsConfig{Application: "clims", Origin: origin, LegacyAccountIDFormat: identitysync.AccountIDUUID, Token: token})
}

// normalizeOrigin accepts only https://host[:port] with an optional trailing slash.
func normalizeOrigin(origin string) (string, bool) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		strings.TrimSuffix(origin, "/") != "https://"+u.Host {
		return "", false
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", false
		}
	}
	return "https://" + u.Host, true
}

// Describe reads the application's contract descriptor (GET /) and caches it.
// A 404 means the application implements 1.0; the result is then the legacy
// descriptor with the registry's account ID format.
func (p *HTTPAccounts) Describe(ctx context.Context) (identitysync.Descriptor, error) {
	resp, err := p.do(ctx, http.MethodGet, "", nil)
	if err != nil {
		return identitysync.Descriptor{}, fmt.Errorf("read identity sync descriptor: %w", err)
	}
	defer resp.Body.Close()
	var descriptor identitysync.Descriptor
	switch resp.StatusCode {
	case http.StatusNotFound:
		descriptor = identitysync.LegacyDescriptor(p.application, p.legacyFormat)
	case http.StatusOK:
		if err := decodeBody(resp, &descriptor); err != nil {
			return identitysync.Descriptor{}, fmt.Errorf("decode identity sync descriptor: %w", err)
		}
	default:
		return identitysync.Descriptor{}, fmt.Errorf("read identity sync descriptor: HTTP %d", resp.StatusCode)
	}
	if err := descriptor.Validate(p.application); err != nil {
		return identitysync.Descriptor{}, fmt.Errorf("%w: %v", ErrContractViolation, err)
	}
	p.mu.Lock()
	p.descriptor = &descriptor
	p.mu.Unlock()
	return descriptor, nil
}

func (p *HTTPAccounts) cachedDescriptor(ctx context.Context) (identitysync.Descriptor, error) {
	p.mu.Lock()
	cached := p.descriptor
	p.mu.Unlock()
	if cached != nil {
		return *cached, nil
	}
	return p.Describe(ctx)
}

// atLeast11 reports whether a validated descriptor version is 1.1 or newer.
func atLeast11(d identitysync.Descriptor) bool {
	minor, err := strconv.Atoi(strings.TrimPrefix(d.Version, "1."))
	return err == nil && minor >= 1
}

type wireLink struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

type wireAccount struct {
	ID    string    `json:"id"`
	Email string    `json:"email"`
	Link  *wireLink `json:"link"`
}

func (p *HTTPAccounts) ListEligibleAccounts(ctx context.Context, cursor string, limit int) (identitysync.AccountPage, error) {
	if limit < 1 || limit > identitysync.MaxPageSize {
		return identitysync.AccountPage{}, errors.New("invalid identity sync account page")
	}
	d, err := p.cachedDescriptor(ctx)
	if err != nil {
		return identitysync.AccountPage{}, err
	}
	v11 := atLeast11(d)
	validCursor := func(c string) bool {
		if v11 {
			return identitysync.ValidCursor(c)
		}
		return c == "" || identitysync.ValidAccountID(d.AccountIDFormat, c)
	}
	if !validCursor(cursor) {
		return identitysync.AccountPage{}, errors.New("invalid identity sync account page")
	}
	query := url.Values{}
	if cursor != "" {
		query.Set("after", cursor)
	}
	requested := min(limit, d.MaxPageSize)
	if v11 {
		query.Set("limit", strconv.Itoa(requested))
	}
	path := "/accounts"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	resp, err := p.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return identitysync.AccountPage{}, fmt.Errorf("list identity sync accounts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return identitysync.AccountPage{}, fmt.Errorf("list identity sync accounts: HTTP %d", resp.StatusCode)
	}
	var wire struct {
		Accounts   []wireAccount `json:"accounts"`
		NextCursor string        `json:"nextCursor"`
	}
	if err := decodeBody(resp, &wire); err != nil {
		return identitysync.AccountPage{}, fmt.Errorf("decode identity sync account page: %w", err)
	}
	if len(wire.Accounts) > identitysync.MaxPageSize || !validCursor(wire.NextCursor) || (v11 && len(wire.Accounts) > requested) {
		return identitysync.AccountPage{}, fmt.Errorf("%w: invalid account page", ErrContractViolation)
	}
	accounts := make([]identitysync.Account, 0, len(wire.Accounts))
	for _, w := range wire.Accounts {
		if !identitysync.ValidAccountID(d.AccountIDFormat, w.ID) {
			return identitysync.AccountPage{}, fmt.Errorf("%w: invalid account ID", ErrContractViolation)
		}
		account := identitysync.Account{ID: w.ID, Email: w.Email}
		if v11 && w.Link != nil {
			link := identitysync.Identity{Issuer: w.Link.Issuer, Subject: w.Link.Subject}
			if !identitysync.ValidIdentity(link, link.Issuer) {
				return identitysync.AccountPage{}, fmt.Errorf("%w: invalid account link", ErrContractViolation)
			}
			account.Link = link
		}
		accounts = append(accounts, account)
	}
	next := wire.NextCursor
	// A 1.0 application has a fixed page size and its cursor is the last
	// account ID, so a longer page can be cut client-side.
	if !v11 && len(accounts) > limit {
		accounts = accounts[:limit]
		next = accounts[limit-1].ID
	}
	return identitysync.AccountPage{Accounts: accounts, NextCursor: next}, nil
}

func (p *HTTPAccounts) LinkIdentity(ctx context.Context, accountID string, identity identitysync.Identity) error {
	d, err := p.cachedDescriptor(ctx)
	if err != nil {
		return err
	}
	if !identitysync.ValidAccountID(d.AccountIDFormat, accountID) || !identitysync.ValidIdentity(identity, identity.Issuer) {
		return errors.New("invalid application identity link")
	}
	body, err := json.Marshal(map[string]string{"issuer": identity.Issuer, "subject": identity.Subject})
	if err != nil {
		return err
	}
	resp, err := p.do(ctx, http.MethodPut, "/accounts/"+url.PathEscape(accountID)+"/identity", body)
	if err != nil {
		return fmt.Errorf("link application identity: %w", err)
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

// RevokeSessions ends every local session of the account linked to identity
// (POST /sessions/revoke, contract 1.1). It returns identitysync.ErrUnsupported
// when the application's descriptor does not offer sessions.revoke.
func (p *HTTPAccounts) RevokeSessions(ctx context.Context, identity identitysync.Identity) error {
	d, err := p.cachedDescriptor(ctx)
	if err != nil {
		return err
	}
	if !d.Supports(identitysync.CapabilityRevokeSessions) {
		return identitysync.ErrUnsupported
	}
	if !identitysync.ValidIdentity(identity, identity.Issuer) {
		return errors.New("invalid identity for session revocation")
	}
	body, err := json.Marshal(map[string]string{"issuer": identity.Issuer, "subject": identity.Subject})
	if err != nil {
		return err
	}
	resp, err := p.do(ctx, http.MethodPost, "/sessions/revoke", body)
	if err != nil {
		return fmt.Errorf("revoke application sessions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("revoke application sessions: HTTP %d", resp.StatusCode)
	}
	return nil
}

// do sends one contract request with the contract's retry policy: network
// errors, timeouts, 429, 502, 503 and 504 are retried up to maxAttempts with
// exponential backoff and jitter, honouring Retry-After. Other statuses are
// returned to the caller unchanged and never retried.
func (p *HTTPAccounts) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var lastErr error
	for attempt := 1; ; attempt++ {
		resp, err := p.attempt(ctx, method, path, body)
		var wait time.Duration
		switch {
		case err != nil:
			if ctx.Err() != nil || !retryableError(err) {
				return nil, err
			}
			lastErr = err
		case retryableStatus(resp.StatusCode):
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			wait = retryAfter(resp.Header.Get("Retry-After"), time.Now())
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
			resp.Body.Close()
			if wait > maxRetryAfterHonour {
				return nil, fmt.Errorf("%w (Retry-After exceeds %s)", lastErr, maxRetryAfterHonour)
			}
		default:
			return resp, nil
		}
		if attempt >= maxAttempts {
			return nil, fmt.Errorf("giving up after %d attempts: %w", attempt, lastErr)
		}
		if err := p.sleep(ctx, max(wait, p.backoff(attempt))); err != nil {
			return nil, err
		}
	}
}

func (p *HTTPAccounts) attempt(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	token, err := p.token(ctx)
	if err != nil {
		return nil, fmt.Errorf("get identity sync service access token: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("empty identity sync service access token")
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.origin+contractBasePath+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &transportError{err: err}
	}
	return resp, nil
}

// transportError marks failures of the HTTP exchange itself (as opposed to
// token or request construction errors, which are not retried).
type transportError struct{ err error }

func (e *transportError) Error() string { return e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

func retryableError(err error) bool {
	var transport *transportError
	if !errors.As(err, &transport) {
		return false
	}
	// A certificate the dashboard does not trust will not become trusted on retry.
	var certErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	if errors.As(err, &certErr) || errors.As(err, &unknownAuthority) || errors.As(err, &hostnameErr) || errors.As(err, &invalidCert) {
		return false
	}
	// Everything else from the exchange is a network error or timeout.
	return true
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoff returns the delay before retry number attempt (1-based): exponential
// growth from retryBaseDelay, capped at retryMaxDelay, with jitter in [d/2, d).
func (p *HTTPAccounts) backoff(attempt int) time.Duration {
	d := retryBaseDelay << (attempt - 1)
	if d <= 0 || d > retryMaxDelay {
		d = retryMaxDelay
	}
	return d/2 + time.Duration(p.jitter()*float64(d/2))
}

// retryAfter parses a Retry-After header (delay seconds or HTTP date).
func retryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		if seconds > int64(24*time.Hour/time.Second) {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

// decodeBody decodes a JSON body of at most maxResponseBytes.
func decodeBody(resp *http.Response, target any) error {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return fmt.Errorf("%w: response exceeds %d bytes", ErrContractViolation, maxResponseBytes)
	}
	return json.Unmarshal(data, target)
}
