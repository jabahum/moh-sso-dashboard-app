package modulekit

import (
	"context"
	"net/http"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// Request is the verified per-request state the host supplies to modules.
type Request struct {
	// Access is meaningful only when Authenticated is true.
	Access        core.RequestAccess
	Authenticated bool
	// RequestID is echoed in the response envelope's meta.
	RequestID string
	// HealthContext is the caller's selected, already-authorized health
	// context node. It is nil when the route is not context scoped or the
	// caller made no selection.
	HealthContext *HealthContextSelection
}

type requestKey struct{}

// WithRequest attaches verified request state. Only the trusted host calls it.
func WithRequest(ctx context.Context, request Request) context.Context {
	return context.WithValue(ctx, requestKey{}, request)
}

// FromContext returns the request state attached by the host.
func FromContext(ctx context.Context) (Request, bool) {
	request, ok := ctx.Value(requestKey{}).(Request)
	return request, ok
}

// FromRequest returns the request state attached by the host.
func FromRequest(r *http.Request) (Request, bool) {
	if r == nil {
		return Request{}, false
	}
	return FromContext(r.Context())
}

// AccessFromRequest returns the verified caller, if the request is authenticated.
func AccessFromRequest(r *http.Request) (core.RequestAccess, bool) {
	request, ok := FromRequest(r)
	if !ok || !request.Authenticated || request.Access.Subject == "" {
		return core.RequestAccess{}, false
	}
	return request.Access, true
}
