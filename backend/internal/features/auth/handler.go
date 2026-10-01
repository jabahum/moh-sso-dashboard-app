package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/features/authsession"
	"github.com/moh-sso-dashboard/internal/http/apierror"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/internal/utils"
)

const (
	cookiePKCEVerifier = "pkce_verifier"
	cookieOAuthState   = "oauth_state"
	// cookieSession holds the opaque server-side session ID. Tokens live in
	// Redis — never in cookies — so response headers stay small enough for
	// default proxy buffers on intermediate hops.
	cookieSession = authsession.CookieName

	// Legacy JWT cookies from the pre-session scheme. Still read as a
	// fallback so logged-in browsers survive the transition, and cleared
	// on every login/logout/refresh.
	cookieAccessToken  = "access_token"
	cookieRefreshToken = "refresh_token"
	cookieIDToken      = "id_token"

	defaultSessionTTL = 30 * time.Minute
)

type sessionStore interface {
	Create(context.Context, authsession.Data, time.Duration) (string, error)
	Get(context.Context, string) (*authsession.Data, error)
	Update(context.Context, string, authsession.Data, time.Duration) error
	Delete(context.Context, string) error
}

type Handler struct {
	authService         service.AuthService
	auditService        *service.AuditService
	notificationService service.NotificationsService
	sessions            sessionStore
	config              *config.Config
	authzResolver       authz.PermissionResolver
}

func NewHandler(
	authService service.AuthService,
	auditService *service.AuditService,
	notificationService service.NotificationsService,
	sessions *authsession.Store,
	config *config.Config,
	authzResolver ...authz.PermissionResolver,
) *Handler {
	var resolver authz.PermissionResolver
	if len(authzResolver) > 0 {
		resolver = authzResolver[0]
	}

	return &Handler{
		authService:         authService,
		auditService:        auditService,
		notificationService: notificationService,
		sessions:            sessions,
		config:              config,
		authzResolver:       resolver,
	}
}

