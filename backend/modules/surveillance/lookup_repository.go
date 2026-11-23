package surveillance

import (
	"context"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type LookupRepository interface {
	ResolveDiseaseByName(ctx context.Context, name string) (db.Disease, error)
	ResolveIndicatorByName(ctx context.Context, name string) (db.Indicator, error)
	ResolveWeekByYearWeek(ctx context.Context, arg db.ResolveWeekByYearWeekParams) (db.EpiWeek, error)
	ResolveDistrictByName(ctx context.Context, name string) (db.District, error)
	ResolveRegionByName(ctx context.Context, name string) (db.Region, error)
	ResolveSubCountyByNameAndDistrict(ctx context.Context, arg db.ResolveSubCountyByNameAndDistrictParams) (db.SubCounty, error)
	ResolveFacilityByExternalID(ctx context.Context, externalID string) (db.Facility, error)
	ResolveFacilityByNameAndDistrict(ctx context.Context, arg db.ResolveFacilityByNameAndDistrictParams) (db.Facility, error)
}
