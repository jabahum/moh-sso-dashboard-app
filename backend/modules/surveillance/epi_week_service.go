package surveillance

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"log/slog"
)

type EpiWeekService struct {
	log           *slog.Logger
	epiWeekRepo   EpiWeekRepository
	notifications modulekit.Notifier
}

func NewEpiWeekService(
	log *slog.Logger,
	epiWeekRepo EpiWeekRepository,
	notifications ...modulekit.Notifier,
) *EpiWeekService {
	var notificationSvc modulekit.Notifier
	if len(notifications) > 0 {
		notificationSvc = notifications[0]
	}

	return &EpiWeekService{
		log:           log,
		epiWeekRepo:   epiWeekRepo,
		notifications: notificationSvc,
	}
}

func (s *EpiWeekService) GetEpiWeekByID(
	ctx context.Context,
	id uuid.UUID,
) (db.EpiWeek, error) {
	if s == nil {
		return db.EpiWeek{}, errors.New("surveillance epi week service is nil")
	}

	if s.epiWeekRepo == nil {
		return db.EpiWeek{}, errors.New("epi week repository is nil")
	}

	if id == uuid.Nil {
		return db.EpiWeek{}, errors.New("epi week id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting epi week by id", "week_id", id)
	}

	item, err := s.epiWeekRepo.GetByID(ctx, id)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get epi week by id", "week_id", id, "error", err)
		}

		return db.EpiWeek{}, fmt.Errorf("get epi week by id: %w", err)
	}

	return item, nil
}

func (s *EpiWeekService) GetEpiWeekByYearWeek(
	ctx context.Context,
	arg db.GetEpiWeekByYearWeekParams,
) (db.EpiWeek, error) {
	if s == nil {
		return db.EpiWeek{}, errors.New("surveillance epi week service is nil")
	}

	if s.epiWeekRepo == nil {
		return db.EpiWeek{}, errors.New("epi week repository is nil")
	}

	if arg.EpiYear <= 0 {
		return db.EpiWeek{}, errors.New("epi year is required")
	}

	if arg.EpiWeek <= 0 {
		return db.EpiWeek{}, errors.New("epi week must be greater than 0")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting epi week by year/week", "year", arg.EpiYear, "week", arg.EpiWeek)
	}

	item, err := s.epiWeekRepo.GetByYearWeek(ctx, arg)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get epi week by year/week", "year", arg.EpiYear, "week", arg.EpiWeek, "error", err)
		}

		return db.EpiWeek{}, fmt.Errorf("get epi week by year/week: %w", err)
	}

	return item, nil
}

func (s *EpiWeekService) GetEpiWeekByYearWeekValue(
	ctx context.Context,
	year int32,
	week int32,
) (db.EpiWeek, error) {
	return s.GetEpiWeekByYearWeek(ctx, db.GetEpiWeekByYearWeekParams{
		EpiYear: year,
		EpiWeek: week,
	})
}

func (s *EpiWeekService) ListEpiWeeksByYear(
	ctx context.Context,
	year int32,
) ([]db.EpiWeek, error) {
	if s == nil {
		return nil, errors.New("surveillance epi week service is nil")
	}

	if s.epiWeekRepo == nil {
		return nil, errors.New("epi week repository is nil")
	}

	if year <= 0 {
		return nil, errors.New("epi year is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing epi weeks by year", "year", year)
	}

	items, err := s.epiWeekRepo.ListByYear(ctx, year)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list epi weeks by year", "year", year, "error", err)
		}

		return []db.EpiWeek{}, fmt.Errorf("list epi weeks by year: %w", err)
	}

	return items, nil
}

func (s *EpiWeekService) UpsertEpiWeek(
	ctx context.Context,
	arg db.UpsertEpiWeekParams,
) (db.EpiWeek, error) {
	if s == nil {
		return db.EpiWeek{}, errors.New("surveillance epi week service is nil")
	}

	if s.epiWeekRepo == nil {
		return db.EpiWeek{}, errors.New("epi week repository is nil")
	}

	if arg.EpiYear <= 0 {
		return db.EpiWeek{}, errors.New("epi year is required")
	}

	if arg.EpiWeek <= 0 {
		return db.EpiWeek{}, errors.New("epi week must be greater than 0")
	}

	if arg.EpiWeek > 53 {
		return db.EpiWeek{}, errors.New("epi week must not be greater than 53")
	}

	if s.log != nil {
		s.log.InfoContext(ctx, "upserting epi week", "year", arg.EpiYear, "week", arg.EpiWeek)
	}

	item, err := s.epiWeekRepo.Upsert(ctx, arg)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to upsert epi week", "year", arg.EpiYear, "week", arg.EpiWeek, "error", err)
		}

		return db.EpiWeek{}, fmt.Errorf("upsert epi week: %w", err)
	}

	s.notify(ctx, modulekit.Notification{
		Type:       "SURVEILLANCE_EPI_WEEK_UPSERTED",
		Title:      "EPI week updated",
		Severity:   "info",
		Message:    "Surveillance EPI week created or updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"epi_week_id": item.ID.String(),
			"epi_year":    item.EpiYear,
			"epi_week":    item.EpiWeek,
			"start_date":  item.WeekStartDate,
			"end_date":    item.WeekEndDate,
		}),
	})

	return item, nil
}

func (s *EpiWeekService) notify(
	ctx context.Context,
	notification modulekit.Notification,
) {
	if s == nil || s.notifications == nil {
		return
	}

	if notification.TargetRole == "" {
		notification.TargetRole = "admin"
	}

	if err := s.notifications.Notify(ctx, notification); err != nil {
		if s.log != nil {
			s.log.ErrorContext(
				ctx,
				"surveillance epi week notification failed",
				"type", notification.Type,
				"error", err,
			)
		}
	}
}
