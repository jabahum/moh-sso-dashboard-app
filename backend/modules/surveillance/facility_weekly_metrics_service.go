package surveillance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"log/slog"
)

type facilityWeeklyMetricsPayload struct {
	RecordID  string  `json:"record_id"`
	Facility  string  `json:"facility"`
	Region    string  `json:"region"`
	District  string  `json:"district"`
	SubCounty string  `json:"sub_county"`
	Metric    string  `json:"disease"`
	Value     float64 `json:"value"`
	Year      int32   `json:"year"`
	Week      int32   `json:"weeks"`
	EpiWeek   int32   `json:"epi_week"`
}

type rawFacilityWeeklyMetricsPayload struct {
	RecordID  string          `json:"record_id"`
	Facility  string          `json:"facility"`
	Region    string          `json:"region"`
	District  string          `json:"district"`
	SubCounty string          `json:"sub_county"`
	Metric    string          `json:"disease"`
	Value     json.RawMessage `json:"value"`
	Year      json.RawMessage `json:"year"`
	Week      json.RawMessage `json:"weeks"`
	EpiWeek   json.RawMessage `json:"epi_week"`
}

type FacilityWeeklyMetricsService struct {
	log                       *slog.Logger
	facilityWeeklyMetricsRepo FacilityWeeklyMetricsRepository
	surveillanceImportRepo    ImportRepository
}

func NewFacilityWeeklyMetricsService(
	log *slog.Logger,
	facilityWeeklyMetricsRepo FacilityWeeklyMetricsRepository,
	surveillanceImportRepo ImportRepository,
) *FacilityWeeklyMetricsService {
	return &FacilityWeeklyMetricsService{
		log:                       log,
		facilityWeeklyMetricsRepo: facilityWeeklyMetricsRepo,
		surveillanceImportRepo:    surveillanceImportRepo,
	}
}

func (s *FacilityWeeklyMetricsService) ListFacilityWeeklyMetricsByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekRow, error) {
	s.log.DebugContext(ctx, "listing facility weekly metrics by epi week", "epi_week_id", epiWeekID)

	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekRow{}, errors.New("epi week id is required")
	}

	items, err := s.facilityWeeklyMetricsRepo.ListDiseaseMetricsByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility weekly metrics by epi week", "epi_week_id", epiWeekID, "error", err)
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekRow{}, err
	}

	return items, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityWeeklyMetricsByFacility(
	ctx context.Context,
	facilityID uuid.UUID,
) ([]db.ListFacilityMetricsByFacilityRow, error) {
	s.log.DebugContext(ctx, "listing facility weekly metrics by facility", "facility_id", facilityID)

	if facilityID == uuid.Nil {
		return []db.ListFacilityMetricsByFacilityRow{}, errors.New("facility id is required")
	}

	items, err := s.facilityWeeklyMetricsRepo.ListByFacility(ctx, facilityID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility weekly metrics by facility", "facility_id", facilityID, "error", err)
		return []db.ListFacilityMetricsByFacilityRow{}, err
	}

	return items, nil
}

func (s *FacilityWeeklyMetricsService) UpsertFacilityWeeklyIndicatorMetric(
	ctx context.Context,
	arg db.UpsertFacilityWeeklyIndicatorMetricParams,
) (db.FacilityWeeklyMetric, error) {
	s.log.DebugContext(ctx, "upserting facility weekly indicator metric")

	item, err := s.facilityWeeklyMetricsRepo.UpsertIndicatorMetric(ctx, arg)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to upsert facility weekly indicator metric", "error", err)
		return db.FacilityWeeklyMetric{}, err
	}

	return item, nil
}

