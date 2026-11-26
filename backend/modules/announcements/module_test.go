package announcements

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type httpRepo struct{ Repository }

func (*httpRepo) ListPublicAnnouncements(context.Context, int32, int32) ([]Announcement, error) {
	return []Announcement{}, nil
}
func httpDependencies() Dependencies {
	return Dependencies{
		Repository: &httpRepo{},
		Actor: func(*http.Request) (Actor, error) {
			return Actor{UserID: uuid.MustParse("10000000-0000-0000-0000-000000000001")}, nil
		},
		RequireAdmin: func(*http.Request) error { return nil },
		AuthorizeAny: func(*http.Request, ...string) error { return nil },
		Limit:        func(*http.Request, string, int, time.Duration) (http.Header, error) { return nil, nil },
	}
}
func TestHTTPDependencyValidation(t *testing.T) {
	for _, mutate := range []func(*Dependencies){
		func(d *Dependencies) { d.Actor = nil }, func(d *Dependencies) { d.RequireAdmin = nil },
		func(d *Dependencies) { d.AuthorizeAny = nil }, func(d *Dependencies) { d.Limit = nil },
		func(d *Dependencies) { d.Repository = nil }, func(d *Dependencies) { d.MaxRequestBytes = -1 },
	} {
		d := httpDependencies()
		mutate(&d)
		if _, err := NewModule(d); !errors.Is(err, ErrInvalidDependencies) {
			t.Fatalf("expected dependency error: %v", err)
		}
	}
	if _, err := Register(nil, httpDependencies()); !errors.Is(err, ErrInvalidDependencies) {
		t.Fatal(err)
	}
}
func TestAllPrivateRoutesDenyBeforeBodyOrRepository(t *testing.T) {
	for _, route := range endpoints(&httpHandler{}) {
		if route.permission == "" {
			continue
		}
		for _, scenario := range []string{"anonymous", "permission", "admin", "unavailable"} {
			t.Run(route.method+route.path+scenario, func(t *testing.T) {
				d := httpDependencies()
				want := 403
				switch scenario {
				case "anonymous":
					d.Actor = func(*http.Request) (Actor, error) { return Actor{}, nil }
					want = 401
				case "permission":
					d.AuthorizeAny = func(*http.Request, ...string) error { return ErrForbidden }
				case "admin":
					if !route.admin {
						return
					}
					d.RequireAdmin = func(*http.Request) error { return ErrForbidden }
				case "unavailable":
					d.AuthorizeAny = func(*http.Request, ...string) error { return errors.New("offline") }
					want = 503
				}
				m, err := NewModule(d)
				if err != nil {
					t.Fatal(err)
				}
				path := strings.NewReplacer("{id}", uuid.NewString(), "{attachmentId}", uuid.NewString(), "{role_name}", "manager", "{client_id}", uuid.NewString()).Replace(route.path)
				req := httptest.NewRequest(route.method, path, strings.NewReader("invalid json"))
				req.Header.Set("X-User-ID", uuid.NewString())
				req.Header.Set("X-Roles", "admin")
				w := httptest.NewRecorder()
				m.ServeHTTP(w, req)
				if w.Code != want {
					t.Fatalf("got %d: %s", w.Code, w.Body.String())
				}
			})
		}
	}
}

type limited struct{}

func (limited) Error() string             { return "limited" }
func (limited) StatusCode() int           { return 429 }
func (limited) RetryAfter() time.Duration { return 1500 * time.Millisecond }
func TestWriteAndPublishPolicyFailClosed(t *testing.T) {
	for _, route := range endpoints(&httpHandler{}) {
		if route.policy == "" {
			continue
		}
		d := httpDependencies()
		d.Limit = func(_ *http.Request, policy string, limit int, window time.Duration) (http.Header, error) {
			want := WriteLimit
			if route.policy == PublishPolicy {
				want = PublishLimit
			}
			if policy != route.policy || limit != want || window != time.Minute {
				t.Fatal("policy drift")
			}
			return http.Header{"X-RateLimit-Policy": []string{policy}}, limited{}
		}
		m, err := NewModule(d)
		if err != nil {
			t.Fatal(err)
		}
		path := strings.NewReplacer("{id}", uuid.NewString(), "{attachmentId}", uuid.NewString()).Replace(route.path)
		w := httptest.NewRecorder()
		w.Header().Set("X-RateLimit-Policy", "outer")
		m.ServeHTTP(w, httptest.NewRequest(route.method, path, strings.NewReader("invalid json")))
		if w.Code != 429 || w.Header().Get("Retry-After") != "2" || w.Header().Get("X-RateLimit-Policy") != route.policy {
			t.Fatalf("%s: %+v", path, w)
		}
	}
}
func TestPublicRoutesNeedNoIdentityAndMetadataMatchesRoutes(t *testing.T) {
	d := httpDependencies()
	d.Actor = func(*http.Request) (Actor, error) { t.Fatal("public identity callback"); return Actor{}, nil }
	d.RequestID = func(*http.Request) string { return "fixture-request" }
	m, err := NewModule(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/announcements/public", "/announcements/active"} {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "fixture-request") {
			t.Fatalf("%+v", w)
		}
	}
	var doc struct{ Paths map[string]map[string]any }
	if err := json.Unmarshal(m.OpenAPI(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(m.Patterns()) != 26 {
		t.Fatal(m.Patterns())
	}
	for _, pattern := range m.Patterns() {
		parts := strings.SplitN(pattern, " ", 2)
		if doc.Paths[parts[1]][strings.ToLower(parts[0])] == nil {
			t.Fatalf("missing metadata: %s", pattern)
		}
	}
	patterns := m.Patterns()
	patterns[0] = "changed"
	if m.Patterns()[0] == "changed" {
		t.Fatal("mutable inventory")
	}
	for _, method := range []string{"HEAD", "OPTIONS", "PUT"} {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, httptest.NewRequest(method, "/announcements/public", nil))
		if w.Code != 405 {
			t.Fatalf("%s: %d", method, w.Code)
		}
	}
}

