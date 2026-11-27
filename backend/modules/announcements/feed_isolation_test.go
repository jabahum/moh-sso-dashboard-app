package announcements

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// brokenGroupRepo serves one public and one group-targeted announcement whose
// group audience cannot be loaded.
type brokenGroupRepo struct{ audiencePageRepo }

func (brokenGroupRepo) ListGroupAudience(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, errors.New("group audience table unavailable")
}

type anyGroups struct{ GroupDirectory }

func TestFeedHidesAnnouncementWhoseAudienceCheckFails(t *testing.T) {
	rows := []Announcement{
		{ID: uuid.New(), Title: "group-only", Status: AnnouncementStatusPUBLISHED, AudienceType: AnnouncementAudienceTypeSPECIFICGROUPS},
		{ID: uuid.New(), Title: "everyone", Status: AnnouncementStatusPUBLISHED, AudienceType: AnnouncementAudienceTypeALLUSERS},
	}
	var reported []error
	d := httpDependencies()
	d.Repository = brokenGroupRepo{audiencePageRepo{rows: rows}}
	d.Ports.Groups = anyGroups{}
	d.StreamError = func(_ *http.Request, err error) { reported = append(reported, err) }
	m, err := NewModule(d)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/announcements/me", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "everyone") || strings.Contains(body, "group-only") {
		t.Fatalf("got %d: %s", w.Code, body)
	}
	if len(reported) != 1 || !strings.Contains(reported[0].Error(), rows[0].ID.String()) {
		t.Fatalf("hidden announcement was not reported: %v", reported)
	}
}