func (s *FacilityWeeklyMetricsService) GetFacilityMetricBySourceRecordID(
	ctx context.Context,
	sourceRecordID string,
) (db.FacilityWeeklyMetric, error) {
	s.log.DebugContext(ctx, "getting facility metric by source record id", "source_record_id", sourceRecordID)

	item, err := s.facilityWeeklyMetricsRepo.GetBySourceRecordID(ctx, sourceRecordID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get facility metric by source record id", "source_record_id", sourceRecordID, "error", err)
		return db.FacilityWeeklyMetric{}, err
	}

	return item, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityWeeklyDiseaseMetricsByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekRow, error) {
	s.log.DebugContext(ctx, "listing facility weekly disease metrics by week", "epi_week_id", epiWeekID)

	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekRow{}, errors.New("epi week id is required")
	}

	items, err := s.facilityWeeklyMetricsRepo.ListDiseaseMetricsByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility weekly disease metrics by week", "epi_week_id", epiWeekID, "error", err)
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekRow{}, err
	}

	return items, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityWeeklyDiseaseMetricsByWeekInHealthContext(
	ctx context.Context,
	epiWeekID uuid.UUID,
	scope HealthContextScope,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekRow, error) {
	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekRow{}, errors.New("epi week id is required")
	}
	return s.facilityWeeklyMetricsRepo.ListDiseaseMetricsByWeekInHealthContext(ctx, epiWeekID, scope)
}

func (s *FacilityWeeklyMetricsService) ListFacilityWeeklyIndicatorMetricsByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.ListFacilityWeeklyIndicatorMetricsByWeekRow, error) {
	s.log.DebugContext(ctx, "listing facility weekly indicator metrics by week", "epi_week_id", epiWeekID)

	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyIndicatorMetricsByWeekRow{}, errors.New("epi week id is required")
	}

	items, err := s.facilityWeeklyMetricsRepo.ListIndicatorMetricsByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility weekly indicator metrics by week", "epi_week_id", epiWeekID, "error", err)
		return []db.ListFacilityWeeklyIndicatorMetricsByWeekRow{}, err
	}

	return items, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityDiseaseMetricsTrend(
	ctx context.Context,
	facilityID, diseaseID uuid.UUID,
) ([]db.ListFacilityDiseaseMetricsTrendRow, error) {
	s.log.DebugContext(ctx, "listing facility disease metrics trend",
		"facility_id", facilityID,
		"disease_id", diseaseID,
	)

	if facilityID == uuid.Nil {
		return []db.ListFacilityDiseaseMetricsTrendRow{}, errors.New("facility id is required")
	}

	if diseaseID == uuid.Nil {
		return []db.ListFacilityDiseaseMetricsTrendRow{}, errors.New("disease id is required")
	}

	rows, err := s.facilityWeeklyMetricsRepo.ListDiseaseTrend(ctx, db.ListFacilityDiseaseMetricsTrendParams{
		FacilityID: facilityID,
		DiseaseID: uuid.NullUUID{
			UUID:  diseaseID,
			Valid: diseaseID != uuid.Nil,
		},
	})
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility disease metrics trend",
			"facility_id", facilityID,
			"disease_id", diseaseID,
			"error", err,
		)
		return []db.ListFacilityDiseaseMetricsTrendRow{}, err
	}

	return rows, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityIndicatorMetricsTrend(
	ctx context.Context,
	facilityID, indicatorID uuid.UUID,
) ([]db.ListFacilityIndicatorMetricsTrendRow, error) {
	s.log.DebugContext(ctx, "listing facility indicator metrics trend",
		"facility_id", facilityID,
		"indicator_id", indicatorID,
	)

	if facilityID == uuid.Nil {
		return []db.ListFacilityIndicatorMetricsTrendRow{}, errors.New("facility id is required")
	}

	if indicatorID == uuid.Nil {
		return []db.ListFacilityIndicatorMetricsTrendRow{}, errors.New("indicator id is required")
	}

	rows, err := s.facilityWeeklyMetricsRepo.ListIndicatorTrend(ctx, db.ListFacilityIndicatorMetricsTrendParams{
		FacilityID: facilityID,
		IndicatorID: uuid.NullUUID{
			UUID:  indicatorID,
			Valid: indicatorID != uuid.Nil,
		},
	})
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility indicator metrics trend",
			"facility_id", facilityID,
			"indicator_id", indicatorID,
			"error", err,
		)
		return []db.ListFacilityIndicatorMetricsTrendRow{}, err
	}

	return rows, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityDiseaseMetricsByWeekAndDisease(
	ctx context.Context,
	epiWeekID, diseaseID uuid.UUID,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow, error) {
	s.log.DebugContext(ctx, "listing facility disease metrics by week and disease",
		"epi_week_id", epiWeekID,
		"disease_id", diseaseID,
	)

	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow{}, errors.New("epi week id is required")
	}

	if diseaseID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow{}, errors.New("disease id is required")
	}

	rows, err := s.facilityWeeklyMetricsRepo.ListDiseaseMetricsByWeekAndDisease(
		ctx,
		db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseParams{
			EpiWeekID: epiWeekID,
			DiseaseID: uuid.NullUUID{
				UUID:  diseaseID,
				Valid: diseaseID != uuid.Nil,
			},
		},
	)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility disease metrics by week and disease",
			"epi_week_id", epiWeekID,
			"disease_id", diseaseID,
			"error", err,
		)
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow{}, err
	}

	return rows, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityDiseaseMetricsByWeekAndDiseaseInHealthContext(
	ctx context.Context,
	epiWeekID, diseaseID uuid.UUID,
	scope HealthContextScope,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow, error) {
	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow{}, errors.New("epi week id is required")
	}
	if diseaseID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow{}, errors.New("disease id is required")
	}
	return s.facilityWeeklyMetricsRepo.ListDiseaseMetricsByWeekAndDiseaseInHealthContext(
		ctx,
		epiWeekID,
		diseaseID,
		scope,
	)
}

