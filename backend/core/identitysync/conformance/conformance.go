// Package conformance checks an application's identity-sync endpoints against
// contract 1.1 (docs/integrations/identity-sync-contract.md) over real HTTP.
//
// Run takes the application's base URL, an *http.Client, token sources and
// synthetic fixtures, and returns one Result per check. A check whose input is
// missing (for example a token source left nil, or the session probe not
// enabled) is reported as skipped with the reason, never as passed. A check
// that cannot run because an earlier check failed is skipped and names it.
//
// The suite creates links for the fixture accounts. Run it only against a
// disposable test environment with synthetic accounts. The package uses only
// the Go standard library.
package conformance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
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
	BaseURL string   `json:"baseUrl"`
	Results []Result `json:"results"`
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

// Tokens are the bearer tokens the suite presents. Only Sync is required; a
// nil source skips the checks that need it.
type Tokens struct {
	// Sync is a valid token of the application's identity-sync client.
	Sync TokenSource
	// ExpiredOrGarbage is expired or otherwise unverifiable (expects 401).
	ExpiredOrGarbage TokenSource
	// WrongAudience is a valid realm token whose aud lacks the application (401).
	WrongAudience TokenSource
	// User is an interactive user's access token for the application (403).
	User TokenSource
	// OtherClient is another service client's token with the application in aud (403).
	OtherClient TokenSource
	// MissingRole is a token of the sync client without the identity_sync role (403).
	MissingRole TokenSource
}

// Fixtures describe the synthetic accounts in the test environment.
type Fixtures struct {
	// Application is the expected descriptor application (the Keycloak client ID).
	Application string `json:"application"`
	// Issuer is the application's configured public issuer.
	Issuer string `json:"issuer"`
	// LinkableAccounts are two eligible, currently unlinked account IDs.
	LinkableAccounts []string `json:"linkableAccounts"`
	// IneligibleAccount exists but must not be listed or linkable.
	IneligibleAccount string `json:"ineligibleAccount"`
	// Subjects are three synthetic, unused Keycloak subjects. When empty the
	// suite generates random ones.
	Subjects []string `json:"subjects,omitempty"`
}

// Config configures Run.
type Config struct {
	// BaseURL is https://<origin>/api/v1/internal/identity-sync.
	BaseURL string
	// Client sends requests; nil means a client with a 15 second timeout. The
	// suite uses a copy that never follows redirects.
	Client   *http.Client
	Tokens   Tokens
	Fixtures Fixtures
	// Probe runs the session checks that use the test-only session probe.
	Probe bool
	// AllowHTTP accepts an http:// base URL for local runs. HTTPS is then
	// reported as not verified.
	AllowHTTP bool
}

const (
	maxResponseBody = 256 << 10
	maxRequestBody  = 4 << 10
	maxWalkPages    = 20000
	wrongIssuer     = "https://identity-sync-conformance.invalid/realms/wrong"
)

var errorCodes = map[string]bool{
	"invalid_request": true, "account_ineligible": true, "link_conflict": true,
	"rate_limited": true, "unavailable": true,
}

type exchange struct {
	label    string
	status   int
	header   http.Header
	body     []byte
	tooLarge bool
}

type runner struct {
	mu      sync.Mutex // guards seen and tokens for concurrent checks
	cfg     Config
	base    string
	client  *http.Client
	report  Report
	seen    []exchange
	tokens  map[string]bool
	format  identitysync.AccountIDFormat
	maxPage int
	revoke  bool // descriptor advertises sessions.revoke
	s1      string
	s2      string
	s3      string
}

// Run executes every check and returns the report. It returns an error only
// when cfg itself is unusable; failures of the application are results.
func Run(ctx context.Context, cfg Config) (Report, error) {
	r, err := newRunner(cfg)
	if err != nil {
		return Report{}, err
	}
	r.descriptor(ctx)
	r.authorization(ctx)
	fixturesOK := r.pagination(ctx)
	linked := r.linking(ctx, fixturesOK)
	r.sessions(ctx, linked)
	r.retries(ctx, linked)
	r.transport(ctx, linked)
	return r.report, nil
}

