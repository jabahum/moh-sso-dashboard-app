package documenttemplates

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates/internal/postgres"
)

// memory implements the three repositories over maps.
type memory struct {
	templates map[uuid.UUID]db.DocumentTemplate
	sheets    map[uuid.UUID]db.DocumentTemplateSheet
	columns   map[uuid.UUID]db.DocumentTemplateColumn
}

func newMemory() *memory {
	return &memory{templates: map[uuid.UUID]db.DocumentTemplate{}, sheets: map[uuid.UUID]db.DocumentTemplateSheet{}, columns: map[uuid.UUID]db.DocumentTemplateColumn{}}
}

type templateRepo struct{ *memory }
type sheetRepo struct{ *memory }
type columnRepo struct{ *memory }

func (m templateRepo) Create(_ context.Context, arg db.CreateDocumentTemplateParams) (db.DocumentTemplate, error) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t := db.DocumentTemplate{ID: arg.ID, DocumentID: arg.DocumentID, Code: arg.Code, Name: arg.Name, Description: arg.Description,
		FileType: arg.FileType, Version: arg.Version, IsActive: arg.IsActive, Configuration: arg.Configuration,
		CreatedBy: arg.CreatedBy, CreatedAt: now, UpdatedAt: now}
	m.templates[t.ID] = t
	return t, nil
}
func (m templateRepo) GetByID(_ context.Context, id uuid.UUID) (db.DocumentTemplate, error) {
	if t, ok := m.templates[id]; ok {
		return t, nil
	}
	return db.DocumentTemplate{}, ErrNotFound
}
func (m templateRepo) GetByCode(_ context.Context, code string) (db.DocumentTemplate, error) {
	for _, t := range m.templates {
		if t.Code == code {
			return t, nil
		}
	}
	return db.DocumentTemplate{}, ErrNotFound
}
func (m templateRepo) GetByDocumentID(context.Context, uuid.UUID) (db.DocumentTemplate, error) {
	return db.DocumentTemplate{}, sql.ErrNoRows
}
func (m templateRepo) List(context.Context) ([]db.DocumentTemplate, error) {
	out := []db.DocumentTemplate{}
	for _, t := range m.templates {
		out = append(out, t)
	}
	return out, nil
}
func (m templateRepo) ListActive(ctx context.Context) ([]db.DocumentTemplate, error) {
	out := []db.DocumentTemplate{}
	for _, t := range m.templates {
		if t.IsActive {
			out = append(out, t)
		}
	}
	return out, nil
}
func (m templateRepo) ListVersions(_ context.Context, code string) ([]db.DocumentTemplate, error) {
	out := []db.DocumentTemplate{}
	for _, t := range m.templates {
		if t.Code == code {
			out = append(out, t)
		}
	}
	return out, nil
}
func (m templateRepo) Update(_ context.Context, arg db.UpdateDocumentTemplateParams) (db.DocumentTemplate, error) {
	t, ok := m.templates[arg.ID]
	if !ok {
		return db.DocumentTemplate{}, sql.ErrNoRows
	}
	t.Name, t.Description, t.FileType, t.Configuration, t.IsActive = arg.Name, arg.Description, arg.FileType, arg.Configuration, arg.IsActive
	m.templates[arg.ID] = t
	return t, nil
}
func (m templateRepo) Archive(_ context.Context, id uuid.UUID) error {
	t := m.templates[id]
	t.ArchivedAt = sql.NullTime{Time: time.Now(), Valid: true}
	m.templates[id] = t
	return nil
}
func (m templateRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.templates, id)
	return nil
}
func (m templateRepo) ExistsCode(ctx context.Context, code string) (bool, error) {
	_, err := m.GetByCode(ctx, code)
	return err == nil, nil
}