func (s *FacilityWeeklyMetricsService) ListDiseaseWeeklyTrendAggregated(
	ctx context.Context,
	epiYear int32,
	diseaseID uuid.UUID,
	regionID *uuid.UUID,
	districtID *uuid.UUID,
) ([]db.ListDiseaseWeeklyTrendAggregatedRow, error) {
	s.log.DebugContext(ctx, "listing aggregated disease weekly trend",
		"epi_year", epiYear,
		"disease_id", diseaseID,
		"region_id", regionID,
		"district_id", districtID,
	)

	if epiYear == 0 {
		return []db.ListDiseaseWeeklyTrendAggregatedRow{}, errors.New("epi year is required")
	}

	if diseaseID == uuid.Nil {
		return []db.ListDiseaseWeeklyTrendAggregatedRow{}, errors.New("disease id is required")
	}

	arg := db.ListDiseaseWeeklyTrendAggregatedParams{
		EpiYear: epiYear,
		DiseaseID: uuid.NullUUID{
			UUID:  diseaseID,
			Valid: true,
		},
		RegionID:   uuid.NullUUID{},
		DistrictID: uuid.NullUUID{},
	}

	if regionID != nil && *regionID != uuid.Nil {
		arg.RegionID = uuid.NullUUID{
			UUID:  *regionID,
			Valid: true,
		}
	}

	if districtID != nil && *districtID != uuid.Nil {
		arg.DistrictID = uuid.NullUUID{
			UUID:  *districtID,
			Valid: true,
		}
	}

	rows, err := s.facilityWeeklyMetricsRepo.ListDiseaseWeeklyTrendAggregated(ctx, arg)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list aggregated disease weekly trend",
			"epi_year", epiYear,
			"disease_id", diseaseID,
			"region_id", regionID,
			"district_id", districtID,
			"error", err,
		)
		return []db.ListDiseaseWeeklyTrendAggregatedRow{}, err
	}

	return rows, nil
}