func newRunner(cfg Config) (*runner, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		!(u.Scheme == "https" || cfg.AllowHTTP && u.Scheme == "http") {
		return nil, errors.New("conformance: BaseURL must be an https URL without query or credentials")
	}
	if cfg.Tokens.Sync == nil {
		return nil, errors.New("conformance: Tokens.Sync is required")
	}
	f := cfg.Fixtures
	if f.Application == "" || f.Issuer == "" {
		return nil, errors.New("conformance: fixtures need application and issuer")
	}
	if len(f.LinkableAccounts) != 2 || f.LinkableAccounts[0] == f.LinkableAccounts[1] ||
		f.IneligibleAccount == "" || f.IneligibleAccount == f.LinkableAccounts[0] || f.IneligibleAccount == f.LinkableAccounts[1] {
		return nil, errors.New("conformance: fixtures need two distinct linkable accounts and a different ineligible account")
	}
	for _, id := range append([]string{f.IneligibleAccount}, f.LinkableAccounts...) {
		if !identitysync.ValidAccountID(identitysync.AccountIDOpaque, id) {
			return nil, fmt.Errorf("conformance: fixture account ID %q does not match the contract pattern", id)
		}
	}
	subjects := f.Subjects
	if len(subjects) == 0 {
		suffix := make([]byte, 8)
		if _, err := rand.Read(suffix); err != nil {
			return nil, err
		}
		for i := 1; i <= 3; i++ {
			subjects = append(subjects, fmt.Sprintf("conformance-%x-%d", suffix, i))
		}
	}
	if len(subjects) != 3 || subjects[0] == subjects[1] || subjects[0] == subjects[2] || subjects[1] == subjects[2] {
		return nil, errors.New("conformance: fixtures need zero or three distinct subjects")
	}
	for _, s := range subjects {
		if !identitysync.ValidIdentity(identitysync.Identity{Issuer: f.Issuer, Subject: s}, f.Issuer) {
			return nil, fmt.Errorf("conformance: subject %q must be 1-255 characters without surrounding spaces", s)
		}
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	copied := *client
	copied.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &runner{
		cfg:     cfg,
		base:    strings.TrimRight(cfg.BaseURL, "/"),
		client:  &copied,
		report:  Report{BaseURL: strings.TrimRight(cfg.BaseURL, "/")},
		tokens:  map[string]bool{},
		format:  identitysync.AccountIDOpaque,
		maxPage: identitysync.MaxPageSize,
		s1:      subjects[0], s2: subjects[1], s3: subjects[2],
	}, nil
}

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

// request describes one HTTP call. token "" sends no Authorization header;
// rawAuth, when set, is sent verbatim instead.
type request struct {
	method, path string
	token        TokenSource
	rawAuth      string
	body         []byte
	headers      map[string]string
}

func (r *runner) do(ctx context.Context, q request) (exchange, error) {
	label := q.method + " " + q.path
	req, err := http.NewRequestWithContext(ctx, q.method, r.base+q.path, bytes.NewReader(q.body))
	if err != nil {
		return exchange{}, err
	}
	if q.body == nil {
		req.Body = http.NoBody
		req.ContentLength = 0
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	switch {
	case q.rawAuth != "":
		req.Header.Set("Authorization", q.rawAuth)
	case q.token != nil:
		token, err := q.token(ctx)
		if err != nil {
			return exchange{}, fmt.Errorf("get token: %w", err)
		}
		if token == "" {
			return exchange{}, errors.New("token source returned an empty token")
		}
		r.mu.Lock()
		r.tokens[token] = true
		r.mu.Unlock()
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range q.headers {
		req.Header.Set(k, v)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return exchange{}, fmt.Errorf("%s: %w", label, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return exchange{}, fmt.Errorf("%s: read body: %w", label, err)
	}
	ex := exchange{label: label, status: resp.StatusCode, header: resp.Header, body: body, tooLarge: len(body) > maxResponseBody}
	r.mu.Lock()
	r.seen = append(r.seen, ex)
	r.mu.Unlock()
	return ex, nil
}

// expect performs q and returns a problem description unless the status is one of want.
func (r *runner) expect(ctx context.Context, q request, want ...int) (exchange, string) {
	ex, err := r.do(ctx, q)
	if err != nil {
		return ex, err.Error()
	}
	for _, w := range want {
		if ex.status == w {
			return ex, ""
		}
	}
	wants := make([]string, len(want))
	for i, w := range want {
		wants[i] = strconv.Itoa(w)
	}
	return ex, fmt.Sprintf("%s returned %d, want %s", ex.label, ex.status, strings.Join(wants, " or "))
}

func (r *runner) identityBody(issuer, subject string) []byte {
	b, _ := json.Marshal(map[string]string{"issuer": issuer, "subject": subject})
	return b
}

func (r *runner) linkPath(id string) string {
	return "/accounts/" + url.PathEscape(id) + "/identity"
}

// ---- Descriptor -----------------------------------------------------------

func (r *runner) descriptor(ctx context.Context) {
	const area = "descriptor"
	ex, problem := r.expect(ctx, request{method: http.MethodGet, path: "/", token: r.cfg.Tokens.Sync}, http.StatusOK)
	var d identitysync.Descriptor
	shapeOK := false
	if problem == "" {
		dec := json.NewDecoder(bytes.NewReader(ex.body))
		dec.DisallowUnknownFields()
		switch err := dec.Decode(&d); {
		case err != nil:
			problem = "descriptor is not the contract's JSON object: " + err.Error()
		case d.Validate(d.Application) != nil:
			problem = d.Validate(d.Application).Error()
		case d.Version == identitysync.Version10:
			problem = "descriptor reports version 1.0; 1.1 is required"
		default:
			shapeOK = true
		}
	}
	if !shapeOK {
		r.add("descriptor.shape", area, "GET / returns a valid 1.1 descriptor", Fail, problem)
		r.add("descriptor.application", area, "descriptor names the expected application", Skip, "blocked by descriptor.shape")
		r.add("descriptor.capabilities", area, "advertised capabilities match the endpoints present", Skip, "blocked by descriptor.shape")
		return
	}
	r.add("descriptor.shape", area, "GET / returns a valid 1.1 descriptor", Pass, "")
	r.format, r.maxPage, r.revoke = d.AccountIDFormat, d.MaxPageSize, d.Supports(identitysync.CapabilityRevokeSessions)
	if d.Application == r.cfg.Fixtures.Application {
		r.add("descriptor.application", area, "descriptor names the expected application", Pass, "")
	} else {
		r.add("descriptor.application", area, "descriptor names the expected application", Fail,
			fmt.Sprintf("descriptor names %q, expected %q", d.Application, r.cfg.Fixtures.Application))
	}
	// Every 1.1 descriptor offers listing and linking (checked by Validate and
	// exercised below). Here, check that sessions.revoke is present exactly
	// when advertised. An unknown, unlinked subject has no side effects.
	ex, err := r.do(ctx, request{method: http.MethodPost, path: "/sessions/revoke", token: r.cfg.Tokens.Sync,
		body: r.identityBody(r.cfg.Fixtures.Issuer, r.s3)})
	var problems []string
	switch {
	case err != nil:
		problems = append(problems, err.Error())
	case r.revoke && (ex.status == http.StatusNotFound || ex.status == http.StatusMethodNotAllowed):
		problems = append(problems, fmt.Sprintf("sessions.revoke advertised but POST /sessions/revoke returned %d", ex.status))
	case !r.revoke && ex.status != http.StatusNotFound && ex.status != http.StatusMethodNotAllowed:
		problems = append(problems, fmt.Sprintf("sessions.revoke not advertised but POST /sessions/revoke returned %d", ex.status))
	}
	r.check("descriptor.capabilities", area, "advertised capabilities match the endpoints present", problems)
}

// ---- Authorization --------------------------------------------------------

func (r *runner) authorization(ctx context.Context) {
	const area = "authorization"
	f := r.cfg.Fixtures
	// Requests that change nothing even if authorization is broken: the
	// ineligible account cannot be linked, and the third subject is unlinked.
	targets := []request{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodGet, path: "/accounts?limit=1"},
		{method: http.MethodPut, path: r.linkPath(f.IneligibleAccount), body: r.identityBody(f.Issuer, r.s3)},
	}
	if r.revoke {
		targets = append(targets, request{method: http.MethodPost, path: "/sessions/revoke", body: r.identityBody(f.Issuer, r.s3)})
	}
	if r.cfg.Probe {
		targets = append(targets,
			request{method: http.MethodPost, path: "/test/sessions", body: r.identityBody(f.Issuer, r.s3)},
			request{method: http.MethodGet, path: "/test/sessions/check", headers: map[string]string{"X-Test-Session": "conformance-invalid-session"}})
	}
	variants := []struct {
		id, name string
		want     int
		token    TokenSource
		rawAuth  string
		always   bool
	}{
		{id: "no-token", name: "no bearer token is refused with 401", want: 401, always: true},
		{id: "non-bearer", name: "a non-Bearer Authorization scheme is refused with 401", want: 401, rawAuth: "Basic Y29uZm9ybWFuY2U6Y29uZm9ybWFuY2U=", always: true},
		{id: "garbage-token", name: "an unverifiable bearer token is refused with 401", want: 401, rawAuth: "Bearer conformance.invalid.token", always: true},
		{id: "expired-token", name: "an expired or unverifiable token is refused with 401", want: 401, token: r.cfg.Tokens.ExpiredOrGarbage},
		{id: "wrong-audience", name: "a token without the application in aud is refused with 401", want: 401, token: r.cfg.Tokens.WrongAudience},
		{id: "user-token", name: "a user token is refused with 403", want: 403, token: r.cfg.Tokens.User},
		{id: "other-client", name: "another client's token is refused with 403", want: 403, token: r.cfg.Tokens.OtherClient},
		{id: "missing-role", name: "a token without the identity_sync role is refused with 403", want: 403, token: r.cfg.Tokens.MissingRole},
	}
	for _, v := range variants {
		id := "authorization." + v.id
		if !v.always && v.token == nil {
			r.add(id, area, v.name, Skip, "no token source configured for this check")
			continue
		}
		var problems []string
		for _, t := range targets {
			q := t
			q.token, q.rawAuth = v.token, v.rawAuth
			if _, problem := r.expect(ctx, q, v.want); problem != "" {
				problems = append(problems, problem)
			}
		}
		r.check(id, area, v.name, problems)
	}
	// The valid sync token is accepted on every endpoint (exercised below);
	// check it on the descriptor and listing here so a broken verifier that
	// refuses everything is reported under authorization.
	var problems []string
	for _, t := range targets[:2] {
		q := t
		q.token = r.cfg.Tokens.Sync
		if _, problem := r.expect(ctx, q, http.StatusOK); problem != "" {
			problems = append(problems, problem)
		}
	}
	r.check("authorization.sync-token", area, "the identity-sync client's token is accepted", problems)
}

// ---- Pagination -----------------------------------------------------------

type listedAccount struct {
	id      string
	email   string
	link    *identitysync.Identity
	hasLink bool // the link field was present (null or object)
}

type wireAccount struct {
	ID    *string         `json:"id"`
	Email *string         `json:"email"`
	Link  json.RawMessage `json:"link"`
}

type wirePage struct {
	Accounts   *[]wireAccount `json:"accounts"`
	NextCursor *string        `json:"nextCursor"`
}

func (r *runner) parsePage(body []byte, limit int) ([]listedAccount, string, error) {
	var page wirePage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", fmt.Errorf("page is not JSON: %v", err)
	}
	if page.Accounts == nil || page.NextCursor == nil {
		return nil, "", errors.New("page lacks accounts or nextCursor")
	}
	if len(*page.Accounts) > limit || len(*page.Accounts) > r.maxPage {
		return nil, "", fmt.Errorf("page has %d accounts for limit %d (maxPageSize %d)", len(*page.Accounts), limit, r.maxPage)
	}
	if !identitysync.ValidCursor(*page.NextCursor) {
		return nil, "", fmt.Errorf("nextCursor %q is not a valid cursor", *page.NextCursor)
	}
	out := make([]listedAccount, 0, len(*page.Accounts))
	for _, a := range *page.Accounts {
		if a.ID == nil || a.Email == nil {
			return nil, "", errors.New("account lacks id or email")
		}
		if !identitysync.ValidAccountID(r.format, *a.ID) {
			return nil, "", fmt.Errorf("account ID %q does not match format %s", *a.ID, r.format)
		}
		if len(*a.Email) > 320 {
			return nil, "", fmt.Errorf("account %s email exceeds 320 bytes", *a.ID)
		}
		la := listedAccount{id: *a.ID, email: *a.Email, hasLink: a.Link != nil}
		if a.Link != nil && string(a.Link) != "null" {
			var link struct {
				Issuer  *string `json:"issuer"`
				Subject *string `json:"subject"`
			}
			dec := json.NewDecoder(bytes.NewReader(a.Link))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&link); err != nil || link.Issuer == nil || link.Subject == nil {
				return nil, "", fmt.Errorf("account %s link is not {issuer, subject}", *a.ID)
			}
			id := identitysync.Identity{Issuer: *link.Issuer, Subject: *link.Subject}
			if !identitysync.ValidIdentity(id, r.cfg.Fixtures.Issuer) {
				return nil, "", fmt.Errorf("account %s link has another issuer or an invalid subject", *a.ID)
			}
			la.link = &id
		}
		out = append(out, la)
	}
	return out, *page.NextCursor, nil
}

func (r *runner) less(a, b string) bool {
	if r.format == identitysync.AccountIDPositiveInteger && len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// walk lists every account with the given page size (0 omits limit).
func (r *runner) walk(ctx context.Context, limit int) ([]listedAccount, error) {
	var all []listedAccount
	cursors := map[string]bool{}
	cursor := ""
	effective := limit
	if effective == 0 {
		effective = identitysync.MaxPageSize
	}
	for page := 0; page < maxWalkPages; page++ {
		q := url.Values{}
		if limit > 0 {
			q.Set("limit", strconv.Itoa(limit))
		}
		if cursor != "" {
			q.Set("after", cursor)
		}
		path := "/accounts"
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
		ex, problem := r.expect(ctx, request{method: http.MethodGet, path: path, token: r.cfg.Tokens.Sync}, http.StatusOK)
		if problem != "" {
			return nil, errors.New(problem)
		}
		accounts, next, err := r.parsePage(ex.body, effective)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page+1, err)
		}
		for _, a := range accounts {
			if len(all) > 0 && !r.less(all[len(all)-1].id, a.id) {
				return nil, fmt.Errorf("account %s follows %s: not strictly ascending (duplicate or out of order)", a.id, all[len(all)-1].id)
			}
			all = append(all, a)
		}
		if next == "" {
			return all, nil
		}
		if cursors[next] {
			return nil, fmt.Errorf("nextCursor %q repeated", next)
		}
		cursors[next] = true
		cursor = next
	}
	return nil, fmt.Errorf("listing did not end within %d pages", maxWalkPages)
}

