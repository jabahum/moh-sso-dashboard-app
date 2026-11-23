package dataquality

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// fakeService records what the handlers pass to the issue service.
type fakeService struct {
	Service
	created    []createIssueInput
	scopes     []healthContextScope
	filters    []listIssuesFilter
	assigned   []assignIssuesInput
	notFound   bool
	issueCount int64
}

func (f *fakeService) CreateIssue(_ context.Context, input createIssueInput, scope healthContextScope) (issueResponse, error) {
	f.created = append(f.created, input)
	f.scopes = append(f.scopes, scope)
	code := "ISS-1"
	return issueResponse{IssueID: 1, IssueCode: &code, DateReported: "2026-01-02"}, nil
}

func (f *fakeService) ListIssues(_ context.Context, filter listIssuesFilter) ([]issueResponse, error) {
	f.filters = append(f.filters, filter)
	return nil, nil
}

func (f *fakeService) CountIssues(context.Context, listIssuesFilter) (int64, error) {
	return f.issueCount, nil
}

func (f *fakeService) GetIssueByCode(_ context.Context, _ string, scope healthContextScope) (issueResponse, error) {
	f.scopes = append(f.scopes, scope)
	if f.notFound {
		return issueResponse{}, sql.ErrNoRows
	}
	return issueResponse{}, nil
}

func (f *fakeService) AssignIssues(_ context.Context, input assignIssuesInput, _ healthContextScope) ([]issueStageResponse, error) {
	f.assigned = append(f.assigned, input)
	return nil, nil
}

type fakeDirectory struct {
	groups  []DirectoryGroup
	members map[string][]DirectoryUser
	users   map[string]*DirectoryUser
	failed  bool
}

func (d *fakeDirectory) ListGroups(context.Context) ([]DirectoryGroup, error) {
	if d.failed {
		return nil, errors.New("down")
	}
	return d.groups, nil
}
func (d *fakeDirectory) ListGroupMembers(_ context.Context, id string) ([]DirectoryUser, error) {
	return d.members[id], nil
}
func (d *fakeDirectory) FindUsers(_ context.Context, query string, _ bool) ([]DirectoryUser, error) {
	if user, ok := d.users[query]; ok {
		return []DirectoryUser{*user}, nil
	}
	return nil, nil
}
func (d *fakeDirectory) GetUserGroups(context.Context, string) ([]DirectoryGroup, error) {
	return d.groups, nil
}
func (d *fakeDirectory) GetUser(id string) (*DirectoryUser, error) { return d.users[id], nil }

