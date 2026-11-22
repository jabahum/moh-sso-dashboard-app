package identityreconcile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

const (
	syncBase   = "/api/v1/internal/identity-sync"
	testIssuer = "https://sso.example.test/realms/moh"
	testUUID   = "19a3adb5-71bd-4f32-af89-e23a3f29fef1"
)

func descriptorJSON(version, app, format string, capabilities ...string) string {
	data, _ := json.Marshal(map[string]any{
		"contract": "moh-identity-sync", "version": version, "application": app,
		"accountIdFormat": format, "maxPageSize": 50, "capabilities": capabilities,
	})
	return string(data)
}

type recordedSleeps struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (r *recordedSleeps) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.delays = append(r.delays, d)
	return nil
}

// newTestConnector starts a TLS server and returns a connector whose retry sleeps are recorded.
func newTestConnector(t *testing.T, app string, format identitysync.AccountIDFormat, handler http.HandlerFunc) (*HTTPAccounts, *recordedSleeps) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	connector, err := NewHTTPAccounts(HTTPAccountsConfig{
		Application: app, Origin: server.URL, LegacyAccountIDFormat: format,
		Token: func(context.Context) (string, error) { return "fixture-token", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	connector.client.Transport = server.Client().Transport
	sleeps := &recordedSleeps{}
	connector.sleep = sleeps.sleep
	connector.jitter = func() float64 { return 0 }
	return connector, sleeps
}

func TestNewHTTPAccountsRejectsInvalidConfiguration(t *testing.T) {
	token := func(context.Context) (string, error) { return "t", nil }
	for name, cfg := range map[string]HTTPAccountsConfig{
		"http origin":      {Application: "app", Origin: "http://app.example", LegacyAccountIDFormat: identitysync.AccountIDOpaque, Token: token},
		"origin with path": {Application: "app", Origin: "https://app.example/x", LegacyAccountIDFormat: identitysync.AccountIDOpaque, Token: token},
		"userinfo":         {Application: "app", Origin: "https://u@app.example", LegacyAccountIDFormat: identitysync.AccountIDOpaque, Token: token},
		"bad port":         {Application: "app", Origin: "https://app.example:99999", LegacyAccountIDFormat: identitysync.AccountIDOpaque, Token: token},
		"no token":         {Application: "app", Origin: "https://app.example", LegacyAccountIDFormat: identitysync.AccountIDOpaque},
		"no application":   {Origin: "https://app.example", LegacyAccountIDFormat: identitysync.AccountIDOpaque, Token: token},
		"unknown format":   {Application: "app", Origin: "https://app.example", LegacyAccountIDFormat: "email", Token: token},
	} {
		if _, err := NewHTTPAccounts(cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := NewHTTPAccounts(HTTPAccountsConfig{Application: "app", Origin: "https://app.example:8443/", LegacyAccountIDFormat: identitysync.AccountIDOpaque, Token: token}); err != nil {
		t.Errorf("valid origin with port rejected: %v", err)
	}
}

func TestDescribeReadsAndValidatesDescriptor(t *testing.T) {
	body := descriptorJSON("1.1", "alerts", "opaque", "accounts.list", "identity.link", "sessions.revoke")
	connector, _ := newTestConnector(t, "alerts", identitysync.AccountIDUUID, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != syncBase || r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		_, _ = io.WriteString(w, body)
	})
	d, err := connector.Describe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != "1.1" || d.AccountIDFormat != identitysync.AccountIDOpaque || !d.Supports(identitysync.CapabilityRevokeSessions) {
		t.Fatalf("descriptor = %+v", d)
	}
}

func TestDescribeRejectsDescriptorForAnotherApplication(t *testing.T) {
	connector, _ := newTestConnector(t, "clims", identitysync.AccountIDUUID, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, descriptorJSON("1.1", "poesgo", "uuid", "accounts.list", "identity.link"))
	})
	if _, err := connector.Describe(context.Background()); !errors.Is(err, ErrContractViolation) {
		t.Fatalf("err = %v, want contract violation", err)
	}
}

func TestDescribeFallsBackToLegacyOn404(t *testing.T) {
	connector, _ := newTestConnector(t, "poesgo", identitysync.AccountIDPositiveInteger, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	d, err := connector.Describe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != "1.0" || d.AccountIDFormat != identitysync.AccountIDPositiveInteger || d.Supports(identitysync.CapabilityRevokeSessions) {
		t.Fatalf("legacy descriptor = %+v", d)
	}
}

func TestDescribeRefusesOtherStatusesAndRedirects(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusFound, http.StatusInternalServerError} {
		calls := 0
		connector, _ := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
			calls++
			if status == http.StatusFound {
				w.Header().Set("Location", "/elsewhere")
			}
			w.WriteHeader(status)
		})
		if _, err := connector.Describe(context.Background()); err == nil {
			t.Errorf("HTTP %d accepted", status)
		}
		if calls != 1 {
			t.Errorf("HTTP %d: %d requests, want 1 (not retried, redirect not followed)", status, calls)
		}
	}
}

