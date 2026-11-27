package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Platform is the standard set of trusted host capabilities offered to every
// compiled module. A module accepts one Platform instead of declaring its own
// callbacks for identity, authorization, rate limits, tracing and audit, so the
// host builds it once and passes the same value to every module.
//
// Every function reads only identity the host has already verified; none of
// them may derive claims from request headers. A module states which
// capabilities it needs with Require and must fail closed when one is missing.
type Platform struct {
	// Access returns the verified identity and access decision for r, or false
	// when the request is not authenticated.
	Access func(r *http.Request) (RequestAccess, bool)
	// AuthorizeAny authenticates r and grants it when the caller holds at least
	// one of the permissions. Errors may implement StatusCode/ErrorCode.
	AuthorizeAny func(r *http.Request, permissions ...string) error
	// RequireAdmin grants only verified platform administrators.
	RequireAdmin func(r *http.Request) error
	// RequireSystem grants callers with access to the system (client) ID.
	RequireSystem func(r *http.Request, system string) error
	// Limit applies the named per-user policy (limit requests per window) to the
	// verified caller. It returns rate-limit headers to copy to the response and
	// an error when the request must be refused. It never fails open.
	Limit func(r *http.Request, policy string, limit int, window time.Duration) (http.Header, error)
	// RequestID returns the trusted trace ID for r.
	RequestID func(r *http.Request) string
	// ReportError records a failure that cannot be returned to the caller, for
	// example a partially streamed response or an item skipped from a list.
	ReportError func(r *http.Request, err error)
	// Audit records security and business events.
	Audit AuditRecorder
}

// Capability names one Platform field for Require.
type Capability string

const (
	CapabilityAccess        Capability = "access"
	CapabilityAuthorizeAny  Capability = "authorize-any"
	CapabilityRequireAdmin  Capability = "require-admin"
	CapabilityRequireSystem Capability = "require-system"
	CapabilityLimit         Capability = "limit"
	CapabilityRequestID     Capability = "request-id"
	CapabilityReportError   Capability = "report-error"
	CapabilityAudit         Capability = "audit"
)

// ErrMissingCapability reports that the host did not supply a capability a
// module requires.
var ErrMissingCapability = errors.New("missing platform capability")

// Require returns ErrMissingCapability, naming every missing capability, when
// any of the listed capabilities is absent. Modules call it while they are
// constructed so a host misconfiguration fails at startup, not per request.
func (p Platform) Require(capabilities ...Capability) error {
	var missing []string
	for _, capability := range capabilities {
		if !p.has(capability) {
			missing = append(missing, string(capability))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingCapability, strings.Join(missing, ", "))
	}
	return nil
}

func (p Platform) has(capability Capability) bool {
	switch capability {
	case CapabilityAccess:
		return p.Access != nil
	case CapabilityAuthorizeAny:
		return p.AuthorizeAny != nil
	case CapabilityRequireAdmin:
		return p.RequireAdmin != nil
	case CapabilityRequireSystem:
		return p.RequireSystem != nil
	case CapabilityLimit:
		return p.Limit != nil
	case CapabilityRequestID:
		return p.RequestID != nil
	case CapabilityReportError:
		return p.ReportError != nil
	case CapabilityAudit:
		return p.Audit != nil
	default:
		return false
	}
}
