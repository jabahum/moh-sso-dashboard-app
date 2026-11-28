package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/identityreconcile"
)

type fakeIdentitySync struct {
	saved     []identityreconcile.Integration
	actors    []string
	reasons   []string
	deleted   []string
	saveErr   error
	getErr    error
	deleteErr error
}

func (f *fakeIdentitySync) Get(_ context.Context, app string) (identityreconcile.Integration, error) {
	if f.getErr != nil {
		return identityreconcile.Integration{}, f.getErr
	}
	return identityreconcile.Integration{ApplicationClientID: app, Origin: "https://alerts.example.test", CredentialRef: "env:ALERTS_SYNC_CLIENT_SECRET", ContractVersion: "1.1"}, nil
}

func (f *fakeIdentitySync) Save(_ context.Context, i identityreconcile.Integration, actor, reason string) (identityreconcile.Integration, error) {
	if f.saveErr != nil {
		return identityreconcile.Integration{}, f.saveErr
	}
	f.saved, f.actors, f.reasons = append(f.saved, i), append(f.actors, actor), append(f.reasons, reason)
	if i.Enabled {
		i.ContractVersion = "1.1"
	}
	return i, nil
}

func (f *fakeIdentitySync) Delete(_ context.Context, app, actor, reason string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted, f.actors, f.reasons = append(f.deleted, app), append(f.actors, actor), append(f.reasons, reason)
	return nil
}

func identitySyncRouter(registry IdentitySyncRegistry, permissions ...authz.Permission) *gin.Engine {
	h := &Handler{Repository: &testRepository{}, Verifier: testVerifier{token: validToken()}}
	if registry != nil {
		h.IdentitySync = registry
	}
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(authz.ContextKey, authz.Context{UserID: "operator-1", Permissions: permissions})
		c.Set("user_id", "operator-1")
	})
	RegisterRoutes(router.Group("/api/v1"), protected, h, nil)
	return router
}

const identitySyncPath = "/api/v1/admin/platform/applications/alerts/identity-sync"

