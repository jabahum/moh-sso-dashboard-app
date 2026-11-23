// Package adapter implements the application side of the identity-sync
// contract 1.1 (docs/integrations/identity-sync-contract.md) as a net/http
// handler.
//
// The application supplies its account storage (Store), its own OIDC access
// token verification (TokenVerifier) and its configuration. The handler does
// everything else the contract requires: bearer authorization with the exact
// 401/403 split, request validation, status codes, Cache-Control: no-store,
// request and response size limits, and the optional JSON error body.
//
// The package uses only the Go standard library.
package adapter

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

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// BasePath is the path under the application's origin where the contract lives.
const BasePath = "/api/v1/internal/identity-sync"

// IdentitySyncRole is the client role the dashboard's sync client must hold on
// the application's Keycloak client.
const IdentitySyncRole = "identity_sync"

// ProbeEnvVar enables the test-only session probe when set to exactly "1".
const ProbeEnvVar = "IDENTITY_SYNC_TEST_PROBE"

// Size limits from the contract.
const (
	MaxRequestBody  = 4 << 10
	MaxResponseBody = 256 << 10
	maxBearerToken  = 16 << 10
	maxSessionToken = 4 << 10
)

// Errors a Store, TokenVerifier or SessionProbe may return to select a status.
var (
	// ErrInvalidCursor reports a cursor the store cannot resume from (400).
	ErrInvalidCursor = errors.New("invalid identity-sync cursor")
	// ErrRateLimited asks the dashboard to retry later (429 with Retry-After).
	ErrRateLimited = errors.New("identity-sync rate limited")
	// ErrUnavailable reports a temporary failure (503 with Retry-After). A
	// TokenVerifier returns it when it cannot reach the realm's keys, so the
	// dashboard retries instead of treating the token as invalid.
	ErrUnavailable = errors.New("identity-sync temporarily unavailable")
)

// Claims are the verified claims of a bearer access token.
type Claims struct {
	Issuer          string
	Audience        []string
	AuthorizedParty string // azp
	Type            string // typ
	ExpiresAt       time.Time
	NotBefore       time.Time // zero when the token has no nbf
	// ResourceAccess maps a client ID to resource_access[client].roles.
	ResourceAccess map[string][]string
}

// TokenVerifier verifies an access token with the application's own OIDC
// library. It MUST verify the RS256 signature against the realm's published
// keys and SHOULD check exp and nbf. It returns an error (any error except
// ErrUnavailable means 401) when the token cannot be verified. The handler
// checks issuer, expiry, audience, typ, azp and roles itself.
type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, rawToken string) (Claims, error)
}

// TokenVerifierFunc adapts a function to TokenVerifier.
type TokenVerifierFunc func(ctx context.Context, rawToken string) (Claims, error)

// VerifyAccessToken calls f.
func (f TokenVerifierFunc) VerifyAccessToken(ctx context.Context, rawToken string) (Claims, error) {
	return f(ctx, rawToken)
}

// Store is the application's account storage, scoped to one application and
// the configured issuer.
type Store interface {
	// ListEligibleAccounts returns at most limit eligible accounts after cursor
	// (empty means the first page), in stable ascending ID order, each with its
	// current link for the configured issuer (zero Identity when unlinked).
	// NextCursor is empty on the last page. Return ErrInvalidCursor for a cursor
	// it cannot resume from.
	ListEligibleAccounts(ctx context.Context, cursor string, limit int) (identitysync.AccountPage, error)

	// LinkIdentity checks eligibility and both uniqueness rules in one
	// transaction and never replaces a link. It returns nil when linked now or
	// already linked to exactly this identity, identitysync.ErrAccountNotFound
	// when the account does not exist or is not eligible, and
	// identitysync.ErrLinkConflict otherwise. It must not grant local roles.
	LinkIdentity(ctx context.Context, accountID string, identity identitysync.Identity) error

	// RevokeSessions ends every local session and refresh token of the account
	// linked to identity before returning. It returns nil when the identity is
	// not linked or has no sessions, and never removes the link or roles.
	RevokeSessions(ctx context.Context, identity identitysync.Identity) error
}

