package surveillance

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

// ErrDeletionRestricted is returned when a shared geography record (region,
// district, sub-county or facility) would be deleted. Geography is owned by
// the primary registry; operators disable its health context instead.
var ErrDeletionRestricted = db.ErrDeletionRestricted

type Handler struct {
	available                    func() bool
	epiWeekService               *EpiWeekService
	diseaseService               *DiseaseService
	locationService              *LocationService
	facilityWeeklyMetricsService *FacilityWeeklyMetricsService
	weeklyStatusService          *WeeklyStatusService
	alertService                 *AlertService
	importService                *ImportService
	healthContextService         modulekit.HealthContextDirectory
}

func NewHandler(
	epiWeekService *EpiWeekService,
	diseaseService *DiseaseService,
	locationService *LocationService,
	facilityWeeklyMetricsService *FacilityWeeklyMetricsService,
	weeklyStatusService *WeeklyStatusService,
	alertService *AlertService,
	importService *ImportService,
	healthContextService modulekit.HealthContextDirectory,
) *Handler {
	return &Handler{
		epiWeekService:               epiWeekService,
		diseaseService:               diseaseService,
		locationService:              locationService,
		facilityWeeklyMetricsService: facilityWeeklyMetricsService,
		weeklyStatusService:          weeklyStatusService,
		alertService:                 alertService,
		importService:                importService,
		healthContextService:         healthContextService,
	}
}

// SetAvailabilityCheck gates only Observatory-backed routes during initialization
// and outages. It does not disable any authentication or authorization checks.
func (h *Handler) SetAvailabilityCheck(available func() bool) { h.available = available }

// requireAvailable answers 503 while the Observatory is unavailable. It runs
// before the endpoint's permission and rate-limit checks, as the route group
// middleware did before extraction.
func (h *Handler) requireAvailable(w http.ResponseWriter, r *http.Request) bool {
	if h.available != nil && !h.available() {
		w.Header().Set("Retry-After", "15")
		modulekit.WriteFail(w, r, http.StatusServiceUnavailable, "OBSERVATORY_UNAVAILABLE", "Surveillance is temporarily unavailable. Please try again shortly.")
		return false
	}
	return true
}

func (h *Handler) healthContextScopeFromRequest(c *modulekit.Context) (HealthContextScope, bool) {
	selection := c.HealthContext()
	if selection == nil {
		return HealthContextScope{}, false
	}
	id, err := uuid.Parse(selection.NodeID)
	if err != nil || id == uuid.Nil {
		return HealthContextScope{}, false
	}
	return HealthContextScope{
		ID:                 id,
		service:            h.healthContextService,
		IncludeDescendants: selection.IncludeDescendants,
	}, true
}

func (h *Handler) requireAliasInHealthContext(
	c *modulekit.Context,
	namespace string,
	externalID uuid.UUID,
) bool {
	scope, scoped := h.healthContextScopeFromRequest(c)
	if !scoped {
		return true
	}
	allowed, err := scope.containsAlias(c.Context(), h.healthContextService, namespace, externalID)
	if err != nil || !allowed {
		c.Fail(http.StatusForbidden, "resource is outside the selected health context", "access denied")
		return false
	}
	return true
}

func parseWeeklyStatusListInput(c *modulekit.Context, validateStatus bool) (WeeklyStatusListInput, bool) {
	var input WeeklyStatusListInput

	if epiWeekID := c.Query("epiWeekID"); epiWeekID != "" {
		id, err := uuid.Parse(epiWeekID)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid epiWeekID", "request failed")
			return input, false
		}
		input.EpiWeekID = id
	}

	if regionID := c.Query("regionID"); regionID != "" {
		id, err := uuid.Parse(regionID)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid regionID", "request failed")
			return input, false
		}
		input.RegionID = id
	}

	if districtID := c.Query("districtID"); districtID != "" {
		id, err := uuid.Parse(districtID)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid districtID", "request failed")
			return input, false
		}
		input.DistrictID = id
	}

	if subCountyID := c.Query("subCountyID"); subCountyID != "" {
		id, err := uuid.Parse(subCountyID)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid subCountyID", "request failed")
			return input, false
		}
		input.SubCountyID = id
	}

	if diseaseID := c.Query("diseaseID"); diseaseID != "" {
		id, err := uuid.Parse(diseaseID)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid diseaseID", "request failed")
			return input, false
		}
		input.DiseaseID = id
	}

	if indicatorID := c.Query("indicatorID"); indicatorID != "" {
		id, err := uuid.Parse(indicatorID)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid indicatorID", "request failed")
			return input, false
		}
		input.IndicatorID = id
	}

	if status := c.Query("status"); status != "" {
		if validateStatus && !isValidRiskLevel(status) {
			c.Fail(http.StatusBadRequest, "invalid status", status)
			return input, false
		}
		input.Status = status
	}

	return input, true
}

