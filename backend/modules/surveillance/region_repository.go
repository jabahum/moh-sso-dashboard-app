package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type RegionRepository interface {
	Create(ctx context.Context, arg db.CreateRegionParams) (db.Region, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.Region, error)
	GetByName(ctx context.Context, name string) (db.Region, error)
	List(ctx context.Context) ([]db.Region, error)
	ListInHealthContext(ctx context.Context, scope HealthContextScope) ([]db.Region, error)
	Upsert(ctx context.Context, arg db.UpsertRegionParams) (db.Region, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
