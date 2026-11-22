package users

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

type removalCall struct {
	userID, clientID, actor string
	roles                   []string
}

type recordingRemovalHook struct {
	calls []removalCall
	err   error
}

func (h *recordingRemovalHook) ClientRolesRemoved(_ context.Context, userID, clientID string, roles []string, actorID string) error {
	sorted := append([]string(nil), roles...)
	sort.Strings(sorted)
	h.calls = append(h.calls, removalCall{userID: userID, clientID: clientID, actor: actorID, roles: sorted})
	return h.err
}

func TestAccessRoleRemovalHookRunsAfterRemovalOnly(t *testing.T) {
	ctx := context.Background()
	user, admin := uuid.New(), uuid.New()
	hook := &recordingRemovalHook{}

	// UpdateUserClientRoles (the portal RBAC UpdateUserAccess path) removing the access role.
	repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_access"}, {Name: "clims_viewer"}}}
	service := NewService(repo, nil)
	service.SetAccessRoleRemovalHook(hook)
	if err := service.UpdateUserClientRoles(ctx, user, "clims", "", []string{"clims_viewer", "clims_editor"}, admin); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 1 {
		t.Fatalf("hook calls = %+v", hook.calls)
	}
	call := hook.calls[0]
	if call.userID != user.String() || call.clientID != "clims" || call.actor != admin.String() || strings.Join(call.roles, ",") != "clims_access" {
		t.Fatalf("hook call = %+v", call)
	}
	if strings.Join(repo.removed, ",") != "clims_access" {
		t.Fatalf("removed = %v", repo.removed)
	}

	// Only additions: no hook call.
	hook.calls = nil
	repo = &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_viewer"}}}
	service = NewService(repo, nil)
	service.SetAccessRoleRemovalHook(hook)
	if err := service.UpdateUserClientRoles(ctx, user, "clims", "", []string{"clims_viewer", "clims_access"}, admin); err != nil {
		t.Fatal(err)
	}
	if err := service.AddUserClientRoles(ctx, user, "clims", "", []string{"clims_editor"}, admin); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 0 {
		t.Fatalf("hook called without a removal: %+v", hook.calls)
	}

	// RemoveUserClientRoles (users API DELETE /:id/clients/:clientID/roles).
	if err := service.RemoveUserClientRoles(ctx, user, "poesgo", "", []string{"poesgo_access"}, admin); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 1 || hook.calls[0].clientID != "poesgo" || strings.Join(hook.calls[0].roles, ",") != "poesgo_access" {
		t.Fatalf("hook calls = %+v", hook.calls)
	}
}

func TestAccessRoleRemovalSucceedsWhenRevocationFails(t *testing.T) {
	ctx := context.Background()
	hook := &recordingRemovalHook{err: errors.New("identity-sync revocation incomplete")}
	repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_access"}}}
	service := NewService(repo, nil)
	service.SetAccessRoleRemovalHook(hook)
	if err := service.UpdateUserClientRoles(ctx, uuid.New(), "clims", "", nil, uuid.New()); err != nil {
		t.Fatalf("role removal failed because revocation failed: %v", err)
	}
	if err := service.RemoveUserClientRoles(ctx, uuid.New(), "clims", "", []string{"clims_access"}, uuid.New()); err != nil {
		t.Fatalf("role removal failed because revocation failed: %v", err)
	}
	if len(hook.calls) != 2 || len(repo.removed) != 2 {
		t.Fatalf("calls = %+v, removed = %v", hook.calls, repo.removed)
	}
}

func TestNoAccessRoleRemovalHookIsSafe(t *testing.T) {
	repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_access"}}}
	service := NewService(repo, nil)
	service.SetAccessRoleRemovalHook(nil)
	if err := service.UpdateUserClientRoles(context.Background(), uuid.New(), "clims", "", nil, uuid.New()); err != nil {
		t.Fatal(err)
	}
}
