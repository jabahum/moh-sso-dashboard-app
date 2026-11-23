package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresIndicatorRepository struct {
	db db.Store
}

func NewIndicatorRepository(db db.Store) IndicatorRepository {
	return &postgresIndicatorRepository{db: db}
}

func (r *postgresIndicatorRepository) Create(ctx context.Context, arg db.CreateIndicatorParams) (db.Indicator, error) {
	return r.db.CreateIndicator(ctx, arg)
}

func (r *postgresIndicatorRepository) GetByID(ctx context.Context, id uuid.UUID) (db.Indicator, error) {
	return r.db.GetIndicatorByID(ctx, id)
}

func (r *postgresIndicatorRepository) GetByName(ctx context.Context, name string) (db.Indicator, error) {
	return r.db.GetIndicatorByName(ctx, name)
}

func (r *postgresIndicatorRepository) List(ctx context.Context) ([]db.Indicator, error) {
	return r.db.ListIndicators(ctx)
}

func (r *postgresIndicatorRepository) ListActive(ctx context.Context) ([]db.Indicator, error) {
	return r.db.ListActiveIndicators(ctx)
}

func (r *postgresIndicatorRepository) Upsert(ctx context.Context, arg db.UpsertIndicatorParams) (db.Indicator, error) {
	return r.db.UpsertIndicator(ctx, arg)
}

func (r *postgresIndicatorRepository) SetActiveState(ctx context.Context, arg db.SetIndicatorActiveStateParams) (db.Indicator, error) {
	return r.db.SetIndicatorActiveState(ctx, arg)
}

func (r *postgresIndicatorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DeleteIndicator(ctx, id)
}
