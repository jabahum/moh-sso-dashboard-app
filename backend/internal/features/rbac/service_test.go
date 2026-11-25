package rbac

import (
	"context"
	"encoding/json"
	"testing"
)

func TestParseImportSeedAcceptsJSONSeed(t *testing.T) {
	payload := json.RawMessage(`{
		"systems": [
			{
				"clientId": "test-system",
				"displayName": "Test System",
				"roles": [{"name": "viewer"}]
			}
		],
		"realmRoles": [{"name": "admin"}],
		"groups": [
			{
				"name": "Data Team",
				"path": "/Data Team",
				"realmRoles": ["user"],
				"systemRoles": {"test-system": ["viewer"]},
				"permissions": ["systems:read"]
			}
		],
		"groupMemberships": [
			{"username": "analyst", "groups": ["/Data Team"]}
		]
	}`)

	seed, err := parseImportSeed(payload)
	if err != nil {
		t.Fatalf("parseImportSeed returned error: %v", err)
	}
	if got := seed.Systems[0].ClientID; got != "test-system" {
		t.Fatalf("expected clientId test-system, got %q", got)
	}
	if got := seed.RealmRoles[0].Name; got != "admin" {
		t.Fatalf("expected realm role admin, got %q", got)
	}
	if got := len(seed.Groups); got != 1 {
		t.Fatalf("expected one group, got %d", got)
	}
	if got := len(seed.GroupMemberships); got != 1 {
		t.Fatalf("expected one group membership, got %d", got)
	}
}

func TestParseImportSeedAcceptsJSONStringYAMLSeed(t *testing.T) {
	yamlPayload := `systems:
  - clientId: test-system
    displayName: Test System
    roles:
      - name: viewer
realmRoles:
  - name: admin
`
	payloadBytes, err := json.Marshal(yamlPayload)
	if err != nil {
		t.Fatalf("marshal YAML string: %v", err)
	}

	seed, err := parseImportSeed(payloadBytes)
	if err != nil {
		t.Fatalf("parseImportSeed returned error: %v", err)
	}
	if got := seed.Systems[0].ClientID; got != "test-system" {
		t.Fatalf("expected clientId test-system, got %q", got)
	}
}

func TestPreviewChangeFlagsHighRiskOperations(t *testing.T) {
	service := NewService(nil)

	preview, err := service.PreviewChange(context.Background(), ChangePreviewRequest{
		Action:       "delete-role",
		ResourceType: "system-role",
		ResourceID:   "role-id",
	})
	if err != nil {
		t.Fatalf("PreviewChange returned error: %v", err)
	}
	if !preview.HighRisk {
		t.Fatal("expected delete-role to be high risk")
	}
	if preview.RiskLevel != "high" {
		t.Fatalf("expected high risk level, got %q", preview.RiskLevel)
	}
}