func (m sheetRepo) Create(_ context.Context, arg db.CreateDocumentTemplateSheetParams) (db.DocumentTemplateSheet, error) {
	s := db.DocumentTemplateSheet{ID: arg.ID, TemplateID: arg.TemplateID, Code: arg.Code, Name: arg.Name, DisplayName: arg.DisplayName,
		Required: arg.Required, SheetOrder: arg.SheetOrder, HeaderRow: arg.HeaderRow, StartRow: arg.StartRow,
		AllowExtraColumns: arg.AllowExtraColumns, AllowDuplicateHeaders: arg.AllowDuplicateHeaders, Configuration: arg.Configuration}
	m.sheets[s.ID] = s
	return s, nil
}
func (m sheetRepo) GetByID(_ context.Context, id uuid.UUID) (db.DocumentTemplateSheet, error) {
	if s, ok := m.sheets[id]; ok {
		return s, nil
	}
	return db.DocumentTemplateSheet{}, ErrNotFound
}
func (m sheetRepo) GetByCode(context.Context, uuid.UUID, string) (db.DocumentTemplateSheet, error) {
	return db.DocumentTemplateSheet{}, ErrNotFound
}
func (m sheetRepo) List(_ context.Context, templateID uuid.UUID) ([]db.DocumentTemplateSheet, error) {
	out := []db.DocumentTemplateSheet{}
	for _, s := range m.sheets {
		if s.TemplateID == templateID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m sheetRepo) ListRequired(ctx context.Context, templateID uuid.UUID) ([]db.DocumentTemplateSheet, error) {
	return m.List(ctx, templateID)
}
func (m sheetRepo) Update(context.Context, db.UpdateDocumentTemplateSheetParams) (db.DocumentTemplateSheet, error) {
	return db.DocumentTemplateSheet{}, errors.New("unused")
}
func (m sheetRepo) Archive(context.Context, uuid.UUID) error { return nil }
func (m sheetRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.sheets, id)
	return nil
}
func (m sheetRepo) ExistsCode(context.Context, uuid.UUID, string) (bool, error) { return false, nil }
func (m sheetRepo) ExistsName(context.Context, uuid.UUID, string) (bool, error) { return false, nil }

func (m columnRepo) Create(_ context.Context, arg db.CreateDocumentTemplateColumnParams) (db.DocumentTemplateColumn, error) {
	c := db.DocumentTemplateColumn{ID: arg.ID, SheetID: arg.SheetID, ColumnKey: arg.ColumnKey, ColumnName: arg.ColumnName,
		DisplayName: arg.DisplayName, DataType: arg.DataType, Required: arg.Required, IsUnique: arg.IsUnique,
		ColumnOrder: arg.ColumnOrder, DefaultValue: arg.DefaultValue, Configuration: arg.Configuration,
		AllowedValues: arg.AllowedValues, Aliases: arg.Aliases}
	m.columns[c.ID] = c
	return c, nil
}
func (m columnRepo) GetByID(_ context.Context, id uuid.UUID) (db.DocumentTemplateColumn, error) {
	if c, ok := m.columns[id]; ok {
		return c, nil
	}
	return db.DocumentTemplateColumn{}, ErrNotFound
}
func (m columnRepo) GetByKey(context.Context, uuid.UUID, string) (db.DocumentTemplateColumn, error) {
	return db.DocumentTemplateColumn{}, ErrNotFound
}
func (m columnRepo) List(_ context.Context, sheetID uuid.UUID) ([]db.DocumentTemplateColumn, error) {
	out := []db.DocumentTemplateColumn{}
	for _, c := range m.columns {
		if c.SheetID == sheetID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (m columnRepo) ListRequired(ctx context.Context, sheetID uuid.UUID) ([]db.DocumentTemplateColumn, error) {
	return m.List(ctx, sheetID)
}
func (m columnRepo) ListUnique(ctx context.Context, sheetID uuid.UUID) ([]db.DocumentTemplateColumn, error) {
	return m.List(ctx, sheetID)
}
func (m columnRepo) Update(context.Context, db.UpdateDocumentTemplateColumnParams) (db.DocumentTemplateColumn, error) {
	return db.DocumentTemplateColumn{}, errors.New("unused")
}
func (m columnRepo) Archive(context.Context, uuid.UUID) error { return nil }
func (m columnRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.columns, id)
	return nil
}
func (m columnRepo) ExistsKey(context.Context, uuid.UUID, string) (bool, error)  { return false, nil }
func (m columnRepo) ExistsName(context.Context, uuid.UUID, string) (bool, error) { return false, nil }

type recordingNotifier struct{ got []modulekit.Notification }

func (r *recordingNotifier) Notify(_ context.Context, n modulekit.Notification) error {
	r.got = append(r.got, n)
	return nil
}

// warehouse is a database/sql driver answering the has-data query.
type warehouse struct {
	mu   sync.Mutex
	args []driver.Value
}

func (w *warehouse) Open(string) (driver.Conn, error) { return warehouseConn{w}, nil }

type warehouseConn struct{ w *warehouse }

func (c warehouseConn) Prepare(query string) (driver.Stmt, error) { return warehouseStmt(c), nil }
func (c warehouseConn) Close() error                              { return nil }
func (c warehouseConn) Begin() (driver.Tx, error)                 { return nil, errors.New("no transactions") }

type warehouseStmt struct{ w *warehouse }

func (s warehouseStmt) Close() error  { return nil }
func (s warehouseStmt) NumInput() int { return -1 }
func (s warehouseStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("no exec")
}
func (s warehouseStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.w.mu.Lock()
	s.w.args = args
	s.w.mu.Unlock()
	return &warehouseRows{}, nil
}

type warehouseRows struct{ done bool }

func (r *warehouseRows) Columns() []string { return []string{"exists"} }
func (r *warehouseRows) Close() error      { return nil }
func (r *warehouseRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = true
	return nil
}

var registerWarehouse sync.Once
var testWarehouse = &warehouse{}

func newTestModule(t *testing.T) (*Module, *memory, *recordingNotifier) {
	t.Helper()
	registerWarehouse.Do(func() { sql.Register("document-templates-warehouse", testWarehouse) })
	warehouseDB, err := sql.Open("document-templates-warehouse", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = warehouseDB.Close() })
	store := newMemory()
	notifier := &recordingNotifier{}
	module, err := newModule(Dependencies{
		Warehouse: warehouseDB, Notifier: notifier,
		Platform: modulekit.Platform{SystemAdminEmail: "admin@example.org", AdminDashboardURL: "https://portal.example/admin/home"},
	}, repositories{templates: templateRepo{store}, sheets: sheetRepo{store}, columns: columnRepo{store}})
	if err != nil {
		t.Fatal(err)
	}
	return module, store, notifier
}