func (s *FacilityWeeklyMetricsService) ListDiseaseWeeklyTrendAggregatedInHealthContext(
	ctx context.Context,
	epiYear int32,
	diseaseID uuid.UUID,
	regionID *uuid.UUID,
	districtID *uuid.UUID,
	scope HealthContextScope,
) ([]db.ListDiseaseWeeklyTrendAggregatedRow, error) {
	if epiYear == 0 {
		return []db.ListDiseaseWeeklyTrendAggregatedRow{}, errors.New("epi year is required")
	}
	if diseaseID == uuid.Nil {
		return []db.ListDiseaseWeeklyTrendAggregatedRow{}, errors.New("disease id is required")
	}
	arg := db.ListDiseaseWeeklyTrendAggregatedParams{
		EpiYear: epiYear,
		DiseaseID: uuid.NullUUID{
			UUID:  diseaseID,
			Valid: true,
		},
	}
	if regionID != nil && *regionID != uuid.Nil {
		arg.RegionID = uuid.NullUUID{UUID: *regionID, Valid: true}
	}
	if districtID != nil && *districtID != uuid.Nil {
		arg.DistrictID = uuid.NullUUID{UUID: *districtID, Valid: true}
	}
	return s.facilityWeeklyMetricsRepo.ListDiseaseWeeklyTrendAggregatedInHealthContext(
		ctx,
		arg,
		scope,
	)
}

func (s *FacilityWeeklyMetricsService) UpsertFacilityWeeklyDiseaseMetric(
	ctx context.Context,
	arg db.UpsertFacilityWeeklyDiseaseMetricParams,
) (db.FacilityWeeklyMetric, error) {
	s.log.DebugContext(ctx, "upserting facility weekly disease metric")

	item, err := s.facilityWeeklyMetricsRepo.UpsertDiseaseMetric(ctx, arg)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to upsert facility weekly disease metric", "error", err)
		return db.FacilityWeeklyMetric{}, err
	}

	return item, nil
}

func (s *FacilityWeeklyMetricsService) ListFacilityMetricsByFacility(
	ctx context.Context,
	facilityID uuid.UUID,
) ([]db.ListFacilityMetricsByFacilityRow, error) {
	s.log.DebugContext(ctx, "listing facility metrics by facility", "facility_id", facilityID)

	if facilityID == uuid.Nil {
		return []db.ListFacilityMetricsByFacilityRow{}, errors.New("facility id is required")
	}

	items, err := s.facilityWeeklyMetricsRepo.ListByFacility(ctx, facilityID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list facility metrics by facility", "facility_id", facilityID, "error", err)
		return []db.ListFacilityMetricsByFacilityRow{}, err
	}

	return items, nil
}

func (s *FacilityWeeklyMetricsService) ProcessFacilityMetrics(
	ctx context.Context,
	batchID uuid.UUID,
) error {
	if batchID == uuid.Nil {
		return fmt.Errorf("batch id is required")
	}

	rows, err := s.surveillanceImportRepo.ListImportRawRowsByBatch(ctx, batchID)
	if err != nil {
		return fmt.Errorf("list import raw rows by batch: %w", err)
	}

	var successRows int32
	var failedRows int32

	for _, raw := range rows {
		err := s.facilityWeeklyMetricsRepo.WithTx(ctx, func(q db.Querier) error {
			return s.processFacilityMetricRow(ctx, q, raw)
		})

		if err != nil {
			failedRows++

			if markErr := s.surveillanceImportRepo.MarkRawRowFailed(ctx, raw.ID, "failed to process facility metrics row"); markErr != nil {
				return fmt.Errorf("mark raw row failed: %w", markErr)
			}

			continue
		}

		successRows++

		if err := s.surveillanceImportRepo.MarkRawRowProcessed(ctx, raw.ID); err != nil {
			return fmt.Errorf("mark raw row processed: %w", err)
		}
	}

	if err := s.surveillanceImportRepo.CompleteImportBatch(ctx, batchID, successRows, failedRows); err != nil {
		return fmt.Errorf("complete batch: %w", err)
	}

	if failedRows == 0 {
		if err := s.surveillanceImportRepo.DeleteProcessedRawRows(ctx, batchID); err != nil {
			return fmt.Errorf("delete processed raw rows: %w", err)
		}
	}

	return nil
}

