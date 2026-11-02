package rbac

import (
	"context"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	"testing"
)

type startupRegistryRepository struct {
	Repository
	current System
	writes  int
}

func (r *startupRegistryRepository) GetSystem(context.Context, string) (SystemDetail, error) {
	return SystemDetail{System: r.current}, nil
}
func (r *startupRegistryRepository) UpsertSystem(context.Context, UpsertSystemInput) (System, error) {
	r.writes++
	return System{}, nil
}
func TestStartupDiscoveryPreservesGovernedConfiguration(t *testing.T) {
	for _, status := range []string{"active", "suspended", "retired", "registered", "configured"} {
		t.Run(status, func(t *testing.T) {
			original := System{ClientID: "pilot", DisplayName: "Approved name", OwnerTeam: "Application owner", LaunchURL: "https://approved.example.org", LifecycleStatus: status, Enabled: status == "active"}
			repo := &startupRegistryRepository{current: original}
			result, err := NewService(repo).UpsertSystem(systemrbac.WithStartupDiscovery(context.Background()), UpsertSystemInput{ClientID: "pilot", DisplayName: "Discovered name", LaunchURL: "https://discovered.example.org"})
			if err != nil {
				t.Fatal(err)
			}
			if result != original || repo.writes != 0 {
				t.Fatalf("startup overwrote %s configuration: %+v", status, result)
			}
		})
	}
}
