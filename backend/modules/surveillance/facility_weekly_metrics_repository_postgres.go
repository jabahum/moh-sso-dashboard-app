package surveillance

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresFacilityWeeklyMetricsRepository struct {
	db db.Store
}

func NewFacilityWeeklyMetricsRepository(store db.Store) FacilityWeeklyMetricsRepository {
	return &postgresFacilityWeeklyMetricsRepository{
		db: store,
	}
}

func (r *postgresFacilityWeeklyMetricsRepository) CreateDiseaseMetric(
	ctx context.Context,
	arg db.CreateFacilityWeeklyDiseaseMetricParams,
) (db.FacilityWeeklyMetric, error) {
	return r.db.CreateFacilityWeeklyDiseaseMetric(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) UpsertDiseaseMetric(
	ctx context.Context,
	arg db.UpsertFacilityWeeklyDiseaseMetricParams,
) (db.FacilityWeeklyMetric, error) {
	return r.db.UpsertFacilityWeeklyDiseaseMetric(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) CreateIndicatorMetric(
	ctx context.Context,
	arg db.CreateFacilityWeeklyIndicatorMetricParams,
) (db.FacilityWeeklyMetric, error) {
	return r.db.CreateFacilityWeeklyIndicatorMetric(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) UpsertIndicatorMetric(
	ctx context.Context,
	arg db.UpsertFacilityWeeklyIndicatorMetricParams,
) (db.FacilityWeeklyMetric, error) {
	return r.db.UpsertFacilityWeeklyIndicatorMetric(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (db.FacilityWeeklyMetric, error) {
	if id == uuid.Nil {
		return db.FacilityWeeklyMetric{}, errors.New("id is required")
	}

	return r.db.GetFacilityWeeklyMetricByID(ctx, id)
}

func (r *postgresFacilityWeeklyMetricsRepository) GetBySourceRecordID(
	ctx context.Context,
	sourceRecordID string,
) (db.FacilityWeeklyMetric, error) {
	sourceRecordID = strings.TrimSpace(sourceRecordID)
	if sourceRecordID == "" {
		return db.FacilityWeeklyMetric{}, errors.New("source record id is required")
	}

	return r.db.GetFacilityMetricBySourceRecordID(ctx, sql.NullString{
		String: sourceRecordID,
		Valid:  true,
	})
}

func (r *postgresFacilityWeeklyMetricsRepository) ListDiseaseMetricsByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekRow, error) {
	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyDiseaseMetricsByWeekRow{}, errors.New("epi week id is required")
	}

	return r.db.ListFacilityWeeklyDiseaseMetricsByWeek(ctx, epiWeekID)
}

func (r *postgresFacilityWeeklyMetricsRepository) ListDiseaseMetricsByWeekInHealthContext(
	ctx context.Context,
	epiWeekID uuid.UUID,
	scope HealthContextScope,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekRow, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-facility", false)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.ListFacilityWeeklyDiseaseMetricsByWeekInHealthContext(
		ctx,
		db.ListFacilityWeeklyDiseaseMetricsByWeekInHealthContextParams{
			EpiWeekID: epiWeekID, AllowedFacilityIds: allowedIDs,
		},
	)
	if err != nil {
		return nil, err
	}
	items := make([]db.ListFacilityWeeklyDiseaseMetricsByWeekRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, db.ListFacilityWeeklyDiseaseMetricsByWeekRow{
			ID: row.ID, SourceRecordID: row.SourceRecordID, FacilityID: row.FacilityID,
			DiseaseID: row.DiseaseID, IndicatorID: row.IndicatorID, EpiWeekID: row.EpiWeekID,
			MetricValue: row.MetricValue, SourceName: row.SourceName, ImportedAt: row.ImportedAt,
			CreatedAt: row.CreatedAt, FacilityName: row.FacilityName, SubCountyID: row.SubCountyID,
			SubCountyName: row.SubCountyName, DistrictID: row.DistrictID,
			DistrictName: row.DistrictName, RegionID: row.RegionID, RegionName: row.RegionName,
			DiseaseName: row.DiseaseName, EpiYear: row.EpiYear, EpiWeek: row.EpiWeek,
		})
	}
	return items, nil
}

func (r *postgresFacilityWeeklyMetricsRepository) ListDiseaseMetricsByWeekAndDisease(
	ctx context.Context,
	arg db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseParams,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow, error) {
	return r.db.ListFacilityWeeklyDiseaseMetricsByWeekAndDisease(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) ListDiseaseMetricsByWeekAndDiseaseInHealthContext(
	ctx context.Context,
	epiWeekID uuid.UUID,
	diseaseID uuid.UUID,
	scope HealthContextScope,
) ([]db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-facility", false)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseInHealthContext(
		ctx,
		db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseInHealthContextParams{
			EpiWeekID:          epiWeekID,
			DiseaseID:          uuid.NullUUID{UUID: diseaseID, Valid: diseaseID != uuid.Nil},
			AllowedFacilityIds: allowedIDs,
		},
	)
	if err != nil {
		return nil, err
	}
	items := make([]db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow{
			ID: row.ID, SourceRecordID: row.SourceRecordID, FacilityID: row.FacilityID,
			DiseaseID: row.DiseaseID, IndicatorID: row.IndicatorID, EpiWeekID: row.EpiWeekID,
			MetricValue: row.MetricValue, SourceName: row.SourceName, ImportedAt: row.ImportedAt,
			CreatedAt: row.CreatedAt, FacilityName: row.FacilityName, SubCountyID: row.SubCountyID,
			SubCountyName: row.SubCountyName, DistrictID: row.DistrictID,
			DistrictName: row.DistrictName, RegionID: row.RegionID, RegionName: row.RegionName,
			DiseaseName: row.DiseaseName, EpiYear: row.EpiYear, EpiWeek: row.EpiWeek,
		})
	}
	return items, nil
}

