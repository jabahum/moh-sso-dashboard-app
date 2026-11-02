package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/moh-sso-dashboard/internal/authz"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
)

var ErrApplicationExists = errors.New("application already registered")

var ErrLifecycleConflict = errors.New("invalid application lifecycle transition")
var ErrLifecycleForbidden = errors.New("application lifecycle permission required")

// LifecycleTransitions is the supported state machine; retirement is terminal.
var lifecycleTransitions = map[string][]string{
	"registered": {"configured", "retired"},
	"configured": {"validated", "retired"},
	"validated":  {"approved", "configured", "retired"},
	"approved":   {"active", "configured", "retired"},
	"active":     {"suspended", "retired"},
	"suspended":  {"configured", "retired"},
	"retired":    {},
}

func (s *Service) TransitionSystem(ctx context.Context, clientID string, input LifecycleTransitionInput, actor authz.Context) (System, error) {
	clientID = strings.TrimSpace(clientID)
	input.Status = strings.TrimSpace(input.Status)
	input.Reason = strings.TrimSpace(input.Reason)
	if clientID == "" || input.Reason == "" || len(input.Reason) > 2000 {
		return System{}, fmt.Errorf("%w: clientId and a reason of 1–2000 characters are required", ErrInvalidInput)
	}
	if _, ok := lifecycleTransitions[input.Status]; !ok {
		return System{}, fmt.Errorf("%w: unknown lifecycle status", ErrInvalidInput)
	}
	allowed := actor.UserID != "" && actor.HasPermission(authz.PermissionRBACWrite)
	if input.Status == "approved" || input.Status == "active" {
		allowed = allowed && actor.HasPermission(authz.PermissionRBACRolesWrite)
	}
	if !allowed {
		err := s.repository.RecordAuditEvent(ctx, lifecycleAudit(clientID, actor.UserID, "system.lifecycle.denied", "", input, "permission denied"))
		if err != nil {
			return System{}, err
		}
		return System{}, ErrLifecycleForbidden
	}
	return s.repository.TransitionSystem(ctx, clientID, input, actor.UserID, func(system System, accessRoles int) error {
		if !canTransition(system.LifecycleStatus, input.Status) {
			return ErrLifecycleConflict
		}
		if input.Status == "validated" || input.Status == "approved" || input.Status == "active" {
			return validateApplication(system, accessRoles)
		}
		return nil
	})
}

func canTransition(from, to string) bool {
	for _, status := range lifecycleTransitions[from] {
		if status == to {
			return true
		}
	}
	return false
}

func validateApplication(system System, accessRoles int) error {
	required := map[string]string{"displayName": system.DisplayName, "ownerTeam": system.OwnerTeam, "ownerName": system.OwnerName, "ownerEmail": system.OwnerEmail, "supportUrl": system.SupportURL, "environment": system.Environment, "launchUrl": system.LaunchURL}
	for _, key := range []string{"displayName", "ownerTeam", "ownerName", "ownerEmail", "supportUrl", "environment", "launchUrl"} {
		if strings.TrimSpace(required[key]) == "" {
			return fmt.Errorf("%w: %s is required before validation", ErrInvalidInput, key)
		}
	}
	address, err := mail.ParseAddress(system.OwnerEmail)
	if err != nil || address.Address != system.OwnerEmail {
		return fmt.Errorf("%w: ownerEmail must be an email address", ErrInvalidInput)
	}
	if err := validateOptionalURL("supportUrl", system.SupportURL); err != nil {
		return err
	}
	if err := validateOptionalURL("documentationUrl", system.DocumentationURL); err != nil {
		return err
	}
	if system.Environment != "development" && system.Environment != "test" && system.Environment != "staging" && system.Environment != "production" {
		return fmt.Errorf("%w: environment must be development, test, staging, or production", ErrInvalidInput)
	}
	if system.ContractVersion != "1" {
		return fmt.Errorf("%w: unsupported contractVersion", ErrInvalidInput)
	}
	expected := "external"
	if system.SystemType == "platform" {
		expected = "hosted_module"
	}
	if system.IntegrationType != expected {
		return fmt.Errorf("%w: integrationType must match systemType", ErrInvalidInput)
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