const testUser = "7b0c1a52-6a4b-4f43-9a0e-3d0c9d8f2a11"

func callAs(module http.Handler, subject, method, target, body string, permissions ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(modulekit.WithRequest(req.Context(), modulekit.Request{
		Authenticated: true, RequestID: "r1",
		Access: core.RequestAccess{Identity: core.Identity{Subject: subject}, Permissions: permissions},
	}))
	recorder := httptest.NewRecorder()
	module.ServeHTTP(recorder, req)
	return recorder
}

func call(module http.Handler, method, target, body string, permissions ...string) *httptest.ResponseRecorder {
	return callAs(module, testUser, method, target, body, permissions...)
}

func TestEndpointsKeepTheirPermissions(t *testing.T) {
	module, _, _ := newTestModule(t)
	if got := call(module, "GET", "/document-templates", ""); got.Code != 403 ||
		got.Body.String() != `{"success":false,"error":{"code":"FORBIDDEN","message":"permission required: document_templates:read"},"meta":{"requestId":"r1"}}` {
		t.Fatalf("read without permission: %d %s", got.Code, got.Body.String())
	}
	id := uuid.NewString()
	for _, write := range [][2]string{
		{"POST", "/document-templates"}, {"POST", "/document-templates/structure"}, {"PUT", "/document-templates/" + id},
		{"DELETE", "/document-templates/" + id}, {"PUT", "/document-templates/" + id + "/structure"},
	} {
		if got := call(module, write[0], write[1], "{}", ReadPermission); got.Code != 403 || !strings.Contains(got.Body.String(), WritePermission) {
			t.Fatalf("%s %s with read permission: %d %s", write[0], write[1], got.Code, got.Body.String())
		}
	}
	for _, path := range []string{"/publish", "/archive"} {
		if got := call(module, "POST", "/document-templates/"+id+path, "", WritePermission); got.Code != 403 || !strings.Contains(got.Body.String(), PublishPermission) {
			t.Fatalf("%s with write permission: %d %s", path, got.Code, got.Body.String())
		}
	}
}

