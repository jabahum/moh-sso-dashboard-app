package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresEpiWeekRepository struct {
	db db.Store
}

func NewEpiWeekRepository(db db.Store) EpiWeekRepository {
	return &postgresEpiWeekRepository{db: db}
}

func (r *postgresEpiWeekRepository) Create(ctx context.Context, arg db.CreateEpiWeekParams) (db.EpiWeek, error) {
	return r.db.CreateEpiWeek(ctx, arg)
}

func (r *postgresEpiWeekRepository) GetByID(ctx context.Context, id uuid.UUID) (db.EpiWeek, error) {
	return r.db.GetEpiWeekByID(ctx, id)
}

func (r *postgresEpiWeekRepository) GetByYearWeek(ctx context.Context, arg db.GetEpiWeekByYearWeekParams) (db.EpiWeek, error) {
	return r.db.GetEpiWeekByYearWeek(ctx, arg)
}

func (r *postgresEpiWeekRepository) ListByYear(ctx context.Context, year int32) ([]db.EpiWeek, error) {
	return r.db.ListEpiWeeksByYear(ctx, year)
}

func (r *postgresEpiWeekRepository) Upsert(ctx context.Context, arg db.UpsertEpiWeekParams) (db.EpiWeek, error) {
	return r.db.UpsertEpiWeek(ctx, arg)
}
