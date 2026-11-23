package surveillance

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

// ID is the module's registry identifier.
const ID = "surveillance"

// System is the platform system every surveillance endpoint belongs to.
const System = "data-statistics"

// Permissions checked by the module's endpoints.
const (
	ReadPermission            = "surveillance:read"
	ImportPermission          = "surveillance:import"
	ManageLocationsPermission = "surveillance:manage_locations"
)

// Rate limits applied to write endpoints, per user.
var (
	ImportRateLimit = modulekit.RateLimit{Name: "surveillance-import", Limit: 10, Window: time.Minute}
	ManageRateLimit = modulekit.RateLimit{Name: "surveillance-manage", Limit: 10, Window: time.Minute}
)

//go:embed openapi.json
var openAPIDocument []byte

// Dependencies are supplied by the host.
type Dependencies struct {
	// Database is the Observatory (surveillance) database. Required.
	Database *sql.DB
	// GeographyPrimary and ReplicateGeography route every geography write
	// (regions, districts, sub-counties, facilities) through the primary
	// registry and then replicate it into the Observatory. Supply both or
	// neither.
	GeographyPrimary   *sql.DB
	ReplicateGeography func(context.Context) error
	// HealthContext resolves the caller's selected health context. Required.
	HealthContext modulekit.HealthContextDirectory
	// Available reports Observatory readiness; while it returns false every
	// endpoint answers 503. Optional (always available when nil).
	Available func() bool
	// Imports is the host's document pipeline used by CSV imports. Optional;
	// without it ImportCSV fails.
	Imports ImportHost
	// Notifier receives admin notifications. Optional.
	Notifier modulekit.Notifier
	Platform modulekit.Platform
	// Limiter enforces the write endpoints' rate limits. Optional.
	Limiter modulekit.Limiter
	Logger  *slog.Logger
}

// Module is the mounted HTTP module plus the import processors the host's
// document worker runs.
type Module struct {
	*modulekit.HTTPModule
	handler *Handler
	csv     *SurveillanceCSVProcessor
	batches *SurveillanceBatchProcessor
}

// NewModule validates dependencies and builds the module.
func NewModule(deps Dependencies) (*Module, error) {
	if deps.Database == nil {
		return nil, errors.New("surveillance: Database is required")
	}
	if (deps.GeographyPrimary == nil) != (deps.ReplicateGeography == nil) {
		return nil, errors.New("surveillance: GeographyPrimary and ReplicateGeography must be supplied together")
	}
	var store db.Store = db.NewStore(deps.Database)
	if deps.GeographyPrimary != nil {
		store = db.NewGeographyStore(store, deps.GeographyPrimary, deps.ReplicateGeography)
	}
	return newModule(deps, store)
}

