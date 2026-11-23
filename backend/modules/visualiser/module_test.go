package visualiser

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type fakeRepository struct {
	datasets     []DatasetResponse
	elements     []DataElementResponse
	values       DataValuesResponse
	err          error
	dataSetIDs   []*string
	valueRequest *DataValuesRequest
}

func (f *fakeRepository) ListDatasets(context.Context) ([]DatasetResponse, error) {
	return f.datasets, f.err
}
func (f *fakeRepository) ListDataElements(_ context.Context, id *string) ([]DataElementResponse, error) {
	f.dataSetIDs = append(f.dataSetIDs, id)
	return f.elements, f.err
}
func (f *fakeRepository) ListDataValues(_ context.Context, req DataValuesRequest) (DataValuesResponse, error) {
	f.valueRequest = &req
	return f.values, f.err
}

type fakeDirectory struct {
	modulekit.HealthContextDirectory
	node       modulekit.HealthContextNode
	aliases    []modulekit.HealthContextAlias
	nodeErr    error
	aliasErr   error
	aliasCalls []bool
}

func (f *fakeDirectory) GetNode(context.Context, string) (modulekit.HealthContextNode, error) {
	return f.node, f.nodeErr
}
func (f *fakeDirectory) ListAliasesInScope(_ context.Context, _ string, descendants bool) ([]modulekit.HealthContextAlias, error) {
	f.aliasCalls = append(f.aliasCalls, descendants)
	return f.aliases, f.aliasErr
}

