package platform

import (
	"database/sql"
	"errors"
	"github.com/moh-sso-dashboard/internal/observability"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/http/response"
)

type Handler struct {
	WorkerStaleAfter time.Duration
	Repository       Repository
	Verifier         TokenVerifier
}

func (h *Handler) AuthenticateToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Fields(c.GetHeader("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			fail(c, ErrUnauthorized)
			return
		}
		if h == nil || h.Verifier == nil {
			fail(c, ErrUnavailable)
			return
		}
		token, err := h.Verifier.Verify(c.Request.Context(), fields[1])
		if err != nil {
			fail(c, err)
			return
		}
		c.Set("platform_token", token)
		c.Set("oauth_client_id", token.ClientID)
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
func (h *Handler) AuthenticateMachine() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := c.MustGet("platform_token").(Token)
		if !ok || h.Repository == nil {
			fail(c, ErrUnavailable)
			return
		}
		p, err := h.Repository.ResolveIdentity(c.Request.Context(), token)
		if err != nil {
			fail(c, err)
			return
		}
		c.Set("platform_principal", p)
		c.Set("application_id", p.ApplicationID)
		c.Request = c.Request.WithContext(observability.WithApplicationID(c.Request.Context(), p.ApplicationID))
		c.Set("machine_subject", p.Subject)
		c.Next()
	}
}
func requireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := c.Get("platform_principal")
		principal, valid := p.(Principal)
		if !ok || !valid || !principal.HasScope(scope) {
			fail(c, ErrForbidden)
			return
		}
		c.Next()
	}
}
func (h *Handler) GetIdentity(c *gin.Context) {
	p := c.MustGet("platform_principal").(Principal)
	response.OK(c, http.StatusOK, p)
}
func (h *Handler) GetUserAccess(c *gin.Context) {
	token := c.MustGet("platform_token").(Token)
	if strings.HasPrefix(token.Username, "service-account-") || !contains(strings.Fields(token.Scope), IdentityRead) {
		fail(c, ErrForbidden)
		return
	}
	result, err := h.Repository.UserAccess(c.Request.Context(), c.Param("clientId"), token)
	if err != nil {
		fail(c, err)
		return
	}
	c.Set("application_id", result.ApplicationID)
	c.Set("user_id", result.Subject)
	response.OK(c, http.StatusOK, result)
}
func (h *Handler) SaveIdentity(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	var input IdentityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, ErrInvalid)
		return
	}
	input.Subject, input.Reason = strings.TrimSpace(input.Subject), strings.TrimSpace(input.Reason)
	if id, err := uuid.Parse(input.Subject); err != nil || id == uuid.Nil || input.Reason == "" || len(input.Reason) > 2000 || len(input.Scopes) > 3 {
		fail(c, ErrInvalid)
		return
	}
	if input.Scopes == nil {
		input.Scopes = []string{}
	}
	seen := map[string]bool{}
	for _, scope := range input.Scopes {
		if seen[scope] || !contains([]string{IdentityRead, NotificationsRead, NotificationsWrite}, scope) {
			fail(c, ErrInvalid)
			return
		}
		seen[scope] = true
	}
	result, err := h.Repository.SaveIdentity(c.Request.Context(), c.Param("clientId"), c.Param("oauthClientId"), c.GetString("user_id"), input)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func fail(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "PLATFORM_ERROR", "platform request failed"
	switch {
	case errors.Is(err, ErrIdempotencyConflict):
		status, code, message = http.StatusConflict, "IDEMPOTENCY_CONFLICT", "idempotency key already used with a different request"
	case errors.Is(err, ErrQuota):
		status, code, message = http.StatusTooManyRequests, "APPLICATION_QUOTA_EXCEEDED", "application notification quota exceeded"
		c.Header("Retry-After", "60")
	case errors.Is(err, ErrUnauthorized):
		status, code, message = http.StatusUnauthorized, "INVALID_IDENTITY", "valid bearer token required"
		c.Header("WWW-Authenticate", `Bearer realm="moh-platform"`)
	case errors.Is(err, ErrForbidden):
		status, code, message = http.StatusForbidden, "APPLICATION_ACCESS_DENIED", "application access denied"
	case errors.Is(err, ErrUnavailable):
		status, code, message = http.StatusServiceUnavailable, "PLATFORM_UNAVAILABLE", "platform dependency unavailable"
	case errors.Is(err, ErrInvalid):
		status, code, message = http.StatusBadRequest, "INVALID_REQUEST", "invalid platform request"
	case errors.Is(err, sql.ErrNoRows):
		status, code, message = http.StatusNotFound, "NOT_FOUND", "resource not found"
	}
	response.Fail(c, status, code, message)
	c.Abort()
}
