package documents

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	"github.com/xuri/excelize/v2"
)

type memoryRepository struct {
	mu        sync.Mutex
	docs      map[uuid.UUID]Document
	processes []Process
	pending   []Process
	scopes    []HealthContextScope
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{docs: map[uuid.UUID]Document{}}
}

func (m *memoryRepository) CreateDocument(_ context.Context, arg NewDocument) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc := Document{
		ID: arg.ID, OriginalFilename: arg.OriginalFilename, ContentType: arg.ContentType, SizeBytes: arg.SizeBytes,
		ChecksumSha256: arg.ChecksumSha256, StorageLocationID: arg.StorageLocationID, ObjectKey: arg.ObjectKey,
		UploadedBy: arg.UploadedBy, Status: arg.Status, Metadata: arg.Metadata, IsTemplate: arg.IsTemplate,
		HealthContextID: arg.HealthContextID, CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
	m.docs[doc.ID] = doc
	return doc, nil
}
func (m *memoryRepository) GetDocument(_ context.Context, id uuid.UUID) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if doc, ok := m.docs[id]; ok {
		return doc, nil
	}
	return Document{}, sql.ErrNoRows
}
func (m *memoryRepository) GetDocumentInHealthContext(ctx context.Context, id uuid.UUID, scope HealthContextScope) (Document, error) {
	m.mu.Lock()
	m.scopes = append(m.scopes, scope)
	m.mu.Unlock()
	doc, err := m.GetDocument(ctx, id)
	if err != nil || !doc.HealthContextID.Valid || doc.HealthContextID.UUID != scope.ID {
		return Document{}, sql.ErrNoRows
	}
	return doc, nil
}
func (m *memoryRepository) ListDocuments(context.Context, Pagination) ([]Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Document{}
	for _, doc := range m.docs {
		if !doc.IsTemplate {
			out = append(out, doc)
		}
	}
	return out, nil
}
func (m *memoryRepository) ListDocumentsInHealthContext(_ context.Context, scope HealthContextScope, _ Pagination) ([]Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scopes = append(m.scopes, scope)
	out := []Document{}
	for _, doc := range m.docs {
		if doc.HealthContextID.Valid && doc.HealthContextID.UUID == scope.ID {
			out = append(out, doc)
		}
	}
	return out, nil
}
func (m *memoryRepository) EditDocument(_ context.Context, arg DocumentEdit) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.docs[arg.ID]
	if !ok {
		return Document{}, sql.ErrNoRows
	}
	doc.OriginalFilename, doc.ContentType = arg.OriginalFilename, arg.ContentType
	m.docs[arg.ID] = doc
	return doc, nil
}
func (m *memoryRepository) mark(id uuid.UUID, status string) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.docs[id]
	if !ok {
		return Document{}, sql.ErrNoRows
	}
	doc.Status = status
	m.docs[id] = doc
	return doc, nil
}
func (m *memoryRepository) MarkDocumentPending(_ context.Context, id uuid.UUID) (Document, error) {
	return m.mark(id, DocumentStatusPending)
}
func (m *memoryRepository) MarkDocumentProcessing(_ context.Context, id uuid.UUID) (Document, error) {
	return m.mark(id, DocumentStatusProcessing)
}
func (m *memoryRepository) MarkDocumentCompleted(_ context.Context, id uuid.UUID) (Document, error) {
	return m.mark(id, DocumentStatusCompleted)
}
func (m *memoryRepository) MarkDocumentFailed(_ context.Context, id uuid.UUID) (Document, error) {
	return m.mark(id, DocumentStatusFailed)
}
func (m *memoryRepository) DeleteDocument(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, id)
	return nil
}
func (m *memoryRepository) Stats(context.Context, *HealthContextScope) (Stats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Stats{Total: int64(len(m.docs))}, nil
}
func (m *memoryRepository) CreateProcess(_ context.Context, arg NewProcess) (Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := Process{ID: arg.ID, DocumentID: arg.DocumentID, ProcessType: arg.ProcessType, Status: "PENDING", CreatedBy: arg.CreatedBy}
	m.processes = append(m.processes, p)
	m.pending = append(m.pending, p)
	return p, nil
}
func (m *memoryRepository) ListProcessesByDocument(_ context.Context, documentID uuid.UUID) ([]Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Process{}
	for _, p := range m.processes {
		if p.DocumentID == documentID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (m *memoryRepository) GetLatestProcessByDocumentID(_ context.Context, documentID uuid.UUID) (Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.processes) - 1; i >= 0; i-- {
		if m.processes[i].DocumentID == documentID {
			return m.processes[i], nil
		}
	}
	return Process{}, sql.ErrNoRows
}
func (m *memoryRepository) ClaimNextPendingProcess(context.Context) (Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pending) == 0 {
		return Process{}, sql.ErrNoRows
	}
	p := m.pending[0]
	m.pending = m.pending[1:]
	p.Status = ProcessStatusProcessing
	return p, nil
}
func (m *memoryRepository) TemplateSheetColumns(context.Context, string, string) (string, []string, error) {
	return "", nil, nil
}