func TestTemplateLifecycleKeepsResponsesAndNotifications(t *testing.T) {
	module, store, notifier := newTestModule(t)

	if got := call(module, "POST", "/document-templates", ``, WritePermission); got.Code != 400 ||
		!strings.Contains(got.Body.String(), `"code":"VALIDATION_FAILED","message":"Invalid request payload"`) {
		t.Fatalf("empty body: %d %s", got.Code, got.Body.String())
	}
	if got := callAs(module, "not-a-uuid", "POST", "/document-templates", `{"code":"X"}`, WritePermission); got.Code != 401 ||
		!strings.Contains(got.Body.String(), `"code":"INVALID_USER"`) {
		t.Fatalf("non-uuid subject: %d %s", got.Code, got.Body.String())
	}

	got := call(module, "POST", "/document-templates", `{"code":" HMIS ","name":"Hmis","file_type":"xlsx","configuration":{"a":1}}`, WritePermission)
	if got.Code != 201 || !strings.Contains(got.Body.String(), `"code":"HMIS"`) || !strings.Contains(got.Body.String(), `"created_by":"`+testUser+`"`) {
		t.Fatalf("create: %d %s", got.Code, got.Body.String())
	}
	created := notifier.got[0]
	if created.Type != "DOCUMENT_TEMPLATE_CREATED" || created.UserID != testUser || len(created.Deliveries) != 0 {
		t.Fatalf("create notification: %+v", created)
	}
	if got := call(module, "POST", "/document-templates", `{"code":"HMIS","name":"Hmis","file_type":"xlsx"}`, WritePermission); got.Code != 500 ||
		!strings.Contains(got.Body.String(), `"code":"CREATE_TEMPLATE_FAILED","message":"Failed to create template"`) {
		t.Fatalf("duplicate create keeps its 500: %d %s", got.Code, got.Body.String())
	}

	structure := `{"template":{"code":"OPD","name":"Opd","file_type":"xlsx"},"sheets":[{"sheet":{"code":"s1","name":"Sheet 1","header_row":1,"start_row":2},"columns":[{"column_key":"k","column_name":"K","data_type":"number"}]}]}`
	got = call(module, "POST", "/document-templates/structure", structure, WritePermission)
	if got.Code != 201 || !strings.Contains(got.Body.String(), `"data_type":"DECIMAL"`) {
		t.Fatalf("create structure: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "POST", "/document-templates/structure", structure, WritePermission); got.Code != 409 ||
		!strings.Contains(got.Body.String(), `"code":"TEMPLATE_CODE_EXISTS"`) {
		t.Fatalf("duplicate structure: %d %s", got.Code, got.Body.String())
	}
	structureCreated := notifier.got[1]
	email := structureCreated.Deliveries[1]
	if structureCreated.Type != "DOCUMENT_TEMPLATE_STRUCTURE_CREATED" || len(structureCreated.Deliveries) != 2 ||
		email.TemplateName != "document-template-structure-created" || email.Recipient["email"] != "admin@example.org" ||
		email.TemplateData["ActionURL"] != "https://portal.example/admin/document-templates" ||
		email.TemplateData["SheetsCount"] != 1 || email.TemplateData["ColumnsCount"] != 1 || email.MaxAttempts != 5 {
		t.Fatalf("structure notification: %+v", structureCreated)
	}

	got = call(module, "GET", "/document-templates/code/OPD/structure", "", ReadPermission)
	var body struct {
		Data TemplateStructureResponse `json:"data"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil || got.Code != 200 || len(body.Data.Sheets) != 1 || len(body.Data.Sheets[0].Columns) != 1 {
		t.Fatalf("structure by code: %d %s", got.Code, got.Body.String())
	}
	templateID := body.Data.Template.ID.String()
	sheetID := body.Data.Sheets[0].ID.String()
	if got := call(module, "GET", "/document-templates/"+templateID+"/sheets/"+sheetID+"/columns", "", ReadPermission); got.Code != 200 ||
		!strings.Contains(got.Body.String(), `"column_key":"k"`) {
		t.Fatalf("columns: %d %s", got.Code, got.Body.String())
	}

	// Route/parameter mismatches preserved from the host handler.
	if got := call(module, "GET", "/document-templates/"+templateID, "", ReadPermission); got.Code != 400 || !strings.Contains(got.Body.String(), "MISSING_TEMPLATE_CODE") {
		t.Fatalf("get by id: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "GET", "/document-templates/"+templateID+"/structure", "", ReadPermission); got.Code != 400 || !strings.Contains(got.Body.String(), "MISSING_TEMPLATE_CODE") {
		t.Fatalf("structure by id: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "GET", "/document-templates/"+templateID+"/sheets", "", ReadPermission); got.Code != 400 || !strings.Contains(got.Body.String(), "INVALID_TEMPLATE_ID") {
		t.Fatalf("sheets: %d %s", got.Code, got.Body.String())
	}

	if got := call(module, "GET", "/document-templates?active=TRUE", "", ReadPermission); got.Code != 200 || got.Body.String() != `{"success":true,"data":[],"meta":{"requestId":"r1"}}` {
		t.Fatalf("active list: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "POST", "/document-templates/"+templateID+"/publish", "", PublishPermission); got.Code != 204 || got.Body.Len() != 0 {
		t.Fatalf("publish: %d %s", got.Code, got.Body.String())
	}
	published := notifier.got[len(notifier.got)-1]
	if published.Type != "DOCUMENT_TEMPLATE_PUBLISHED" || published.Deliveries[1].TemplateName != "document-template-published" {
		t.Fatalf("publish notification: %+v", published)
	}
	if !store.templates[body.Data.Template.ID].IsActive {
		t.Fatal("publish did not activate the template")
	}

	replace := `{"sheets":[{"sheet":{"code":"s2","name":"Sheet 2","header_row":1,"start_row":2},"columns":[]}]}`
	if got := call(module, "PUT", "/document-templates/"+templateID+"/structure", replace, WritePermission); got.Code != 200 || !strings.Contains(got.Body.String(), `"code":"s2"`) {
		t.Fatalf("replace: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "PUT", "/document-templates/"+templateID+"/structure", `{"sheets":[{"sheet":{"code":"","name":"x"}}]}`, WritePermission); got.Code != 500 ||
		!strings.Contains(got.Body.String(), `"code":"REPLACE_FAILED","message":"sheet code is required"`) {
		t.Fatalf("replace failure keeps its message: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "DELETE", "/document-templates/"+templateID, "", WritePermission); got.Code != 204 {
		t.Fatalf("delete: %d %s", got.Code, got.Body.String())
	}
	if got := call(module, "DELETE", "/document-templates/not-a-uuid", "", WritePermission); got.Code != 400 || !strings.Contains(got.Body.String(), "INVALID_TEMPLATE_ID") {
		t.Fatalf("delete invalid id: %d %s", got.Code, got.Body.String())
	}
}

func TestHasDataQueriesTheWarehouse(t *testing.T) {
	module, _, _ := newTestModule(t)
	got := call(module, "GET", "/document-templates/code/%20opd%20/has-data", "", ReadPermission)
	if got.Code != 200 || got.Body.String() != `{"success":true,"data":{"has_data":true},"meta":{"requestId":"r1"}}` {
		t.Fatalf("has-data: %d %s", got.Code, got.Body.String())
	}
	testWarehouse.mu.Lock()
	defer testWarehouse.mu.Unlock()
	if len(testWarehouse.args) != 1 || testWarehouse.args[0] != "OPD" {
		t.Fatalf("has-data argument: %v", testWarehouse.args)
	}
}

func TestDependenciesAndContract(t *testing.T) {
	if _, err := NewModule(Dependencies{Warehouse: new(sql.DB)}); err == nil {
		t.Fatal("missing database accepted")
	}
	if _, err := NewModule(Dependencies{Database: new(sql.DB)}); err == nil {
		t.Fatal("missing warehouse accepted")
	}
	module, err := NewModule(Dependencies{Database: new(sql.DB), Warehouse: new(sql.DB)})
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Patterns()) != 14 || len(module.OpenAPI()) == 0 || module.Service == nil || module.SheetService == nil || module.ColumnService == nil {
		t.Fatal("module contract incomplete")
	}
	if Version() == "" {
		t.Fatal("empty version")
	}
}
