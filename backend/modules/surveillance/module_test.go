package surveillance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

// memoryStore implements the queries the tests exercise; any other query
// panics through the nil embedded Querier.
type memoryStore struct {
	db.Querier
	diseases       []db.Disease
	regions        []db.Region
	regionFilter   []uuid.UUID
	upserted       []db.UpsertDistrictParams
	deleteErr      error
	batches        []db.CreateImportBatchParams
	rawRows        []db.CreateImportRawRowParams
	statusUpdates  []db.UpdateImportBatchStatusParams
	progressNotes  []string
	failedBatches  []db.FailSurveillanceImportBatchParams
	completedBatch []db.CompleteSurveillanceImportBatchParams
	latestBatch    db.SurveillanceImportBatch
}

func (m *memoryStore) ExecTx(ctx context.Context, fn func(db.Querier) error) error { return fn(m) }

func (m *memoryStore) ListDiseases(context.Context) ([]db.Disease, error) { return m.diseases, nil }
func (m *memoryStore) ListRegions(context.Context) ([]db.Region, error)   { return m.regions, nil }
func (m *memoryStore) ListRegionsInHealthContext(_ context.Context, ids []uuid.UUID) ([]db.Region, error) {
	m.regionFilter = ids
	out := []db.Region{}
	for _, region := range m.regions {
		for _, id := range ids {
			if region.ID == id {
				out = append(out, region)
			}
		}
	}
	return out, nil
}
func (m *memoryStore) UpsertDistrict(_ context.Context, arg db.UpsertDistrictParams) (db.District, error) {
	m.upserted = append(m.upserted, arg)
	return db.District{ID: uuid.New(), Name: arg.Name, RegionID: arg.RegionID, Code: arg.Code}, nil
}
func (m *memoryStore) DeleteSubCounty(context.Context, uuid.UUID) error { return m.deleteErr }
func (m *memoryStore) GetSubCountyByID(context.Context, uuid.UUID) (db.SubCounty, error) {
	return db.SubCounty{}, sql.ErrNoRows
}
func (m *memoryStore) CreateImportBatch(_ context.Context, arg db.CreateImportBatchParams) (db.SurveillanceImportBatch, error) {
	m.batches = append(m.batches, arg)
	return db.SurveillanceImportBatch{ID: uuid.New(), SourceName: arg.SourceName, DatasetType: arg.DatasetType, Status: arg.Status, FileName: arg.FileName, Notes: arg.Notes, DocumentID: arg.DocumentID}, nil
}
func (m *memoryStore) CreateImportRawRow(_ context.Context, arg db.CreateImportRawRowParams) (db.SurveillanceImportRawRow, error) {
	m.rawRows = append(m.rawRows, arg)
	return db.SurveillanceImportRawRow{ID: uuid.New(), BatchID: arg.BatchID, RowNumber: arg.RowNumber, Payload: arg.Payload}, nil
}
func (m *memoryStore) UpdateImportBatchProgress(_ context.Context, arg db.UpdateImportBatchProgressParams) error {
	m.progressNotes = append(m.progressNotes, arg.Notes.String)
	return nil
}
func (m *memoryStore) UpdateImportBatchStatus(_ context.Context, arg db.UpdateImportBatchStatusParams) (db.SurveillanceImportBatch, error) {
	m.statusUpdates = append(m.statusUpdates, arg)
	return db.SurveillanceImportBatch{ID: arg.ID, Status: arg.Status, Notes: arg.Notes}, nil
}
func (m *memoryStore) GetLatestImportBatchByDocumentID(context.Context, uuid.NullUUID) (db.SurveillanceImportBatch, error) {
	return m.latestBatch, nil
}
func (m *memoryStore) GetImportBatchByID(context.Context, uuid.UUID) (db.SurveillanceImportBatch, error) {
	return m.latestBatch, nil
}
func (m *memoryStore) FailSurveillanceImportBatch(_ context.Context, arg db.FailSurveillanceImportBatchParams) error {
	m.failedBatches = append(m.failedBatches, arg)
	return nil
}
func (m *memoryStore) CompleteSurveillanceImportBatch(_ context.Context, arg db.CompleteSurveillanceImportBatchParams) error {
	m.completedBatch = append(m.completedBatch, arg)
	return nil
}

