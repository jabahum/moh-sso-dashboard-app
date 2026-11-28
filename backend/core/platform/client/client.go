// Package client calls the MoH Integrated Health Portal's platform API from an
// application backend: caller identity, readiness, notifications and per-user
// access decisions.
//
// It obtains and caches client-credentials tokens, sets a timeout on every
// request, retries only requests that are safe to repeat, refuses redirects
// and plain HTTP (except to loopback addresses, for tests), limits response
// sizes and reports failures as *Error values that match the sentinel errors
// with errors.Is. It uses only the standard library.
//
// The contract is backend/core/platform/contract/platform.openapi.yaml and
// the guide docs/integrations/platform-api.md.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes   = 1 << 20
	defaultTimeout     = 10 * time.Second
	defaultMaxAttempts = 3
	maxRetryAfter      = 10 * time.Second
	userAgent          = "moh-platform-client-go/1"
)

// Config configures a Client.
type Config struct {
	// PortalURL is the portal's origin, for example https://portals.health.go.ug.
	PortalURL string
	// Tokens supplies machine-client tokens, usually a *ClientCredentials.
	// It may be nil when only Access (which uses the user's token) is called.
	Tokens TokenSource
	// HTTPClient sends portal requests. Nil uses a client that does not follow
	// redirects. A supplied client's redirect policy is replaced the same way.
	HTTPClient *http.Client
	// Timeout bounds each attempt. Default 10 s.
	Timeout time.Duration
	// MaxAttempts bounds attempts for requests that are safe to repeat.
	// Default 3; 1 disables retries.
	MaxAttempts int
	// UserAgent is sent before the client's own product token.
	UserAgent string
}

// Client calls the platform API. It is safe for concurrent use.
type Client struct {
	base        *url.URL
	tokens      TokenSource
	http        *http.Client
	timeout     time.Duration
	maxAttempts int
	userAgent   string
	sleep       func(context.Context, time.Duration) error
}

// New validates cfg and returns a Client.
func New(cfg Config) (*Client, error) {
	if err := checkEndpoint(cfg.PortalURL); err != nil {
		return nil, fmt.Errorf("platform client: portal URL: %w", err)
	}
	base, _ := url.Parse(strings.TrimRight(cfg.PortalURL, "/"))
	if base.Path != "" || base.RawQuery != "" {
		return nil, errors.New("platform client: portal URL must be an origin without a path or query")
	}
	httpClient := &http.Client{}
	if cfg.HTTPClient != nil {
		copied := *cfg.HTTPClient
		httpClient = &copied
	}
	httpClient.CheckRedirect = refuseRedirect
	c := &Client{
		base: base, tokens: cfg.Tokens, http: httpClient,
		timeout: cfg.Timeout, maxAttempts: cfg.MaxAttempts, userAgent: userAgent,
		sleep: sleepContext,
	}
	if c.timeout <= 0 {
		c.timeout = defaultTimeout
	}
	if c.maxAttempts <= 0 {
		c.maxAttempts = defaultMaxAttempts
	}
	if cfg.UserAgent != "" {
		c.userAgent = cfg.UserAgent + " " + userAgent
	}
	return c, nil
}

// Identity returns the calling machine client's application, client ID,
// subject and effective scopes. Requires identity:read.
func (c *Client) Identity(ctx context.Context) (Identity, error) {
	var out Identity
	_, err := c.do(ctx, call{method: http.MethodGet, path: "/api/v1/platform/identity", retry: true}, &out)
	return out, err
}

