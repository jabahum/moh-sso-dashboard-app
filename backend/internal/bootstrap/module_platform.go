package bootstrap

import (
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// modulePlatform is the single place the host assembles the capabilities it
// offers compiled modules. Module wiring takes capabilities from this value
// rather than naming host functions one by one.
func modulePlatform(deps handlerDependencies) core.Platform {
	var audit core.AuditRecorder
	// Avoid turning a nil *AuditService into a non-nil interface.
	if deps.Services.Audit != nil {
		audit = deps.Services.Audit
	}
	return modulehttp.NewPlatform(ratelimit.New(deps.Redis), audit)
}
