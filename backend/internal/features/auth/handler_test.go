package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/service"
)

type fakeAuthService struct {
	processAuthCodeCalled bool
	getMeToken            string
	getMeUser             *keycloak.AuthUser
	getMeErr              error
	refreshToken          string
	refreshTokens         *keycloak.TokenResponse
	refreshErr            error
	logoutToken           string
	logoutURL             string
}

func (f *fakeAuthService) ProcessAuthCode(code string, codeVerifier string) (*keycloak.TokenResponse, error) {
	f.processAuthCodeCalled = true
	return nil, errors.New("not implemented")
}

func (f *fakeAuthService) GetAccessToken(refreshToken string) (*keycloak.TokenResponse, error) {
	f.refreshToken = refreshToken
	return f.refreshTokens, f.refreshErr
}

func (f *fakeAuthService) GetMe(accessToken string) (*keycloak.AuthUser, error) {
	f.getMeToken = accessToken
	return f.getMeUser, f.getMeErr
}

func (f *fakeAuthService) SaveSession(userID, access, refresh string, expires int) error {
	return nil
}

func (f *fakeAuthService) LogOut(refreshToken string) error {
	f.logoutToken = refreshToken
	return nil
}

func (f *fakeAuthService) GetLogoutURL(idTokenHint, postLogoutRedirectURI string) string {
	if f.logoutURL != "" {
		return f.logoutURL
	}
	return postLogoutRedirectURI
}

func TestHandleAuthLoginRedirectsToKeycloakWithPKCEAndState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newTestHandler(&fakeAuthService{})
	router := gin.New()
	router.GET("/auth/login", handler.HandleAuthLogin)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)

	router.ServeHTTP(res, req)

	if res.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, res.Code)
	}

	location := res.Header().Get("Location")
	if location == "" {
		t.Fatal("expected redirect Location header")
	}

	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}

	expectedPath := "/realms/moh-realm/protocol/openid-connect/auth"
	if redirectURL.Scheme != "https" || redirectURL.Host != "auth.example.test" || redirectURL.Path != expectedPath {
		t.Fatalf("unexpected Keycloak redirect URL: %s", location)
	}

	query := redirectURL.Query()
	assertQueryValue(t, query, "client_id", "dashboard-web")
	assertQueryValue(t, query, "response_type", "code")
	assertQueryValue(t, query, "scope", "openid profile email")
	assertQueryValue(t, query, "redirect_uri", "https://portal.example.test/api/v1/auth/callback")
	assertQueryValue(t, query, "code_challenge_method", "S256")

	if query.Get("state") == "" {
		t.Fatal("expected state query parameter")
	}
	if query.Get("code_challenge") == "" {
		t.Fatal("expected code_challenge query parameter")
	}

	assertSetCookiePresent(t, res, cookieOAuthState)
	assertSetCookiePresent(t, res, cookiePKCEVerifier)
}

func TestHandleAuthLoginReusesExistingFlowCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newTestHandler(&fakeAuthService{})
	router := gin.New()
	router.GET("/auth/login", handler.HandleAuthLogin)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	req.AddCookie(&http.Cookie{Name: cookieOAuthState, Value: "existing-state"})
	req.AddCookie(&http.Cookie{Name: cookiePKCEVerifier, Value: "existing-verifier"})
	req.AddCookie(&http.Cookie{Name: cookieOAuthReturnTo, Value: encodeReturnCookie("old-state", "https://portal.example.test/admin/home")})

	router.ServeHTTP(res, req)

	location := res.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	assertQueryValue(t, redirectURL.Query(), "state", "existing-state")

	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == cookieOAuthState || cookie.Name == cookiePKCEVerifier {
			t.Fatalf("expected login to reuse existing %s cookie without re-setting it", cookie.Name)
		}
	}
	if !strings.Contains(strings.Join(res.Header().Values("Set-Cookie"), "\n"), cookieOAuthReturnTo+"=;") {
		t.Fatal("expected login without returnTo to clear a stale destination")
	}
}

