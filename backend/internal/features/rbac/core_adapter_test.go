package rbac

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

type coreRegistryRepository struct {
	Repository
	detail    SystemDetail
	err       error
	requested string
}

func (r *coreRegistryRepository) GetSystem(_ context.Context, id string) (SystemDetail, error) {
	r.requested = id
	return r.detail, r.err
}
func TestCoreRegistryPreservesGovernanceAndLaunchMetadata(t *testing.T) {
	system := validApplication()
	system.ID = "registry-id"
	system.Description = "description"
	system.AuthenticatedLaunchURL = "/auth/launch"
	system.Icon = "icon"
	system.Category = "health"
	system.DocumentationURL = "/docs"
	system.Criticality = "high"
	system.Navigation = "navigation"
	system.DisplayInLauncher = true
	system.DisplayInSideNav = true
	system.SortOrder = 7
	repository := &coreRegistryRepository{detail: SystemDetail{System: system, AccessRoles: []string{"viewer"}}}
	registry := NewCoreApplicationRegistry(NewService(repository))
	application, err := registry.GetApplication(context.Background(), " pilot ")
	expected := core.Application{
		ID: system.ID, Description: system.Description, OwnerTeam: system.OwnerTeam, OwnerName: system.OwnerName,
		OwnerEmail: system.OwnerEmail, SupportURL: system.SupportURL, DocumentationURL: system.DocumentationURL,
		Environment: system.Environment, Criticality: system.Criticality, Enabled: false, SortOrder: 7,
		LifecycleStatus: "registered", IntegrationType: "external", ContractVersion: "1",
		Access: core.SystemAccess{ClientID: "pilot", DisplayName: "Pilot", LaunchURL: "https://example.org",
			AuthenticatedLaunchURL: "/auth/launch", Icon: "icon", Category: "health", Navigation: "navigation",
			SystemType: "external", DisplayInLauncher: true, DisplayInSideNav: true, LaunchMode: "new_tab", Roles: []string{"viewer"}},
	}
	if err != nil || !reflect.DeepEqual(application, expected) || repository.requested != "pilot" {
		t.Fatalf("%+v %v", application, err)
	}
	application.Access.Roles[0] = "changed"
	if repository.detail.AccessRoles[0] != "viewer" {
		t.Fatal("registry role alias")
	}
	for _, roles := range [][]string{nil, {}} {
		repository.detail.AccessRoles = roles
		application, err = registry.GetApplication(context.Background(), "pilot")
		if err != nil || !reflect.DeepEqual(application.Access.Roles, roles) {
			t.Fatal("lost nil/empty role shape", application, err)
		}
	}
	repository.err = errors.New("registry unavailable")
	application, err = registry.GetApplication(context.Background(), "pilot")
	if !errors.Is(err, repository.err) || !reflect.DeepEqual(application, core.Application{}) {
		t.Fatal("partial registration leaked", application, err)
	}
	if NewCoreApplicationRegistry(nil) != nil {
		t.Fatal("nil adapter should remain nil")
	}
}