func isValidRiskLevel(status string) bool {
	switch status {
	case "MAROON", "RED", "YELLOW", "GREEN":
		return true
	default:
		return false
	}
}

// weeks
func (h *Handler) ListEpiWeeksByYearWeek(c *modulekit.Context) {
	ctx := c.Context()

	yearStr := c.Query("year")
	if yearStr == "" {
		c.Fail(http.StatusBadRequest, "year is required", "nil")
		return
	}

	weekStr := c.Query("week")
	if weekStr == "" {
		c.Fail(http.StatusBadRequest, "week is required", "nil")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid year", "request failed")
		return
	}

	week, err := strconv.Atoi(weekStr)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid week", "request failed")
		return
	}

	data, err := h.epiWeekService.GetEpiWeekByYearWeekValue(ctx, int32(year), int32(week))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to get epi week by year and week", "request failed")
		return
	}

	c.OK(http.StatusOK, toEpiWeekResponse(data))
}

func (h *Handler) ListEpiWeeksByYear(c *modulekit.Context) {
	ctx := c.Context()

	yearStr := c.Query("year")
	if yearStr == "" {
		c.Fail(http.StatusBadRequest, "year is required", "nil")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid year", "request failed")
		return
	}

	data, err := h.epiWeekService.ListEpiWeeksByYear(ctx, int32(year))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list epi weeks by year", "request failed")
		return
	}

	c.OK(http.StatusOK, toEpiWeekResponses(data))
}

// diseases
func (h *Handler) ListDiseases(c *modulekit.Context) {
	ctx := c.Context()

	data, err := h.diseaseService.ListDiseases(ctx)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list diseases", "request failed")
		return
	}

	c.OK(http.StatusOK, toDiseaseResponses(data))
}

// locations
func (h *Handler) ListRegions(c *modulekit.Context) {
	ctx := c.Context()

	scope, scoped := h.healthContextScopeFromRequest(c)
	var (
		data []db.Region
		err  error
	)
	if scoped {
		data, err = h.locationService.ListRegionsInHealthContext(ctx, scope)
	} else {
		data, err = h.locationService.ListRegions(ctx)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list regions", "request failed")
		return
	}

	c.OK(http.StatusOK, toRegionResponses(data))
}

func (h *Handler) ListDistricts(c *modulekit.Context) {
	ctx := c.Context()

	scope, scoped := h.healthContextScopeFromRequest(c)
	var (
		data []db.ListDistrictsRow
		err  error
	)
	if scoped {
		data, err = h.locationService.ListDistrictsInHealthContext(ctx, scope)
	} else {
		data, err = h.locationService.ListDistricts(ctx)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list districts", "request failed")
		return
	}

	c.OK(http.StatusOK, toDistrictListResponses(data))
}

