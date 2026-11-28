package platform

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// IdentitySyncOperations runs reviewed batch syncs and revocation upkeep for
// the portal. *identityreconcile.Operations implements it. Starting, applying
// and resuming return a background job at once; the portal polls ListJobs.
type IdentitySyncOperations interface {
	Jobs(application string) []identityreconcile.Job
	StartRun(ctx context.Context, application, actor string) (identityreconcile.Job, error)
	GetRun(ctx context.Context, application, runID string) (identityreconcile.BatchRun, []identityreconcile.BatchRow, error)
	Decide(ctx context.Context, application, runID string, accountIDs []string, approve bool, reviewer, note string) ([]identityreconcile.BatchRow, error)
	ApplyRun(ctx context.Context, application, runID, actor string, resume bool) (identityreconcile.Job, error)
	ListRuns(ctx context.Context, application string, limit int) ([]identityreconcile.BatchRun, error)
	ListRevocations(ctx context.Context, application string, includeRestored, onlyUnfinished bool, limit int) ([]identityreconcile.Revocation, error)
	ResumeRevocations(application, actor string) (identityreconcile.Job, error)
	RestoreRevocation(ctx context.Context, application, subject, actor, reason string) (identityreconcile.Revocation, error)
	Describe(ctx context.Context, application string) (identitysync.Descriptor, error)
}

// IdentitySyncDecisionInput approves or rejects rows of a run. The reviewer is
// the signed-in administrator; it is never taken from the request.
type IdentitySyncDecisionInput struct {
	Decision   string   `json:"decision" binding:"required"`
	AccountIDs []string `json:"accountIds" binding:"required"`
	Note       string   `json:"note"`
}

// IdentitySyncApplyInput applies (resume=false) or resumes a run.
type IdentitySyncApplyInput struct {
	Resume bool `json:"resume"`
}

// IdentitySyncRestoreInput clears a person's active revocation.
type IdentitySyncRestoreInput struct {
	Subject string `json:"subject" binding:"required"`
	Reason  string `json:"reason" binding:"required"`
}

// IdentitySyncRunDetail is a run with its rows and counts.
type IdentitySyncRunDetail struct {
	Run    identityreconcile.BatchRun   `json:"run"`
	Counts map[string]int               `json:"counts"`
	Rows   []identityreconcile.BatchRow `json:"rows"`
}

// maxDecisionAccounts bounds one decision request.
const maxDecisionAccounts = 500

// portalActor names the signed-in administrator in run, review and audit records.
func portalActor(c *gin.Context) string {
	if u, ok := c.Get("user"); ok {
		if user, ok := u.(*keycloak.AuthUser); ok && user != nil {
			if email := strings.TrimSpace(user.Email); email != "" {
				return "portal:" + email
			}
			if username := strings.TrimSpace(user.Username); username != "" {
				return "portal:" + username
			}
		}
	}
	if id := strings.TrimSpace(c.GetString("user_id")); id != "" {
		return "portal:" + id
	}
	return ""
}

func (h *Handler) identitySyncOps(c *gin.Context) (IdentitySyncOperations, bool) {
	if h.IdentitySyncOps == nil {
		fail(c, ErrUnavailable)
		return nil, false
	}
	return h.IdentitySyncOps, true
}

func runCounts(rows []identityreconcile.BatchRow) map[string]int {
	counts := map[string]int{"total": len(rows)}
	for _, row := range rows {
		counts["proposal:"+string(row.Proposal)]++
		counts["decision:"+string(row.Decision)]++
		if row.LastError != "" {
			counts["error"]++
		}
	}
	return counts
}

// ListIdentitySyncRuns returns an application's recent runs and jobs.
func (h *Handler) ListIdentitySyncRuns(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	app := c.Param("clientId")
	runs, err := ops.ListRuns(c.Request.Context(), app, 20)
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	if runs == nil {
		runs = []identityreconcile.BatchRun{}
	}
	response.OK(c, http.StatusOK, gin.H{"runs": runs, "jobs": ops.Jobs(app)})
}

