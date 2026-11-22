package identityreconcile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// fakeKeycloak is a minimal in-memory Keycloak admin API for invitations.
type fakeKeycloak struct {
	t  *testing.T
	mu sync.Mutex

	users  []keycloak.KeycloakUser
	roles  map[string]map[string]bool // user ID -> client role names
	emails map[string][][]string      // user ID -> execute-actions-email action lists

	dropAttributes bool // the realm does not keep unmanaged attributes
	dropCreateResp bool // create succeeds but the connection drops (a timeout)
	failEmail      int  // execute-actions-email failures to return
	creates        int
	deletes        int
}

func newFakeKeycloak(t *testing.T) (*fakeKeycloak, *keycloak.KeyAdminClient) {
	f := &fakeKeycloak{t: t, roles: map[string]map[string]bool{}, emails: map[string][][]string{}}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	return f, keycloak.NewAdminClient(server.URL, "moh", "operator", "operator-secret")
}

func (f *fakeKeycloak) find(id string) int {
	for i, u := range f.users {
		if u.ID == id {
			return i
		}
	}
	return -1
}

func (f *fakeKeycloak) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := r.URL.Path
	writeJSON := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if path == "/realms/moh/protocol/openid-connect/token" {
		writeJSON(map[string]any{"access_token": "admin-token", "expires_in": 300})
		return
	}
	if r.Header.Get("Authorization") != "Bearer admin-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	const prefix = "/admin/realms/moh/"
	if !strings.HasPrefix(path, prefix) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	switch {
	case len(parts) == 1 && parts[0] == "clients" && r.Method == http.MethodGet:
		if r.URL.Query().Get("clientId") == "alerts" {
			writeJSON([]keycloak.ClientInfo{{ID: "client-uuid", ClientID: "alerts", Enabled: true}})
			return
		}
		writeJSON([]keycloak.ClientInfo{})
	case len(parts) == 4 && parts[0] == "clients" && parts[2] == "roles" && r.Method == http.MethodGet:
		if parts[1] != "client-uuid" || parts[3] != "alerts_access" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(keycloak.ClientRoleRep{ID: "role-uuid", Name: "alerts_access"})
	case len(parts) == 1 && parts[0] == "users" && r.Method == http.MethodGet:
		q := r.URL.Query()
		if q.Get("exact") != "true" || q.Get("briefRepresentation") != "false" {
			f.t.Errorf("user search without exact full representations: %s", r.URL.RawQuery)
		}
		var out []keycloak.KeycloakUser
		for _, u := range f.users {
			if email := q.Get("email"); email != "" && !strings.EqualFold(u.Email, email) {
				continue
			}
			if attr := q.Get("q"); attr != "" {
				name, value, _ := strings.Cut(attr, ":")
				if !hasAttribute(u, name, value) {
					continue
				}
			}
			out = append(out, u)
		}
		writeJSON(out)
	case len(parts) == 1 && parts[0] == "users" && r.Method == http.MethodPost:
		var req keycloak.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, u := range f.users {
			if strings.EqualFold(u.Username, req.Username) || strings.EqualFold(u.Email, req.Email) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"errorMessage":"User exists with same username"}`))
				return
			}
		}
		user := keycloak.KeycloakUser{ID: uuid.NewString(), Username: req.Username, Email: req.Email, Enabled: req.Enabled,
			EmailVerified: req.EmailVerified, RequiredActions: req.RequiredActions, Attributes: req.Attributes}
		if f.dropAttributes {
			user.Attributes = nil
		}
		f.users = append(f.users, user)
		f.creates++
		if f.dropCreateResp {
			f.dropCreateResp = false
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}
		w.Header().Set("Location", "http://kc/admin/realms/moh/users/"+user.ID)
		w.WriteHeader(http.StatusCreated)
	case len(parts) == 2 && parts[0] == "users":
		i := f.find(parts[1])
		if i < 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(f.users[i])
		case http.MethodDelete:
			f.users = append(f.users[:i], f.users[i+1:]...)
			f.deletes++
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	case len(parts) == 3 && parts[0] == "users" && parts[2] == "execute-actions-email" && r.Method == http.MethodPut:
		if f.find(parts[1]) < 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if f.failEmail > 0 {
			f.failEmail--
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorMessage":"smtp host secret.internal refused"}`))
			return
		}
		var actions []string
		_ = json.NewDecoder(r.Body).Decode(&actions)
		f.emails[parts[1]] = append(f.emails[parts[1]], actions)
		w.WriteHeader(http.StatusNoContent)
	case len(parts) >= 5 && parts[0] == "users" && parts[2] == "role-mappings" && parts[3] == "clients" && parts[4] == "client-uuid":
		if f.find(parts[1]) < 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if len(parts) == 6 && parts[5] == "composite" && r.Method == http.MethodGet {
			var out []keycloak.ClientRoleRep
			for name := range f.roles[parts[1]] {
				out = append(out, keycloak.ClientRoleRep{Name: name})
			}
			writeJSON(out)
			return
		}
		if len(parts) == 5 && r.Method == http.MethodPost {
			var reqs []keycloak.ClientRoleRequest
			_ = json.NewDecoder(r.Body).Decode(&reqs)
			if f.roles[parts[1]] == nil {
				f.roles[parts[1]] = map[string]bool{}
			}
			for _, req := range reqs {
				f.roles[parts[1]][req.Name] = true
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		f.t.Errorf("unexpected Keycloak request %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeKeycloak) addUser(u keycloak.KeycloakUser) keycloak.KeycloakUser {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	f.users = append(f.users, u)
	return u
}

func (f *fakeKeycloak) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.users)
}

