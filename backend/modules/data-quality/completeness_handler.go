package dataquality

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type completenessRunRequest struct {
	Period string `json:"period"`
}

func (h *handlers) RunCompleteness(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req completenessRunRequest
	if err := bindJSON(c, &req); err != nil && !errors.Is(err, io.EOF) {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	period, err := parseCompletenessPeriod(req.Period, time.Now())
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PERIOD", err.Error())
		return
	}
	run, err := h.dqaStore.RunCompleteness(c.Context(), period, h.callerName(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "COMPLETENESS_RUN_FAILED", err.Error())
		return
	}
	c.OK(http.StatusCreated, run)
}

func (h *handlers) ListCompletenessRuns(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	runs, err := h.dqaStore.ListCompletenessRuns(c.Context(), parseListLimit(c, 25, 200), parseListOffset(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "COMPLETENESS_RUNS_LIST_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, runs)
}

func (h *handlers) GetCompletenessRun(c *modulekit.Context) {
	runID, ok := h.completenessRunID(c)
	if !ok {
		return
	}
	run, err := h.dqaStore.GetCompletenessRun(c.Context(), runID)
	if errors.Is(err, errCompletenessRunNotFound) {
		c.Fail(http.StatusNotFound, "COMPLETENESS_RUN_NOT_FOUND", err.Error())
		return
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "COMPLETENESS_RUN_GET_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, run)
}

func (h *handlers) ListCompletenessDistricts(c *modulekit.Context) {
	runID, ok := h.completenessRunID(c)
	if !ok {
		return
	}
	districts, err := h.dqaStore.ListCompletenessDistricts(c.Context(), runID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "COMPLETENESS_DISTRICTS_LIST_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, districts)
}

func (h *handlers) ListCompletenessVHTs(c *modulekit.Context) {
	runID, ok := h.completenessRunID(c)
	if !ok {
		return
	}
	status := c.Query("status")
	if status != "" && status != "missing" && status != "reported" {
		c.Fail(http.StatusBadRequest, "INVALID_STATUS", "status must be missing or reported")
		return
	}
	filter := CompletenessVHTFilter{District: c.Query("district"), Status: status}
	vhts, err := h.dqaStore.ListCompletenessVHTs(c.Context(), runID, filter,
		parseListLimit(c, 100, 1000), parseListOffset(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "COMPLETENESS_VHTS_LIST_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, vhts)
}

func (h *handlers) completenessRunID(c *modulekit.Context) (int64, bool) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return 0, false
	}
	runID, err := strconv.ParseInt(c.Param("runId"), 10, 64)
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_RUN_ID", "runId must be an integer")
		return 0, false
	}
	return runID, true
}