func (r *runner) pagination(ctx context.Context) (fixturesOK bool) {
	const area = "pagination"
	walks := map[int][]listedAccount{}
	for _, limit := range []int{1, 7, 100} {
		id := fmt.Sprintf("pagination.walk-limit-%d", limit)
		name := fmt.Sprintf("a full walk with limit %d lists each eligible account once, in ascending order, ending on an empty cursor", limit)
		accounts, err := r.walk(ctx, limit)
		if err != nil {
			r.add(id, area, name, Fail, err.Error())
			continue
		}
		walks[limit] = accounts
		r.add(id, area, name, Pass, fmt.Sprintf("%d accounts", len(accounts)))
	}
	if len(walks) == 3 {
		var problems []string
		ids := func(list []listedAccount) string {
			s := make([]string, len(list))
			for i, a := range list {
				s[i] = a.id
			}
			return strings.Join(s, ",")
		}
		if ids(walks[1]) != ids(walks[7]) || ids(walks[1]) != ids(walks[100]) {
			problems = append(problems, fmt.Sprintf("walks differ: limit 1 saw %d, limit 7 saw %d, limit 100 saw %d accounts",
				len(walks[1]), len(walks[7]), len(walks[100])))
		}
		for _, a := range walks[100] {
			if !a.hasLink {
				problems = append(problems, fmt.Sprintf("account %s lacks the 1.1 link field", a.id))
				break
			}
		}
		r.check("pagination.consistent", area, "every walk covers the same accounts and reports each account's link", problems)
	} else {
		r.add("pagination.consistent", area, "every walk covers the same accounts and reports each account's link", Skip, "blocked by a failed walk")
	}

	if list, ok := walks[100]; ok {
		f := r.cfg.Fixtures
		byID := map[string]listedAccount{}
		for _, a := range list {
			byID[a.id] = a
		}
		var problems []string
		for _, id := range f.LinkableAccounts {
			a, ok := byID[id]
			switch {
			case !ok:
				problems = append(problems, fmt.Sprintf("linkable account %s is not listed", id))
			case a.link != nil:
				problems = append(problems, fmt.Sprintf("linkable account %s is already linked; reset the synthetic data", id))
			}
		}
		if _, ok := byID[f.IneligibleAccount]; ok {
			problems = append(problems, fmt.Sprintf("ineligible account %s is listed", f.IneligibleAccount))
		}
		fixturesOK = r.check("pagination.fixtures", area, "linkable fixtures are listed and unlinked; the ineligible account is not listed", problems)
	} else {
		// Unknown: listing is broken, but linking can still be checked.
		fixturesOK = true
		r.add("pagination.fixtures", area, "linkable fixtures are listed and unlinked; the ineligible account is not listed", Skip, "blocked by pagination.walk-limit-100")
	}

	var problems []string
	if _, p := r.expect(ctx, request{method: http.MethodGet, path: "/accounts?after=" + url.QueryEscape("not a cursor!"), token: r.cfg.Tokens.Sync}, http.StatusBadRequest); p != "" {
		problems = append(problems, p)
	}
	r.check("pagination.bad-cursor", area, "an invalid after cursor is refused with 400", problems)
	problems = nil
	for _, limit := range []string{"0", "101", "abc"} {
		if _, p := r.expect(ctx, request{method: http.MethodGet, path: "/accounts?limit=" + limit, token: r.cfg.Tokens.Sync}, http.StatusBadRequest); p != "" {
			problems = append(problems, p)
		}
	}
	r.check("pagination.bad-limit", area, "a limit outside 1-100 is refused with 400", problems)
	return fixturesOK
}