// memoryDirectory is a flat health-context hierarchy.
type memoryDirectory struct {
	modulekit.HealthContextDirectory
	nodes    map[string]modulekit.HealthContextNode
	aliases  []modulekit.HealthContextAlias
	failure  error
	parentOf map[string]string
}

func (d *memoryDirectory) GetNode(_ context.Context, id string) (modulekit.HealthContextNode, error) {
	node, ok := d.nodes[id]
	if !ok {
		return modulekit.HealthContextNode{}, modulekit.ErrHealthContextNotFound
	}
	return node, nil
}
func (d *memoryDirectory) ListAliasesInScope(_ context.Context, id string, _ bool) ([]modulekit.HealthContextAlias, error) {
	if d.failure != nil {
		return nil, d.failure
	}
	out := []modulekit.HealthContextAlias{}
	for _, alias := range d.aliases {
		if alias.NodeID == id {
			out = append(out, alias)
		}
	}
	return out, nil
}
func (d *memoryDirectory) ListAncestors(_ context.Context, id string) ([]modulekit.HealthContextNode, error) {
	if parent, ok := d.parentOf[id]; ok {
		return []modulekit.HealthContextNode{d.nodes[parent]}, nil
	}
	return nil, nil
}
func (d *memoryDirectory) ListAliases(_ context.Context, id string) ([]modulekit.HealthContextAlias, error) {
	out := []modulekit.HealthContextAlias{}
	for _, alias := range d.aliases {
		if alias.NodeID == id {
			out = append(out, alias)
		}
	}
	return out, nil
}
func (d *memoryDirectory) ResolveAlias(_ context.Context, namespace, externalID string) (modulekit.HealthContextNode, error) {
	for _, alias := range d.aliases {
		if alias.Namespace == namespace && alias.ExternalID == externalID {
			return d.nodes[alias.NodeID], nil
		}
	}
	return modulekit.HealthContextNode{}, modulekit.ErrHealthContextNotFound
}
func (d *memoryDirectory) IsDescendant(_ context.Context, ancestor, node string) (bool, error) {
	return d.parentOf[node] == ancestor, nil
}

type recordingLimiter struct{ got []modulekit.RateLimit }

func (r *recordingLimiter) limit(_ *http.Request, limit modulekit.RateLimit) (bool, http.Header) {
	r.got = append(r.got, limit)
	return false, http.Header{"X-Ratelimit-Policy": {limit.Name}}
}

func newTestModule(t *testing.T, store *memoryStore, directory *memoryDirectory, deps Dependencies) *Module {
	t.Helper()
	if directory == nil {
		directory = &memoryDirectory{}
	}
	deps.HealthContext = directory
	module, err := newModule(deps, store)
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func reader(permissions ...string) core.RequestAccess {
	return core.RequestAccess{Identity: core.Identity{Subject: "user-1"}, Permissions: permissions, Systems: []string{System}}
}

func serve(t *testing.T, module *Module, method, target, body string, access *core.RequestAccess, selection *modulekit.HealthContextSelection) *httptest.ResponseRecorder {
	t.Helper()
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, payload)
	state := modulekit.Request{RequestID: "req-1", HealthContext: selection}
	if access != nil {
		state.Access, state.Authenticated = *access, true
	}
	request = request.WithContext(modulekit.WithRequest(request.Context(), state))
	response := httptest.NewRecorder()
	module.ServeHTTP(response, request)
	return response
}

