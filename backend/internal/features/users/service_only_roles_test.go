package users

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/keycloak"
	models "github.com/moh-sso-dashboard/internal/model"
)

type clientRoleRepository struct {
	Repository
	current []keycloak.ClientRoleRep
	added   []string
	removed []string
}

func (r *clientRoleRepository) GetUserClientRolesForClient(context.Context, string, string, string) ([]keycloak.ClientRoleRep, error) {
	return r.current, nil
}

func (r *clientRoleRepository) AddUserClientRoles(_ context.Context, _ string, _ string, _ string, roles []string) error {
	r.added = append(r.added, roles...)
	return nil
}

func (r *clientRoleRepository) RemoveUserClientRoles(_ context.Context, _ string, _ string, _ string, roles []string) error {
	r.removed = append(r.removed, roles...)
	return nil
}

func (r *clientRoleRepository) GetUserByID(id uuid.UUID) (*models.User, error) {
	return &models.User{ID: id.String()}, nil
}

type serviceOnlyChecker map[string]bool

func (c serviceOnlyChecker) IsServiceOnlyRole(_ context.Context, clientID, role string) (bool, error) {
	return c[clientID+"/"+role], nil
}

func TestDirectClientRoleGrantsRejectServiceOnlyRoles(t *testing.T) {
	ctx := context.Background()
	user, admin := uuid.New(), uuid.New()
	checker := serviceOnlyChecker{"clims/identity_sync": true}

	repo := &clientRoleRepository{}
	service := NewService(repo, nil)
	service.SetServiceOnlyRoleChecker(checker)

	if err := service.AddUserClientRoles(ctx, user, "clims", "", []string{"clims_access", "identity_sync"}, admin); !errors.Is(err, ErrServiceOnlyRole) {
		t.Fatalf("AddUserClientRoles = %v, want ErrServiceOnlyRole", err)
	}
	if err := service.UpdateUserClientRoles(ctx, user, "clims", "", []string{"identity_sync"}, admin); !errors.Is(err, ErrServiceOnlyRole) {
		t.Fatalf("UpdateUserClientRoles = %v, want ErrServiceOnlyRole", err)
	}
	if len(repo.added) != 0 {
		t.Fatalf("service-only role reached Keycloak: %v", repo.added)
	}

	if err := service.AddUserClientRoles(ctx, user, "clims", "", []string{"clims_access"}, admin); err != nil {
		t.Fatalf("ordinary role rejected: %v", err)
	}
}

func TestServiceOnlyRoleCanStillBeRemovedFromAPerson(t *testing.T) {
	repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "identity_sync"}, {Name: "clims_access"}}}
	service := NewService(repo, nil)
	service.SetServiceOnlyRoleChecker(serviceOnlyChecker{"clims/identity_sync": true})

	if err := service.UpdateUserClientRoles(context.Background(), uuid.New(), "clims", "", []string{"clims_access"}, uuid.New()); err != nil {
		t.Fatalf("removing a service-only role failed: %v", err)
	}
	if len(repo.removed) != 1 || repo.removed[0] != "identity_sync" {
		t.Fatalf("removed = %v, want [identity_sync]", repo.removed)
	}
}
