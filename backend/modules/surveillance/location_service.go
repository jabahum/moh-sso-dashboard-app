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

type LocationService struct {
	log           *slog.Logger
	regionRepo    RegionRepository
	districtRepo  DistrictRepository
	subCountyRepo SubCountyRepository
	notifications modulekit.Notifier
	cfg           *modulekit.Platform
}

func NewLocationService(
	log *slog.Logger,
	regionRepo RegionRepository,
	districtRepo DistrictRepository,
	subCountyRepo SubCountyRepository,
	notifications modulekit.Notifier,
	cfg ...*modulekit.Platform,
) *LocationService {
	var appConfig *modulekit.Platform
	if len(cfg) > 0 {
		appConfig = cfg[0]
	}

	return &LocationService{
		log:           log,
		regionRepo:    regionRepo,
		districtRepo:  districtRepo,
		subCountyRepo: subCountyRepo,
		notifications: notifications,
		cfg:           appConfig,
	}
}

// ============================================================
// Regions
// ============================================================

func (s *LocationService) ListRegions(ctx context.Context) ([]db.Region, error) {
	if s == nil {
		return nil, errors.New("surveillance location service is nil")
	}

	if s.regionRepo == nil {
		return nil, errors.New("region repository is nil")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing regions")
	}

	items, err := s.regionRepo.List(ctx)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list regions", "error", err)
		}

		return []db.Region{}, fmt.Errorf("list regions: %w", err)
	}

	return items, nil
}

func (s *LocationService) ListRegionsInHealthContext(
	ctx context.Context,
	scope HealthContextScope,
) ([]db.Region, error) {
	if scope.ID == uuid.Nil {
		return nil, errors.New("health context id is required")
	}
	return s.regionRepo.ListInHealthContext(ctx, scope)
}

func (s *LocationService) GetRegionByID(
	ctx context.Context,
	id uuid.UUID,
) (db.Region, error) {
	if s == nil {
		return db.Region{}, errors.New("surveillance location service is nil")
	}

	if s.regionRepo == nil {
		return db.Region{}, errors.New("region repository is nil")
	}

	if id == uuid.Nil {
		return db.Region{}, errors.New("region id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting region by id", "region_id", id)
	}

	item, err := s.regionRepo.GetByID(ctx, id)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get region by id", "region_id", id, "error", err)
		}

		return db.Region{}, fmt.Errorf("get region by id: %w", err)
	}

	return item, nil
}

func (s *LocationService) GetRegionByName(
	ctx context.Context,
	name string,
) (db.Region, error) {
	if s == nil {
		return db.Region{}, errors.New("surveillance location service is nil")
	}

	if s.regionRepo == nil {
		return db.Region{}, errors.New("region repository is nil")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return db.Region{}, errors.New("region name is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting region by name", "name", name)
	}

	item, err := s.regionRepo.GetByName(ctx, name)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get region by name", "name", name, "error", err)
		}

		return db.Region{}, fmt.Errorf("get region by name: %w", err)
	}

	return item, nil
}

func (s *LocationService) UpsertRegion(
	ctx context.Context,
	arg db.UpsertRegionParams,
) (db.Region, error) {
	if s == nil {
		return db.Region{}, errors.New("surveillance location service is nil")
	}

	if s.regionRepo == nil {
		return db.Region{}, errors.New("region repository is nil")
	}

	arg.Name = strings.TrimSpace(arg.Name)
	if arg.Name == "" {
		return db.Region{}, errors.New("region name is required")
	}

	if s.log != nil {
		s.log.InfoContext(ctx, "upserting region", "name", arg.Name)
	}

	item, err := s.regionRepo.Upsert(ctx, arg)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to upsert region", "name", arg.Name, "error", err)
		}

		return db.Region{}, fmt.Errorf("upsert region: %w", err)
	}

	// In-app only. Location upserts can happen during imports.
	s.notify(ctx, modulekit.Notification{
		Type:       "SURVEILLANCE_REGION_UPSERTED",
		Title:      "Region updated",
		Severity:   "info",
		Message:    "Surveillance region created or updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"region_id": item.ID.String(),
			"name":      item.Name,
		}),
	})

	return item, nil
}

// ============================================================
// Districts
// ============================================================