func (h *Handler) ListDistrictsByRegion(c *modulekit.Context) {
	ctx := c.Context()

	regionIDParam := c.Param("regionID")
	if regionIDParam == "" {
		c.Fail(http.StatusBadRequest, "region id is required", "nil")
		return
	}

	regionID, err := uuid.Parse(regionIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid region id", "request failed")
		return
	}
	if !h.requireAliasInHealthContext(c, healthContextNamespaceRegion, regionID) {
		return
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var data []db.District
	if scoped {
		data, err = h.locationService.ListDistrictsByRegionInHealthContext(ctx, regionID, scope)
	} else {
		data, err = h.locationService.ListDistrictsByRegion(ctx, regionID)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list districts by region", "request failed")
		return
	}

	c.OK(http.StatusOK, toDistrictResponses(data))
}
func (h *Handler) ListSubcountiesByDistrict(c *modulekit.Context) {
	ctx := c.Context()

	districtIDParam := c.Param("districtID")
	if districtIDParam == "" {
		c.Fail(http.StatusBadRequest, "districtID is required", "nil")
		return
	}

	districtID, err := uuid.Parse(districtIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid districtID", "request failed")
		return
	}
	if !h.requireAliasInHealthContext(c, healthContextNamespaceDistrict, districtID) {
		return
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var data []db.SubCounty
	if scoped {
		data, err = h.locationService.ListSubcountiesByDistrictInHealthContext(ctx, districtID, scope)
	} else {
		data, err = h.locationService.ListSubcountiesByDistrict(ctx, districtID)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list subcounties", "request failed")
		return
	}

	c.OK(http.StatusOK, toSubCountyResponses(data))
}

func (h *Handler) GetSubcountyByID(c *modulekit.Context) {
	ctx := c.Context()

	idParam := c.Param("id")
	if idParam == "" {
		c.Fail(http.StatusBadRequest, "id is required", "")
		return
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid id", "request failed")
		return
	}
	if !h.requireAliasInHealthContext(c, healthContextNamespaceSubCounty, id) {
		return
	}

	data, err := h.locationService.GetSubcountyByID(ctx, id)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to get subcounty", "request failed")
		return
	}

	c.OK(http.StatusOK, toSubCountyResponse(data))
}

func (h *Handler) UpsertDistrict(c *modulekit.Context) {
	ctx := c.Context()

	var req UpsertDistrictRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "invalid request body", "request failed")
		return
	}

	data, err := h.locationService.UpsertDistrictFromRequest(ctx, req)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to upsert district", "request failed")
		return
	}

	c.OK(http.StatusOK, toDistrictResponse(data))
}

func (h *Handler) UpsertSubcounty(c *modulekit.Context) {
	ctx := c.Context()

	var req UpsertSubCountyRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "invalid request body", "request failed")
		return
	}
	if req.DistrictID == nil || *req.DistrictID == uuid.Nil {
		c.Fail(http.StatusBadRequest, "invalid request body", "district_id is required")
		return
	}

	data, err := h.locationService.UpsertSubcountyFromRequest(ctx, req)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to upsert subcounty", "request failed")
		return
	}

	c.OK(http.StatusOK, toSubCountyResponse(data))
}

func (h *Handler) DeleteSubcounty(c *modulekit.Context) {
	ctx := c.Context()

	idParam := c.Param("id")
	if idParam == "" {
		c.Fail(http.StatusBadRequest, "id is required", "nil")
		return
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid id", "request failed")
		return
	}

	if err := h.locationService.DeleteSubcounty(ctx, id); err != nil {
		if errors.Is(err, ErrDeletionRestricted) {
			c.Fail(http.StatusConflict, "GEOGRAPHY_DELETE_RESTRICTED", "Shared geography cannot be deleted. Disable its health context instead.")
			return
		}
		c.Fail(http.StatusInternalServerError, "failed to delete subcounty", "request failed")
		return
	}

	c.OK(http.StatusOK, DeleteResponse{Deleted: true})
}

