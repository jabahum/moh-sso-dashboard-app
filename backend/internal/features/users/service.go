package users

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/http/apierror"
	"github.com/moh-sso-dashboard/internal/keycloak"
	models "github.com/moh-sso-dashboard/internal/model"
	sharedservice "github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/internal/utils"
)

var errSelfDisable = apierror.New(http.StatusForbidden, "SELF_DISABLE_FORBIDDEN", "You cannot disable your own account")

type CreateUserRequest struct {
	Username         string              `json:"username"`
	Email            string              `json:"email"`
	FirstName        string              `json:"first_name,omitempty"`
	LastName         string              `json:"last_name,omitempty"`
	FullName         string              `json:"full_name,omitempty"`
	Enabled          *bool               `json:"enabled,omitempty"`
	EmailVerified    *bool               `json:"email_verified,omitempty"`
	RequirePwdChange *bool               `json:"require_pwd_change,omitempty"`
	Password         string              `json:"password,omitempty"`
	SendInvite       bool                `json:"send_invite,omitempty"`
	RealmRoles       []string            `json:"realm_roles,omitempty"`
	ClientRoles      map[string][]string `json:"client_roles,omitempty"`
}

type UpdateUserRequest struct {
	ID            string `json:"id"`
	Username      string `json:"username,omitempty"`
	Email         string `json:"email,omitempty"`
	FirstName     string `json:"first_name,omitempty"`
	LastName      string `json:"last_name,omitempty"`
	Enabled       *bool  `json:"enabled,omitempty"`
	EmailVerified *bool  `json:"email_verified,omitempty"`
}

type Service struct {
	repo             Repository
	notifications    sharedservice.NotificationsService
	cfg              *config.Config
	serviceOnlyRoles ServiceOnlyRoleChecker
	accessRemoval    AccessRoleRemovalHook
}

// AccessRoleRemovalHook is told, after the fact, that client roles were removed
// from a person, so access to a registered identity-sync application can be ended
// fully (its sessions included). An error never undoes the role removal; the
// hook records the failure itself (audit and revocation status).
type AccessRoleRemovalHook interface {
	ClientRolesRemoved(ctx context.Context, userID, clientID string, roles []string, actorID string) error
}

// SetAccessRoleRemovalHook enables identity-sync revocation on client-role removal.
// A nil hook disables it.
func (s *Service) SetAccessRoleRemovalHook(hook AccessRoleRemovalHook) {
	s.accessRemoval = hook
}

func (s *Service) clientRolesRemoved(ctx context.Context, userID, clientID string, roles []string, adminID uuid.UUID) {
	if s == nil || s.accessRemoval == nil || len(roles) == 0 {
		return
	}
	if err := s.accessRemoval.ClientRolesRemoved(ctx, userID, clientID, roles, adminID.String()); err != nil {
		log.Printf("WARN: client roles removed from user %s on %s, but access removal is incomplete: %v", userID, clientID, err)
	}
}

// ErrServiceOnlyRole is returned when an administrator tries to give a person a client
// role reserved for Keycloak service accounts.
var ErrServiceOnlyRole = errors.New("service-only system roles cannot be granted to people")

// ServiceOnlyRoleChecker reports whether a client role is reserved for service accounts.
type ServiceOnlyRoleChecker interface {
	IsServiceOnlyRole(ctx context.Context, clientID string, roleName string) (bool, error)
}

// SetServiceOnlyRoleChecker enables the service-only role guard on direct client-role grants.
func (s *Service) SetServiceOnlyRoleChecker(checker ServiceOnlyRoleChecker) {
	s.serviceOnlyRoles = checker
}

func (s *Service) rejectServiceOnlyRoles(ctx context.Context, clientID string, roles []string) error {
	if s.serviceOnlyRoles == nil {
		return nil
	}
	for _, role := range roles {
		serviceOnly, err := s.serviceOnlyRoles.IsServiceOnlyRole(ctx, clientID, role)
		if err != nil {
			return fmt.Errorf("check service-only role: %w", err)
		}
		if serviceOnly {
			return fmt.Errorf("%w: %s/%s", ErrServiceOnlyRole, clientID, role)
		}
	}
	return nil
}

func NewService(
	repo Repository,
	notifications sharedservice.NotificationsService,
	cfg ...*config.Config,
) *Service {
	var appConfig *config.Config
	if len(cfg) > 0 {
		appConfig = cfg[0]
	}

	return &Service{
		repo:          repo,
		notifications: notifications,
		cfg:           appConfig,
	}
}

