package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

type opsCall struct {
	method, app, runID, actor, note, subject, reason string
	accounts                                         []string
	approve, resume                                  bool
}

type fakeOps struct {
	calls []opsCall
	err   error
	rows  []identityreconcile.BatchRow
}

func (f *fakeOps) record(c opsCall) error {
	f.calls = append(f.calls, c)
	return f.err
}

func (f *fakeOps) Jobs(app string) []identityreconcile.Job {
	return []identityreconcile.Job{{ID: "job-1", Application: app, Kind: identityreconcile.JobStartRun, Status: identityreconcile.JobRunning}}
}

func (f *fakeOps) StartRun(_ context.Context, app, actor string) (identityreconcile.Job, error) {
	return identityreconcile.Job{ID: "job-2", Application: app, Actor: actor, Status: identityreconcile.JobRunning}, f.record(opsCall{method: "start", app: app, actor: actor})
}

func (f *fakeOps) GetRun(_ context.Context, app, runID string) (identityreconcile.BatchRun, []identityreconcile.BatchRow, error) {
	return identityreconcile.BatchRun{ID: runID, ApplicationClientID: app}, f.rows, f.record(opsCall{method: "get", app: app, runID: runID})
}

func (f *fakeOps) Decide(_ context.Context, app, runID string, accounts []string, approve bool, reviewer, note string) ([]identityreconcile.BatchRow, error) {
	return f.rows, f.record(opsCall{method: "decide", app: app, runID: runID, accounts: accounts, approve: approve, actor: reviewer, note: note})
}

func (f *fakeOps) ApplyRun(_ context.Context, app, runID, actor string, resume bool) (identityreconcile.Job, error) {
	return identityreconcile.Job{ID: "job-3", RunID: runID}, f.record(opsCall{method: "apply", app: app, runID: runID, actor: actor, resume: resume})
}

func (f *fakeOps) ListRuns(_ context.Context, app string, _ int) ([]identityreconcile.BatchRun, error) {
	return nil, f.record(opsCall{method: "runs", app: app})
}

func (f *fakeOps) ListRevocations(_ context.Context, app string, all, unfinished bool, _ int) ([]identityreconcile.Revocation, error) {
	return []identityreconcile.Revocation{{ID: "r1", ApplicationClientID: app, RoleRemoved: "done", ConsentsRevoked: "done", SessionsRevoked: "failed"}},
		f.record(opsCall{method: "revocations", app: app, approve: all, resume: unfinished})
}

func (f *fakeOps) ResumeRevocations(app, actor string) (identityreconcile.Job, error) {
	return identityreconcile.Job{ID: "job-4"}, f.record(opsCall{method: "resume-revocations", app: app, actor: actor})
}

func (f *fakeOps) RestoreRevocation(_ context.Context, app, subject, actor, reason string) (identityreconcile.Revocation, error) {
	return identityreconcile.Revocation{ID: "r1"}, f.record(opsCall{method: "restore", app: app, subject: subject, actor: actor, reason: reason})
}

func (f *fakeOps) Describe(_ context.Context, app string) (identitysync.Descriptor, error) {
	return identitysync.Descriptor{Application: app, Version: "1.1"}, f.record(opsCall{method: "describe", app: app})
}

func opsRouter(ops IdentitySyncOperations, permissions ...authz.Permission) *gin.Engine {
	h := &Handler{Repository: &testRepository{}, Verifier: testVerifier{token: validToken()}, IdentitySyncOps: ops}
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(authz.ContextKey, authz.Context{UserID: "operator-1", Permissions: permissions})
		c.Set("user_id", "operator-1")
		c.Set("user", &keycloak.AuthUser{ID: "operator-1", Email: "admin@example.org", Username: "admin"})
	})
	RegisterRoutes(router.Group("/api/v1"), protected, h, nil)
	return router
}

const opsBase = "/api/v1/admin/platform/applications/alerts/identity-sync"

var opsPermissions = []authz.Permission{"rbac:write", "rbac:roles:write"}