func (s *FacilityWeeklyMetricsService) processFacilityMetricRow(
	ctx context.Context,
	q db.Querier,
	raw db.SurveillanceImportRawRow,
) error {
	payload, err := parseFacilityWeeklyMetricsPayload(raw.Payload)
	if err != nil {
		return err
	}

	region, err := getOrCreateRegion(ctx, q, payload.Region)
	if err != nil {
		return fmt.Errorf("resolve region %q: %w", payload.Region, err)
	}

	district, err := getOrCreateDistrict(ctx, q, payload.District, region.ID)
	if err != nil {
		return fmt.Errorf("resolve district %q: %w", payload.District, err)
	}

	subCounty, err := getOrCreateSubCounty(ctx, q, payload.SubCounty, district.ID)
	if err != nil {
		return fmt.Errorf("resolve sub county %q: %w", payload.SubCounty, err)
	}

	facility, err := getOrCreateFacility(ctx, q, payload.Facility, district.ID, subCounty.ID)
	if err != nil {
		return fmt.Errorf("resolve facility %q: %w", payload.Facility, err)
	}

	epiWeek, err := getOrCreateEpiWeek(ctx, q, payload.Year, payload.Week)
	if err != nil {
		return fmt.Errorf("resolve epi week year=%d week=%d: %w", payload.Year, payload.Week, err)
	}

	metricName := strings.TrimSpace(payload.Metric)
	if metricName == "" {
		return fmt.Errorf("metric is required")
	}

	// 1. If metric name itself is a disease, store as disease metric.
	diseaseByMetric, diseaseErr := q.GetDiseaseByName(ctx, metricName)
	if diseaseErr == nil {
		_, err = q.UpsertFacilityWeeklyDiseaseMetric(ctx, db.UpsertFacilityWeeklyDiseaseMetricParams{
			FacilityID: facility.ID,
			DiseaseID: uuid.NullUUID{
				UUID:  diseaseByMetric.ID,
				Valid: diseaseByMetric.ID != uuid.Nil,
			},
			EpiWeekID:   epiWeek.ID,
			MetricValue: strconv.FormatFloat(payload.Value, 'f', -1, 64),
		})
		if err != nil {
			return fmt.Errorf("upsert facility weekly disease metric: %w", err)
		}

		return nil
	}

	if diseaseErr != nil && !errors.Is(diseaseErr, sql.ErrNoRows) {
		return fmt.Errorf("find disease %q: %w", metricName, diseaseErr)
	}

	// 2. Otherwise resolve indicator by name.
	indicator, indicatorErr := q.GetIndicatorByName(ctx, metricName)
	if indicatorErr != nil {
		if errors.Is(indicatorErr, sql.ErrNoRows) {
			return fmt.Errorf("metric %q was not found as a disease or indicator", metricName)
		}
		return fmt.Errorf("find indicator %q: %w", metricName, indicatorErr)
	}

	// 3. You need the disease context from payload for indicator metrics.
	diseaseName := strings.TrimSpace(payload.Metric)
	if diseaseName == "" {
		return fmt.Errorf("disease is required when metric %q is an indicator", metricName)
	}

	disease, err := q.GetDiseaseByName(ctx, diseaseName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("disease %q was not found", diseaseName)
		}
		return fmt.Errorf("find disease %q: %w", diseaseName, err)
	}

	allowed, err := isIndicatorAllowedForDisease(ctx, q, disease.ID, indicator.ID)
	if err != nil {
		return fmt.Errorf(
			"check disease indicator mapping for disease=%q indicator=%q: %w",
			diseaseName,
			metricName,
			err,
		)
	}

	if !allowed {
		return fmt.Errorf(
			"indicator %q is not configured for disease %q",
			metricName,
			diseaseName,
		)
	}

	_, err = q.UpsertFacilityWeeklyIndicatorMetric(ctx, db.UpsertFacilityWeeklyIndicatorMetricParams{
		FacilityID: facility.ID,
		IndicatorID: uuid.NullUUID{
			UUID:  indicator.ID,
			Valid: indicator.ID != uuid.Nil,
		},
		EpiWeekID:   epiWeek.ID,
		MetricValue: strconv.FormatFloat(payload.Value, 'f', -1, 64),
	})
	if err != nil {
		return fmt.Errorf("upsert facility weekly indicator metric: %w", err)
	}

	return nil
}

