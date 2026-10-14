package visualiser

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/http/response"
)

type Handler struct {
	service Service
}

func NewHandler(
	config *config.Config,
	db *sql.DB,
) *Handler {
	_ = config
	return &Handler{
		service: NewService(NewRepository(db)),
	}
}

// GetDatasets gets all datasets.
func (h *Handler) GetDatasets(c *gin.Context) {
	results, err := h.service.ListDatasets(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
		return
	}
	response.OK(c, http.StatusOK, toDatasetResponses(results))
}

// GetDataElements gets data elements, optionally filtered by data_set_id.
func (h *Handler) GetDataElements(c *gin.Context) {
	var req DataElementsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		req = DataElementsRequest{}
	}

	results, err := h.service.ListDataElements(c.Request.Context(), req.DataSetID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
		return
	}
	response.OK(c, http.StatusOK, toDataElementResponses(results))
}

// GetDataValues gets data values with optional filters.
func (h *Handler) GetDataValues(c *gin.Context) {
	var req DataValuesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON")
		return
	}

	results, err := h.service.ListDataValues(c.Request.Context(), req)
	if err != nil {
		log.Printf(
			"[VISUALIZER DATAVALUES] request_id=%s dx=%v pe=%v ou=%v levelOfCare=%v ownership=%v startDate=%q endDate=%q orgunitLevel=%v aggregationLevel=%q error=%v",
			c.GetString("request_id"),
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
		)
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
		return
	}
	response.OK(c, http.StatusOK, toDataValuesResponse(results))
}

func normalizeOrgunitLevel(v any) *string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return &s
	case float64:
		// JSON numbers decode to float64 by default.
		s := strconv.FormatInt(int64(x), 10)
		return &s
	default:
		return nil
	}
}

func aggregationExpressions(level string) (orgUnitExpr, facilityExpr, levelExpr, regionExpr, districtExpr, subCountyExpr string) {
	switch level {
	case "1":
		return "'National'", "'National'", "'1'", "''", "''", "''"
	case "2":
		return "COALESCE(NULLIF(hs.region, ''), 'Unknown Region')", "COALESCE(NULLIF(hs.region, ''), 'Unknown Region')", "'2'", "COALESCE(NULLIF(hs.region, ''), 'Unknown Region')", "''", "''"
	case "3":
		return "COALESCE(NULLIF(hs.district, ''), 'Unknown District')", "COALESCE(NULLIF(hs.district, ''), 'Unknown District')", "'3'", "COALESCE(hs.region, '')", "COALESCE(NULLIF(hs.district, ''), 'Unknown District')", "''"
	case "5":
		return "COALESCE(NULLIF(hs.sub_county, ''), 'Unknown Subcounty')", "COALESCE(NULLIF(hs.sub_county, ''), 'Unknown Subcounty')", "'5'", "COALESCE(hs.region, '')", "COALESCE(hs.district, '')", "COALESCE(NULLIF(hs.sub_county, ''), 'Unknown Subcounty')"
	default:
		return "hs.org_unit_id", "COALESCE(NULLIF(hs.facility, ''), hs.org_unit_id)", "COALESCE(NULLIF(hs.\"level\", ''), '6')", "COALESCE(hs.region, '')", "COALESCE(hs.district, '')", "COALESCE(hs.sub_county, '')"
	}
}
