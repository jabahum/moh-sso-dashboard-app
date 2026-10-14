package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureGroupPathOmitsServerOwnedFields(t *testing.T) {
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/test/protocol/openid-connect/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 300})
		case "/admin/realms/test/groups":
			child := GroupRep{ID: "data", Name: "Data", Path: "/MOH/Data"}
			if created {
				child.SubGroups = []GroupRep{{ID: "scheduler", Name: "Schedulers", Path: "/MOH/Data/Schedulers"}}
			}
			_ = json.NewEncoder(w).Encode([]GroupRep{{ID: "moh", Name: "MOH", Path: "/MOH", SubGroups: []GroupRep{child}}})
		case "/admin/realms/test/groups/data/children":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected method %s", r.Method)
				w.WriteHeader(405)
				return
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			for _, field := range []string{"id", "path", "subGroups"} {
				if _, exists := payload[field]; exists {
					t.Errorf("creation payload contains server-owned field %s", field)
					w.WriteHeader(404)
					return
				}
			}
			if payload["name"] != "Schedulers" {
				t.Errorf("unexpected group name: %v", payload["name"])
			}
			if payload["attributes"] == nil {
				t.Error("group attributes were dropped")
			}
			created = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := NewAdminClient(server.URL, "test", "admin", "secret")
	group, err := client.EnsureGroupPath(context.Background(), "/MOH/Data/Schedulers", "Schedulers", "Own report schedules", map[string][]string{"group_type": {"access"}})
	if err != nil {
		t.Fatal(err)
	}
	if group.ID != "scheduler" || !created {
		t.Fatalf("group was not created: %+v", group)
	}
}
