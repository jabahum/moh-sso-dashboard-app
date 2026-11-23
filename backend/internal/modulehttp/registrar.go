// Package modulehttp integrates standard-library modules with the trusted host.
// It contains no application implementation or dependency on a published module.
package modulehttp

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/http/apierror"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type accessKey struct{}
type access struct {
	actor       authz.Context
	valid       bool
	healthScope core.HealthScope
}

// AuthorizeAny reads only the verified context supplied by the registrar after
// the platform authentication middleware. Request headers never create claims.
func AuthorizeAny(r *http.Request, permissions ...string) error {
	actor, valid := RequestAccess(r)
	if !valid {
		return apierror.New(http.StatusUnauthorized, "UNAUTHORIZED", http.StatusText(http.StatusUnauthorized))
	}
	if core.HasAnyPermission(actor.Permissions, permissions...) {
		return nil
	}
	return apierror.New(http.StatusForbidden, "FORBIDDEN", http.StatusText(http.StatusForbidden))
}

// Actor returns only the identity verified by the platform middleware.
func Actor(r *http.Request) (authz.Context, bool) {
	current, ok := r.Context().Value(accessKey{}).(access)
	return current.actor, ok && current.valid && current.actor.UserID != ""
}

// RequireAdmin grants access only from verified platform identity.
func RequireAdmin(r *http.Request) error {
	actor, valid := Actor(r)
	if !valid {
		return apierror.New(http.StatusUnauthorized, "UNAUTHORIZED", http.StatusText(http.StatusUnauthorized))
	}
	if !actor.IsAdmin {
		return apierror.New(http.StatusForbidden, "FORBIDDEN", http.StatusText(http.StatusForbidden))
	}
	return nil
}
func RequireSystem(r *http.Request, system string) error {
	actor, valid := Actor(r)
	if !valid {
		return apierror.New(http.StatusUnauthorized, "UNAUTHORIZED", http.StatusText(http.StatusUnauthorized))
	}
	if !actor.HasSystem(system) {
		return apierror.New(http.StatusForbidden, "FORBIDDEN", http.StatusText(http.StatusForbidden))
	}
	return nil
}

// RequestAccess returns access resolved by the host for an authenticated request.
func RequestAccess(r *http.Request) (core.RequestAccess, bool) {
	current, _ := r.Context().Value(accessKey{}).(access)
	actor, valid := Actor(r)
	if !valid {
		return core.RequestAccess{}, false
	}
	return core.RequestAccess{
		Identity:    core.Identity{Subject: actor.UserID, RealmRoles: actor.RealmRoles, ClientRoles: actor.ClientRoles},
		Permissions: actor.PermissionStrings(),
		Systems:     actor.AccessibleSystems,
		HealthScope: current.healthScope,
		IsAdmin:     actor.IsAdmin,
	}, true
}

func RequestID(r *http.Request) string { return response.MetaFromRequest(r).RequestID }

type HTTPModule interface{ http.Handler }
type Registrar struct{ group *gin.RouterGroup }

func NewRegistrar(group *gin.RouterGroup) *Registrar { return &Registrar{group: group} }
func Register(group *gin.RouterGroup, module HTTPModule) {
	if group == nil {
		return
	}
	registrar := NewRegistrar(group)
	switch described := module.(type) {
	case interface{ Patterns() []string }:
		for _, pattern := range described.Patterns() {
			registrar.Handle(pattern, module)
		}
	case interface{ Pattern() string }:
		registrar.Handle(described.Pattern(), module)
	default:
		panic("module does not declare HTTP patterns")
	}
}

var parameter = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func (r *Registrar) Handle(pattern string, handler http.Handler) {
	fields := strings.Fields(pattern)
	if len(fields) != 2 || !strings.HasPrefix(fields[1], "/") {
		panic(fmt.Sprintf("invalid module HTTP pattern %q", pattern))
	}
	route := parameter.ReplaceAllString(fields[1], ":$1")
	if strings.ContainsAny(route, "{}") {
		panic("unsupported module wildcard pattern")
	}
	base := strings.TrimSuffix(r.group.BasePath(), "/")
	mounted := handler
	if base != "" {
		mounted = http.StripPrefix(base, handler)
	}
	r.group.Handle(fields[0], route, func(c *gin.Context) {
		actor, valid := authz.FromGin(c)
		rawHealth, _ := c.Get("health_context")
		health, _ := rawHealth.(map[string]string)
		scope := core.HealthScope{District: strings.TrimSpace(health["district"]), Facility: strings.TrimSpace(health["facility"])}
		ctx := context.WithValue(c.Request.Context(), accessKey{}, access{actor: actor, valid: valid, healthScope: scope})
		ctx = modulekit.WithRequest(ctx, kitRequest(c, actor, valid, scope))
		ctx = context.WithValue(ctx, rateIdentityKey{}, ratelimit.ByUser(c))
		mounted.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
	})
}

// kitRequest exposes the verified caller to modules built on core/modulekit.
func kitRequest(c *gin.Context, actor authz.Context, valid bool, scope core.HealthScope) modulekit.Request {
	request := modulekit.Request{RequestID: response.MetaFromRequest(c.Request).RequestID}
	// Set by the health-context selection middleware on context-scoped routes.
	if rawID, ok := c.Get("health_context_id"); ok {
		if id, ok := rawID.(uuid.UUID); ok && id != uuid.Nil {
			mode, _ := c.Get("health_context_scope_mode")
			request.HealthContext = &modulekit.HealthContextSelection{
				NodeID:             id.String(),
				IncludeDescendants: fmt.Sprint(mode) == core.ContextNodeAndDescendants,
			}
		}
	}
	if !valid || actor.UserID == "" {
		return request
	}
	request.Authenticated = true
	request.Access = core.RequestAccess{
		Identity:    core.Identity{Subject: actor.UserID, RealmRoles: actor.RealmRoles, ClientRoles: actor.ClientRoles},
		Permissions: actor.PermissionStrings(),
		Systems:     actor.AccessibleSystems,
		HealthScope: scope,
		IsAdmin:     actor.IsAdmin,
	}
	return request
}
