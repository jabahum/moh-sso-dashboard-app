package users

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/config"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	"github.com/moh-sso-dashboard/internal/keycloak"
	logger "github.com/moh-sso-dashboard/internal/log"
	models "github.com/moh-sso-dashboard/internal/model"
)

type userRepository struct {
	keycloakClient *keycloak.KeyAdminClient
	config         *config.Config
	db             db.Store
	logger         *logger.Logger
}

func NewUserRepository(
	keycloakClient *keycloak.KeyAdminClient,
	config *config.Config,
	store db.Store,
	log logger.Logger,
) UserRepository {
	return &userRepository{
		keycloakClient: keycloakClient,
		config:         config,
		db:             store,
		logger:         &log,
	}
}

// ----------------------------------------------------
// USERS
// ----------------------------------------------------

func (r *userRepository) CreateUser(user *models.User) (string, error) {
	ctx := context.Background()

	if user == nil {
		return "", fmt.Errorf("user is required")
	}

	if strings.TrimSpace(user.Username) == "" {
		return "", fmt.Errorf("username is required")
	}

	if strings.TrimSpace(user.Email) == "" {
		return "", fmt.Errorf("email is required")
	}

	if r == nil || r.keycloakClient == nil {
		return "", fmt.Errorf("user repository is not configured")
	}

	kcID, err := r.keycloakClient.CreateUser(user)
	if err != nil {
		r.logger.Error("failed creating user in keycloak", "error", err)
		return "", fmt.Errorf("keycloak user creation failed: %w", err)
	}

	uid, err := uuid.Parse(kcID)
	if err != nil {
		return "", fmt.Errorf("invalid keycloak user id: %w", err)
	}

	if err := r.upsertLocalUser(ctx, uid, user); err != nil {
		r.logger.Warn(
			"user created in keycloak but db sync failed",
			"userId", kcID,
			"error", err,
		)
	}

	if err := r.keycloakClient.SendUserOnboardingEmail(ctx, kcID); err != nil {
		r.logger.Warn(
			"user created in keycloak but onboarding email failed",
			"userId", kcID,
			"error", err,
		)
	}

	return kcID, nil
}

func (r *userRepository) GetUserByID(id uuid.UUID) (*models.User, error) {
	ctx := context.Background()

	if r == nil || r.keycloakClient == nil {
		return nil, fmt.Errorf("user repository is not configured")
	}

	kcUser, err := r.keycloakClient.GetUser(id.String())
	if err != nil {
		return nil, fmt.Errorf("keycloak user not found: %w", err)
	}
	// GET users/{id} has no role mappings; read them explicitly. Failing here
	// rather than returning empty roles matters: callers compute role removals
	// from this set.
	realmRoles, clientRoles, err := r.keycloakClient.GetUserRoleMappings(ctx, id.String())
	if err != nil {
		return nil, fmt.Errorf("keycloak user role mappings: %w", err)
	}
	kcUser.RealmRoles = realmRoles
	kcUser.ClientRoles = clientRoles

	row, err := r.db.GetUserByID(ctx, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var createdAt time.Time
	var updatedAt time.Time
	var lastLoginAt *time.Time

	if err == nil {
		if row.CreatedAt.Valid {
			createdAt = row.CreatedAt.Time
		}

		if row.UpdatedAt.Valid {
			updatedAt = row.UpdatedAt.Time
		}

		lastLoginAt = pickTime(row.LastLoginAt, kcUser.LastLoginAt)
	} else {
		lastLoginAt = kcUser.LastLoginAt
		createdAt = keycloakCreatedAt(kcUser)
	}

	return &models.User{
		ID:            kcUser.ID,
		Username:      kcUser.Username,
		Email:         kcUser.Email,
		FirstName:     kcUser.FirstName,
		LastName:      kcUser.LastName,
		FullName:      repositoryFullName(kcUser.FirstName, kcUser.LastName),
		RealmRoles:    kcUser.RealmRoles,
		ClientRoles:   kcUser.ClientRoles,
		IsAdmin:       slices.Contains(kcUser.RealmRoles, "admin"),
		Enabled:       kcUser.Enabled,
		EmailVerified: kcUser.EmailVerified,
		LastLoginAt:   lastLoginAt,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}

func (r *userRepository) GetUserByUsername(
	ctx context.Context,
	username string,
) (*models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	if r == nil || r.keycloakClient == nil {
		return nil, fmt.Errorf("user repository is not configured")
	}

	row, err := r.db.GetUserByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn(
				"user not found by username",
				"username", username,
			)

			return nil, nil
		}

		return nil, fmt.Errorf(
			"get user by username %q: %w",
			username,
			err,
		)
	}

	user, err := r.GetUserByID(row.ID)
	if err != nil {
		return nil, fmt.Errorf(
			"load keycloak user for username %q: %w",
			username,
			err,
		)
	}

	return user, nil
}