// findAccounts lists all accounts with limit 100 and returns those requested.
func (r *runner) findAccounts(ctx context.Context, ids ...string) (map[string]listedAccount, error) {
	list, err := r.walk(ctx, 100)
	if err != nil {
		return nil, err
	}
	found := map[string]listedAccount{}
	for _, a := range list {
		for _, id := range ids {
			if a.id == id {
				found[id] = a
			}
		}
	}
	return found, nil
}

// ---- Linking --------------------------------------------------------------

func (r *runner) linking(ctx context.Context, fixturesOK bool) (linked bool) {
	const area = "linking"
	f := r.cfg.Fixtures
	a, b := f.LinkableAccounts[0], f.LinkableAccounts[1]
	put := func(id string, body []byte, want int) string {
		_, p := r.expect(ctx, request{method: http.MethodPut, path: r.linkPath(id), token: r.cfg.Tokens.Sync, body: body}, want)
		return p
	}
	type step struct {
		id, name string
		run      func() string
		needLink bool
	}
	steps := []step{
		{"linking.link", "linking an eligible account returns 204", func() string { return put(a, r.identityBody(f.Issuer, r.s1), 204) }, false},
		{"linking.repeat", "repeating the same link returns 204", func() string { return put(a, r.identityBody(f.Issuer, r.s1), 204) }, true},
		{"linking.other-subject", "a different subject for a linked account returns 409", func() string { return put(a, r.identityBody(f.Issuer, r.s2), 409) }, true},
		{"linking.subject-taken", "a subject already linked to another account returns 409", func() string { return put(b, r.identityBody(f.Issuer, r.s1), 409) }, true},
		{"linking.ineligible", "linking the ineligible account returns 404", func() string { return put(f.IneligibleAccount, r.identityBody(f.Issuer, r.s3), 404) }, false},
		{"linking.wrong-issuer", "an issuer other than the configured one returns 400", func() string { return put(b, r.identityBody(wrongIssuer, r.s3), 400) }, false},
		{"linking.malformed", "a malformed body or invalid subject returns 400", func() string {
			var problems []string
			for _, body := range []string{
				`{"issuer":` + strconv.Quote(f.Issuer) + `}`,
				`{"issuer":` + strconv.Quote(f.Issuer) + `,"subject":""}`,
				`{"issuer":` + strconv.Quote(f.Issuer) + `,"subject":` + strconv.Quote(strings.Repeat("s", 256)) + `}`,
				`not json`,
			} {
				if p := put(b, []byte(body), 400); p != "" {
					problems = append(problems, p)
				}
			}
			return strings.Join(problems, "; ")
		}, false},
	}
	for _, s := range steps {
		switch {
		case !fixturesOK:
			r.add(s.id, area, s.name, Skip, "blocked by pagination.fixtures")
			continue
		case s.needLink && !linked:
			r.add(s.id, area, s.name, Skip, "blocked by linking.link")
			continue
		}
		problem := s.run()
		if problem == "" {
			r.add(s.id, area, s.name, Pass, "")
		} else {
			r.add(s.id, area, s.name, Fail, problem)
		}
		if s.id == "linking.link" {
			linked = problem == ""
		}
	}
	name := "the listing shows the new link, and refused requests changed nothing"
	if !fixturesOK || !linked {
		r.add("linking.visible", area, name, Skip, "blocked by linking.link")
		return linked
	}
	status, detail := statusOf(r.linkState(ctx, a, b, r.s1))
	r.add("linking.visible", area, name, status, detail)
	return linked
}