// SessionProbe is the test-only session probe (see the contract appendix
// "Test-only session probe"). It is mounted only when Config.ProbeEnabled.
type SessionProbe interface {
	// CreateTestSession creates a real local session for the eligible account
	// linked to identity and returns an opaque token that identifies it. It
	// returns identitysync.ErrIdentityNotLinked or identitysync.ErrAccountNotFound
	// when no eligible account is linked (404).
	CreateTestSession(ctx context.Context, identity identitysync.Identity) (string, error)
	// CheckTestSession reports whether the session is still usable, exactly as
	// the application's own session middleware would decide.
	CheckTestSession(ctx context.Context, sessionToken string) (bool, error)
}

// Config configures a Handler.
type Config struct {
	// Application is the application's Keycloak client ID (for example "clims").
	Application string
	// Issuer is the configured public issuer, compared exactly.
	Issuer string
	// SyncClientID is the dedicated identity-sync client (for example "clims-identity-sync").
	SyncClientID string
	// AccountIDFormat is the declared local account ID format.
	AccountIDFormat identitysync.AccountIDFormat
	// MaxPageSize is the largest page returned; 0 means identitysync.MaxPageSize.
	MaxPageSize int

	Store    Store
	Verifier TokenVerifier

	// ProbeEnabled mounts the test-only session probe. Set it only from
	// ProbeEnabledFromEnv in test environments; Probe must then be non-nil.
	ProbeEnabled bool
	Probe        SessionProbe

	// ClockSkew is tolerated on exp and nbf; 0 means 30 seconds.
	ClockSkew time.Duration
	// Now returns the current time; nil means time.Now.
	Now func() time.Time
	// OnError, when set, receives unexpected errors (never tokens).
	OnError func(r *http.Request, err error)
}

// ProbeEnabledFromEnv reports whether IDENTITY_SYNC_TEST_PROBE is exactly "1".
func ProbeEnabledFromEnv(getenv func(string) string) bool {
	return getenv(ProbeEnvVar) == "1"
}

// Handler serves the identity-sync contract. It accepts request paths either
// with the BasePath prefix or already stripped of it.
type Handler struct {
	cfg Config
}

// New validates cfg and returns a Handler.
func New(cfg Config) (*Handler, error) {
	switch {
	case cfg.Application == "":
		return nil, errors.New("adapter: Application is required")
	case !validIssuer(cfg.Issuer):
		return nil, errors.New("adapter: Issuer must be an absolute https URL")
	case cfg.SyncClientID == "" || cfg.SyncClientID == cfg.Application:
		return nil, errors.New("adapter: SyncClientID is required and must differ from Application")
	case cfg.AccountIDFormat != identitysync.AccountIDUUID &&
		cfg.AccountIDFormat != identitysync.AccountIDPositiveInteger &&
		cfg.AccountIDFormat != identitysync.AccountIDOpaque:
		return nil, fmt.Errorf("adapter: unknown AccountIDFormat %q", cfg.AccountIDFormat)
	case cfg.MaxPageSize < 0 || cfg.MaxPageSize > identitysync.MaxPageSize:
		return nil, fmt.Errorf("adapter: MaxPageSize must be 1-%d", identitysync.MaxPageSize)
	case cfg.Store == nil:
		return nil, errors.New("adapter: Store is required")
	case cfg.Verifier == nil:
		return nil, errors.New("adapter: Verifier is required")
	case cfg.ProbeEnabled && cfg.Probe == nil:
		return nil, errors.New("adapter: ProbeEnabled requires Probe")
	}
	if cfg.MaxPageSize == 0 {
		cfg.MaxPageSize = identitysync.MaxPageSize
	}
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = 30 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Handler{cfg: cfg}, nil
}