func (r *userRepository) ListUsers() ([]models.User, error) {
	ctx := context.Background()

	if r == nil || r.keycloakClient == nil {
		return nil, fmt.Errorf("user repository is not configured")
	}

	kcUsers, err := r.keycloakClient.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keycloak users: %w", err)
	}

	dbUsers, err := r.db.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	dbMap := make(map[string]db.User, len(dbUsers))
	for _, user := range dbUsers {
		dbMap[user.ID.String()] = user
	}

	result := make([]models.User, 0, len(kcUsers))

	for _, kcUser := range kcUsers {
		localUser, hasLocalUser := dbMap[kcUser.ID]

		user := models.User{
			ID:            kcUser.ID,
			Username:      kcUser.Username,
			Email:         kcUser.Email,
			FirstName:     kcUser.FirstName,
			LastName:      kcUser.LastName,
			FullName:      repositoryFullName(kcUser.FirstName, kcUser.LastName),
			RealmRoles:    kcUser.RealmRoles,
			ClientRoles:   kcUser.ClientRoles,
			IsAdmin:       slices.Contains(kcUser.RealmRoles, "admin"),
			Enabled:       kcUser.Enabled,
			EmailVerified: kcUser.EmailVerified,
			LastLoginAt:   kcUser.LastLoginAt,
			CreatedAt:     keycloakCreatedAt(&kcUser),
		}

		if hasLocalUser {
			user.ID = localUser.ID.String()
			user.LastLoginAt = pickTime(localUser.LastLoginAt, kcUser.LastLoginAt)

			if localUser.CreatedAt.Valid {
				user.CreatedAt = localUser.CreatedAt.Time
			}

			if localUser.UpdatedAt.Valid {
				user.UpdatedAt = localUser.UpdatedAt.Time
			}
		}

		result = append(result, user)
	}

	return result, nil
}

func (r *userRepository) SyncUsersFromKeycloak(ctx context.Context) (int, error) {
	if r == nil || r.keycloakClient == nil {
		return 0, fmt.Errorf("user repository is not configured")
	}

	kcUsers, err := r.keycloakClient.ListUsers()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch keycloak users: %w", err)
	}

	synced := 0
	for _, kcUser := range kcUsers {
		userID, err := uuid.Parse(strings.TrimSpace(kcUser.ID))
		if err != nil {
			r.logger.Warn(
				"skipping keycloak user with invalid id during startup sync",
				"userId", kcUser.ID,
				"username", kcUser.Username,
				"error", err,
			)
			continue
		}

		user := &models.User{
			ID:            kcUser.ID,
			Username:      kcUser.Username,
			Email:         kcUser.Email,
			FirstName:     kcUser.FirstName,
			LastName:      kcUser.LastName,
			Enabled:       kcUser.Enabled,
			EmailVerified: kcUser.EmailVerified,
		}
		if err := r.upsertLocalUser(ctx, userID, user); err != nil {
			r.logger.Warn(
				"failed to sync keycloak user into local db",
				"userId", kcUser.ID,
				"username", kcUser.Username,
				"error", err,
			)
			continue
		}
		synced++
	}

	return synced, nil
}