// ----------------------------------------------------
// LOGIN (redirect → Keycloak with PKCE + state)
// ----------------------------------------------------
func (h *Handler) HandleAuthLogin(c *gin.Context) {
	_ = h.auditService.LoginInitiated(
		c.Request.Context(),
		c.ClientIP(),
		c.Request.UserAgent(),
		h.config.KeycloakWebClientID,
	)

	// Reuse existing login-flow cookies if present.
	// This prevents double login redirects from overwriting oauth_state/pkce_verifier.
	codeVerifier, _ := c.Cookie(cookiePKCEVerifier)
	state, _ := c.Cookie(cookieOAuthState)

	newFlow := codeVerifier == "" || state == ""
	if newFlow {
		codeVerifier = utils.GenerateCodeVerifier()
		state = uuid.NewString()

		h.setCookie(c, cookiePKCEVerifier, codeVerifier, oauthFlowCookieTTL, true)
		h.setCookie(c, cookieOAuthState, state, oauthFlowCookieTTL, true)
	}

	if rawReturnTo, present := c.GetQuery("returnTo"); present {
		returnTo, err := h.normalizeReturnURL(rawReturnTo)
		if err != nil || returnTo == "" {
			h.clearCookie(c, cookieOAuthReturnTo, true)
			response.Fail(c, http.StatusBadRequest, "INVALID_INPUT", "invalid returnTo URL")
			return
		}
		h.setCookie(c, cookieOAuthReturnTo, encodeReturnCookie(state, returnTo), oauthFlowCookieTTL, true)
	} else {
		h.clearCookie(c, cookieOAuthReturnTo, true)
	}

	codeChallenge := utils.GenerateCodeChallenge(codeVerifier)

	keycloakBaseURL := strings.TrimRight(h.config.KeycloakExternalURL, "/")

	authURL, err := url.Parse(
		keycloakBaseURL +
			"/realms/" + h.config.KeycloakRealm +
			"/protocol/openid-connect/auth",
	)
	if err != nil {
		log.Println("[AUTH LOGIN] failed to parse Keycloak URL:", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	q := authURL.Query()
	q.Set("client_id", h.config.KeycloakWebClientID)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("redirect_uri", h.config.KeycloakRedirectURI)
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	authURL.RawQuery = q.Encode()

	c.Redirect(http.StatusTemporaryRedirect, authURL.String())
}

// HandleAuthLaunch starts the same fixed-callback OAuth flow as /auth/login.
// It exists as the stable entrypoint exposed by the system registry to external apps.
func (h *Handler) HandleAuthLaunch(c *gin.Context) {
	h.HandleAuthLogin(c)
}

// ----------------------------------------------------
// GET CURRENT USER (API)
// ----------------------------------------------------
func (h *Handler) HandleAuthGetMe(c *gin.Context) {
	accessToken := h.accessTokenFromRequest(c)
	if accessToken == "" {
		response.Fail(
			c,
			http.StatusUnauthorized,
			apierror.ErrUnauthorized.Code,
			apierror.ErrUnauthorized.Message,
		)
		return
	}

	user, err := h.authService.GetMe(accessToken)
	if err != nil {
		log.Printf(
			"[AUTH ME] failed: ip=%s user_agent=%s err=%v",
			c.ClientIP(),
			c.Request.UserAgent(),
			err,
		)

		response.Fail(
			c,
			http.StatusUnauthorized,
			apierror.ErrTokenInvalid.Code,
			"Invalid or expired token",
		)
		return
	}

	h.applyResolvedAccess(c, user)

	// Userinfo is the authoritative source of the user ID; the access
	// token may omit "sub" (Keycloak lightweight access tokens).
	userID := user.ID
	if userID == "" {
		userID = utils.ExtractUserIDFromJWT(accessToken)
	}

	_ = h.auditService.Log(
		c.Request.Context(),
		utils.ToNullUUID(userID),
		"auth.get_me",
		map[string]interface{}{
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		},
	)

	response.OK(c, http.StatusOK, AuthMeResponse{User: user})
}

func (h *Handler) applyResolvedAccess(c *gin.Context, user interface {
	GetAuthorizationFields() (string, []string, map[string][]string)
}) {
	userID, realmRoles, clientRoles := user.GetAuthorizationFields()
	_ = userID

	access := authz.ResolveAccess(c.Request.Context(), h.authzResolver, realmRoles, clientRoles)

	type authorizationSetter interface {
		SetAuthorizationAccess([]string, []string, []authz.SystemAccess)
	}

	if setter, ok := user.(authorizationSetter); ok {
		permissions := make([]string, 0, len(access.Permissions))
		for _, permission := range access.Permissions {
			permissions = append(permissions, string(permission))
		}

		systems := make([]string, 0, len(access.Systems))
		for _, system := range access.Systems {
			if system.ClientID != "" {
				systems = append(systems, system.ClientID)
			}
		}

		setter.SetAuthorizationAccess(permissions, systems, access.Systems)
	}
}

// ----------------------------------------------------
// OIDC CALLBACK (PKCE + state verification)
// ----------------------------------------------------
func (h *Handler) HandleAuthCallback(c *gin.Context) {
	if kcErr := c.Query("error"); kcErr != "" {
		h.auditLoginFailure(c)
		h.clearOAuthCookies(c)

		c.Redirect(
			http.StatusTemporaryRedirect,
			h.defaultFrontendRedirect(),
		)
		return
	}

	code := c.Query("code")
	if code == "" {
		h.auditLoginFailure(c)
		h.clearOAuthCookies(c)

		c.Redirect(
			http.StatusTemporaryRedirect,
			h.defaultFrontendRedirect(),
		)
		return
	}

	returnedState := c.Query("state")

	cookieState, err := c.Cookie(cookieOAuthState)
	if err != nil ||
		cookieState == "" ||
		returnedState == "" ||
		returnedState != cookieState {
		h.clearOAuthCookies(c)
		c.Redirect(
			http.StatusTemporaryRedirect,
			h.defaultFrontendRedirect(),
		)
		return
	}

	codeVerifier, err := c.Cookie(cookiePKCEVerifier)
	if err != nil || codeVerifier == "" {
		h.clearOAuthCookies(c)
		c.Redirect(
			http.StatusTemporaryRedirect,
			h.defaultFrontendRedirect(),
		)
		return
	}

	returnCookie, _ := c.Cookie(cookieOAuthReturnTo)
	returnTo := decodeReturnCookie(returnCookie, cookieState)
	if returnTo != "" {
		validatedReturnTo, validationErr := h.normalizeReturnURL(returnTo)
		if validationErr != nil {
			returnTo = ""
		} else {
			returnTo = validatedReturnTo
		}
	}

	tokens, err := h.authService.ProcessAuthCode(code, codeVerifier)
	if err != nil {
		h.clearOAuthCookies(c)
		if h.isCodeAlreadyUsedError(err) {
			c.Redirect(
				http.StatusTemporaryRedirect,
				h.defaultFrontendRedirect(),
			)
			return
		}

		h.notificationService.NotifyLoginFailed(
			c.Request.Context(),
			h.config.KeycloakWebClientID,
			c.ClientIP(),
			c.Request.UserAgent(),
			err,
		)

		h.auditLoginFailure(c)

		response.Fail(
			c,
			http.StatusUnauthorized,
			apierror.ErrTokenInvalid.Code,
			"authentication failed",
		)
		return
	}

	if tokens == nil || tokens.AccessToken == "" {
		h.auditLoginFailure(c)
		h.clearOAuthCookies(c)

		response.Fail(
			c,
			http.StatusUnauthorized,
			apierror.ErrTokenInvalid.Code,
			"authentication failed",
		)
		return
	}

	h.clearOAuthCookies(c)

	userID := utils.ExtractUserIDFromTokens(
		tokens.AccessToken,
		tokens.IDToken,
	)

	_ = h.auditService.LoginResult(
		c.Request.Context(),
		utils.ToNullUUID(userID),
		true,
		h.config.KeycloakWebClientID,
		c.ClientIP(),
		c.Request.UserAgent(),
		"",
		"",
	)

	ttl := h.sessionTTL(
		tokens.RefreshExpiresIn,
		tokens.ExpiresIn,
	)

	sessionID, err := h.sessions.Create(
		c.Request.Context(),
		authsession.Data{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			IDToken:      tokens.IDToken,
		},
		ttl,
	)
	if err != nil {
		response.Fail(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"failed to create session",
		)
		return
	}

	h.clearLegacyTokenCookies(c)

	h.setCookie(
		c,
		cookieSession,
		sessionID,
		int(ttl.Seconds()),
		true,
	)

	redirectURL := returnTo
	if redirectURL == "" {
		redirectURL = h.redirectAfterLogin(tokens.AccessToken)
	}

	c.Redirect(
		http.StatusTemporaryRedirect,
		redirectURL,
	)
}

// ----------------------------------------------------
// REFRESH TOKEN (API)
// ----------------------------------------------------
func (h *Handler) HandleAuthRefreshToken(c *gin.Context) {
	// Prefer the server-side session; fall back to the legacy refresh
	// cookie so browsers logged in before the session scheme keep working.
	sessionID, _ := c.Cookie(cookieSession)

	var refreshToken string
	if sessionID != "" {
		if sess, err := h.sessions.Get(c.Request.Context(), sessionID); err == nil {
			refreshToken = sess.RefreshToken
		} else if errors.Is(err, authsession.ErrNotFound) {
			sessionID = ""
		} else {
			response.Fail(c, http.StatusServiceUnavailable, "SESSION_UNAVAILABLE", "Session service is temporarily unavailable")
			return
		}
	}
	if refreshToken == "" {
		refreshToken, _ = c.Cookie(cookieRefreshToken)
	}

	if refreshToken == "" {
		_ = h.auditService.TokenRefresh(
			c.Request.Context(),
			uuid.NullUUID{},
			false,
			c.ClientIP(),
			c.Request.UserAgent(),
		)

		// Missing session/refresh token is usually normal:
		// user not logged in, initial app load, cookies cleared, or public page.
		// Do not notify admins and do not clear OAuth login-flow cookies.
		response.Fail(
			c,
			http.StatusUnauthorized,
			apierror.ErrTokenExpired.Code,
			"Session expired",
		)
		return
	}

	tokens, err := h.authService.GetAccessToken(refreshToken)
	if err != nil {
		log.Printf(
			"[AUTH REFRESH] failed: ip=%s user_agent=%s err=%v",
			c.ClientIP(),
			c.Request.UserAgent(),
			err,
		)

		_ = h.auditService.TokenRefresh(
			c.Request.Context(),
			uuid.NullUUID{},
			false,
			c.ClientIP(),
			c.Request.UserAgent(),
		)

		// Important:
		// Refresh failure should destroy the session only.
		// Do not clear oauth_state/pkce_verifier because a login flow may be in progress.
		h.destroySession(c)

		response.Fail(
			c,
			http.StatusUnauthorized,
			apierror.ErrTokenInvalid.Code,
			"Invalid or expired refresh token",
		)
		return
	}

	if tokens == nil || tokens.AccessToken == "" {
		log.Printf("[AUTH REFRESH] invalid token response from Keycloak")

		// Same rule: destroy the session only.
		h.destroySession(c)

		response.Fail(
			c,
			http.StatusUnauthorized,
			apierror.ErrTokenInvalid.Code,
			"Invalid token response",
		)
		return
	}

	// With refresh token rotation, Keycloak may return a new refresh token.
	// Keep the previous one when it doesn't.
	newData := authsession.Data{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IDToken:      tokens.IDToken,
	}
	if newData.RefreshToken == "" {
		newData.RefreshToken = refreshToken
	}

	ttl := h.sessionTTL(tokens.RefreshExpiresIn, tokens.ExpiresIn)

	if sessionID != "" {
		if err := h.sessions.Update(c.Request.Context(), sessionID, newData, ttl); err != nil {
			log.Printf("[AUTH REFRESH] failed to update session: %v", err)
			// A rotated token may already have invalidated the stored token.
			// Clear only session cookies; preserve any in-flight OAuth login.
			h.destroySession(c)
			response.Fail(c, http.StatusServiceUnavailable, "SESSION_UNAVAILABLE", "Unable to save refreshed session; please sign in again")
			return
		}
		// Re-issue the cookie so its lifetime follows the refreshed session.
		h.setCookie(c, cookieSession, sessionID, int(ttl.Seconds()), true)
	} else {
		// Legacy cookie-based browser: migrate it to a server-side session.
		newSessionID, err := h.sessions.Create(c.Request.Context(), newData, ttl)
		if err != nil {
			log.Printf("[AUTH REFRESH] failed to migrate legacy session: %v", err)

			h.destroySession(c)

			response.Fail(
				c,
				http.StatusServiceUnavailable,
				"SESSION_UNAVAILABLE",
				"Unable to save refreshed session; please sign in again",
			)
			return
		}

		log.Printf("[AUTH REFRESH] migrated legacy cookie session to server-side session")

		h.clearLegacyTokenCookies(c)
		h.setCookie(c, cookieSession, newSessionID, int(ttl.Seconds()), true)
	}

	userID := utils.ExtractUserIDFromTokens(tokens.AccessToken, tokens.IDToken)
	_ = h.auditService.TokenRefresh(
		c.Request.Context(),
		utils.ToNullUUID(userID),
		true,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	response.OK(c, http.StatusOK, AuthRefreshResponse{
		ExpiresIn: tokens.ExpiresIn,
		CookieDebug: AuthRefreshCookieDebug{
			Environment:      h.environment(),
			Secure:           h.cookieSecure(),
			SameSite:         h.cookieSameSiteString(),
			Domain:           h.cookieDomain(),
			HasAccessToken:   tokens.AccessToken != "",
			HasRefreshToken:  tokens.RefreshToken != "",
			RefreshExpiresIn: tokens.RefreshExpiresIn,
			FrontendBaseURL:  h.config.FrontendBaseURL,
			KeycloakRedirect: h.config.KeycloakRedirectURI,
			KeycloakExternal: h.config.KeycloakExternalURL,
			KeycloakInternal: h.config.KeycloakTokenBaseURL(),
		},
	})
}

// ----------------------------------------------------
// LOGOUT
// ----------------------------------------------------
func (h *Handler) HandleAuthLogout(c *gin.Context) {
	ctx := c.Request.Context()

	userID := utils.ToNullUUID(c.GetString("user_id"))

	_ = h.auditService.Logout(
		ctx,
		userID,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	// Resolve tokens from the server-side session, falling back to legacy
	// cookies for browsers logged in before the session scheme.
	var refreshToken, idTokenHint string

	if sessionID, _ := c.Cookie(cookieSession); sessionID != "" {
		if sess, err := h.sessions.Get(ctx, sessionID); err == nil {
			refreshToken = sess.RefreshToken
			idTokenHint = sess.IDToken
		}
	}
	if refreshToken == "" {
		refreshToken, _ = c.Cookie(cookieRefreshToken)
	}
	if idTokenHint == "" {
		idTokenHint, _ = c.Cookie(cookieIDToken)
	}

	if refreshToken != "" {
		if err := h.authService.LogOut(refreshToken); err != nil {
			log.Printf("[AUTH LOGOUT] token revocation failed: %v", err)
		}
	}

	redirectURL := h.authService.GetLogoutURL(
		idTokenHint,
		h.config.FrontendBaseURL,
	)

	// Logout clears everything: server-side session, session cookie,
	// legacy token cookies, and OAuth flow cookies.
	h.destroySession(c)
	h.clearOAuthCookies(c)

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// ----------------------------------------------------
// AUDIT HELPERS
// ----------------------------------------------------
func (h *Handler) auditLoginFailure(c *gin.Context) {
	_ = h.auditService.LoginResult(
		c.Request.Context(),
		uuid.NullUUID{},
		false,
		h.config.KeycloakWebClientID,
		c.ClientIP(),
		c.Request.UserAgent(),
		"",
		"",
	)
}

// ----------------------------------------------------
// CONFIG / ENV HELPERS
// ----------------------------------------------------
func (h *Handler) environment() string {
	if h == nil || h.config == nil {
		return "development"
	}

	env := strings.TrimSpace(h.config.Environment)
	if env == "" {
		return "development"
	}

	return strings.ToLower(env)
}

func (h *Handler) cookieSecure() bool {
	if h == nil || h.config == nil {
		return false
	}

	return h.config.CookieSecure()
}

func (h *Handler) cookieDomain() string {
	if h == nil || h.config == nil {
		return ""
	}

	return h.config.CookieDomainValue()
}

func (h *Handler) cookieSameSite() http.SameSite {
	if h == nil || h.config == nil {
		return http.SameSiteLaxMode
	}

	switch strings.ToLower(strings.TrimSpace(h.config.CookieSameSiteMode())) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}

func (h *Handler) cookieSameSiteString() string {
	switch h.cookieSameSite() {
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return "Default"
	}
}

func (h *Handler) defaultFrontendRedirect() string {
	if h == nil || h.config == nil {
		return "/"
	}

	baseURL := strings.TrimRight(h.config.FrontendBaseURL, "/")
	if baseURL == "" {
		return "/"
	}

	return baseURL + "/"
}

func (h *Handler) redirectAfterLogin(accessToken string) string {
	baseURL := strings.TrimRight(h.config.FrontendBaseURL, "/")

	log.Printf(
		"[AUTH REDIRECT] resolving post-login redirect: frontend_base_url=%q normalized_base_url=%q token_present=%v",
		h.config.FrontendBaseURL,
		baseURL,
		accessToken != "",
	)

	if baseURL == "" {
		log.Printf("[AUTH REDIRECT] empty frontend base url, redirecting to root")
		return "/"
	}

	isAdmin := utils.TokenHasRealmRole(accessToken, "admin")
	isUser := utils.TokenHasRealmRole(accessToken, "user")

	log.Printf(
		"[AUTH REDIRECT] token role checks: is_admin=%v is_user=%v",
		isAdmin,
		isUser,
	)

	if isAdmin {
		redirectURL := baseURL + "/admin/home"

		log.Printf(
			"[AUTH REDIRECT] admin role matched, redirecting to %q",
			redirectURL,
		)

		return redirectURL
	}

	if isUser {
		redirectURL := baseURL + "/apps/news"

		log.Printf(
			"[AUTH REDIRECT] user role matched, redirecting to %q",
			redirectURL,
		)

		return redirectURL
	}

	redirectURL := baseURL + "/"

	log.Printf(
		"[AUTH REDIRECT] no supported role matched, redirecting to default %q",
		redirectURL,
	)

	return redirectURL
}

// ----------------------------------------------------
// COOKIE HELPERS
// ----------------------------------------------------
func (h *Handler) setCookie(
	c *gin.Context,
	name string,
	value string,
	maxAge int,
	httpOnly bool,
) {
	if value == "" || maxAge <= 0 {
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   h.cookieDomain(),
		MaxAge:   maxAge,
		Expires:  time.Now().Add(time.Duration(maxAge) * time.Second),
		HttpOnly: httpOnly,
		Secure:   h.cookieSecure(),
		SameSite: h.cookieSameSite(),
	})
}

// sessionTTL derives the server-side session lifetime from the token
// response: the refresh token expiry bounds how long the session is usable.
func (h *Handler) sessionTTL(refreshExpiresIn, expiresIn int64) time.Duration {
	if refreshExpiresIn > 0 {
		return time.Duration(refreshExpiresIn) * time.Second
	}
	if expiresIn > 0 {
		return time.Duration(expiresIn) * time.Second
	}
	return defaultSessionTTL
}

// accessTokenFromRequest resolves the caller's access token: server-side
// session first, then the legacy access_token cookie.
func (h *Handler) accessTokenFromRequest(c *gin.Context) string {
	if sessionID, _ := c.Cookie(cookieSession); sessionID != "" {
		if sess, err := h.sessions.Get(c.Request.Context(), sessionID); err == nil {
			return sess.AccessToken
		}
	}

	if token, err := c.Cookie(cookieAccessToken); err == nil {
		return strings.TrimSpace(token)
	}

	return ""
}

// destroySession deletes the server-side session (best-effort) and clears
// the session cookie plus any legacy token cookies.
func (h *Handler) destroySession(c *gin.Context) {
	if sessionID, _ := c.Cookie(cookieSession); sessionID != "" {
		if err := h.sessions.Delete(c.Request.Context(), sessionID); err != nil {
			log.Printf("[AUTH SESSION] failed to delete session: %v", err)
		}
	}

	h.clearCookie(c, cookieSession, true)
	h.clearLegacyTokenCookies(c)
}

func (h *Handler) clearCookie(
	c *gin.Context,
	name string,
	httpOnly bool,
) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   h.cookieDomain(),
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: httpOnly,
		Secure:   h.cookieSecure(),
		SameSite: h.cookieSameSite(),
	})
}

func (h *Handler) clearLegacyTokenCookies(c *gin.Context) {
	h.clearCookie(c, cookieAccessToken, true)
	h.clearCookie(c, cookieRefreshToken, true)
	h.clearCookie(c, cookieIDToken, true)
}

func (h *Handler) clearOAuthCookies(c *gin.Context) {
	h.clearCookie(c, cookiePKCEVerifier, true)
	h.clearCookie(c, cookieOAuthState, true)
	h.clearCookie(c, cookieOAuthReturnTo, true)
}

// ----------------------------------------------------
// AUTH FLOW HELPERS
// ----------------------------------------------------
func (h *Handler) isCodeAlreadyUsedError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "code not valid") ||
		strings.Contains(msg, "code already used")
}

func (h *Handler) hasAnyAuthCookie(c *gin.Context) bool {
	if c == nil {
		return false
	}

	if sessionID, err := c.Cookie(cookieSession); err == nil && sessionID != "" {
		return true
	}

	if token, err := c.Cookie(cookieAccessToken); err == nil && token != "" {
		return true
	}

	if token, err := c.Cookie(cookieRefreshToken); err == nil && token != "" {
		return true
	}

	return false
}
