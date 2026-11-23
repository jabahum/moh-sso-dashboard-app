package surveillance

import (
	"context"
	"database/sql"
	"errors"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresLookupRepository struct {
	db db.Store
}

func NewLookupRepository(db db.Store) LookupRepository {
	return &postgresLookupRepository{db: db}
}

func (r *postgresLookupRepository) ResolveDiseaseByName(ctx context.Context, name string) (db.Disease, error) {
	return r.db.ResolveDiseaseByName(ctx, name)
}

func (r *postgresLookupRepository) ResolveIndicatorByName(ctx context.Context, name string) (db.Indicator, error) {
	return r.db.ResolveIndicatorByName(ctx, name)
}

func (r *postgresLookupRepository) ResolveWeekByYearWeek(ctx context.Context, arg db.ResolveWeekByYearWeekParams) (db.EpiWeek, error) {
	return r.db.ResolveWeekByYearWeek(ctx, arg)
}

func (r *postgresLookupRepository) ResolveDistrictByName(ctx context.Context, name string) (db.District, error) {
	return r.db.ResolveDistrictByName(ctx, name)
}

func (r *postgresLookupRepository) ResolveRegionByName(ctx context.Context, name string) (db.Region, error) {
	return r.db.ResolveRegionByName(ctx, name)
}

func (r *postgresLookupRepository) ResolveSubCountyByNameAndDistrict(ctx context.Context, arg db.ResolveSubCountyByNameAndDistrictParams) (db.SubCounty, error) {
	return r.db.ResolveSubCountyByNameAndDistrict(ctx, arg)
}

func (r *postgresLookupRepository) ResolveFacilityByExternalID(ctx context.Context, externalID string) (db.Facility, error) {

	if externalID == "" {
		return db.Facility{}, errors.New("external id is required")
	}

	param := sql.NullString{
		String: externalID,
		Valid:  true,
	}

	facility, err := r.db.ResolveFacilityByExternalID(ctx, param)
	if err != nil {
		return db.Facility{}, err
	}

	return facility, nil
}

func (r *postgresLookupRepository) ResolveFacilityByNameAndDistrict(ctx context.Context, arg db.ResolveFacilityByNameAndDistrictParams) (db.Facility, error) {
	return r.db.ResolveFacilityByNameAndDistrict(ctx, arg)
}