func parseFacilityWeeklyMetricsPayload(rawPayload []byte) (facilityWeeklyMetricsPayload, error) {
	var raw rawFacilityWeeklyMetricsPayload
	if err := json.Unmarshal(rawPayload, &raw); err != nil {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("invalid payload json: %w", err)
	}

	year, err := parseInt32Field(raw.Year, "year")
	if err != nil {
		return facilityWeeklyMetricsPayload{}, err
	}

	week, err := parseInt32Field(raw.Week, "weeks")
	if err != nil && len(bytesTrimSpace(raw.EpiWeek)) > 0 {
		week, err = parseInt32Field(raw.EpiWeek, "epi_week")
	}
	if err != nil {
		return facilityWeeklyMetricsPayload{}, err
	}

	value, err := parseFloat64Field(raw.Value, "value")
	if err != nil {
		return facilityWeeklyMetricsPayload{}, err
	}

	payload := facilityWeeklyMetricsPayload{
		RecordID:  strings.TrimSpace(raw.RecordID),
		Facility:  strings.TrimSpace(raw.Facility),
		Region:    strings.TrimSpace(raw.Region),
		District:  strings.TrimSpace(raw.District),
		SubCounty: strings.TrimSpace(raw.SubCounty),
		Metric:    strings.TrimSpace(raw.Metric),
		Value:     value,
		Year:      year,
		Week:      week,
		EpiWeek:   week,
	}

	if payload.Facility == "" {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("facility is required")
	}

	if payload.Region == "" {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("region is required")
	}

	if payload.District == "" {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("district is required")
	}

	if payload.SubCounty == "" {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("sub_county is required")
	}

	if payload.Metric == "" {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("metric is required")
	}

	if payload.Year <= 0 {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("year is required")
	}

	if payload.Week <= 0 {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("weeks is required")
	}

	if payload.Value < 0 {
		return facilityWeeklyMetricsPayload{}, fmt.Errorf("value must be greater than or equal to 0")
	}

	return payload, nil
}

func parseFloat64Field(raw json.RawMessage, fieldName string) (float64, error) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("%s is required", fieldName)
	}

	var asFloat float64
	if err := json.Unmarshal(raw, &asFloat); err == nil {
		return asFloat, nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return 0, fmt.Errorf("%s is required", fieldName)
		}

		n, err := strconv.ParseFloat(asString, 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a valid number", fieldName)
		}

		return n, nil
	}

	return 0, fmt.Errorf("%s must be a valid number", fieldName)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func getOrCreateRegion(
	ctx context.Context,
	q db.Querier,
	name string,
) (db.Region, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return db.Region{}, fmt.Errorf("region name is required")
	}

	region, err := q.GetRegionByName(ctx, name)
	if err == nil {
		return region, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return db.Region{}, err
	}

	return q.CreateRegion(ctx, db.CreateRegionParams{
		Name: name,
	})
}

func getOrCreateDistrict(
	ctx context.Context,
	q db.Querier,
	name string,
	regionID uuid.UUID,
) (db.District, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return db.District{}, fmt.Errorf("district name is required")
	}

	district, err := q.GetDistrictByName(ctx, name)
	if err == nil {
		return district, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return db.District{}, err
	}

	return q.CreateDistrict(ctx, db.CreateDistrictParams{
		Name: name,
		RegionID: uuid.NullUUID{
			UUID:  regionID,
			Valid: regionID != uuid.Nil,
		},
	})
}