func (r *userRepository) UpdateUser(user *models.User) error {
	ctx := context.Background()

	if user == nil {
		return fmt.Errorf("user is required")
	}

	if strings.TrimSpace(user.ID) == "" {
		return fmt.Errorf("user id is required")
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	uid, err := uuid.Parse(user.ID)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	if err := r.keycloakClient.UpdateUser(&models.User{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Enabled:       user.Enabled,
		EmailVerified: user.EmailVerified,
	}); err != nil {
		return fmt.Errorf("keycloak update failed: %w", err)
	}

	if err := r.upsertLocalUser(ctx, uid, user); err != nil {
		r.logger.Warn(
			"user updated in keycloak but db sync failed",
			"userId", user.ID,
			"error", err,
		)
	}

	return nil
}

func (r *userRepository) DeleteUser(id string) error {
	ctx := context.Background()

	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("user id is required")
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	if err := r.keycloakClient.DeleteUser(id); err != nil {
		return fmt.Errorf("keycloak delete failed: %w", err)
	}

	if err := r.db.DeleteUser(ctx, uid); err != nil {
		r.logger.Warn(
			"user deleted in keycloak but db delete failed",
			"userId", id,
			"error", err,
		)
	}

	return nil
}

func (r *userRepository) ToggleUserEnabled(
	ctx context.Context,
	userID string,
	enabled bool,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	if err := r.keycloakClient.SetUserEnabled(ctx, userID, enabled); err != nil {
		return fmt.Errorf("keycloak toggle user failed: %w", err)
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	if err := r.db.UpdateUserEnabled(ctx, db.UpdateUserEnabledParams{
		ID:      uid,
		Enabled: sql.NullBool{Bool: enabled, Valid: true},
	}); err != nil {
		r.logger.Warn(
			"user enabled toggled in keycloak but db sync failed",
			"userId", userID,
			"enabled", enabled,
			"error", err,
		)
	}

	return nil
}

// ResetUserPassword sends a Keycloak UPDATE_PASSWORD email.
// It does not generate or store a temporary password.
func (r *userRepository) ResetUserPassword(
	ctx context.Context,
	userID string,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	if err := r.keycloakClient.SendUserPasswordResetEmail(ctx, userID); err != nil {
		return fmt.Errorf("send password reset email failed: %w", err)
	}

	r.logger.Info(
		"user password reset email sent",
		"userId", userID,
	)

	return nil
}

// ----------------------------------------------------
// USER EMAIL ACTIONS
// ----------------------------------------------------

func (r *userRepository) SendUserOnboardingEmail(
	ctx context.Context,
	userID string,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	if err := r.keycloakClient.SendUserOnboardingEmail(ctx, userID); err != nil {
		return fmt.Errorf("send onboarding email failed: %w", err)
	}

	r.logger.Info("user onboarding email sent", "userId", userID)

	return nil
}

func (r *userRepository) SendUserVerificationEmail(
	ctx context.Context,
	userID string,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	if err := r.keycloakClient.SendUserVerificationEmail(ctx, userID); err != nil {
		return fmt.Errorf("send verification email failed: %w", err)
	}

	r.logger.Info("user verification email sent", "userId", userID)

	return nil
}

func (r *userRepository) SendUserPasswordResetEmail(
	ctx context.Context,
	userID string,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	if err := r.keycloakClient.SendUserPasswordResetEmail(ctx, userID); err != nil {
		return fmt.Errorf("send password reset email failed: %w", err)
	}

	r.logger.Info("user password reset email sent", "userId", userID)

	return nil
}

// ----------------------------------------------------
// REALM ROLES
// ----------------------------------------------------

func (r *userRepository) GetUsersByRealmRole(
	ctx context.Context,
	roleName string,
) ([]models.User, error) {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return nil, fmt.Errorf("roleName is required")
	}

	if r == nil || r.keycloakClient == nil {
		return nil, fmt.Errorf("user repository is not configured")
	}

	users, err := r.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("list users for realm role lookup: %w", err)
	}

	targetRole := strings.ToLower(roleName)
	result := make([]models.User, 0)

	for _, user := range users {
		if !user.Enabled {
			continue
		}

		if strings.TrimSpace(user.Email) == "" {
			continue
		}

		for _, realmRole := range user.RealmRoles {
			if strings.ToLower(strings.TrimSpace(realmRole)) == targetRole {
				result = append(result, user)
				break
			}
		}
	}

	return result, nil
}