// facility weekly metrics
func (h *Handler) ListFacilityWeeklyMetricsByWeek(c *modulekit.Context) {
	ctx := c.Context()

	epiWeekIDParam := c.Param("epiWeekID")
	if epiWeekIDParam == "" {
		c.Fail(http.StatusBadRequest, "epiWeekID is required", "nil")
		return
	}

	epiWeekID, err := uuid.Parse(epiWeekIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid epiWeekID", "request failed")
		return
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var diseaseMetrics []db.ListFacilityWeeklyDiseaseMetricsByWeekRow
	if scoped {
		diseaseMetrics, err = h.facilityWeeklyMetricsService.
			ListFacilityWeeklyDiseaseMetricsByWeekInHealthContext(ctx, epiWeekID, scope)
	} else {
		diseaseMetrics, err = h.facilityWeeklyMetricsService.ListFacilityWeeklyDiseaseMetricsByWeek(ctx, epiWeekID)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list facility weekly disease metrics by week", "request failed")
		return
	}

	data := make([]FacilityWeeklyMetricResponse, 0, len(diseaseMetrics))
	data = append(data, toFacilityWeeklyDiseaseMetricResponses(diseaseMetrics)...)

	c.OK(http.StatusOK, data)
}

func (h *Handler) ListFacilityWeeklyMetricsByFacility(c *modulekit.Context) {
	ctx := c.Context()

	facilityIDParam := c.Param("facilityID")
	if facilityIDParam == "" {
		c.Fail(http.StatusBadRequest, "facilityID is required", "nil")
		return
	}

	facilityID, err := uuid.Parse(facilityIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid facilityID", "request failed")
		return
	}
	if !h.requireAliasInHealthContext(c, healthContextNamespaceFacility, facilityID) {
		return
	}

	data, err := h.facilityWeeklyMetricsService.ListFacilityWeeklyMetricsByFacility(ctx, facilityID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list facility weekly metrics by facility", "request failed")
		return
	}

	c.OK(http.StatusOK, toFacilityMetricByFacilityResponses(data))
}

func (h *Handler) ListFacilityDiseaseMetricsTrend(c *modulekit.Context) {
	ctx := c.Context()

	facilityIDParam := c.Param("facilityID")
	if facilityIDParam == "" {
		c.Fail(http.StatusBadRequest, "facilityID is required", "nil")
		return
	}

	diseaseIDParam := c.Param("diseaseID")
	if diseaseIDParam == "" {
		c.Fail(http.StatusBadRequest, "diseaseID is required", "nil")
		return
	}

	facilityID, err := uuid.Parse(facilityIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid facilityID", "request failed")
		return
	}
	if !h.requireAliasInHealthContext(c, healthContextNamespaceFacility, facilityID) {
		return
	}

	diseaseID, err := uuid.Parse(diseaseIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid diseaseID", "request failed")
		return
	}

	data, err := h.facilityWeeklyMetricsService.ListFacilityDiseaseMetricsTrend(ctx, facilityID, diseaseID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list facility disease trend", "request failed")
		return
	}

	c.OK(http.StatusOK, toFacilityDiseaseMetricTrendResponses(data))
}

func (h *Handler) ListFacilityIndicatorMetricsTrend(c *modulekit.Context) {
	ctx := c.Context()

	facilityIDParam := c.Param("facilityID")
	if facilityIDParam == "" {
		c.Fail(http.StatusBadRequest, "facilityID is required", "nil")
		return
	}

	indicatorIDParam := c.Param("indicatorID")
	if indicatorIDParam == "" {
		c.Fail(http.StatusBadRequest, "indicatorID is required", "nil")
		return
	}

	facilityID, err := uuid.Parse(facilityIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid facilityID", "request failed")
		return
	}
	if !h.requireAliasInHealthContext(c, healthContextNamespaceFacility, facilityID) {
		return
	}

	indicatorID, err := uuid.Parse(indicatorIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid indicatorID", "request failed")
		return
	}

	data, err := h.facilityWeeklyMetricsService.ListFacilityIndicatorMetricsTrend(ctx, facilityID, indicatorID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list facility indicator trend", "request failed")
		return
	}

	c.OK(http.StatusOK, toFacilityIndicatorMetricTrendResponses(data))
}

func (h *Handler) ListFacilityDiseaseMetricsByWeekAndDisease(c *modulekit.Context) {
	ctx := c.Context()

	epiWeekIDParam := c.Param("epiWeekID")
	if epiWeekIDParam == "" {
		c.Fail(http.StatusBadRequest, "epiWeekID is required", "nil")
		return
	}

	diseaseIDParam := c.Param("diseaseID")
	if diseaseIDParam == "" {
		c.Fail(http.StatusBadRequest, "diseaseID is required", "nil")
		return
	}

	epiWeekID, err := uuid.Parse(epiWeekIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid epiWeekID", "request failed")
		return
	}

	diseaseID, err := uuid.Parse(diseaseIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid diseaseID", "request failed")
		return
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var rows []db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow
	if scoped {
		rows, err = h.facilityWeeklyMetricsService.
			ListFacilityDiseaseMetricsByWeekAndDiseaseInHealthContext(ctx, epiWeekID, diseaseID, scope)
	} else {
		rows, err = h.facilityWeeklyMetricsService.
			ListFacilityDiseaseMetricsByWeekAndDisease(ctx, epiWeekID, diseaseID)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list facility disease metrics by week and disease", "request failed")
		return
	}

	data := make([]FacilityWeeklyMetricResponse, 0, len(rows))
	data = append(data, toFacilityWeeklyDiseaseMetricByWeekAndDiseaseResponses(rows)...)

	c.OK(http.StatusOK, data)
}

func (h *Handler) ListDiseaseWeeklyTrendAggregated(c *modulekit.Context) {
	ctx := c.Context()

	epiYearParam := c.Query("epiYear")
	if epiYearParam == "" {
		c.Fail(http.StatusBadRequest, "epiYear is required", "nil")
		return
	}

	diseaseIDParam := c.Query("diseaseID")
	if diseaseIDParam == "" {
		c.Fail(http.StatusBadRequest, "diseaseID is required", "nil")
		return
	}

	epiYear64, err := strconv.ParseInt(epiYearParam, 10, 32)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid epiYear", "request failed")
		return
	}

	diseaseID, err := uuid.Parse(diseaseIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid diseaseID", "request failed")
		return
	}

	var regionID *uuid.UUID
	regionIDParam := c.Query("regionID")
	if regionIDParam != "" {
		parsedRegionID, err := uuid.Parse(regionIDParam)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid regionID", "request failed")
			return
		}
		regionID = &parsedRegionID
	}

	var districtID *uuid.UUID
	districtIDParam := c.Query("districtID")
	if districtIDParam != "" {
		parsedDistrictID, err := uuid.Parse(districtIDParam)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid districtID", "request failed")
			return
		}
		districtID = &parsedDistrictID
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var data []db.ListDiseaseWeeklyTrendAggregatedRow
	if scoped {
		data, err = h.facilityWeeklyMetricsService.ListDiseaseWeeklyTrendAggregatedInHealthContext(
			ctx, int32(epiYear64), diseaseID, regionID, districtID, scope,
		)
	} else {
		data, err = h.facilityWeeklyMetricsService.ListDiseaseWeeklyTrendAggregated(
			ctx, int32(epiYear64), diseaseID, regionID, districtID,
		)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list aggregated disease weekly trend", "request failed")
		return
	}

	c.OK(http.StatusOK, toDiseaseWeeklyTrendAggregateResponses(data))
}