type fakeLocations struct{ provider string }

func (f fakeLocations) GetByID(_ context.Context, id string) (Location, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return Location{}, err
	}
	return Location{ID: parsed, Provider: f.provider}, nil
}

type memoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *memoryStore) Upload(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	return nil
}
func (s *memoryStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
func (s *memoryStore) GetObjectURL(_ context.Context, key string) (string, error) {
	return "mem://" + key, nil
}
func (s *memoryStore) GetDownloadURL(_ context.Context, key, _ string) (string, error) {
	return "mem://" + key + "?download", nil
}
func (s *memoryStore) GetViewURL(_ context.Context, key, _ string) (string, error) {
	return "mem://" + key + "?view", nil
}

type fakeStores struct{ store *memoryStore }

func (f fakeStores) Get(provider string) (FileStore, error) {
	if provider != "memory" {
		return nil, errors.New("unsupported provider")
	}
	return f.store, nil
}

type recordingNotifier struct {
	mu  sync.Mutex
	got []modulekit.Notification
}

func (r *recordingNotifier) Notify(_ context.Context, n modulekit.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, n)
	return nil
}

type fixture struct {
	module   *Module
	repo     *memoryRepository
	store    *memoryStore
	notifier *recordingNotifier
}

func newFixture(t *testing.T, deps Dependencies) fixture {
	t.Helper()
	f := fixture{repo: newMemoryRepository(), store: &memoryStore{objects: map[string][]byte{}}, notifier: &recordingNotifier{}}
	deps.Repository = f.repo
	deps.StorageLocations = fakeLocations{provider: "memory"}
	deps.FileStores = fakeStores{store: f.store}
	deps.Notifier = f.notifier
	deps.Platform = modulekit.Platform{SystemAdminEmail: "admin@example.org", AdminDashboardURL: "https://portal.example/admin/home"}
	module, err := NewModule(deps)
	if err != nil {
		t.Fatal(err)
	}
	f.module = module
	return f
}

const userID = "6b0f1f38-2b4c-4bb3-9d0c-1f2a2f6f3f01"

type requestOption func(*modulekit.Request)

func withContext(id string, descendants bool) requestOption {
	return func(r *modulekit.Request) {
		r.HealthContext = &modulekit.HealthContextSelection{NodeID: id, IncludeDescendants: descendants}
	}
}

