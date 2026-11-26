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

// createRepo records what the create handler persists. Audience listings
// fail so the response skips them; any unexpected repository call panics.
type createRepo struct {
	Repository
	created   []CreateAnnouncementParams
	published []uuid.UUID
}

var errNoAudience = errors.New("not stored in this fixture")

func (r *createRepo) Create(_ context.Context, params CreateAnnouncementParams) (Announcement, error) {
	r.created = append(r.created, params)
	return Announcement{ID: uuid.New(), Title: params.Title, Status: params.Status, AudienceType: params.AudienceType}, nil
}
func (r *createRepo) ReplaceHealthContextAudience(context.Context, uuid.UUID, []uuid.UUID, bool) error {
	return nil
}
func (r *createRepo) GetByID(_ context.Context, id uuid.UUID) (Announcement, error) {
	return Announcement{ID: id, Status: AnnouncementStatusDRAFT}, nil
}
func (r *createRepo) PublishNow(_ context.Context, id, actor uuid.UUID) (Announcement, error) {
	r.published = append(r.published, id)
	return Announcement{ID: id, Status: AnnouncementStatusPUBLISHED, PublishedBy: uuid.NullUUID{UUID: actor, Valid: true}}, nil
}
func (r *createRepo) ListClientAudience(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, errNoAudience
}

func createRequest(status string) *http.Request {
	body := `{"title":"Clinic hours","message":"Open late","level":"INFO","audience_type":"ALL_USERS","status":"` + status + `"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/announcements", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func createModule(t *testing.T, repo *createRepo, canPublish bool) *Module {
	t.Helper()
	d := httpDependencies()
	d.Repository = repo
	d.AuthorizeAny = func(_ *http.Request, permissions ...string) error {
		for _, permission := range permissions {
			if permission == PublishPermission && !canPublish {
				return ErrForbidden
			}
		}
		return nil
	}
	m, err := NewModule(d)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCreateNonDraftRequiresPublishPermission(t *testing.T) {
	for _, status := range []string{"PUBLISHED", "published", "SCHEDULED"} {
		repo := &createRepo{}
		w := httptest.NewRecorder()
		createModule(t, repo, false).ServeHTTP(w, createRequest(status))
		if w.Code != http.StatusForbidden || len(repo.created) != 0 {
			t.Fatalf("%s without publish permission: got %d, created %d", status, w.Code, len(repo.created))
		}
	}
}

func TestCreateRejectsUnsupportedStatuses(t *testing.T) {
	repo := &createRepo{}
	w := httptest.NewRecorder()
	createModule(t, repo, true).ServeHTTP(w, createRequest("ARCHIVED"))
	if w.Code != http.StatusBadRequest || len(repo.created) != 0 {
		t.Fatalf("ARCHIVED create: got %d", w.Code)
	}

	w = httptest.NewRecorder()
	createModule(t, repo, true).ServeHTTP(w, createRequest("SCHEDULED")) // no publish_at
	if w.Code != http.StatusBadRequest || len(repo.created) != 0 {
		t.Fatalf("SCHEDULED without publish_at: got %d", w.Code)
	}
}

func TestCreatePublishedInsertsDraftThenPublishes(t *testing.T) {
	repo := &createRepo{}
	w := httptest.NewRecorder()
	createModule(t, repo, true).ServeHTTP(w, createRequest("PUBLISHED"))
	if w.Code != http.StatusCreated {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.created) != 1 || repo.created[0].Status != AnnouncementStatusDRAFT {
		t.Fatalf("expected a draft insert, got %+v", repo.created)
	}
	if len(repo.published) != 1 || !strings.Contains(w.Body.String(), `"status":"PUBLISHED"`) {
		t.Fatalf("expected the publish path to run: %v %s", repo.published, w.Body.String())
	}
}

func TestCreateDraftNeedsOnlyWritePermission(t *testing.T) {
	repo := &createRepo{}
	w := httptest.NewRecorder()
	createModule(t, repo, false).ServeHTTP(w, createRequest("DRAFT"))
	if w.Code != http.StatusCreated || len(repo.created) != 1 || len(repo.published) != 0 {
		t.Fatalf("draft create: got %d", w.Code)
	}
}