func (s *LocationService) ListDistricts(
	ctx context.Context,
) ([]db.ListDistrictsRow, error) {
	if s == nil {
		return nil, errors.New("surveillance location service is nil")
	}

	if s.districtRepo == nil {
		return nil, errors.New("district repository is nil")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing districts")
	}

	items, err := s.districtRepo.List(ctx)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list districts", "error", err)
		}

		return []db.ListDistrictsRow{}, fmt.Errorf("list districts: %w", err)
	}

	return items, nil
}

func (s *LocationService) ListDistrictsInHealthContext(
	ctx context.Context,
	scope HealthContextScope,
) ([]db.ListDistrictsRow, error) {
	if scope.ID == uuid.Nil {
		return nil, errors.New("health context id is required")
	}
	return s.districtRepo.ListInHealthContext(ctx, scope)
}

func (s *LocationService) ListDistrictsByRegion(
	ctx context.Context,
	regionID uuid.UUID,
) ([]db.District, error) {
	if s == nil {
		return nil, errors.New("surveillance location service is nil")
	}

	if s.districtRepo == nil {
		return nil, errors.New("district repository is nil")
	}

	if regionID == uuid.Nil {
		return nil, errors.New("region id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing districts by region", "region_id", regionID)
	}

	items, err := s.districtRepo.ListByRegion(ctx, regionID)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list districts by region", "region_id", regionID, "error", err)
		}

		return []db.District{}, fmt.Errorf("list districts by region: %w", err)
	}

	return items, nil
}

func (s *LocationService) ListDistrictsByRegionInHealthContext(
	ctx context.Context,
	regionID uuid.UUID,
	scope HealthContextScope,
) ([]db.District, error) {
	if scope.ID == uuid.Nil {
		return nil, errors.New("health context id is required")
	}
	return s.districtRepo.ListByRegionInHealthContext(ctx, regionID, scope)
}

func (s *LocationService) GetDistrictByID(
	ctx context.Context,
	id uuid.UUID,
) (db.District, error) {
	if s == nil {
		return db.District{}, errors.New("surveillance location service is nil")
	}

	if s.districtRepo == nil {
		return db.District{}, errors.New("district repository is nil")
	}

	if id == uuid.Nil {
		return db.District{}, errors.New("district id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting district by id", "district_id", id)
	}

	item, err := s.districtRepo.GetByID(ctx, id)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get district by id", "district_id", id, "error", err)
		}

		return db.District{}, fmt.Errorf("get district by id: %w", err)
	}

	return item, nil
}

func (s *LocationService) GetDistrictByName(
	ctx context.Context,
	name string,
) (db.District, error) {
	if s == nil {
		return db.District{}, errors.New("surveillance location service is nil")
	}

	if s.districtRepo == nil {
		return db.District{}, errors.New("district repository is nil")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return db.District{}, errors.New("district name is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting district by name", "name", name)
	}

	item, err := s.districtRepo.GetByName(ctx, name)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get district by name", "name", name, "error", err)
		}

		return db.District{}, fmt.Errorf("get district by name: %w", err)
	}

	return item, nil
}

func (s *LocationService) UpsertDistrict(
	ctx context.Context,
	arg db.UpsertDistrictParams,
) (db.District, error) {
	if s == nil {
		return db.District{}, errors.New("surveillance location service is nil")
	}

	if s.districtRepo == nil {
		return db.District{}, errors.New("district repository is nil")
	}

	arg.Name = strings.TrimSpace(arg.Name)
	if arg.Name == "" {
		return db.District{}, errors.New("district name is required")
	}

	if !arg.RegionID.Valid || arg.RegionID.UUID == uuid.Nil {
		return db.District{}, errors.New("region id is required")
	}

	if s.log != nil {
		s.log.InfoContext(
			ctx,
			"upserting district",
			"name", arg.Name,
			"region_id", arg.RegionID.UUID.String(),
		)
	}

	item, err := s.districtRepo.Upsert(ctx, arg)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to upsert district", "name", arg.Name, "error", err)
		}

		return db.District{}, fmt.Errorf("upsert district: %w", err)
	}

	// In-app only. Location upserts can happen during imports.
	s.notify(ctx, modulekit.Notification{
		Type:       "SURVEILLANCE_DISTRICT_UPSERTED",
		Title:      "District updated",
		Severity:   "info",
		Message:    "Surveillance district created or updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"district_id": item.ID.String(),
			"name":        item.Name,
			"region_id":   nullUUIDString(item.RegionID),
		}),
	})

	return item, nil
}

