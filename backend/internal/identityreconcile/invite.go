package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// OriginAttribute is the Keycloak user attribute that marks a user created by
// identity-sync for one application account: "<application>:<accountId>".
// A retry finds the user through it instead of creating a second one.
//
// Keycloak 24+ drops attributes the realm's user profile does not declare
// unless unmanaged attributes are enabled; EnsureUser reads the user back and
// refuses to continue (deleting the user it just created) when the attribute
// was not stored.
const OriginAttribute = "ihp_identity_sync_origin"

// Invitation required actions, in the order Keycloak shows them.
var inviteActions = []string{"VERIFY_EMAIL", "UPDATE_PASSWORD"}

var (
	// ErrInviteEmailCollision: an unrelated Keycloak user already has the email.
	ErrInviteEmailCollision = errors.New("an unrelated Keycloak user already has this email")
	// ErrInviteAmbiguous: more than one user carries this account's origin attribute.
	ErrInviteAmbiguous = errors.New("more than one Keycloak user is marked for this application account")
	// ErrInviteUserDisabled: the user created for this account was disabled since.
	ErrInviteUserDisabled = errors.New("the Keycloak user created for this account is disabled")
	// ErrInviteAttributeNotStored: the realm did not keep the origin attribute.
	ErrInviteAttributeNotStored = errors.New("Keycloak did not store the identity-sync origin attribute; declare it in the realm user profile")
	// ErrInviteOriginMismatch: the user marked for this account has another email.
	ErrInviteOriginMismatch = errors.New("the Keycloak user marked for this account has a different email")
	// ErrInviteInvalidAccount: the account has no usable ID or email.
	ErrInviteInvalidAccount = errors.New("account needs an ID and an email to be invited")
	// ErrInviteEmailFailed: the user exists but the invitation email was not sent.
	ErrInviteEmailFailed = errors.New("Keycloak user exists but the invitation email could not be sent")
)

// KeycloakDirectory is the Keycloak implementation of Directory (and
// AccessChecker) for one application: it matches accounts by email, invites
// new users, and grants or checks the application's access role.
type KeycloakDirectory struct {
	Admin *keycloak.KeyAdminClient
	// Issuer is the public issuer written into links (KEYCLOAK_EXTERNAL_URL/realms/<realm>).
	Issuer string
	// Application is the application's Keycloak client ID.
	Application string
	// AccessRole is the registry's access role for the application.
	AccessRole string
}

var (
	_ Directory     = KeycloakDirectory{}
	_ AccessChecker = KeycloakDirectory{}
)

// FindEmail returns every exact email match. Only enabled users with a
// verified email yield a nonzero Identity.
func (d KeycloakDirectory) FindEmail(ctx context.Context, email string) ([]identitysync.Identity, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, errors.New("email is required")
	}
	users, err := d.Admin.SearchUsers(ctx, keycloak.UserSearch{Email: email, Max: 2})
	if err != nil {
		return nil, err
	}
	matches := make([]identitysync.Identity, 0, len(users))
	for _, user := range users {
		if !strings.EqualFold(user.Email, email) {
			return nil, errors.New("Keycloak returned an unexpected email match")
		}
		var identity identitysync.Identity
		if user.Enabled && user.EmailVerified && user.ID != "" {
			identity = identitysync.Identity{Issuer: d.Issuer, Subject: user.ID}
		}
		matches = append(matches, identity)
	}
	return matches, nil
}

func (d KeycloakDirectory) origin(accountID string) string {
	return d.Application + ":" + accountID
}

// findOrigin returns the user created for this application account, if any.
func (d KeycloakDirectory) findOrigin(ctx context.Context, accountID string) (*keycloak.KeycloakUser, error) {
	origin := d.origin(accountID)
	users, err := d.Admin.SearchUsers(ctx, keycloak.UserSearch{AttributeName: OriginAttribute, AttributeValue: origin, Max: 5})
	if err != nil {
		return nil, err
	}
	var found []keycloak.KeycloakUser
	for _, user := range users {
		if hasAttribute(user, OriginAttribute, origin) {
			found = append(found, user)
		}
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return &found[0], nil
	default:
		return nil, ErrInviteAmbiguous
	}
}

func hasAttribute(user keycloak.KeycloakUser, name, value string) bool {
	for _, v := range user.Attributes[name] {
		if v == value {
			return true
		}
	}
	return false
}

func hasPendingInviteActions(user keycloak.KeycloakUser) bool {
	for _, action := range user.RequiredActions {
		for _, invite := range inviteActions {
			if action == invite {
				return true
			}
		}
	}
	return false
}

