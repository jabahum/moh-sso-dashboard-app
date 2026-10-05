package report_scheduler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/http/response"
)

type Handler struct{ service *Service }
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) GetModule(c *gin.Context) {
	module := h.service.Module()
	module.HealthContext = healthContextFromGin(c)
	response.OK(c, http.StatusOK, module)
}

func (h *Handler) ListReports(c *gin.Context) {
	items, err := h.service.ListReports(c.Request.Context()); if err != nil { healthBIFailure(c, err); return }
	response.OK(c, http.StatusOK, items)
}
func (h *Handler) GetReport(c *gin.Context) {
	item, err := h.service.GetReport(c.Request.Context(), c.Param("reportId")); if err != nil { healthBIFailure(c, err); return }
	response.OK(c, http.StatusOK, item)
}
func (h *Handler) GetReportParameters(c *gin.Context) {
	items, err := h.service.GetParameters(c.Request.Context(), c.Param("reportId")); if err != nil { healthBIFailure(c, err); return }
	response.OK(c, http.StatusOK, items)
}
func (h *Handler) GenerateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil { response.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error()); return }
	job, err := h.service.Generate(c.Request.Context(), c.Param("reportId"), req); if err != nil { healthBIFailure(c, err); return }
	response.OK(c, http.StatusAccepted, job)
}
func (h *Handler) GetJob(c *gin.Context) {
	job, err := h.service.GetJob(c.Request.Context(), c.Param("jobId")); if err != nil { healthBIFailure(c, err); return }
	response.OK(c, http.StatusOK, job)
}

func (h *Handler) CreateSchedule(c *gin.Context) {
	var req CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil { response.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error()); return }
	item, err := h.service.CreateSchedule(c.Request.Context(), req, c.GetString("user_id"), healthContextFromGin(c)); if err != nil { response.Fail(c, http.StatusBadRequest, "SCHEDULE_CREATE_FAILED", err.Error()); return }
	response.OK(c, http.StatusCreated, item)
}
func (h *Handler) ListSchedules(c *gin.Context) {
	options, ok := parseListOptions(c, true)
	if !ok { return }
	items, err := h.service.ListSchedules(c.Request.Context(), c.GetString("user_id"), canManage(c), healthContextFromGin(c), options); if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_LIST_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, items)
}
func (h *Handler) GetSchedule(c *gin.Context) {
	item, err := h.service.GetSchedule(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c)); if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }; if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_GET_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}
func (h *Handler) UpdateSchedule(c *gin.Context) {
	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil { response.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error()); return }
	item, err := h.service.UpdateSchedule(c.Request.Context(), c.Param("scheduleId"), req, c.GetString("user_id"), canManage(c), healthContextFromGin(c)); if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }; if err != nil { response.Fail(c, http.StatusBadRequest, "SCHEDULE_UPDATE_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}
func (h *Handler) DeleteSchedule(c *gin.Context) {
	err := h.service.DeleteSchedule(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c)); if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }; if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_DELETE_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, gin.H{"deleted": true})
}
func (h *Handler) PauseSchedule(c *gin.Context) {
	item, err := h.service.PauseSchedule(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }
	if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_PAUSE_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}
func (h *Handler) ResumeSchedule(c *gin.Context) {
	item, err := h.service.ResumeSchedule(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }
	if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_RESUME_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}