// ----------------------------------------------------
// USER LIFECYCLE
// ----------------------------------------------------

func (s *Service) CreateUser(
	ctx context.Context,
	req CreateUserRequest,
	adminID uuid.UUID,
) (*models.User, error) {
	if s == nil {
		return nil, errors.New("user service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("user repository is nil")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.FullName = strings.TrimSpace(req.FullName)

	if req.Username == "" {
		return nil, errors.New("username is required")
	}

	if req.Email == "" {
		return nil, errors.New("email is required")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	emailVerified := false
	if req.EmailVerified != nil {
		emailVerified = *req.EmailVerified
	}

	if req.FirstName == "" && req.LastName == "" && req.FullName != "" {
		firstName, lastName := splitFullName(req.FullName)
		req.FirstName = firstName
		req.LastName = lastName
	}

	user := &models.User{
		Username:      req.Username,
		Email:         req.Email,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		FullName:      fullName(req.FirstName, req.LastName),
		Enabled:       enabled,
		EmailVerified: emailVerified,
	}

	kcID, err := s.repo.CreateUser(user)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	user.ID = kcID

	nt := models.UserCreated
	s.notify(ctx, models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "New user account created",
		TargetRole: "admin",
		UserID:     user.ID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":            user.ID,
			"username":           user.Username,
			"email":              user.Email,
			"first_name":         user.FirstName,
			"last_name":          user.LastName,
			"enabled":            user.Enabled,
			"email_verified":     user.EmailVerified,
			"send_invite":        req.SendInvite,
			"require_pwd_change": req.RequirePwdChange != nil && *req.RequirePwdChange,
			"realm_roles":        req.RealmRoles,
			"client_roles":       req.ClientRoles,
			"admin_id":           adminID.String(),
		}),
	})

	return user, nil
}

func (s *Service) UpdateUser(
	ctx context.Context,
	req UpdateUserRequest,
	adminID uuid.UUID,
) (*models.User, error) {
	if s == nil {
		return nil, errors.New("user service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("user repository is nil")
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	if req.ID == "" {
		return nil, errors.New("user id is required")
	}

	userID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}
	if req.Enabled != nil && !*req.Enabled && userID == adminID {
		return nil, errSelfDisable
	}

	current, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("get user before update: %w", err)
	}

	enabled := current.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	emailVerified := current.EmailVerified
	if req.EmailVerified != nil {
		emailVerified = *req.EmailVerified
	}

	updated := &models.User{
		ID:            req.ID,
		Username:      firstNonEmpty(req.Username, current.Username),
		Email:         firstNonEmpty(req.Email, current.Email),
		FirstName:     firstNonEmpty(req.FirstName, current.FirstName),
		LastName:      firstNonEmpty(req.LastName, current.LastName),
		Enabled:       enabled,
		EmailVerified: emailVerified,
	}

	updated.FullName = fullName(updated.FirstName, updated.LastName)

	if err := s.repo.UpdateUser(updated); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	nt := models.UserUpdated
	s.notify(ctx, models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User account updated",
		TargetRole: "admin",
		UserID:     updated.ID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":          updated.ID,
			"previous_email":   current.Email,
			"email":            updated.Email,
			"previous_enabled": current.Enabled,
			"enabled":          updated.Enabled,
			"email_verified":   updated.EmailVerified,
			"admin_id":         adminID.String(),
		}),
	})

	return updated, nil
}

