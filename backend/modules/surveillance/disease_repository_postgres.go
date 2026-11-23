package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresDiseaseRepository struct {
	db db.Store
}

func NewDiseaseRepository(db db.Store) DiseaseRepository {
	return &postgresDiseaseRepository{db: db}
}

func (r *postgresDiseaseRepository) Create(ctx context.Context, arg db.CreateDiseaseParams) (db.Disease, error) {
	return r.db.CreateDisease(ctx, arg)
}

func (r *postgresDiseaseRepository) GetByID(ctx context.Context, id uuid.UUID) (db.Disease, error) {
	return r.db.GetDiseaseByID(ctx, id)
}

func (r *postgresDiseaseRepository) GetByName(ctx context.Context, name string) (db.Disease, error) {
	return r.db.GetDiseaseByName(ctx, name)
}

func (r *postgresDiseaseRepository) List(ctx context.Context) ([]db.Disease, error) {
	return r.db.ListDiseases(ctx)
}

func (r *postgresDiseaseRepository) ListActive(ctx context.Context) ([]db.Disease, error) {
	return r.db.ListActiveDiseases(ctx)
}

func (r *postgresDiseaseRepository) Upsert(ctx context.Context, arg db.UpsertDiseaseParams) (db.Disease, error) {
	return r.db.UpsertDisease(ctx, arg)
}

func (r *postgresDiseaseRepository) SetActiveState(ctx context.Context, arg db.SetDiseaseActiveStateParams) (db.Disease, error) {
	return r.db.SetDiseaseActiveState(ctx, arg)
}

func (r *postgresDiseaseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DeleteDisease(ctx, id)
}