func (h *Handler) DuplicateSchedule(c *gin.Context) {
	item, err := h.service.DuplicateSchedule(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }
	if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_DUPLICATE_FAILED", err.Error()); return }
	response.OK(c, http.StatusCreated, item)
}
func (h *Handler) RunNow(c *gin.Context) {
	execution, err := h.service.RunNow(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }
	if err != nil { response.Fail(c, http.StatusBadGateway, "SCHEDULE_RUN_FAILED", err.Error()); return }
	response.OK(c, http.StatusAccepted, execution)
}
func (h *Handler) ListExecutions(c *gin.Context) {
	options, ok := parseListOptions(c, false)
	if !ok { return }
	items, err := h.service.ListExecutions(c.Request.Context(), c.GetString("user_id"), canManage(c), healthContextFromGin(c), options); if err != nil { response.Fail(c, http.StatusInternalServerError, "EXECUTION_LIST_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, items)
}

func (h *Handler) Overview(c *gin.Context) {
	item, err := h.service.Overview(c.Request.Context(), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULER_OVERVIEW_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}

func (h *Handler) GetExecution(c *gin.Context) {
	item, err := h.service.GetExecutionDetail(c.Request.Context(), c.Param("executionId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "EXECUTION_NOT_FOUND", "report execution not found"); return }
	if err != nil { response.Fail(c, http.StatusInternalServerError, "EXECUTION_GET_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}

func (h *Handler) RetryExecution(c *gin.Context) {
	item, err := h.service.RetryExecution(c.Request.Context(), c.Param("executionId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "EXECUTION_NOT_FOUND", "report execution not found"); return }
	if err != nil { response.Fail(c, http.StatusBadGateway, "EXECUTION_RETRY_FAILED", err.Error()); return }
	response.OK(c, http.StatusAccepted, item)
}

func (h *Handler) PreviewRecipients(c *gin.Context) {
	var req struct {
		Recipients []ScheduleRecipient `json:"recipients" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { response.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error()); return }
	preview, err := h.service.PreviewRecipients(c.Request.Context(), req.Recipients)
	if err != nil { response.Fail(c, http.StatusBadRequest, "RECIPIENT_RESOLUTION_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, preview)
}

func (h *Handler) ListPortalReports(c *gin.Context) {
	limit, ok := parseLimit(c, 100)
	if !ok { return }
	items, err := h.service.ListPortalReports(c.Request.Context(), c.GetString("user_id"), limit)
	if err != nil { response.Fail(c, http.StatusInternalServerError, "PORTAL_REPORTS_LIST_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, items)
}

func parseListOptions(c *gin.Context, allowEnabled bool) (ListOptions, bool) {
	limit, ok := parseLimit(c, 100)
	if !ok { return ListOptions{}, false }
	options := ListOptions{Limit: limit, Status: strings.TrimSpace(c.Query("status")), Search: strings.TrimSpace(c.Query("search"))}
	if len(options.Search) > 200 { response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "search must be 200 characters or fewer"); return ListOptions{}, false }
	if allowEnabled && strings.TrimSpace(c.Query("enabled")) != "" {
		value, err := strconv.ParseBool(c.Query("enabled"))
		if err != nil { response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "enabled must be true or false"); return ListOptions{}, false }
		options.Enabled = &value
	}
	if !allowEnabled && options.Status != "" {
		valid := map[string]bool{"queued":true,"generating":true,"polling":true,"generated":true,"delivering":true,"completed":true,"failed":true,"cancelled":true,"retrying":true}
		if !valid[options.Status] { response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "invalid execution status filter"); return ListOptions{}, false }
	}
	return options, true
}

func parseLimit(c *gin.Context, defaultLimit int) (int, bool) {
	limit := defaultLimit
	if value := strings.TrimSpace(c.Query("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 200 { response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "limit must be between 1 and 200"); return 0, false }
		limit = parsed
	}
	return limit, true
}

func (h *Handler) DownloadArtifact(c *gin.Context) {
	_, target, err := h.service.ArtifactDownloadURL(c.Request.Context(), c.Param("artifactId"), c.GetString("user_id"), canManage(c), healthContextFromGin(c))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "ARTIFACT_NOT_FOUND", "report artifact not found"); return }
	if err != nil { response.Fail(c, http.StatusBadGateway, "ARTIFACT_DOWNLOAD_FAILED", err.Error()); return }
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Redirect(http.StatusFound, target)
}

func (h *Handler) DownloadDeliveryLink(c *gin.Context) {
	_, target, err := h.service.PublicDeliveryDownloadURL(c.Request.Context(), c.Param("token"))
	if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "DELIVERY_LINK_INVALID", "report delivery link is invalid or expired"); return }
	if err != nil { response.Fail(c, http.StatusBadGateway, "DELIVERY_DOWNLOAD_FAILED", "report download is currently unavailable"); return }
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	c.Redirect(http.StatusFound, target)
}

func canManage(c *gin.Context) bool { ctx, ok := authz.FromGin(c); return ok && ctx.HasPermission(authz.PermissionReportSchedulerManage) }
func healthContextFromGin(c *gin.Context) HealthContext {
	value, ok := c.Get("health_context")
	if !ok {
		return HealthContext{}
	}
	raw, ok := value.(map[string]string)
	if !ok {
		return HealthContext{}
	}
	return HealthContext{District: strings.TrimSpace(raw["district"]), Facility: strings.TrimSpace(raw["facility"])}
}
func healthBIFailure(c *gin.Context, err error) {
	code := "HEALTH_BI_REQUEST_FAILED"; status := http.StatusBadGateway
	if strings.Contains(strings.ToLower(err.Error()), "not configured") { code = "HEALTH_BI_UNAVAILABLE"; status = http.StatusServiceUnavailable }
	response.Fail(c, status, code, err.Error())
}