func identitySyncRequest(router *gin.Engine, method, body string) (int, string) {
	req := httptest.NewRequest(method, identitySyncPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func validIdentitySyncBody(mutate func(map[string]any)) string {
	body := map[string]any{
		"origin": "https://alerts.example.test", "syncClientId": "alerts-identity-sync", "accessRole": "alerts_access",
		"credentialRef": "env:ALERTS_SYNC_CLIENT_SECRET", "accountIdFormat": "uuid", "enabled": true, "reason": "  onboard alerts ",
	}
	if mutate != nil {
		mutate(body)
	}
	data, _ := json.Marshal(body)
	return string(data)
}

func TestIdentitySyncAdminRequiresBothRBACWritePermissions(t *testing.T) {
	for name, permissions := range map[string][]authz.Permission{
		"none":             nil,
		"read only":        {authz.PermissionRBACRead},
		"rbac:write only":  {authz.PermissionRBACWrite},
		"roles:write only": {authz.PermissionRBACRolesWrite},
	} {
		registry := &fakeIdentitySync{}
		router := identitySyncRouter(registry, permissions...)
		for _, method := range []string{"GET", "PUT", "DELETE"} {
			if code, _ := identitySyncRequest(router, method, validIdentitySyncBody(nil)); code != 403 {
				t.Errorf("%s %s: status %d, want 403", name, method, code)
			}
		}
		if len(registry.saved)+len(registry.deleted) != 0 {
			t.Errorf("%s: registry changed without permission", name)
		}
	}
}

func TestIdentitySyncAdminSaveValidatesInput(t *testing.T) {
	registry := &fakeIdentitySync{}
	router := identitySyncRouter(registry, authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite)
	for name, mutate := range map[string]func(map[string]any){
		"missing reason":      func(b map[string]any) { delete(b, "reason") },
		"blank reason":        func(b map[string]any) { b["reason"] = "   " },
		"long reason":         func(b map[string]any) { b["reason"] = strings.Repeat("x", 2001) },
		"http origin":         func(b map[string]any) { b["origin"] = "http://alerts.example.test" },
		"origin with path":    func(b map[string]any) { b["origin"] = "https://alerts.example.test/sync" },
		"origin with slash":   func(b map[string]any) { b["origin"] = "https://alerts.example.test/" },
		"origin with query":   func(b map[string]any) { b["origin"] = "https://alerts.example.test?x=1" },
		"origin with user":    func(b map[string]any) { b["origin"] = "https://user@alerts.example.test" },
		"inline secret":       func(b map[string]any) { b["credentialRef"] = "super-secret" },
		"relative file ref":   func(b map[string]any) { b["credentialRef"] = "file:secrets/alerts" },
		"unknown id format":   func(b map[string]any) { b["accountIdFormat"] = "email" },
		"missing sync client": func(b map[string]any) { delete(b, "syncClientId") },
		"missing access role": func(b map[string]any) { delete(b, "accessRole") },
	} {
		if code, _ := identitySyncRequest(router, "PUT", validIdentitySyncBody(mutate)); code != 400 {
			t.Errorf("%s: status %d, want 400", name, code)
		}
	}
	if code, _ := identitySyncRequest(router, "PUT", `{"origin":`); code != 400 {
		t.Errorf("malformed JSON: %d", code)
	}
	if len(registry.saved) != 0 {
		t.Fatal("invalid input reached the registry")
	}
}

func TestIdentitySyncAdminSaveGetDelete(t *testing.T) {
	registry := &fakeIdentitySync{}
	router := identitySyncRouter(registry, authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite)
	code, body := identitySyncRequest(router, "PUT", validIdentitySyncBody(func(b map[string]any) { b["origin"] = "https://alerts.example.test:8443" }))
	if code != 200 {
		t.Fatalf("PUT status %d: %s", code, body)
	}
	if len(registry.saved) != 1 || registry.actors[0] != "operator-1" || registry.reasons[0] != "onboard alerts" {
		t.Fatalf("saved = %+v, actors = %v, reasons = %v", registry.saved, registry.actors, registry.reasons)
	}
	saved := registry.saved[0]
	if saved.ApplicationClientID != "alerts" || saved.Origin != "https://alerts.example.test:8443" || saved.AccountIDFormat != "uuid" || !saved.Enabled {
		t.Fatalf("saved = %+v", saved)
	}
	if !strings.Contains(body, `"contractVersion":"1.1"`) || !strings.Contains(body, `"credentialRef":"env:ALERTS_SYNC_CLIENT_SECRET"`) {
		t.Fatalf("PUT body = %s", body)
	}
	if code, body := identitySyncRequest(router, "GET", ""); code != 200 || !strings.Contains(body, `"applicationId":"alerts"`) {
		t.Fatalf("GET %d: %s", code, body)
	}
	if code, _ := identitySyncRequest(router, "DELETE", ``); code != 400 {
		t.Fatalf("DELETE without reason: %d", code)
	}
	if code, _ := identitySyncRequest(router, "DELETE", `{"reason":" "}`); code != 400 {
		t.Fatalf("DELETE with blank reason: %d", code)
	}
	if code, _ := identitySyncRequest(router, "DELETE", `{"reason":"retire alerts"}`); code != 200 || len(registry.deleted) != 1 || registry.reasons[1] != "retire alerts" {
		t.Fatalf("DELETE %d, deleted = %v", code, registry.deleted)
	}
}

func TestIdentitySyncAdminMapsRegistryErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{identityreconcile.ErrIntegrationNotFound, 404},
		{fmt.Errorf("%w: access role", identityreconcile.ErrInvalidIntegration), 400},
		{fmt.Errorf("%w: HTTP 403 with secret-looking detail", identityreconcile.ErrDescriptorUnavailable), 422},
		{identityreconcile.ErrIntegrationInUse, 409},
		{fmt.Errorf("%w: alerts was registered at 1.1", identityreconcile.ErrContractDowngrade), 409},
	} {
		registry := &fakeIdentitySync{saveErr: tc.err, getErr: tc.err, deleteErr: tc.err}
		router := identitySyncRouter(registry, authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite)
		code, body := identitySyncRequest(router, "PUT", validIdentitySyncBody(nil))
		if code != tc.want || strings.Contains(body, "secret-looking") {
			t.Errorf("PUT %v: %d %s", tc.err, code, body)
		}
		// GET answers an unregistered application with 200 and data: null.
		if errors.Is(tc.err, identityreconcile.ErrIntegrationNotFound) {
			if code, body := identitySyncRequest(router, "GET", ""); code != 200 || !strings.Contains(body, `"data":null`) {
				t.Errorf("GET %v: %d %s", tc.err, code, body)
			}
		} else if code, _ := identitySyncRequest(router, "GET", ""); code != tc.want {
			t.Errorf("GET %v: %d", tc.err, code)
		}
		if code, _ := identitySyncRequest(router, "DELETE", `{"reason":"x"}`); code != tc.want {
			t.Errorf("DELETE %v: %d", tc.err, code)
		}
	}
	router := identitySyncRouter(nil, authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite)
	if code, _ := identitySyncRequest(router, "GET", ""); code != 503 {
		t.Errorf("unconfigured registry: %d", code)
	}
}