func TestHandleAuthLoginStoresStateBoundReturnTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newTestHandler(&fakeAuthService{})
	router := gin.New()
	router.GET("/auth/launch", handler.HandleAuthLaunch)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/launch?returnTo=%2Fapps%2Fdwh%2Fdashboards%3Ftab%3Dmine", nil)
	router.ServeHTTP(res, req)

	if res.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d: %s", http.StatusTemporaryRedirect, res.Code, res.Body.String())
	}

	var state, encodedReturn string
	for _, cookie := range res.Result().Cookies() {
		switch cookie.Name {
		case cookieOAuthState:
			state = cookie.Value
		case cookieOAuthReturnTo:
			encodedReturn = cookie.Value
		}
	}
	if state == "" || encodedReturn == "" {
		t.Fatal("expected state and return target cookies")
	}
	if got := decodeReturnCookie(encodedReturn, state); got != "https://portal.example.test/apps/dwh/dashboards?tab=mine" {
		t.Fatalf("unexpected normalized return target %q", got)
	}
	if got := decodeReturnCookie(encodedReturn, "different-state"); got != "" {
		t.Fatalf("expected state mismatch to reject return target, got %q", got)
	}
}

func TestHandleAuthLoginRejectsUnsafeReturnTargets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unsafeTargets := []string{
		"https://evil.example/apps/news",
		"//evil.example/apps/news",
		"apps/news",
		"/api/v1/auth/logout",
		"/ssobackend/api/v1/auth/logout",
		"/backend/api/v1/auth/logout",
		"/swagger/index.html",
		"/health/ready",
		"/version",
		"/apps/../admin/home",
		"/apps/%2e%2e/admin/home",
		"/apps%2f..%2fadmin/home",
		"/apps/%252e%252e/admin",
		"/apps/%5Cevil",
		"/portal/../admin/home",
		"/portal/../api/v1/auth/logout",
	}

	for _, target := range unsafeTargets {
		t.Run(target, func(t *testing.T) {
			handler := newTestHandler(&fakeAuthService{})
			router := gin.New()
			router.GET("/auth/login", handler.HandleAuthLogin)

			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/auth/login?returnTo="+url.QueryEscape(target), nil)
			router.ServeHTTP(res, req)

			assertErrorCode(t, res, http.StatusBadRequest, "INVALID_INPUT")
		})
	}
}

func TestNormalizeReturnURLAllowsConfiguredAbsoluteOrigin(t *testing.T) {
	handler := newTestHandler(&fakeAuthService{})
	handler.config.AuthReturnURLAllowedOrigins = "https://portal.example.test, https://staging.example.test"

	got, err := handler.normalizeReturnURL("https://staging.example.test/apps/news#latest")
	if err != nil {
		t.Fatalf("normalize allowed absolute return target: %v", err)
	}
	if got != "https://staging.example.test/apps/news#latest" {
		t.Fatalf("unexpected normalized target %q", got)
	}
}

func TestNormalizeReturnURLMapsLegacyPortalPrefix(t *testing.T) {
	handler := newTestHandler(&fakeAuthService{})

	for raw, want := range map[string]string{
		"/":                     "https://portal.example.test/",
		"/admin/home":           "https://portal.example.test/admin/home",
		"/portal":               "https://portal.example.test/",
		"/portal/":              "https://portal.example.test/",
		"/portal/apps/news?x=1": "https://portal.example.test/apps/news?x=1",
		"/portalish/apps":       "https://portal.example.test/portalish/apps",
		"/apis-catalog":         "https://portal.example.test/apis-catalog",
	} {
		got, err := handler.normalizeReturnURL(raw)
		if err != nil {
			t.Fatalf("normalize %q: %v", raw, err)
		}
		if got != want {
			t.Fatalf("normalize %q = %q, want %q", raw, got, want)
		}
	}
}

func TestHandleAuthGetMeRequiresToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newTestHandler(&fakeAuthService{})
	router := gin.New()
	router.GET("/auth/me", handler.HandleAuthGetMe)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)

	router.ServeHTTP(res, req)

	assertErrorCode(t, res, http.StatusUnauthorized, "UNAUTHORIZED")
}

func TestHandleAuthGetMeReturnsUserFromLegacyAccessCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authSvc := &fakeAuthService{
		getMeUser: &keycloak.AuthUser{
			ID:         "user-1",
			Username:   "testuser",
			Email:      "testuser@example.test",
			RealmRoles: []string{"user"},
			ClientRoles: map[string][]string{
				"dashboard-web": {"dashboard-web_access"},
			},
		},
	}
	handler := newTestHandler(authSvc)
	router := gin.New()
	router.GET("/auth/me", handler.HandleAuthGetMe)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieAccessToken, Value: "legacy-access-token"})

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}
	if authSvc.getMeToken != "legacy-access-token" {
		t.Fatalf("expected GetMe token %q, got %q", "legacy-access-token", authSvc.getMeToken)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			User struct {
				Username string `json:"username"`
				Email    string `json:"email"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if !body.Success {
		t.Fatal("expected success response")
	}
	if body.Data.User.Username != "testuser" || body.Data.User.Email != "testuser@example.test" {
		t.Fatalf("unexpected user response: %+v", body.Data.User)
	}
}

func TestHandleAuthGetMeRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newTestHandler(&fakeAuthService{getMeErr: errors.New("invalid token")})
	router := gin.New()
	router.GET("/auth/me", handler.HandleAuthGetMe)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieAccessToken, Value: "bad-token"})

	router.ServeHTTP(res, req)

	assertErrorCode(t, res, http.StatusUnauthorized, "TOKEN_INVALID")
}

func TestHandleAuthCallbackRejectsMissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authSvc := &fakeAuthService{}
	handler := newTestHandler(authSvc)
	router := gin.New()
	router.GET("/auth/callback", handler.HandleAuthCallback)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=abc", nil)

	router.ServeHTTP(res, req)

	if res.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect status, got %d", res.Code)
	}
	if got := res.Header().Get("Location"); got != "https://portal.example.test/" {
		t.Fatalf("expected frontend redirect, got %q", got)
	}
	if authSvc.processAuthCodeCalled {
		t.Fatal("expected missing code to stop before token exchange")
	}
	if !strings.Contains(strings.Join(res.Header().Values("Set-Cookie"), "\n"), cookieOAuthReturnTo+"=;") {
		t.Fatal("expected terminal callback failure to clear the return target")
	}
}

func TestHandleAuthCallbackRejectsStateMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authSvc := &fakeAuthService{}
	handler := newTestHandler(authSvc)
	router := gin.New()
	router.GET("/auth/callback", handler.HandleAuthCallback)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=returned", nil)
	req.AddCookie(&http.Cookie{Name: cookieOAuthState, Value: "cookie-state"})
	req.AddCookie(&http.Cookie{Name: cookiePKCEVerifier, Value: "verifier"})

	router.ServeHTTP(res, req)

	if res.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect status, got %d", res.Code)
	}
	if authSvc.processAuthCodeCalled {
		t.Fatal("expected state mismatch to stop before token exchange")
	}
}

func TestHandleAuthRefreshTokenRequiresSessionOrLegacyRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newTestHandler(&fakeAuthService{})
	router := gin.New()
	router.POST("/auth/refresh", handler.HandleAuthRefreshToken)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)

	router.ServeHTTP(res, req)

	assertErrorCode(t, res, http.StatusUnauthorized, "TOKEN_EXPIRED")
}

func TestSessionTTLUsesRefreshExpiryThenAccessExpiryThenDefault(t *testing.T) {
	handler := newTestHandler(&fakeAuthService{})

	if got := handler.sessionTTL(120, 60).Seconds(); got != 120 {
		t.Fatalf("expected refresh expiry ttl 120s, got %.0fs", got)
	}
	if got := handler.sessionTTL(0, 60).Seconds(); got != 60 {
		t.Fatalf("expected access expiry ttl 60s, got %.0fs", got)
	}
	if got := handler.sessionTTL(0, 0); got != defaultSessionTTL {
		t.Fatalf("expected default ttl %s, got %s", defaultSessionTTL, got)
	}
}

func newTestHandler(authSvc service.AuthService) *Handler {
	cfg := &config.Config{
		Environment:           "test",
		FrontendBaseURL:       "https://portal.example.test",
		KeycloakExternalURL:   "https://auth.example.test",
		KeycloakInternalURL:   "https://keycloak.internal.test",
		KeycloakRealm:         "moh-realm",
		KeycloakWebClientID:   "dashboard-web",
		KeycloakRedirectURI:   "https://portal.example.test/api/v1/auth/callback",
		KeycloakAdminClientID: "dashboard-admin",
	}

	return NewHandler(
		authSvc,
		service.NewAuditService(nil, nil, nil, cfg),
		nil,
		nil,
		cfg,
	)
}

func assertQueryValue(t *testing.T, values url.Values, key string, want string) {
	t.Helper()

	if got := values.Get(key); got != want {
		t.Fatalf("expected query %s=%q, got %q", key, want, got)
	}
}

func assertSetCookiePresent(t *testing.T, res *httptest.ResponseRecorder, name string) {
	t.Helper()

	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == name && cookie.Value != "" {
			return
		}
	}

	t.Fatalf("expected Set-Cookie for %s", name)
}

func assertErrorCode(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	if res.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, res.Code, res.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if body.Success {
		t.Fatal("expected failure response")
	}
	if body.Error.Code != code {
		t.Fatalf("expected error code %q, got %q", code, body.Error.Code)
	}
}

func TestCookieSameSiteDefaultsToLax(t *testing.T) {
	handler := newTestHandler(&fakeAuthService{})

	if got := handler.cookieSameSiteString(); got != "Lax" {
		t.Fatalf("expected SameSite Lax, got %s", got)
	}
}

func TestIsCodeAlreadyUsedError(t *testing.T) {
	handler := newTestHandler(&fakeAuthService{})

	for _, message := range []string{
		"invalid_grant",
		"Code not valid",
		"code already used",
	} {
		if !handler.isCodeAlreadyUsedError(errors.New(message)) {
			t.Fatalf("expected %q to be treated as already-used code", message)
		}
	}

	if handler.isCodeAlreadyUsedError(errors.New("network timeout")) {
		t.Fatal("did not expect unrelated error to be treated as already-used code")
	}
}

func TestHasAnyAuthCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestHandler(&fakeAuthService{})

	res := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(res)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	if handler.hasAnyAuthCookie(c) {
		t.Fatal("expected no auth cookies")
	}

	c.Request.AddCookie(&http.Cookie{Name: cookieSession, Value: "session-id"})
	if !handler.hasAnyAuthCookie(c) {
		t.Fatal("expected session cookie to count as an auth cookie")
	}
}

func TestClearOAuthCookiesExpiresFlowCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestHandler(&fakeAuthService{})

	res := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(res)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler.clearOAuthCookies(c)

	header := strings.Join(res.Header().Values("Set-Cookie"), "\n")
	if !strings.Contains(header, cookieOAuthState+"=;") {
		t.Fatalf("expected oauth state cookie to be cleared, got %q", header)
	}
	if !strings.Contains(header, cookiePKCEVerifier+"=;") {
		t.Fatalf("expected pkce verifier cookie to be cleared, got %q", header)
	}
	if !strings.Contains(header, cookieOAuthReturnTo+"=;") {
		t.Fatalf("expected oauth return target cookie to be cleared, got %q", header)
	}
}
