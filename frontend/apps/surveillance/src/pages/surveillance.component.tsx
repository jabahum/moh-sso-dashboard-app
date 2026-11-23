import {
  useSurveillanceRecovery,
  SurveillanceUnavailableNotice,
} from "./surveillance-availability";
import { PageShell } from "@moh-sso/ui";
import { InlineLoading } from "@carbon/react";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { Feature, FeatureCollection, GeoJsonProperties, Geometry } from "geojson";

import { districtKey } from "./surveillance-geography";

import SurveillanceFilters from "./surveillance-filters.component";
import SurveillanceTiles from "./surveillance-tiles.component";
import SurveillancePanels from "./surveillance-panel.component";
import SurveillanceUgandaMap from "./surveillance-uganda-map.component";
import "./surveillance.scss";

import {
  useGetGeoJsonQuery,
  useListAlertsQuery,
  useListDiseasesQuery,
  useListDistrictsByRegionQuery,
  useListEpiWeeksQuery,
  useListFacilityWeeklyMetricsByWeekQuery,
  useListRegionsQuery,
  useListSubcountiesByDistrictQuery,
  useListWeeklyStatusesDetailedQuery,
  useListWeeklyStatusesQuery,
} from "../api";

type FilterOption = {
  value: string;
  label: string;
};

type GenericFeatureCollection = FeatureCollection<Geometry, GeoJsonProperties>;

const EMPTY_LIST: never[] = [];

const EMPTY_FEATURE_COLLECTION: GenericFeatureCollection = {
  type: "FeatureCollection",
  features: [],
};

function slugifyDiseaseName(name: string) {
  return name.trim().toLowerCase().replace(/\s+/g, "-");
}

function normalize(value: unknown): string {
  if (typeof value !== "string") {
    return "";
  }

  return value.trim().toLowerCase();
}

function asFeatureCollection(value: unknown): GenericFeatureCollection {
  if (
    value &&
    typeof value === "object" &&
    "type" in value &&
    (value as { type?: string }).type === "FeatureCollection" &&
    "features" in value &&
    Array.isArray((value as { features?: unknown[] }).features)
  ) {
    return value as GenericFeatureCollection;
  }

  return EMPTY_FEATURE_COLLECTION;
}

function getDistrictFeatureName(properties?: GeoJsonProperties | null): string {
  if (!properties) {
    return "";
  }

  return districtKey(
    properties.District ??
      properties.district ??
      properties.district_name ??
      properties.name ??
      properties.DISTRICT ??
      "",
  );
}

function getSubcountyFeatureName(properties?: GeoJsonProperties | null): string {
  if (!properties) {
    return "";
  }

  return normalize(
    properties.Subcounty ??
      properties.sname2019 ??
      properties.subcounty ??
      properties.sub_county ??
      properties.subcounty_name ??
      properties.name ??
      properties.SCOUNTY ??
      "",
  );
}

function filterFeatureCollection(
  collection: GenericFeatureCollection,
  predicate: (feature: Feature<Geometry, GeoJsonProperties>) => boolean,
): GenericFeatureCollection {
  return {
    ...collection,
    features: collection.features.filter(predicate),
  };
}