func inviteDirectory(admin *keycloak.KeyAdminClient) KeycloakDirectory {
	return KeycloakDirectory{Admin: admin, Issuer: batchIssuer, Application: "alerts", AccessRole: "alerts_access"}
}

var nurse = identitysync.Account{ID: "acct-7", Email: "Nurse@Example.org"}

func TestInviteCreatesUserOnceAndReusesIt(t *testing.T) {
	ctx := context.Background()
	kc, admin := newFakeKeycloak(t)
	dir := inviteDirectory(admin)
	identity, err := dir.EnsureUser(ctx, nurse)
	if err != nil {
		t.Fatal(err)
	}
	if kc.count() != 1 || identity != (identitysync.Identity{Issuer: batchIssuer, Subject: kc.users[0].ID}) {
		t.Fatalf("identity %+v, users %+v", identity, kc.users)
	}
	u := kc.users[0]
	if u.Username != "nurse@example.org" || u.Email != "Nurse@Example.org" || !u.Enabled || u.EmailVerified ||
		strings.Join(u.RequiredActions, ",") != "VERIFY_EMAIL,UPDATE_PASSWORD" ||
		strings.Join(u.Attributes[OriginAttribute], ",") != "alerts:acct-7" || len(u.Attributes) != 1 {
		t.Fatalf("created user = %+v", u)
	}
	if got := kc.emails[u.ID]; len(got) != 1 || strings.Join(got[0], ",") != "VERIFY_EMAIL,UPDATE_PASSWORD" {
		t.Fatalf("invitation emails = %v", got)
	}
	// A retry reuses the same user and re-sends the pending invitation.
	again, err := dir.EnsureUser(ctx, nurse)
	if err != nil || again != identity || kc.count() != 1 || kc.creates != 1 || len(kc.emails[u.ID]) != 2 {
		t.Fatalf("retry = %+v, %v; users %d, emails %d", again, err, kc.count(), len(kc.emails[u.ID]))
	}
	// Once the person completed the invitation, a retry sends nothing.
	kc.mu.Lock()
	kc.users[0].EmailVerified, kc.users[0].RequiredActions = true, nil
	kc.mu.Unlock()
	if again, err = dir.EnsureUser(ctx, nurse); err != nil || again != identity || len(kc.emails[u.ID]) != 2 {
		t.Fatalf("completed invitation retry = %+v, %v; emails %d", again, err, len(kc.emails[u.ID]))
	}
	// The same email under another account of the application is a collision.
	if _, err := dir.EnsureUser(ctx, identitysync.Account{ID: "acct-8", Email: "nurse@example.org"}); !errors.Is(err, ErrInviteEmailCollision) {
		t.Fatalf("second account with same email: %v", err)
	}
}

