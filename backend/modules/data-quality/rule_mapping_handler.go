package dataquality

import (
	"net/http"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type ruleMappingPreviewRequest struct {
	TableID string `json:"table_id" binding:"required"`
}

type ruleMappingApplyRequest struct {
	TableID string   `json:"table_id" binding:"required"`
	Codes   []string `json:"codes" binding:"required"`
}

func (h *handlers) PreviewRuleMapping(c *modulekit.Context) {
	if h.dqaStore == nil || h.dwhDB == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req ruleMappingPreviewRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	candidates, err := h.dqaStore.PreviewRuleMapping(c.Context(), h.dwhDB, strings.TrimSpace(req.TableID))
	if err != nil {
		c.Fail(http.StatusBadRequest, "RULE_MAPPING_PREVIEW_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, candidates)
}

func (h *handlers) ApplyRuleMapping(c *modulekit.Context) {
	if h.dqaStore == nil {
		c.Fail(http.StatusServiceUnavailable, "DWH_UNAVAILABLE", "DWH connection is not configured")
		return
	}
	var req ruleMappingApplyRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	result, err := h.dqaStore.ApplyRuleMapping(c.Context(), strings.TrimSpace(req.TableID),
		req.Codes, h.callerName(c))
	if err != nil {
		c.Fail(http.StatusBadRequest, "RULE_MAPPING_APPLY_FAILED", err.Error())
		return
	}
	c.OK(http.StatusOK, result)
}
