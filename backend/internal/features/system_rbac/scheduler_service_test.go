package system_rbac

import "testing"

func TestSchedulerServiceRoleHasNoPortalUserAccess(t *testing.T) {
	seed, err := LoadSeedFile("../../../config/system-rbac.seed.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSeed(seed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, system := range seed.Systems {
		if system.ClientID != "data-statistics" {
			continue
		}
		for _, role := range system.Roles {
			if role.Name == "report_scheduler_service" {
				found = true
				if len(role.Permissions) != 0 {
					t.Fatal("service role must not grant user permissions")
				}
			}
		}
		for _, role := range system.AccessRoles {
			if role == "report_scheduler_service" {
				t.Fatal("service role must not grant launcher access")
			}
		}
	}
	if !found {
		t.Fatal("scheduler service role missing")
	}
}
