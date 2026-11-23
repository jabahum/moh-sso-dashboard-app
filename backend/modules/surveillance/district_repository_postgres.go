package surveillance

import (
	"context"
	"errors"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresDistrictRepository struct {
	db db.Store
}

func NewDistrictRepository(db db.Store) DistrictRepository {
	return &postgresDistrictRepository{db: db}
}

func (r *postgresDistrictRepository) Create(ctx context.Context, arg db.CreateDistrictParams) (db.District, error) {
	return r.db.CreateDistrict(ctx, arg)
}

func (r *postgresDistrictRepository) GetByID(ctx context.Context, id uuid.UUID) (db.District, error) {
	return r.db.GetDistrictByID(ctx, id)
}

func (r *postgresDistrictRepository) GetByName(ctx context.Context, name string) (db.District, error) {
	return r.db.GetDistrictByName(ctx, name)
}

func (r *postgresDistrictRepository) List(ctx context.Context) ([]db.ListDistrictsRow, error) {
	return r.db.ListDistricts(ctx)
}

func (r *postgresDistrictRepository) ListInHealthContext(
	ctx context.Context,
	scope HealthContextScope,
) ([]db.ListDistrictsRow, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-district", true)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.ListDistrictsInHealthContext(ctx, allowedIDs)
	if err != nil {
		return nil, err
	}
	items := make([]db.ListDistrictsRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, db.ListDistrictsRow{
			ID: row.ID, Name: row.Name, RegionID: row.RegionID, Code: row.Code,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, RegionName: row.RegionName,
		})
	}
	return items, nil
}

func (r *postgresDistrictRepository) ListByRegion(ctx context.Context, regionID uuid.UUID) ([]db.District, error) {

	if regionID == uuid.Nil {
		return []db.District{}, errors.New("district id is required")
	}

	param := uuid.NullUUID{
		UUID:  regionID,
		Valid: true,
	}

	districts, err := r.db.ListDistrictsByRegion(ctx, param)
	if err != nil {
		return []db.District{}, err
	}

	return districts, nil
}

func (r *postgresDistrictRepository) ListByRegionInHealthContext(
	ctx context.Context,
	regionID uuid.UUID,
	scope HealthContextScope,
) ([]db.District, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-district", true)
	if err != nil {
		return nil, err
	}

	if regionID == uuid.Nil {
		return []db.District{}, errors.New("region id is required")
	}
	return r.db.ListDistrictsByRegionInHealthContext(
		ctx,
		db.ListDistrictsByRegionInHealthContextParams{
			RegionID:           uuid.NullUUID{UUID: regionID, Valid: true},
			AllowedDistrictIds: allowedIDs,
		},
	)
}

func (r *postgresDistrictRepository) Upsert(ctx context.Context, arg db.UpsertDistrictParams) (db.District, error) {
	return r.db.UpsertDistrict(ctx, arg)
}

func (r *postgresDistrictRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DeleteDistrict(ctx, id)
}
