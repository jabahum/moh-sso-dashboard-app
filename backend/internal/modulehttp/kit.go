package modulehttp

import (
	"net/http"
	"time"

	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type rateIdentityKey struct{}

// PolicyLimiter enforces modulekit rate limits exactly like the host's
// ratelimit.MiddlewareForPolicy with a per-user key: the bucket key, response
// headers and 429 body are unchanged, and a missing or failing store lets the
// request through (fail-open), as it did before the routes moved into modules.
func PolicyLimiter(limiter *ratelimit.Limiter) modulekit.Limiter {
	return func(r *http.Request, limit modulekit.RateLimit) (bool, http.Header) {
		if !limiter.Available() || limit.Limit <= 0 {
			return true, nil
		}
		if limit.Window <= 0 {
			limit.Window = time.Minute
		}
		identity, _ := r.Context().Value(rateIdentityKey{}).(string)
		key := identity
		if len(key) < 3 || key[:3] != "rl:" {
			key = ratelimit.Key(limit.Name, identity)
		}
		allowed, remaining, reset, err := limiter.Allow(r.Context(), key, limit.Limit, limit.Window)
		headers := http.Header{}
		ratelimit.SetHeaders(headers, limit.Name, limit.Limit, remaining, reset)
		if err != nil {
			return true, headers
		}
		return allowed, headers
	}
}