// linkState checks that account a is linked to subject and b is unlinked.
func (r *runner) linkState(ctx context.Context, a, b, subject string) (bool, string) {
	found, err := r.findAccounts(ctx, a, b)
	if err != nil {
		return false, err.Error()
	}
	var problems []string
	switch la, ok := found[a]; {
	case !ok:
		problems = append(problems, fmt.Sprintf("account %s is no longer listed", a))
	case la.link == nil || *la.link != (identitysync.Identity{Issuer: r.cfg.Fixtures.Issuer, Subject: subject}):
		problems = append(problems, fmt.Sprintf("account %s does not show its link to the first subject", a))
	}
	switch lb, ok := found[b]; {
	case !ok:
		problems = append(problems, fmt.Sprintf("account %s is no longer listed", b))
	case lb.link != nil:
		problems = append(problems, fmt.Sprintf("account %s became linked by a refused request", b))
	}
	return len(problems) == 0, strings.Join(problems, "; ")
}

func statusOf(ok bool, detail string) (Status, string) {
	if ok {
		return Pass, ""
	}
	return Fail, detail
}

// ---- Sessions -------------------------------------------------------------

func (r *runner) sessions(ctx context.Context, linked bool) {
	const area = "sessions"
	f := r.cfg.Fixtures
	ids := []struct{ id, name string }{
		{"sessions.probe-create", "the probe creates a session for the linked account (201)"},
		{"sessions.probe-active", "the probe reports the new session as usable (200)"},
		{"sessions.probe-unlinked", "the probe refuses an unlinked identity (404)"},
		{"sessions.revoke", "revoking the linked identity's sessions returns 204"},
		{"sessions.revoked-session-rejected", "a session created before the revoke is rejected afterwards (401)"},
		{"sessions.revoke-repeat", "revoking again returns 204"},
		{"sessions.revoke-unknown", "revoking an unlinked identity returns 204"},
		{"sessions.wrong-issuer", "revoking with another issuer returns 400"},
		{"sessions.link-kept", "revocation keeps the link"},
	}
	name := map[string]string{}
	for _, x := range ids {
		name[x.id] = x.name
	}
	if !r.revoke {
		r.add("sessions.supported", area, "the application implements sessions.revoke", Fail,
			"the descriptor does not advertise sessions.revoke; access removal must end sessions immediately")
		for _, x := range ids {
			r.add(x.id, area, x.name, Skip, "blocked by sessions.supported")
		}
		return
	}
	r.add("sessions.supported", area, "the application implements sessions.revoke", Pass, "")
	revoke := func(issuer, subject string) string {
		_, p := r.expect(ctx, request{method: http.MethodPost, path: "/sessions/revoke", token: r.cfg.Tokens.Sync, body: r.identityBody(issuer, subject)}, 204)
		return p
	}
	result := func(id, problem string) bool {
		if problem == "" {
			return r.add(id, area, name[id], Pass, "")
		}
		return r.add(id, area, name[id], Fail, problem)
	}
	skip := func(id, why string) { r.add(id, area, name[id], Skip, why) }

	sessionToken := ""
	switch {
	case !r.cfg.Probe:
		for _, id := range []string{"sessions.probe-create", "sessions.probe-active", "sessions.probe-unlinked"} {
			skip(id, "the session probe is not enabled for this run")
		}
	case !linked:
		for _, id := range []string{"sessions.probe-create", "sessions.probe-active", "sessions.probe-unlinked"} {
			skip(id, "blocked by linking.link")
		}
	default:
		ex, p := r.expect(ctx, request{method: http.MethodPost, path: "/test/sessions", token: r.cfg.Tokens.Sync, body: r.identityBody(f.Issuer, r.s1)}, 201)
		if p == "" {
			var out struct {
				SessionToken string `json:"sessionToken"`
			}
			if err := json.Unmarshal(ex.body, &out); err != nil || out.SessionToken == "" {
				p = "probe response lacks sessionToken"
			}
			sessionToken = out.SessionToken
		}
		if result("sessions.probe-create", p) {
			_, p = r.expect(ctx, r.probeCheck(sessionToken), 200)
			if !result("sessions.probe-active", p) {
				sessionToken = ""
			}
			// Only meaningful once the route is known to exist: an absent
			// probe route also answers 404.
			_, p = r.expect(ctx, request{method: http.MethodPost, path: "/test/sessions", token: r.cfg.Tokens.Sync, body: r.identityBody(f.Issuer, r.s3)}, 404)
			result("sessions.probe-unlinked", p)
		} else {
			sessionToken = ""
			skip("sessions.probe-active", "blocked by sessions.probe-create")
			skip("sessions.probe-unlinked", "blocked by sessions.probe-create")
		}
	}

	if linked {
		result("sessions.revoke", revoke(f.Issuer, r.s1))
	} else {
		skip("sessions.revoke", "blocked by linking.link")
	}
	switch {
	case !r.cfg.Probe:
		skip("sessions.revoked-session-rejected", "the session probe is not enabled for this run; revocation's effect is not verified")
	case sessionToken == "":
		skip("sessions.revoked-session-rejected", "blocked by sessions.probe-active")
	default:
		_, p := r.expect(ctx, r.probeCheck(sessionToken), 401)
		result("sessions.revoked-session-rejected", p)
	}
	if linked {
		result("sessions.revoke-repeat", revoke(f.Issuer, r.s1))
	} else {
		skip("sessions.revoke-repeat", "blocked by linking.link")
	}
	result("sessions.revoke-unknown", revoke(f.Issuer, r.s3))
	_, p := r.expect(ctx, request{method: http.MethodPost, path: "/sessions/revoke", token: r.cfg.Tokens.Sync, body: r.identityBody(wrongIssuer, r.s1)}, 400)
	result("sessions.wrong-issuer", p)
	if linked {
		ok, detail := r.linkState(ctx, f.LinkableAccounts[0], f.LinkableAccounts[1], r.s1)
		if ok {
			detail = ""
		}
		result("sessions.link-kept", detail)
	} else {
		skip("sessions.link-kept", "blocked by linking.link")
	}
}

