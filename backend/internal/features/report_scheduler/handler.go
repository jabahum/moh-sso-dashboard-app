package report_scheduler

import (
	"errors"
	"net/http"
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
	items, err := h.service.ListSchedules(c.Request.Context(), c.GetString("user_id"), canManage(c)); if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_LIST_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, items)
}
func (h *Handler) GetSchedule(c *gin.Context) {
	item, err := h.service.GetSchedule(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c)); if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }; if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_GET_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}
func (h *Handler) UpdateSchedule(c *gin.Context) {
	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil { response.Fail(c, http.StatusBadRequest, "INVALID_BODY", err.Error()); return }
	item, err := h.service.UpdateSchedule(c.Request.Context(), c.Param("scheduleId"), req, c.GetString("user_id"), canManage(c), healthContextFromGin(c)); if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }; if err != nil { response.Fail(c, http.StatusBadRequest, "SCHEDULE_UPDATE_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, item)
}
func (h *Handler) DeleteSchedule(c *gin.Context) {
	err := h.service.DeleteSchedule(c.Request.Context(), c.Param("scheduleId"), c.GetString("user_id"), canManage(c)); if errors.Is(err, ErrScheduleNotFound) { response.Fail(c, http.StatusNotFound, "SCHEDULE_NOT_FOUND", "report schedule not found"); return }; if err != nil { response.Fail(c, http.StatusInternalServerError, "SCHEDULE_DELETE_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, gin.H{"deleted": true})
}
func (h *Handler) ListExecutions(c *gin.Context) {
	items, err := h.service.ListExecutions(c.Request.Context(), c.GetString("user_id"), canManage(c)); if err != nil { response.Fail(c, http.StatusInternalServerError, "EXECUTION_LIST_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, items)
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
	items, err := h.service.ListPortalReports(c.Request.Context(), c.GetString("user_id"))
	if err != nil { response.Fail(c, http.StatusInternalServerError, "PORTAL_REPORTS_LIST_FAILED", err.Error()); return }
	response.OK(c, http.StatusOK, items)
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