func TestInviteRejectsUnrelatedUsers(t *testing.T) {
	ctx := context.Background()
	kc, admin := newFakeKeycloak(t)
	dir := inviteDirectory(admin)
	kc.addUser(keycloak.KeycloakUser{Username: "someone", Email: "NURSE@example.org", Enabled: true})
	if _, err := dir.EnsureUser(ctx, nurse); !errors.Is(err, ErrInviteEmailCollision) {
		t.Fatalf("email collision: %v", err)
	}
	// Username taken by a user with another email: the create conflicts.
	kc2, admin2 := newFakeKeycloak(t)
	kc2.addUser(keycloak.KeycloakUser{Username: "nurse@example.org", Email: "other@example.org", Enabled: true})
	if _, err := inviteDirectory(admin2).EnsureUser(ctx, nurse); !errors.Is(err, ErrInviteEmailCollision) {
		t.Fatalf("username collision: %v", err)
	}
	if kc.creates != 0 || kc2.creates != 0 {
		t.Fatal("a colliding user was created")
	}
	// A marked user whose email no longer matches is not reused.
	kc3, admin3 := newFakeKeycloak(t)
	kc3.addUser(keycloak.KeycloakUser{Username: "x", Email: "attacker@example.org", Enabled: true,
		Attributes: map[string][]string{OriginAttribute: {"alerts:acct-7"}}})
	if _, err := inviteDirectory(admin3).EnsureUser(ctx, nurse); !errors.Is(err, ErrInviteOriginMismatch) {
		t.Fatalf("origin with other email: %v", err)
	}
	kc4, admin4 := newFakeKeycloak(t)
	for range 2 {
		kc4.addUser(keycloak.KeycloakUser{Username: uuid.NewString(), Email: "nurse@example.org", Enabled: true,
			Attributes: map[string][]string{OriginAttribute: {"alerts:acct-7"}}})
	}
	if _, err := inviteDirectory(admin4).EnsureUser(ctx, nurse); !errors.Is(err, ErrInviteAmbiguous) {
		t.Fatalf("two marked users: %v", err)
	}
	kc5, admin5 := newFakeKeycloak(t)
	kc5.addUser(keycloak.KeycloakUser{Username: "nurse@example.org", Email: "nurse@example.org", Enabled: false,
		Attributes: map[string][]string{OriginAttribute: {"alerts:acct-7"}}})
	if _, err := inviteDirectory(admin5).EnsureUser(ctx, nurse); !errors.Is(err, ErrInviteUserDisabled) {
		t.Fatalf("disabled marked user: %v", err)
	}
	for _, bad := range []identitysync.Account{{ID: "", Email: "a@b.c"}, {ID: "x", Email: ""}, {ID: "x", Email: "no-at"}, {ID: "x", Email: "a b@c.d"}} {
		if _, err := dir.EnsureUser(ctx, bad); !errors.Is(err, ErrInviteInvalidAccount) {
			t.Errorf("account %+v: %v", bad, err)
		}
	}
}