func TestNormalizeDecisionSupportsPromptAliases(t *testing.T) {
	cases := map[string]string{
		"approve": "approved",
		"reject":  "rejected",
		"cancel":  "cancelled",
		"apply":   "applied",
	}
	for input, want := range cases {
		if got := normalizeDecision(input); got != want {
			t.Fatalf("normalizeDecision(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateSystemLaunchURLAcceptsPortalAndAppPaths(t *testing.T) {
	for _, value := range []string{"/", "/apps/dwh/surveillance", "/admin/home", "/portal/apps/dwh/surveillance", "https://portal.example.org/apps/dwh"} {
		if err := validateSystemLaunchURL(value); err != nil {
			t.Fatalf("validateSystemLaunchURL(%q) returned error: %v", value, err)
		}
	}
	for _, value := range []string{"apps/dwh", "//evil.example/app", "/api/v1/auth/logout", "/ssobackend/api/v1/auth/logout", "/portal/api/v1/users"} {
		if err := validateSystemLaunchURL(value); err == nil {
			t.Fatalf("validateSystemLaunchURL(%q) unexpectedly succeeded", value)
		}
	}
}

func TestValidateOptionalURLAcceptsRootRelativeSupportLinks(t *testing.T) {
	for _, value := range []string{"/support/surveillance", "/docs/systems/surveillance", "https://docs.example.org/surveillance"} {
		if err := validateOptionalURL("supportUrl", value); err != nil {
			t.Fatalf("validateOptionalURL(%q) returned error: %v", value, err)
		}
	}
}

func TestKeycloakClientURLResolvesPortalRelativeLaunchURL(t *testing.T) {
	service := NewService(nil)
	service.SetFrontendBaseURL("http://localhost:3000")

	got := service.keycloakClientURL("/apps/outbreak-management")
	want := "http://localhost:3000/apps/outbreak-management"
	if got != want {
		t.Fatalf("keycloakClientURL returned %q, want %q", got, want)
	}
}

func TestKeycloakClientURLMapsLegacyPortalLaunchURL(t *testing.T) {
	service := NewService(nil)
	service.SetFrontendBaseURL("http://localhost:3000")

	got := service.keycloakClientURL("/portal/apps/outbreak-management")
	want := "http://localhost:3000/apps/outbreak-management"
	if got != want {
		t.Fatalf("keycloakClientURL returned %q, want %q", got, want)
	}
}

func TestKeycloakClientURLResolvesAppPathUnderBasePath(t *testing.T) {
	service := NewService(nil)
	service.SetFrontendBaseURL("http://localhost:3000/sub")

	got := service.keycloakClientURL("/apps/dwh/surveillance")
	want := "http://localhost:3000/sub/apps/dwh/surveillance"
	if got != want {
		t.Fatalf("keycloakClientURL returned %q, want %q", got, want)
	}
}

func TestKeycloakClientURLKeepsAbsoluteLaunchURL(t *testing.T) {
	service := NewService(nil)
	service.SetFrontendBaseURL("http://localhost:3000")

	got := service.keycloakClientURL("https://systems.health.go.ug/outbreak")
	want := "https://systems.health.go.ug/outbreak"
	if got != want {
		t.Fatalf("keycloakClientURL returned %q, want %q", got, want)
	}
}

func TestKeycloakClientURLReturnsEmptyWithoutAbsoluteFrontendBase(t *testing.T) {
	service := NewService(nil)
	service.SetFrontendBaseURL("/")

	if got := service.keycloakClientURL("/apps/outbreak-management"); got != "" {
		t.Fatalf("keycloakClientURL returned %q, want empty string", got)
	}
}

func TestParseRealmExportRequiresExplicitPortalEnrollment(t *testing.T) {
	payload := []byte(`{
		"roles":{"realm":[],"client":{"portal-app":[{"name":"portal-app_access"}],"technical-app":[{"name":"technical-app_access"}]}},
		"clients":[
			{"clientId":"portal-app","name":"Portal App","enabled":true,"attributes":{"portal.system":"true","ui.systemType":"external","ui.launchMode":"new_tab","ui.launchUrl":"https://example.org/app"}},
			{"clientId":"technical-app","name":"Technical App","enabled":true,"attributes":{"ui.icon":"application"}}
		]
	}`)
	discovered, err := parseRealmExport(payload)
	if err != nil {
		t.Fatalf("parseRealmExport returned error: %v", err)
	}
	if len(discovered.Systems) != 1 || discovered.Systems[0].ClientID != "portal-app" {
		t.Fatalf("expected only explicitly enrolled portal app, got %+v", discovered.Systems)
	}
	if discovered.Systems[0].LaunchMode != "new_tab" || discovered.Systems[0].SystemType != "external" {
		t.Fatalf("expected explicit external behavior, got %+v", discovered.Systems[0])
	}
}

func TestParseRealmExportKeepsKnownLegacySystem(t *testing.T) {
	payload := []byte(`{
		"roles":{"realm":[],"client":{"legacy-app":[{"name":"legacy-app_access"}]}},
		"clients":[{"clientId":"legacy-app","name":"Legacy App","enabled":true,"attributes":{"ui.home":"/apps/legacy"}}]
	}`)
	discovered, err := parseRealmExport(payload, map[string]bool{"legacy-app": true})
	if err != nil {
		t.Fatalf("parseRealmExport returned error: %v", err)
	}
	if len(discovered.Systems) != 1 || discovered.Systems[0].ClientID != "legacy-app" {
		t.Fatalf("expected known legacy app, got %+v", discovered.Systems)
	}
}

func TestParseRealmExportPreservesModuleLauncherNavigation(t *testing.T) {
	navigation := `[{"id":"documents","label":"Documents","path":"/apps/dwh/documents","permission":"documents:read","displayInLauncher":true,"displayInSideNav":false}]`
	payload, err := json.Marshal(map[string]any{
		"roles": map[string]any{
			"realm": []any{},
			"client": map[string]any{
				"data-statistics": []map[string]string{{"name": "data-statistics_access"}},
			},
		},
		"clients": []map[string]any{{
			"clientId": "data-statistics",
			"name":     "Data & Statistics",
			"enabled":  true,
			"attributes": map[string]string{
				"portal.system":       "true",
				"ui.systemType":       "platform",
				"ui.launchMode":       "internal",
				"ui.launchUrl":        "/apps/dwh",
				"ui.navigation":       navigation,
				"ui.displayInSideNav": "true",
			},
		}},
	})
	if err != nil {
		t.Fatalf("marshal realm export: %v", err)
	}

	discovered, err := parseRealmExport(payload)
	if err != nil {
		t.Fatalf("parseRealmExport returned error: %v", err)
	}
	if len(discovered.Systems) != 1 {
		t.Fatalf("expected one discovered system, got %d", len(discovered.Systems))
	}
	if discovered.Systems[0].Navigation != navigation {
		t.Fatalf("navigation changed during discovery: %q", discovered.Systems[0].Navigation)
	}
}

func TestSystemConfigurationDifferencesDetectsModuleLauncherMetadataChanges(t *testing.T) {
	discovered := KeycloakDiscoveredSystem{
		SystemType:        "platform",
		DisplayInLauncher: false,
		DisplayInSideNav:  true,
		LaunchMode:        "internal",
		LaunchURL:         "/apps/dwh",
		Navigation:        `[{"id":"documents","label":"Documents","path":"/apps/dwh/documents","displayInLauncher":true}]`,
	}
	current := System{
		SystemType:        "platform",
		DisplayInLauncher: false,
		DisplayInSideNav:  true,
		LaunchMode:        "internal",
		LaunchURL:         "/apps/dwh",
		Navigation:        `[{"id":"documents","label":"Documents","path":"/apps/dwh/documents"}]`,
	}

	differences := systemConfigurationDifferences(discovered, current)
	if !containsString(differences, "navigation") {
		t.Fatalf("expected navigation drift, got %#v", differences)
	}
}

func TestParseRealmExportDiscoversGroupsRolesAndMembers(t *testing.T) {
	payload := []byte(`{
		"roles":{"realm":[{"name":"user"}],"client":{"data-statistics":[{"name":"data-statistics_access"},{"name":"document_viewer"}]}},
		"clients":[{"clientId":"data-statistics","name":"Data & Statistics","enabled":true,"attributes":{"portal.system":"true","portal.accessRoles":"data-statistics_access"}}],
		"groups":[{
			"name":"MOH",
			"subGroups":[{
				"name":"Document Viewers",
				"realmRoles":["user"],
				"clientRoles":{"data-statistics":["document_viewer"]}
			}]
		}],
		"users":[{
			"id":"11111111-1111-1111-1111-111111111111",
			"username":"document.viewer",
			"email":"document.viewer@example.org",
			"groups":["/MOH/Document Viewers"]
		}]
	}`)

	discovered, err := parseRealmExport(payload)
	if err != nil {
		t.Fatalf("parseRealmExport returned error: %v", err)
	}

	var group discoveredGroup
	for _, candidate := range discovered.Groups {
		if candidate.Path == "/MOH/Document Viewers" {
			group = candidate
			break
		}
	}
	if group.Path == "" {
		t.Fatalf("expected /MOH/Document Viewers group, got %#v", discovered.Groups)
	}
	if !containsString(group.RealmRoles, "user") {
		t.Fatalf("expected group realm role, got %#v", group.RealmRoles)
	}
	if !containsString(group.ClientRoles["data-statistics"], "document_viewer") {
		t.Fatalf("expected group client role, got %#v", group.ClientRoles)
	}
	if len(group.Members) != 1 || group.Members[0].Username != "document.viewer" {
		t.Fatalf("expected group member from user group assignment, got %#v", group.Members)
	}
}

func TestApplyRealmExportSyncPersistsGroups(t *testing.T) {
	payload := []byte(`{
		"roles":{"realm":[{"name":"user"}],"client":{"data-statistics":[{"name":"data-statistics_access"},{"name":"document_viewer"}]}},
		"clients":[{"clientId":"data-statistics","name":"Data & Statistics","enabled":true,"attributes":{"portal.system":"true","portal.accessRoles":"data-statistics_access"}}],
		"groups":[{
			"name":"MOH",
			"subGroups":[{
				"name":"Document Viewers",
				"realmRoles":["user"],
				"clientRoles":{"data-statistics":["document_viewer"]}
			}]
		}],
		"users":[{
			"id":"11111111-1111-1111-1111-111111111111",
			"username":"document.viewer",
			"email":"document.viewer@example.org",
			"groups":["/MOH/Document Viewers"]
		}]
	}`)

	repo := newTestRBACRepository()
	service := NewService(repo)

	result, err := service.ApplyRealmExportSync(context.Background(), payload)
	if err != nil {
		t.Fatalf("ApplyRealmExportSync returned error: %v", err)
	}
	if result.GroupsSynced != 2 {
		t.Fatalf("expected root and nested groups synced, got %d", result.GroupsSynced)
	}
	if repo.group.Path != "/MOH/Document Viewers" {
		t.Fatalf("expected final synced group path, got %#v", repo.group)
	}
	if len(repo.groupMembers) != 1 || repo.groupMembers[0].Username != "document.viewer" {
		t.Fatalf("expected synced group member, got %#v", repo.groupMembers)
	}
	if !containsString(repo.groupRealmRoles, "user") {
		t.Fatalf("expected synced group realm role, got %#v", repo.groupRealmRoles)
	}
	if len(repo.groupSystemRoles) != 1 ||
		repo.groupSystemRoles[0].ClientID != "data-statistics" ||
		repo.groupSystemRoles[0].RoleName != "document_viewer" {
		t.Fatalf("expected synced group system role, got %#v", repo.groupSystemRoles)
	}
}

func TestRealmDiscoveryPreservesOnboardingProvenance(t *testing.T) {
	for _, fixture := range []struct {
		profile string
		managed bool
	}{{"", false}, {"public-web", true}} {
		payload := []byte(`{"roles":{"client":{}},"clients":[{"clientId":"portal-app","enabled":true,"attributes":{"portal.system":"true","portal.onboarding.profile":"` + fixture.profile + `"}}]}`)
		discovered, err := parseRealmExport(payload)
		if err != nil || len(discovered.Systems) != 1 || discovered.Systems[0].LifecycleManaged != fixture.managed {
			t.Fatalf("profile %q: %+v %v", fixture.profile, discovered.Systems, err)
		}
	}
}
