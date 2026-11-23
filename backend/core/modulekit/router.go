package modulekit

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// RateLimit names a per-caller request budget enforced by the host's limiter.
type RateLimit struct {
	Name   string
	Limit  int
	Window time.Duration
}

// Limiter consumes one request from the caller's budget. It returns the
// rate-limit headers to send and whether the request may proceed. The host
// decides how store failures are handled.
type Limiter func(r *http.Request, limit RateLimit) (allowed bool, headers http.Header)

// Endpoint declares one module operation and its access requirements. The
// Router checks them, in this order, before calling Handle:
//
//   - Public endpoints skip every check.
//   - Otherwise the caller must be authenticated (401).
//   - Admin requires an administrator (403).
//   - System requires access to the named system (403).
//   - Permission, AnyPermissions and AllPermissions require those grants (403).
//   - RateLimit, when set, consumes from the caller's budget (429).
type Endpoint struct {
	Method string
	// Path uses net/http patterns, for example "/storage-locations/{id}".
	Path           string
	Public         bool
	Admin          bool
	System         string
	Permission     string
	AnyPermissions []string
	AllPermissions []string
	RateLimit      *RateLimit
	Handle         func(*Context)
}

// Router serves a module's declared endpoints.
type Router struct {
	mux      *http.ServeMux
	patterns []string
	limiter  Limiter
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithLimiter supplies the host's rate limiter. Without one, RateLimit
// declarations are not enforced.
func WithLimiter(limiter Limiter) RouterOption {
	return func(r *Router) { r.limiter = limiter }
}

// NewRouter validates and mounts endpoints.
func NewRouter(endpoints []Endpoint, options ...RouterOption) (*Router, error) {
	router := &Router{mux: http.NewServeMux()}
	for _, option := range options {
		option(router)
	}
	seen := map[string]bool{}
	for _, endpoint := range endpoints {
		if endpoint.Handle == nil {
			return nil, fmt.Errorf("endpoint %s %s has no handler", endpoint.Method, endpoint.Path)
		}
		method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
		if method == "" || !strings.HasPrefix(endpoint.Path, "/") {
			return nil, fmt.Errorf("invalid endpoint %q %q", endpoint.Method, endpoint.Path)
		}
		pattern := method + " " + endpoint.Path
		if seen[pattern] {
			return nil, fmt.Errorf("duplicate endpoint %s", pattern)
		}
		seen[pattern] = true
		if endpoint.Public && (endpoint.Admin || endpoint.System != "" || endpoint.Permission != "" || len(endpoint.AnyPermissions) > 0 || len(endpoint.AllPermissions) > 0) {
			return nil, fmt.Errorf("public endpoint %s declares access requirements", pattern)
		}
		current := endpoint
		router.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			router.serve(current, w, r)
		})
		router.patterns = append(router.patterns, pattern)
	}
	if len(router.patterns) == 0 {
		return nil, errors.New("module declares no endpoints")
	}
	return router, nil
}

// Patterns lists "METHOD /path" patterns for host route registration.
func (r *Router) Patterns() []string { return append([]string(nil), r.patterns...) }

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.mux.ServeHTTP(w, req) }

func (r *Router) serve(endpoint Endpoint, w http.ResponseWriter, req *http.Request) {
	if !endpoint.Public {
		if failure := authorize(endpoint, req); failure != nil {
			WriteError(w, req, failure)
			return
		}
	}
	if endpoint.RateLimit != nil && r.limiter != nil {
		allowed, headers := r.limiter(req, *endpoint.RateLimit)
		for name, values := range headers {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		if !allowed {
			WriteFail(w, req, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests, slow down")
			return
		}
	}
	endpoint.Handle(&Context{Writer: w, Request: req})
}

// authorize reproduces the host middleware responses for each requirement.
func authorize(endpoint Endpoint, r *http.Request) *Error {
	access, ok := AccessFromRequest(r)
	if !ok {
		return NewError(http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
	}
	if endpoint.Admin && !access.IsAdmin {
		return NewError(http.StatusForbidden, "FORBIDDEN", "admin access required")
	}
	if endpoint.System != "" && !access.HasSystem(endpoint.System) {
		return NewError(http.StatusForbidden, "FORBIDDEN", "system access required: "+endpoint.System)
	}
	if endpoint.Permission != "" && !access.HasPermission(endpoint.Permission) {
		return NewError(http.StatusForbidden, "FORBIDDEN", "permission required: "+endpoint.Permission)
	}
	if len(endpoint.AnyPermissions) > 0 && !core.HasAnyPermission(access.Permissions, endpoint.AnyPermissions...) {
		return NewError(http.StatusForbidden, "FORBIDDEN", "one of these permissions is required: "+strings.Join(endpoint.AnyPermissions, ", "))
	}
	if missing := core.MissingPermissions(access.Permissions, endpoint.AllPermissions...); len(missing) > 0 {
		return NewError(http.StatusForbidden, "FORBIDDEN", "missing required permissions: "+strings.Join(missing, ", "))
	}
	return nil
}
