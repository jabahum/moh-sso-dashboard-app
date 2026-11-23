package storagelocations

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type memoryRepository struct {
	items   map[uuid.UUID]Location
	created []CreateInput
}

func (m *memoryRepository) Create(_ context.Context, input CreateInput) (Location, error) {
	m.created = append(m.created, input)
	location := Location{ID: uuid.New(), Code: input.Code, Name: input.Name, Provider: input.Provider, BaseUri: input.BaseUri, IsActive: input.IsActive}
	m.items[location.ID] = location
	return location, nil
}
func (m *memoryRepository) GetByID(_ context.Context, id uuid.UUID) (Location, error) {
	if location, ok := m.items[id]; ok {
		return location, nil
	}
	return Location{}, sql.ErrNoRows
}
func (m *memoryRepository) GetByCode(_ context.Context, code string) (Location, error) {
	for _, location := range m.items {
		if location.Code == code {
			return location, nil
		}
	}
	return Location{}, sql.ErrNoRows
}
func (m *memoryRepository) ListActive(context.Context) ([]Location, error) {
	out := []Location{}
	for _, location := range m.items {
		out = append(out, location)
	}
	return out, nil
}
func (m *memoryRepository) Update(_ context.Context, input UpdateStorageLocationInput) (Location, error) {
	location, ok := m.items[input.ID]
	if !ok {
		return Location{}, sql.ErrNoRows
	}
	location.Name, location.Provider, location.BaseUri, location.IsActive = input.Name, input.Provider, input.BaseUri, input.IsActive
	m.items[input.ID] = location
	return location, nil
}
func (m *memoryRepository) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.items[id]; !ok {
		return errors.New("missing")
	}
	delete(m.items, id)
	return nil
}

type recordingNotifier struct{ got []modulekit.Notification }

func (r *recordingNotifier) Notify(_ context.Context, n modulekit.Notification) error {
	r.got = append(r.got, n)
	return nil
}

func newTestModule(t *testing.T) (*Module, *memoryRepository, *recordingNotifier) {
	t.Helper()
	repo := &memoryRepository{items: map[uuid.UUID]Location{}}
	notifier := &recordingNotifier{}
	module, err := NewModule(Dependencies{Repository: repo, Notifier: notifier, Platform: modulekit.Platform{SystemAdminEmail: "admin@example.org", AdminDashboardURL: "https://portal.example/admin/home"}})
	if err != nil {
		t.Fatal(err)
	}
	return module, repo, notifier
}

func call(module http.Handler, method, target, body string, permissions ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(modulekit.WithRequest(req.Context(), modulekit.Request{
		Authenticated: true, RequestID: "r1",
		Access: core.RequestAccess{Identity: core.Identity{Subject: "u1"}, Permissions: permissions},
	}))
	recorder := httptest.NewRecorder()
	module.ServeHTTP(recorder, req)
	return recorder
}

func TestEndpointsKeepTheirPermissionsAndResponses(t *testing.T) {
	module, repo, notifier := newTestModule(t)
	if got := call(module, "GET", "/storage-locations", ""); got.Code != 403 ||
		got.Body.String() != `{"success":false,"error":{"code":"FORBIDDEN","message":"permission required: storage_locations:read"},"meta":{"requestId":"r1"}}` {
		t.Fatalf("read without permission: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "POST", "/storage-locations", `{"code":"c","name":"n","provider":"s3","is_active":true}`, ReadPermission); got.Code != 403 {
		t.Fatalf("write with read permission: %d", got.Code)
	}
	if got := call(module, "POST", "/storage-locations", `{"code":"c","name":"n","provider":"s3"}`, WritePermission); got.Code != 400 {
		t.Fatalf("missing required is_active: %d %s", got.Code, got.Body.String())
	}
	got := call(module, "POST", "/storage-locations", `{"code":" c ","name":"n","provider":"s3","base_uri":"s3://b","is_active":true}`, WritePermission)
	if got.Code != 201 || !strings.Contains(got.Body.String(), `"code":"c"`) {
		t.Fatalf("create: %d %s", got.Code, got.Body.String())
	}
	if repo.created[0].IsActive.Valid {
		t.Fatal("create changed its stored is_active value")
	}
	if len(notifier.got) != 1 || notifier.got[0].Type != "STORAGE_LOCATION_CREATED" || len(notifier.got[0].Deliveries) != 0 {
		t.Fatalf("create notification: %+v", notifier.got)
	}
	var id uuid.UUID
	for key := range repo.items {
		id = key
	}
	if got := call(module, "GET", "/storage-locations/"+id.String(), "", ReadPermission); got.Code != 200 {
		t.Fatalf("get: %d", got.Code)
	}
	if got := call(module, "GET", "/storage-locations/not-a-uuid", "", ReadPermission); got.Code != 404 {
		t.Fatalf("invalid id keeps its 404: %d", got.Code)
	}
	if got := call(module, "PUT", "/storage-locations/"+id.String(), `{"name":"n2","provider":"s3","is_active":{"Bool":true,"Valid":true}}`, WritePermission); got.Code != 200 {
		t.Fatalf("update: %d %s", got.Code, got.Body.String())
	}
	update := notifier.got[1]
	if update.Type != "STORAGE_LOCATION_UPDATED" || len(update.Deliveries) != 2 ||
		update.Deliveries[1].TemplateData["ActionURL"] != "https://portal.example/admin/storage-locations" {
		t.Fatalf("update notification: %+v", update)
	}
	if got := call(module, "DELETE", "/storage-locations/"+id.String(), "", WritePermission); got.Code != 200 ||
		!strings.Contains(got.Body.String(), `"data":{"message":"deleted"}`) {
		t.Fatalf("delete: %d %s", got.Code, got.Body.String())
	}
}

func TestDependenciesAndContract(t *testing.T) {
	if _, err := NewModule(Dependencies{}); err == nil {
		t.Fatal("missing persistence accepted")
	}
	if _, err := NewModule(Dependencies{Database: new(sql.DB), Repository: &memoryRepository{}}); err == nil {
		t.Fatal("ambiguous persistence accepted")
	}
	module, _, _ := newTestModule(t)
	if len(module.Patterns()) != 5 || len(module.OpenAPI()) == 0 {
		t.Fatal("module contract incomplete")
	}
	if Version() == "" {
		t.Fatal("empty version")
	}
}