func (s *LocationService) UpsertDistrictFromRequest(
	ctx context.Context,
	req UpsertDistrictRequest,
) (db.District, error) {
	return s.UpsertDistrict(ctx, db.UpsertDistrictParams{
		Name:     req.Name,
		RegionID: uuidNullFromPtr(req.RegionID),
		Code:     sqlNullStringFromPtr(req.Code),
	})
}

// ============================================================
// Sub-counties
// ============================================================

func (s *LocationService) ListSubcountiesByDistrict(
	ctx context.Context,
	districtID uuid.UUID,
) ([]db.SubCounty, error) {
	if s == nil {
		return nil, errors.New("surveillance location service is nil")
	}

	if s.subCountyRepo == nil {
		return nil, errors.New("subcounty repository is nil")
	}

	if districtID == uuid.Nil {
		return []db.SubCounty{}, errors.New("district id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing subcounties by district", "district_id", districtID)
	}

	items, err := s.subCountyRepo.ListByDistrict(ctx, districtID)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list subcounties by district", "district_id", districtID, "error", err)
		}

		return []db.SubCounty{}, fmt.Errorf("list subcounties by district: %w", err)
	}

	return items, nil
}

func (s *LocationService) ListSubcountiesByDistrictInHealthContext(
	ctx context.Context,
	districtID uuid.UUID,
	scope HealthContextScope,
) ([]db.SubCounty, error) {
	if scope.ID == uuid.Nil {
		return nil, errors.New("health context id is required")
	}
	return s.subCountyRepo.ListByDistrictInHealthContext(ctx, districtID, scope)
}

func (s *LocationService) GetSubcountyByID(
	ctx context.Context,
	id uuid.UUID,
) (db.SubCounty, error) {
	if s == nil {
		return db.SubCounty{}, errors.New("surveillance location service is nil")
	}

	if s.subCountyRepo == nil {
		return db.SubCounty{}, errors.New("subcounty repository is nil")
	}

	if id == uuid.Nil {
		return db.SubCounty{}, errors.New("subcounty id is required")
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting subcounty by id", "id", id)
	}

	item, err := s.subCountyRepo.GetByID(ctx, id)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get subcounty by id", "id", id, "error", err)
		}

		return db.SubCounty{}, fmt.Errorf("get subcounty by id: %w", err)
	}

	return item, nil
}

func (s *LocationService) UpsertSubcounty(
	ctx context.Context,
	arg db.UpsertSubCountyParams,
) (db.SubCounty, error) {
	if s == nil {
		return db.SubCounty{}, errors.New("surveillance location service is nil")
	}

	if s.subCountyRepo == nil {
		return db.SubCounty{}, errors.New("subcounty repository is nil")
	}

	arg.Name = strings.TrimSpace(arg.Name)
	if arg.Name == "" {
		return db.SubCounty{}, errors.New("subcounty name is required")
	}

	if arg.DistrictID == uuid.Nil {
		return db.SubCounty{}, errors.New("district id is required")
	}

	if s.log != nil {
		s.log.InfoContext(ctx, "upserting subcounty", "name", arg.Name, "district_id", arg.DistrictID)
	}

	item, err := s.subCountyRepo.Upsert(ctx, arg)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to upsert subcounty", "name", arg.Name, "district_id", arg.DistrictID, "error", err)
		}

		return db.SubCounty{}, fmt.Errorf("upsert subcounty: %w", err)
	}

	// In-app only. Location upserts can happen during imports.
	s.notify(ctx, modulekit.Notification{
		Type:       "SURVEILLANCE_SUBCOUNTY_UPSERTED",
		Title:      "Sub-county updated",
		Severity:   "info",
		Message:    "Surveillance sub-county created or updated",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"subcounty_id": item.ID.String(),
			"name":         item.Name,
			"district_id":  item.DistrictID.String(),
		}),
	})

	return item, nil
}

func (s *LocationService) UpsertSubcountyFromRequest(
	ctx context.Context,
	req UpsertSubCountyRequest,
) (db.SubCounty, error) {
	if req.DistrictID == nil {
		return db.SubCounty{}, errors.New("district id is required")
	}
	return s.UpsertSubcounty(ctx, db.UpsertSubCountyParams{
		Name:       req.Name,
		DistrictID: *req.DistrictID,
		Code:       sqlNullStringFromPtr(req.Code),
	})
}

