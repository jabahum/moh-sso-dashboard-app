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

// updateRepo stores one announcement; audience listings fail so the
// response skips them, and any unexpected repository call panics.
type updateRepo struct {
	Repository
	item      Announcement
	updates   int
	published int
	archived  int
}

func (r *updateRepo) GetByID(context.Context, uuid.UUID) (Announcement, error) { return r.item, nil }
func (r *updateRepo) Update(_ context.Context, params UpdateAnnouncementParams) (Announcement, error) {
	r.updates++
	r.item.Title = params.Title
	return r.item, nil
}
func (r *updateRepo) ReplaceClientAudience(context.Context, uuid.UUID, []uuid.UUID) error { return nil }
func (r *updateRepo) ReplaceRoleAudience(context.Context, uuid.UUID, []string) error      { return nil }
func (r *updateRepo) ReplaceUserAudience(context.Context, uuid.UUID, []uuid.UUID) error   { return nil }
func (r *updateRepo) ReplaceGroupAudience(context.Context, uuid.UUID, []uuid.UUID) error  { return nil }
func (r *updateRepo) ReplaceHealthContextAudience(context.Context, uuid.UUID, []uuid.UUID, bool) error {
	return nil
}
func (r *updateRepo) PublishNow(context.Context, uuid.UUID, uuid.UUID) (Announcement, error) {
	r.published++
	r.item.Status = AnnouncementStatusPUBLISHED
	return r.item, nil
}
func (r *updateRepo) Archive(context.Context, uuid.UUID, uuid.UUID) (Announcement, error) {
	r.archived++
	r.item.Status = AnnouncementStatusARCHIVED
	return r.item, nil
}
func (r *updateRepo) ListClientAudience(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, errors.New("not stored in this fixture")
}

func updateModule(t *testing.T, repo *updateRepo, canPublish bool) *Module {
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

func putStatus(id uuid.UUID, status string) *http.Request {
	body := `{"title":"Edited","message":"Body","level":"INFO","audience_type":"ALL_USERS","status":"` + status + `"}`
	req := httptest.NewRequest(http.MethodPut, "/admin/announcements/"+id.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestUpdateAppliesStatusChangeThroughLifecycle(t *testing.T) {
	id := uuid.New()
	for status, check := range map[string]func(*updateRepo) bool{
		"PUBLISHED": func(r *updateRepo) bool { return r.published == 1 },
		"archived":  func(r *updateRepo) bool { return r.archived == 1 },
	} {
		repo := &updateRepo{item: Announcement{ID: id, Status: AnnouncementStatusDRAFT}}
		w := httptest.NewRecorder()
		updateModule(t, repo, true).ServeHTTP(w, putStatus(id, status))
		if w.Code != http.StatusOK || repo.updates != 1 || !check(repo) {
			t.Fatalf("%s: got %d %+v: %s", status, w.Code, repo, w.Body.String())
		}
	}
}

func TestUpdateStatusChangeNeedsPublishPermission(t *testing.T) {
	id := uuid.New()
	repo := &updateRepo{item: Announcement{ID: id, Status: AnnouncementStatusDRAFT}}
	w := httptest.NewRecorder()
	updateModule(t, repo, false).ServeHTTP(w, putStatus(id, "PUBLISHED"))
	if w.Code != http.StatusForbidden || repo.updates != 0 || repo.published != 0 {
		t.Fatalf("got %d, updates=%d published=%d", w.Code, repo.updates, repo.published)
	}
}

func TestUpdateWithUnchangedStatusNeedsOnlyWrite(t *testing.T) {
	id := uuid.New()
	repo := &updateRepo{item: Announcement{ID: id, Status: AnnouncementStatusPUBLISHED}}
	w := httptest.NewRecorder()
	updateModule(t, repo, false).ServeHTTP(w, putStatus(id, "PUBLISHED"))
	if w.Code != http.StatusOK || repo.updates != 1 || repo.published != 0 {
		t.Fatalf("got %d, updates=%d published=%d", w.Code, repo.updates, repo.published)
	}
}

func TestUpdateRejectsScheduleWithoutPublishAt(t *testing.T) {
	id := uuid.New()
	repo := &updateRepo{item: Announcement{ID: id, Status: AnnouncementStatusDRAFT}}
	w := httptest.NewRecorder()
	updateModule(t, repo, true).ServeHTTP(w, putStatus(id, "SCHEDULED"))
	if w.Code != http.StatusBadRequest || repo.updates != 0 {
		t.Fatalf("got %d, updates=%d", w.Code, repo.updates)
	}
}
