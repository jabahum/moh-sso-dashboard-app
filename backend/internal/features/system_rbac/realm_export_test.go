package system_rbac

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/moh-sso-dashboard/internal/authz"
)

func sortedRoleMap(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for client, roles := range in {
		roles = append([]string(nil), roles...)
		sort.Strings(roles)
		out[client] = roles
	}
	return out
}

// Migration 53 is what the database applies for the `user` realm role; the
// compiled baseline and the YAML seed are already checked against each other.
func TestMigration53MatchesCompiledUserBaseline(t *testing.T) {
	data, err := os.ReadFile("../../db/migrations/000053_standard_user_client_role_defaults.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(data)
	start := strings.Index(sqlText, "WITH defaults (client_id, role_name) AS (VALUES")
	end := strings.Index(sqlText, "INSERT INTO ihp_realm_role_system_roles")
	if start < 0 || end < start {
		t.Fatal("migration 53 no longer has the defaults VALUES list this test reads")
	}
	got := map[string][]string{}
	for _, m := range regexp.MustCompile(`\('([^']+)',\s*'([^']+)'\)`).FindAllStringSubmatch(sqlText[start:end], -1) {
		got[m[1]] = append(got[m[1]], m[2])
	}
	if want := sortedRoleMap(authz.DefaultUserClientRoles()); !reflect.DeepEqual(sortedRoleMap(got), want) {
		t.Fatalf("migration 53 user defaults = %v, want %v", got, want)
	}
}

type realmExport struct {
	Roles struct {
		Realm []struct {
			Name       string `json:"name"`
			Composite  bool   `json:"composite"`
			Composites *struct {
				Client map[string][]string `json:"client"`
			} `json:"composites"`
		} `json:"realm"`
		Client map[string][]struct {
			Name string `json:"name"`
		} `json:"client"`
	} `json:"roles"`
}

// A realm imported from a shipped export must give the `user` realm role the
// same client-role composites the database grants since migration 53, and every
// composite must name a client role the export defines (Keycloak rejects the
// import otherwise).
func TestRealmExportUserCompositesMatchMigration53(t *testing.T) {
	want := sortedRoleMap(authz.DefaultUserClientRoles())
	for _, name := range []string{"realm-export.json", "realm-export-prod.json"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "keycloak", name))
			if err != nil {
				t.Fatal(err)
			}
			var realm realmExport
			if err := json.Unmarshal(data, &realm); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, role := range realm.Roles.Realm {
				if role.Name != authz.RoleUser {
					continue
				}
				found = true
				if !role.Composite || role.Composites == nil {
					t.Fatal("user realm role has no composites")
				}
				if got := sortedRoleMap(role.Composites.Client); !reflect.DeepEqual(got, want) {
					t.Fatalf("user composites = %v, want %v", got, want)
				}
				for client, roles := range role.Composites.Client {
					defined := map[string]bool{}
					for _, r := range realm.Roles.Client[client] {
						defined[r.Name] = true
					}
					for _, r := range roles {
						if !defined[r] {
							t.Fatalf("user composite %s/%s is not a client role in the export", client, r)
						}
					}
				}
			}
			if !found {
				t.Fatal("export has no user realm role")
			}
		})
	}
}
