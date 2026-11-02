package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func validToken() Token {
	return Token{TokenType: "Bearer", Active: true, Issuer: "https://identity.test/realms/moh", Subject: "subject", Audience: jwt.ClaimStrings{"moh-platform-api"}, ClientID: "pilot-service", Username: "service-account-pilot-service", Scope: "identity:read notifications:write", ExpiresAt: time.Now().Add(time.Minute).Unix()}
}
func TestIntrospectionValidatesIdentityContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Token)
		want   error
	}{
		{"valid", func(*Token) {}, nil},
		{"revoked", func(v *Token) { v.Active = false }, ErrUnauthorized},
		{"expired", func(v *Token) { v.ExpiresAt = 1 }, ErrUnauthorized},
		{"missing expiry", func(v *Token) { v.ExpiresAt = 0 }, ErrUnauthorized},
		{"future", func(v *Token) { v.NotBefore = time.Now().Add(time.Hour).Unix() }, ErrUnauthorized},
		{"issuer", func(v *Token) { v.Issuer = "https://other.test" }, ErrUnauthorized},
		{"audience", func(v *Token) { v.Audience = jwt.ClaimStrings{"other-api"} }, ErrUnauthorized},
		{"party", func(v *Token) { v.AuthorizedParty = "other-client" }, ErrUnauthorized},
		{"refresh", func(v *Token) { v.TokenType = "Refresh" }, ErrUnauthorized},
		{"subject", func(v *Token) { v.Subject = "" }, ErrUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := validToken()
			tc.mutate(&token)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id, secret, ok := r.BasicAuth()
				if !ok || id != "introspector" || secret != "test-secret" {
					t.Error("missing introspector authentication")
				}
				if r.Method != http.MethodPost || r.FormValue("token") != "opaque-access-token" {
					t.Error("invalid introspection request")
				}
				_ = json.NewEncoder(w).Encode(token)
			}))
			defer server.Close()
			v := &Introspector{Endpoint: server.URL, Issuer: validToken().Issuer, Audience: "moh-platform-api", ClientID: "introspector", Secret: "test-secret"}
			_, err := v.Verify(context.Background(), "opaque-access-token")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
func TestIntrospectionFailsClosedAndDoesNotFollowRedirects(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Location", "/leaked")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	v := &Introspector{Endpoint: server.URL, Issuer: "issuer", Audience: "aud", ClientID: "client", Secret: "secret"}
	if _, err := v.Verify(context.Background(), "token"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatal("followed a credential-bearing redirect")
	}
	v.Secret = ""
	if _, err := v.Verify(context.Background(), "token"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
func TestScopesRequireBothTokenAndRegistryGrant(t *testing.T) {
	scopes := intersectScopes([]string{IdentityRead, NotificationsRead}, "identity:read notifications:write")
	if len(scopes) != 1 || scopes[0] != IdentityRead {
		t.Fatal(scopes)
	}
}

type testVerifier struct {
	token Token
	err   error
}

func (v testVerifier) Verify(context.Context, string) (Token, error) { return v.token, v.err }

type testRepository struct {
	Repository
	principal   Principal
	err         error
	resolutions int
}

func (r *testRepository) ResolveIdentity(context.Context, Token) (Principal, error) {
	r.resolutions++
	return r.principal, r.err
}
func TestMachineRoutesRejectCookiesScopesAndRevokedApplications(t *testing.T) {
	repo := &testRepository{principal: Principal{ApplicationID: "pilot", Subject: "subject", Scopes: []string{IdentityRead}}}
	h := &Handler{Repository: repo, Verifier: testVerifier{token: validToken()}}
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), router.Group("/api/v1"), h, nil)
	request := func(bearer bool) int {
		req := httptest.NewRequest("GET", "/api/v1/platform/identity", nil)
		req.AddCookie(&http.Cookie{Name: "sso_session", Value: "browser-cookie"})
		if bearer {
			req.Header.Set("Authorization", "Bearer token")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	if code := request(false); code != 401 {
		t.Fatal(code)
	}
	if code := request(true); code != 200 {
		t.Fatal(code)
	}
	repo.principal.Scopes = nil
	if code := request(true); code != 403 {
		t.Fatal(code)
	}
	repo.principal.Scopes = []string{IdentityRead}
	repo.err = ErrForbidden
	if code := request(true); code != 403 {
		t.Fatal(code)
	}
	if repo.resolutions != 3 {
		t.Fatal("identity must be resolved on each request")
	}
	req := httptest.NewRequest("GET", "/api/v1/platform/applications/pilot/access", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("machine token accepted as user")
	}
}
