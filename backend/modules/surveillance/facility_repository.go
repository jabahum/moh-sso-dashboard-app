package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type FacilityRepository interface {
	Create(ctx context.Context, arg db.CreateFacilityParams) (db.Facility, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.Facility, error)
	GetByExternalID(ctx context.Context, externalID string) (db.Facility, error)
	GetByNameAndDistrict(ctx context.Context, arg db.GetFacilityByNameAndDistrictParams) (db.Facility, error)
	List(ctx context.Context) ([]db.ListFacilitiesRow, error)
	ListByDistrict(ctx context.Context, districtID uuid.UUID) ([]db.Facility, error)
	UpsertByExternalID(ctx context.Context, arg db.UpsertFacilityByExternalIDParams) (db.Facility, error)
	UpsertByNameDistrict(ctx context.Context, arg db.UpsertFacilityByNameDistrictParams) (db.Facility, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
