package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/keycloak"
)

// The client-role endpoints keep their success status (204 with nothing to
// report) and return 200 with a "warnings" array when access removal or a
// restore did not fully succeed.
func TestClientRoleHandlersReturnWarnings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user, clientUUID := uuid.New(), uuid.New()
	body := `{"clientId":"clims","clientUuid":"` + clientUUID.String() + `","roles":["clims_access"]}`
	emptyRoles := `{"clientId":"clims","clientUuid":"` + clientUUID.String() + `","roles":[]}`
	for _, tc := range []struct {
		name, method, path, body string
		hookErr                  error
		status                   int
	}{
		{"update with incomplete revocation", http.MethodPut, "/users/" + user.String() + "/client-roles", emptyRoles, errors.New("incomplete"), http.StatusOK},
		{"update without problems", http.MethodPut, "/users/" + user.String() + "/client-roles", emptyRoles, nil, http.StatusNoContent},
		{"remove with incomplete revocation", http.MethodDelete, "/users/" + user.String() + "/client-roles", body, errors.New("incomplete"), http.StatusOK},
		{"remove without problems", http.MethodDelete, "/users/" + user.String() + "/client-roles", body, nil, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_access"}}}
			service := NewService(repo, nil)
			service.SetAccessRoleRemovalHook(&recordingRemovalHook{err: tc.hookErr})
			handler := NewHandler(service, nil, nil)
			router := gin.New()
			router.PUT("/users/:id/client-roles", handler.UpdateUserClientRoles)
			router.DELETE("/users/:id/client-roles", handler.RemoveUserClientRoles)
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, tc.status, recorder.Body.String())
			}
			if tc.status == http.StatusNoContent {
				return
			}
			var envelope struct {
				Data struct {
					Warnings []struct{ Code, ClientID string } `json:"warnings"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("body %s: %v", recorder.Body.String(), err)
			}
			if len(envelope.Data.Warnings) != 1 || envelope.Data.Warnings[0].ClientID != "clims" || envelope.Data.Warnings[0].Code == "" {
				t.Fatalf("body = %s", recorder.Body.String())
			}
		})
	}
}
