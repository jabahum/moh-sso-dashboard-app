package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresRegionRepository struct {
	db db.Store
}

func NewRegionRepository(db db.Store) RegionRepository {
	return &postgresRegionRepository{db: db}
}

func (r *postgresRegionRepository) Create(ctx context.Context, arg db.CreateRegionParams) (db.Region, error) {
	return r.db.CreateRegion(ctx, arg)
}

func (r *postgresRegionRepository) GetByID(ctx context.Context, id uuid.UUID) (db.Region, error) {
	return r.db.GetRegionByID(ctx, id)
}

func (r *postgresRegionRepository) GetByName(ctx context.Context, name string) (db.Region, error) {
	return r.db.GetRegionByName(ctx, name)
}

func (r *postgresRegionRepository) List(ctx context.Context) ([]db.Region, error) {
	return r.db.ListRegions(ctx)
}

func (r *postgresRegionRepository) ListInHealthContext(
	ctx context.Context,
	scope HealthContextScope,
) ([]db.Region, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-region", true)
	if err != nil {
		return nil, err
	}

	return r.db.ListRegionsInHealthContext(ctx, allowedIDs)
}

func (r *postgresRegionRepository) Upsert(ctx context.Context, arg db.UpsertRegionParams) (db.Region, error) {
	return r.db.UpsertRegion(ctx, arg)
}

func (r *postgresRegionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DeleteRegion(ctx, id)
}
