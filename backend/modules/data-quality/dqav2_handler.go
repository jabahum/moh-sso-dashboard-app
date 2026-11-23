package dataquality

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// ── tables ───────────────────────────────────────────────────────────────

func (h *handlers) ListDQATables(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	tables, err := h.dqaStore.ListTables(c.Context())
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_TABLES_LIST_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, tables)
}

func (h *handlers) UpsertDQATable(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req dqaTableRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if err := h.dqaStore.UpsertTable(c.Context(), req.toMapping()); err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_TABLE_SAVE_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, req.toMapping())
}

func (h *handlers) DeleteDQATable(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	tableID := c.Param("tableId")
	removed, err := h.dqaStore.DeleteTable(c.Context(), tableID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_TABLE_DELETE_FAILED", err.Error())
		return
	}
	if !removed {
		c.Fail(http.StatusNotFound, "NOT_FOUND", "table mapping not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// ── rules ────────────────────────────────────────────────────────────────

func (h *handlers) ListDQARules(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	tableID := c.Query("table_id")
	records, err := h.dqaStore.ListRules(c.Context(), tableID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_RULES_LIST_FAILED", err.Error())
		return
	}
	out := make([]dqaRuleResponse, 0, len(records))
	for _, r := range records {
		out = append(out, ruleRecordToResponse(r))
	}
	c.OK(http.StatusOK, out)
}

func (h *handlers) UpsertDQARule(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req dqaRuleRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	rule := req.toRule()

	// Compile before saving: a rule that can't be compiled is rejected
	// outright rather than silently stored broken.
	compiled, err := h.dqaStore.CompilePreview(c.Context(), rule)
	if err != nil {
		c.Fail(http.StatusBadRequest, "RULE_COMPILE_FAILED", err.Error())
		return
	}

	rec, err := h.dqaStore.UpsertRule(c.Context(), rule, compiled.MeasureSQL, h.callerName(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_RULE_SAVE_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, ruleRecordToResponse(rec))
}

func (h *handlers) DeleteDQARule(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	tableID, code := c.Param("tableId"), c.Param("code")
	removed, err := h.dqaStore.DeleteRule(c.Context(), tableID, code)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_RULE_DELETE_FAILED", err.Error())
		return
	}
	if !removed {
		c.Fail(http.StatusNotFound, "NOT_FOUND", "rule not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handlers) CompileDQARule(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req dqaRuleRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	compiled, err := h.dqaStore.CompilePreview(c.Context(), req.toRule())
	if err != nil {
		c.Fail(http.StatusBadRequest, "RULE_COMPILE_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, dqaCompilePreviewResponse{
		Kind: compiled.Kind, MeasureSQL: compiled.MeasureSQL, SampleSQL: compiled.SampleSQL,
	})
}

// ListDQAPhysicalColumns backs the column pickers in table configuration.
func (h *handlers) ListDQAPhysicalColumns(c *modulekit.Context) {
	if h.dwhDB == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	table := strings.TrimSpace(c.Query("table"))
	if table == "" {
		c.Fail(http.StatusBadRequest, "INVALID_TABLE", "table is required")
		return
	}
	columns, err := ListPhysicalColumns(c.Context(), h.dwhDB, table)
	if err != nil {
		c.Fail(http.StatusNotFound, "TABLE_NOT_FOUND", err.Error())
		return
	}
	c.OK(http.StatusOK, columns)
}

// ── runs + flags ─────────────────────────────────────────────────────────

func (h *handlers) RunDQATable(c *modulekit.Context) {
	if h.dqaStore == nil || h.dwhDB == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req dqaRunRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	result, err := h.dqaStore.RunTable(c.Context(), h.dwhDB, req.TableID, req.Scope, h.callerName(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_RUN_FAILED", err.Error())
		return
	}
	c.OK(http.StatusCreated, result)
}

func (h *handlers) ListDQARuns(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	tableID := c.Query("table_id")
	limit := parseListLimit(c, 25, 200)
	offset := parseListOffset(c)
	runs, err := h.dqaStore.ListRuns(c.Context(), tableID, limit, offset)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_RUNS_LIST_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, runs)
}

func (h *handlers) ListDQAFlags(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	runID, err := strconv.ParseInt(c.Param("runId"), 10, 64)
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_RUN_ID", "runId must be an integer")
		return
	}
	filter := FlagFilter{
		Severity:  c.Query("severity"),
		District:  c.Query("district"),
		Facility:  c.Query("facility"),
		VHT:       c.Query("vht"),
		Region:    c.Query("region"),
		Subcounty: c.Query("subcounty"),
		Year:      parseOptionalInt(c.Query("year")),
		Month:     parseOptionalInt(c.Query("month")),
	}
	limit := parseListLimit(c, 100, 1000)
	offset := parseListOffset(c)
	flags, err := h.dqaStore.ListFlags(c.Context(), runID, filter, limit, offset)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_FLAGS_LIST_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, flags)
}

func parseOptionalInt(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &v
}

// ListDQAFlagFilters returns the distinct filter values present in a run, so the
// dashboard's Year/Month/District/Facility/VHT dropdowns reflect real data.
func (h *handlers) ListDQAFlagFilters(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	runID, err := strconv.ParseInt(c.Param("runId"), 10, 64)
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_RUN_ID", "runId must be an integer")
		return
	}
	opts, err := h.dqaStore.FlagFilterOptions(c.Context(), runID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DQA_FLAG_FILTERS_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, opts)
}