func newTestRouter(t *testing.T, service Service, directory UserDirectory, caller CallerName) *modulekit.Router {
	t.Helper()
	h := &handlers{service: service, keyAdminClient: directory, dqaStore: NewDQAStore(nil), caller: caller}
	router, err := modulekit.NewRouter(endpoints(h))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

type callOptions struct {
	permissions []string
	selection   *modulekit.HealthContextSelection
}

func call(handler http.Handler, method, target, body string, options callOptions) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(modulekit.WithRequest(req.Context(), modulekit.Request{
		Authenticated: true, RequestID: "r1", HealthContext: options.selection,
		Access: core.RequestAccess{Identity: core.Identity{Subject: "user-1"}, Permissions: options.permissions},
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func with(permissions ...string) callOptions { return callOptions{permissions: permissions} }

func TestEndpointsKeepTheirPermissions(t *testing.T) {
	router := newTestRouter(t, &fakeService{}, nil, nil)
	cases := []struct {
		method, path, want string
	}{
		{"GET", "/issues", "permission required: issue_tracker:read"},
		{"POST", "/issues", "permission required: issue_tracker:write"},
		{"PUT", "/issues/ISS-1", "permission required: issue_tracker:write"},
		{"POST", "/issues/ISS-1/resolveIssue", "one of these permissions is required: issue_tracker:comment, issue_tracker:manage, issue_tracker:close"},
		{"POST", "/issues/assign", "one of these permissions is required: issue_tracker:assign, issue_tracker:manage, issue_tracker:write"},
		{"POST", "/issues/ISS-1/assign", "one of these permissions is required: issue_tracker:assign, issue_tracker:manage, issue_tracker:write"},
		{"GET", "/issues/keycloak-groups/g1/members", "permission required: issue_tracker:read"},
		{"GET", "/data-quality/dqa/tables", "permission required: data_quality:read"},
		{"POST", "/data-validation/dqa/tables", "permission required: data_quality:write"},
		{"POST", "/data-quality/dqa/rules/mapping/preview", "permission required: data_quality:read"},
		{"POST", "/data-validation/dqa/rules/mapping/apply", "permission required: data_quality:write"},
		{"DELETE", "/data-quality/dqa/schedules/1", "permission required: data_quality:write"},
		{"GET", "/data-validation/dqa/completeness/runs/1/vhts", "permission required: data_quality:read"},
	}
	for _, tc := range cases {
		got := call(router, tc.method, tc.path, "", with())
		want := `{"success":false,"error":{"code":"FORBIDDEN","message":"` + tc.want + `"},"meta":{"requestId":"r1"}}`
		if got.Code != http.StatusForbidden || got.Body.String() != want {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, got.Code, got.Body.String())
		}
	}
	if got := call(router, "GET", "/data-quality/dqa/tables", "", with(IssueTrackerReadPermission)); got.Code != http.StatusForbidden {
		t.Fatalf("issue permission opened DQA: %d", got.Code)
	}
}

func TestIssueRoutesUseSelectionAndCaller(t *testing.T) {
	service := &fakeService{issueCount: 7}
	router := newTestRouter(t, service, nil, func(*http.Request) string { return "ops@example.org" })
	node := uuid.New()
	got := call(router, "POST", "/issues", `{"issue":"  broken  ","time_period":"2026Q1"}`, callOptions{
		permissions: []string{IssueTrackerWritePermission},
		selection:   &modulekit.HealthContextSelection{NodeID: node.String(), IncludeDescendants: true},
	})
	if got.Code != http.StatusCreated || got.Body.String() != `{"success":true,"data":{"issue_id":1,"issue_code":"ISS-1","date_reported":"2026-01-02"},"meta":{"requestId":"r1"}}` {
		t.Fatalf("create: %d %s", got.Code, got.Body.String())
	}
	input := service.created[0]
	if input.Issue != "broken" || input.ReportedBy != (sql.NullString{String: "ops@example.org", Valid: true}) ||
		input.TimePeriod != (sql.NullString{String: "2026Q1", Valid: true}) {
		t.Fatalf("create input: %+v", input)
	}
	if service.scopes[0] != (healthContextScope{ContextID: node, IncludeDescendants: true}) {
		t.Fatalf("scope: %+v", service.scopes[0])
	}

	if got := call(router, "POST", "/issues", `{"program":"x"}`, with(IssueTrackerWritePermission)); got.Code != 400 ||
		!strings.Contains(got.Body.String(), `"code":"INVALID_PAYLOAD","message":"invalid request payload"`) {
		t.Fatalf("missing issue: %d %s", got.Code, got.Body.String())
	}

	got = call(router, "GET", "/issues?orgUnit=OU&limit=99999", "", with(IssueTrackerReadPermission))
	if got.Code != 200 || got.Body.String() != `{"success":true,"data":[],"meta":{"requestId":"r1"}}` ||
		got.Header().Get("X-Total-Count") != "7" || got.Header().Get("Access-Control-Expose-Headers") != "X-Total-Count" {
		t.Fatalf("list: %d %s %v", got.Code, got.Body.String(), got.Header())
	}
	if filter := service.filters[0]; filter.OrgUnit != "OU" || filter.Limit != 10000 || filter.Scope != (healthContextScope{}) {
		t.Fatalf("list filter: %+v", filter)
	}

	service.notFound = true
	if got := call(router, "GET", "/issues/ISS-9", "", with(IssueTrackerReadPermission)); got.Code != 404 ||
		!strings.Contains(got.Body.String(), `"ISSUE_NOT_FOUND"`) {
		t.Fatalf("missing issue: %d %s", got.Code, got.Body.String())
	}

	// Without a host-supplied name the verified subject is used, as before.
	router = newTestRouter(t, service, nil, nil)
	if got := call(router, "POST", "/issues/ISS-3/assign", `{"assigned_to":"a@example.org"}`, with(IssueTrackerAssignPermission)); got.Code != 200 {
		t.Fatalf("assign: %d %s", got.Code, got.Body.String())
	}
	if a := service.assigned[0]; a.AssignedBy != "user-1" || len(a.IssueCodes) != 1 || a.IssueCodes[0] != "ISS-3" {
		t.Fatalf("assign input: %+v", a)
	}
}

func TestKeycloakGroupRoutes(t *testing.T) {
	read := with(IssueTrackerReadPermission)
	if got := call(newTestRouter(t, &fakeService{}, nil, nil), "GET", "/issues/keycloak-groups", "", read); got.Body.String() != `{"success":true,"data":[],"meta":{"requestId":"r1"}}` {
		t.Fatalf("no directory: %s", got.Body.String())
	}
	directory := &fakeDirectory{
		groups:  []DirectoryGroup{{ID: "g1", Name: "Team", Path: "/Team"}},
		members: map[string][]DirectoryUser{"g1": {{ID: "u1", Username: "u", Email: "u@example.org", FirstName: "F", LastName: "L"}}},
	}
	router := newTestRouter(t, &fakeService{}, directory, nil)
	if got := call(router, "GET", "/issues/keycloak-groups", "", read); got.Body.String() != `{"success":true,"data":[{"id":"g1","name":"Team","path":"/Team"}],"meta":{"requestId":"r1"}}` {
		t.Fatalf("groups: %s", got.Body.String())
	}
	if got := call(router, "GET", "/issues/keycloak-groups/g1/members", "", read); got.Body.String() != `{"success":true,"data":[{"id":"u1","username":"u","email":"u@example.org","firstName":"F","lastName":"L"}],"meta":{"requestId":"r1"}}` {
		t.Fatalf("members: %s", got.Body.String())
	}
	directory.failed = true
	if got := call(router, "GET", "/issues/keycloak-groups", "", read); got.Code != 500 || !strings.Contains(got.Body.String(), "KEYCLOAK_GROUPS_FAILED") {
		t.Fatalf("directory failure: %d %s", got.Code, got.Body.String())
	}
}

func TestDQAValidationKeepsFrameworkMessages(t *testing.T) {
	router := newTestRouter(t, &fakeService{}, nil, nil)
	write, read := with(DataQualityWritePermission), with(DataQualityReadPermission)
	cases := []struct {
		method, path, body string
		options            callOptions
		status             int
		want               string
	}{
		{"POST", "/data-quality/dqa/run", `{}`, write, 503,
			`{"code":"DWH_UNAVAILABLE","message":"DWH connection is not configured"}`},
		{"POST", "/data-validation/dqa/schedules", `{}`, write, 400,
			`{"code":"INVALID_BODY","message":"Key: 'scheduleRunRequest.TableID' Error:Field validation for 'TableID' failed on the 'required' tag\nKey: 'scheduleRunRequest.ScheduledAt' Error:Field validation for 'ScheduledAt' failed on the 'required' tag"}`},
		{"POST", "/data-validation/dqa/rules", `{"code":"c"}`, write, 400,
			`{"code":"INVALID_BODY","message":"Key: 'dqaRuleRequest.TableID' Error:Field validation for 'TableID' failed on the 'required' tag\nKey: 'dqaRuleRequest.Type' Error:Field validation for 'Type' failed on the 'required' tag"}`},
		{"POST", "/data-quality/dqa/rules/mapping/apply", `{"table_id":"t"}`, write, 400,
			`{"code":"INVALID_BODY","message":"Key: 'ruleMappingApplyRequest.Codes' Error:Field validation for 'Codes' failed on the 'required' tag"}`},
		{"POST", "/data-quality/dqa/schedules", `{"table_id":"t","scheduled_at":"2000-01-01T00:00:00Z"}`, write, 400,
			`{"code":"INVALID_SCHEDULE","message":"scheduled_at is in the past"}`},
		{"POST", "/data-quality/dqa/tables", `{"table_id":1}`, write, 400,
			`{"code":"INVALID_BODY","message":"json: cannot unmarshal number into Go struct field dqaTableRequest.table_id of type string"}`},
		{"GET", "/data-quality/dqa/runs/x/flags", "", read, 400,
			`{"code":"INVALID_RUN_ID","message":"runId must be an integer"}`},
		{"DELETE", "/data-validation/dqa/schedules/x", "", write, 400,
			`{"code":"INVALID_SCHEDULE_ID","message":"scheduleId must be an integer"}`},
		{"GET", "/data-quality/dqa/schedules?status=bogus", "", read, 400,
			`{"code":"INVALID_STATUS","message":"unknown status"}`},
		{"GET", "/data-quality/dqa/completeness/runs/1/vhts?status=bogus", "", read, 400,
			`{"code":"INVALID_STATUS","message":"status must be missing or reported"}`},
		{"GET", "/data-quality/dqa/columns?table=t", "", read, 503,
			`{"code":"DWH_UNAVAILABLE","message":"DWH connection is not configured"}`},
		{"POST", "/data-quality/dqa/rules/mapping/preview", `{"table_id":"t"}`, read, 503,
			`{"code":"DWH_UNAVAILABLE","message":"DWH connection is not configured"}`},
	}
	for _, tc := range cases {
		got := call(router, tc.method, tc.path, tc.body, tc.options)
		want := `{"success":false,"error":` + tc.want + `,"meta":{"requestId":"r1"}}`
		if got.Code != tc.status || got.Body.String() != want {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, got.Code, got.Body.String())
		}
	}

	// With a warehouse configured the body is validated before any query.
	h := &handlers{service: &fakeService{}, dwhDB: new(sql.DB), dqaStore: NewDQAStore(nil)}
	withWarehouse, err := modulekit.NewRouter(endpoints(h))
	if err != nil {
		t.Fatal(err)
	}
	got := call(withWarehouse, "POST", "/data-quality/dqa/run", `{"scope":{}}`, write)
	if got.Code != 400 || !strings.Contains(got.Body.String(), `"message":"Key: 'dqaRunRequest.TableID' Error:Field validation for 'TableID' failed on the 'required' tag"`) {
		t.Fatalf("run validation: %d %s", got.Code, got.Body.String())
	}
}

type recordingMailer struct {
	sendErr error
	sent    chan EmailMessage
	backup  chan EmailMessage
}

func (m *recordingMailer) Send(_ context.Context, msg EmailMessage) error {
	m.sent <- msg
	return m.sendErr
}
func (m *recordingMailer) Fallback(_ context.Context, msg EmailMessage) error {
	m.backup <- msg
	return nil
}

type fakeRepository struct {
	Repository
	issues map[string]issueResponse
}

func (r *fakeRepository) AssignIssues(context.Context, assignIssuesInput, healthContextScope) ([]issueStageResponse, error) {
	return nil, nil
}
func (r *fakeRepository) ResolveIssue(_ context.Context, input resolveIssueInput, _ healthContextScope) (issueStageResponse, error) {
	return issueStageResponse{IssueCode: input.IssueCode}, nil
}
func (r *fakeRepository) GetIssuesByCodes(_ context.Context, codes []string, _ healthContextScope) ([]issueResponse, error) {
	var out []issueResponse
	for _, code := range codes {
		if issue, ok := r.issues[code]; ok {
			out = append(out, issue)
		}
	}
	return out, nil
}

func receive(t *testing.T, ch chan EmailMessage) EmailMessage {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("email not sent")
		return EmailMessage{}
	}
}

func TestIssueEmailsKeepContentAndFallback(t *testing.T) {
	code, desc := "ISS-1", "Broken totals"
	repo := &fakeRepository{issues: map[string]issueResponse{code: {IssueCode: &code, Issue: &desc}}}
	mailer := &recordingMailer{sendErr: errors.New("smtp down"), sent: make(chan EmailMessage, 1), backup: make(chan EmailMessage, 1)}
	service := NewService(repo, mailer, nil, "https://portal.example/")
	if _, err := service.AssignIssues(context.Background(), assignIssuesInput{IssueCodes: []string{code}, AssignedTo: "a@example.org"}, healthContextScope{}); err != nil {
		t.Fatal(err)
	}
	msg := receive(t, mailer.sent)
	if msg.Subject != "[MOH Issue Tracker] Issue ISS-1 Assigned to You" || len(msg.To) != 1 || msg.To[0] != "a@example.org" ||
		!strings.Contains(msg.HTMLBody, "https://portal.example/portal/apps/dwh/issue-tracker/issues?issueCode=ISS-1") ||
		!strings.Contains(msg.TextBody, "• Issue Code: ISS-1\n  Description: Broken totals\n") {
		t.Fatalf("assignment email: %+v", msg)
	}
	if backup := receive(t, mailer.backup); backup.Subject != msg.Subject {
		t.Fatalf("fallback email: %+v", backup)
	}

	directory := &fakeDirectory{
		groups:  []DirectoryGroup{{ID: "g1"}},
		members: map[string][]DirectoryUser{"g1": {{Email: " Peer@Example.org "}}},
		users:   map[string]*DirectoryUser{"resolver": {ID: "u-resolver"}},
	}
	mailer = &recordingMailer{sent: make(chan EmailMessage, 1), backup: make(chan EmailMessage, 1)}
	service = NewService(repo, mailer, directory, "")
	if _, err := service.ResolveIssue(context.Background(), resolveIssueInput{IssueCode: code, Status: "CLOSED", ResolvedBy: "resolver"}, healthContextScope{}); err != nil {
		t.Fatal(err)
	}
	msg = receive(t, mailer.sent)
	if msg.Subject != "[MOH Issue Tracker] Issue ISS-1 Resolved by resolver" || len(msg.To) != 1 || msg.To[0] != "peer@example.org" ||
		!strings.Contains(msg.HTMLBody, "https://dashboards.health.go.ug/portal/apps/dwh/issue-tracker/issues?issueCode=ISS-1") {
		t.Fatalf("resolution email: %+v", msg)
	}
	select {
	case <-mailer.backup:
		t.Fatal("fallback used although the primary send succeeded")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHealthContextScopeFromRequest(t *testing.T) {
	node := uuid.New()
	for name, tc := range map[string]struct {
		selection *modulekit.HealthContextSelection
		want      healthContextScope
	}{
		"none":        {nil, healthContextScope{}},
		"node":        {&modulekit.HealthContextSelection{NodeID: node.String()}, healthContextScope{ContextID: node}},
		"descendants": {&modulekit.HealthContextSelection{NodeID: node.String(), IncludeDescendants: true}, healthContextScope{ContextID: node, IncludeDescendants: true}},
		"invalid":     {&modulekit.HealthContextSelection{NodeID: "x"}, healthContextScope{}},
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req = req.WithContext(modulekit.WithRequest(req.Context(), modulekit.Request{HealthContext: tc.selection}))
		if got := healthContextScopeFromRequest(modulekit.NewContext(httptest.NewRecorder(), req)); got != tc.want {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

func TestDependenciesContractAndWorkerLifecycle(t *testing.T) {
	if _, err := NewModule(Dependencies{}); err == nil {
		t.Fatal("missing primary database accepted")
	}
	module, err := NewModule(Dependencies{Primary: new(sql.DB)})
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Patterns()) != 55 || len(module.OpenAPI()) == 0 {
		t.Fatalf("module contract incomplete: %d", len(module.Patterns()))
	}
	if module.worker != nil {
		t.Fatal("worker built without the DQA and warehouse connections")
	}
	// No warehouse: migrations are skipped and no worker starts.
	stop, err := module.Initialize(context.Background())
	if err != nil || stop != nil {
		t.Fatalf("initialize without warehouse: %v %v", stop, err)
	}
	withWorker, err := NewModule(Dependencies{Primary: new(sql.DB), Warehouse: new(sql.DB), DQA: new(sql.DB)})
	if err != nil || withWorker.worker == nil || withWorker.worker.interval != time.Minute {
		t.Fatalf("worker not configured: %v", err)
	}

	// The registry stop cancels and joins the worker.
	idle := &Module{
		repository: newPostgresRepository(nil, nil), store: NewDQAStore(nil), logger: slog.Default(),
		worker: NewScheduleWorker(NewDQAStore(nil), nil, time.Hour, nil),
	}
	stop, err = idle.Initialize(context.Background())
	if err != nil || stop == nil {
		t.Fatalf("initialize: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if Version() == "" {
		t.Fatal("empty version")
	}
}
