package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type DistrictRepository interface {
	Create(ctx context.Context, arg db.CreateDistrictParams) (db.District, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.District, error)
	GetByName(ctx context.Context, name string) (db.District, error)
	List(ctx context.Context) ([]db.ListDistrictsRow, error)
	ListInHealthContext(ctx context.Context, scope HealthContextScope) ([]db.ListDistrictsRow, error)
	ListByRegion(ctx context.Context, regionID uuid.UUID) ([]db.District, error)
	ListByRegionInHealthContext(
		ctx context.Context,
		regionID uuid.UUID,
		scope HealthContextScope,
	) ([]db.District, error)
	Upsert(ctx context.Context, arg db.UpsertDistrictParams) (db.District, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
