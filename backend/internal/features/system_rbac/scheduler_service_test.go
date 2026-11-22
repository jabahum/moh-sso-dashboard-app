package system_rbac

import "testing"

func TestServiceAccountRolesAreServiceOnly(t *testing.T) {
	seed, err := LoadSeedFile("../../../config/system-rbac.seed.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSeed(seed); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"data-statistics": "report_scheduler_service",
		"clims":           "identity_sync",
		"poesgo":          "identity_sync",
	}
	found := map[string]bool{}
	for _, system := range seed.Systems {
		roleName, ok := want[system.ClientID]
		if !ok {
			continue
		}
		for _, role := range system.Roles {
			if role.Name != roleName {
				continue
			}
			found[system.ClientID] = true
			if !role.ServiceOnly {
				t.Errorf("%s/%s must be marked serviceOnly", system.ClientID, roleName)
			}
			if len(role.Permissions) != 0 {
				t.Errorf("%s/%s must not grant user permissions", system.ClientID, roleName)
			}
		}
		for _, role := range system.AccessRoles {
			if role == roleName {
				t.Errorf("%s/%s must not grant launcher access", system.ClientID, roleName)
			}
		}
	}
	for clientID := range want {
		if !found[clientID] {
			t.Errorf("%s service role missing", clientID)
		}
	}
}

func TestValidateSeedRejectsServiceOnlyRoleGrants(t *testing.T) {
	base := func() SeedFile {
		yes := true
		return SeedFile{Systems: []SeedSystem{{
			ClientID:    "clims",
			DisplayName: "CLIMS",
			LaunchURL:   "https://clims.example.org/",
			SystemType:  "external",
			LaunchMode:  "new_tab",
			Enabled:     &yes,
			AccessRoles: []string{"clims_access"},
			Roles: []SeedRole{
				{Name: "clims_access", Permissions: []string{"portal:access"}},
				{Name: "identity_sync", ServiceOnly: true},
			},
		}}}
	}
	if err := ValidateSeed(base()); err != nil {
		t.Fatalf("valid seed rejected: %v", err)
	}
	cases := map[string]func(*SeedFile){
		"permissions": func(s *SeedFile) { s.Systems[0].Roles[1].Permissions = []string{"portal:access"} },
		"access role": func(s *SeedFile) {
			s.Systems[0].AccessRoles = append(s.Systems[0].AccessRoles, "identity_sync")
		},
		"realm role": func(s *SeedFile) {
			s.RealmRoles = []SeedRealmRole{{Name: "user", SystemRoles: map[string][]string{"clims": {"identity_sync"}}}}
		},
		"group": func(s *SeedFile) {
			s.Groups = []SeedGroup{{Name: "lab", Path: "/lab", SystemRoles: map[string][]string{"clims": {"identity_sync"}}}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			seed := base()
			mutate(&seed)
			if err := ValidateSeed(seed); err == nil {
				t.Fatal("expected service-only grant to be rejected")
			}
		})
	}
}