func TestNewModuleValidatesDependencies(t *testing.T) {
	directory := &memoryDirectory{}
	for name, deps := range map[string]Dependencies{
		"database":      {HealthContext: directory},
		"health":        {Database: new(sql.DB)},
		"geography":     {Database: new(sql.DB), HealthContext: directory, GeographyPrimary: new(sql.DB)},
		"replicateOnly": {Database: new(sql.DB), HealthContext: directory, ReplicateGeography: func(context.Context) error { return nil }},
	} {
		if _, err := NewModule(deps); err == nil {
			t.Errorf("%s: invalid dependencies accepted", name)
		}
	}
	module, err := NewModule(Dependencies{Database: new(sql.DB), HealthContext: directory, GeographyPrimary: new(sql.DB), ReplicateGeography: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Patterns()) != 27 || !strings.Contains(string(module.OpenAPI()), "/surveillance/imports/{batchID}/raw-rows") {
		t.Fatalf("unexpected routes %v", module.Patterns())
	}
}

func TestEndpointsEnforceAccessInHostOrder(t *testing.T) {
	module := newTestModule(t, &memoryStore{}, nil, Dependencies{})
	if response := serve(t, module, "GET", "/surveillance/diseases", "", nil, nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", response.Code)
	}
	noSystem := core.RequestAccess{Identity: core.Identity{Subject: "user-1"}, Permissions: []string{ReadPermission}}
	if response := serve(t, module, "GET", "/surveillance/diseases", "", &noSystem, nil); response.Code != http.StatusForbidden {
		t.Fatalf("missing system: %d", response.Code)
	}
	access := reader()
	response := serve(t, module, "GET", "/surveillance/diseases", "", &access, nil)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "permission required: surveillance:read") {
		t.Fatalf("missing read permission: %d %s", response.Code, response.Body.String())
	}
	access = reader(ReadPermission)
	for _, write := range []struct{ method, path, permission string }{
		{"POST", "/surveillance/districts", ManageLocationsPermission},
		{"POST", "/surveillance/subcounties", ManageLocationsPermission},
		{"DELETE", "/surveillance/subcounties/" + uuid.NewString(), ManageLocationsPermission},
		{"POST", "/surveillance/imports", ImportPermission},
		{"PATCH", "/surveillance/imports/" + uuid.NewString() + "/status", ImportPermission},
	} {
		response := serve(t, module, write.method, write.path, "{}", &access, nil)
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "permission required: "+write.permission) {
			t.Fatalf("%s %s: %d %s", write.method, write.path, response.Code, response.Body.String())
		}
	}
}

func TestUnavailableObservatoryReturns503AndAutomaticallyEnablesAfterRecovery(t *testing.T) {
	var available atomic.Bool
	store := &memoryStore{regions: []db.Region{}}
	module := newTestModule(t, store, nil, Dependencies{Available: available.Load})
	access := reader(ReadPermission)
	for _, ready := range []bool{false, true, false, true} {
		available.Store(ready)
		response := serve(t, module, "GET", "/surveillance/regions", "", &access, nil)
		if ready {
			if response.Code != http.StatusOK {
				t.Fatalf("recovered request: %d", response.Code)
			}
		} else if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "15" || !strings.Contains(response.Body.String(), "OBSERVATORY_UNAVAILABLE") {
			t.Fatalf("unavailable request: %d %s", response.Code, response.Body.String())
		}
	}
	// The gate runs before the permission check, as the route group did.
	available.Store(false)
	denied := reader()
	if response := serve(t, module, "GET", "/surveillance/regions", "", &denied, nil); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("availability must be checked before permissions: %d", response.Code)
	}
}