func TestInviteIsIdempotentAcrossTimeouts(t *testing.T) {
	ctx := context.Background()
	kc, admin := newFakeKeycloak(t)
	dir := inviteDirectory(admin)
	// The create succeeds in Keycloak but the response is lost.
	kc.dropCreateResp = true
	identity, err := dir.EnsureUser(ctx, nurse)
	if err != nil {
		t.Fatal(err)
	}
	if kc.count() != 1 || kc.creates != 1 || identity.Subject != kc.users[0].ID || len(kc.emails[identity.Subject]) != 1 {
		t.Fatalf("after lost create response: %+v, users %d, emails %v", identity, kc.count(), kc.emails)
	}
	// The invitation email fails; the user stays and a retry sends it.
	kc2, admin2 := newFakeKeycloak(t)
	kc2.failEmail = 1
	_, err = inviteDirectory(admin2).EnsureUser(ctx, nurse)
	if !errors.Is(err, ErrInviteEmailFailed) || strings.Contains(err.Error(), "secret.internal") {
		t.Fatalf("email failure: %v", err)
	}
	identity, err = inviteDirectory(admin2).EnsureUser(ctx, nurse)
	if err != nil || kc2.count() != 1 || len(kc2.emails[identity.Subject]) != 1 {
		t.Fatalf("retry after email failure: %+v, %v", identity, err)
	}
}

func TestInviteRefusesWhenTheRealmDropsTheMarker(t *testing.T) {
	ctx := context.Background()
	kc, admin := newFakeKeycloak(t)
	kc.dropAttributes = true
	if _, err := inviteDirectory(admin).EnsureUser(ctx, nurse); !errors.Is(err, ErrInviteAttributeNotStored) {
		t.Fatalf("dropped attribute: %v", err)
	}
	if kc.count() != 0 || kc.deletes != 1 || len(kc.emails) != 0 {
		t.Fatalf("the unmarked user must be removed before any email: users %d, deletes %d", kc.count(), kc.deletes)
	}
}

func TestKeycloakDirectoryFindGrantAndAccess(t *testing.T) {
	ctx := context.Background()
	kc, admin := newFakeKeycloak(t)
	dir := inviteDirectory(admin)
	ok := kc.addUser(keycloak.KeycloakUser{Username: "a", Email: "verified@example.org", Enabled: true, EmailVerified: true})
	kc.addUser(keycloak.KeycloakUser{Username: "b", Email: "pending@example.org", Enabled: true})
	kc.addUser(keycloak.KeycloakUser{Username: "c", Email: "off@example.org", EmailVerified: true})
	for email, want := range map[string]identitysync.Identity{
		"VERIFIED@example.org": {Issuer: batchIssuer, Subject: ok.ID},
		"pending@example.org":  {},
		"off@example.org":      {},
	} {
		matches, err := dir.FindEmail(ctx, email)
		if err != nil || len(matches) != 1 || matches[0] != want {
			t.Errorf("FindEmail(%s) = %+v, %v", email, matches, err)
		}
	}
	if matches, err := dir.FindEmail(ctx, "nobody@example.org"); err != nil || len(matches) != 0 {
		t.Errorf("no match = %+v, %v", matches, err)
	}
	identity := identitysync.Identity{Issuer: batchIssuer, Subject: ok.ID}
	if has, err := dir.HasAccess(ctx, identity); err != nil || has {
		t.Fatalf("access before grant = %v, %v", has, err)
	}
	for range 2 {
		if err := dir.GrantAccess(ctx, identity); err != nil {
			t.Fatal(err)
		}
	}
	if has, err := dir.HasAccess(ctx, identity); err != nil || !has {
		t.Fatalf("access after grant = %v, %v", has, err)
	}
	if has, err := dir.HasAccess(ctx, identitysync.Identity{Issuer: batchIssuer, Subject: "deleted-user"}); err != nil || has {
		t.Fatalf("unknown user access = %v, %v", has, err)
	}
	if err := dir.GrantAccess(ctx, identitysync.Identity{Issuer: "https://other.test/realms/x", Subject: ok.ID}); err == nil {
		t.Fatal("granted an identity from another issuer")
	}
	other := dir
	other.Application = "missing-app"
	if err := other.GrantAccess(ctx, identity); err == nil {
		t.Fatal("granted through an unknown client")
	}
}
