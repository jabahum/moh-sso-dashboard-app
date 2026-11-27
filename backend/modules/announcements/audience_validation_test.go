package announcements

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAudienceSelectionValidation(t *testing.T) {
	for _, tc := range []struct {
		audience string
		roles    []string
		ok       bool
	}{
		{"", nil, true},
		{"ALL_USERS", nil, true},
		{"admins_only", nil, true},
		{"SPECIFIC_ROLES", []string{"manager"}, true},
		{"SPECIFIC_ROLES", nil, false},
		{"SPECIFIC_ROLES", []string{"  "}, false},
		{"EVERYONE_IN_THE_WORLD", nil, false},
	} {
		_, _, _, _, ok := parseAnnouncementAudienceSelection(tc.audience, nil, tc.roles, nil, nil, nil)
		if ok != tc.ok {
			t.Fatalf("%q roles=%v: got ok=%v", tc.audience, tc.roles, ok)
		}
	}
}

func TestUpdateRequiresAudienceType(t *testing.T) {
	m, err := NewModule(httpDependencies())
	if err != nil {
		t.Fatal(err)
	}
	// httpRepo panics on writes, so a 400 proves nothing was updated.
	body := `{"title":"Only the title changed","message":"Body","level":"INFO"}`
	req := httptest.NewRequest(http.MethodPut, "/admin/announcements/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	m.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "audience_type is required") {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}