func TestListDiseasesAndScopedRegions(t *testing.T) {
	now := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	inside, outside, parent := uuid.New(), uuid.New(), uuid.New()
	node, root := uuid.NewString(), uuid.NewString()
	store := &memoryStore{
		diseases: []db.Disease{{ID: uuid.New(), Name: "Cholera", Code: sql.NullString{String: "CHO", Valid: true}, IsActive: true, CreatedAt: now, UpdatedAt: now}},
		regions:  []db.Region{{ID: inside, Name: "Inside"}, {ID: outside, Name: "Outside"}, {ID: parent, Name: "Parent"}},
	}
	directory := &memoryDirectory{
		nodes:    map[string]modulekit.HealthContextNode{node: {ID: node, Code: "D1"}, root: {ID: root, Code: "UG"}},
		parentOf: map[string]string{node: root},
		aliases: []modulekit.HealthContextAlias{
			{NodeID: node, Namespace: healthContextNamespaceRegion, ExternalID: inside.String()},
			{NodeID: root, Namespace: healthContextNamespaceRegion, ExternalID: parent.String()},
		},
	}
	module := newTestModule(t, store, directory, Dependencies{})
	access := reader(ReadPermission)

	response := serve(t, module, "GET", "/surveillance/diseases", "", &access, nil)
	if response.Code != http.StatusOK || response.Body.String() != `{"success":true,"data":[{"id":"`+store.diseases[0].ID.String()+`","name":"Cholera","code":"CHO","is_active":true,"created_at":"2026-01-05T00:00:00Z","updated_at":"2026-01-05T00:00:00Z"}],"meta":{"requestId":"req-1"}}` {
		t.Fatalf("diseases: %d %s", response.Code, response.Body.String())
	}

	response = serve(t, module, "GET", "/surveillance/regions", "", &access, &modulekit.HealthContextSelection{NodeID: node})
	var body struct {
		Data []RegionResponse `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK {
		t.Fatalf("regions: %d %s", response.Code, response.Body.String())
	}
	if len(body.Data) != 2 || body.Data[0].ID != inside || body.Data[1].ID != parent {
		t.Fatalf("scoped regions must include the node and its ancestors only: %+v", body.Data)
	}

	// A resource outside the selected context is refused before any query.
	response = serve(t, module, "GET", "/surveillance/regions/"+outside.String()+"/districts", "", &access, &modulekit.HealthContextSelection{NodeID: node})
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "resource is outside the selected health context") {
		t.Fatalf("outside region: %d %s", response.Code, response.Body.String())
	}
}

func TestManageEndpointsApplyRateLimitAndKeepErrors(t *testing.T) {
	limiter := &recordingLimiter{}
	store := &memoryStore{deleteErr: ErrDeletionRestricted}
	module := newTestModule(t, store, nil, Dependencies{Limiter: limiter.limit})
	access := reader(ManageLocationsPermission, ImportPermission)
	response := serve(t, module, "POST", "/surveillance/districts", `{"name":"Gulu"}`, &access, nil)
	if response.Code != http.StatusTooManyRequests || len(limiter.got) != 1 || limiter.got[0] != ManageRateLimit || response.Header().Get("X-Ratelimit-Policy") != "surveillance-manage" {
		t.Fatalf("manage limit: %d %v", response.Code, limiter.got)
	}
	if serve(t, module, "POST", "/surveillance/imports", `{}`, &access, nil); limiter.got[1] != ImportRateLimit {
		t.Fatalf("import limit: %v", limiter.got)
	}
	if ManageRateLimit.Limit != 10 || ManageRateLimit.Window != time.Minute || ImportRateLimit.Limit != 10 || ImportRateLimit.Window != time.Minute {
		t.Fatal("rate limit policies changed")
	}

	notifier := &recordingNotifier{}
	module = newTestModule(t, store, nil, Dependencies{Notifier: notifier})
	response = serve(t, module, "POST", "/surveillance/districts", `{"code":"G"}`, &access, nil)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid request body"`) {
		t.Fatalf("missing name: %d %s", response.Code, response.Body.String())
	}
	response = serve(t, module, "POST", "/surveillance/districts", `{"name":" Gulu ","code":"G","region_id":"`+uuid.NewString()+`"}`, &access, nil)
	if response.Code != http.StatusOK || len(store.upserted) != 1 || store.upserted[0].Name != "Gulu" || store.upserted[0].Code.String != "G" ||
		len(notifier.got) != 1 || notifier.got[0].Type != "SURVEILLANCE_DISTRICT_UPSERTED" || notifier.got[0].TargetRole != "admin" {
		t.Fatalf("upsert district: %d %s", response.Code, response.Body.String())
	}
	response = serve(t, module, "POST", "/surveillance/subcounties", `{"name":"Laroo","district_id":"00000000-0000-0000-0000-000000000000"}`, &access, nil)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "district_id is required") {
		t.Fatalf("nil district: %d %s", response.Code, response.Body.String())
	}
	response = serve(t, module, "DELETE", "/surveillance/subcounties/"+uuid.NewString(), "", &access, nil)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "GEOGRAPHY_DELETE_RESTRICTED") {
		t.Fatalf("restricted delete: %d %s", response.Code, response.Body.String())
	}
	response = serve(t, module, "POST", "/surveillance/imports", `{"source_name":"manual","dataset_type":"alerts"}`, &access, nil)
	if response.Code != http.StatusCreated || len(store.batches) != 1 || store.batches[0].Status != "pending" {
		t.Fatalf("create batch: %d %s", response.Code, response.Body.String())
	}
}