func getOrCreateSubCounty(
	ctx context.Context,
	q db.Querier,
	name string,
	districtID uuid.UUID,
) (db.SubCounty, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return db.SubCounty{}, fmt.Errorf("sub county name is required")
	}

	subCounty, err := q.GetSubCountyByNameAndDistrict(ctx, db.GetSubCountyByNameAndDistrictParams{Lower: name, DistrictID: districtID})
	if err == nil {
		return subCounty, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return db.SubCounty{}, err
	}

	return q.CreateSubCounty(ctx, db.CreateSubCountyParams{
		Name:       name,
		DistrictID: districtID,
	})
}

func getOrCreateFacility(
	ctx context.Context,
	q db.Querier,
	name string,
	districtID uuid.UUID,
	subCountyID uuid.UUID,
) (db.Facility, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return db.Facility{}, fmt.Errorf("facility name is required")
	}

	facility, err := q.GetFacilityByNameAndDistrict(ctx, db.GetFacilityByNameAndDistrictParams{Lower: name, DistrictID: uuid.NullUUID{UUID: districtID, Valid: true}})
	if err == nil {
		return facility, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return db.Facility{}, err
	}

	return q.CreateFacility(ctx, db.CreateFacilityParams{
		Name: name,
		DistrictID: uuid.NullUUID{
			UUID:  districtID,
			Valid: districtID != uuid.Nil,
		},
		SubCountyID: uuid.NullUUID{
			UUID:  subCountyID,
			Valid: subCountyID != uuid.Nil,
		},
	})
}

func getOrCreateEpiWeek(
	ctx context.Context,
	q db.Querier,
	year int32,
	week int32,
) (db.EpiWeek, error) {
	if year <= 0 {
		return db.EpiWeek{}, fmt.Errorf("epi year is required")
	}

	if week <= 0 {
		return db.EpiWeek{}, fmt.Errorf("epi week is required")
	}

	item, err := q.GetEpiWeekByYearWeek(ctx, db.GetEpiWeekByYearWeekParams{
		EpiYear: year,
		EpiWeek: week,
	})
	if err == nil {
		return item, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return db.EpiWeek{}, err
	}

	return q.CreateEpiWeek(ctx, db.CreateEpiWeekParams{
		EpiYear: year,
		EpiWeek: week,
	})
}

func getOrCreateDisease(
	ctx context.Context,
	q db.Querier,
	name string,
) (db.Disease, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return db.Disease{}, fmt.Errorf("disease name is required")
	}

	disease, err := q.GetDiseaseByName(ctx, normalizedName)
	if err == nil {
		return disease, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return db.Disease{}, fmt.Errorf("find disease %q: %w", normalizedName, err)
	}

	createdDisease, err := q.CreateDisease(ctx, db.CreateDiseaseParams{
		Name: normalizedName,
	})
	if err != nil {
		return db.Disease{}, fmt.Errorf("create disease %q: %w", normalizedName, err)
	}

	return createdDisease, nil
}

func getOrCreateIndicator(
	ctx context.Context,
	q db.Querier,
	name string,
) (db.Indicator, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return db.Indicator{}, fmt.Errorf("indicator name is required")
	}

	indicator, err := q.GetIndicatorByName(ctx, normalizedName)
	if err == nil {
		return indicator, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return db.Indicator{}, fmt.Errorf("find indicator %q: %w", normalizedName, err)
	}

	createdIndicator, err := q.CreateIndicator(ctx, db.CreateIndicatorParams{
		Name: normalizedName,
	})
	if err != nil {
		return db.Indicator{}, fmt.Errorf("create indicator %q: %w", normalizedName, err)
	}

	return createdIndicator, nil
}

func isIndicatorAllowedForDisease(
	ctx context.Context,
	q db.Querier,
	diseaseID uuid.UUID,
	indicatorID uuid.UUID,
) (bool, error) {
	return q.ExistsDiseaseIndicator(ctx, db.ExistsDiseaseIndicatorParams{
		DiseaseID:   diseaseID,
		IndicatorID: indicatorID,
	})
}
