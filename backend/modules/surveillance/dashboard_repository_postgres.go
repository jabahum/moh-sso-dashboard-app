package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresDashboardRepository struct {
	db db.Store
}

func NewDashboardRepository(db db.Store) DashboardRepository {
	return &postgresDashboardRepository{db: db}
}

func (r *postgresDashboardRepository) GetNationalWeeklyStatusSummary(ctx context.Context, epiWeekID uuid.UUID) ([]db.GetNationalWeeklyStatusSummaryRow, error) {
	return r.db.GetNationalWeeklyStatusSummary(ctx, epiWeekID)
}

func (r *postgresDashboardRepository) ListNationalWeeklySubjectsByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListNationalWeeklySubjectsByWeekRow, error) {
	return r.db.ListNationalWeeklySubjectsByWeek(ctx, epiWeekID)
}

func (r *postgresDashboardRepository) ListDistrictWeeklySubjectsByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListDistrictWeeklySubjectsByWeekRow, error) {
	return r.db.ListDistrictWeeklySubjectsByWeek(ctx, epiWeekID)
}

func (r *postgresDashboardRepository) ListRegionWeeklySubjectsByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListRegionWeeklySubjectsByWeekRow, error) {
	return r.db.ListRegionWeeklySubjectsByWeek(ctx, epiWeekID)
}

func (r *postgresDashboardRepository) GetWeeklySubjectTotals(ctx context.Context, epiWeekID uuid.UUID) ([]db.GetWeeklySubjectTotalsRow, error) {
	return r.db.GetWeeklySubjectTotals(ctx, epiWeekID)
}

func (r *postgresDashboardRepository) GetDiseaseDashboardSummaryByWeek(ctx context.Context, arg db.GetDiseaseDashboardSummaryByWeekParams) (db.GetDiseaseDashboardSummaryByWeekRow, error) {
	return r.db.GetDiseaseDashboardSummaryByWeek(ctx, arg)
}

func (r *postgresDashboardRepository) GetIndicatorDashboardSummaryByWeek(ctx context.Context, arg db.GetIndicatorDashboardSummaryByWeekParams) (db.GetIndicatorDashboardSummaryByWeekRow, error) {
	return r.db.GetIndicatorDashboardSummaryByWeek(ctx, arg)
}

func (r *postgresDashboardRepository) GetTopFacilitiesByDiseaseAndWeek(ctx context.Context, arg db.GetTopFacilitiesByDiseaseAndWeekParams) ([]db.GetTopFacilitiesByDiseaseAndWeekRow, error) {
	return r.db.GetTopFacilitiesByDiseaseAndWeek(ctx, arg)
}

func (r *postgresDashboardRepository) GetTopFacilitiesByIndicatorAndWeek(ctx context.Context, arg db.GetTopFacilitiesByIndicatorAndWeekParams) ([]db.GetTopFacilitiesByIndicatorAndWeekRow, error) {
	return r.db.GetTopFacilitiesByIndicatorAndWeek(ctx, arg)
}