func newTestModule(t *testing.T, repo *fakeRepository, directory *fakeDirectory, logs *bytes.Buffer) *Module {
	t.Helper()
	deps := Dependencies{Repository: repo, HealthContexts: directory}
	if logs != nil {
		deps.Logger = slog.New(slog.NewTextHandler(logs, nil))
	}
	module, err := NewModule(deps)
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func call(module http.Handler, method, target, body string, selection *modulekit.HealthContextSelection, permissions ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(modulekit.WithRequest(req.Context(), modulekit.Request{
		Authenticated: true, RequestID: "r1", HealthContext: selection,
		Access: core.RequestAccess{Identity: core.Identity{Subject: "u1"}, Permissions: permissions},
	}))
	recorder := httptest.NewRecorder()
	module.ServeHTTP(recorder, req)
	return recorder
}

func TestEndpointsRequireAnyReadPermission(t *testing.T) {
	module := newTestModule(t, &fakeRepository{}, &fakeDirectory{}, nil)
	want := `{"success":false,"error":{"code":"FORBIDDEN","message":"one of these permissions is required: report_browser:read, documents:read, data_quality:read, surveillance:read"},"meta":{"requestId":"r1"}}`
	for _, route := range [][2]string{{"GET", "/visualizer/datasets"}, {"GET", "/visualizer/dataelements"}, {"POST", "/visualizer/datavalues"}} {
		if got := call(module, route[0], route[1], `{}`, nil, "users:read"); got.Code != 403 || got.Body.String() != want {
			t.Errorf("%s %s without permission: %d %s", route[0], route[1], got.Code, got.Body.String())
		}
		for _, permission := range ReadPermissions {
			if got := call(module, route[0], route[1], `{}`, nil, permission); got.Code != 200 {
				t.Errorf("%s %s with %s: %d %s", route[0], route[1], permission, got.Code, got.Body.String())
			}
		}
	}
	req := httptest.NewRequest("GET", "/visualizer/datasets", nil)
	recorder := httptest.NewRecorder()
	module.ServeHTTP(recorder, req)
	if recorder.Code != 401 {
		t.Fatalf("anonymous request: %d", recorder.Code)
	}
}

func TestListEndpointsKeepResponses(t *testing.T) {
	repo := &fakeRepository{}
	module := newTestModule(t, repo, &fakeDirectory{}, nil)
	if got := call(module, "GET", "/visualizer/datasets", "", nil, DataQualityReadPermission); got.Body.String() != `{"success":true,"data":[],"meta":{"requestId":"r1"}}` {
		t.Fatalf("empty datasets: %s", got.Body.String())
	}
	repo.datasets = []DatasetResponse{{DatasetKey: 1, DatasetID: "ds", DisplayName: "DS", IsCurrent: true, CreateDate: "2024-01-01"}}
	if got := call(module, "GET", "/visualizer/datasets", "", nil, DataQualityReadPermission); got.Body.String() !=
		`{"success":true,"data":[{"dataset_key":1,"dataset_id":"ds","display_name":"DS","is_current":true,"create_date":"2024-01-01"}],"meta":{"requestId":"r1"}}` {
		t.Fatalf("datasets: %s", got.Body.String())
	}

	call(module, "GET", "/visualizer/dataelements", "", nil, DataQualityReadPermission)
	call(module, "GET", "/visualizer/dataelements?data_set_id=", "", nil, DataQualityReadPermission)
	call(module, "GET", "/visualizer/dataelements?data_set_id=abc&data_set_id=def", "", nil, DataQualityReadPermission)
	if len(repo.dataSetIDs) != 3 || repo.dataSetIDs[0] != nil || repo.dataSetIDs[1] == nil || *repo.dataSetIDs[1] != "" || *repo.dataSetIDs[2] != "abc" {
		t.Fatalf("data_set_id parsing: %+v", repo.dataSetIDs)
	}
	if got := call(module, "GET", "/visualizer/dataelements", "", nil, DataQualityReadPermission); got.Body.String() != `{"success":true,"data":[],"meta":{"requestId":"r1"}}` {
		t.Fatalf("empty elements: %s", got.Body.String())
	}

	repo.err = errors.New("DWH connection is not configured")
	for _, target := range []string{"/visualizer/datasets", "/visualizer/dataelements"} {
		if got := call(module, "GET", target, "", nil, DataQualityReadPermission); got.Code != 500 ||
			got.Body.String() != `{"success":false,"error":{"code":"INTERNAL_ERROR","message":"request failed"},"meta":{"requestId":"r1"}}` {
			t.Fatalf("%s failure: %d %s", target, got.Code, got.Body.String())
		}
	}
}

func TestDataValuesValidationAndFailures(t *testing.T) {
	repo := &fakeRepository{}
	logs := &bytes.Buffer{}
	module := newTestModule(t, repo, &fakeDirectory{}, logs)
	badJSON := `{"success":false,"error":{"code":"BAD_REQUEST","message":"invalid JSON"},"meta":{"requestId":"r1"}}`
	for _, body := range []string{``, `not json`, `{"aggregationLevel":"5"}`, `{"ou":"x"}`} {
		if got := call(module, "POST", "/visualizer/datavalues", body, nil, DataQualityReadPermission); got.Code != 400 || got.Body.String() != badJSON {
			t.Errorf("%q: %d %s", body, got.Code, got.Body.String())
		}
	}
	got := call(module, "POST", "/visualizer/datavalues", `{"aggregationLevel":"6","dx":["d1"],"orgunitLevel":3}`, nil, DataQualityReadPermission)
	if got.Code != 200 || got.Body.String() != `{"success":true,"data":{"rows":[]},"meta":{"requestId":"r1"}}` {
		t.Fatalf("unscoped data values: %d %s", got.Code, got.Body.String())
	}
	if repo.valueRequest.OU != nil || repo.valueRequest.OrgunitLevel != float64(3) || repo.valueRequest.AggregationLevel != "6" {
		t.Fatalf("request passed through: %+v", repo.valueRequest)
	}
	repo.err = errors.New("boom")
	got = call(module, "POST", "/visualizer/datavalues", `{"dx":["d1"]}`, nil, DataQualityReadPermission)
	if got.Code != 500 || !strings.Contains(got.Body.String(), `"INTERNAL_ERROR"`) {
		t.Fatalf("query failure: %d %s", got.Code, got.Body.String())
	}
	if !strings.Contains(logs.String(), "[VISUALIZER DATAVALUES] request_id=r1 dx=[d1]") || !strings.Contains(logs.String(), "error=boom") {
		t.Fatalf("failure log: %s", logs.String())
	}
}

func TestDataValuesAreScopedToTheSelectedHealthContext(t *testing.T) {
	district := &modulekit.HealthContextSelection{NodeID: "node-1", IncludeDescendants: true}
	nodeOnly := &modulekit.HealthContextSelection{NodeID: "node-1"}
	aliases := []modulekit.HealthContextAlias{
		{Namespace: "dwh-org-unit", ExternalID: " ou-a "},
		{Namespace: "surveillance-facility", ExternalID: "ou-b"},
		{Namespace: "keycloak-group", ExternalID: "ou-c"},
		{Namespace: "dwh-org-unit", ExternalID: "  "},
	}
	mappingRequired := `{"success":false,"error":{"code":"HEALTH_CONTEXT_MAPPING_REQUIRED","message":"selected health context is not mapped to a DWH organisation unit"},"meta":{"requestId":"r1"}}`
	unavailable := `{"success":false,"error":{"code":"HEALTH_CONTEXT_UNAVAILABLE","message":"health context could not be resolved"},"meta":{"requestId":"r1"}}`

	cases := []struct {
		name       string
		selection  *modulekit.HealthContextSelection
		directory  fakeDirectory
		body       string
		status     int
		response   string
		ou         []string
		aliasCalls []bool
	}{
		{name: "no selection", selection: nil, body: `{"ou":["any"]}`, status: 200, ou: []string{"any"}},
		{name: "national with descendants", selection: district, directory: fakeDirectory{node: modulekit.HealthContextNode{Code: "UG"}}, body: `{"ou":["any"]}`, status: 200, ou: []string{"any"}},
		{name: "national node only", selection: nodeOnly, directory: fakeDirectory{node: modulekit.HealthContextNode{Code: "UG"}, aliases: aliases}, body: `{}`, status: 200, ou: []string{"ou-a", "ou-b"}, aliasCalls: []bool{false}},
		{name: "defaults to aliases", selection: district, directory: fakeDirectory{node: modulekit.HealthContextNode{Code: "D1"}, aliases: aliases}, body: `{}`, status: 200, ou: []string{"ou-a", "ou-b"}, aliasCalls: []bool{true}},
		{name: "filters requested units", selection: district, directory: fakeDirectory{aliases: aliases}, body: `{"ou":["ou-b"," ou-a","ou-c"]}`, status: 200, ou: []string{"ou-b", " ou-a"}, aliasCalls: []bool{true}},
		{name: "no requested unit allowed", selection: district, directory: fakeDirectory{aliases: aliases}, body: `{"ou":["ou-c"]}`, status: 403, response: mappingRequired, aliasCalls: []bool{true}},
		{name: "no supported aliases", selection: district, directory: fakeDirectory{aliases: aliases[2:]}, body: `{}`, status: 403, response: mappingRequired, aliasCalls: []bool{true}},
		{name: "empty node id", selection: &modulekit.HealthContextSelection{}, body: `{}`, status: 403, response: mappingRequired},
		{name: "node lookup fails", selection: district, directory: fakeDirectory{nodeErr: modulekit.ErrHealthContextNotFound}, body: `{}`, status: 503, response: unavailable},
		{name: "alias lookup fails", selection: district, directory: fakeDirectory{aliasErr: errors.New("db down")}, body: `{}`, status: 503, response: unavailable, aliasCalls: []bool{true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			directory := tc.directory
			module := newTestModule(t, repo, &directory, nil)
			got := call(module, "POST", "/visualizer/datavalues", tc.body, tc.selection, SurveillanceReadPermission)
			if got.Code != tc.status || (tc.response != "" && got.Body.String() != tc.response) {
				t.Fatalf("got %d %s", got.Code, got.Body.String())
			}
			if !reflect.DeepEqual(directory.aliasCalls, tc.aliasCalls) {
				t.Fatalf("alias calls %v, want %v", directory.aliasCalls, tc.aliasCalls)
			}
			if tc.status != 200 {
				if repo.valueRequest != nil {
					t.Fatal("rejected request reached the warehouse")
				}
				return
			}
			ou := append([]string(nil), repo.valueRequest.OU...)
			if tc.name == "defaults to aliases" || tc.name == "national node only" {
				sort.Strings(ou)
			}
			if !reflect.DeepEqual(ou, tc.ou) {
				t.Fatalf("ou %q, want %q", ou, tc.ou)
			}
		})
	}
}

func TestBuildDataValuesQueryBindsParametersInOrder(t *testing.T) {
	query, values := buildDataValuesQuery(DataValuesRequest{
		OU: []string{"a"}, DX: []string{"d"}, StartDate: "2024-01", EndDate: "2024-03",
		LevelOfCare: []string{"HCIV"}, OrgunitLevel: "3",
	})
	if len(values) != 6 || values[2] != "2024-01" || values[3] != "2024-03" || values[5] != "3" {
		t.Fatalf("values: %#v", values)
	}
	for _, fragment := range []string{"unnest($1::text[])", "hs.data_element_id = ANY($2)", "hs.tperiod >= $3", "hs.tperiod <= $4", "h.level_of_care = ANY($5)", "WHERE su.selected_level = $6", "LEFT JOIN org_unit_attrs oa"} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks %q", fragment)
		}
	}
	query, values = buildDataValuesQuery(DataValuesRequest{PE: []string{"202401"}, OrgunitLevel: float64(2)})
	if len(values) != 1 || !strings.Contains(query, `hs."period" = ANY($1)`) || !strings.Contains(query, "COALESCE(NULLIF(hs.region, ''), 'Unknown Region') AS org_unit_id") {
		t.Fatalf("aggregated query: %v\n%s", values, query)
	}
	if _, err := (&postgresRepository{}).ListDataValues(context.Background(), DataValuesRequest{}); err == nil || err.Error() != "DWH connection is not configured" {
		t.Fatalf("nil warehouse: %v", err)
	}
}

func TestDependenciesAndContract(t *testing.T) {
	if _, err := NewModule(Dependencies{Repository: &fakeRepository{}}); err == nil {
		t.Fatal("missing health-context directory accepted")
	}
	if _, err := NewModule(Dependencies{Database: new(sql.DB), Repository: &fakeRepository{}, HealthContexts: &fakeDirectory{}}); err == nil {
		t.Fatal("ambiguous persistence accepted")
	}
	module, err := NewModule(Dependencies{HealthContexts: &fakeDirectory{}})
	if err != nil {
		t.Fatalf("unconfigured warehouse rejected: %v", err)
	}
	got := call(module, "GET", "/visualizer/datasets", "", nil, ReportBrowserReadPermission)
	if got.Code != 500 {
		t.Fatalf("unconfigured warehouse: %d %s", got.Code, got.Body.String())
	}
	if len(module.Patterns()) != 3 || len(module.OpenAPI()) == 0 {
		t.Fatal("module contract incomplete")
	}
	if Version() == "" {
		t.Fatal("empty version")
	}
}