// district weekly status

func (h *Handler) ListDistrictWeeklyStatusesByWeek(c *modulekit.Context) {
	ctx := c.Context()

	epiWeekIDParam := c.Param("epiWeekID")
	if epiWeekIDParam == "" {
		c.Fail(http.StatusBadRequest, "epiWeekID is required", "nil")
		return
	}

	epiWeekID, err := uuid.Parse(epiWeekIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid epiWeekID", "request failed")
		return
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var data []db.WeeklyStatus
	if scoped {
		data, err = h.weeklyStatusService.ListLevelByWeekInHealthContext(ctx, epiWeekID, "district", scope)
	} else {
		data, err = h.weeklyStatusService.ListDistrictByWeek(ctx, epiWeekID)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list district weekly statuses by week", "request failed")
		return
	}

	c.OK(http.StatusOK, data)
}

// weekly status

func (h *Handler) ListWeeklyStatuses(c *modulekit.Context) {
	ctx := c.Context()

	params, ok := parseWeeklyStatusListInput(c, false)
	if !ok {
		return
	}

	if params.RegionID != uuid.Nil &&
		!h.requireAliasInHealthContext(c, healthContextNamespaceRegion, params.RegionID) {
		return
	}
	if params.DistrictID != uuid.Nil &&
		!h.requireAliasInHealthContext(c, healthContextNamespaceDistrict, params.DistrictID) {
		return
	}
	if params.SubCountyID != uuid.Nil &&
		!h.requireAliasInHealthContext(c, healthContextNamespaceSubCounty, params.SubCountyID) {
		return
	}
	scope, scoped := h.healthContextScopeFromRequest(c)
	var (
		data []db.WeeklyStatus
		err  error
	)
	if scoped {
		data, err = h.weeklyStatusService.ListFromInputInHealthContext(ctx, params, scope)
	} else {
		data, err = h.weeklyStatusService.ListFromInput(ctx, params)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list weekly statuses", "request failed")
		return
	}

	c.OK(http.StatusOK, toWeeklyStatusResponses(data))
}

func (h *Handler) ListWeeklyStatusesDetailed(c *modulekit.Context) {
	ctx := c.Context()

	params, ok := parseWeeklyStatusListInput(c, true)
	if !ok {
		return
	}

	if params.RegionID != uuid.Nil &&
		!h.requireAliasInHealthContext(c, healthContextNamespaceRegion, params.RegionID) {
		return
	}
	if params.DistrictID != uuid.Nil &&
		!h.requireAliasInHealthContext(c, healthContextNamespaceDistrict, params.DistrictID) {
		return
	}
	if params.SubCountyID != uuid.Nil &&
		!h.requireAliasInHealthContext(c, healthContextNamespaceSubCounty, params.SubCountyID) {
		return
	}
	scope, scoped := h.healthContextScopeFromRequest(c)
	var (
		rows []db.ListWeeklyStatusesDetailedRow
		err  error
	)
	if scoped {
		rows, err = h.weeklyStatusService.ListDetailedFromInputInHealthContext(ctx, params, scope)
	} else {
		rows, err = h.weeklyStatusService.ListDetailedFromInput(ctx, params)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list weekly statuses detailed", "request failed")
		return
	}

	data := make([]WeeklyStatusDetailedResponse, 0, len(rows))
	for _, row := range rows {
		data = append(data, toWeeklyStatusDetailedResponse(row))
	}

	c.OK(http.StatusOK, data)
}

// region weekly status

func (h *Handler) ListRegionWeeklyStatusesByWeek(c *modulekit.Context) {
	ctx := c.Context()

	epiWeekIDParam := c.Param("epiWeekID")
	if epiWeekIDParam == "" {
		c.Fail(http.StatusBadRequest, "epiWeekID is required", "nil")
		return
	}

	epiWeekID, err := uuid.Parse(epiWeekIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid epiWeekID", "request failed")
		return
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var data []db.WeeklyStatus
	if scoped {
		data, err = h.weeklyStatusService.ListLevelByWeekInHealthContext(ctx, epiWeekID, "region", scope)
	} else {
		data, err = h.weeklyStatusService.ListRegionByWeek(ctx, epiWeekID)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list region weekly statuses by week", "request failed")
		return
	}

	c.OK(http.StatusOK, toWeeklyStatusResponses(data))
}

// national weekly status

func (h *Handler) ListNationalWeeklyStatusesByWeek(c *modulekit.Context) {
	ctx := c.Context()

	epiWeekIDParam := c.Param("epiWeekID")
	if epiWeekIDParam == "" {
		c.Fail(http.StatusBadRequest, "epiWeekID is required", "nil")
		return
	}

	epiWeekID, err := uuid.Parse(epiWeekIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid epiWeekID", "request failed")
		return
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var data []db.WeeklyStatus
	if scoped {
		data, err = h.weeklyStatusService.ListLevelByWeekInHealthContext(ctx, epiWeekID, "national", scope)
	} else {
		data, err = h.weeklyStatusService.ListNationalByWeek(ctx, epiWeekID)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list national weekly statuses by week", "request failed")
		return
	}

	c.OK(http.StatusOK, toWeeklyStatusResponses(data))
}

// alerts
func (h *Handler) ListAlerts(c *modulekit.Context) {
	ctx := c.Context()

	params := AlertListParams{}
	var err error

	if value := c.Query("epiWeekID"); value != "" {
		params.EpiWeekID, err = uuid.Parse(value)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid epiWeekID", "request failed")
			return
		}
	}

	if value := c.Query("diseaseID"); value != "" {
		params.DiseaseID, err = uuid.Parse(value)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid diseaseID", "request failed")
			return
		}
	}

	if value := c.Query("districtID"); value != "" {
		params.DistrictID, err = uuid.Parse(value)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid districtID", "request failed")
			return
		}
		if !h.requireAliasInHealthContext(c, healthContextNamespaceDistrict, params.DistrictID) {
			return
		}
	}

	if value := c.Query("regionID"); value != "" {
		params.RegionID, err = uuid.Parse(value)
		if err != nil {
			c.Fail(http.StatusBadRequest, "invalid regionID", "request failed")
			return
		}
		if !h.requireAliasInHealthContext(c, healthContextNamespaceRegion, params.RegionID) {
			return
		}
	}

	scope, scoped := h.healthContextScopeFromRequest(c)
	var data []db.ListAlertsRow
	if scoped {
		data, err = h.alertService.ListAlertsFromParamsInHealthContext(ctx, params, scope)
	} else {
		data, err = h.alertService.ListAlertsFromParams(ctx, params)
	}
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list alerts", "request failed")
		return
	}

	c.OK(http.StatusOK, toAlertResponses(data))
}

