package system_rbac

import (
	"testing"

	"github.com/moh-sso-dashboard/internal/authz"
)

func TestDefaultSeedGivesUserDefaultSystems(t *testing.T) {
	seed := DefaultSeed()
	if err := ValidateSeed(seed); err != nil {
		t.Fatalf("default seed is invalid: %v", err)
	}

	for _, realmRole := range seed.RealmRoles {
		if realmRole.Name != authz.RoleUser {
			continue
		}
		expected := map[string]string{
			authz.SystemDataStatistics: authz.DataStatisticsAccess,
			authz.SystemUtilities:      authz.UtilitiesAccess,
			authz.SystemSettings:       authz.SettingsAccess,
		}
		for clientID, roleName := range expected {
			if !contains(realmRole.SystemRoles[clientID], roleName) {
				t.Fatalf("user defaults missing %s/%s: %+v", clientID, roleName, realmRole.SystemRoles)
			}
		}
		return
	}
	t.Fatal("default seed does not define user realm role")
}

func TestDefaultSeedGivesManagerSurveillanceDataStatisticsRole(t *testing.T) {
	seed := DefaultSeed()
	if err := ValidateSeed(seed); err != nil {
		t.Fatalf("default seed is invalid: %v", err)
	}

	for _, realmRole := range seed.RealmRoles {
		if realmRole.Name != authz.RoleManager {
			continue
		}
		if !contains(realmRole.SystemRoles[authz.SystemDataStatistics], authz.SurveillanceManager) {
			t.Fatalf("manager defaults missing %s/%s: %+v", authz.SystemDataStatistics, authz.SurveillanceManager, realmRole.SystemRoles)
		}
		return
	}
	t.Fatal("default seed does not define manager realm role")
}

func TestDefaultSeedIncludesDataStatisticsModuleLauncherMetadata(t *testing.T) {
	seed := DefaultSeed()

	for _, system := range seed.Systems {
		if system.ClientID != authz.SystemDataStatistics {
			continue
		}

		items, err := parseNavigation(system.Navigation)
		if err != nil {
			t.Fatalf("parse data statistics navigation: %v", err)
		}
		launcherCount := 0
		schedulerFound := false
		for _, item := range items {
			if item.ID == "report-scheduler" {
				schedulerFound = true
				if item.Path != "/apps/dwh/report-scheduler" || item.Permission != string(authz.PermissionReportSchedulerRead) || item.DisplayInLauncher == nil || !*item.DisplayInLauncher || item.DisplayInSideNav == nil || !*item.DisplayInSideNav {
					t.Fatalf("invalid report scheduler navigation: %+v", item)
				}
			}
			if item.DisplayInLauncher != nil && *item.DisplayInLauncher {
				launcherCount++
			}
		}
		if launcherCount != 6 {
			t.Fatalf("expected six module launcher entries, got %d", launcherCount)
		}
		if !schedulerFound {
			t.Fatal("missing report scheduler module entry")
		}
		return
	}

	t.Fatal("default seed does not define data-statistics")
}

func TestValidateSeedRejectsUnknownDefaultSystemRole(t *testing.T) {
	seed := DefaultSeed()
	for index := range seed.RealmRoles {
		if seed.RealmRoles[index].Name == authz.RoleUser {
			seed.RealmRoles[index].SystemRoles[authz.SystemDataStatistics] = []string{"missing-role"}
		}
	}
	if err := ValidateSeed(seed); err == nil {
		t.Fatal("expected unknown default system role validation error")
	}
}

func TestValidateSeedAcceptsGroupsAndMemberships(t *testing.T) {
	seed := DefaultSeed()
	seed.Groups = []SeedGroup{
		{
			Name:        "Data Team",
			Path:        "/Data Team",
			RealmRoles:  []string{authz.RoleUser},
			SystemRoles: map[string][]string{authz.SystemDataStatistics: {authz.DataStatisticsAccess}},
			Permissions: []string{string(authz.PermissionSystemsRead)},
			Subgroups: []SeedGroup{
				{
					Name:        "Analysts",
					Permissions: []string{string(authz.PermissionReportBrowserRead)},
				},
			},
		},
	}
	seed.GroupMemberships = []SeedGroupMembership{
		{Username: "analyst", Groups: []string{"/Data Team", "/Data Team/Analysts"}},
	}

	if err := ValidateSeed(seed); err != nil {
		t.Fatalf("expected valid group seed: %v", err)
	}
}

func TestValidateSeedRejectsUnknownGroupSystemRole(t *testing.T) {
	seed := DefaultSeed()
	seed.Groups = []SeedGroup{
		{
			Name:        "Data Team",
			Path:        "/Data Team",
			SystemRoles: map[string][]string{authz.SystemDataStatistics: {"missing-role"}},
		},
	}

	if err := ValidateSeed(seed); err == nil {
		t.Fatal("expected unknown group system role validation error")
	}
}

