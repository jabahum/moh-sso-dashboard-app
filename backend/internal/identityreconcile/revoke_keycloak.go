package identityreconcile

import (
	"context"
	"errors"

	"github.com/moh-sso-dashboard/internal/keycloak"
)

// KeycloakAccessAdmin is the part of the Keycloak admin client access removal
// uses. *keycloak.KeyAdminClient implements it.
type KeycloakAccessAdmin interface {
	RemoveUserClientRoleIfPresent(ctx context.Context, userID, clientID, roleName string) (bool, error)
	UserHasEffectiveClientRole(ctx context.Context, userID, clientID, roleName string) (bool, error)
	RevokeUserClientConsent(ctx context.Context, userID, clientID string) error
}

// KeycloakAccessDirectory implements AccessDirectory with the Keycloak admin
// API. It never calls a user-wide logout: the user's SSO session and other
// applications are left alone.
type KeycloakAccessDirectory struct {
	Admin KeycloakAccessAdmin
}

var _ AccessDirectory = KeycloakAccessDirectory{}

func (d KeycloakAccessDirectory) RemoveAccessRole(ctx context.Context, subject, clientID, role string) error {
	if _, err := d.Admin.RemoveUserClientRoleIfPresent(ctx, subject, clientID, role); err != nil {
		if errors.Is(err, keycloak.ErrUserNotFound) {
			return nil // a deleted user holds no access
		}
		return err
	}
	still, err := d.HasAccessRole(ctx, subject, clientID, role)
	if err != nil {
		return err
	}
	if still {
		return ErrAccessRoleInherited
	}
	return nil
}

func (d KeycloakAccessDirectory) RevokeClientConsents(ctx context.Context, subject, clientID string) error {
	return d.Admin.RevokeUserClientConsent(ctx, subject, clientID)
}

func (d KeycloakAccessDirectory) HasAccessRole(ctx context.Context, subject, clientID, role string) (bool, error) {
	has, err := d.Admin.UserHasEffectiveClientRole(ctx, subject, clientID, role)
	if errors.Is(err, keycloak.ErrUserNotFound) {
		return false, nil
	}
	return has, err
}