// Descriptor returns the contract descriptor the handler serves at GET /.
func (h *Handler) Descriptor() identitysync.Descriptor {
	return identitysync.Descriptor{
		Contract:        identitysync.ContractName,
		Version:         identitysync.Version11,
		Application:     h.cfg.Application,
		AccountIDFormat: h.cfg.AccountIDFormat,
		MaxPageSize:     h.cfg.MaxPageSize,
		Capabilities: []identitysync.Capability{
			identitysync.CapabilityListAccounts,
			identitysync.CapabilityLinkIdentity,
			identitysync.CapabilityRevokeSessions,
		},
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	path := r.URL.Path
	if rest, ok := strings.CutPrefix(path, BasePath); ok && (rest == "" || rest[0] == '/') {
		path = rest
	}
	if path == "" {
		path = "/"
	}

	if status := h.authorize(r); status != 0 {
		switch status {
		case http.StatusUnauthorized:
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		case http.StatusForbidden:
			w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope"`)
		case http.StatusServiceUnavailable:
			w.Header().Set("Retry-After", "5")
		}
		w.WriteHeader(status)
		return
	}

	switch {
	case path == "/":
		h.route(w, r, http.MethodGet, h.getDescriptor)
	case path == "/accounts":
		h.route(w, r, http.MethodGet, h.listAccounts)
	case accountIdentityPath(path):
		id := path[len("/accounts/") : len(path)-len("/identity")]
		h.route(w, r, http.MethodPut, func(w http.ResponseWriter, r *http.Request) { h.linkIdentity(w, r, id) })
	case path == "/sessions/revoke":
		h.route(w, r, http.MethodPost, h.revokeSessions)
	case h.cfg.ProbeEnabled && path == "/test/sessions":
		h.route(w, r, http.MethodPost, h.createTestSession)
	case h.cfg.ProbeEnabled && path == "/test/sessions/check":
		h.route(w, r, http.MethodGet, h.checkTestSession)
	default:
		writeError(w, http.StatusNotFound, "invalid_request", "not found")
	}
}

func (h *Handler) route(w http.ResponseWriter, r *http.Request, method string, next http.HandlerFunc) {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
		return
	}
	next(w, r)
}

// authorize returns 0 when the request may proceed, or the refusal status.
func (h *Handler) authorize(r *http.Request) int {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return http.StatusUnauthorized
	}
	scheme, token, ok := strings.Cut(values[0], " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > maxBearerToken ||
		strings.ContainsAny(token, " \t\r\n") {
		return http.StatusUnauthorized
	}
	claims, err := h.cfg.Verifier.VerifyAccessToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			h.reportError(r, fmt.Errorf("verify identity-sync token: %w", err))
			return http.StatusServiceUnavailable
		}
		return http.StatusUnauthorized
	}
	now := h.cfg.Now()
	switch {
	case claims.Issuer != h.cfg.Issuer:
		return http.StatusUnauthorized
	case claims.ExpiresAt.IsZero() || !now.Before(claims.ExpiresAt.Add(h.cfg.ClockSkew)):
		return http.StatusUnauthorized
	case !claims.NotBefore.IsZero() && now.Add(h.cfg.ClockSkew).Before(claims.NotBefore):
		return http.StatusUnauthorized
	case !contains(claims.Audience, h.cfg.Application):
		return http.StatusUnauthorized
	case claims.Type != "Bearer":
		return http.StatusForbidden
	case claims.AuthorizedParty != h.cfg.SyncClientID:
		return http.StatusForbidden
	case !contains(claims.ResourceAccess[h.cfg.Application], IdentitySyncRole):
		return http.StatusForbidden
	}
	return 0
}

func (h *Handler) getDescriptor(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, r, http.StatusOK, h.Descriptor())
}

type wireIdentity struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

type wireAccount struct {
	ID    string        `json:"id"`
	Email string        `json:"email"`
	Link  *wireIdentity `json:"link"`
}

type wirePage struct {
	Accounts   []wireAccount `json:"accounts"`
	NextCursor string        `json:"nextCursor"`
}

