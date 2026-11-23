package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresSubCountyRepository struct {
	db db.Store
}

func NewSubCountyRepository(db db.Store) SubCountyRepository {
	return &postgresSubCountyRepository{db: db}
}

func (r *postgresSubCountyRepository) Create(ctx context.Context, arg db.CreateSubCountyParams) (db.SubCounty, error) {
	return r.db.CreateSubCounty(ctx, arg)
}

func (r *postgresSubCountyRepository) GetByID(ctx context.Context, id uuid.UUID) (db.SubCounty, error) {
	return r.db.GetSubCountyByID(ctx, id)
}

func (r *postgresSubCountyRepository) GetByNameAndDistrict(ctx context.Context, arg db.GetSubCountyByNameAndDistrictParams) (db.SubCounty, error) {
	return r.db.GetSubCountyByNameAndDistrict(ctx, arg)
}

func (r *postgresSubCountyRepository) ListByDistrict(ctx context.Context, districtID uuid.UUID) ([]db.SubCounty, error) {
	return r.db.ListSubCountiesByDistrict(ctx, districtID)
}

func (r *postgresSubCountyRepository) ListByDistrictInHealthContext(
	ctx context.Context,
	districtID uuid.UUID,
	scope HealthContextScope,
) ([]db.SubCounty, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-sub-county", true)
	if err != nil {
		return nil, err
	}

	return r.db.ListSubCountiesByDistrictInHealthContext(
		ctx,
		db.ListSubCountiesByDistrictInHealthContextParams{
			DistrictID:          districtID,
			AllowedSubCountyIds: allowedIDs,
		},
	)
}

func (r *postgresSubCountyRepository) Upsert(ctx context.Context, arg db.UpsertSubCountyParams) (db.SubCounty, error) {
	return r.db.UpsertSubCounty(ctx, arg)
}

func (r *postgresSubCountyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DeleteSubCounty(ctx, id)
}
