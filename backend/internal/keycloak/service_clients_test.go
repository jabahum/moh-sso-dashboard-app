package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeServiceClientRealm is a minimal in-memory Keycloak admin API for one
// sync client (clims-identity-sync) and its role client (clims).
type fakeServiceClientRealm struct {
	t              *testing.T
	syncClient     map[string]any // nil until created
	mappers        []protocolMapperRep
	saRoles        []string
	scopeRoles     []string
	writes         []string
	createdPayload map[string]any
}

func (f *fakeServiceClientRealm) handler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/realms/moh/")
	key := r.Method + " " + path
	if r.Method != http.MethodGet {
		f.writes = append(f.writes, key)
	}
	switch {
	case key == "GET clients" && r.URL.Query().Get("clientId") == "clims-identity-sync":
		if f.syncClient == nil {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{f.syncClient})
	case key == "GET clients" && r.URL.Query().Get("clientId") == "clims":
		_, _ = io.WriteString(w, `[{"id":"uuid-clims","clientId":"clims"}]`)
	case key == "POST clients":
		_ = json.NewDecoder(r.Body).Decode(&f.createdPayload)
		f.syncClient = map[string]any{
			"id": "uuid-sync", "clientId": "clims-identity-sync",
			"serviceAccountsEnabled": true,
		}
		w.Header().Set("Location", "http://kc/admin/realms/moh/clients/uuid-sync")
		w.WriteHeader(http.StatusCreated)
	case key == "GET clients/uuid-sync/protocol-mappers/models":
		_ = json.NewEncoder(w).Encode(f.mappers)
	case key == "POST clients/uuid-sync/protocol-mappers/models":
		var mapper protocolMapperRep
		_ = json.NewDecoder(r.Body).Decode(&mapper)
		f.mappers = append(f.mappers, mapper)
		w.WriteHeader(http.StatusCreated)
	case key == "GET clients/uuid-sync/service-account-user":
		_, _ = io.WriteString(w, `{"id":"sa-user"}`)
	case key == "GET clients/uuid-clims/roles/identity_sync":
		_, _ = io.WriteString(w, `{"id":"role-sync","name":"identity_sync"}`)
	case key == "GET users/sa-user/role-mappings/clients/uuid-clims":
		_ = json.NewEncoder(w).Encode(roleReps(f.saRoles))
	case key == "POST users/sa-user/role-mappings/clients/uuid-clims":
		f.saRoles = append(f.saRoles, "identity_sync")
		w.WriteHeader(http.StatusNoContent)
	case key == "GET clients/uuid-sync/scope-mappings/clients/uuid-clims":
		_ = json.NewEncoder(w).Encode(roleReps(f.scopeRoles))
	case key == "POST clients/uuid-sync/scope-mappings/clients/uuid-clims":
		f.scopeRoles = append(f.scopeRoles, "identity_sync")
		w.WriteHeader(http.StatusNoContent)
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusTeapot)
	}
}

func roleReps(names []string) []ClientRoleRep {
	out := []ClientRoleRep{}
	for _, name := range names {
		out = append(out, ClientRoleRep{ID: "role-" + name, Name: name})
	}
	return out
}

func climsSyncSpec() ServiceClientSpec {
	return ServiceClientSpec{
		ClientID:    "clims-identity-sync",
		Name:        "clims identity sync",
		Description: "MoH SSO Dashboard reconciler",
		Audience:    "clims",
		Roles:       []ServiceClientRole{{ClientID: "clims", Role: "identity_sync"}},
	}
}

