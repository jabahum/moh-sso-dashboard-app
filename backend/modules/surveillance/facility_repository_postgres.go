package surveillance

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresFacilityRepository struct {
	db db.Store
}

func NewFacilityRepository(db db.Store) FacilityRepository {
	return &postgresFacilityRepository{db: db}
}

func (r *postgresFacilityRepository) Create(ctx context.Context, arg db.CreateFacilityParams) (db.Facility, error) {
	return r.db.CreateFacility(ctx, arg)
}

func (r *postgresFacilityRepository) GetByID(ctx context.Context, id uuid.UUID) (db.Facility, error) {
	return r.db.GetFacilityByID(ctx, id)
}

func (r *postgresFacilityRepository) GetByExternalID(ctx context.Context, externalID string) (db.Facility, error) {

	if externalID == "" {
		return db.Facility{}, errors.New("external id is required")
	}
	param := sql.NullString{
		String: externalID,
		Valid:  true,
	}

	facility, err := r.db.GetFacilityByExternalID(ctx, param)
	if err != nil {
		return db.Facility{}, err
	}

	return facility, nil

}

func (r *postgresFacilityRepository) GetByNameAndDistrict(ctx context.Context, arg db.GetFacilityByNameAndDistrictParams) (db.Facility, error) {
	return r.db.GetFacilityByNameAndDistrict(ctx, arg)
}

func (r *postgresFacilityRepository) List(ctx context.Context) ([]db.ListFacilitiesRow, error) {
	return r.db.ListFacilities(ctx)
}

func (r *postgresFacilityRepository) ListByDistrict(ctx context.Context, districtID uuid.UUID) ([]db.Facility, error) {
	if districtID == uuid.Nil {
		return []db.Facility{}, errors.New("district id is required")
	}

	param := uuid.NullUUID{
		UUID:  districtID,
		Valid: true,
	}

	facility, err := r.db.ListFacilitiesByDistrict(ctx, param)
	if err != nil {
		return []db.Facility{}, err
	}

	return facility, nil
}

func (r *postgresFacilityRepository) UpsertByExternalID(ctx context.Context, arg db.UpsertFacilityByExternalIDParams) (db.Facility, error) {
	return r.db.UpsertFacilityByExternalID(ctx, arg)
}

func (r *postgresFacilityRepository) UpsertByNameDistrict(ctx context.Context, arg db.UpsertFacilityByNameDistrictParams) (db.Facility, error) {
	return r.db.UpsertFacilityByNameDistrict(ctx, arg)
}

func (r *postgresFacilityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DeleteFacility(ctx, id)
}
