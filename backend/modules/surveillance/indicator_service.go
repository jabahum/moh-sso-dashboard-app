package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"log/slog"
)

type IndicatorService struct {
	log           *slog.Logger
	indicatorRepo IndicatorRepository
}

func NewIndicatorService(
	log *slog.Logger,
	indicatorRepo IndicatorRepository,
) *IndicatorService {
	return &IndicatorService{
		log:           log,
		indicatorRepo: indicatorRepo,
	}
}

func (s *IndicatorService) ListIndicators(ctx context.Context) ([]db.Indicator, error) {
	s.log.DebugContext(ctx, "listing indicators")

	items, err := s.indicatorRepo.List(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list indicators", "error", err)
		return []db.Indicator{}, err
	}

	return items, nil
}

func (s *IndicatorService) ListActiveIndicators(ctx context.Context) ([]db.Indicator, error) {
	s.log.DebugContext(ctx, "listing active indicators")

	items, err := s.indicatorRepo.ListActive(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to list active indicators", "error", err)
		return []db.Indicator{}, err
	}

	return items, nil
}

func (s *IndicatorService) GetIndicatorByID(ctx context.Context, id uuid.UUID) (db.Indicator, error) {
	s.log.DebugContext(ctx, "getting indicator by id", "indicator_id", id)

	item, err := s.indicatorRepo.GetByID(ctx, id)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get indicator by id", "indicator_id", id, "error", err)
		return db.Indicator{}, err
	}

	return item, nil
}

func (s *IndicatorService) GetIndicatorByName(ctx context.Context, name string) (db.Indicator, error) {
	s.log.DebugContext(ctx, "getting indicator by name", "name", name)

	item, err := s.indicatorRepo.GetByName(ctx, name)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get indicator by name", "name", name, "error", err)
		return db.Indicator{}, err
	}

	return item, nil
}

func (s *IndicatorService) CreateIndicator(ctx context.Context, arg db.CreateIndicatorParams) (db.Indicator, error) {
	s.log.InfoContext(ctx, "creating indicator", "name", arg.Name)

	item, err := s.indicatorRepo.Create(ctx, arg)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to create indicator", "name", arg.Name, "error", err)
		return db.Indicator{}, err
	}

	return item, nil
}

func (s *IndicatorService) UpsertIndicator(ctx context.Context, arg db.UpsertIndicatorParams) (db.Indicator, error) {
	s.log.InfoContext(ctx, "upserting indicator", "name", arg.Name)

	item, err := s.indicatorRepo.Upsert(ctx, arg)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to upsert indicator", "name", arg.Name, "error", err)
		return db.Indicator{}, err
	}

	return item, nil
}

func (s *IndicatorService) SetIndicatorActiveState(ctx context.Context, arg db.SetIndicatorActiveStateParams) (db.Indicator, error) {
	s.log.InfoContext(ctx, "setting indicator active state", "indicator_id", arg.ID)

	item, err := s.indicatorRepo.SetActiveState(ctx, arg)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to set indicator active state", "indicator_id", arg.ID, "error", err)
		return db.Indicator{}, err
	}

	return item, nil
}

func (s *IndicatorService) DeleteIndicator(ctx context.Context, id uuid.UUID) error {
	s.log.InfoContext(ctx, "deleting indicator", "indicator_id", id)

	if err := s.indicatorRepo.Delete(ctx, id); err != nil {
		s.log.ErrorContext(ctx, "failed to delete indicator", "indicator_id", id, "error", err)
		return err
	}

	return nil
}
