package modulehttp

import (
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// NewPlatform builds the standard capability set handed to every compiled
// module. Every capability reads only the verified identity the registrar
// stores in the request context. limiter backs Limit (a nil or unavailable
// limiter makes Limit refuse requests, never allow them); audit may be nil when
// the host has no audit recorder, leaving the audit capability absent.
func NewPlatform(limiter UserLimiter, audit core.AuditRecorder) core.Platform {
	return core.Platform{
		Access:        RequestAccess,
		AuthorizeAny:  AuthorizeAny,
		RequireAdmin:  RequireAdmin,
		RequireSystem: RequireSystem,
		Limit:         PerUserLimit(limiter),
		RequestID:     RequestID,
		ReportError:   StreamError,
		Audit:         audit,
	}
}