func opsRequest(router *gin.Engine, method, path, body string) (int, map[string]any) {
	req := httptest.NewRequest(method, opsBase+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

const runID = "8a305fe4-9fd0-4642-9f88-4a4b3e7ad4d4"

func TestIdentitySyncOpsRoutes(t *testing.T) {
	ops := &fakeOps{rows: []identityreconcile.BatchRow{
		{AccountID: "1", Proposal: identityreconcile.ProposalReuse, Decision: identityreconcile.DecisionPending},
		{AccountID: "2", Proposal: identityreconcile.ProposalReview, Decision: identityreconcile.DecisionPending, LastError: "directory_changed"},
	}}
	router := opsRouter(ops, opsPermissions...)
	cases := []struct {
		method, path, body string
		status             int
		want               opsCall
	}{
		{http.MethodGet, "/runs", "", 200, opsCall{method: "runs", app: "alerts"}},
		{http.MethodPost, "/runs", "", 202, opsCall{method: "start", app: "alerts", actor: "portal:admin@example.org"}},
		{http.MethodGet, "/runs/" + runID, "", 200, opsCall{method: "get", app: "alerts", runID: runID}},
		// The reviewer is the signed-in admin, even when the body names someone else.
		{http.MethodPost, "/runs/" + runID + "/decisions", `{"decision":"approve","accountIds":["1"],"note":"ok","reviewer":"someone-else"}`, 200,
			opsCall{method: "decide", app: "alerts", runID: runID, accounts: []string{"1"}, approve: true, actor: "portal:admin@example.org", note: "ok"}},
		{http.MethodPost, "/runs/" + runID + "/decisions", `{"decision":"reject","accountIds":["2"]}`, 200,
			opsCall{method: "decide", app: "alerts", runID: runID, accounts: []string{"2"}, actor: "portal:admin@example.org"}},
		{http.MethodPost, "/runs/" + runID + "/apply", `{"resume":true}`, 202, opsCall{method: "apply", app: "alerts", runID: runID, actor: "portal:admin@example.org", resume: true}},
		{http.MethodGet, "/revocations?all=true&unfinished=1", "", 200, opsCall{method: "revocations", app: "alerts", approve: true, resume: true}},
		{http.MethodPost, "/revocations/resume", "", 202, opsCall{method: "resume-revocations", app: "alerts", actor: "portal:admin@example.org"}},
		{http.MethodPost, "/revocations/restore", `{"subject":" s-1 ","reason":" back "}`, 200,
			opsCall{method: "restore", app: "alerts", subject: "s-1", actor: "portal:admin@example.org", reason: "back"}},
		{http.MethodGet, "/describe", "", 200, opsCall{method: "describe", app: "alerts"}},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			ops.calls = nil
			status, body := opsRequest(router, tc.method, tc.path, tc.body)
			if status != tc.status {
				t.Fatalf("status = %d, body %v", status, body)
			}
			if len(ops.calls) != 1 || fmt.Sprint(ops.calls[0]) != fmt.Sprint(tc.want) {
				t.Fatalf("calls = %+v, want %+v", ops.calls, tc.want)
			}
		})
	}

	_, body := opsRequest(router, http.MethodGet, "/runs/"+runID, "")
	counts := body["data"].(map[string]any)["counts"].(map[string]any)
	if counts["total"] != 2.0 || counts["proposal:reuse"] != 1.0 || counts["error"] != 1.0 {
		t.Fatalf("counts = %v", counts)
	}
	_, body = opsRequest(router, http.MethodGet, "/runs", "")
	if runs := body["data"].(map[string]any)["runs"]; runs == nil {
		t.Fatal("runs must be an empty list, not null")
	}
	_, body = opsRequest(router, http.MethodGet, "/revocations", "")
	rev := body["data"].(map[string]any)["revocations"].([]any)[0].(map[string]any)
	if rev["complete"] != false || rev["sessionsStillOpen"] != true {
		t.Fatalf("revocation = %v", rev)
	}
}

func TestIdentitySyncOpsRejectsBadInput(t *testing.T) {
	ops := &fakeOps{}
	router := opsRouter(ops, opsPermissions...)
	many := make([]string, maxDecisionAccounts+1)
	for i := range many {
		many[i] = fmt.Sprintf(`"%d"`, i)
	}
	for name, tc := range map[string]struct{ path, body string }{
		"unknown decision": {"/runs/" + runID + "/decisions", `{"decision":"maybe","accountIds":["1"]}`},
		"no accounts":      {"/runs/" + runID + "/decisions", `{"decision":"approve","accountIds":[]}`},
		"too many":         {"/runs/" + runID + "/decisions", `{"decision":"approve","accountIds":[` + strings.Join(many, ",") + `]}`},
		"apply not json":   {"/runs/" + runID + "/apply", `resume`},
		"restore blank":    {"/revocations/restore", `{"subject":" ","reason":"x"}`},
		"restore reason":   {"/revocations/restore", `{"subject":"s","reason":" "}`},
	} {
		t.Run(name, func(t *testing.T) {
			if status, body := opsRequest(router, http.MethodPost, tc.path, tc.body); status != http.StatusBadRequest {
				t.Fatalf("status = %d, body %v", status, body)
			}
		})
	}
	if len(ops.calls) != 0 {
		t.Fatalf("invalid requests reached the service: %+v", ops.calls)
	}
}

func TestIdentitySyncOpsErrors(t *testing.T) {
	for err, want := range map[error]int{
		identityreconcile.ErrRunNotFound:                                404,
		identityreconcile.ErrRevocationNotFound:                         404,
		identityreconcile.ErrJobRunning:                                 409,
		identityreconcile.ErrRunBusy:                                    409,
		identityreconcile.ErrRowStarted:                                 409,
		fmt.Errorf("x: %w", identityreconcile.ErrIntegrationDisabled):   409,
		identityreconcile.ErrRowNotApprovable:                           400,
		identityreconcile.ErrIntegrationNotFound:                        404,
		fmt.Errorf("x: %w", identityreconcile.ErrPrivateOrigin):         422,
		fmt.Errorf("upstream https://app.internal/?t=s3cret: HTTP 500"): 500,
	} {
		ops := &fakeOps{err: err}
		status, body := opsRequest(opsRouter(ops, opsPermissions...), http.MethodPost, "/runs", "")
		if status != want {
			t.Errorf("%v: status = %d, want %d (%v)", err, status, want, body)
		}
		if strings.Contains(fmt.Sprint(body), "s3cret") {
			t.Errorf("%v: response leaks upstream detail: %v", err, body)
		}
	}
}

func TestIdentitySyncOpsNeedsPermissionsAndConfiguration(t *testing.T) {
	ops := &fakeOps{}
	if status, _ := opsRequest(opsRouter(ops, "rbac:write"), http.MethodPost, "/runs", ""); status != http.StatusForbidden {
		t.Fatalf("without rbac:roles:write: status = %d", status)
	}
	if len(ops.calls) != 0 {
		t.Fatal("a forbidden request reached the service")
	}
	if status, _ := opsRequest(opsRouter(nil, opsPermissions...), http.MethodGet, "/runs", ""); status != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured: status = %d", status)
	}
}
