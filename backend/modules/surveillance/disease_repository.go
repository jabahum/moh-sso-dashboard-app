package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type DiseaseRepository interface {
	Create(ctx context.Context, arg db.CreateDiseaseParams) (db.Disease, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.Disease, error)
	GetByName(ctx context.Context, name string) (db.Disease, error)
	List(ctx context.Context) ([]db.Disease, error)
	ListActive(ctx context.Context) ([]db.Disease, error)
	Upsert(ctx context.Context, arg db.UpsertDiseaseParams) (db.Disease, error)
	SetActiveState(ctx context.Context, arg db.SetDiseaseActiveStateParams) (db.Disease, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
