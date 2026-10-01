package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/features/authsession"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

func ExtractAuthContext(
	kc *keycloak.Client,
	sessions *authsession.Store,
	resolver authz.PermissionResolver,
	cfg *config.Config,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		// LOCAL DEV ONLY: DEV_AUTH_BYPASS=true skips Keycloak entirely.
		if DevAuthBypassEnabled(cfg) {
			applyDevBypass(c, resolver)
			return
		}

		accessToken := extractAccessToken(c, sessions)
		if accessToken == "" {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing token")
			return
		}

		user, err := kc.Me(accessToken)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid user session")
			return
		}

		authContext := authz.NewContextWithResolver(
			c.Request.Context(),
			resolver,
			user.ID,
			user.RealmRoles,
			user.ClientRoles,
		)

		c.Set("access_token", accessToken)
		c.Set(authz.ContextKey, authContext)
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("client_roles", authContext.ClientRoles)
		c.Set("realm_roles", authContext.RealmRoles)
		c.Set("permissions", authContext.Permissions)
		c.Set("systems", authContext.AccessibleSystems)
		c.Set("accessible_systems", authContext.AccessibleSystemDetails)
		c.Set("is_admin", authContext.IsAdmin)
		c.Set("is_user", authContext.IsUser)

		c.Next()
	}
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists || userID == "" {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}

		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authContext, ok := authz.FromGin(c); ok && authContext.IsAdmin {
			c.Next()
			return
		}

		isAdmin, exists := c.Get("is_admin")
		if !exists {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", "admin access required")
			return
		}

		admin, ok := isAdmin.(bool)
		if !ok || !admin {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", "admin access required")
			return
		}

		c.Next()
	}
}

func RequirePermission(permission authz.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		authContext, ok := authz.FromGin(c)
		if !ok {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}

		if !authContext.HasPermission(permission) {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("permission required: %s", permission))
			return
		}

		c.Next()
	}
}

func RequireAnyPermission(permissions ...authz.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		authContext, ok := authz.FromGin(c)
		if !ok {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}

		if !authContext.HasAnyPermission(permissions...) {
			required := make([]string, 0, len(permissions))
			for _, permission := range permissions {
				required = append(required, string(permission))
			}

			abortWithError(c, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("one of these permissions is required: %s", strings.Join(required, ", ")))
			return
		}

		c.Next()
	}
}

func RequireAllPermissions(permissions ...authz.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		authContext, ok := authz.FromGin(c)
		if !ok {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}

		missing := make([]string, 0)
		for _, permission := range permissions {
			if !authContext.HasPermission(permission) {
				missing = append(missing, string(permission))
			}
		}
		if len(missing) > 0 {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("missing required permissions: %s", strings.Join(missing, ", ")))
			return
		}

		c.Next()
	}
}

func RequireSystem(systemClientID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authContext, ok := authz.FromGin(c)
		if !ok {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}

		if !authContext.HasSystem(systemClientID) {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("system access required: %s", systemClientID))
			return
		}

		c.Next()
	}
}

func RequireSystemRole(systemClientID string, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authContext, ok := authz.FromGin(c)
		if !ok {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
			return
		}

		if !authContext.HasSystemRole(systemClientID, role) {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("system role required: %s:%s", systemClientID, role))
			return
		}

		c.Next()
	}
}

func RequireRealmRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authContext, ok := authz.FromGin(c)
		if !ok || !authContext.HasRealmRole(role) {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("realm role required: %s", role))
			return
		}

		c.Next()
	}
}

func RequireClientRole(clientIDParam string, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authContext, ok := authz.FromGin(c)
		clientID := strings.TrimSpace(c.Param(clientIDParam))
		if !ok || !authContext.HasClientRole(clientID, role) {
			abortWithError(c, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("client role required: %s", role))
			return
		}

		c.Next()
	}
}

func abortWithError(c *gin.Context, status int, code string, message string) {
	response.Fail(c, status, code, message)
	c.Abort()
}

func extractAccessToken(c *gin.Context, sessions *authsession.Store) string {
	// 1. Prefer Authorization header for API/mobile/backward compatibility.
	if token := extractBearerToken(c); token != "" {
		return token
	}

	// 2. Server-side session referenced by the opaque session cookie.
	if token := extractSessionToken(c, sessions); token != "" {
		return token
	}

	// 3. Legacy HttpOnly access_token cookie (pre-session deployments).
	if token := extractAccessTokenCookie(c); token != "" {
		return token
	}

	return ""
}

func extractSessionToken(c *gin.Context, sessions *authsession.Store) string {
	if sessions == nil {
		return ""
	}

	sessionID, err := c.Cookie(authsession.CookieName)
	if err != nil || sessionID == "" {
		return ""
	}

	sess, err := sessions.Get(c.Request.Context(), sessionID)
	if err != nil {
		return ""
	}

	return sess.AccessToken
}

func extractBearerToken(c *gin.Context) string {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return ""
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func extractAccessTokenCookie(c *gin.Context) string {
	token, err := c.Cookie("access_token")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(token)
}