func TestListAccounts11SendsLimitAndDecodesLinks(t *testing.T) {
	connector, _ := newTestConnector(t, "alerts", identitysync.AccountIDUUID, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case syncBase:
			_, _ = io.WriteString(w, descriptorJSON("1.1", "alerts", "opaque", "accounts.list", "identity.link"))
		case syncBase + "/accounts":
			// The descriptor caps pages at 50, so a request for 80 asks for 50.
			if got := r.URL.Query().Get("limit"); got != "50" {
				t.Errorf("limit = %q, want 50", got)
			}
			if got := r.URL.Query().Get("after"); got != "c~1" {
				t.Errorf("after = %q", got)
			}
			_, _ = io.WriteString(w, `{"accounts":[
				{"id":"user.a","email":"a@example.org","link":null},
				{"id":"user:b","email":"","link":{"issuer":"`+testIssuer+`","subject":"kc-b"}}
			],"nextCursor":"opaque~cursor.2"}`)
		}
	})
	page, err := connector.ListEligibleAccounts(context.Background(), "c~1", 80)
	if err != nil {
		t.Fatal(err)
	}
	want := identitysync.AccountPage{
		Accounts: []identitysync.Account{
			{ID: "user.a", Email: "a@example.org"},
			{ID: "user:b", Link: identitysync.Identity{Issuer: testIssuer, Subject: "kc-b"}},
		},
		NextCursor: "opaque~cursor.2",
	}
	if len(page.Accounts) != 2 || page.Accounts[0] != want.Accounts[0] || page.Accounts[1] != want.Accounts[1] || page.NextCursor != want.NextCursor {
		t.Fatalf("page = %+v", page)
	}
}

func TestListAccounts11RejectsContractViolations(t *testing.T) {
	for name, body := range map[string]string{
		"longer than limit": `{"accounts":[{"id":"a","email":""},{"id":"b","email":""}],"nextCursor":""}`,
		"invalid id":        `{"accounts":[{"id":"-bad","email":""}],"nextCursor":""}`,
		"invalid cursor":    `{"accounts":[],"nextCursor":"has space"}`,
		"empty link":        `{"accounts":[{"id":"a","email":"","link":{"issuer":"","subject":"x"}}],"nextCursor":""}`,
		"oversize body":     `{"accounts":[],"nextCursor":"","pad":"` + strings.Repeat("x", 300<<10) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			connector, _ := newTestConnector(t, "alerts", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == syncBase {
					_, _ = io.WriteString(w, descriptorJSON("1.1", "alerts", "opaque", "accounts.list", "identity.link"))
					return
				}
				_, _ = io.WriteString(w, body)
			})
			if _, err := connector.ListEligibleAccounts(context.Background(), "", 1); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestListAccounts10OmitsLimitIgnoresLinksAndTruncates(t *testing.T) {
	connector, _ := newTestConnector(t, "poesgo", identitysync.AccountIDPositiveInteger, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case syncBase:
			w.WriteHeader(http.StatusNotFound)
		case syncBase + "/accounts":
			if r.URL.Query().Has("limit") {
				t.Error("limit sent to a 1.0 application")
			}
			_, _ = io.WriteString(w, `{"accounts":[{"id":"5","email":"a@example.org","link":{"issuer":"`+testIssuer+`","subject":"s"}},{"id":"9","email":"b@example.org"}],"nextCursor":"9"}`)
		}
	})
	page, err := connector.ListEligibleAccounts(context.Background(), "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Accounts) != 1 || page.Accounts[0].ID != "5" || page.Accounts[0].Link != (identitysync.Identity{}) || page.NextCursor != "5" {
		t.Fatalf("page = %+v", page)
	}
}