// ListIdentitySyncJobs returns an application's recent background jobs.
func (h *Handler) ListIdentitySyncJobs(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	response.OK(c, http.StatusOK, gin.H{"jobs": ops.Jobs(c.Param("clientId"))})
}

// StartIdentitySyncRun starts reading every eligible account in the background.
func (h *Handler) StartIdentitySyncRun(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	job, err := ops.StartRun(c.Request.Context(), c.Param("clientId"), portalActor(c))
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	response.OK(c, http.StatusAccepted, job)
}

// GetIdentitySyncRun returns one run with its rows.
func (h *Handler) GetIdentitySyncRun(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	run, rows, err := ops.GetRun(c.Request.Context(), c.Param("clientId"), c.Param("runId"))
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	if rows == nil {
		rows = []identityreconcile.BatchRow{}
	}
	response.OK(c, http.StatusOK, IdentitySyncRunDetail{Run: run, Counts: runCounts(rows), Rows: rows})
}

// DecideIdentitySyncRows approves or rejects rows; the reviewer is the signed-in admin.
func (h *Handler) DecideIdentitySyncRows(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	var input IdentitySyncDecisionInput
	if err := c.ShouldBindJSON(&input); err != nil || len(input.AccountIDs) == 0 || len(input.AccountIDs) > maxDecisionAccounts {
		fail(c, ErrInvalid)
		return
	}
	var approve bool
	switch input.Decision {
	case "approve":
		approve = true
	case "reject":
	default:
		fail(c, ErrInvalid)
		return
	}
	rows, err := ops.Decide(c.Request.Context(), c.Param("clientId"), c.Param("runId"), input.AccountIDs, approve, portalActor(c), input.Note)
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	response.OK(c, http.StatusOK, gin.H{"rows": rows})
}

// ApplyIdentitySyncRun applies or resumes a run in the background.
func (h *Handler) ApplyIdentitySyncRun(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var input IdentitySyncApplyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, ErrInvalid)
		return
	}
	job, err := ops.ApplyRun(c.Request.Context(), c.Param("clientId"), c.Param("runId"), portalActor(c), input.Resume)
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	response.OK(c, http.StatusAccepted, job)
}

// ListIdentitySyncRevocations lists an application's revocations. Query
// parameters: all=true includes restored ones, unfinished=true keeps only
// active revocations with a pending or failed step.
func (h *Handler) ListIdentitySyncRevocations(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	all, _ := strconv.ParseBool(c.DefaultQuery("all", "false"))
	unfinished, _ := strconv.ParseBool(c.DefaultQuery("unfinished", "false"))
	app := c.Param("clientId")
	list, err := ops.ListRevocations(c.Request.Context(), app, all, unfinished, 200)
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	type item struct {
		identityreconcile.Revocation
		Complete          bool `json:"complete"`
		SessionsStillOpen bool `json:"sessionsStillOpen"`
	}
	items := make([]item, 0, len(list))
	for _, r := range list {
		items = append(items, item{Revocation: r, Complete: r.Complete(), SessionsStillOpen: r.SessionsStillOpen()})
	}
	response.OK(c, http.StatusOK, gin.H{"revocations": items, "jobs": ops.Jobs(app)})
}

// ResumeIdentitySyncRevocations retries unfinished revocation steps in the background.
func (h *Handler) ResumeIdentitySyncRevocations(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	job, err := ops.ResumeRevocations(c.Param("clientId"), portalActor(c))
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	response.OK(c, http.StatusAccepted, job)
}

// RestoreIdentitySyncRevocation clears an active revocation. It never grants access.
func (h *Handler) RestoreIdentitySyncRevocation(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var input IdentitySyncRestoreInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, ErrInvalid)
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	input.Subject = strings.TrimSpace(input.Subject)
	if input.Reason == "" || len(input.Reason) > 2000 || input.Subject == "" || len(input.Subject) > 255 {
		fail(c, ErrInvalid)
		return
	}
	revocation, err := ops.RestoreRevocation(c.Request.Context(), c.Param("clientId"), input.Subject, portalActor(c), input.Reason)
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	response.OK(c, http.StatusOK, revocation)
}

