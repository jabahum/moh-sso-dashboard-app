package rbac

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

type fakeServiceClientSource struct {
	specs   []keycloak.ServiceClientSpec
	results map[string]keycloak.ServiceClientResult
	errs    map[string]error
}

func (f *fakeServiceClientSource) EnsureServiceClient(_ context.Context, spec keycloak.ServiceClientSpec) (keycloak.ServiceClientResult, error) {
	f.specs = append(f.specs, spec)
	result := f.results[spec.ClientID]
	result.ClientID = spec.ClientID
	return result, f.errs[spec.ClientID]
}

func seedServiceClients() []systemrbac.SeedServiceClient {
	return []systemrbac.SeedServiceClient{
		{
			ClientID: "clims-identity-sync", DisplayName: "clims identity sync", Audience: "CLIMS",
			Roles: []systemrbac.SeedServiceClientRole{{System: "clims", Role: "Identity_Sync"}},
		},
		{
			ClientID: "poesgo-identity-sync", Audience: "poesgo",
			Roles: []systemrbac.SeedServiceClientRole{{System: "poesgo", Role: "identity_sync"}},
		},
	}
}

func TestPushServiceClientsToKeycloakMapsTheSeedAndAudits(t *testing.T) {
	repo := newTestRBACRepository()
	service := NewService(repo, nil)
	source := &fakeServiceClientSource{results: map[string]keycloak.ServiceClientResult{
		"clims-identity-sync": {Created: true, RolesAssigned: []string{"clims/identity_sync"}},
	}}

	result, err := service.PushServiceClientsToKeycloak(context.Background(), source, seedServiceClients())
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(result.Changed) != 1 || result.Changed[0].ClientID != "clims-identity-sync" || result.Unchanged != 1 {
		t.Fatalf("result = %+v", result)
	}
	spec := source.specs[0]
	if spec.Audience != "clims" || spec.Name != "clims identity sync" ||
		len(spec.Roles) != 1 || spec.Roles[0] != (keycloak.ServiceClientRole{ClientID: "clims", Role: "identity_sync"}) {
		t.Fatalf("spec = %+v, want normalised audience and role", spec)
	}
	if len(repo.auditEvents) != 1 || repo.auditEvents[0].Action != "rbac.keycloak_service_clients_pushed" {
		t.Fatalf("audit = %+v", repo.auditEvents)
	}
}

func TestPushServiceClientsToKeycloakSkipsConflictingClientsWithAWarning(t *testing.T) {
	repo := newTestRBACRepository()
	service := NewService(repo, nil)
	source := &fakeServiceClientSource{errs: map[string]error{
		"clims-identity-sync": fmt.Errorf("%w: clims-identity-sync", keycloak.ErrNotServiceClient),
	}}

	result, err := service.PushServiceClientsToKeycloak(context.Background(), source, seedServiceClients())
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(source.specs) != 2 {
		t.Fatalf("push stopped after the conflict; specs = %+v", source.specs)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "clims-identity-sync") || result.Unchanged != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(repo.auditEvents) != 1 {
		t.Fatalf("a warning must be audited; audit = %+v", repo.auditEvents)
	}
}

func TestPushServiceClientsToKeycloakContinuesPastAFailedClient(t *testing.T) {
	repo := newTestRBACRepository()
	service := NewService(repo, nil)
	missingRole := errors.New(`get client role failed [404]: {"error":"Could not find role"}`)
	source := &fakeServiceClientSource{
		errs: map[string]error{"clims-identity-sync": missingRole},
		results: map[string]keycloak.ServiceClientResult{
			// clims-identity-sync was created before its role lookup failed.
			"clims-identity-sync":  {Created: true},
			"poesgo-identity-sync": {Created: true},
		},
	}
	result, err := service.PushServiceClientsToKeycloak(context.Background(), source, seedServiceClients())
	if !errors.Is(err, missingRole) || !strings.Contains(err.Error(), "clims-identity-sync") {
		t.Fatalf("err = %v, want the clims failure", err)
	}
	if len(source.specs) != 2 {
		t.Fatalf("pushed %d clients, want both", len(source.specs))
	}
	if len(result.Failed) != 1 || !strings.Contains(result.Failed[0], "clims-identity-sync") {
		t.Fatalf("failed = %v", result.Failed)
	}
	if len(result.Changed) != 2 || result.Changed[0].ClientID != "clims-identity-sync" || result.Changed[1].ClientID != "poesgo-identity-sync" {
		t.Fatalf("changed = %+v", result.Changed)
	}
	if len(repo.auditEvents) != 1 || !strings.Contains(string(repo.auditEvents[0].Details), "Could not find role") {
		t.Fatalf("the failure must be audited; audit = %+v", repo.auditEvents)
	}
}

func TestPushServiceClientsToKeycloakReportsEveryFailedClient(t *testing.T) {
	service := NewService(newTestRBACRepository(), nil)
	source := &fakeServiceClientSource{errs: map[string]error{
		"clims-identity-sync":  errors.New("HTTP 500"),
		"poesgo-identity-sync": errors.New("HTTP 502"),
	}}
	result, err := service.PushServiceClientsToKeycloak(context.Background(), source, seedServiceClients())
	if err == nil || !strings.Contains(err.Error(), "clims-identity-sync") || !strings.Contains(err.Error(), "poesgo-identity-sync") {
		t.Fatalf("err = %v, want both clients", err)
	}
	if len(result.Failed) != 2 || len(result.Changed) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestPushServiceClientsToKeycloakIsQuietWhenNothingChanged(t *testing.T) {
	repo := newTestRBACRepository()
	service := NewService(repo, nil)
	result, err := service.PushServiceClientsToKeycloak(context.Background(), &fakeServiceClientSource{}, seedServiceClients())
	if err != nil || result.Unchanged != 2 || len(result.Changed) != 0 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if len(repo.auditEvents) != 0 {
		t.Fatalf("an unchanged run must not be audited; audit = %+v", repo.auditEvents)
	}
	if _, err := service.PushServiceClientsToKeycloak(context.Background(), nil, nil); err != nil {
		t.Fatalf("no clients and no source: %v", err)
	}
}