func (s *Service) SetUserEnabled(
	ctx context.Context,
	userID uuid.UUID,
	enabled bool,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}
	if !enabled && userID == adminID {
		return errSelfDisable
	}

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("get user before toggle enabled: %w", err)
	}

	if user.Enabled == enabled {
		return nil
	}

	if err := s.repo.ToggleUserEnabled(ctx, userID.String(), enabled); err != nil {
		return fmt.Errorf("toggle user enabled: %w", err)
	}

	var nt models.NotificationType
	msg := "User account updated"

	if enabled {
		nt = models.UserEnabled
		msg = "User account enabled"
	} else {
		nt = models.UserDisabled
		msg = "User account disabled"
	}

	notification := models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    msg,
		TargetRole: "admin",
		UserID:     user.ID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":          user.ID,
			"username":         user.Username,
			"email":            user.Email,
			"previous_enabled": user.Enabled,
			"enabled":          enabled,
			"admin_id":         adminID.String(),
		}),
	}

	if !enabled {
		s.attachAdminEmailDelivery(
			&notification,
			"user-disabled",
			"User account disabled",
			fmt.Sprintf("User account %s was disabled.", user.Username),
			map[string]any{
				"Name":      s.systemAdminName(),
				"Platform":  s.platformName(),
				"Username":  user.Username,
				"Email":     user.Email,
				"ActionURL": s.adminUsersURL(),
				"Details": fmt.Sprintf(
					"User ID: %s\nUsername: %s\nEmail: %s\nPrevious Enabled: %v\nEnabled: %v\nAdmin ID: %s",
					user.ID,
					user.Username,
					user.Email,
					user.Enabled,
					enabled,
					adminID.String(),
				),
			},
		)
	}

	s.notify(ctx, notification)

	return nil
}

func (s *Service) ResetUserPassword(
	ctx context.Context,
	userID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.SendUserPasswordResetEmail(ctx, userID, adminID)
}

func (s *Service) DeleteUser(
	ctx context.Context,
	id uuid.UUID,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("user id is required")
	}

	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return fmt.Errorf("get user before delete: %w", err)
	}

	if err := s.repo.DeleteUser(id.String()); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	nt := models.UserDeleted
	notification := models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User account deleted",
		TargetRole: "admin",
		UserID:     user.ID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":  user.ID,
			"username": user.Username,
			"email":    user.Email,
			"admin_id": adminID.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"user-deleted",
		"User account deleted",
		fmt.Sprintf("User account %s was deleted.", user.Username),
		map[string]any{
			"Name":      s.systemAdminName(),
			"Platform":  s.platformName(),
			"Username":  user.Username,
			"Email":     user.Email,
			"ActionURL": s.adminUsersURL(),
			"Details": fmt.Sprintf(
				"User ID: %s\nUsername: %s\nEmail: %s\nAdmin ID: %s",
				user.ID,
				user.Username,
				user.Email,
				adminID.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

func (s *Service) GetUser(id uuid.UUID) (*models.User, error) {
	if s == nil {
		return nil, errors.New("user service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("user repository is nil")
	}

	if id == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

func (s *Service) ListUsers() ([]models.User, error) {
	if s == nil {
		return nil, errors.New("user service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("user repository is nil")
	}

	users, err := s.repo.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}

func (s *Service) SyncUsersFromKeycloak(ctx context.Context) (int, error) {
	if s == nil {
		return 0, errors.New("user service is nil")
	}

	if s.repo == nil {
		return 0, errors.New("user repository is nil")
	}

	return s.repo.SyncUsersFromKeycloak(ctx)
}

// ----------------------------------------------------
// USER EMAIL ACTIONS
// ----------------------------------------------------

func (s *Service) SendUserOnboardingEmail(
	ctx context.Context,
	userID uuid.UUID,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("get user before sending onboarding email: %w", err)
	}

	if err := s.repo.SendUserOnboardingEmail(ctx, userID.String()); err != nil {
		return fmt.Errorf("send onboarding email: %w", err)
	}

	nt := models.UserInvitationSent
	notification := models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User onboarding email sent",
		TargetRole: "admin",
		UserID:     user.ID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":  user.ID,
			"username": user.Username,
			"email":    user.Email,
			"admin_id": adminID.String(),
		}),
	}

	s.notify(ctx, notification)

	return nil
}

func (s *Service) SendUserVerificationEmail(
	ctx context.Context,
	userID uuid.UUID,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("get user before sending verification email: %w", err)
	}

	if err := s.repo.SendUserVerificationEmail(ctx, userID.String()); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}

	nt := models.UserVerificationEmailSent
	notification := models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User verification email sent",
		TargetRole: "admin",
		UserID:     user.ID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":        user.ID,
			"username":       user.Username,
			"email":          user.Email,
			"email_verified": user.EmailVerified,
			"admin_id":       adminID.String(),
		}),
	}

	s.notify(ctx, notification)

	return nil
}