func serve(handler http.Handler, req *http.Request, permissions []string, options ...requestOption) *httptest.ResponseRecorder {
	request := modulekit.Request{
		Authenticated: true, RequestID: "r1",
		Access: core.RequestAccess{Identity: core.Identity{Subject: userID}, Permissions: permissions},
	}
	for _, option := range options {
		option(&request)
	}
	req = req.WithContext(modulekit.WithRequest(req.Context(), request))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func call(handler http.Handler, method, target, body string, permissions ...string) *httptest.ResponseRecorder {
	return serve(handler, httptest.NewRequest(method, target, strings.NewReader(body)), permissions)
}

func multipartUpload(t *testing.T, target string, fields map[string]string, filename, contentType string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
		if contentType != "" {
			header.Set("Content-Type", contentType)
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(content)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || !envelope.Success {
		t.Fatalf("unexpected body %d %s", recorder.Code, recorder.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatal(err)
	}
}

func TestEndpointsKeepTheirPermissions(t *testing.T) {
	f := newFixture(t, Dependencies{})
	id := uuid.NewString()
	cases := []struct{ method, target, permission string }{
		{"GET", "/documents", ReadPermission},
		{"GET", "/documents/stats", ReadPermission},
		{"POST", "/documents", WritePermission},
		{"POST", "/documents/scan-structure", WritePermission},
		{"GET", "/documents/" + id, ReadPermission},
		{"PUT", "/documents/" + id, WritePermission},
		{"DELETE", "/documents/" + id, WritePermission},
		{"GET", "/documents/" + id + "/processes", ReadPermission},
		{"POST", "/documents/" + id + "/reprocess", ProcessPermission},
		{"GET", "/documents/" + id + "/data-preview", ReadPermission},
		{"POST", "/documents/" + id + "/data-preview/export", ReadPermission},
		{"GET", "/documents/" + id + "/parse-structure", ReadPermission},
		{"GET", "/documents/files/" + id + "/view", ReadPermission},
		{"GET", "/documents/files/" + id + "/download", ReadPermission},
	}
	for _, tc := range cases {
		got := call(f.module, tc.method, tc.target, "")
		want := `{"success":false,"error":{"code":"FORBIDDEN","message":"permission required: ` + tc.permission + `"},"meta":{"requestId":"r1"}}`
		if got.Code != http.StatusForbidden || got.Body.String() != want {
			t.Errorf("%s %s: %d %s", tc.method, tc.target, got.Code, got.Body.String())
		}
	}
	if len(f.module.Patterns()) != len(cases) {
		t.Fatalf("expected %d routes, got %d", len(cases), len(f.module.Patterns()))
	}
}

func TestUploadQueuesProcessingAndServesTheFile(t *testing.T) {
	f := newFixture(t, Dependencies{})
	contextID := uuid.NewString()
	location := uuid.NewString()

	// Validation failures keep their codes.
	for name, tc := range map[string]struct {
		fields   map[string]string
		filename string
		content  string
		code     string
	}{
		"location": {map[string]string{"storage_location": "nope"}, "a.pdf", "x", "INVALID_STORAGE_LOCATION"},
		"metadata": {map[string]string{"storage_location": location, "metadata": "{"}, "a.pdf", "x", "INVALID_METADATA"},
		"file":     {map[string]string{"storage_location": location}, "", "", "FILE_REQUIRED"},
		"empty":    {map[string]string{"storage_location": location}, "a.pdf", "", "EMPTY_FILE"},
		"process":  {map[string]string{"storage_location": location}, "a.csv", "a,b", "PROCESS_TYPE_REQUIRED"},
		"type":     {map[string]string{"storage_location": location, "process_type": "NOPE"}, "a.csv", "a,b", "INVALID_PROCESS_TYPE"},
	} {
		got := serve(f.module, multipartUpload(t, "/documents", tc.fields, tc.filename, "", []byte(tc.content)), []string{WritePermission})
		if got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%s: %d %s", name, got.Code, got.Body.String())
		}
	}
	if got := call(f.module, "POST", "/documents", "x", WritePermission); got.Code != 400 || !strings.Contains(got.Body.String(), "INVALID_MULTIPART") {
		t.Fatalf("non-multipart body: %d %s", got.Code, got.Body.String())
	}

	req := multipartUpload(t, "/documents", map[string]string{
		"storage_location": location, "process_type": "CSV_IMPORT", "metadata": `{"template_code":" t1 "}`,
	}, "data.csv", "text/csv", []byte("a,b\n1,2\n"))
	got := serve(f.module, req, []string{WritePermission}, withContext(contextID, false))
	if got.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", got.Code, got.Body.String())
	}
	var created DocumentResponse
	decode(t, got, &created)
	if created.Status != DocumentStatusPending || created.ContentType != "text/csv" || created.ChecksumSHA256 == nil ||
		created.Metadata["template_code"] != "t1" || created.HealthContextID == nil || created.HealthContextID.String() != contextID ||
		created.UploadedBy.String() != userID || created.ObjectURL != "mem://"+created.ObjectKey {
		t.Fatalf("created document: %+v", created)
	}
	if len(f.repo.processes) != 1 || f.repo.processes[0].ProcessType != "CSV_IMPORT" {
		t.Fatalf("queued processes: %+v", f.repo.processes)
	}
	if n := f.notifier.got; len(n) != 1 || n[0].Type != "DOCUMENT_CREATED" || n[0].Message != "Document created and queued for processing" ||
		len(n[0].Deliveries) != 2 || n[0].Deliveries[1].TemplateData["ActionURL"] != "https://portal.example/admin/documents" {
		t.Fatalf("create notification: %+v", n)
	}

	// Scoped reads only see documents in the selected context.
	if got := serve(f.module, httptest.NewRequest("GET", "/documents/"+created.ID.String(), nil), []string{ReadPermission}, withContext(uuid.NewString(), true)); got.Code != 404 {
		t.Fatalf("out-of-scope read: %d", got.Code)
	}
	if scope := f.repo.scopes[len(f.repo.scopes)-1]; !scope.IncludeDescendants {
		t.Fatalf("descendant scope lost: %+v", scope)
	}
	if got := serve(f.module, httptest.NewRequest("GET", "/documents/"+created.ID.String(), nil), []string{ReadPermission}, withContext(contextID, false)); got.Code != 200 {
		t.Fatalf("in-scope read: %d %s", got.Code, got.Body.String())
	}

	download := call(f.module, "GET", "/documents/files/"+created.ID.String()+"/download", "", ReadPermission)
	if download.Code != 200 || download.Body.String() != "a,b\n1,2\n" ||
		download.Header().Get("Content-Disposition") != `attachment; filename="data.csv"` || download.Header().Get("Content-Type") != "text/csv" {
		t.Fatalf("download: %d %v %q", download.Code, download.Header(), download.Body.String())
	}
	view := call(f.module, "GET", "/documents/files/"+created.ID.String()+"/view", "", ReadPermission)
	if view.Code != 200 || view.Header().Get("Content-Disposition") != `inline; filename="data.csv"` {
		t.Fatalf("view: %d %v", view.Code, view.Header())
	}
	if got := call(f.module, "GET", "/documents/files/nope/view", "", ReadPermission); got.Code != 400 || !strings.Contains(got.Body.String(), "INVALID_UUID") {
		t.Fatalf("invalid view id: %d %s", got.Code, got.Body.String())
	}

	// Edit, list, processes, reprocess, delete.
	if got := call(f.module, "PUT", "/documents/"+created.ID.String(), `{"original_filename":"x.csv"}`, WritePermission); got.Code != 400 ||
		!strings.Contains(got.Body.String(), "original_filename and content_type are required") {
		t.Fatalf("partial edit: %d %s", got.Code, got.Body.String())
	}
	if got := call(f.module, "PUT", "/documents/"+created.ID.String(), `{"original_filename":"x.csv","content_type":"text/csv"}`, WritePermission); got.Code != 200 ||
		!strings.Contains(got.Body.String(), `"original_filename":"x.csv"`) {
		t.Fatalf("edit: %d %s", got.Code, got.Body.String())
	}
	var listed []DocumentResponse
	decode(t, call(f.module, "GET", "/documents?limit=5", "", ReadPermission), &listed)
	if len(listed) != 1 || listed[0].DownloadURL == "" {
		t.Fatalf("list: %+v", listed)
	}
	var processes []ProcessResponse
	decode(t, call(f.module, "GET", "/documents/"+created.ID.String()+"/processes", "", ReadPermission), &processes)
	if len(processes) != 1 || processes[0].Status != "PENDING" {
		t.Fatalf("processes: %+v", processes)
	}
	if got := call(f.module, "POST", "/documents/"+created.ID.String()+"/reprocess", "", ProcessPermission); got.Code != 200 ||
		got.Body.String() != `{"success":true,"data":{"message":"reprocessing started"},"meta":{"requestId":"r1"}}` {
		t.Fatalf("reprocess: %d %s", got.Code, got.Body.String())
	}
	if got := call(f.module, "POST", "/documents/nope/reprocess", "", ProcessPermission); got.Code != 400 ||
		got.Body.String() != `{"success":false,"error":{"code":"error","message":"invalid document id"},"meta":{"requestId":"r1"}}` {
		t.Fatalf("reprocess invalid id: %d %s", got.Code, got.Body.String())
	}
	if got := call(f.module, "GET", "/documents/stats", "", ReadPermission); got.Code != 200 || !strings.Contains(got.Body.String(), `"total":1`) {
		t.Fatalf("stats: %d %s", got.Code, got.Body.String())
	}
	if got := call(f.module, "GET", "/documents/"+created.ID.String()+"/data-preview", "", ReadPermission); got.Code != 503 || !strings.Contains(got.Body.String(), "NO_REMOTE_DB") {
		t.Fatalf("preview without upload database: %d %s", got.Code, got.Body.String())
	}
	if got := call(f.module, "DELETE", "/documents/"+created.ID.String(), "", WritePermission); got.Code != 200 ||
		!strings.Contains(got.Body.String(), `"message":"document deleted successfully"`) {
		t.Fatalf("delete: %d %s", got.Code, got.Body.String())
	}
}

func TestUploadOfUnknownProviderAndPlainFiles(t *testing.T) {
	f := newFixture(t, Dependencies{})
	module, err := NewModule(Dependencies{
		Repository: f.repo, StorageLocations: fakeLocations{provider: "ftp"}, FileStores: fakeStores{store: f.store},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := multipartUpload(t, "/documents", map[string]string{"storage_location": uuid.NewString()}, "guide.pdf", "application/pdf", []byte("%PDF"))
	if got := serve(module, req, []string{WritePermission}); got.Code != 400 || !strings.Contains(got.Body.String(), "INVALID_PROVIDER") {
		t.Fatalf("unknown provider: %d %s", got.Code, got.Body.String())
	}

	f = newFixture(t, Dependencies{})
	req = multipartUpload(t, "/documents", map[string]string{"storage_location": uuid.NewString()}, "guide.pdf", "application/pdf", []byte("%PDF"))
	got := serve(f.module, req, []string{WritePermission})
	var created DocumentResponse
	decode(t, got, &created)
	if got.Code != 201 || created.Status != DocumentStatusCompleted || len(f.repo.processes) != 0 || created.HealthContextID != nil {
		t.Fatalf("plain upload: %d %+v", got.Code, created)
	}
	if n := f.notifier.got; len(n) != 1 || n[0].Message != "Document created" || len(n[0].Deliveries) != 0 {
		t.Fatalf("plain upload notification: %+v", n)
	}
}

func spreadsheet(t *testing.T) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer book.Close()
	_ = book.SetSheetRow("Sheet1", "A1", &[]any{"Title"})
	_ = book.SetSheetRow("Sheet1", "A3", &[]any{"Facility Name", "District", "Total (Cases)", "Total (Cases)"})
	var out bytes.Buffer
	if err := book.Write(&out); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestScanStructureDetectsHeaders(t *testing.T) {
	f := newFixture(t, Dependencies{})
	if got := serve(f.module, multipartUpload(t, "/documents/scan-structure", nil, "a.pdf", "", []byte("x")), []string{WritePermission}); got.Code != 400 ||
		!strings.Contains(got.Body.String(), "UNSUPPORTED_FORMAT") {
		t.Fatalf("pdf scan: %d %s", got.Code, got.Body.String())
	}
	if got := serve(f.module, multipartUpload(t, "/documents/scan-structure", nil, "", "", nil), []string{WritePermission}); got.Code != 400 ||
		!strings.Contains(got.Body.String(), "NO_FILE") {
		t.Fatalf("missing file: %d %s", got.Code, got.Body.String())
	}
	got := serve(f.module, multipartUpload(t, "/documents/scan-structure", nil, "book.xlsx", "", spreadsheet(t)), []string{WritePermission})
	want := `{"success":true,"data":{"sheets":[{"name":"Sheet1","header_row":3,"start_row":4,"columns":[{"column_key":"facility_name","column_name":"Facility Name"},{"column_key":"district","column_name":"District"},{"column_key":"total_cases","column_name":"Total (Cases)"},{"column_key":"total_cases_2","column_name":"Total (Cases)"}]}]},"meta":{"requestId":"r1"}}`
	if got.Code != 200 || got.Body.String() != want {
		t.Fatalf("scan: %d %s", got.Code, got.Body.String())
	}
}

type recordingLimiter struct{ names []string }

func (r *recordingLimiter) limit(_ *http.Request, limit modulekit.RateLimit) (bool, http.Header) {
	r.names = append(r.names, limit.Name)
	return false, nil
}

func TestRateLimitsApplyToUploadAndReprocess(t *testing.T) {
	limiter := &recordingLimiter{}
	f := newFixture(t, Dependencies{Limiter: limiter.limit})
	if got := call(f.module, "POST", "/documents", "", WritePermission); got.Code != 429 {
		t.Fatalf("upload limit: %d", got.Code)
	}
	if got := call(f.module, "POST", "/documents/"+uuid.NewString()+"/reprocess", "", ProcessPermission); got.Code != 429 {
		t.Fatalf("reprocess limit: %d", got.Code)
	}
	if got := call(f.module, "GET", "/documents", "", ReadPermission); got.Code != 200 {
		t.Fatalf("unlimited read: %d", got.Code)
	}
	if strings.Join(limiter.names, ",") != "documents-write,documents-process" || WriteRateLimit.Limit != 30 || ProcessRateLimit.Limit != 10 {
		t.Fatalf("policies: %v", limiter.names)
	}
}

type executorFunc func(context.Context, uuid.UUID) error

func (f executorFunc) Execute(ctx context.Context, id uuid.UUID) error { return f(ctx, id) }

func TestWorkerExecutesClaimedJobsAndStops(t *testing.T) {
	executed := make(chan uuid.UUID, 4)
	var calls atomic.Int32
	f := newFixture(t, Dependencies{PollInterval: time.Millisecond, Executor: executorFunc(func(_ context.Context, id uuid.UUID) error {
		executed <- id
		if calls.Add(1) == 1 {
			panic("processor bug")
		}
		return nil
	})})
	first, _ := f.repo.CreateProcess(context.Background(), NewProcess{ID: uuid.New(), DocumentID: uuid.New(), ProcessType: "CSV_IMPORT"})
	second, _ := f.repo.CreateProcess(context.Background(), NewProcess{ID: uuid.New(), DocumentID: uuid.New(), ProcessType: "CSV_IMPORT"})
	stop, err := f.module.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []uuid.UUID{first.ID, second.ID} {
		select {
		case got := <-executed:
			if got != want {
				t.Fatalf("executed %s, want %s", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("job not executed; a panicking job must not stop the worker")
		}
	}
	if err := stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDependenciesAndContract(t *testing.T) {
	repo := newMemoryRepository()
	locations, stores := fakeLocations{}, fakeStores{}
	for name, deps := range map[string]Dependencies{
		"no persistence": {StorageLocations: locations, FileStores: stores},
		"ambiguous":      {Database: new(sql.DB), Repository: repo, StorageLocations: locations, FileStores: stores},
		"no locations":   {Repository: repo, FileStores: stores},
		"no stores":      {Repository: repo, StorageLocations: locations},
	} {
		if _, err := NewModule(deps); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	module, err := NewModule(Dependencies{Repository: repo, StorageLocations: locations, FileStores: stores})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.Initialize(context.Background()); err == nil {
		t.Fatal("worker started without a process executor")
	}
	if len(module.OpenAPI()) == 0 || Version() == "" {
		t.Fatal("module contract incomplete")
	}
}