type downloadRepo struct {
	Repository
	item Announcement
	file AnnouncementAttachment
}

func (r downloadRepo) GetByID(context.Context, uuid.UUID) (Announcement, error) { return r.item, nil }
func (r downloadRepo) GetAttachmentByID(context.Context, GetAnnouncementAttachmentByIDParams) (AnnouncementAttachment, error) {
	return r.file, nil
}

type brokenRead struct{ sent bool }

func (r *brokenRead) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "partial"), nil
	}
	return 0, errors.New("stream failed")
}
func (*brokenRead) Close() error { return nil }

type downloadStorage struct{ AttachmentStorage }

func (downloadStorage) Download(context.Context, string) (io.ReadCloser, error) {
	return &brokenRead{}, nil
}
func TestStreamErrorAndEncodedFilename(t *testing.T) {
	id, file := uuid.New(), uuid.New()
	d := httpDependencies()
	d.Repository = downloadRepo{item: Announcement{ID: id, Status: AnnouncementStatusPUBLISHED, AudienceType: AnnouncementAudienceTypeALLUSERS},
		file: AnnouncementAttachment{ID: file, AnnouncementID: id, OriginalFileName: "quote\"\r\n.txt", StorageKey: "fixture"}}
	d.Ports.Storage = downloadStorage{}
	reported := false
	d.StreamError = func(_ *http.Request, err error) { reported = err != nil }
	m, err := NewModule(d)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", "/announcements/"+id.String()+"/attachments/"+file.String()+"/download", nil))
	if w.Code != 200 || w.Body.String() != "partial" || !reported {
		t.Fatalf("stream status/body/report: %+v", w)
	}
	if strings.ContainsAny(w.Header().Get("Content-Disposition"), "\r\n") {
		t.Fatal("unsafe disposition")
	}
}

func TestJSONRequiredFieldsAndBodyLimit(t *testing.T) {
	d := httpDependencies()
	d.MaxRequestBytes = 32
	m, err := NewModule(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"{}", strings.Repeat("x", 64), `{"title":"a","message":"b"}`} {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, httptest.NewRequest("POST", "/admin/announcements", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("validation bypass: %d", w.Code)
		}
	}
}

type audiencePageRepo struct {
	Repository
	rows []Announcement
	user uuid.UUID
}

func (r audiencePageRepo) ListActivePublished(_ context.Context, p ListActivePublishedAnnouncementsParams) ([]Announcement, error) {
	if int(p.Offset) >= len(r.rows) {
		return nil, nil
	}
	end := int(p.Offset + p.Limit)
	if end > len(r.rows) {
		end = len(r.rows)
	}
	return r.rows[int(p.Offset):end], nil
}
func (r audiencePageRepo) ListUserAudience(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.New()}, nil
}
func (audiencePageRepo) ListAttachmentsByAnnouncementID(context.Context, uuid.UUID) ([]AnnouncementAttachment, error) {
	return nil, nil
}
func TestVisiblePaginationSkipsPrivatePages(t *testing.T) {
	rows := make([]Announcement, 102)
	for i := range rows {
		rows[i] = Announcement{ID: uuid.New(), Status: AnnouncementStatusPUBLISHED, AudienceType: AnnouncementAudienceTypeSPECIFICUSERS}
	}
	rows[100].AudienceType = AnnouncementAudienceTypeALLUSERS
	rows[100].Title = "first-public"
	rows[101].AudienceType = AnnouncementAudienceTypeALLUSERS
	rows[101].Title = "second-public"
	d := httpDependencies()
	d.Repository = audiencePageRepo{rows: rows}
	m, err := NewModule(d)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", "/announcements/me?offset=1&limit=1", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "first-public") || !strings.Contains(w.Body.String(), "second-public") {
		t.Fatalf("pagination leak/omission: %s", w.Body.String())
	}
}

func TestPublishAndScheduleRejectServerPathAttachments(t *testing.T) {
	m, err := NewModule(httpDependencies())
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"/publish", "/schedule"} {
		body := `{"publish_at":"2030-01-01T09:00:00Z","attachments":[{"file_name":"a.pdf","path":"/proc/self/environ"}]}`
		req := httptest.NewRequest(http.MethodPost, "/admin/announcements/"+uuid.NewString()+suffix, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		// httpRepo panics on any repository use, so a 400 proves the request
		// was refused before the service ran.
		m.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "attachment path is not supported") {
			t.Fatalf("%s: got %d: %s", suffix, w.Code, w.Body.String())
		}
	}
}

func TestRequestAttachmentsNeverCarryServerPath(t *testing.T) {
	out := mapAnnouncementAttachments([]announcementAttachmentRequest{{FileName: "a.pdf", Path: "/etc/passwd", DataBase64: "AA=="}})
	if out[0].Path != "" {
		t.Fatalf("server path forwarded to the email pipeline: %q", out[0].Path)
	}
}
