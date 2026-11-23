package dataquality

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type scheduleRunRequest struct {
	TableID     string    `json:"table_id" binding:"required"`
	Scope       RunScope  `json:"scope"`
	ScheduledAt time.Time `json:"scheduled_at" binding:"required"`
}

func (h *handlers) ScheduleDQARun(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req scheduleRunRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.ScheduledAt.Before(time.Now()) {
		c.Fail(http.StatusBadRequest, "INVALID_SCHEDULE", "scheduled_at is in the past")
		return
	}
	scheduled, err := h.dqaStore.ScheduleRun(c.Context(), strings.TrimSpace(req.TableID),
		req.Scope, req.ScheduledAt, h.callerName(c))
	if err != nil {
		c.Fail(http.StatusBadRequest, "DQA_SCHEDULE_FAILED", err.Error())
		return
	}
	c.OK(http.StatusCreated, scheduled)
}

func (h *handlers) ListDQAScheduledRuns(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	status := c.Query("status")
	switch status {
	case "", "pending", "running", "done", "failed", "cancelled":
	default:
		c.Fail(http.StatusBadRequest, "INVALID_STATUS", "unknown status")
		return
	}
	runs, err := h.dqaStore.ListScheduledRuns(c.Context(), status,
		parseListLimit(c, 50, 200), parseListOffset(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_SCHEDULES_LIST_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, runs)
}

func (h *handlers) CancelDQAScheduledRun(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	id, err := strconv.ParseInt(c.Param("scheduleId"), 10, 64)
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_SCHEDULE_ID", "scheduleId must be an integer")
		return
	}
	scheduled, err := h.dqaStore.CancelScheduledRun(c.Context(), id)
	if errors.Is(err, errScheduledRunNotFound) {
		c.Fail(http.StatusNotFound, "SCHEDULE_NOT_FOUND", "no pending scheduled run with that id")
		return
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_SCHEDULE_CANCEL_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, scheduled)
}