func (s *Service) SendUserPasswordResetEmail(
	ctx context.Context,
	userID uuid.UUID,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("get user before password reset: %w", err)
	}

	if err := s.repo.SendUserPasswordResetEmail(ctx, userID.String()); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	nt := models.UserPasswordReset
	notification := models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User password reset email sent",
		TargetRole: "admin",
		UserID:     user.ID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":  user.ID,
			"username": user.Username,
			"email":    user.Email,
			"admin_id": adminID.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"user-password-reset",
		"User password reset",
		fmt.Sprintf("Password reset was triggered for user %s.", user.Username),
		map[string]any{
			"Name":      s.systemAdminName(),
			"Platform":  s.platformName(),
			"Username":  user.Username,
			"Email":     user.Email,
			"ActionURL": s.adminUsersURL(),
			"Details": fmt.Sprintf(
				"User ID: %s\nUsername: %s\nEmail: %s\nAdmin ID: %s",
				user.ID,
				user.Username,
				user.Email,
				adminID.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

// ----------------------------------------------------
// CLIENT ROLES
// ----------------------------------------------------

func (s *Service) GetUserClientRoles(
	ctx context.Context,
	userID uuid.UUID,
) ([]keycloak.UserClientRoleAssignment, error) {
	if s == nil {
		return nil, errors.New("user service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	return s.repo.GetUserClientRoles(ctx, userID.String())
}

func (s *Service) UpdateUserRealmRoles(
	ctx context.Context,
	userID uuid.UUID,
	roles []string,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	roles = normalizeRoleNames(roles)

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("get user before realm role update: %w", err)
	}

	currentSet := make(map[string]bool)
	for _, role := range normalizeRoleNames(user.RealmRoles) {
		currentSet[role] = true
	}

	desiredSet := make(map[string]bool)
	for _, role := range roles {
		desiredSet[role] = true
	}

	var toAdd []string
	var toRemove []string

	for role := range desiredSet {
		if !currentSet[role] {
			toAdd = append(toAdd, role)
		}
	}

	for role := range currentSet {
		if !desiredSet[role] {
			toRemove = append(toRemove, role)
		}
	}

	if len(toAdd) > 0 {
		if err := s.repo.AddUserRealmRoles(ctx, userID.String(), toAdd); err != nil {
			return fmt.Errorf("add user realm roles: %w", err)
		}
	}

	if len(toRemove) > 0 {
		if err := s.repo.RemoveUserRealmRoles(ctx, userID.String(), toRemove); err != nil {
			return fmt.Errorf("remove user realm roles: %w", err)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil
	}

	nt := models.UserUpdated
	s.notify(ctx, models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User realm roles updated",
		TargetRole: "admin",
		UserID:     userID.String(),
		Metadata: utils.MustJSON(map[string]any{
			"user_id":  userID.String(),
			"username": user.Username,
			"email":    user.Email,
			"added":    toAdd,
			"removed":  toRemove,
			"roles":    roles,
			"admin_id": adminID.String(),
		}),
	})

	return nil
}

func (s *Service) GetUserClientRolesForClient(
	ctx context.Context,
	userID uuid.UUID,
	clientID string,
	clientUUID string,
) ([]keycloak.ClientRoleRep, error) {
	if s == nil {
		return nil, errors.New("user service is nil")
	}

	if s.repo == nil {
		return nil, errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if clientID == "" {
		return nil, errors.New("client id is required")
	}

	return s.repo.GetUserClientRolesForClient(
		ctx,
		userID.String(),
		clientID,
		clientUUID,
	)
}

func (s *Service) UpdateUserClientRoles(
	ctx context.Context,
	userID uuid.UUID,
	clientID string,
	clientUUID string,
	roles []string,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if clientID == "" {
		return errors.New("client id is required")
	}

	roles = normalizeRoleNames(roles)

	current, err := s.repo.GetUserClientRolesForClient(
		ctx,
		userID.String(),
		clientID,
		clientUUID,
	)
	if err != nil {
		return fmt.Errorf("get current user client roles: %w", err)
	}

	currentSet := make(map[string]bool)
	for _, role := range current {
		roleName := strings.TrimSpace(role.Name)
		if roleName != "" {
			currentSet[roleName] = true
		}
	}

	desiredSet := make(map[string]bool)
	for _, role := range roles {
		desiredSet[role] = true
	}

	var toAdd []string
	var toRemove []string

	for role := range desiredSet {
		if !currentSet[role] {
			toAdd = append(toAdd, role)
		}
	}

	for role := range currentSet {
		if !desiredSet[role] {
			toRemove = append(toRemove, role)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil
	}

	if err := s.rejectServiceOnlyRoles(ctx, clientID, toAdd); err != nil {
		return err
	}

	if len(toAdd) > 0 {
		if err := s.repo.AddUserClientRoles(
			ctx,
			userID.String(),
			clientID,
			clientUUID,
			toAdd,
		); err != nil {
			return fmt.Errorf("add user client roles: %w", err)
		}
	}

	if len(toRemove) > 0 {
		if err := s.repo.RemoveUserClientRoles(
			ctx,
			userID.String(),
			clientID,
			clientUUID,
			toRemove,
		); err != nil {
			return fmt.Errorf("remove user client roles: %w", err)
		}
		s.clientRolesRemoved(ctx, userID.String(), clientID, toRemove, adminID)
	}

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		user = &models.User{
			ID:       userID.String(),
			Username: "unknown",
		}
	}

	nt := models.ClientRolesUpdated
	notification := models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User client roles updated",
		TargetRole: "admin",
		UserID:     userID.String(),
		ClientID:   clientID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":     userID.String(),
			"username":    user.Username,
			"email":       user.Email,
			"client_id":   clientID,
			"client_uuid": clientUUID,
			"added":       toAdd,
			"removed":     toRemove,
			"roles":       roles,
			"admin_id":    adminID.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"client-roles-updated",
		"User client roles updated",
		fmt.Sprintf("Client roles were updated for user %s.", user.Username),
		map[string]any{
			"Name":         s.systemAdminName(),
			"Platform":     s.platformName(),
			"Username":     user.Username,
			"Email":        user.Email,
			"ClientID":     clientID,
			"ClientUUID":   clientUUID,
			"AddedRoles":   strings.Join(toAdd, ", "),
			"RemovedRoles": strings.Join(toRemove, ", "),
			"ActionURL":    s.adminUsersURL(),
			"Details": fmt.Sprintf(
				"User ID: %s\nUsername: %s\nEmail: %s\nClient ID: %s\nClient UUID: %s\nAdded Roles: %s\nRemoved Roles: %s\nAll Roles: %s\nAdmin ID: %s",
				userID.String(),
				user.Username,
				user.Email,
				clientID,
				clientUUID,
				strings.Join(toAdd, ", "),
				strings.Join(toRemove, ", "),
				strings.Join(roles, ", "),
				adminID.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

func (s *Service) AddUserClientRoles(
	ctx context.Context,
	userID uuid.UUID,
	clientID string,
	clientUUID string,
	roles []string,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)
	roles = normalizeRoleNames(roles)

	if clientID == "" {
		return errors.New("client id is required")
	}

	if len(roles) == 0 {
		return nil
	}

	if err := s.rejectServiceOnlyRoles(ctx, clientID, roles); err != nil {
		return err
	}

	if err := s.repo.AddUserClientRoles(
		ctx,
		userID.String(),
		clientID,
		clientUUID,
		roles,
	); err != nil {
		return fmt.Errorf("add user client roles: %w", err)
	}

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		user = &models.User{
			ID:       userID.String(),
			Username: "unknown",
		}
	}

	nt := models.ClientRolesUpdated
	s.notify(ctx, models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User client roles added",
		TargetRole: "admin",
		UserID:     userID.String(),
		ClientID:   clientID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":     userID.String(),
			"username":    user.Username,
			"email":       user.Email,
			"client_id":   clientID,
			"client_uuid": clientUUID,
			"added":       roles,
			"admin_id":    adminID.String(),
		}),
	})

	return nil
}

func (s *Service) RemoveUserClientRoles(
	ctx context.Context,
	userID uuid.UUID,
	clientID string,
	clientUUID string,
	roles []string,
	adminID uuid.UUID,
) error {
	if s == nil {
		return errors.New("user service is nil")
	}

	if s.repo == nil {
		return errors.New("user repository is nil")
	}

	if userID == uuid.Nil {
		return errors.New("user id is required")
	}

	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)
	roles = normalizeRoleNames(roles)

	if clientID == "" {
		return errors.New("client id is required")
	}

	if len(roles) == 0 {
		return nil
	}

	if err := s.repo.RemoveUserClientRoles(
		ctx,
		userID.String(),
		clientID,
		clientUUID,
		roles,
	); err != nil {
		return fmt.Errorf("remove user client roles: %w", err)
	}
	s.clientRolesRemoved(ctx, userID.String(), clientID, roles, adminID)

	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		user = &models.User{
			ID:       userID.String(),
			Username: "unknown",
		}
	}

	nt := models.ClientRolesUpdated
	s.notify(ctx, models.Notification{
		Type:       string(nt),
		Title:      nt.Title(),
		Severity:   nt.Severity(),
		Message:    "User client roles removed",
		TargetRole: "admin",
		UserID:     userID.String(),
		ClientID:   clientID,
		Metadata: utils.MustJSON(map[string]any{
			"user_id":     userID.String(),
			"username":    user.Username,
			"email":       user.Email,
			"client_id":   clientID,
			"client_uuid": clientUUID,
			"removed":     roles,
			"admin_id":    adminID.String(),
		}),
	})

	return nil
}