func TestCursorAndIDValidationFollowFormat(t *testing.T) {
	for _, tc := range []struct {
		name    string
		format  identitysync.AccountIDFormat
		version string
		cursor  string
		ok      bool
	}{
		{"1.0 uuid cursor", identitysync.AccountIDUUID, "1.0", testUUID, true},
		{"1.0 uuid rejects integer", identitysync.AccountIDUUID, "1.0", "12", false},
		{"1.0 uuid rejects nil uuid", identitysync.AccountIDUUID, "1.0", "00000000-0000-0000-0000-000000000000", false},
		{"1.0 integer cursor", identitysync.AccountIDPositiveInteger, "1.0", "12", true},
		{"1.0 integer rejects zero padding", identitysync.AccountIDPositiveInteger, "1.0", "012", false},
		{"1.0 integer rejects uuid", identitysync.AccountIDPositiveInteger, "1.0", testUUID, false},
		{"1.0 opaque cursor", identitysync.AccountIDOpaque, "1.0", "user.a", true},
		{"1.0 opaque rejects tilde", identitysync.AccountIDOpaque, "1.0", "a~b", false},
		{"1.1 opaque cursor with tilde", identitysync.AccountIDPositiveInteger, "1.1", "a~b", true},
		{"1.1 cursor rejects slash", identitysync.AccountIDPositiveInteger, "1.1", "a/b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			connector, _ := newTestConnector(t, "app", tc.format, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == syncBase {
					if tc.version == "1.0" {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					_, _ = io.WriteString(w, descriptorJSON("1.1", "app", string(tc.format), "accounts.list", "identity.link"))
					return
				}
				requests++
				_, _ = io.WriteString(w, `{"accounts":[],"nextCursor":""}`)
			})
			_, err := connector.ListEligibleAccounts(context.Background(), tc.cursor, 10)
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
			if !tc.ok && requests != 0 {
				t.Fatal("invalid cursor was sent to the application")
			}
		})
	}
	// The account ID format comes from the 1.1 descriptor, not the registry.
	connector, _ := newTestConnector(t, "app", identitysync.AccountIDUUID, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == syncBase {
			_, _ = io.WriteString(w, descriptorJSON("1.1", "app", "positive-integer", "accounts.list", "identity.link"))
			return
		}
		_, _ = io.WriteString(w, `{"accounts":[{"id":"`+testUUID+`","email":""}],"nextCursor":""}`)
	})
	if _, err := connector.ListEligibleAccounts(context.Background(), "", 10); !errors.Is(err, ErrContractViolation) {
		t.Fatalf("uuid accepted for a positive-integer descriptor: %v", err)
	}
	if err := connector.LinkIdentity(context.Background(), testUUID, identitysync.Identity{Issuer: testIssuer, Subject: "s"}); err == nil {
		t.Fatal("link accepted an ID outside the descriptor format")
	}
}

func TestRetryPolicyRetriesTransientStatusesWithBackoff(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		attempts := 0
		connector, sleeps := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(status)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		if _, err := connector.Describe(context.Background()); err != nil {
			t.Fatalf("HTTP %d: %v", status, err)
		}
		if attempts != 3 {
			t.Fatalf("HTTP %d: attempts = %d, want 3", status, attempts)
		}
		// Zero jitter gives half of 500ms, then half of 1s.
		if len(sleeps.delays) != 2 || sleeps.delays[0] != 250*time.Millisecond || sleeps.delays[1] != 500*time.Millisecond {
			t.Fatalf("HTTP %d: delays = %v", status, sleeps.delays)
		}
	}
}

func TestRetryPolicyStopsAfterFiveAttempts(t *testing.T) {
	attempts := 0
	connector, sleeps := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	_, err := connector.Describe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "5 attempts") {
		t.Fatalf("err = %v", err)
	}
	if attempts != 5 || len(sleeps.delays) != 4 {
		t.Fatalf("attempts = %d, sleeps = %v", attempts, sleeps.delays)
	}
	for i := 1; i < len(sleeps.delays); i++ {
		if sleeps.delays[i] <= sleeps.delays[i-1] {
			t.Fatalf("backoff not exponential: %v", sleeps.delays)
		}
	}
}

