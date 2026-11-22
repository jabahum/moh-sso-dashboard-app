import { useEffect, useMemo, useRef } from "react";
import { GeoJSON, MapContainer, useMap } from "react-leaflet";
import type { Feature, FeatureCollection, Geometry } from "geojson";
import type { Layer, PathOptions } from "leaflet";
import L from "leaflet";
import { Button, InlineLoading } from "@carbon/react";
import { ArrowRight, Reset } from "@carbon/react/icons";

export type DistrictMapMetric = {
  district: string;
  region: string;
  critical: number;
  open: number;
  resolved: number;
  total: number;
  facilitiesCount: number;
};

type UgandaDistrictFeatureProperties = {
  District?: string;
  Region?: string;
  regions?: string;
  district?: string;
  district_name?: string;
  region?: string;
  ADM1_NAME?: string;
  ADM2_NAME?: string;
  name?: string;
  [key: string]: unknown;
};

export interface IssueChoroplethMapProps {
  geoJson?: FeatureCollection;
  waterGeoJson?: FeatureCollection;
  isLoadingGeoJson?: boolean;
  districtMetricsMap: Map<string, DistrictMapMetric>;
  selectedFilter: string;
  chartViewMode: "total" | "status";
  onSelectFilter: (value: string) => void;
  onNavigateToDistrict: (districtName: string) => void;
}

const WATER_STYLE: PathOptions = {
  fillColor: "#a6c8ff",
  fillOpacity: 0.85,
  color: "#4589ff",
  weight: 0.8,
  opacity: 1,
  interactive: false,
};