// DescribeIdentitySync reads the application's live GET / descriptor.
func (h *Handler) DescribeIdentitySync(c *gin.Context) {
	ops, ok := h.identitySyncOps(c)
	if !ok {
		return
	}
	descriptor, err := ops.Describe(c.Request.Context(), c.Param("clientId"))
	if err != nil {
		failIdentitySyncOp(c, err)
		return
	}
	response.OK(c, http.StatusOK, descriptor)
}

// identitySyncNetworkChecker runs network checks; *identityreconcile.RegistryService implements it.
type identitySyncNetworkChecker interface {
	CheckNetwork(ctx context.Context, origin string) identityreconcile.NetworkReport
	CheckIntegrationsNetwork(ctx context.Context, application string) ([]identityreconcile.NetworkReport, error)
}

const identitySyncNetworkCheckTimeout = time.Minute

// CheckIdentitySyncNetwork checks, from the portal server, each step needed to
// reach the application's identity-sync endpoint: DNS, the private-address
// policy, TCP, TLS and an unauthenticated HTTP request. With ?origin= it checks
// that origin (for example before saving it); otherwise the registered one.
// It sends no token and returns only stage results.
func (h *Handler) CheckIdentitySyncNetwork(c *gin.Context) {
	checker, ok := h.IdentitySync.(identitySyncNetworkChecker)
	if h.IdentitySync == nil || !ok {
		fail(c, ErrUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), identitySyncNetworkCheckTimeout)
	defer cancel()
	if origin := c.Query("origin"); origin != "" {
		if !identityreconcile.ValidOrigin(origin) {
			fail(c, ErrInvalid)
			return
		}
		report := checker.CheckNetwork(ctx, origin)
		report.ApplicationID = c.Param("clientId")
		response.OK(c, http.StatusOK, report)
		return
	}
	reports, err := checker.CheckIntegrationsNetwork(ctx, c.Param("clientId"))
	if err != nil {
		failIdentitySync(c, err)
		return
	}
	if len(reports) != 1 {
		fail(c, sql.ErrNoRows)
		return
	}
	response.OK(c, http.StatusOK, reports[0])
}

// failIdentitySyncOp maps run, job and revocation errors to stable responses.
func failIdentitySyncOp(c *gin.Context, err error) {
	code, message := identityreconcile.JobErrorCode(err)
	switch {
	case errors.Is(err, identityreconcile.ErrRunNotFound), errors.Is(err, identityreconcile.ErrRowNotFound),
		errors.Is(err, identityreconcile.ErrRevocationNotFound):
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, identityreconcile.ErrJobRunning), errors.Is(err, identityreconcile.ErrRunBusy),
		errors.Is(err, identityreconcile.ErrRunCancelled), errors.Is(err, identityreconcile.ErrRowStarted):
		response.Fail(c, http.StatusConflict, "IDENTITY_SYNC_BUSY", err.Error())
	case errors.Is(err, identityreconcile.ErrIntegrationDisabled):
		response.Fail(c, http.StatusConflict, "IDENTITY_SYNC_DISABLED", message)
	case errors.Is(err, identityreconcile.ErrRowNotApprovable), errors.Is(err, identityreconcile.ErrReviewerRequired),
		errors.Is(err, identityreconcile.ErrInvalidNote), errors.Is(err, identityreconcile.ErrInvalidRevocation):
		response.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
	case errors.Is(err, identityreconcile.ErrPrivateOrigin), errors.Is(err, identityreconcile.ErrDescriptorMissing),
		errors.Is(err, identityreconcile.ErrContractViolation):
		response.Fail(c, http.StatusUnprocessableEntity, "IDENTITY_SYNC_"+strings.ToUpper(code), message)
	default:
		failIdentitySync(c, err)
		return
	}
	c.Abort()
}