func TestRetryPolicyHonoursRetryAfter(t *testing.T) {
	attempts := 0
	connector, sleeps := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := connector.Describe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sleeps.delays) != 1 || sleeps.delays[0] != 7*time.Second {
		t.Fatalf("delays = %v, want [7s]", sleeps.delays)
	}

	// A Retry-After longer than the connector will wait ends the call instead of retrying early.
	attempts = 0
	connector, sleeps = newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if _, err := connector.Describe(context.Background()); err == nil {
		t.Fatal("accepted")
	}
	if attempts != 1 || len(sleeps.delays) != 0 {
		t.Fatalf("attempts = %d, sleeps = %v", attempts, sleeps.delays)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for value, want := range map[string]time.Duration{
		"":                              0,
		"0":                             0,
		"-3":                            0,
		"12":                            12 * time.Second,
		"soon":                          0,
		"Fri, 02 Jan 2026 03:04:35 GMT": 30 * time.Second,
		"Fri, 02 Jan 2026 03:00:00 GMT": 0,
	} {
		if got := retryAfter(value, now); got != want {
			t.Errorf("retryAfter(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestRetryPolicyNeverRetriesOther4xx(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusRequestEntityTooLarge} {
		attempts := 0
		connector, sleeps := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == syncBase {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			attempts++
			w.WriteHeader(status)
		})
		err := connector.LinkIdentity(context.Background(), "acct", identitysync.Identity{Issuer: testIssuer, Subject: "s"})
		if err == nil {
			t.Fatalf("HTTP %d accepted", status)
		}
		if status == http.StatusConflict && !errors.Is(err, identitysync.ErrLinkConflict) {
			t.Fatalf("409 = %v", err)
		}
		if attempts != 1 || len(sleeps.delays) != 0 {
			t.Fatalf("HTTP %d: attempts = %d, sleeps = %v", status, attempts, sleeps.delays)
		}
	}
}

func TestRetryPolicyRetriesNetworkErrorsAndResendsBody(t *testing.T) {
	attempts := 0
	var bodies []string
	connector, sleeps := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == syncBase {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		attempts++
		data, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(data))
		if attempts == 1 {
			// Drop the connection without a response.
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijacker")
			}
			conn, _, _ := hijacker.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := connector.LinkIdentity(context.Background(), "acct", identitysync.Identity{Issuer: testIssuer, Subject: "s"}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || len(sleeps.delays) != 1 || bodies[0] != bodies[1] || !strings.Contains(bodies[1], `"subject":"s"`) {
		t.Fatalf("attempts = %d, sleeps = %v, bodies = %q", attempts, sleeps.delays, bodies)
	}
}

func TestRetryStopsWhenContextEnds(t *testing.T) {
	connector, _ := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ctx, cancel := context.WithCancel(context.Background())
	connector.sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return ctx.Err()
	}
	if _, err := connector.Describe(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestRevokeSessionsUnsupportedFor10AndMissingCapability(t *testing.T) {
	for name, descriptor := range map[string]string{
		"1.0":                "",
		"1.1 without revoke": descriptorJSON("1.1", "app", "opaque", "accounts.list", "identity.link"),
	} {
		t.Run(name, func(t *testing.T) {
			connector, _ := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != syncBase {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				if descriptor == "" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				_, _ = io.WriteString(w, descriptor)
			})
			err := connector.RevokeSessions(context.Background(), identitysync.Identity{Issuer: testIssuer, Subject: "s"})
			if !errors.Is(err, identitysync.ErrUnsupported) {
				t.Fatalf("err = %v, want ErrUnsupported", err)
			}
		})
	}
}

func TestRevokeSessionsSupported(t *testing.T) {
	var got map[string]string
	revokes := 0
	connector, _ := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == syncBase:
			_, _ = io.WriteString(w, descriptorJSON("1.1", "app", "opaque", "accounts.list", "identity.link", "sessions.revoke"))
		case r.Method == http.MethodPost && r.URL.Path == syncBase+"/sessions/revoke":
			revokes++
			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("missing JSON content type")
			}
			_ = json.NewDecoder(r.Body).Decode(&got)
			if revokes == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
	})
	identity := identitysync.Identity{Issuer: testIssuer, Subject: "kc-subject"}
	if err := connector.RevokeSessions(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	if revokes != 2 || got["issuer"] != testIssuer || got["subject"] != "kc-subject" {
		t.Fatalf("revokes = %d, body = %v", revokes, got)
	}
	if err := connector.RevokeSessions(context.Background(), identitysync.Identity{Issuer: testIssuer, Subject: " "}); err == nil {
		t.Fatal("blank subject accepted")
	}

	// A 400 (for example a wrong issuer) is reported, not retried.
	connector, sleeps := newTestConnector(t, "app", identitysync.AccountIDOpaque, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == syncBase {
			_, _ = io.WriteString(w, descriptorJSON("1.1", "app", "opaque", "accounts.list", "identity.link", "sessions.revoke"))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	})
	if err := connector.RevokeSessions(context.Background(), identity); err == nil || len(sleeps.delays) != 0 {
		t.Fatalf("err = %v, sleeps = %v", err, sleeps.delays)
	}
}
