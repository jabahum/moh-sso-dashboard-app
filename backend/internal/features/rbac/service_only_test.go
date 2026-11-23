package rbac

import (
	"context"
	"errors"
	"testing"
)

// serviceOnlyRepository adds a service-only role to the shared test repository and
// records any write that would grant it to people.
type serviceOnlyRepository struct {
	*testRBACRepository
	writes []string
}

func (r *serviceOnlyRepository) roles(clientID string) []SystemRole {
	return []SystemRole{
		{ID: "role-document-viewer", ClientID: clientID, Name: "document_viewer", Enabled: true,
			Permissions: []Permission{{Key: "documents:read", Status: "active"}}},
		{ID: "role-identity-sync", ClientID: clientID, Name: "identity_sync", Enabled: true, ServiceOnly: true},
	}
}

func (r *serviceOnlyRepository) GetSystem(ctx context.Context, clientID string) (SystemDetail, error) {
	detail, err := r.testRBACRepository.GetSystem(ctx, clientID)
	detail.Roles = r.roles(clientID)
	return detail, err
}

func (r *serviceOnlyRepository) ListSystemRoles(_ context.Context, clientID string) ([]SystemRole, error) {
	return r.roles(clientID), nil
}

func (r *serviceOnlyRepository) GetSystemRole(_ context.Context, roleID string) (SystemRole, error) {
	for _, role := range r.roles("data-statistics") {
		if role.ID == roleID {
			return role, nil
		}
	}
	return SystemRole{}, errors.New("not found")
}

func (r *serviceOnlyRepository) AssignSystemRolePermission(_ context.Context, roleID, permission string) error {
	r.writes = append(r.writes, "permission:"+roleID+":"+permission)
	return nil
}

func (r *serviceOnlyRepository) AssignGroupSystemRole(_ context.Context, _ string, clientID, roleName string) error {
	r.writes = append(r.writes, "group:"+clientID+":"+roleName)
	return nil
}

func (r *serviceOnlyRepository) AddSystemAccessRole(_ context.Context, clientID, roleName string) error {
	r.writes = append(r.writes, "access:"+clientID+":"+roleName)
	return nil
}

func newServiceOnlyTestService() (*Service, *serviceOnlyRepository) {
	repo := &serviceOnlyRepository{testRBACRepository: newTestRBACRepository()}
	return NewService(repo, &testUserLookup{}), repo
}

func TestAssignableUserAccessHidesServiceOnlyRoles(t *testing.T) {
	service, _ := newServiceOnlyTestService()
	assignable, err := service.ListAssignableUserAccess(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, system := range assignable.Systems {
		for _, role := range system.Roles {
			if role.Name == "identity_sync" {
				t.Fatalf("service-only role listed as assignable for %s", system.ClientID)
			}
		}
	}
	err = validateAssignableUserAccess(assignable, nil, map[string][]string{"data-statistics": {"identity_sync"}}, nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("direct assignment of a service-only role = %v, want invalid input", err)
	}
}

func TestServiceOnlyRoleCannotBeGrantedToPeople(t *testing.T) {
	ctx := context.Background()
	service, repo := newServiceOnlyTestService()

	if err := service.AssignGroupSystemRole(ctx, "group-data", GroupSystemRoleInput{ClientID: "data-statistics", RoleName: "identity_sync"}); !errors.Is(err, ErrServiceOnlyRole) {
		t.Errorf("group grant = %v, want ErrServiceOnlyRole", err)
	}
	if err := service.AddSystemAccessRole(ctx, "data-statistics", "IDENTITY_SYNC"); !errors.Is(err, ErrServiceOnlyRole) {
		t.Errorf("access role = %v, want ErrServiceOnlyRole", err)
	}
	if err := service.AssignSystemRolePermission(ctx, "role-identity-sync", "documents:read"); !errors.Is(err, ErrServiceOnlyRole) {
		t.Errorf("permission = %v, want ErrServiceOnlyRole", err)
	}
	if len(repo.writes) != 0 {
		t.Fatalf("service-only role was written: %v", repo.writes)
	}

	// Ordinary roles are unaffected.
	if err := service.AssignGroupSystemRole(ctx, "group-data", GroupSystemRoleInput{ClientID: "data-statistics", RoleName: "document_viewer"}); err != nil {
		t.Errorf("ordinary group grant: %v", err)
	}
	if err := service.AddSystemAccessRole(ctx, "data-statistics", "document_viewer"); err != nil {
		t.Errorf("ordinary access role: %v", err)
	}
}

func TestMarkingRoleServiceOnlyRequiresNoPermissions(t *testing.T) {
	service, _ := newServiceOnlyTestService()
	yes := true
	_, err := service.UpdateSystemRole(context.Background(), "role-document-viewer", RoleInput{Name: "document_viewer", ServiceOnly: &yes})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateSystemRole = %v, want invalid input", err)
	}
}

func TestIsServiceOnlyRole(t *testing.T) {
	service, _ := newServiceOnlyTestService()
	for role, want := range map[string]bool{"identity_sync": true, "document_viewer": false, "missing": false} {
		got, err := service.IsServiceOnlyRole(context.Background(), "data-statistics", role)
		if err != nil || got != want {
			t.Errorf("IsServiceOnlyRole(%q) = %v, %v; want %v", role, got, err, want)
		}
	}
}