func TestEnsureServiceClientCreatesTheRealmExportShapeAndIsIdempotent(t *testing.T) {
	realm := &fakeServiceClientRealm{t: t}
	admin := revocationAdminFixture(t, realm.handler)
	ctx := context.Background()

	result, err := admin.EnsureServiceClient(ctx, climsSyncSpec())
	if err != nil {
		t.Fatalf("EnsureServiceClient: %v", err)
	}
	if !result.Created || result.AudienceMapperAdded ||
		len(result.RolesAssigned) != 1 || len(result.ScopeMappingsAdded) != 1 {
		t.Fatalf("first run result = %+v", result)
	}

	p := realm.createdPayload
	want := map[string]any{
		"clientId": "clims-identity-sync", "publicClient": false, "clientAuthenticatorType": "client-secret",
		"serviceAccountsEnabled": true, "standardFlowEnabled": false, "implicitFlowEnabled": false,
		"directAccessGrantsEnabled": false, "fullScopeAllowed": false, "enabled": true,
	}
	for field, value := range want {
		if p[field] != value {
			t.Errorf("created %s = %v, want %v", field, p[field], value)
		}
	}
	if _, ok := p["secret"]; ok {
		t.Error("created client carries a secret; Keycloak must generate it")
	}
	attrs, _ := p["attributes"].(map[string]any)
	if attrs["portal.system"] != "false" || attrs["ui.displayInLauncher"] != "false" {
		t.Errorf("created attributes = %v, want the client kept out of the portal", attrs)
	}
	scopes, _ := p["defaultClientScopes"].([]any)
	if len(scopes) != 1 || scopes[0] != "roles" {
		t.Errorf("defaultClientScopes = %v, want [roles]", scopes)
	}
	mappers, _ := p["protocolMappers"].([]any)
	if len(mappers) != 1 {
		t.Fatalf("protocolMappers = %v, want one audience mapper", mappers)
	}
	mapper := mappers[0].(map[string]any)
	config := mapper["config"].(map[string]any)
	if mapper["protocolMapper"] != "oidc-audience-mapper" || config["included.client.audience"] != "clims" ||
		config["access.token.claim"] != "true" || config["id.token.claim"] != "false" {
		t.Errorf("audience mapper = %v", mapper)
	}

	// A second run against the now complete client changes nothing.
	realm.mappers = []protocolMapperRep{audienceMapper("clims")}
	realm.writes = nil
	result, err = admin.EnsureServiceClient(ctx, climsSyncSpec())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if result.Changed() || len(realm.writes) != 0 {
		t.Fatalf("second run result = %+v, writes = %v; want no changes", result, realm.writes)
	}
}

func TestEnsureServiceClientCompletesAnExistingClient(t *testing.T) {
	realm := &fakeServiceClientRealm{t: t, syncClient: map[string]any{
		"id": "uuid-sync", "clientId": "clims-identity-sync", "serviceAccountsEnabled": true,
	}}
	admin := revocationAdminFixture(t, realm.handler)

	result, err := admin.EnsureServiceClient(context.Background(), climsSyncSpec())
	if err != nil {
		t.Fatalf("EnsureServiceClient: %v", err)
	}
	if result.Created || !result.AudienceMapperAdded || len(result.RolesAssigned) != 1 || len(result.ScopeMappingsAdded) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(realm.mappers) != 1 || realm.mappers[0].Config["included.client.audience"] != "clims" {
		t.Fatalf("mappers = %+v", realm.mappers)
	}
}

func TestEnsureServiceClientNeverReshapesAnotherKindOfClient(t *testing.T) {
	for name, client := range map[string]map[string]any{
		"browser flow":       {"standardFlowEnabled": true, "serviceAccountsEnabled": true},
		"no service account": {"serviceAccountsEnabled": false},
		"public":             {"publicClient": true, "serviceAccountsEnabled": true},
		"password grant":     {"directAccessGrantsEnabled": true, "serviceAccountsEnabled": true},
	} {
		t.Run(name, func(t *testing.T) {
			client["id"], client["clientId"] = "uuid-sync", "clims-identity-sync"
			realm := &fakeServiceClientRealm{t: t, syncClient: client}
			admin := revocationAdminFixture(t, realm.handler)
			_, err := admin.EnsureServiceClient(context.Background(), climsSyncSpec())
			if !errors.Is(err, ErrNotServiceClient) {
				t.Fatalf("err = %v, want ErrNotServiceClient", err)
			}
			if len(realm.writes) != 0 {
				t.Fatalf("writes = %v, want none", realm.writes)
			}
		})
	}
}

func TestEnsureServiceClientRequiresClientIDAndAudience(t *testing.T) {
	admin := revocationAdminFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL.Path)
	})
	for _, spec := range []ServiceClientSpec{{Audience: "clims"}, {ClientID: "clims-identity-sync"}} {
		if _, err := admin.EnsureServiceClient(context.Background(), spec); err == nil {
			t.Errorf("spec %+v: want an error", spec)
		}
	}
}
