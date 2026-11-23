package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type DashboardRepository interface {
	GetNationalWeeklyStatusSummary(ctx context.Context, epiWeekID uuid.UUID) ([]db.GetNationalWeeklyStatusSummaryRow, error)
	ListNationalWeeklySubjectsByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListNationalWeeklySubjectsByWeekRow, error)
	ListDistrictWeeklySubjectsByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListDistrictWeeklySubjectsByWeekRow, error)
	ListRegionWeeklySubjectsByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListRegionWeeklySubjectsByWeekRow, error)
	GetWeeklySubjectTotals(ctx context.Context, epiWeekID uuid.UUID) ([]db.GetWeeklySubjectTotalsRow, error)
	GetDiseaseDashboardSummaryByWeek(ctx context.Context, arg db.GetDiseaseDashboardSummaryByWeekParams) (db.GetDiseaseDashboardSummaryByWeekRow, error)
	GetIndicatorDashboardSummaryByWeek(ctx context.Context, arg db.GetIndicatorDashboardSummaryByWeekParams) (db.GetIndicatorDashboardSummaryByWeekRow, error)
	GetTopFacilitiesByDiseaseAndWeek(ctx context.Context, arg db.GetTopFacilitiesByDiseaseAndWeekParams) ([]db.GetTopFacilitiesByDiseaseAndWeekRow, error)
	GetTopFacilitiesByIndicatorAndWeek(ctx context.Context, arg db.GetTopFacilitiesByIndicatorAndWeekParams) ([]db.GetTopFacilitiesByIndicatorAndWeekRow, error)
}
