package modulekit

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

type callerNameKey struct{}

// CallerNameMiddleware copies the authenticated user's email (or username)
// from the Gin context into the request context, so a module mounted behind it
// can name the caller through CallerName. It never authorizes anything.
func CallerNameMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if name := callerName(c); name != "" {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), callerNameKey{}, name))
		}
		c.Next()
	}
}

// CallerName returns the name stored by CallerNameMiddleware, or "".
func CallerName(r *http.Request) string {
	name, _ := r.Context().Value(callerNameKey{}).(string)
	return name
}

func callerName(c *gin.Context) string {
	raw, exists := c.Get("user")
	if !exists {
		return ""
	}
	user, ok := raw.(*keycloak.AuthUser)
	if !ok || user == nil {
		return ""
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		return email
	}
	return strings.TrimSpace(user.Username)
}