func (r *runner) probeCheck(sessionToken string) request {
	return request{method: http.MethodGet, path: "/test/sessions/check", token: r.cfg.Tokens.Sync,
		headers: map[string]string{"X-Test-Session": sessionToken}}
}

// ---- Transport ------------------------------------------------------------

func (r *runner) transport(ctx context.Context, linked bool) {
	const area = "transport"
	f := r.cfg.Fixtures

	// Body size: a link that would otherwise be an idempotent 204, padded with
	// JSON whitespace past 4 KiB, must be refused.
	name := "a request body over 4 KiB is refused (400 or 413)"
	if linked {
		body := append(r.identityBody(f.Issuer, r.s1), bytes.Repeat([]byte(" "), maxRequestBody)...)
		_, p := r.expect(ctx, request{method: http.MethodPut, path: r.linkPath(f.LinkableAccounts[0]), token: r.cfg.Tokens.Sync, body: body},
			http.StatusBadRequest, http.StatusRequestEntityTooLarge)
		if p == "" {
			r.add("transport.request-body-limit", area, name, Pass, "")
		} else {
			r.add("transport.request-body-limit", area, name, Fail, p)
		}
	} else {
		r.add("transport.request-body-limit", area, name, Skip, "blocked by linking.link")
	}

	var noStore, redirects, large, errorBodies []string
	for _, ex := range r.seen {
		if !hasNoStore(ex.header) {
			noStore = append(noStore, fmt.Sprintf("%s (%d)", ex.label, ex.status))
		}
		if ex.status >= 300 && ex.status < 400 {
			redirects = append(redirects, fmt.Sprintf("%s (%d)", ex.label, ex.status))
		}
		if ex.tooLarge {
			large = append(large, ex.label)
		}
		if p := r.errorBodyProblem(ex); p != "" {
			errorBodies = append(errorBodies, p)
		}
	}
	r.check("transport.no-store", area, "every response sets Cache-Control: no-store", summarize(noStore, len(r.seen)))
	r.check("transport.no-redirects", area, "no endpoint redirects", summarize(redirects, len(r.seen)))
	r.check("transport.response-size", area, "every response body is at most 256 KiB", summarize(large, len(r.seen)))
	r.check("transport.error-bodies", area, "error bodies follow the contract and never echo tokens", summarize(errorBodies, len(r.seen)))
	if strings.HasPrefix(r.base, "https://") {
		r.add("transport.https", area, "the base URL uses HTTPS", Pass, "")
	} else {
		r.add("transport.https", area, "the base URL uses HTTPS", Skip, "run with an http:// base URL; HTTPS not verified")
	}
}

