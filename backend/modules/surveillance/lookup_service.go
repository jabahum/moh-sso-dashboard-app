package surveillance

import (
	"context"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"log/slog"
)

type LookupService struct {
	log        *slog.Logger
	lookupRepo LookupRepository
}

func NewLookupService(
	log *slog.Logger,
	lookupRepo LookupRepository,
) *LookupService {
	return &LookupService{
		log:        log,
		lookupRepo: lookupRepo,
	}
}

func (s *LookupService) ResolveDiseaseByName(ctx context.Context, name string) (db.Disease, error) {
	return s.lookupRepo.ResolveDiseaseByName(ctx, name)
}

func (s *LookupService) ResolveIndicatorByName(ctx context.Context, name string) (db.Indicator, error) {
	return s.lookupRepo.ResolveIndicatorByName(ctx, name)
}

func (s *LookupService) ResolveWeekByYearWeek(ctx context.Context, arg db.ResolveWeekByYearWeekParams) (db.EpiWeek, error) {
	return s.lookupRepo.ResolveWeekByYearWeek(ctx, arg)
}

func (s *LookupService) ResolveDistrictByName(ctx context.Context, name string) (db.District, error) {
	return s.lookupRepo.ResolveDistrictByName(ctx, name)
}

func (s *LookupService) ResolveRegionByName(ctx context.Context, name string) (db.Region, error) {
	return s.lookupRepo.ResolveRegionByName(ctx, name)
}

func (s *LookupService) ResolveSubCountyByNameAndDistrict(ctx context.Context, arg db.ResolveSubCountyByNameAndDistrictParams) (db.SubCounty, error) {
	return s.lookupRepo.ResolveSubCountyByNameAndDistrict(ctx, arg)
}

func (s *LookupService) ResolveFacilityByExternalID(ctx context.Context, externalID string) (db.Facility, error) {
	return s.lookupRepo.ResolveFacilityByExternalID(ctx, externalID)
}

func (s *LookupService) ResolveFacilityByNameAndDistrict(ctx context.Context, arg db.ResolveFacilityByNameAndDistrictParams) (db.Facility, error) {
	return s.lookupRepo.ResolveFacilityByNameAndDistrict(ctx, arg)
}