export function normalizeDistrictKey(value: unknown): string {
  if (typeof value !== "string") return "";
  let s = value
    .trim()
    .toLowerCase()
    .replace(/\s+(district|local government|municipality|dl)$/i, "")
    .replace(/^district\s+of\s+/i, "")
    .replace(/[-_]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();

  if (s === "luwero") s = "luweero";
  if (s === "sembabule") s = "ssembabule";
  if (s === "kassanda") s = "kasanda";
  return s;
}

export function normalizeRegionKey(value: unknown): string {
  if (typeof value !== "string") return "";
  const s = value.trim().toLowerCase().replace(/\s+/g, " ");
  if (s === "north central") return "north buganda";
  if (s === "south central") return "south buganda";
  return s;
}

function getFeatureDistrictName(props?: UgandaDistrictFeatureProperties): string {
  return (
    props?.District ||
    props?.district_name ||
    props?.district ||
    props?.ADM2_NAME ||
    props?.name ||
    ""
  );
}

function getFeatureRegionName(props?: UgandaDistrictFeatureProperties): string {
  return props?.Region || props?.regions || props?.region || props?.ADM1_NAME || "";
}

function formatDistrictLabel(rawName: string): string {
  if (!rawName) return "Unknown District";
  return rawName
    .toLowerCase()
    .split(" ")
    .map((word) => (word ? word[0].toUpperCase() + word.slice(1) : ""))
    .join(" ");
}

function hasGeoJsonFeatures(geoJson?: FeatureCollection): geoJson is FeatureCollection {
  return Boolean(
    geoJson &&
    geoJson.type === "FeatureCollection" &&
    Array.isArray(geoJson.features) &&
    geoJson.features.length > 0,
  );
}

function getTotalChoroplethColor(total: number): string {
  if (total <= 0) return "#f4f4f4";
  if (total <= 10) return "#d0e2ff";
  if (total <= 50) return "#78a9ff";
  if (total <= 150) return "#4589ff";
  if (total <= 300) return "#0f62fe";
  return "#002d9c";
}

function getStatusChoroplethColor(metric?: DistrictMapMetric): string {
  if (!metric || metric.total <= 0) return "#f4f4f4";
  if (metric.critical > 0) return "#a2191f";
  if (metric.open > metric.resolved && metric.open > 0) return "#da1e28";
  if (metric.open > 0) return "#f1c21b";
  return "#198038";
}

function RefreshMapSize() {
  const map = useMap();

  useEffect(() => {
    const firstTimer = window.setTimeout(() => {
      map.invalidateSize();
    }, 0);
    const secondTimer = window.setTimeout(() => {
      map.invalidateSize();
    }, 180);

    return () => {
      window.clearTimeout(firstTimer);
      window.clearTimeout(secondTimer);
    };
  }, [map]);

  return null;
}

function FitMapToSelection({
  geoJson,
  selectedFilter,
  districtMetricsMap,
}: {
  geoJson: FeatureCollection;
  selectedFilter: string;
  districtMetricsMap: Map<string, DistrictMapMetric>;
}) {
  const map = useMap();

  useEffect(() => {
    const isAll =
      !selectedFilter ||
      selectedFilter === "All Districts" ||
      selectedFilter === "All Regions";

    if (isAll) {
      const fullLayer = L.geoJSON(geoJson);
      const fullBounds = fullLayer.getBounds();
      if (fullBounds.isValid()) {
        map.invalidateSize();
        const fitZoom = map.getBoundsZoom(fullBounds, false, L.point(12, 12));
        map.setView(fullBounds.getCenter(), Math.max(7, fitZoom), { animate: false });
      }
      return;
    }

    const targetDistrictKey = normalizeDistrictKey(selectedFilter);
    const targetRegionKey = normalizeRegionKey(selectedFilter);

    const matchingFeatures = geoJson.features.filter((feature) => {
      const props = feature.properties as UgandaDistrictFeatureProperties | undefined;
      const distName = getFeatureDistrictName(props);
      const distKey = normalizeDistrictKey(distName);
      const stripCityKey = normalizeDistrictKey(distName.replace(/\s+city$/i, ""));
      if (distKey === targetDistrictKey || (stripCityKey && stripCityKey === targetDistrictKey)) {
        return true;
      }

      const metric =
        districtMetricsMap.get(distKey) ||
        (stripCityKey ? districtMetricsMap.get(stripCityKey) : undefined);
      const featureRegKey = normalizeRegionKey(metric?.region || getFeatureRegionName(props));
      return Boolean(targetRegionKey && featureRegKey === targetRegionKey);
    });

    if (matchingFeatures.length > 0) {
      const subLayer = L.geoJSON({
        type: "FeatureCollection",
        features: matchingFeatures,
      } as FeatureCollection);
      const subBounds = subLayer.getBounds();
      if (subBounds.isValid()) {
        map.invalidateSize();
        map.fitBounds(subBounds, {
          animate: true,
          padding: [24, 24],
          maxZoom: matchingFeatures.length === 1 ? 10 : 9,
        });
        return;
      }
    }

    const fallbackLayer = L.geoJSON(geoJson);
    const fallbackBounds = fallbackLayer.getBounds();
    if (fallbackBounds.isValid()) {
      const fitZoom = map.getBoundsZoom(fallbackBounds, false, L.point(12, 12));
      map.setView(fallbackBounds.getCenter(), Math.max(7, fitZoom), { animate: false });
    }
  }, [geoJson, selectedFilter, districtMetricsMap, map]);

  return null;
}

export function IssueChoroplethMap({
  geoJson,
  waterGeoJson,
  isLoadingGeoJson = false,
  districtMetricsMap,
  selectedFilter,
  chartViewMode,
  onSelectFilter,
  onNavigateToDistrict,
}: IssueChoroplethMapProps) {
  const mapRef = useRef<L.Map | null>(null);
  const geoJsonRef = useRef<L.GeoJSON | null>(null);

  const isAllSelected =
    !selectedFilter ||
    selectedFilter === "All Districts" ||
    selectedFilter === "All Regions";

  const lookupMetric = useMemo(() => {
    return (rawDistrictName: string): DistrictMapMetric | undefined => {
      const key = normalizeDistrictKey(rawDistrictName);
      if (!key) return undefined;
      const direct = districtMetricsMap.get(key);
      if (direct) return direct;
      const withoutCity = normalizeDistrictKey(rawDistrictName.replace(/\s+city$/i, ""));
      if (withoutCity && withoutCity !== key) {
        return districtMetricsMap.get(withoutCity);
      }
      return undefined;
    };
  }, [districtMetricsMap]);

  const isFeatureSelected = (feature?: Feature<Geometry, UgandaDistrictFeatureProperties>): boolean => {
    if (isAllSelected || !feature?.properties) return true;
    const distName = getFeatureDistrictName(feature.properties);
    const distKey = normalizeDistrictKey(distName);
    const stripCityKey = normalizeDistrictKey(distName.replace(/\s+city$/i, ""));
    const targetDistKey = normalizeDistrictKey(selectedFilter);
    if (distKey === targetDistKey || (stripCityKey && stripCityKey === targetDistKey)) {
      return true;
    }
    const metric = lookupMetric(distName);
    const regKey = normalizeRegionKey(metric?.region || getFeatureRegionName(feature.properties));
    const targetRegKey = normalizeRegionKey(selectedFilter);
    return Boolean(targetRegKey && regKey === targetRegKey);
  };

  const mapSummaryStats = useMemo(() => {
    const allDistricts = Array.from(districtMetricsMap.values()).filter(
      (d) => d.district && d.district.toLowerCase() !== "unspecified" && d.total > 0,
    );
    allDistricts.sort((a, b) => b.total - a.total || b.critical - a.critical);

    const filteredDistricts = isAllSelected
      ? allDistricts
      : allDistricts.filter((d) => {
        const dKey = normalizeDistrictKey(d.district);
        const targetDKey = normalizeDistrictKey(selectedFilter);
        if (dKey === targetDKey) return true;
        return normalizeRegionKey(d.region) === normalizeRegionKey(selectedFilter);
      });


    return {
      affectedDistrictsCount: allDistricts.length,
      topDistricts: filteredDistricts.slice(0, 12),
      maxDistrictTotal: allDistricts[0]?.total ?? 0,
    };
  }, [districtMetricsMap, isAllSelected, selectedFilter]);

  const styleFeature = (
    feature?: Feature<Geometry, UgandaDistrictFeatureProperties>,
  ): PathOptions => {
    const distName = getFeatureDistrictName(feature?.properties);
    const metric = lookupMetric(distName);
    const selected = isFeatureSelected(feature);

    const fillColor =
      chartViewMode === "status"
        ? getStatusChoroplethColor(metric)
        : getTotalChoroplethColor(metric?.total ?? 0);

    return {
      fillColor,
      fillOpacity: selected ? 0.9 : 0.22,
      color: selected && !isAllSelected ? "#161616" : "#6f6f6f",
      weight: selected && !isAllSelected ? 1.8 : 0.8,
      opacity: 1,
    };
  };

  const onEachFeature = (
    feature: Feature<Geometry, UgandaDistrictFeatureProperties>,
    layer: Layer,
  ) => {
    const rawDistrict = getFeatureDistrictName(feature.properties);
    const displayDistrict = formatDistrictLabel(rawDistrict);
    const metric = lookupMetric(rawDistrict);
    const regionLabel = metric?.region || getFeatureRegionName(feature.properties) || "Uganda";

    layer.on({
      mouseover: () => {
        const pathLayer = layer as L.Path;
        pathLayer.setStyle({
          weight: 2,
          color: "#0f62fe",
          fillOpacity: 1,
        });
        pathLayer.bringToFront();
      },
      mouseout: () => {
        geoJsonRef.current?.resetStyle(layer);
      },
      click: () => {
        const clickedName = metric?.district || `${displayDistrict} District`;
        if (normalizeDistrictKey(selectedFilter) === normalizeDistrictKey(rawDistrict)) {
          onSelectFilter("All Districts");
        } else {
          onSelectFilter(clickedName);
        }
      },
    });

    const tooltip = document.createElement("div");
    tooltip.className = "issue-choropleth-tooltip";

    const heading = document.createElement("strong");
    heading.textContent = `${displayDistrict} (${regionLabel})`;
    tooltip.append(heading);

    const lines = [
      `Total Issues: ${(metric?.total ?? 0).toLocaleString()}`,
      `Critical (Open): ${(metric?.critical ?? 0).toLocaleString()}`,
      `Open Issues: ${(metric?.open ?? 0).toLocaleString()}`,
      `Resolved Issues: ${(metric?.resolved ?? 0).toLocaleString()}`,
      `Facilities Affected: ${(metric?.facilitiesCount ?? 0).toLocaleString()}`,
    ];

    for (const text of lines) {
      const row = document.createElement("div");
      row.textContent = text;
      tooltip.append(row);
    }

    layer.bindTooltip(tooltip, {
      sticky: true,
      direction: "top",
      opacity: 0.97,
      className: "issue-choropleth-leaflet-tooltip",
    });
  };

  const handleResetView = () => {
    onSelectFilter("All Districts");
    if (mapRef.current && hasGeoJsonFeatures(geoJson)) {
      const fullLayer = L.geoJSON(geoJson);
      const bounds = fullLayer.getBounds();
      if (bounds.isValid()) {
        mapRef.current.invalidateSize();
        const fitZoom = mapRef.current.getBoundsZoom(bounds, false, L.point(12, 12));
        mapRef.current.setView(bounds.getCenter(), Math.max(7, fitZoom), { animate: false });
      }
    }
  };

  const geoJsonKey = useMemo(
    () =>
      JSON.stringify([
        chartViewMode,
        selectedFilter,
        Array.from(districtMetricsMap.entries()).map(([k, v]) => [
          k,
          v.total,
          v.open,
          v.critical,
          v.resolved,
        ]),
      ]),
    [chartViewMode, selectedFilter, districtMetricsMap],
  );

  return (
    <div className="issue-choropleth-card">
      <div className="issue-choropleth-header">
        <div>
          <h4 className="issue-choropleth-title">
            {isAllSelected
              ? "Issues by District - All Districts"
              : `Issues by District -  ${selectedFilter}`}
          </h4>
          <p className="issue-choropleth-subtitle">
            Click any district on the map or ranking list to inspect or filter.
          </p>
        </div>

        {!isAllSelected && (
          <Button
            size="sm"
            kind="ghost"
            renderIcon={Reset}
            onClick={handleResetView}
          >
            All Districts
          </Button>
        )}
      </div>

      <div className="issue-choropleth-body">
        <div className="issue-choropleth-map-pane">
          {isLoadingGeoJson ? (
            <div className="issue-choropleth-loading">
              <InlineLoading status="active" description="Loading Uganda district boundaries..." />
            </div>
          ) : !hasGeoJsonFeatures(geoJson) ? (
            <div className="issue-choropleth-loading">
              <p>District boundary map is currently unavailable.</p>
            </div>
          ) : (
            <div className="issue-choropleth-canvas">
              <MapContainer
                ref={mapRef}
                center={[1.3733, 32.2903]}
                zoom={7}
                zoomControl={true}
                attributionControl={false}
                scrollWheelZoom={false}
                className="issue-choropleth-leaflet"
              >
                <RefreshMapSize />
                <FitMapToSelection
                  geoJson={geoJson}
                  selectedFilter={selectedFilter}
                  districtMetricsMap={districtMetricsMap}
                />

                {hasGeoJsonFeatures(waterGeoJson) && (
                  <GeoJSON
                    key={`water-${waterGeoJson.features.length}`}
                    data={waterGeoJson}
                    interactive={false}
                    style={() => WATER_STYLE}
                  />
                )}

                <GeoJSON
                  key={geoJsonKey}
                  ref={geoJsonRef}
                  data={geoJson}
                  style={styleFeature}
                  onEachFeature={onEachFeature}
                />
              </MapContainer>
            </div>
          )}

          {/* Legend */}
          <div className="issue-choropleth-footer">
            {chartViewMode === "total" ? (
              <div className="issue-choropleth-legend" aria-label="District total issues legend">
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--0" />
                  0 Issues
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--1" />
                  1–10
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--2" />
                  11–50
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--3" />
                  51–150
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--4" />
                  151–300
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--5" />
                  &gt; 300
                </span>
              </div>
            ) : (
              <div className="issue-choropleth-legend" aria-label="District issue status legend">
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--critical" />
                  Critical Open
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--open" />
                  Majority Open
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--mixed" />
                  In Progress
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--resolved" />
                  All Resolved
                </span>
                <span className="issue-choropleth-legend-item">
                  <span className="issue-choropleth-swatch issue-choropleth-swatch--0" />
                  No Issues
                </span>
              </div>
            )}

            <button
              type="button"
              className="issue-choropleth-reset-btn"
              onClick={handleResetView}
            >
              Reset Map View
            </button>
          </div>
        </div>

        {/* Side ranking list for quick district inspection */}
        <div className="issue-choropleth-sidebar">
          <div className="issue-choropleth-sidebar-header">
            <span className="issue-choropleth-sidebar-title">
              {isAllSelected ? "Top Affected Districts" : `Districts (${selectedFilter})`}
            </span>
            <span className="issue-choropleth-sidebar-count">
              {mapSummaryStats.topDistricts.length} shown
            </span>
          </div>

          {mapSummaryStats.topDistricts.length === 0 ? (
            <div className="issue-choropleth-sidebar-empty">
              No reported district issues for the current selection.
            </div>
          ) : (
            <div className="issue-choropleth-district-list">
              {mapSummaryStats.topDistricts.map((d, idx) => {
                const isSelected =
                  !isAllSelected &&
                  normalizeDistrictKey(selectedFilter) === normalizeDistrictKey(d.district);
                return (
                  <div
                    key={d.district}
                    className={`issue-choropleth-district-item ${isSelected ? "active" : ""}`}
                  >
                    <button
                      type="button"
                      className="issue-choropleth-district-select"
                      onClick={() =>
                        onSelectFilter(isSelected ? "All Districts" : d.district)
                      }
                    >
                      <div className="issue-choropleth-district-row">
                        <span className="issue-choropleth-district-rank">#{idx + 1}</span>
                        <span className="issue-choropleth-district-name">{d.district}</span>
                        <span className="issue-choropleth-district-total">{d.total}</span>
                      </div>
                      <div className="issue-choropleth-district-sub">
                        <span>{d.region}</span>
                        <span>•</span>
                        <span className="text-open">{d.open} open</span>
                        <span>•</span>
                        <span className="text-resolved">{d.resolved} resolved</span>
                        {/*{d.critical > 0 && (*/}
                        {/*  <>*/}
                        {/*    <span>•</span>*/}
                        {/*    <span className="text-critical">{d.critical} critical</span>*/}
                        {/*  </>*/}
                        {/*)}*/}
                      </div>
                    </button>
                    <button
                      type="button"
                      className="issue-choropleth-district-nav"
                      title={`View issues in ${d.district}`}
                      onClick={() => onNavigateToDistrict(d.district)}
                    >
                      <ArrowRight size={14} />
                    </button>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
