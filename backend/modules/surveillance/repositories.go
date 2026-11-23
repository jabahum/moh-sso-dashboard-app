package surveillance

import db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"

type Repositories struct {
	Diseases        DiseaseRepository
	Indicators      IndicatorRepository
	Regions         RegionRepository
	Districts       DistrictRepository
	SubCounties     SubCountyRepository
	Facilities      FacilityRepository
	EpiWeeks        EpiWeekRepository
	FacilityMetrics FacilityWeeklyMetricsRepository
	WeeklyStatus    WeeklyStatusRepository
	Dashboard       DashboardRepository
	Alerts          AlertRepository
	Lookup          LookupRepository
	Imports         ImportRepository
}

func NewRepositories(db db.Store) *Repositories {
	return &Repositories{
		Diseases:        NewDiseaseRepository(db),
		Indicators:      NewIndicatorRepository(db),
		Regions:         NewRegionRepository(db),
		Districts:       NewDistrictRepository(db),
		SubCounties:     NewSubCountyRepository(db),
		Facilities:      NewFacilityRepository(db),
		EpiWeeks:        NewEpiWeekRepository(db),
		FacilityMetrics: NewFacilityWeeklyMetricsRepository(db),
		WeeklyStatus:    NewWeeklyStatusRepository(db),
		Dashboard:       NewDashboardRepository(db),
		Alerts:          NewAlertRepository(db),
		Lookup:          NewLookupRepository(db),
		Imports:         NewImportRepository(db),
	}
}
