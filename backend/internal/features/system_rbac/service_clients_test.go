package system_rbac

import (
	"strings"
	"testing"
)

func TestConfiguredSeedDeclaresTheIdentitySyncServiceClients(t *testing.T) {
	seed, err := LoadSeedFile("../../../config/system-rbac.seed.yaml")
	if err != nil {
		t.Fatalf("LoadSeedFile: %v", err)
	}
	if err := ValidateSeed(seed); err != nil {
		t.Fatalf("ValidateSeed: %v", err)
	}
	want := map[string]string{"clims-identity-sync": "clims", "poesgo-identity-sync": "poesgo"}
	got := map[string]string{}
	for _, client := range seed.ServiceClients {
		got[client.ClientID] = client.Audience
		if len(client.Roles) != 1 || client.Roles[0].System != client.Audience || client.Roles[0].Role != "identity_sync" {
			t.Errorf("%s roles = %+v, want %s/identity_sync", client.ClientID, client.Roles, client.Audience)
		}
	}
	for clientID, audience := range want {
		if got[clientID] != audience {
			t.Errorf("service client %s audience = %q, want %q", clientID, got[clientID], audience)
		}
	}
}

func serviceClientTestSeed(clients ...SeedServiceClient) SeedFile {
	return SeedFile{
		Systems: []SeedSystem{{
			ClientID:    "clims",
			DisplayName: "CLIMS",
			LaunchURL:   "https://clims.example.org/api/v1/auth/sso/start",
			SystemType:  "external",
			LaunchMode:  "new_tab",
			AccessRoles: []string{"clims_access"},
			Roles: []SeedRole{
				{Name: "clims_access", Permissions: []string{"portal:access"}},
				{Name: "identity_sync", ServiceOnly: true},
			},
		}},
		ServiceClients: clients,
	}
}

func syncClient() SeedServiceClient {
	return SeedServiceClient{
		ClientID: "clims-identity-sync",
		Audience: "clims",
		Roles:    []SeedServiceClientRole{{System: "clims", Role: "identity_sync"}},
	}
}

func TestValidateSeedAcceptsAServiceClient(t *testing.T) {
	if err := ValidateSeed(serviceClientTestSeed(syncClient())); err != nil {
		t.Fatalf("ValidateSeed: %v", err)
	}
}

func TestValidateSeedRejectsUnsafeServiceClients(t *testing.T) {
	cases := map[string]struct {
		mutate func(*SeedFile)
		want   string
	}{
		"missing clientId":   {func(s *SeedFile) { s.ServiceClients[0].ClientID = "" }, "clientId is required"},
		"uppercase clientId": {func(s *SeedFile) { s.ServiceClients[0].ClientID = "CLIMS-Identity-Sync" }, "lowercase"},
		"duplicate":          {func(s *SeedFile) { s.ServiceClients = append(s.ServiceClients, syncClient()) }, "duplicate service client"},
		"also a system": {func(s *SeedFile) {
			s.ServiceClients[0].ClientID = "clims"
		}, "is also a system"},
		"missing audience": {func(s *SeedFile) { s.ServiceClients[0].Audience = "" }, "audience is required"},
		"unknown audience": {func(s *SeedFile) { s.ServiceClients[0].Audience = "poesgo" }, "not a seeded system"},
		"no roles":         {func(s *SeedFile) { s.ServiceClients[0].Roles = nil }, "at least one role"},
		"unknown role":     {func(s *SeedFile) { s.ServiceClients[0].Roles[0].Role = "admin" }, "unknown role"},
		"people role":      {func(s *SeedFile) { s.ServiceClients[0].Roles[0].Role = "clims_access" }, "not service-only"},
		"role listed twice": {func(s *SeedFile) {
			s.ServiceClients[0].Roles = append(s.ServiceClients[0].Roles, s.ServiceClients[0].Roles[0])
		}, "twice"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			seed := serviceClientTestSeed(syncClient())
			tc.mutate(&seed)
			err := ValidateSeed(seed)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
