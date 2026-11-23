package keycloak

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestGetUserRoleMappingsReadsDirectRealmAndClientRoles(t *testing.T) {
	admin := revocationAdminFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/realms/moh/users/user-1/role-mappings":
			_, _ = io.WriteString(w, `{
				"realmMappings":[{"id":"r2","name":"user"},{"id":"r1","name":"admin"}],
				"clientMappings":{
					"clims":{"id":"uuid-clims","client":"clims","mappings":[{"id":"c1","name":"clims_access"}]},
					"poesgo":{"id":"uuid-poes","client":"poesgo","mappings":[]}
				}}`)
		case "/admin/realms/moh/users/missing/role-mappings":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	realm, clients, err := admin.GetUserRoleMappings(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(realm, []string{"admin", "user"}) {
		t.Fatalf("realm roles = %v", realm)
	}
	if !reflect.DeepEqual(clients, map[string][]string{"clims": {"clims_access"}}) {
		t.Fatalf("client roles = %v", clients)
	}
	if _, _, err := admin.GetUserRoleMappings(context.Background(), "missing"); err == nil {
		t.Fatal("404 accepted")
	}
	if _, _, err := admin.GetUserRoleMappings(context.Background(), " "); err == nil {
		t.Fatal("blank user accepted")
	}
}
