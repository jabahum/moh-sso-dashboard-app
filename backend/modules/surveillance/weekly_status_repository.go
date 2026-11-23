package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type WeeklyStatusRepository interface {
	Create(ctx context.Context, arg db.CreateWeeklyStatusParams) (db.WeeklyStatus, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.WeeklyStatus, error)
	Delete(ctx context.Context, id uuid.UUID) error

	List(ctx context.Context, arg db.ListWeeklyStatusesParams) ([]db.WeeklyStatus, error)
	ListInHealthContext(
		ctx context.Context,
		arg db.ListWeeklyStatusesParams,
		scope HealthContextScope,
	) ([]db.WeeklyStatus, error)
	ListDetailed(ctx context.Context, arg db.ListWeeklyStatusesDetailedParams) ([]db.ListWeeklyStatusesDetailedRow, error)
	ListDetailedInHealthContext(
		ctx context.Context,
		arg db.ListWeeklyStatusesDetailedParams,
		scope HealthContextScope,
	) ([]db.ListWeeklyStatusesDetailedRow, error)

	ListByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.WeeklyStatus, error)
	ListByRegion(ctx context.Context, regionID uuid.UUID) ([]db.WeeklyStatus, error)
	ListByDistrict(ctx context.Context, districtID uuid.UUID) ([]db.WeeklyStatus, error)
	ListBySubCounty(ctx context.Context, subCountyID uuid.UUID) ([]db.WeeklyStatus, error)

	ListNationalByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.WeeklyStatus, error)
	ListRegionByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.WeeklyStatus, error)
	ListDistrictByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.WeeklyStatus, error)
	ListSubCountyByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.WeeklyStatus, error)

	UpsertNationalDisease(ctx context.Context, arg db.UpsertNationalDiseaseWeeklyStatusParams) (db.WeeklyStatus, error)
	UpsertNationalIndicator(ctx context.Context, arg db.UpsertNationalIndicatorWeeklyStatusParams) (db.WeeklyStatus, error)
	UpsertRegionDisease(ctx context.Context, arg db.UpsertRegionDiseaseWeeklyStatusParams) (db.WeeklyStatus, error)
	UpsertRegionIndicator(ctx context.Context, arg db.UpsertRegionIndicatorWeeklyStatusParams) (db.WeeklyStatus, error)
	UpsertDistrictDisease(ctx context.Context, arg db.UpsertDistrictDiseaseWeeklyStatusParams) (db.WeeklyStatus, error)
	UpsertDistrictIndicator(ctx context.Context, arg db.UpsertDistrictIndicatorWeeklyStatusParams) (db.WeeklyStatus, error)
	UpsertSubCountyDisease(ctx context.Context, arg db.UpsertSubCountyDiseaseWeeklyStatusParams) (db.WeeklyStatus, error)
	UpsertSubCountyIndicator(ctx context.Context, arg db.UpsertSubCountyIndicatorWeeklyStatusParams) (db.WeeklyStatus, error)

	WithTx(ctx context.Context, fn func(q db.Querier) error) error
}