// ----------------------------------------------------
// NOTIFICATIONS
// ----------------------------------------------------

func (s *Service) notify(
	ctx context.Context,
	notification models.Notification,
) {
	if s == nil || s.notifications == nil {
		return
	}

	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}

	_, _ = s.notifications.Notify(ctx, notification)
}

func (s *Service) attachAdminEmailDelivery(
	notification *models.Notification,
	templateName string,
	subject string,
	textBody string,
	templateData map[string]any,
) {
	if notification == nil {
		return
	}

	adminEmail := strings.TrimSpace(s.systemAdminEmail())
	if adminEmail == "" {
		return
	}

	if templateData == nil {
		templateData = map[string]any{}
	}

	if _, ok := templateData["Name"]; !ok {
		templateData["Name"] = s.systemAdminName()
	}

	if _, ok := templateData["Platform"]; !ok {
		templateData["Platform"] = s.platformName()
	}

	if _, ok := templateData["ActionURL"]; !ok {
		templateData["ActionURL"] = s.adminUsersURL()
	}

	notification.Deliveries = []models.NotificationDeliveryRequest{
		{
			Channel: models.NotificationChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		},
		{
			Channel: models.NotificationChannelEmail,
			Recipient: map[string]any{
				"name":  s.systemAdminName(),
				"email": adminEmail,
			},
			TemplateName: templateName,
			TemplateData: templateData,
			Payload: map[string]any{
				"subject":   subject,
				"text_body": textBody,
			},
			MaxAttempts: 5,
		},
	}
}