func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query["after"]) > 1 || len(query["limit"]) > 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid query")
		return
	}
	cursor := ""
	if values, ok := query["after"]; ok {
		cursor = values[0]
		if cursor == "" || !identitysync.ValidCursor(cursor) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid after cursor")
			return
		}
	}
	limit := identitysync.MaxPageSize
	if values, ok := query["limit"]; ok {
		n, err := strconv.Atoi(values[0])
		if err != nil || n < 1 || n > identitysync.MaxPageSize || strconv.Itoa(n) != values[0] {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be 1-100")
			return
		}
		limit = n
	}
	limit = min(limit, h.cfg.MaxPageSize)

	page, err := h.cfg.Store.ListEligibleAccounts(r.Context(), cursor, limit)
	if err != nil {
		if errors.Is(err, ErrInvalidCursor) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid after cursor")
			return
		}
		h.storeError(w, r, fmt.Errorf("list eligible accounts: %w", err))
		return
	}
	if err := h.checkPage(page, cursor, limit); err != nil {
		h.storeError(w, r, err)
		return
	}
	out := wirePage{Accounts: make([]wireAccount, 0, len(page.Accounts)), NextCursor: page.NextCursor}
	for _, account := range page.Accounts {
		wa := wireAccount{ID: account.ID, Email: account.Email}
		if account.Link != (identitysync.Identity{}) {
			wa.Link = &wireIdentity{Issuer: account.Link.Issuer, Subject: account.Link.Subject}
		}
		out.Accounts = append(out.Accounts, wa)
	}
	h.writeJSON(w, r, http.StatusOK, out)
}

// checkPage refuses to publish a page that would break the contract, so a
// store bug becomes a 500 instead of silently wrong data.
func (h *Handler) checkPage(page identitysync.AccountPage, cursor string, limit int) error {
	if len(page.Accounts) > limit {
		return fmt.Errorf("store returned %d accounts for limit %d", len(page.Accounts), limit)
	}
	if !identitysync.ValidCursor(page.NextCursor) || page.NextCursor == cursor && cursor != "" {
		return errors.New("store returned an invalid next cursor")
	}
	for i, account := range page.Accounts {
		if !identitysync.ValidAccountID(h.cfg.AccountIDFormat, account.ID) {
			return errors.New("store returned an account ID that does not match the declared format")
		}
		if i > 0 && !AccountIDLess(h.cfg.AccountIDFormat, page.Accounts[i-1].ID, account.ID) {
			return errors.New("store returned accounts out of ascending ID order")
		}
		if len(account.Email) > 320 {
			return errors.New("store returned an email longer than 320 bytes")
		}
		if account.Link != (identitysync.Identity{}) && !identitysync.ValidIdentity(account.Link, h.cfg.Issuer) {
			return errors.New("store returned a link for another issuer or an invalid subject")
		}
	}
	return nil
}

// AccountIDLess orders account IDs as the contract's "ascending by account ID":
// numerically for positive-integer IDs and bytewise otherwise.
func AccountIDLess(format identitysync.AccountIDFormat, a, b string) bool {
	if format == identitysync.AccountIDPositiveInteger && len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

func (h *Handler) linkIdentity(w http.ResponseWriter, r *http.Request, accountID string) {
	identity, ok := h.readIdentity(w, r)
	if !ok {
		return
	}
	if !identitysync.ValidAccountID(h.cfg.AccountIDFormat, accountID) {
		writeError(w, http.StatusNotFound, "account_ineligible", "account not found or not eligible")
		return
	}
	switch err := h.cfg.Store.LinkIdentity(r.Context(), accountID, identity); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, identitysync.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, "account_ineligible", "account not found or not eligible")
	case errors.Is(err, identitysync.ErrLinkConflict):
		writeError(w, http.StatusConflict, "link_conflict", "link conflicts with an existing link")
	default:
		h.storeError(w, r, fmt.Errorf("link identity: %w", err))
	}
}

