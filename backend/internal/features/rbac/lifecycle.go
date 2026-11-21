package rbac

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/moh-sso-dashboard/internal/authz"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
)

var ErrApplicationExists = errors.New("application already registered")

var ErrLifecycleConflict = errors.New("invalid application lifecycle transition")
var ErrLifecycleForbidden = errors.New("application lifecycle permission required")

func (s *Service) TransitionSystem(ctx context.Context, clientID string, input LifecycleTransitionInput, actor authz.Context) (System, error) {
	clientID = strings.TrimSpace(clientID)
	input.Status = strings.TrimSpace(input.Status)
	input.Reason = strings.TrimSpace(input.Reason)
	if clientID == "" || input.Reason == "" || len(input.Reason) > 2000 {
		return System{}, fmt.Errorf("%w: clientId and a reason of 1–2000 characters are required", ErrInvalidInput)
	}
	if !core.IsApplicationLifecycleStatus(input.Status) {
		return System{}, fmt.Errorf("%w: unknown lifecycle status", ErrInvalidInput)
	}
	allowed := core.CanManageApplicationLifecycle(core.LifecycleActor{
		Subject:      actor.UserID,
		CanConfigure: actor.HasPermission(authz.PermissionRBACWrite),
		CanApprove:   actor.HasPermission(authz.PermissionRBACRolesWrite),
	}, input.Status)
	if !allowed {
		err := s.repository.RecordAuditEvent(ctx, lifecycleAudit(clientID, actor.UserID, "system.lifecycle.denied", "", input, "permission denied"))
		if err != nil {
			return System{}, err
		}
		return System{}, ErrLifecycleForbidden
	}
	return s.repository.TransitionSystem(ctx, clientID, input, actor.UserID, func(system System, accessRoles int) error {
		if !core.CanTransitionApplication(system.LifecycleStatus, input.Status) {
			return ErrLifecycleConflict
		}
		if core.ApplicationTransitionRequiresValidation(input.Status) {
			return validateApplication(system, accessRoles)
		}
		return nil
	})
}

func validateApplication(system System, accessRoles int) error {
	if err := core.ValidateApplicationMetadata(coreApplication(system)); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	launcher, sideNav := system.DisplayInLauncher, system.DisplayInSideNav
	if err := systemrbac.ValidateSystemBehavior(systemrbac.SeedSystem{
		SystemType: system.SystemType, LaunchURL: system.LaunchURL, AuthenticatedLaunchURL: system.AuthenticatedLaunchURL,
		Navigation: system.Navigation, LaunchMode: system.LaunchMode, DisplayInLauncher: &launcher, DisplayInSideNav: &sideNav,
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if accessRoles == 0 {
		return fmt.Errorf("%w: configure at least one enabled system access role", ErrInvalidInput)
	}
	return nil
}