// ---- Retries --------------------------------------------------------------

// The dashboard retries network errors, timeouts, 429, 502, 503 and 504 (the
// contract's "Errors and retries"). These checks show the application is safe
// to retry: a repeated read returns the same page, a link or revoke sent again
// while the first is still in flight (a client that timed out) succeeds instead
// of conflicting, and overload answers say when to come back.

const (
	concurrentRetries = 5
	maxSettleAttempts = 3
	maxRetryAfterWait = 5 * time.Second
	defaultRetryWait  = 500 * time.Millisecond
	replayPageLimit   = 7
)

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

// retryAfter parses a Retry-After header (delay seconds or an HTTP date).
func retryAfter(h http.Header) (time.Duration, bool) {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if at, err := http.ParseTime(v); err == nil {
		return max(time.Until(at), 0), true
	}
	return 0, false
}

// settle performs q and, while the answer is retryable, retries it the way the
// dashboard does (Retry-After honoured, capped for the suite).
func (r *runner) settle(ctx context.Context, q request) (exchange, error) {
	var ex exchange
	var err error
	for attempt := 1; attempt <= maxSettleAttempts; attempt++ {
		ex, err = r.do(ctx, q)
		if err != nil || !retryableStatus(ex.status) || attempt == maxSettleAttempts {
			return ex, err
		}
		wait, ok := retryAfter(ex.header)
		if !ok {
			wait = defaultRetryWait
		}
		select {
		case <-ctx.Done():
			return ex, ctx.Err()
		case <-time.After(min(wait, maxRetryAfterWait)):
		}
	}
	return ex, err
}

// concurrent sends q n times at once and reports every answer that is not want
// once retryable answers have been retried.
func (r *runner) concurrent(ctx context.Context, q request, n, want int) []string {
	if q.token != nil {
		token, err := q.token(ctx)
		if err != nil {
			return []string{"get token: " + err.Error()}
		}
		q.token = StaticToken(token)
	}
	results := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ex, err := r.settle(ctx, q)
			switch {
			case err != nil:
				results[i] = err.Error()
			case ex.status != want:
				results[i] = fmt.Sprintf("%s returned %d, want %d", ex.label, ex.status, want)
			}
		}()
	}
	wg.Wait()
	var problems []string
	for _, p := range results {
		if p != "" {
			problems = append(problems, p)
		}
	}
	return problems
}

func sameAccounts(a, b []listedAccount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].id != b[i].id || (a[i].link == nil) != (b[i].link == nil) || a[i].link != nil && *a[i].link != *b[i].link {
			return false
		}
	}
	return true
}