func (h *Handler) revokeSessions(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.readIdentity(w, r)
	if !ok {
		return
	}
	switch err := h.cfg.Store.RevokeSessions(r.Context(), identity); {
	case err == nil, errors.Is(err, identitysync.ErrIdentityNotLinked):
		w.WriteHeader(http.StatusNoContent)
	default:
		h.storeError(w, r, fmt.Errorf("revoke sessions: %w", err))
	}
}

func (h *Handler) createTestSession(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.readIdentity(w, r)
	if !ok {
		return
	}
	token, err := h.cfg.Probe.CreateTestSession(r.Context(), identity)
	switch {
	case err == nil && token != "" && len(token) <= maxSessionToken:
		h.writeJSON(w, r, http.StatusCreated, map[string]string{"sessionToken": token})
	case errors.Is(err, identitysync.ErrIdentityNotLinked), errors.Is(err, identitysync.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, "account_ineligible", "identity is not linked to an eligible account")
	case err == nil:
		h.storeError(w, r, errors.New("probe returned an empty or oversized session token"))
	default:
		h.storeError(w, r, fmt.Errorf("create test session: %w", err))
	}
}

func (h *Handler) checkTestSession(w http.ResponseWriter, r *http.Request) {
	values := r.Header.Values("X-Test-Session")
	if len(values) != 1 || values[0] == "" || len(values[0]) > maxSessionToken {
		writeError(w, http.StatusBadRequest, "invalid_request", "one X-Test-Session header is required")
		return
	}
	active, err := h.cfg.Probe.CheckTestSession(r.Context(), values[0])
	switch {
	case err != nil:
		h.storeError(w, r, fmt.Errorf("check test session: %w", err))
	case active:
		h.writeJSON(w, r, http.StatusOK, map[string]bool{"active": true})
	default:
		w.WriteHeader(http.StatusUnauthorized)
	}
}

// readIdentity reads and validates an {"issuer","subject"} body.
func (h *Handler) readIdentity(w http.ResponseWriter, r *http.Request) (identitysync.Identity, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBody+1))
	if err != nil || len(body) > MaxRequestBody {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be JSON of at most 4 KiB")
		return identitysync.Identity{}, false
	}
	var in struct {
		Issuer  *string `json:"issuer"`
		Subject *string `json:"subject"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || in.Issuer == nil || in.Subject == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "body must be {\"issuer\",\"subject\"}")
		return identitysync.Identity{}, false
	}
	if _, err := dec.Token(); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
		return identitysync.Identity{}, false
	}
	identity := identitysync.Identity{Issuer: *in.Issuer, Subject: *in.Subject}
	if !identitysync.ValidIdentity(identity, h.cfg.Issuer) {
		writeError(w, http.StatusBadRequest, "invalid_request", "issuer must be the configured issuer and subject 1-255 characters")
		return identitysync.Identity{}, false
	}
	return identity, true
}

func (h *Handler) storeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrRateLimited):
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limited")
	case errors.Is(err, ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, "unavailable", "temporarily unavailable")
	default:
		h.reportError(r, err)
		writeError(w, http.StatusInternalServerError, "unavailable", "internal error")
	}
}

func (h *Handler) reportError(r *http.Request, err error) {
	if h.cfg.OnError != nil {
		h.cfg.OnError(r, err)
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := json.Marshal(v)
	if err == nil && len(body) > MaxResponseBody {
		err = fmt.Errorf("response of %d bytes exceeds %d", len(body), MaxResponseBody)
	}
	if err != nil {
		h.reportError(r, err)
		writeError(w, http.StatusInternalServerError, "unavailable", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	body, _ := json.Marshal(map[string]any{"error": map[string]string{"code": code, "message": message}})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func accountIdentityPath(path string) bool {
	return len(path) > len("/accounts/")+len("/identity") &&
		strings.HasPrefix(path, "/accounts/") && strings.HasSuffix(path, "/identity")
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func validIssuer(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