func (s *LocationService) DeleteSubcounty(
	ctx context.Context,
	id uuid.UUID,
) error {
	if s == nil {
		return errors.New("surveillance location service is nil")
	}

	if s.subCountyRepo == nil {
		return errors.New("subcounty repository is nil")
	}

	if id == uuid.Nil {
		return errors.New("subcounty id is required")
	}

	if s.log != nil {
		s.log.InfoContext(ctx, "deleting subcounty", "id", id)
	}

	item, _ := s.subCountyRepo.GetByID(ctx, id)

	if err := s.subCountyRepo.Delete(ctx, id); err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to delete subcounty", "id", id, "error", err)
		}

		return fmt.Errorf("delete subcounty: %w", err)
	}

	if item.ID != uuid.Nil {
		notification := modulekit.Notification{
			Type:       "SURVEILLANCE_SUBCOUNTY_DELETED",
			Title:      "Sub-county deleted",
			Severity:   "warning",
			Message:    "Surveillance sub-county deleted",
			TargetRole: "admin",
			Metadata: mustJSON(map[string]any{
				"subcounty_id": item.ID.String(),
				"name":         item.Name,
				"district_id":  item.DistrictID.String(),
			}),
		}

		s.attachAdminEmailDelivery(
			&notification,
			"admin-alert",
			"Surveillance sub-county deleted",
			fmt.Sprintf("Surveillance sub-county %s was deleted.", item.Name),
			map[string]any{
				"Name":          s.systemAdminName(),
				"Platform":      s.platformName(),
				"Message":       "A surveillance sub-county was deleted.",
				"SubCountyName": item.Name,
				"SubCountyID":   item.ID.String(),
				"DistrictID":    item.DistrictID.String(),
				"ActionURL":     s.adminSurveillanceURL(),
				"Details": fmt.Sprintf(
					"Sub-county ID: %s\nName: %s\nDistrict ID: %s",
					item.ID.String(),
					item.Name,
					item.DistrictID.String(),
				),
			},
		)

		s.notify(ctx, notification)
	}

	return nil
}

func (s *LocationService) notify(
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
				"surveillance location notification failed",
				"type", notification.Type,
				"error", err,
			)
		}
	}
}

func (s *LocationService) attachAdminEmailDelivery(
	notification *modulekit.Notification,
	templateName string,
	subject string,
	textBody string,
	templateData map[string]any,
) {
	if notification == nil {
		return
	}

	adminEmail := strings.TrimSpace(s.systemAdminEmail())
	if adminEmail == "" {
		return
	}

	if templateData == nil {
		templateData = map[string]any{}
	}

	if _, ok := templateData["Name"]; !ok {
		templateData["Name"] = s.systemAdminName()
	}

	if _, ok := templateData["Platform"]; !ok {
		templateData["Platform"] = s.platformName()
	}

	if _, ok := templateData["ActionURL"]; !ok {
		templateData["ActionURL"] = s.adminSurveillanceURL()
	}

	notification.Deliveries = []modulekit.NotificationDelivery{
		{
			Channel: modulekit.ChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		},
		{
			Channel: modulekit.ChannelEmail,
			Recipient: map[string]any{
				"name":  s.systemAdminName(),
				"email": adminEmail,
			},
			TemplateName: templateName,
			TemplateData: templateData,
			Payload: map[string]any{
				"subject":   subject,
				"text_body": textBody,
			},
			MaxAttempts: 5,
		},
	}
}

func (s *LocationService) platformName() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.Name) != "" {
		return strings.TrimSpace(s.cfg.Name)
	}

	return "MOH Integrated Health Portal"
}

func (s *LocationService) systemAdminName() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.SystemAdminName) != "" {
		return strings.TrimSpace(s.cfg.SystemAdminName)
	}

	return "System Administrator"
}

func (s *LocationService) systemAdminEmail() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.SystemAdminEmail) != "" {
		return strings.TrimSpace(s.cfg.SystemAdminEmail)
	}

	return ""
}

func (s *LocationService) adminDashboardURL() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.AdminDashboardURL) != "" {
		return strings.TrimSpace(s.cfg.AdminDashboardURL)
	}

	return "http://localhost:3000/admin/home"
}

func (s *LocationService) adminSurveillanceURL() string {
	base := strings.TrimRight(s.adminDashboardURL(), "/")

	if strings.HasSuffix(base, "/admin/home") {
		return strings.TrimSuffix(base, "/admin/home") + "/surveillance"
	}

	return base + "/surveillance"
}