func (r *userRepository) AddUserRealmRoles(
	ctx context.Context,
	userID string,
	roles []string,
) error {
	userID = strings.TrimSpace(userID)
	roles = normalizeRepositoryRoleNames(roles)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if len(roles) == 0 {
		return nil
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	for _, roleName := range roles {
		role, err := r.keycloakClient.GetRealmRoleByName(ctx, roleName)
		if err != nil {
			return fmt.Errorf("resolve realm role %q: %w", roleName, err)
		}

		if err := r.keycloakClient.AddRealmRoleToUser(ctx, userID, *role); err != nil {
			return fmt.Errorf("add realm role %q: %w", roleName, err)
		}
	}

	return nil
}

func (r *userRepository) RemoveUserRealmRoles(
	ctx context.Context,
	userID string,
	roles []string,
) error {
	userID = strings.TrimSpace(userID)
	roles = normalizeRepositoryRoleNames(roles)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if len(roles) == 0 {
		return nil
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	for _, roleName := range roles {
		role, err := r.keycloakClient.GetRealmRoleByName(ctx, roleName)
		if err != nil {
			return fmt.Errorf("resolve realm role %q: %w", roleName, err)
		}

		if err := r.keycloakClient.RemoveRealmRoleFromUser(ctx, userID, *role); err != nil {
			return fmt.Errorf("remove realm role %q: %w", roleName, err)
		}
	}

	return nil
}

// ----------------------------------------------------
// CLIENT ROLES
// ----------------------------------------------------

func (r *userRepository) GetUserClientRoles(
	ctx context.Context,
	userID string,
) ([]keycloak.UserClientRoleAssignment, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	if r == nil || r.keycloakClient == nil {
		return nil, fmt.Errorf("user repository is not configured")
	}

	clients, err := r.keycloakClient.ListClients()
	if err != nil {
		return nil, err
	}

	out := make([]keycloak.UserClientRoleAssignment, 0)

	for _, client := range clients {
		if strings.TrimSpace(client.ID) == "" || strings.TrimSpace(client.ClientID) == "" {
			continue
		}

		roles, err := r.keycloakClient.GetUserClientRoles(
			ctx,
			userID,
			client.ClientID,
			client.ID,
		)
		if err != nil {
			r.logger.Warn(
				"failed to fetch user client roles",
				"userId", userID,
				"clientId", client.ClientID,
				"clientUuid", client.ID,
				"error", err,
			)
			continue
		}

		if len(roles) == 0 {
			continue
		}

		names := make([]string, 0, len(roles))
		for _, role := range roles {
			if strings.TrimSpace(role.Name) == "" {
				continue
			}

			names = append(names, role.Name)
		}

		if len(names) == 0 {
			continue
		}

		out = append(out, keycloak.UserClientRoleAssignment{
			Id:         client.ID,
			ClientID:   client.ClientID,
			ClientName: client.Name,
			Roles:      names,
		})
	}

	return out, nil
}

func (r *userRepository) GetUserClientRolesForClient(
	ctx context.Context,
	userID string,
	clientID string,
	clientUUID string,
) ([]keycloak.ClientRoleRep, error) {
	userID = strings.TrimSpace(userID)
	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	if clientID == "" {
		return nil, fmt.Errorf("clientID is required")
	}

	if r == nil || r.keycloakClient == nil {
		return nil, fmt.Errorf("user repository is not configured")
	}

	if clientUUID == "" {
		client, err := r.keycloakClient.GetClientByClientID(clientID)
		if err != nil {
			return nil, fmt.Errorf("resolve client by clientID: %w", err)
		}

		if client == nil {
			return nil, fmt.Errorf("client not found: %s", clientID)
		}

		clientUUID = client.ID
	}

	return r.keycloakClient.GetUserClientRoles(
		ctx,
		userID,
		clientID,
		clientUUID,
	)
}

func (r *userRepository) AddUserClientRoles(
	ctx context.Context,
	userID string,
	clientID string,
	clientUUID string,
	roles []string,
) error {
	userID = strings.TrimSpace(userID)
	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if clientID == "" {
		return fmt.Errorf("clientID is required")
	}

	if len(roles) == 0 {
		return nil
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	if clientUUID == "" {
		client, err := r.keycloakClient.GetClientByClientID(clientID)
		if err != nil {
			return fmt.Errorf("resolve client by clientID: %w", err)
		}

		if client == nil {
			return fmt.Errorf("client not found: %s", clientID)
		}

		clientUUID = client.ID
	}

	return r.keycloakClient.AssignClientRolesToUser(
		ctx,
		userID,
		clientID,
		clientUUID,
		normalizeRepositoryRoleNames(roles),
	)
}

func (r *userRepository) RemoveUserClientRoles(
	ctx context.Context,
	userID string,
	clientID string,
	clientUUID string,
	roles []string,
) error {
	userID = strings.TrimSpace(userID)
	clientID = strings.TrimSpace(clientID)
	clientUUID = strings.TrimSpace(clientUUID)

	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	if clientID == "" {
		return fmt.Errorf("clientID is required")
	}

	if len(roles) == 0 {
		return nil
	}

	if r == nil || r.keycloakClient == nil {
		return fmt.Errorf("user repository is not configured")
	}

	if clientUUID == "" {
		client, err := r.keycloakClient.GetClientByClientID(clientID)
		if err != nil {
			return fmt.Errorf("resolve client by clientID: %w", err)
		}

		if client == nil {
			return fmt.Errorf("client not found: %s", clientID)
		}

		clientUUID = client.ID
	}

	return r.keycloakClient.RemoveClientRolesFromUser(
		ctx,
		userID,
		clientID,
		clientUUID,
		normalizeRepositoryRoleNames(roles),
	)
}

// ----------------------------------------------------
// HELPERS
// ----------------------------------------------------

func (r *userRepository) upsertLocalUser(
	ctx context.Context,
	id uuid.UUID,
	user *models.User,
) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}

	return r.db.UpsertUser(ctx, db.UpsertUserParams{
		ID:       id,
		Username: strings.TrimSpace(user.Username),
		Email:    strings.TrimSpace(user.Email),
		FirstName: sql.NullString{
			String: strings.TrimSpace(user.FirstName),
			Valid:  strings.TrimSpace(user.FirstName) != "",
		},
		LastName: sql.NullString{
			String: strings.TrimSpace(user.LastName),
			Valid:  strings.TrimSpace(user.LastName) != "",
		},
		Enabled: sql.NullBool{
			Bool:  user.Enabled,
			Valid: true,
		},
	})
}

func pickTime(dbTime sql.NullTime, kcTime *time.Time) *time.Time {
	if dbTime.Valid {
		return &dbTime.Time
	}

	return kcTime
}

func repositoryFullName(firstName string, lastName string) string {
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

func keycloakCreatedAt(user *keycloak.UserInfo) time.Time {
	if user == nil {
		return time.Time{}
	}

	if !user.CreatedAt.IsZero() {
		return user.CreatedAt
	}

	if user.CreatedTimestamp > 0 {
		return time.UnixMilli(user.CreatedTimestamp)
	}

	return time.Time{}
}

func normalizeRepositoryRoleNames(roles []string) []string {
	out := make([]string, 0, len(roles))
	seen := make(map[string]bool, len(roles))

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
