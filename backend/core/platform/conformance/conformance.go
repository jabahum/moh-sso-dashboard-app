// Package conformance checks a portal's platform API (/api/v1/platform/*)
// against its contract (backend/core/platform/contract/platform.openapi.yaml)
// and the authorization rules in docs/platform-identity.md, over real HTTP.
//
// Run takes the portal URL, token sources for differently configured clients,
// what the bound client is expected to be granted, and optional controls that
// disable the client's binding or suspend its application. It returns one
// Result per check. A check whose input is missing (a token source left nil,
// no controls, notifications not served) is skipped with the reason, never
// passed. Every check that succeeds over raw HTTP is repeated with the Go
// client (package client) to prove both read the API the same way.
//
// The checks are read-only except: notifications are sent only when
// Expect.NotificationRecipient is set, and the revocation checks change the
// portal only through the Controls the caller supplies. Run the full suite
// only against a disposable test environment. The package uses only the Go
// standard library.
package conformance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	platform "github.com/mohuganda/moh-sso-dashboard/backend/core/platform/client"
)

// Status is the outcome of one check.
type Status string

const (
	Pass Status = "pass"
	Fail Status = "fail"
	Skip Status = "skip"
)

// Result is the outcome of one named check. ID is stable across releases.
type Result struct {
	ID     string `json:"id"`
	Area   string `json:"area"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Report holds every check's result in execution order.
type Report struct {
	PortalURL string   `json:"portalUrl"`
	Results   []Result `json:"results"`
}

// Failed reports whether any check failed.
func (r Report) Failed() bool {
	for _, res := range r.Results {
		if res.Status == Fail {
			return true
		}
	}
	return false
}

// Counts returns the number of passed, failed and skipped checks.
func (r Report) Counts() (pass, fail, skip int) {
	for _, res := range r.Results {
		switch res.Status {
		case Pass:
			pass++
		case Fail:
			fail++
		case Skip:
			skip++
		}
	}
	return
}

// Result returns the result with the given ID.
func (r Report) Result(id string) (Result, bool) {
	for _, res := range r.Results {
		if res.ID == id {
			return res, true
		}
	}
	return Result{}, false
}

// TokenSource returns a bearer access token.
type TokenSource func(ctx context.Context) (string, error)

// StaticToken returns a TokenSource for a fixed token.
func StaticToken(token string) TokenSource {
	return func(context.Context) (string, error) { return token, nil }
}

// FromClient adapts a client.TokenSource (for example *client.ClientCredentials).
func FromClient(source platform.TokenSource) TokenSource {
	return source.Token
}

// Tokens are the bearer tokens the suite presents. Only Bound is required; a
// nil source skips the checks that need it.
type Tokens struct {
	// Bound is a machine client bound to Expect.Application, requesting every
	// scope in Expect.Scopes.
	Bound TokenSource
	// Narrow is a machine client bound to the same application whose token
	// lacks notifications:write (requested scope or Keycloak client scope),
	// although the binding may grant it.
	Narrow TokenSource
	// Unbound has the platform audience and every scope but no portal binding (403).
	Unbound TokenSource
	// WrongAudience is a valid realm token without the platform audience (401).
	WrongAudience TokenSource
	// WrongIssuer is a valid token from another realm or Keycloak (401).
	WrongIssuer TokenSource
	// User is a signed-in user's access token issued to the application's
	// interactive client, with the platform audience and identity:read.
	User TokenSource
}

// Expect describes the bound client as configured in the portal.
type Expect struct {
	// Application is the registered application the Bound client belongs to.
	Application string `json:"application"`
	// ClientID is the Bound client's Keycloak client ID.
	ClientID string `json:"clientId"`
	// Scopes are the Bound client's effective scopes (token ∩ binding).
	Scopes []string `json:"scopes"`
	// NotificationRecipient, when set, lets the suite send notifications to
	// this test mailbox. Leave empty against environments with real mail.
	NotificationRecipient string `json:"notificationRecipient,omitempty"`
}

// Controls change the portal for the revocation checks. Leave nil to skip them.
// Each must take effect before it returns, and the suite always restores the
// original state.
type Controls struct {
	// SetBindingEnabled enables or disables the Bound client's binding.
	SetBindingEnabled func(ctx context.Context, enabled bool) error
	// SetApplicationActive suspends (false) or reactivates (true) Expect.Application.
	SetApplicationActive func(ctx context.Context, active bool) error
}

// Config configures Run.
type Config struct {
	// PortalURL is the portal origin, for example https://portals.health.go.ug.
	PortalURL string
	// Client sends requests; nil means a client with a 15 second timeout. The
	// suite uses a copy that never follows redirects.
	Client   *http.Client
	Tokens   Tokens
	Expect   Expect
	Controls Controls
	// AllowHTTP accepts an http:// portal URL for local and CI runs.
	AllowHTTP bool
}

// Check IDs. They are stable: CI and other teams can rely on them.
const (
	IDNoToken          = "auth.no-token"
	IDCookieOnly       = "auth.cookie-only"
	IDGarbageToken     = "auth.garbage-token"
	IDWrongAudience    = "auth.wrong-audience"
	IDWrongIssuer      = "auth.wrong-issuer"
	IDUnbound          = "auth.unbound-client"
	IDIdentity         = "identity.bound-client"
	IDIdentityClient   = "identity.go-client"
	IDNarrowScopes     = "scopes.token-intersection"
	IDNarrowRefused    = "scopes.missing-scope-refused"
	IDReadiness        = "readiness.shape"
	IDReadinessClient  = "readiness.go-client"
	IDAccessMachine    = "access.machine-token-refused"
	IDAccessUser       = "access.user-token"
	IDNotifyValidation = "notifications.validation"
	IDNotifySubmit     = "notifications.submit-and-replay"
	IDNotifyConflict   = "notifications.key-conflict"
	IDNotifyStatus     = "notifications.status"
	IDNotifyOther      = "notifications.unknown-id"
	IDRevokeBinding    = "revocation.binding-disabled"
	IDRevokeSuspended  = "revocation.application-suspended"
)

const (
	maxResponseBody = 1 << 20
	platformPath    = "/api/v1/platform"
)

type runner struct {
	cfg    Config
	base   string
	client *http.Client
	go_    *platform.Client
	report Report
}

// Run executes every check and returns the report. It returns an error only
// when cfg itself is unusable; failures of the portal are results.
func Run(ctx context.Context, cfg Config) (Report, error) {
	r, err := newRunner(cfg)
	if err != nil {
		return Report{}, err
	}
	r.authentication(ctx)
	identityOK := r.identity(ctx)
	r.scopes(ctx, identityOK)
	r.readiness(ctx)
	r.access(ctx)
	r.notifications(ctx, identityOK)
	r.revocation(ctx, identityOK)
	return r.report, nil
}

func newRunner(cfg Config) (*runner, error) {
	u, err := url.Parse(cfg.PortalURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" ||
		!(u.Scheme == "https" || cfg.AllowHTTP && u.Scheme == "http") {
		return nil, errors.New("conformance: PortalURL must be an https origin without path, query or credentials")
	}
	if cfg.Tokens.Bound == nil {
		return nil, errors.New("conformance: Tokens.Bound is required")
	}
	if cfg.Expect.Application == "" || cfg.Expect.ClientID == "" {
		return nil, errors.New("conformance: Expect needs the application and the bound client ID")
	}
	httpClient := cfg.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	copied := *httpClient
	copied.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	base := strings.TrimRight(cfg.PortalURL, "/")
	goClient, err := platform.New(platform.Config{
		PortalURL: base, Tokens: tokenAdapter{cfg.Tokens.Bound}, HTTPClient: &copied,
		MaxAttempts: 1, UserAgent: "moh-platform-conformance",
	})
	if err != nil && !cfg.AllowHTTP {
		return nil, fmt.Errorf("conformance: %w", err)
	}
	return &runner{cfg: cfg, base: base, client: &copied, go_: goClient, report: Report{PortalURL: base}}, nil
}

// tokenAdapter lets the Go client use a conformance TokenSource.
type tokenAdapter struct{ source TokenSource }

func (t tokenAdapter) Token(ctx context.Context) (string, error) { return t.source(ctx) }
func (tokenAdapter) Invalidate()                                 {}

func (r *runner) add(id, area, name string, status Status, detail string) bool {
	r.report.Results = append(r.report.Results, Result{ID: id, Area: area, Name: name, Status: status, Detail: detail})
	return status == Pass
}

func (r *runner) check(id, area, name string, problems []string) bool {
	if len(problems) == 0 {
		return r.add(id, area, name, Pass, "")
	}
	return r.add(id, area, name, Fail, strings.Join(problems, "; "))
}

// ---------------------------------------------------------------- transport

type exchange struct {
	status int
	header http.Header
	body   []byte
}

type request struct {
	method, path string
	token        TokenSource
	rawAuth      string
	cookie       bool
	body         []byte
	headers      map[string]string
}

func (r *runner) do(ctx context.Context, q request) (exchange, error) {
	var body io.Reader
	if q.body != nil {
		body = bytes.NewReader(q.body)
	}
	req, err := http.NewRequestWithContext(ctx, q.method, r.base+q.path, body)
	if err != nil {
		return exchange{}, err
	}
	req.Header.Set("Accept", "application/json")
	if q.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch {
	case q.rawAuth != "":
		req.Header.Set("Authorization", q.rawAuth)
	case q.token != nil:
		token, err := q.token(ctx)
		if err != nil {
			return exchange{}, fmt.Errorf("token: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if q.cookie {
		req.AddCookie(&http.Cookie{Name: "sso_session", Value: "conformance-cookie-only"})
	}
	for name, value := range q.headers {
		req.Header.Set(name, value)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return exchange{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return exchange{}, err
	}
	if len(data) > maxResponseBody {
		return exchange{}, errors.New("response larger than 1 MiB")
	}
	return exchange{status: resp.StatusCode, header: resp.Header, body: data}, nil
}

// expect performs q and returns a problem unless the status is one of want.
func (r *runner) expect(ctx context.Context, q request, want ...int) (exchange, string) {
	ex, err := r.do(ctx, q)
	if err != nil {
		return ex, fmt.Sprintf("%s %s: %v", q.method, q.path, err)
	}
	for _, status := range want {
		if ex.status == status {
			return ex, ""
		}
	}
	return ex, fmt.Sprintf("%s %s: HTTP %d, want %v", q.method, q.path, ex.status, want)
}

// envelope is the portal's response wrapper.
type envelope struct {
	Success *bool           `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// data checks a success envelope and the shape of its data object.
func data(ex exchange, shape string) (map[string]any, []string) {
	var env envelope
	if err := json.Unmarshal(ex.body, &env); err != nil {
		return nil, []string{"response is not JSON"}
	}
	if env.Success == nil || !*env.Success {
		return nil, []string{"success is not true"}
	}
	var obj map[string]any
	if err := json.Unmarshal(env.Data, &obj); err != nil {
		return nil, []string{"data is not an object"}
	}
	return obj, ShapeProblems(shape, obj)
}

// errorCode checks an error envelope and returns its code.
func errorCode(ex exchange) (string, []string) {
	var env envelope
	if err := json.Unmarshal(ex.body, &env); err != nil {
		return "", []string{"error response is not JSON"}
	}
	if env.Success == nil || *env.Success || env.Error == nil || env.Error.Code == "" {
		return "", []string{"error response lacks success:false and error.code"}
	}
	return env.Error.Code, nil
}

func (r *runner) refused(ctx context.Context, id, name string, q request, status int, code string) {
	ex, problem := r.expect(ctx, q, status)
	problems := nonEmpty(problem)
	if problem == "" {
		got, more := errorCode(ex)
		problems = append(problems, more...)
		if len(more) == 0 && got != code {
			problems = append(problems, fmt.Sprintf("error.code %q, want %q", got, code))
		}
		if status == http.StatusUnauthorized && !strings.HasPrefix(ex.header.Get("WWW-Authenticate"), "Bearer") {
			problems = append(problems, "401 without a WWW-Authenticate: Bearer challenge")
		}
	}
	r.check(id, "authentication", name, problems)
}

// ---------------------------------------------------------------- checks

func (r *runner) authentication(ctx context.Context) {
	identity := platformPath + "/identity"
	r.refused(ctx, IDNoToken, "no token is refused", request{method: "GET", path: identity}, 401, "INVALID_IDENTITY")
	r.refused(ctx, IDCookieOnly, "a session cookie without a token is refused", request{method: "GET", path: identity, cookie: true}, 401, "INVALID_IDENTITY")
	garbage := make([]byte, 24)
	_, _ = rand.Read(garbage)
	r.refused(ctx, IDGarbageToken, "an unverifiable token is refused", request{method: "GET", path: identity, rawAuth: fmt.Sprintf("Bearer conformance.%x.invalid", garbage)}, 401, "INVALID_IDENTITY")
	for _, c := range []struct {
		id, name string
		token    TokenSource
		status   int
		code     string
	}{
		{IDWrongAudience, "a token without the platform audience is refused", r.cfg.Tokens.WrongAudience, 401, "INVALID_IDENTITY"},
		{IDWrongIssuer, "a token from another issuer is refused", r.cfg.Tokens.WrongIssuer, 401, "INVALID_IDENTITY"},
		{IDUnbound, "a client without a portal binding is refused", r.cfg.Tokens.Unbound, 403, "APPLICATION_ACCESS_DENIED"},
	} {
		if c.token == nil {
			r.add(c.id, "authentication", c.name, Skip, "no token source configured")
			continue
		}
		r.refused(ctx, c.id, c.name, request{method: "GET", path: identity, token: c.token}, c.status, c.code)
	}
}

func (r *runner) identity(ctx context.Context) bool {
	ex, problem := r.expect(ctx, request{method: "GET", path: platformPath + "/identity", token: r.cfg.Tokens.Bound}, 200)
	problems := nonEmpty(problem)
	if problem == "" {
		obj, more := data(ex, "platform.Principal")
		problems = append(problems, more...)
		if len(more) == 0 {
			problems = append(problems, r.principalProblems(obj, r.cfg.Expect.Scopes)...)
		}
	}
	ok := r.check(IDIdentity, "identity", "the bound client is identified with its effective scopes", problems)
	if !ok {
		r.add(IDIdentityClient, "identity", "the Go client reads the same identity", Skip, "requires "+IDIdentity)
		return false
	}
	if r.go_ == nil {
		r.add(IDIdentityClient, "identity", "the Go client reads the same identity", Skip, "the Go client refuses plain HTTP to a non-loopback portal")
		return true
	}
	id, err := r.go_.Identity(ctx)
	switch {
	case err != nil:
		r.add(IDIdentityClient, "identity", "the Go client reads the same identity", Fail, err.Error())
	case id.ApplicationID != r.cfg.Expect.Application || id.ClientID != r.cfg.Expect.ClientID || !sameSet(id.Scopes, r.cfg.Expect.Scopes):
		r.add(IDIdentityClient, "identity", "the Go client reads the same identity", Fail, fmt.Sprintf("Go client read %+v", id))
	default:
		r.add(IDIdentityClient, "identity", "the Go client reads the same identity", Pass, "")
	}
	return true
}

func (r *runner) principalProblems(obj map[string]any, scopes []string) []string {
	var problems []string
	if obj["applicationId"] != r.cfg.Expect.Application {
		problems = append(problems, fmt.Sprintf("applicationId %v, want %s", obj["applicationId"], r.cfg.Expect.Application))
	}
	if obj["clientId"] != r.cfg.Expect.ClientID {
		problems = append(problems, fmt.Sprintf("clientId %v, want %s", obj["clientId"], r.cfg.Expect.ClientID))
	}
	if got := stringList(obj["scopes"]); !sameSet(got, scopes) {
		problems = append(problems, fmt.Sprintf("scopes %v, want %v", got, scopes))
	}
	return problems
}

func (r *runner) scopes(ctx context.Context, identityOK bool) {
	const area = "scopes"
	if r.cfg.Tokens.Narrow == nil {
		r.add(IDNarrowScopes, area, "effective scopes are the token's scopes that the binding grants", Skip, "no Narrow token source configured")
		r.add(IDNarrowRefused, area, "an endpoint needing a scope the token lacks is refused", Skip, "no Narrow token source configured")
		return
	}
	var want []string
	for _, scope := range r.cfg.Expect.Scopes {
		if scope != "notifications:write" {
			want = append(want, scope)
		}
	}
	ex, problem := r.expect(ctx, request{method: "GET", path: platformPath + "/identity", token: r.cfg.Tokens.Narrow}, 200)
	problems := nonEmpty(problem)
	if problem == "" {
		obj, more := data(ex, "platform.Principal")
		problems = append(problems, more...)
		if len(more) == 0 {
			if got := stringList(obj["scopes"]); contains(got, "notifications:write") || !sameSet(got, want) {
				problems = append(problems, fmt.Sprintf("scopes %v, want %v (no notifications:write)", got, want))
			}
		}
	}
	r.check(IDNarrowScopes, area, "effective scopes are the token's scopes that the binding grants", problems)

	body, _ := json.Marshal(map[string]any{"title": "t", "message": "m", "recipient": map[string]string{"email": "nobody@conformance.invalid"}})
	ex, problem = r.expect(ctx, request{method: "POST", path: platformPath + "/notifications", token: r.cfg.Tokens.Narrow, body: body, headers: map[string]string{"Idempotency-Key": "conformance-narrow"}}, 403, 404)
	switch {
	case problem != "":
		r.add(IDNarrowRefused, area, "an endpoint needing a scope the token lacks is refused", Fail, problem)
	case ex.status == 404:
		r.add(IDNarrowRefused, area, "an endpoint needing a scope the token lacks is refused", Skip, "the portal does not serve notifications (delivery disabled)")
	default:
		code, more := errorCode(ex)
		if len(more) == 0 && code != "APPLICATION_ACCESS_DENIED" {
			more = append(more, fmt.Sprintf("error.code %q", code))
		}
		r.check(IDNarrowRefused, area, "an endpoint needing a scope the token lacks is refused", more)
	}
}

func (r *runner) readiness(ctx context.Context) {
	const area = "readiness"
	if !contains(r.cfg.Expect.Scopes, "identity:read") {
		r.add(IDReadiness, area, "readiness reports ready and worker health", Skip, "the bound client lacks identity:read")
		r.add(IDReadinessClient, area, "the Go client reads the same readiness", Skip, "the bound client lacks identity:read")
		return
	}
	ex, problem := r.expect(ctx, request{method: "GET", path: platformPath + "/readiness", token: r.cfg.Tokens.Bound}, 200, 503)
	problems := nonEmpty(problem)
	var ready any
	if problem == "" {
		obj, more := data(ex, "platform.Readiness")
		problems = append(problems, more...)
		if len(more) == 0 {
			ready = obj["ready"]
			if (ex.status == 200) != (ready == true) {
				problems = append(problems, fmt.Sprintf("HTTP %d with ready %v", ex.status, ready))
			}
			for i, worker := range anyList(obj["workers"]) {
				w, _ := worker.(map[string]any)
				problems = append(problems, prefixed(fmt.Sprintf("workers[%d]", i), ShapeProblems("platform.WorkerHealth", w))...)
			}
		}
	}
	if !r.check(IDReadiness, area, "readiness reports ready and worker health", problems) {
		r.add(IDReadinessClient, area, "the Go client reads the same readiness", Skip, "requires "+IDReadiness)
		return
	}
	if r.go_ == nil {
		r.add(IDReadinessClient, area, "the Go client reads the same readiness", Skip, "the Go client refuses plain HTTP to a non-loopback portal")
		return
	}
	got, err := r.go_.Readiness(ctx)
	switch {
	case err != nil:
		r.add(IDReadinessClient, area, "the Go client reads the same readiness", Fail, err.Error())
	case got.Ready != (ready == true):
		r.add(IDReadinessClient, area, "the Go client reads the same readiness", Fail, fmt.Sprintf("Go client read ready %v", got.Ready))
	default:
		r.add(IDReadinessClient, area, "the Go client reads the same readiness", Pass, "")
	}
}

func (r *runner) access(ctx context.Context) {
	const area = "access"
	path := platformPath + "/applications/" + url.PathEscape(r.cfg.Expect.Application) + "/access"
	ex, problem := r.expect(ctx, request{method: "GET", path: path, token: r.cfg.Tokens.Bound}, 403)
	problems := nonEmpty(problem)
	if problem == "" {
		if code, more := errorCode(ex); len(more) > 0 || code != "APPLICATION_ACCESS_DENIED" {
			problems = append(problems, append(more, fmt.Sprintf("error.code %q", code))...)
		}
	}
	r.check(IDAccessMachine, area, "a machine token is refused as a user", problems)

	if r.cfg.Tokens.User == nil {
		r.add(IDAccessUser, area, "a user's token returns their access to the application", Skip, "no User token source configured")
		return
	}
	ex, problem = r.expect(ctx, request{method: "GET", path: path, token: r.cfg.Tokens.User}, 200)
	problems = nonEmpty(problem)
	if problem == "" {
		obj, more := data(ex, "platform.UserAccess")
		problems = append(problems, more...)
		if len(more) == 0 && obj["applicationId"] != r.cfg.Expect.Application {
			problems = append(problems, fmt.Sprintf("applicationId %v", obj["applicationId"]))
		}
	}
	r.check(IDAccessUser, area, "a user's token returns their access to the application", problems)
}

func (r *runner) notifications(ctx context.Context, identityOK bool) {
	const area = "notifications"
	ids := []struct{ id, name string }{
		{IDNotifyValidation, "invalid notifications are refused before queueing"},
		{IDNotifySubmit, "a notification is queued once and replayed for the same key"},
		{IDNotifyConflict, "reusing a key with a different body is refused"},
		{IDNotifyStatus, "the notification's status is readable"},
		{IDNotifyOther, "an unknown notification ID is not found"},
	}
	skipAll := func(reason string) {
		for _, c := range ids {
			r.add(c.id, area, c.name, Skip, reason)
		}
	}
	if !contains(r.cfg.Expect.Scopes, "notifications:write") {
		skipAll("the bound client lacks notifications:write")
		return
	}
	path := platformPath + "/notifications"
	// Missing key: refused before anything is queued, whatever the recipient.
	body, _ := json.Marshal(map[string]any{"title": "Conformance", "message": "Validation only", "recipient": map[string]string{"email": "nobody@conformance.invalid"}})
	ex, problem := r.expect(ctx, request{method: "POST", path: path, token: r.cfg.Tokens.Bound, body: body}, 400, 404)
	if problem == "" && ex.status == 404 {
		skipAll("the portal does not serve notifications (delivery disabled)")
		return
	}
	problems := nonEmpty(problem)
	badBody, _ := json.Marshal(map[string]any{"title": "a\nb", "message": "", "recipient": map[string]string{"email": "Display <x@conformance.invalid>"}})
	_, problem = r.expect(ctx, request{method: "POST", path: path, token: r.cfg.Tokens.Bound, body: badBody, headers: map[string]string{"Idempotency-Key": "conformance-invalid"}}, 400)
	problems = append(problems, nonEmpty(problem)...)
	r.check(IDNotifyValidation, area, ids[0].name, problems)

	if r.cfg.Expect.NotificationRecipient == "" {
		for _, c := range ids[1:4] {
			r.add(c.id, area, c.name, Skip, "no test NotificationRecipient configured")
		}
	} else {
		r.sendNotification(ctx, path)
	}
	_, problem = r.expect(ctx, request{method: "GET", path: path + "/" + randomUUID(), token: r.cfg.Tokens.Bound}, 404)
	r.check(IDNotifyOther, area, ids[4].name, nonEmpty(problem))
}

func (r *runner) sendNotification(ctx context.Context, path string) {
	const area = "notifications"
	key := "conformance-" + randomUUID()
	n := map[string]any{"title": "Platform API conformance", "message": "Sent by the platform API conformance suite.",
		"recipient": map[string]string{"email": r.cfg.Expect.NotificationRecipient, "name": "Conformance"}}
	body, _ := json.Marshal(n)
	q := request{method: "POST", path: path, token: r.cfg.Tokens.Bound, body: body, headers: map[string]string{"Idempotency-Key": key}}
	ex, problem := r.expect(ctx, q, 202)
	problems := nonEmpty(problem)
	var id string
	if problem == "" {
		obj, more := data(ex, "platform.NotificationStatus")
		problems = append(problems, more...)
		id, _ = obj["id"].(string)
		if location := ex.header.Get("Location"); id == "" || location != path+"/"+id {
			problems = append(problems, fmt.Sprintf("Location %q for id %q", location, id))
		}
		if obj["replayed"] == true {
			problems = append(problems, "first submission reported replayed")
		}
		replay, problem := r.expect(ctx, q, 200)
		problems = append(problems, nonEmpty(problem)...)
		if problem == "" {
			obj, more := data(replay, "platform.NotificationStatus")
			problems = append(problems, more...)
			if obj["id"] != id || obj["replayed"] != true {
				problems = append(problems, fmt.Sprintf("replay returned id %v replayed %v", obj["id"], obj["replayed"]))
			}
		}
	}
	if !r.check(IDNotifySubmit, area, "a notification is queued once and replayed for the same key", problems) {
		r.add(IDNotifyConflict, area, "reusing a key with a different body is refused", Skip, "requires "+IDNotifySubmit)
		r.add(IDNotifyStatus, area, "the notification's status is readable", Skip, "requires "+IDNotifySubmit)
		return
	}
	n["message"] = "A different body with the same key."
	changed, _ := json.Marshal(n)
	ex, problem = r.expect(ctx, request{method: "POST", path: path, token: r.cfg.Tokens.Bound, body: changed, headers: map[string]string{"Idempotency-Key": key}}, 409)
	problems = nonEmpty(problem)
	if problem == "" {
		if code, more := errorCode(ex); len(more) > 0 || code != "IDEMPOTENCY_CONFLICT" {
			problems = append(problems, append(more, fmt.Sprintf("error.code %q", code))...)
		}
	}
	r.check(IDNotifyConflict, area, "reusing a key with a different body is refused", problems)

	if !contains(r.cfg.Expect.Scopes, "notifications:read") {
		r.add(IDNotifyStatus, area, "the notification's status is readable", Skip, "the bound client lacks notifications:read")
		return
	}
	ex, problem = r.expect(ctx, request{method: "GET", path: path + "/" + id, token: r.cfg.Tokens.Bound}, 200)
	problems = nonEmpty(problem)
	if problem == "" {
		obj, more := data(ex, "platform.NotificationStatus")
		problems = append(problems, more...)
		if obj["id"] != id || obj["applicationId"] != r.cfg.Expect.Application {
			problems = append(problems, fmt.Sprintf("status for id %v application %v", obj["id"], obj["applicationId"]))
		}
	}
	r.check(IDNotifyStatus, area, "the notification's status is readable", problems)
}

func (r *runner) revocation(ctx context.Context, identityOK bool) {
	const area = "revocation"
	run := func(id, name string, set func(context.Context, bool) error) {
		switch {
		case set == nil:
			r.add(id, area, name, Skip, "no control configured")
			return
		case !identityOK:
			r.add(id, area, name, Skip, "requires "+IDIdentity)
			return
		}
		identity := request{method: "GET", path: platformPath + "/identity", token: r.cfg.Tokens.Bound}
		var problems []string
		if err := set(ctx, false); err != nil {
			r.add(id, area, name, Fail, "control failed: "+err.Error())
			return
		}
		ex, problem := r.expect(ctx, identity, 403)
		problems = append(problems, nonEmpty(problem)...)
		if problem == "" {
			if code, more := errorCode(ex); len(more) > 0 || code != "APPLICATION_ACCESS_DENIED" {
				problems = append(problems, append(more, fmt.Sprintf("error.code %q", code))...)
			}
		}
		if err := set(ctx, true); err != nil {
			problems = append(problems, "restoring failed: "+err.Error())
		} else if _, problem := r.expect(ctx, identity, 200); problem != "" {
			problems = append(problems, "after restoring: "+problem)
		}
		r.check(id, area, name, problems)
	}
	run(IDRevokeBinding, "disabling the binding refuses the next request; enabling restores it", r.cfg.Controls.SetBindingEnabled)
	run(IDRevokeSuspended, "suspending the application refuses the next request; reactivating restores it", r.cfg.Controls.SetApplicationActive)
}

// ---------------------------------------------------------------- helpers

func nonEmpty(problem string) []string {
	if problem == "" {
		return nil
	}
	return []string{problem}
}

func prefixed(prefix string, problems []string) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = prefix + ": " + p
	}
	return out
}

func anyList(value any) []any {
	list, _ := value.([]any)
	return list
}

func stringList(value any) []string {
	var out []string
	for _, item := range anyList(value) {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func sameSet(a, b []string) bool {
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "\x00") == strings.Join(y, "\x00")
}

func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