// Readiness reports whether the portal and its background workers are
// healthy. A 503 with a readiness body returns Ready false and no error.
// Requires identity:read.
func (c *Client) Readiness(ctx context.Context) (Readiness, error) {
	var out Readiness
	_, err := c.do(ctx, call{method: http.MethodGet, path: "/api/v1/platform/readiness", retry: true, bodyOn503: true}, &out)
	return out, err
}

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// Notify queues one email notification. key must be unique per notification
// (1–128 characters: letters, digits, '.', '_', ':' and '-', starting with a
// letter or digit); repeating a key with the same notification returns the
// original with Replayed true. Requires notifications:write.
func (c *Client) Notify(ctx context.Context, key string, n Notification) (NotificationStatus, error) {
	var out NotificationStatus
	if !idempotencyKeyPattern.MatchString(key) {
		return out, invalid("the idempotency key must be 1-128 characters of letters, digits, '.', '_', ':' or '-', starting with a letter or digit")
	}
	if err := n.validate(); err != nil {
		return out, err
	}
	body, err := json.Marshal(n)
	if err != nil {
		return out, invalid(err.Error())
	}
	_, err = c.do(ctx, call{
		method: http.MethodPost, path: "/api/v1/platform/notifications", body: body,
		headers: map[string]string{"Idempotency-Key": key}, retry: true,
	}, &out)
	return out, err
}

// Notification returns the delivery status of a notification this
// application sent. Requires notifications:read.
func (c *Client) Notification(ctx context.Context, id string) (NotificationStatus, error) {
	var out NotificationStatus
	if id == "" || strings.ContainsAny(id, "/?#%") {
		return out, invalid("a notification ID is required")
	}
	_, err := c.do(ctx, call{method: http.MethodGet, path: "/api/v1/platform/notifications/" + id, retry: true}, &out)
	return out, err
}

// Access asks the portal for a signed-in user's current roles and permissions
// in the application clientID, using the user's own access token (never the
// machine token). Fail closed when it returns an error.
func (c *Client) Access(ctx context.Context, userToken, clientID string) (UserAccess, error) {
	var out UserAccess
	if userToken == "" {
		return out, invalid("the user's access token is required")
	}
	if clientID == "" || strings.ContainsAny(clientID, "/?#%") {
		return out, invalid("an application client ID is required")
	}
	_, err := c.do(ctx, call{
		method: http.MethodGet, path: "/api/v1/platform/applications/" + url.PathEscape(clientID) + "/access",
		tokens: StaticToken(userToken), retry: true,
	}, &out)
	return out, err
}

// ---------------------------------------------------------------- transport

type call struct {
	method, path string
	body         []byte
	headers      map[string]string
	// tokens overrides the client's machine token source (Access).
	tokens TokenSource
	// retry marks a request as safe to repeat (GET, or POST with an idempotency key).
	retry bool
	// bodyOn503 decodes a success-shaped 503 body instead of failing.
	bodyOn503 bool
}

// do sends the call with the retry policy and decodes the envelope's data
// into out. It returns the HTTP status.
func (c *Client) do(ctx context.Context, cl call, out any) (int, error) {
	tokens := cl.tokens
	if tokens == nil {
		tokens = c.tokens
	}
	if tokens == nil {
		return 0, fmt.Errorf("%w: no token source configured", ErrToken)
	}
	refreshed := false
	for attempt := 1; ; attempt++ {
		token, err := tokens.Token(ctx)
		if err != nil {
			return 0, err
		}
		status, header, body, err := c.attempt(ctx, cl, token)
		if err == nil && status == http.StatusUnauthorized && !refreshed {
			// The portal rejected the token (expired or revoked early): get a
			// new one once. A StaticToken stays the same, so it fails again.
			if _, static := tokens.(StaticToken); !static {
				tokens.Invalidate()
				refreshed = true
				attempt--
				continue
			}
		}
		var retryAfter time.Duration
		if err == nil {
			if status >= 200 && status < 300 || status == http.StatusServiceUnavailable && cl.bodyOn503 && decodes(body) {
				if derr := decodeData(body, out); derr != nil {
					return status, &Error{Status: status, Code: "INVALID_RESPONSE", Message: derr.Error(), sentinel: ErrUnavailable}
				}
				return status, nil
			}
			apiErr := errorFrom(status, header, body)
			if !cl.retry || attempt >= c.maxAttempts || !retryableStatus(status) {
				return status, apiErr
			}
			if status == http.StatusTooManyRequests {
				if apiErr.RetryAfter > maxRetryAfter {
					return status, apiErr
				}
				retryAfter = apiErr.RetryAfter
			}
			err = apiErr
		} else if ctx.Err() != nil || !cl.retry || attempt >= c.maxAttempts {
			return 0, err
		}
		if serr := c.sleep(ctx, max(retryAfter, backoff(attempt))); serr != nil {
			return 0, err
		}
	}
}

