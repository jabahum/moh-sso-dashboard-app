package health_context

import (
	"errors"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

var (
	ErrNotFound          = errors.New("health context not found")
	ErrInvalidInput      = errors.New("invalid health context input")
	ErrCycle             = errors.New("health context hierarchy cycle")
	ErrContextForbidden  = core.ErrContextForbidden
	ErrInactiveContext   = errors.New("health context is inactive")
	ErrInvalidAssignment = errors.New("invalid health context assignment")
	ErrContextInUse      = errors.New("health context is in use")
	ErrVersionConflict   = errors.New("health context version conflict")
)