// EnsureUser returns the Keycloak user for a new application account,
// creating and inviting it when needed. It is idempotent across timeouts:
//
//  1. A user carrying OriginAttribute=<app>:<accountId> is reused. If it still
//     has invitation actions pending, the invitation email is sent again (a
//     previous attempt may have timed out before or after sending it).
//  2. Otherwise any user with the same email (or username) is unrelated and
//     the call fails with ErrInviteEmailCollision.
//  3. Otherwise the user is created (username = email, emailVerified=false,
//     VERIFY_EMAIL and UPDATE_PASSWORD) and the invitation email is sent. A
//     create that times out or conflicts is resolved by searching for the
//     origin attribute again.
func (d KeycloakDirectory) EnsureUser(ctx context.Context, account identitysync.Account) (identitysync.Identity, error) {
	email := strings.TrimSpace(account.Email)
	if d.Admin == nil || d.Issuer == "" || d.Application == "" {
		return identitysync.Identity{}, errors.New("Keycloak directory is not configured")
	}
	if !identitysync.ValidAccountID(identitysync.AccountIDOpaque, account.ID) || email == "" || strings.ContainsAny(email, " \t\r\n") || !strings.Contains(email, "@") {
		return identitysync.Identity{}, ErrInviteInvalidAccount
	}
	existing, err := d.findOrigin(ctx, account.ID)
	if err != nil {
		return identitysync.Identity{}, err
	}
	if existing != nil {
		return d.reuse(ctx, *existing, email)
	}
	others, err := d.Admin.SearchUsers(ctx, keycloak.UserSearch{Email: email, Max: 2})
	if err != nil {
		return identitysync.Identity{}, err
	}
	if len(others) > 0 {
		return identitysync.Identity{}, ErrInviteEmailCollision
	}
	id, createErr := d.Admin.CreateUserFromRequest(ctx, keycloak.CreateUserRequest{
		Username:        strings.ToLower(email),
		Email:           email,
		Enabled:         true,
		EmailVerified:   false,
		RequiredActions: append([]string(nil), inviteActions...),
		Attributes:      map[string][]string{OriginAttribute: {d.origin(account.ID)}},
	})
	if createErr != nil {
		// The create may have succeeded before a timeout, or a concurrent
		// attempt for this same account may have won: look for our marker.
		if ctx.Err() != nil {
			return identitysync.Identity{}, createErr
		}
		existing, err := d.findOrigin(ctx, account.ID)
		if err != nil {
			return identitysync.Identity{}, fmt.Errorf("%w (after create failed: %v)", err, createErr)
		}
		if existing != nil {
			return d.reuse(ctx, *existing, email)
		}
		if errors.Is(createErr, keycloak.ErrUserConflict) {
			return identitysync.Identity{}, ErrInviteEmailCollision
		}
		return identitysync.Identity{}, createErr
	}
	created, err := d.Admin.GetUserRepresentation(ctx, id)
	if err != nil {
		return identitysync.Identity{}, err
	}
	if !hasAttribute(*created, OriginAttribute, d.origin(account.ID)) {
		// Without the marker a retry would see an email collision; remove the
		// user this call created so the realm can be fixed and the row retried.
		if delErr := d.Admin.DeleteUserWithContext(ctx, id); delErr != nil {
			return identitysync.Identity{}, fmt.Errorf("%w; removing the new user also failed: %v", ErrInviteAttributeNotStored, delErr)
		}
		return identitysync.Identity{}, ErrInviteAttributeNotStored
	}
	if err := d.Admin.ExecuteActionsEmail(ctx, id, inviteActions); err != nil {
		return identitysync.Identity{}, ErrInviteEmailFailed
	}
	return identitysync.Identity{Issuer: d.Issuer, Subject: id}, nil
}

// reuse accepts the user marked for this account only while it still has the
// account's email, so a user who could set the attribute on themselves (a
// misconfigured user profile) cannot claim another person's account.
func (d KeycloakDirectory) reuse(ctx context.Context, user keycloak.KeycloakUser, email string) (identitysync.Identity, error) {
	if user.ID == "" {
		return identitysync.Identity{}, errors.New("Keycloak returned a user without an ID")
	}
	if !strings.EqualFold(strings.TrimSpace(user.Email), email) {
		return identitysync.Identity{}, ErrInviteOriginMismatch
	}
	if !user.Enabled {
		return identitysync.Identity{}, ErrInviteUserDisabled
	}
	if !user.EmailVerified && hasPendingInviteActions(user) {
		if err := d.Admin.ExecuteActionsEmail(ctx, user.ID, inviteActions); err != nil {
			return identitysync.Identity{}, ErrInviteEmailFailed
		}
	}
	return identitysync.Identity{Issuer: d.Issuer, Subject: user.ID}, nil
}

func (d KeycloakDirectory) client(_ context.Context) (*keycloak.ClientInfo, error) {
	if d.Application == "" {
		return nil, errors.New("application client ID is not configured")
	}
	client, err := d.Admin.GetClientByClientID(d.Application)
	if err != nil {
		return nil, errors.New("look up the application's Keycloak client failed")
	}
	if client == nil || !client.Enabled || client.ClientID != d.Application || client.ID == "" {
		return nil, errors.New("application Keycloak client is unavailable")
	}
	return client, nil
}

// GrantAccess assigns the application's access role. Repeating it succeeds.
func (d KeycloakDirectory) GrantAccess(ctx context.Context, identity identitysync.Identity) error {
	if d.AccessRole == "" {
		return errors.New("application access role is not configured")
	}
	if !identitysync.ValidIdentity(identity, d.Issuer) {
		return errors.New("identity does not belong to this directory's issuer")
	}
	client, err := d.client(ctx)
	if err != nil {
		return err
	}
	return d.Admin.AssignClientRolesToUser(ctx, identity.Subject, d.Application, client.ID, []string{d.AccessRole})
}

// HasAccess reports whether the user holds the access role, directly or
// through composites and groups. A user Keycloak no longer knows has no access.
func (d KeycloakDirectory) HasAccess(ctx context.Context, identity identitysync.Identity) (bool, error) {
	if d.AccessRole == "" {
		return false, errors.New("application access role is not configured")
	}
	if !identitysync.ValidIdentity(identity, d.Issuer) {
		return false, errors.New("identity does not belong to this directory's issuer")
	}
	client, err := d.client(ctx)
	if err != nil {
		return false, err
	}
	roles, err := d.Admin.GetEffectiveUserClientRoles(ctx, identity.Subject, client.ID)
	if errors.Is(err, keycloak.ErrUserNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role.Name == d.AccessRole {
			return true, nil
		}
	}
	return false, nil
}