func (r *postgresFacilityWeeklyMetricsRepository) ListIndicatorMetricsByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.ListFacilityWeeklyIndicatorMetricsByWeekRow, error) {
	if epiWeekID == uuid.Nil {
		return []db.ListFacilityWeeklyIndicatorMetricsByWeekRow{}, errors.New("epi week id is required")
	}

	return r.db.ListFacilityWeeklyIndicatorMetricsByWeek(ctx, epiWeekID)
}

func (r *postgresFacilityWeeklyMetricsRepository) ListByFacility(
	ctx context.Context,
	facilityID uuid.UUID,
) ([]db.ListFacilityMetricsByFacilityRow, error) {
	if facilityID == uuid.Nil {
		return []db.ListFacilityMetricsByFacilityRow{}, errors.New("facility id is required")
	}

	return r.db.ListFacilityMetricsByFacility(ctx, facilityID)
}

func (r *postgresFacilityWeeklyMetricsRepository) ListDiseaseTrend(
	ctx context.Context,
	arg db.ListFacilityDiseaseMetricsTrendParams,
) ([]db.ListFacilityDiseaseMetricsTrendRow, error) {
	return r.db.ListFacilityDiseaseMetricsTrend(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) ListIndicatorTrend(
	ctx context.Context,
	arg db.ListFacilityIndicatorMetricsTrendParams,
) ([]db.ListFacilityIndicatorMetricsTrendRow, error) {
	return r.db.ListFacilityIndicatorMetricsTrend(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) ListDiseaseWeeklyTrendAggregated(
	ctx context.Context,
	arg db.ListDiseaseWeeklyTrendAggregatedParams,
) ([]db.ListDiseaseWeeklyTrendAggregatedRow, error) {

	return r.db.ListDiseaseWeeklyTrendAggregated(ctx, arg)
}

func (r *postgresFacilityWeeklyMetricsRepository) ListDiseaseWeeklyTrendAggregatedInHealthContext(
	ctx context.Context,
	arg db.ListDiseaseWeeklyTrendAggregatedParams,
	scope HealthContextScope,
) ([]db.ListDiseaseWeeklyTrendAggregatedRow, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-facility", false)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.ListDiseaseWeeklyTrendAggregatedInHealthContext(
		ctx,
		db.ListDiseaseWeeklyTrendAggregatedInHealthContextParams{
			DiseaseID: arg.DiseaseID, AllowedFacilityIds: allowedIDs, EpiYear: arg.EpiYear,
			RegionID: arg.RegionID, DistrictID: arg.DistrictID,
		},
	)
	if err != nil {
		return nil, err
	}
	items := make([]db.ListDiseaseWeeklyTrendAggregatedRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, db.ListDiseaseWeeklyTrendAggregatedRow(row))
	}
	return items, nil
}

func (r *postgresFacilityWeeklyMetricsRepository) DeleteByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) error {
	if epiWeekID == uuid.Nil {
		return errors.New("epi week id is required")
	}

	return r.db.DeleteFacilityMetricsByWeek(ctx, epiWeekID)
}

func (r *postgresFacilityWeeklyMetricsRepository) WithTx(
	ctx context.Context,
	fn func(q db.Querier) error,
) error {
	return r.db.ExecTx(ctx, fn)
}