// pageReplay reads the first two pages twice each and compares them.
func (r *runner) pageReplay(ctx context.Context) []string {
	read := func(cursor string) ([]listedAccount, string, string) {
		q := url.Values{"limit": {strconv.Itoa(replayPageLimit)}}
		if cursor != "" {
			q.Set("after", cursor)
		}
		ex, err := r.settle(ctx, request{method: http.MethodGet, path: "/accounts?" + q.Encode(), token: r.cfg.Tokens.Sync})
		if err != nil {
			return nil, "", err.Error()
		}
		if ex.status != http.StatusOK {
			return nil, "", fmt.Sprintf("%s returned %d, want 200", ex.label, ex.status)
		}
		accounts, next, err := r.parsePage(ex.body, replayPageLimit)
		if err != nil {
			return nil, "", err.Error()
		}
		return accounts, next, ""
	}
	var problems []string
	cursor := ""
	for page := 1; page <= 2; page++ {
		first, next, p := read(cursor)
		if p != "" {
			return append(problems, p)
		}
		again, nextAgain, p := read(cursor)
		if p != "" {
			return append(problems, p)
		}
		if !sameAccounts(first, again) || next != nextAgain {
			problems = append(problems, fmt.Sprintf("page %d changed when read again with the same cursor", page))
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return problems
}

func (r *runner) retries(ctx context.Context, linked bool) {
	const area = "retries"
	f := r.cfg.Fixtures
	r.check("retries.page-replay", area, "reading a page again with the same cursor returns the same accounts and cursor", r.pageReplay(ctx))

	name := fmt.Sprintf("%d identical link requests sent at once all return 204 and leave one link", concurrentRetries)
	if linked {
		b := f.LinkableAccounts[1]
		problems := r.concurrent(ctx, request{method: http.MethodPut, path: r.linkPath(b), token: r.cfg.Tokens.Sync,
			body: r.identityBody(f.Issuer, r.s2)}, concurrentRetries, http.StatusNoContent)
		if len(problems) == 0 {
			found, err := r.findAccounts(ctx, b)
			switch lb, ok := found[b]; {
			case err != nil:
				problems = append(problems, err.Error())
			case !ok:
				problems = append(problems, fmt.Sprintf("account %s is no longer listed", b))
			case lb.link == nil || *lb.link != (identitysync.Identity{Issuer: f.Issuer, Subject: r.s2}):
				problems = append(problems, fmt.Sprintf("account %s does not show exactly the requested link", b))
			}
		}
		r.check("retries.link-concurrent", area, name, summarize(problems, concurrentRetries))
	} else {
		r.add("retries.link-concurrent", area, name, Skip, "blocked by linking.link")
	}

	name = fmt.Sprintf("%d identical session revokes sent at once all return 204", concurrentRetries)
	switch {
	case !r.revoke:
		r.add("retries.revoke-concurrent", area, name, Skip, "the descriptor does not list sessions.revoke")
	case !linked:
		r.add("retries.revoke-concurrent", area, name, Skip, "blocked by linking.link")
	default:
		problems := r.concurrent(ctx, request{method: http.MethodPost, path: "/sessions/revoke", token: r.cfg.Tokens.Sync,
			body: r.identityBody(f.Issuer, r.s1)}, concurrentRetries, http.StatusNoContent)
		r.check("retries.revoke-concurrent", area, name, summarize(problems, concurrentRetries))
	}

	var overload []string
	count := 0
	r.mu.Lock()
	for _, ex := range r.seen {
		if ex.status != http.StatusTooManyRequests && ex.status != http.StatusServiceUnavailable {
			continue
		}
		count++
		if _, ok := retryAfter(ex.header); !ok {
			overload = append(overload, fmt.Sprintf("%s (%d)", ex.label, ex.status))
		}
	}
	r.mu.Unlock()
	name = "every 429 or 503 answer carries a valid Retry-After"
	switch {
	case len(overload) > 0:
		r.add("retries.retry-after", area, name, Fail, "without Retry-After: "+strings.Join(summarize(overload, count), "; "))
	case count == 0:
		r.add("retries.retry-after", area, name, Pass, "no 429 or 503 answers were seen during the run")
	default:
		r.add("retries.retry-after", area, name, Pass, "")
	}
}

func (r *runner) errorBodyProblem(ex exchange) string {
	for token := range r.tokens {
		if len(token) >= 8 && bytes.Contains(ex.body, []byte(token)) {
			return ex.label + " echoes a bearer token"
		}
	}
	if ex.status < 400 || len(bytes.TrimSpace(ex.body)) == 0 {
		return ""
	}
	mediaType, _, _ := mime.ParseMediaType(ex.header.Get("Content-Type"))
	if mediaType != "application/json" {
		return ""
	}
	var body struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(ex.body, &body); err != nil || body.Error == nil {
		return fmt.Sprintf("%s (%d) has a JSON body that is not {\"error\":{...}}", ex.label, ex.status)
	}
	if !errorCodes[body.Error.Code] {
		return fmt.Sprintf("%s (%d) uses unknown error code %q", ex.label, ex.status, body.Error.Code)
	}
	if len(body.Error.Message) > 500 {
		return fmt.Sprintf("%s (%d) error message exceeds 500 bytes", ex.label, ex.status)
	}
	return ""
}

func hasNoStore(h http.Header) bool {
	for _, v := range h.Values("Cache-Control") {
		for _, directive := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
				return true
			}
		}
	}
	return false
}

func summarize(problems []string, total int) []string {
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	shown := problems
	if len(shown) > 5 {
		shown = shown[:5]
	}
	return []string{fmt.Sprintf("%d of %d responses: %s", len(problems), total, strings.Join(shown, ", "))}
}