func TestValidateSeedRejectsUnknownMembershipGroup(t *testing.T) {
	seed := DefaultSeed()
	seed.GroupMemberships = []SeedGroupMembership{
		{Username: "analyst", Groups: []string{"/Missing Group"}},
	}

	if err := ValidateSeed(seed); err == nil {
		t.Fatal("expected unknown membership group validation error")
	}
}

func TestValidateSystemBehaviorSupportsAllLaunchModes(t *testing.T) {
	trueValue := true
	falseValue := false
	cases := []SeedSystem{
		{
			SystemType: "platform", LaunchMode: "internal", LaunchURL: "/portal/apps/dwh",
			DisplayInLauncher: &trueValue, DisplayInSideNav: &trueValue,
			Navigation: `[{"id":"dashboards","label":"Dashboards","path":"/apps/dwh/dashboards"}]`,
		},
		{
			SystemType: "external", LaunchMode: "new_tab", LaunchURL: "https://example.org/health",
			DisplayInLauncher: &trueValue, DisplayInSideNav: &falseValue,
		},
		{
			SystemType: "external", LaunchMode: "same_tab", LaunchURL: "https://example.org/support",
			DisplayInLauncher: &trueValue, DisplayInSideNav: &falseValue,
		},
	}
	for _, system := range cases {
		if err := ValidateSystemBehavior(system); err != nil {
			t.Fatalf("expected valid behavior %+v: %v", system, err)
		}
	}
}

func TestValidateSystemBehaviorRejectsUnsafeOrInconsistentConfiguration(t *testing.T) {
	trueValue := true
	cases := []SeedSystem{
		{SystemType: "external", LaunchMode: "new_tab", LaunchURL: "javascript://example.org"},
		{SystemType: "external", LaunchMode: "same_tab", LaunchURL: "https://user:pass@example.org"},
		{SystemType: "platform", LaunchMode: "new_tab", LaunchURL: "/portal/apps/dwh"},
		{SystemType: "external", LaunchMode: "new_tab", LaunchURL: "https://example.org", DisplayInSideNav: &trueValue},
		{SystemType: "platform", LaunchMode: "internal", LaunchURL: "/portal/apps/dwh", DisplayInSideNav: &trueValue, Navigation: "[]"},
	}
	for _, system := range cases {
		if err := ValidateSystemBehavior(system); err == nil {
			t.Fatalf("expected invalid behavior %+v", system)
		}
	}
}

func TestValidateSystemBehaviorAcceptsLegacyAndLauncherNavigation(t *testing.T) {
	trueValue := true
	cases := []string{
		`[{"id":"documents","label":"Documents","path":"/apps/dwh/documents","permission":"documents:read"}]`,
		`[{"id":"data","label":"Data","children":[{"id":"validation","label":"Data Validation","path":"/apps/dwh/data-validation","requiredPermissions":["data_quality:read"],"icon":"action","order":20,"displayInLauncher":true}]}]`,
		`[{"id":"support","label":"Support","path":"https://example.org/support","launchMode":"new_tab","displayInLauncher":true,"displayInSideNav":false}]`,
	}
	for _, navigation := range cases {
		system := SeedSystem{
			SystemType:       "platform",
			LaunchMode:       "internal",
			LaunchURL:        "/portal/apps/dwh",
			DisplayInSideNav: &trueValue,
			Navigation:       navigation,
		}
		if err := ValidateSystemBehavior(system); err != nil {
			t.Fatalf("expected valid navigation %s: %v", navigation, err)
		}
	}
}

func TestValidateSystemBehaviorRejectsInvalidNavigationMetadata(t *testing.T) {
	cases := []string{
		`[{"id":"","label":"Missing ID","path":"/apps/dwh"}]`,
		`[{"id":"missing-label","label":"","path":"/apps/dwh"}]`,
		`[{"id":"duplicate","label":"One","path":"/apps/one"},{"id":"duplicate","label":"Two","path":"/apps/two"}]`,
		`[{"id":"launcher","label":"Launcher","displayInLauncher":true}]`,
		`[{"id":"unsafe","label":"Unsafe","path":"javascript:alert(1)","displayInLauncher":true}]`,
		`[{"id":"external","label":"External","path":"https://example.org","launchMode":"internal","displayInLauncher":true}]`,
		`[{"id":"mode","label":"Mode","path":"/apps/dwh","launchMode":"popup"}]`,
		`[{"id":"permissions","label":"Permissions","path":"/apps/dwh","requiredAnyPermissions":[""]}]`,
	}
	for _, navigation := range cases {
		system := SeedSystem{
			SystemType: "platform",
			LaunchMode: "internal",
			LaunchURL:  "/portal/apps/dwh",
			Navigation: navigation,
		}
		if err := ValidateSystemBehavior(system); err == nil {
			t.Fatalf("expected invalid navigation %s", navigation)
		}
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