// imports

func (h *Handler) ListImportBatches(c *modulekit.Context) {
	ctx := c.Context()

	data, err := h.importService.ListImportBatches(ctx)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list import batches", "request failed")
		return
	}

	c.OK(http.StatusOK, toImportBatchResponses(data))
}

func (h *Handler) GetImportBatchByID(c *modulekit.Context) {
	ctx := c.Context()

	batchIDParam := c.Param("batchID")
	if batchIDParam == "" {
		c.Fail(http.StatusBadRequest, "batchID is required", "nil")
		return
	}

	batchID, err := uuid.Parse(batchIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid batchID", "request failed")
		return
	}

	data, err := h.importService.GetImportBatchByID(ctx, batchID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to get import batch", "request failed")
		return
	}

	c.OK(http.StatusOK, toImportBatchResponse(data))
}

func (h *Handler) ListImportRawRowsByBatch(c *modulekit.Context) {
	ctx := c.Context()

	batchIDParam := c.Param("batchID")
	if batchIDParam == "" {
		c.Fail(http.StatusBadRequest, "batchID is required", "nil")
		return
	}

	batchID, err := uuid.Parse(batchIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid batchID", "request failed")
		return
	}

	data, err := h.importService.ListImportRawRowsByBatch(ctx, batchID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to list import raw rows", "request failed")
		return
	}

	c.OK(http.StatusOK, toImportRawRowResponses(data))
}