type memoryImportHost struct {
	csv      string
	progress []string
	queued   [][2]uuid.UUID
}

func (h *memoryImportHost) GetDocument(context.Context, uuid.UUID) (ImportDocument, error) {
	return ImportDocument{ObjectKey: "uploads/metrics.csv", OriginalFilename: " metrics.csv "}, nil
}
func (h *memoryImportHost) Download(_ context.Context, key string) (io.ReadCloser, error) {
	if key != "uploads/metrics.csv" {
		return nil, errors.New("unexpected key")
	}
	return io.NopCloser(strings.NewReader(h.csv)), nil
}
func (h *memoryImportHost) UpdateProgress(_ context.Context, _ uuid.UUID, progress int32, message *string) error {
	h.progress = append(h.progress, *message)
	return nil
}
func (h *memoryImportHost) QueueBatchProcess(_ context.Context, documentID, createdBy uuid.UUID) error {
	h.queued = append(h.queued, [2]uuid.UUID{documentID, createdBy})
	return nil
}

func TestImportCSVStoresRawRowsAndQueuesBatchProcessing(t *testing.T) {
	store := &memoryStore{}
	host := &memoryImportHost{csv: "Record ID,Facility,Region,District,Sub County,Disease,Value,Year,Weeks\nr1,Clinic,North,Gulu,Laroo,Cholera,4,2026,1\nr2,Clinic,North,Gulu,Laroo,Cholera,5,2026,2\n"}
	module := newTestModule(t, store, nil, Dependencies{Imports: host})
	job := ImportJob{ProcessID: uuid.New(), DocumentID: uuid.New(), CreatedBy: uuid.New()}
	if err := module.ImportCSV(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if len(store.batches) != 1 || store.batches[0].DatasetType != "facility_metrics" || store.batches[0].FileName.String != "metrics.csv" ||
		store.batches[0].ImportedBy.String != job.CreatedBy.String() || store.batches[0].DocumentID.UUID != job.DocumentID {
		t.Fatalf("batch: %+v", store.batches)
	}
	if len(store.rawRows) != 2 || string(store.rawRows[0].Payload) != `{"disease":"Cholera","district":"Gulu","facility":"Clinic","record_id":"r1","region":"North","sub_county":"Laroo","value":"4","weeks":"1","year":"2026"}` {
		t.Fatalf("raw rows: %d %s", len(store.rawRows), store.rawRows[0].Payload)
	}
	if len(store.statusUpdates) != 1 || store.statusUpdates[0].Status != "PENDING" {
		t.Fatalf("status: %+v", store.statusUpdates)
	}
	if len(host.queued) != 1 || host.queued[0] != [2]uuid.UUID{job.DocumentID, job.CreatedBy} || host.progress[len(host.progress)-1] != "Raw import completed" {
		t.Fatalf("queue/progress: %v %v", host.queued, host.progress)
	}

	if err := newTestModule(t, store, nil, Dependencies{}).ImportCSV(context.Background(), job); err == nil {
		t.Fatal("CSV import without a host succeeded")
	}
}

func TestProcessImportBatchRejectsUnknownDatasetType(t *testing.T) {
	store := &memoryStore{latestBatch: db.SurveillanceImportBatch{ID: uuid.New(), DatasetType: "district_status"}}
	module := newTestModule(t, store, nil, Dependencies{})
	err := module.ProcessImportBatch(context.Background(), ImportJob{DocumentID: uuid.New()})
	if err == nil || err.Error() != "unsupported dataset type: district_status" || len(store.failedBatches) != 1 {
		t.Fatalf("unexpected result: %v %+v", err, store.failedBatches)
	}
	if err := module.ProcessImportBatch(context.Background(), ImportJob{}); err == nil || err.Error() != "batch_id is required" {
		t.Fatalf("missing document: %v", err)
	}
}

type recordingNotifier struct{ got []modulekit.Notification }

func (r *recordingNotifier) Notify(_ context.Context, n modulekit.Notification) error {
	r.got = append(r.got, n)
	return nil
}
