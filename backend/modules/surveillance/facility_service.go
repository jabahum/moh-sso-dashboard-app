package surveillance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"log/slog"
)

type FacilityService struct {
	log           *slog.Logger
	facilityRepo  FacilityRepository
	notifications modulekit.Notifier
}

func NewFacilityService(
	log *slog.Logger,
	facilityRepo FacilityRepository,
	notifications ...modulekit.Notifier,
) *FacilityService {
	var notificationSvc modulekit.Notifier
	if len(notifications) > 0 {
		notificationSvc = notifications[0]
	}

	return &FacilityService{
		log:           log,
		facilityRepo:  facilityRepo,
		notifications: notificationSvc,
	}
}

func (s *FacilityService) ListFacilities(
	ctx context.Context,
) ([]db.ListFacilitiesRow, error) {
	if s == nil {
		return nil, errors.New("surveillance facility service is nil")
	}

	if s.facilityRepo == nil {
		return nil, errors.New("facility repository is nil")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing facilities")
	}

	items, err := s.facilityRepo.List(ctx)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list facilities", "error", err)
		}

		return []db.ListFacilitiesRow{}, fmt.Errorf("list facilities: %w", err)
	}

	return items, nil
}

func (s *FacilityService) ListFacilitiesByDistrict(
	ctx context.Context,
	districtID uuid.UUID,
) ([]db.Facility, error) {
	if s == nil {
		return nil, errors.New("surveillance facility service is nil")
	}

	if s.facilityRepo == nil {
		return nil, errors.New("facility repository is nil")
	}

	if districtID == uuid.Nil {
		return nil, errors.New("district id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing facilities by district", "district_id", districtID)
	}

	items, err := s.facilityRepo.ListByDistrict(ctx, districtID)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list facilities by district", "district_id", districtID, "error", err)
		}

		return []db.Facility{}, fmt.Errorf("list facilities by district: %w", err)
	}

	return items, nil
}

func (s *FacilityService) GetFacilityByID(
	ctx context.Context,
	id uuid.UUID,
) (db.Facility, error) {
	if s == nil {
		return db.Facility{}, errors.New("surveillance facility service is nil")
	}

	if s.facilityRepo == nil {
		return db.Facility{}, errors.New("facility repository is nil")
	}

	if id == uuid.Nil {
		return db.Facility{}, errors.New("facility id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting facility by id", "facility_id", id)
	}

	item, err := s.facilityRepo.GetByID(ctx, id)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get facility by id", "facility_id", id, "error", err)
		}

		return db.Facility{}, fmt.Errorf("get facility by id: %w", err)
	}

	return item, nil
}

func (s *FacilityService) GetFacilityByExternalID(
	ctx context.Context,
	externalID string,
) (db.Facility, error) {
	if s == nil {
		return db.Facility{}, errors.New("surveillance facility service is nil")
	}

	if s.facilityRepo == nil {
		return db.Facility{}, errors.New("facility repository is nil")
	}

	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return db.Facility{}, errors.New("facility external id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting facility by external id", "external_id", externalID)
	}

	item, err := s.facilityRepo.GetByExternalID(ctx, externalID)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get facility by external id", "external_id", externalID, "error", err)
		}

		return db.Facility{}, fmt.Errorf("get facility by external id: %w", err)
	}

	return item, nil
}

func (s *FacilityService) UpsertFacilityByExternalID(
	ctx context.Context,
	arg db.UpsertFacilityByExternalIDParams,
) (db.Facility, error) {
	if s == nil {
		return db.Facility{}, errors.New("surveillance facility service is nil")
	}

	if s.facilityRepo == nil {
		return db.Facility{}, errors.New("facility repository is nil")
	}

	arg.Name = strings.TrimSpace(arg.Name)

	if arg.Name == "" {
		return db.Facility{}, errors.New("facility name is required")
	}

	if s.log != nil {
		s.log.InfoContext(ctx, "upserting facility by external id", "external_id", arg.ExternalID, "name", arg.Name)
	}

	item, err := s.facilityRepo.UpsertByExternalID(ctx, arg)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to upsert facility by external id", "external_id", arg.ExternalID, "error", err)
		}

		return db.Facility{}, fmt.Errorf("upsert facility by external id: %w", err)
	}

	s.notify(ctx, modulekit.Notification{
		Type:       "SURVEILLANCE_FACILITY_UPSERTED",
		Title:      "Facility updated",
		Severity:   "info",
		Message:    "Surveillance facility created or updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"facility_id": item.ID.String(),
			"name":        item.Name,
			"external_id": nullStringValue(item.ExternalID),
			"region_id":   nullUUIDString(item.RegionID),
			"sub_county_id": nullUUIDString(
				item.SubCountyID,
			),
		}),
	})

	return item, nil
}

func (s *FacilityService) UpsertFacilityByNameDistrict(
	ctx context.Context,
	arg db.UpsertFacilityByNameDistrictParams,
) (db.Facility, error) {
	if s == nil {
		return db.Facility{}, errors.New("surveillance facility service is nil")
	}

	if s.facilityRepo == nil {
		return db.Facility{}, errors.New("facility repository is nil")
	}

	arg.Name = strings.TrimSpace(arg.Name)

	if arg.Name == "" {
		return db.Facility{}, errors.New("facility name is required")
	}

	if s.log != nil {
		s.log.InfoContext(ctx, "upserting facility by name and district", "name", arg.Name, "district_id", arg.DistrictID)
	}

	item, err := s.facilityRepo.UpsertByNameDistrict(ctx, arg)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to upsert facility by name and district", "name", arg.Name, "error", err)
		}

		return db.Facility{}, fmt.Errorf("upsert facility by name and district: %w", err)
	}

	s.notify(ctx, modulekit.Notification{
		Type:       "SURVEILLANCE_FACILITY_UPSERTED",
		Title:      "Facility updated",
		Severity:   "info",
		Message:    "Surveillance facility created or updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"facility_id": item.ID.String(),
			"name":        item.Name,
			"external_id": nullStringValue(item.ExternalID),
			"district_id": item.DistrictID,
			"region_id":   nullUUIDString(item.RegionID),
			"sub_county_id": nullUUIDString(
				item.SubCountyID,
			),
		}),
	})

	return item, nil
}

func (s *FacilityService) notify(
	ctx context.Context,
	notification modulekit.Notification,
) {
	if s == nil || s.notifications == nil {
		return
	}

	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}

	if err := s.notifications.Notify(ctx, notification); err != nil {
		if s.log != nil {
			s.log.ErrorContext(
				ctx,
				"surveillance facility notification failed",
				"type", notification.Type,
				"error", err,
			)
		}
	}
}

func nullUUIDString(value uuid.NullUUID) string {
	if !value.Valid {
		return ""
	}

	return value.UUID.String()
}
