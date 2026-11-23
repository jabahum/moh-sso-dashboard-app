package visualiser

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type handlers struct {
	service        Service
	healthContexts modulekit.HealthContextDirectory
	logger         *slog.Logger
}

// getDatasets lists the current warehouse datasets.
func (h handlers) getDatasets(c *modulekit.Context) {
	results, err := h.service.ListDatasets(c.Context())
	if err != nil {
		c.Fail(http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
		return
	}
	c.OK(http.StatusOK, toDatasetResponses(results))
}

// getDataElements lists data elements, optionally filtered by data_set_id.
// A present but empty data_set_id is treated as no filter.
func (h handlers) getDataElements(c *modulekit.Context) {
	var dataSetID *string
	if value, ok := c.GetQuery("data_set_id"); ok {
		dataSetID = &value
	}

	results, err := h.service.ListDataElements(c.Context(), dataSetID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
		return
	}
	c.OK(http.StatusOK, toDataElementResponses(results))
}

// getDataValues aggregates data values within the selected health context.
func (h handlers) getDataValues(c *modulekit.Context) {
	var req DataValuesRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "BAD_REQUEST", "invalid JSON")
		return
	}
	req, err := h.scopeDataValuesRequest(c, req)
	if err != nil {
		if errors.Is(err, errHealthContextAliasMissing) {
			c.Fail(
				http.StatusForbidden,
				"HEALTH_CONTEXT_MAPPING_REQUIRED",
				"selected health context is not mapped to a DWH organisation unit",
			)
			return
		}
		c.Fail(http.StatusServiceUnavailable, "HEALTH_CONTEXT_UNAVAILABLE", "health context could not be resolved")
		return
	}

	results, err := h.service.ListDataValues(c.Context(), req)
	if err != nil {
		h.logger.Error(fmt.Sprintf(
			"[VISUALIZER DATAVALUES] request_id=%s dx=%v pe=%v ou=%v levelOfCare=%v ownership=%v startDate=%q endDate=%q orgunitLevel=%v aggregationLevel=%q error=%v",
			c.RequestID(),
			req.DX,
			req.PE,
			req.OU,
			req.LevelOfCare,
			req.Ownership,
			req.StartDate,
			req.EndDate,
			req.OrgunitLevel,
			req.AggregationLevel,
			err,
		))
		c.Fail(http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
		return
	}
	c.OK(http.StatusOK, toDataValuesResponse(results))
}