// attempt sends one request and returns status, headers and a size-limited body.
func (c *Client) attempt(ctx context.Context, cl call, token string) (int, http.Header, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var reader io.Reader
	if cl.body != nil {
		reader = bytes.NewReader(cl.body)
	}
	req, err := http.NewRequestWithContext(ctx, cl.method, c.base.String()+cl.path, reader)
	if err != nil {
		return 0, nil, nil, &Error{Code: "INVALID_REQUEST", Message: err.Error(), sentinel: ErrInvalid}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if cl.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range cl.headers {
		req.Header.Set(name, value)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, nil, &Error{Code: "NETWORK_ERROR", Message: networkMessage(err), sentinel: ErrUnavailable, cause: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return 0, nil, nil, &Error{Status: resp.StatusCode, Code: "NETWORK_ERROR", Message: networkMessage(err), sentinel: ErrUnavailable, cause: err}
	}
	if len(body) > maxResponseBytes {
		return 0, nil, nil, &Error{Status: resp.StatusCode, Code: "INVALID_RESPONSE", Message: "response larger than 1 MiB", sentinel: ErrUnavailable}
	}
	return resp.StatusCode, resp.Header, body, nil
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoff is exponential with full jitter: up to 200 ms, 400 ms, 800 ms … 2 s.
func backoff(attempt int) time.Duration {
	ceiling := min(200*time.Millisecond<<(attempt-1), 2*time.Second)
	return time.Duration(rand.Int64N(int64(ceiling)) + 1)
}

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

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta struct {
		RequestID string `json:"requestId"`
	} `json:"meta"`
}

func decodes(body []byte) bool {
	var env envelope
	return json.Unmarshal(body, &env) == nil && len(env.Data) > 0 && string(env.Data) != "null"
}

func decodeData(body []byte, out any) error {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("malformed response: %v", err)
	}
	if out == nil {
		return nil
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return errors.New("response has no data")
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("unexpected response data: %v", err)
	}
	return nil
}

func errorFrom(status int, header http.Header, body []byte) *Error {
	e := &Error{Status: status, sentinel: sentinelFor(status)}
	var env envelope
	if json.Unmarshal(body, &env) == nil {
		if env.Error != nil {
			e.Code, e.Message = env.Error.Code, env.Error.Message
		}
		e.RequestID = env.Meta.RequestID
	}
	if e.Code == "" {
		e.Code = "HTTP_" + strconv.Itoa(status)
	}
	if e.Message == "" {
		e.Message = http.StatusText(status)
	}
	if seconds, err := strconv.Atoi(header.Get("Retry-After")); err == nil && seconds > 0 {
		e.RetryAfter = time.Duration(seconds) * time.Second
	}
	return e
}

func networkMessage(err error) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout():
		return "the portal did not answer in time"
	}
	return "the portal could not be reached"
}

// refuseRedirect returns the 3xx response itself, which is reported as an
// unexpected answer: following it could send the token to another host.
func refuseRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// checkEndpoint accepts HTTPS URLs, and HTTP only to loopback addresses.
func checkEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return errors.New("must be an absolute URL with a host and no credentials")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if ip := net.ParseIP(host); host == "localhost" || ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	return errors.New("must use HTTPS (HTTP is allowed only for loopback addresses)")
}