func (h *Handler) CreateImportBatch(c *modulekit.Context) {
	ctx := c.Context()

	var req CreateImportBatchRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "invalid request body", "request failed")
		return
	}

	status := req.Status
	if status == "" {
		status = "pending"
	}

	data, err := h.importService.CreateImportBatchFromInput(ctx, CreateImportBatchInput{
		SourceName:  req.SourceName,
		FileName:    req.FileName,
		DatasetType: req.DatasetType,
		ImportedBy:  req.ImportedBy,
		Status:      status,
		Notes:       req.Notes,
		DocumentID:  req.DocumentID,
	})
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to create import batch", "request failed")
		return
	}

	c.OK(http.StatusCreated, toImportBatchResponse(data))
}

func (h *Handler) UpdateImportBatchStatus(c *modulekit.Context) {
	ctx := c.Context()

	batchIDParam := c.Param("batchID")
	if batchIDParam == "" {
		c.Fail(http.StatusBadRequest, "batchID is required", "nil")
		return
	}

	batchID, err := uuid.Parse(batchIDParam)
	if err != nil {
		c.Fail(http.StatusBadRequest, "invalid batchID", "request failed")
		return
	}

	var req UpdateImportBatchStatusRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "invalid request body", "request failed")
		return
	}

	data, err := h.importService.UpdateImportBatchStatusFromInput(ctx, UpdateImportBatchStatusInput{
		ID:     batchID,
		Status: req.Status,
		Notes:  req.Notes,
	})
	if err != nil {
		c.Fail(http.StatusInternalServerError, "failed to update import batch status", "request failed")
		return
	}

	c.OK(http.StatusOK, toImportBatchResponse(data))
}