function SurveillanceAccessDenied() {
  return (
    <div className="surveillance-dashboard-page">
      <div className="surveillance-dashboard-page__header">
        <div className="surveillance-dashboard-page__hero">
          <div className="surveillance-dashboard-page__hero-main">
            <div className="surveillance-dashboard-page__hero-copy">
              <h1 className="surveillance-dashboard-page__title">Access denied</h1>
              <p className="surveillance-dashboard-page__subtitle">
                You need surveillance read access to view this workspace.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function SurveillanceDashboardContent() {
  const navigate = useNavigate();

  const [selectedWeekId, setSelectedWeekId] = useState("");
  const [selectedRegionId, setSelectedRegionId] = useState("");
  const [selectedDistrictId, setSelectedDistrictId] = useState("");
  const [selectedSubCountyId, setSelectedSubCountyId] = useState("");

  const currentYear = new Date().getFullYear();

  const weeksQuery = useListEpiWeeksQuery(currentYear);
  const { data: weeksResponse, isLoading: weeksLoading } = weeksQuery;

  const diseasesQuery = useListDiseasesQuery();
  const { data: diseasesResponse, isLoading: diseasesLoading } = diseasesQuery;

  const regionsQuery = useListRegionsQuery();
  const { data: regionsResponse, isLoading: regionsReferenceLoading } = regionsQuery;

  const districtsQuery = useListDistrictsByRegionQuery(selectedRegionId, {
    skip: !selectedRegionId,
  });
  const { currentData: districtsResponse, isLoading: districtsReferenceLoading } = districtsQuery;

  const subcountiesQuery = useListSubcountiesByDistrictQuery(selectedDistrictId, {
    skip: !selectedDistrictId,
  });
  const { currentData: subcountiesResponse, isLoading: subcountiesReferenceLoading } =
    subcountiesQuery;

  const { data: districtsGeoJsonResponse, isLoading: districtsGeoJsonLoading } =
    useGetGeoJsonQuery("geo_districts");

  const { data: subcountiesGeoJsonResponse, isLoading: subcountiesGeoJsonLoading } =
    useGetGeoJsonQuery("geo_subcounties");

  const { data: waterGeoJsonResponse, isLoading: waterGeoJsonLoading } =
    useGetGeoJsonQuery("geo_water");

  const weeks = Array.isArray(weeksResponse) ? weeksResponse : EMPTY_LIST;

  const diseases = Array.isArray(diseasesResponse) ? diseasesResponse : EMPTY_LIST;

  const regions = Array.isArray(regionsResponse) ? regionsResponse : EMPTY_LIST;

  const districts = Array.isArray(districtsResponse) ? districtsResponse : EMPTY_LIST;

  const subcounties = Array.isArray(subcountiesResponse) ? subcountiesResponse : EMPTY_LIST;

  const districtGeoJson = useMemo(
    () => asFeatureCollection(districtsGeoJsonResponse),
    [districtsGeoJsonResponse],
  );

  const subcountyGeoJson = useMemo(
    () => asFeatureCollection(subcountiesGeoJsonResponse),
    [subcountiesGeoJsonResponse],
  );

  const waterGeoJson = useMemo(
    () => asFeatureCollection(waterGeoJsonResponse),
    [waterGeoJsonResponse],
  );

  const weeklyStatusParams = useMemo(() => {
    if (!selectedWeekId) {
      return undefined;
    }

    return {
      epiWeekID: selectedWeekId,
      regionID: selectedRegionId || undefined,
      districtID: selectedDistrictId || undefined,
      subCountyID: selectedSubCountyId || undefined,
    };
  }, [selectedWeekId, selectedRegionId, selectedDistrictId, selectedSubCountyId]);

  const statusesQuery = useListWeeklyStatusesQuery(weeklyStatusParams, {
    skip: !selectedWeekId,
  });
  const { data: weeklyStatusesResponse, isFetching: weeklyStatusesLoading } = statusesQuery;

  const detailedStatusesQuery = useListWeeklyStatusesDetailedQuery(weeklyStatusParams, {
    skip: !selectedWeekId,
  });
  const { data: weeklyStatusesDetailedResponse, isFetching: weeklyStatusesDetailedLoading } =
    detailedStatusesQuery;

  const metricsQuery = useListFacilityWeeklyMetricsByWeekQuery(selectedWeekId, {
    skip: !selectedWeekId,
  });
  const {
    currentData: facilityMetricsResponse,
    isFetching: facilitiesLoading,
    isError: facilitiesError,
  } = metricsQuery;

  const alertsQuery = useListAlertsQuery();
  const { data: alertsResponse, isFetching: alertsLoading } = alertsQuery;

  const weeklyStatuses = Array.isArray(weeklyStatusesResponse)
    ? weeklyStatusesResponse
    : EMPTY_LIST;

  const weeklyStatusesDetailed = Array.isArray(weeklyStatusesDetailedResponse)
    ? weeklyStatusesDetailedResponse
    : EMPTY_LIST;

  const facilityMetrics = Array.isArray(facilityMetricsResponse)
    ? facilityMetricsResponse
    : EMPTY_LIST;

  const alerts = Array.isArray(alertsResponse) ? alertsResponse : EMPTY_LIST;

  useEffect(() => {
    if (!selectedWeekId && weeks.length > 0) {
      setSelectedWeekId(weeks[0].id);
    }
  }, [weeks, selectedWeekId]);

  useEffect(() => {
    setSelectedDistrictId("");
    setSelectedSubCountyId("");
  }, [selectedRegionId]);

  useEffect(() => {
    setSelectedSubCountyId("");
  }, [selectedDistrictId]);

  const loading =
    weeksLoading ||
    diseasesLoading ||
    regionsReferenceLoading ||
    districtsReferenceLoading ||
    subcountiesReferenceLoading ||
    districtsGeoJsonLoading ||
    subcountiesGeoJsonLoading ||
    waterGeoJsonLoading ||
    weeklyStatusesLoading ||
    weeklyStatusesDetailedLoading ||
    facilitiesLoading ||
    alertsLoading;

  const selectedWeek = useMemo(
    () => weeks.find((week) => week.id === selectedWeekId),
    [weeks, selectedWeekId],
  );

  const epiWeekOptions: FilterOption[] = useMemo(
    () => [
      {
        value: "",
        label: "Select...",
      },
      ...weeks.map((week) => ({
        value: week.id,
        label:
          week.epi_year && week.epi_week
            ? `${week.epi_year} - Week ${week.epi_week}`
            : `Week ${week.epi_week ?? ""}`,
      })),
    ],
    [weeks],
  );

  const regionOptions: FilterOption[] = useMemo(
    () => [
      {
        value: "",
        label: "All regions",
      },
      ...regions.map((item) => ({
        value: item.id,
        label: item.name,
      })),
    ],
    [regions],
  );

  const districtOptions: FilterOption[] = useMemo(
    () => [
      {
        value: "",
        label: "All districts",
      },
      ...districts.map((item) => ({
        value: item.id,
        label: item.name,
      })),
    ],
    [districts],
  );

  const subCountyOptions: FilterOption[] = useMemo(
    () => [
      {
        value: "",
        label: "All sub-counties",
      },
      ...subcounties.map((item) => ({
        value: item.id,
        label: item.name,
      })),
    ],
    [subcounties],
  );

  const filteredWeeklyStatuses = useMemo(() => {
    return weeklyStatuses.filter((item) => {
      const matchesWeek = !selectedWeekId || item.epi_week_id === selectedWeekId;

      const matchesRegion = !selectedRegionId || item.region_id === selectedRegionId;

      const matchesDistrict = !selectedDistrictId || item.district_id === selectedDistrictId;

      const matchesSubCounty = !selectedSubCountyId || item.sub_county_id === selectedSubCountyId;

      return matchesWeek && matchesRegion && matchesDistrict && matchesSubCounty;
    });
  }, [weeklyStatuses, selectedWeekId, selectedRegionId, selectedDistrictId, selectedSubCountyId]);

  const filteredWeeklyStatusesDetailed = useMemo(() => {
    return weeklyStatusesDetailed.filter((item) => {
      const matchesWeek = !selectedWeekId || item.epi_week_id === selectedWeekId;

      const matchesRegion = !selectedRegionId || item.region_id === selectedRegionId;

      const matchesDistrict = !selectedDistrictId || item.district_id === selectedDistrictId;

      const matchesSubCounty = !selectedSubCountyId || item.sub_county_id === selectedSubCountyId;

      return matchesWeek && matchesRegion && matchesDistrict && matchesSubCounty;
    });
  }, [
    weeklyStatusesDetailed,
    selectedWeekId,
    selectedRegionId,
    selectedDistrictId,
    selectedSubCountyId,
  ]);

  const filteredFacilityMetrics = useMemo(() => {
    return facilityMetrics.filter((item) => {
      const matchesRegion =
        !selectedRegionId ||
        ("region_id" in item && String(item.region_id ?? "") === selectedRegionId);

      const matchesDistrict =
        !selectedDistrictId ||
        ("district_id" in item && String(item.district_id ?? "") === selectedDistrictId);

      const matchesSubCounty =
        !selectedSubCountyId ||
        ("sub_county_id" in item && String(item.sub_county_id ?? "") === selectedSubCountyId);

      return matchesRegion && matchesDistrict && matchesSubCounty;
    });
  }, [facilityMetrics, selectedRegionId, selectedDistrictId, selectedSubCountyId]);

  const filteredAlerts = useMemo(() => {
    const selectedDistrictName = districts.find(
      (district) => district.id === selectedDistrictId,
    )?.name;

    return alerts.filter((item) => {
      const matchesWeek = !selectedWeekId || item.epi_week_id === selectedWeekId;

      const matchesDistrict =
        !selectedDistrictId ||
        String(item.district_id ?? "") === selectedDistrictId ||
        normalize(item.district_name) === normalize(selectedDistrictName);

      return matchesWeek && matchesDistrict;
    });
  }, [alerts, selectedWeekId, selectedDistrictId, districts]);

  const diseaseNameById = useMemo(() => {
    return new Map(diseases.map((disease) => [disease.id, disease.name]));
  }, [diseases]);

  const handleOpenDisease = useCallback(
    (diseaseSlug: string) => {
      const params = new URLSearchParams();

      if (selectedWeekId) {
        params.set("weekId", selectedWeekId);
      }

      if (selectedRegionId) {
        params.set("regionId", selectedRegionId);
      }

      if (selectedDistrictId) {
        params.set("districtId", selectedDistrictId);
      }

      if (selectedSubCountyId) {
        params.set("subCountyId", selectedSubCountyId);
      }

      navigate({
        pathname: `/${encodeURIComponent(diseaseSlug)}`,
        search: params.toString() ? `?${params.toString()}` : "",
      });
    },
    [navigate, selectedWeekId, selectedRegionId, selectedDistrictId, selectedSubCountyId],
  );

  const createStatusTile = useCallback(
    (title: string, status: "MAROON" | "RED" | "YELLOW" | "GREEN") => {
      const names = Array.from(
        new Set(
          filteredWeeklyStatuses
            .filter((item) => item.status === status && item.disease_id)
            .map((item) => diseaseNameById.get(String(item.disease_id)))
            .filter((name): name is string => Boolean(name?.trim())),
        ),
      );

      return {
        title,
        total: names.length,
        items: names.slice(0, 6).map((name) => ({
          label: name,
          onClick: () => handleOpenDisease(slugifyDiseaseName(name)),
        })),
      };
    },
    [filteredWeeklyStatuses, diseaseNameById, handleOpenDisease],
  );

  const immediateActionTile = useMemo(
    () => createStatusTile("Immediate Action", "MAROON"),
    [createStatusTile],
  );

  const takeActionTile = useMemo(() => createStatusTile("Take Action", "RED"), [createStatusTile]);

  const alertTile = useMemo(() => createStatusTile("Alert", "YELLOW"), [createStatusTile]);

  const watchTile = useMemo(() => createStatusTile("Watch", "GREEN"), [createStatusTile]);

  const regionDistrictNames = useMemo(() => {
    return new Set(districts.map((district) => districtKey(district.name)).filter(Boolean));
  }, [districts]);

  const districtSubcountyNames = useMemo(() => {
    return new Set(subcounties.map((subcounty) => normalize(subcounty.name)).filter(Boolean));
  }, [subcounties]);

  const activeMap = useMemo(() => {
    if (selectedDistrictId) {
      const districtName = districtKey(
        districts.find((item) => item.id === selectedDistrictId)?.name,
      );
      const subcountyBoundaries = filterFeatureCollection(
        subcountyGeoJson,
        (feature) => getDistrictFeatureName(feature.properties) === districtName,
      );
      if (subcountyBoundaries.features.length) {
        return {
          geoJson: subcountyBoundaries,
          level: "subcounty" as const,
          fallback: false,
          unlinked: subcountyBoundaries.features.some(
            (feature) => !districtSubcountyNames.has(getSubcountyFeatureName(feature.properties)),
          ),
        };
      }
      return {
        geoJson: filterFeatureCollection(
          districtGeoJson,
          (feature) => getDistrictFeatureName(feature.properties) === districtName,
        ),
        level: "district" as const,
        fallback: true,
        unlinked: false,
      };
    }
    return {
      geoJson: selectedRegionId
        ? filterFeatureCollection(districtGeoJson, (feature) =>
            regionDistrictNames.has(getDistrictFeatureName(feature.properties)),
          )
        : districtGeoJson,
      level: "district" as const,
      fallback: false,
      unlinked: false,
    };
  }, [
    selectedDistrictId,
    selectedRegionId,
    districts,
    districtGeoJson,
    subcountyGeoJson,
    districtSubcountyNames,
    regionDistrictNames,
  ]);

  const activeMapLevel = activeMap.level;

  const activeWaterGeoJson = useMemo<GenericFeatureCollection>(() => {
    if (activeMapLevel === "subcounty") {
      return EMPTY_FEATURE_COLLECTION;
    }

    return waterGeoJson;
  }, [activeMapLevel, waterGeoJson]);

  const handleMapDistrictSelect = useCallback(
    (districtName: string) => {
      const match = districts.find((item) => districtKey(item.name) === districtKey(districtName));

      if (match) {
        setSelectedDistrictId(match.id);
      }
    },
    [districts],
  );

  const handleMapSubcountySelect = useCallback(
    (subcountyName: string) => {
      const match = subcounties.find((item) => normalize(item.name) === normalize(subcountyName));

      if (match) {
        setSelectedSubCountyId(match.id);
      }
    },
    [subcounties],
  );

  const unavailable = useSurveillanceRecovery([
    weeksQuery,
    diseasesQuery,
    regionsQuery,
    districtsQuery,
    subcountiesQuery,
    statusesQuery,
    detailedStatusesQuery,
    metricsQuery,
    alertsQuery,
  ]);

  if (unavailable) {
    return (
      <PageShell title="Surveillance Dashboard">
        <SurveillanceUnavailableNotice />
      </PageShell>
    );
  }

  return (
    <div className="surveillance-dashboard-page">
      <PageShell
        title="Surveillance Dashboard"
        subtitle="Monitor surveillance signals, reporting trends, alerts, and priority conditions across regions, districts, and sub-counties."
        actions={loading ? <InlineLoading description="Loading surveillance data..." /> : undefined}
      >
        <section className="surveillance-dashboard-page__section">
          <SurveillanceFilters
            epiWeek={selectedWeekId}
            region={selectedRegionId}
            district={selectedDistrictId}
            subCounty={selectedSubCountyId}
            epiWeekOptions={epiWeekOptions}
            regionOptions={regionOptions}
            districtOptions={districtOptions}
            subCountyOptions={subCountyOptions}
            onEpiWeekChange={setSelectedWeekId}
            onRegionChange={setSelectedRegionId}
            onDistrictChange={setSelectedDistrictId}
            onSubCountyChange={setSelectedSubCountyId}
          />
        </section>

        <section className="surveillance-dashboard-page__section">
          <SurveillanceTiles
            immediateAction={immediateActionTile}
            takeAction={takeActionTile}
            alert={alertTile}
            watch={watchTile}
            loading={weeksLoading || diseasesLoading || weeklyStatusesLoading}
          />
        </section>

        <section className="surveillance-dashboard-page__section">
          <SurveillancePanels
            selectedWeekId={selectedWeek?.id}
            selectedWeek={{
              id: selectedWeek?.id,
              year: selectedWeek?.epi_year,
              week: selectedWeek?.epi_week,
            }}
            regionId={selectedRegionId}
            districtId={selectedDistrictId}
            subCountyId={selectedSubCountyId}
            weeklyStatusesDetailed={filteredWeeklyStatusesDetailed}
            facilityMetrics={filteredFacilityMetrics}
            alerts={filteredAlerts}
            loading={loading}
            mapContent={
              <SurveillanceUgandaMap
                geoJson={activeMap.geoJson}
                boundaryNotice={
                  activeMap.fallback
                    ? "Sub-county boundaries are unavailable for this district. Showing the district boundary instead."
                    : activeMap.unlinked
                      ? "Some geographic sub-counties are not yet linked to reporting locations. Only matched sub-county status records are coloured."
                      : undefined
                }
                waterGeoJson={activeWaterGeoJson}
                mapLevel={activeMapLevel}
                weeklyStatuses={filteredWeeklyStatusesDetailed}
                facilityMetrics={filteredFacilityMetrics}
                metricsLoading={facilitiesLoading}
                metricsError={facilitiesError}
                selectedWeek={{
                  id: selectedWeek?.id,
                  year: selectedWeek?.epi_year,
                  week: selectedWeek?.epi_week,
                }}
                onDistrictSelect={handleMapDistrictSelect}
                onSubCountySelect={handleMapSubcountySelect}
              />
            }
          />
        </section>
      </PageShell>
    </div>
  );
}

export default function SurveillanceDashboardPage() {
  return (
    <PermissionGuard
      permission={PERMISSIONS.surveillanceRead}
      fallback={<SurveillanceAccessDenied />}
    >
      <SurveillanceDashboardContent />
    </PermissionGuard>
  );
}