// ----------------------------------------------------
// CONFIG HELPERS
// ----------------------------------------------------

func (s *Service) platformName() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.Notification.PlatformName) != "" {
		return strings.TrimSpace(s.cfg.Notification.PlatformName)
	}

	return "MOH Integrated Health Portal"
}

func (s *Service) systemAdminName() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.Notification.SystemAdminName) != "" {
		return strings.TrimSpace(s.cfg.Notification.SystemAdminName)
	}

	return "System Administrator"
}

func (s *Service) systemAdminEmail() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.Notification.SystemAdminEmail) != "" {
		return strings.TrimSpace(s.cfg.Notification.SystemAdminEmail)
	}

	return ""
}

func (s *Service) adminDashboardURL() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.Notification.AdminDashboardURL) != "" {
		return strings.TrimSpace(s.cfg.Notification.AdminDashboardURL)
	}

	return "http://localhost:3000/admin/home"
}

func (s *Service) adminUsersURL() string {
	base := strings.TrimRight(s.adminDashboardURL(), "/")

	if strings.HasSuffix(base, "/admin/home") {
		return strings.TrimSuffix(base, "/admin/home") + "/admin/users"
	}

	return base + "/users"
}

// ----------------------------------------------------
// HELPERS
// ----------------------------------------------------

func normalizeRoleNames(roles []string) []string {
	if len(roles) == 0 {
		return []string{}
	}

	seen := make(map[string]bool)
	out := make([]string, 0, len(roles))

	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}

		if seen[role] {
			continue
		}

		seen[role] = true
		out = append(out, role)
	}

	return out
}

func splitFullName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}

	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0], ""
	}

	return parts[0], strings.Join(parts[1:], " ")
}

func fullName(firstName string, lastName string) string {
	return strings.TrimSpace(
		strings.Join(
			[]string{
				strings.TrimSpace(firstName),
				strings.TrimSpace(lastName),
			},
			" ",
		),
	)
}

func firstNonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}

	return strings.TrimSpace(fallback)
}