func TestIdentitySyncAdminPassesExplicitContractDowngrade(t *testing.T) {
	registry := &fakeIdentitySync{}
	router := identitySyncRouter(registry, authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite)
	if code, body := identitySyncRequest(router, "PUT", validIdentitySyncBody(nil)); code != 200 {
		t.Fatalf("PUT: %d %s", code, body)
	}
	if code, body := identitySyncRequest(router, "PUT", validIdentitySyncBody(func(b map[string]any) { b["allowContractDowngrade"] = true })); code != 200 {
		t.Fatalf("PUT with downgrade: %d %s", code, body)
	}
	if len(registry.saved) != 2 || registry.saved[0].AllowContractDowngrade || !registry.saved[1].AllowContractDowngrade {
		t.Fatalf("downgrade option not passed through exactly when requested: %+v", registry.saved)
	}
}

func TestIdentitySyncAdminExplainsNetworkFailures(t *testing.T) {
	dnsErr := &net.DNSError{Err: "no such host", Name: "alerts.example.test", IsNotFound: true}
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	for _, tc := range []struct {
		name, code, phrase string
		err                error
	}{
		{"unresolvable origin on save", "IDENTITY_SYNC_ORIGIN_UNRESOLVED", "does not resolve from the portal server",
			fmt.Errorf("%w: alerts.example.test: %w", identityreconcile.ErrOriginUnresolvable, dnsErr)},
		{"private origin", "IDENTITY_SYNC_PRIVATE_ORIGIN", identityreconcile.AllowedPrivateHostsEnv,
			fmt.Errorf("%w: alerts.example.test (10.0.0.5)", identityreconcile.ErrPrivateOrigin)},
		{"refused while describing", "IDENTITY_SYNC_UNREACHABLE", "connection was refused",
			fmt.Errorf("%w: enable alerts: giving up after 5 attempts: %w", identityreconcile.ErrDescriptorUnavailable, refused)},
		{"unknown describe failure keeps the generic text", "IDENTITY_SYNC_UNREACHABLE", "check the origin, credential reference and sync client",
			fmt.Errorf("%w: HTTP 403", identityreconcile.ErrDescriptorUnavailable)},
	} {
		router := identitySyncRouter(&fakeIdentitySync{saveErr: tc.err}, authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite)
		code, body := identitySyncRequest(router, "PUT", validIdentitySyncBody(nil))
		if code != 422 || !strings.Contains(body, `"code":"`+tc.code+`"`) || !strings.Contains(body, tc.phrase) {
			t.Errorf("%s: %d %s", tc.name, code, body)
		}
		if strings.Contains(body, "platform request failed") {
			t.Errorf("%s: generic platform error returned", tc.name)
		}
	}
}