// newModule builds the module over a store (tests supply an in-memory one).
func newModule(deps Dependencies, store db.Store) (*Module, error) {
	if deps.HealthContext == nil {
		return nil, errors.New("surveillance: HealthContext is required")
	}
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	repos := NewRepositories(store)
	platform := deps.Platform

	epiWeeks := NewEpiWeekService(log, repos.EpiWeeks, deps.Notifier)
	diseases := NewDiseaseService(log, repos.Diseases, deps.Notifier, &platform)
	locations := NewLocationService(log, repos.Regions, repos.Districts, repos.SubCounties, deps.Notifier, &platform)
	facilityMetrics := NewFacilityWeeklyMetricsService(log, repos.FacilityMetrics, repos.Imports)
	weeklyStatus := NewWeeklyStatusService(log, repos.WeeklyStatus)
	imports := NewImportService(repos.Imports)
	alerts := NewAlertService(log, repos.Alerts, repos.Imports, deps.Notifier, &platform)

	h := NewHandler(epiWeeks, diseases, locations, facilityMetrics, weeklyStatus, alerts, imports, deps.HealthContext)
	h.SetAvailabilityCheck(deps.Available)

	read := func(method, path string, handle func(*modulekit.Context)) modulekit.Endpoint {
		return modulekit.Endpoint{Method: method, Path: path, System: System, Permission: ReadPermission, Handle: handle}
	}
	importData := func(method, path string, handle func(*modulekit.Context)) modulekit.Endpoint {
		limit := ImportRateLimit
		return modulekit.Endpoint{Method: method, Path: path, System: System, Permission: ImportPermission, RateLimit: &limit, Handle: handle}
	}
	manageLocations := func(method, path string, handle func(*modulekit.Context)) modulekit.Endpoint {
		limit := ManageRateLimit
		return modulekit.Endpoint{Method: method, Path: path, System: System, Permission: ManageLocationsPermission, RateLimit: &limit, Handle: handle}
	}
	var options []modulekit.RouterOption
	if deps.Limiter != nil {
		options = append(options, modulekit.WithLimiter(deps.Limiter))
	}
	router, err := modulekit.NewRouter([]modulekit.Endpoint{
		read("GET", "/surveillance/weeks", h.ListEpiWeeksByYear),
		read("GET", "/surveillance/alerts", h.ListAlerts),
		read("GET", "/surveillance/diseases", h.ListDiseases),
		read("GET", "/surveillance/regions", h.ListRegions),

		read("GET", "/surveillance/districts", h.ListDistricts),
		read("GET", "/surveillance/regions/{regionID}/districts", h.ListDistrictsByRegion),
		manageLocations("POST", "/surveillance/districts", h.UpsertDistrict),

		read("GET", "/surveillance/districts/{districtID}/subcounties", h.ListSubcountiesByDistrict),
		read("GET", "/surveillance/subcounties/{id}", h.GetSubcountyByID),
		manageLocations("POST", "/surveillance/subcounties", h.UpsertSubcounty),
		manageLocations("DELETE", "/surveillance/subcounties/{id}", h.DeleteSubcounty),

		read("GET", "/surveillance/facility-weekly-metrics/week/{epiWeekID}", h.ListFacilityWeeklyMetricsByWeek),
		read("GET", "/surveillance/facility-weekly-metrics/facility/{facilityID}", h.ListFacilityWeeklyMetricsByFacility),
		read("GET", "/surveillance/facility-weekly-metrics/facility/{facilityID}/disease/{diseaseID}/trend", h.ListFacilityDiseaseMetricsTrend),
		read("GET", "/surveillance/facility-weekly-metrics/facility/{facilityID}/indicator/{indicatorID}/trend", h.ListFacilityIndicatorMetricsTrend),
		read("GET", "/surveillance/facility-weekly-metrics/week/{epiWeekID}/disease/{diseaseID}", h.ListFacilityDiseaseMetricsByWeekAndDisease),
		read("GET", "/surveillance/facility-weekly-metrics/disease-trend", h.ListDiseaseWeeklyTrendAggregated),

		read("GET", "/surveillance/weekly-statuses/list", h.ListWeeklyStatuses),
		read("GET", "/surveillance/weekly-statuses/detailed", h.ListWeeklyStatusesDetailed),
		read("GET", "/surveillance/weekly-statuses/district/week/{epiWeekID}", h.ListDistrictWeeklyStatusesByWeek),
		read("GET", "/surveillance/weekly-statuses/region/week/{epiWeekID}", h.ListRegionWeeklyStatusesByWeek),
		read("GET", "/surveillance/weekly-statuses/national/week/{epiWeekID}", h.ListNationalWeeklyStatusesByWeek),

		read("GET", "/surveillance/imports", h.ListImportBatches),
		importData("POST", "/surveillance/imports", h.CreateImportBatch),
		read("GET", "/surveillance/imports/{batchID}", h.GetImportBatchByID),
		importData("PATCH", "/surveillance/imports/{batchID}/status", h.UpdateImportBatchStatus),
		read("GET", "/surveillance/imports/{batchID}/raw-rows", h.ListImportRawRowsByBatch),
	}, options...)
	if err != nil {
		return nil, err
	}
	httpModule, err := modulekit.NewHTTPModule(router, openAPIDocument)
	if err != nil {
		return nil, err
	}
	return &Module{
		HTTPModule: httpModule,
		handler:    h,
		csv:        NewSurveillanceCSVProcessor(deps.Imports, repos.Imports),
		batches:    NewSurveillanceBatchProcessor(repos.Imports, facilityMetrics, alerts),
	}, nil
}

// ServeHTTP answers 503 while the Observatory is unavailable, before the
// endpoint's permission and rate-limit checks (the order the host's route
// group applied before extraction), then serves the endpoint.
func (m *Module) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.handler.requireAvailable(w, r) {
		return
	}
	m.HTTPModule.ServeHTTP(w, r)
}

// ImportCSV runs a surveillance CSV import job: it stores the document's rows
// as a raw import batch and queues the batch-process job.
func (m *Module) ImportCSV(ctx context.Context, job ImportJob) error {
	return m.csv.Process(ctx, job)
}

// ProcessImportBatch runs a surveillance batch-process job: it processes the
// latest import batch of the job's document into facility metrics or alerts.
func (m *Module) ProcessImportBatch(ctx context.Context, job ImportJob) error {
	return m.batches.Process(ctx, job)
}
