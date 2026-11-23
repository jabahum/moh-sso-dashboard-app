import { useEffect, useMemo, useRef } from "react";
import { GeoJSON, MapContainer, useMap } from "react-leaflet";
import type { Feature, GeoJSON as GeoJSONType, Geometry } from "geojson";
import type { Layer, PathOptions } from "leaflet";
import L from "leaflet";

import { districtKey } from "./surveillance-geography";
import { buildMapMetrics, subcountyMetricKey } from "./surveillance-map-metrics";

import type { FacilityWeeklyMetric, WeeklyStatus, WeeklyStatusDetailed } from "../types";

type StatusKey = "maroon" | "red" | "yellow" | "green" | "default";

type MapLevel = "district" | "subcounty";

type UgandaMapFeatureProperties = {
  District?: string;
  Region?: string;
  regions?: string;
  Subcounty?: string;
  sname2019?: string;
  district?: string;
  district_name?: string;
  region?: string;
  subregion?: string;
  subcounty?: string;
  sub_county?: string;
  subcounty_name?: string;
  name?: string;
  ADM1_NAME?: string;
  ADM2_NAME?: string;
  [key: string]: unknown;
};

interface SurveillanceUgandaMapProps {
  geoJson: GeoJSONType;
  waterGeoJson?: GeoJSONType;
  weeklyStatuses?: WeeklyStatusDetailed[];
  facilityMetrics?: FacilityWeeklyMetric[];
  metricsLoading?: boolean;
  metricsError?: boolean;
  selectedWeek?: {
    id?: string;
    year?: number;
    week?: number;
  };
  title?: string;
  boundaryNotice?: string;
  mapLevel?: MapLevel;
  onDistrictSelect?: (districtName: string) => void;
  onSubCountySelect?: (subcountyName: string) => void;
}

const STATUS_COLORS: Record<StatusKey, string> = {
  maroon: "var(--surveillance-status-maroon)",
  red: "var(--surveillance-status-red)",
  yellow: "var(--surveillance-status-yellow)",
  green: "var(--surveillance-status-green)",
  default: "var(--surveillance-status-default)",
};

/** Legend entries in the same order and wording as the severity tiles above the map. */
const LEGEND_ITEMS: { key: Exclude<StatusKey, "default">; label: string }[] = [
  { key: "maroon", label: "Immediate action" },
  { key: "red", label: "Take action" },
  { key: "yellow", label: "Alert" },
  { key: "green", label: "Watch" },
];

const STATUS_PRIORITY: Record<StatusKey, number> = {
  default: 0,
  green: 1,
  yellow: 2,
  red: 3,
  maroon: 4,
};

function recordStatus(map: Map<string, StatusKey>, name: string, status: StatusKey) {
  if (STATUS_PRIORITY[status] > STATUS_PRIORITY[map.get(name) ?? "default"]) map.set(name, status);
}

const WATER_STYLE: PathOptions = {
  fillColor: "#a6c8ff",
  fillOpacity: 0.85,
  color: "#4589ff",
  weight: 0.8,
  opacity: 1,
  interactive: false,
};

function normalize(value: unknown): string {
  if (typeof value !== "string") {
    return "";
  }

  return value.trim().toLowerCase();
}

function hasGeoJsonFeatures(geoJson?: GeoJSONType): boolean {
  if (!geoJson || geoJson.type !== "FeatureCollection") {
    return false;
  }

  return Array.isArray(geoJson.features) && geoJson.features.length > 0;
}

function mapRiskLevelToStatusKey(status?: WeeklyStatus["status"]): StatusKey {
  switch (status) {
    case "MAROON":
      return "maroon";
    case "RED":
      return "red";
    case "YELLOW":
      return "yellow";
    case "GREEN":
      return "green";
    default:
      return "default";
  }
}

function getFeatureDistrictName(properties?: UgandaMapFeatureProperties): string {
  return (
    properties?.District ||
    properties?.district_name ||
    properties?.district ||
    properties?.ADM2_NAME ||
    properties?.name ||
    ""
  );
}

function getFeatureRegionName(properties?: UgandaMapFeatureProperties): string {
  return (
    properties?.Region || properties?.regions || properties?.region || properties?.ADM1_NAME || ""
  );
}

function getFeatureSubCountyName(properties?: UgandaMapFeatureProperties): string {
  return (
    properties?.Subcounty ||
    properties?.sname2019 ||
    properties?.subcounty_name ||
    properties?.subcounty ||
    properties?.sub_county ||
    properties?.name ||
    ""
  );
}

function RefreshMapSize() {
  const map = useMap();

  useEffect(() => {
    const firstTimer = window.setTimeout(() => {
      map.invalidateSize();
    }, 0);

    const secondTimer = window.setTimeout(() => {
      map.invalidateSize();
    }, 150);

    return () => {
      window.clearTimeout(firstTimer);
      window.clearTimeout(secondTimer);
    };
  }, [map]);

  return null;
}

function FitMapToGeoJson({ geoJson }: { geoJson: GeoJSONType }) {
  const map = useMap();

  useEffect(() => {
    const layer = L.geoJSON(geoJson);
    const bounds = layer.getBounds();

    if (bounds.isValid()) {
      map.invalidateSize();
      map.fitBounds(bounds, {
        animate: false,
        padding: [24, 24],
      });

      map.invalidateSize();
    }
  }, [geoJson, map]);

  return null;
}

export default function SurveillanceUgandaMap({
  geoJson,
  waterGeoJson,
  weeklyStatuses = [],
  facilityMetrics = [],
  metricsLoading = false,
  metricsError = false,
  selectedWeek,
  title,
  boundaryNotice,
  mapLevel = "district",
  onDistrictSelect,
  onSubCountySelect,
}: SurveillanceUgandaMapProps) {
  const mapRef = useRef<L.Map | null>(null);
  const geoJsonRef = useRef<L.GeoJSON | null>(null);
  const areaMetrics = useMemo(
    () => buildMapMetrics(facilityMetrics, selectedWeek?.id),
    [facilityMetrics, selectedWeek?.id],
  );

  const summary = useMemo(() => {
    return weeklyStatuses.reduce(
      (accumulator, item) => {
        const status = mapRiskLevelToStatusKey(item.status);

        if (status === "maroon") {
          accumulator.maroon += 1;
        }

        if (status === "red") {
          accumulator.red += 1;
        }

        if (status === "yellow") {
          accumulator.yellow += 1;
        }

        if (status === "green") {
          accumulator.green += 1;
        }

        return accumulator;
      },
      {
        maroon: 0,
        red: 0,
        yellow: 0,
        green: 0,
      },
    );
  }, [weeklyStatuses]);

  const districtStatusMap = useMemo(() => {
    const statusMap = new Map<string, StatusKey>();

    for (const item of weeklyStatuses) {
      const districtName = districtKey(item.district_name);

      if (!districtName) {
        continue;
      }

      recordStatus(statusMap, districtName, mapRiskLevelToStatusKey(item.status));
    }

    return statusMap;
  }, [weeklyStatuses]);

  const subCountyStatusMap = useMemo(() => {
    const statusMap = new Map<string, StatusKey>();

    for (const item of weeklyStatuses) {
      const subCountyName = normalize(item.sub_county_name);

      if (!subCountyName) {
        continue;
      }

      recordStatus(statusMap, subCountyName, mapRiskLevelToStatusKey(item.status));
    }

    return statusMap;
  }, [weeklyStatuses]);

  const regionStatusMap = useMemo(() => {
    const statusMap = new Map<string, StatusKey>();

    for (const item of weeklyStatuses) {
      const regionName = normalize(item.region_name);

      if (!regionName) {
        continue;
      }

      recordStatus(statusMap, regionName, mapRiskLevelToStatusKey(item.status));
    }

    return statusMap;
  }, [weeklyStatuses]);

  const getFeatureStatus = (feature?: Feature<Geometry, UgandaMapFeatureProperties>): StatusKey => {
    if (!feature?.properties) {
      return "default";
    }

    const districtName = districtKey(getFeatureDistrictName(feature.properties));

    const regionName = normalize(getFeatureRegionName(feature.properties));

    const subCountyName = normalize(getFeatureSubCountyName(feature.properties));

    if (mapLevel === "subcounty") {
      if (subCountyName && subCountyStatusMap.has(subCountyName)) {
        return subCountyStatusMap.get(subCountyName) ?? "default";
      }

      return "default";
    }

    if (districtName && districtStatusMap.has(districtName)) {
      return districtStatusMap.get(districtName) ?? "default";
    }

    if (regionName && regionStatusMap.has(regionName)) {
      return regionStatusMap.get(regionName) ?? "default";
    }

    return "default";
  };

  const getFeatureLabel = (feature?: Feature<Geometry, UgandaMapFeatureProperties>): string => {
    if (!feature?.properties) {
      return "Unknown area";
    }

    if (mapLevel === "subcounty") {
      return (
        getFeatureSubCountyName(feature.properties) ||
        getFeatureDistrictName(feature.properties) ||
        "Unknown area"
      );
    }

    return (
      getFeatureDistrictName(feature.properties) ||
      getFeatureRegionName(feature.properties) ||
      "Unknown area"
    );
  };

  const styleFeature = (feature?: Feature<Geometry, UgandaMapFeatureProperties>): PathOptions => {
    const status = getFeatureStatus(feature);

    return {
      fillColor: STATUS_COLORS[status],
      fillOpacity: 1,
      color: "#6f6f6f",
      weight: 0.8,
      opacity: 1,
    };
  };

  const highlightFeature = (layer: Layer) => {
    const pathLayer = layer as L.Path;

    pathLayer.setStyle({
      weight: 1.5,
      color: "#1f1f1f",
      fillOpacity: 1,
    });

    pathLayer.bringToFront();
  };

  const resetHighlight = (layer: Layer) => {
    geoJsonRef.current?.resetStyle(layer);
  };

  const handleReset = () => {
    if (!mapRef.current) {
      return;
    }

    const layer = L.geoJSON(geoJson);
    const bounds = layer.getBounds();

    if (bounds.isValid()) {
      mapRef.current.fitBounds(bounds, {
        animate: false,
        padding: [24, 24],
      });

      mapRef.current.invalidateSize();
    }
  };

  const onEachFeature = (feature: Feature<Geometry, UgandaMapFeatureProperties>, layer: Layer) => {
    const label = getFeatureLabel(feature);
    const status = getFeatureStatus(feature);

    layer.on({
      mouseover: () => {
        highlightFeature(layer);
      },

      mouseout: () => {
        resetHighlight(layer);
      },

      click: () => {
        const boundsLayer = layer as L.FeatureGroup;

        if (mapRef.current && typeof boundsLayer.getBounds === "function") {
          const bounds = boundsLayer.getBounds();

          if (bounds.isValid()) {
            mapRef.current.fitBounds(bounds, {
              padding: [20, 20],
              maxZoom: mapLevel === "subcounty" ? 10 : 9,
            });
          }
        }

        if (mapLevel === "subcounty") {
          const subCountyName = getFeatureSubCountyName(feature.properties);

          if (subCountyName) {
            onSubCountySelect?.(subCountyName);
          }

          return;
        }

        const districtName = getFeatureDistrictName(feature.properties);

        if (districtName) {
          onDistrictSelect?.(districtName);
        }
      },
    });

    const district = getFeatureDistrictName(feature.properties);
    const metrics =
      mapLevel === "subcounty"
        ? areaMetrics.subcounties.get(
            subcountyMetricKey(district, getFeatureSubCountyName(feature.properties)),
          )
        : areaMetrics.districts.get(districtKey(district));
    const unavailable = metricsLoading ? "Loading…" : metricsError ? "Unavailable" : "No data";
    const tooltip = document.createElement("div");
    tooltip.className = "surveillance-map__tooltip";
    const heading = document.createElement("strong");
    heading.textContent = label;
    tooltip.append(heading);
    for (const text of [
      `Status: ${status === "default" ? "No data" : status.toUpperCase()}`,
      `Reported cases: ${!metricsLoading && !metricsError && metrics ? metrics.cases.toLocaleString() : unavailable}`,
      `Reporting facilities: ${!metricsLoading && !metricsError && metrics ? metrics.facilities.size.toLocaleString() : unavailable}`,
    ]) {
      const line = document.createElement("div");
      line.textContent = text;
      tooltip.append(line);
    }
    layer.bindTooltip(tooltip, {
      sticky: true,
      direction: "top",
      opacity: 1,
      className: "surveillance-map__tooltip",
    });
  };

  const displayTitle =
    title ||
    (selectedWeek?.year
      ? `Week ${selectedWeek.week ?? ""} ${selectedWeek.year}`.trim()
      : "Uganda surveillance map");

  const waterFeatureCount =
    waterGeoJson?.type === "FeatureCollection"
      ? waterGeoJson.features.length
      : waterGeoJson
        ? 1
        : 0;

  // GeoJSON data and onEachFeature are immutable in React Leaflet. Remount
  // when the selected geometry or risk records change, even at equal feature counts.
  const geoJsonKey = useMemo(
    () =>
      JSON.stringify([
        mapLevel,
        geoJson.type === "FeatureCollection"
          ? geoJson.features.map((feature) => feature.properties)
          : geoJson,
        weeklyStatuses,
        selectedWeek,
        metricsLoading,
        metricsError,
        ...[areaMetrics.districts, areaMetrics.subcounties].map((areas) =>
          Array.from(areas, ([name, summary]) => [name, summary.cases, summary.facilities.size]),
        ),
      ]),
    [mapLevel, geoJson, weeklyStatuses, selectedWeek, metricsLoading, metricsError, areaMetrics],
  );

  const waterGeoJsonKey = `water-${waterFeatureCount}`;

  const showWaterLayer = mapLevel === "district" && hasGeoJsonFeatures(waterGeoJson);

  return (
    <div className="surveillance-map">
      <div className="surveillance-map__header">
        <h3>{displayTitle}</h3>
        <p>
          {mapLevel === "subcounty"
            ? "Colours show sub-county status records. District-level flags remain in the tiles."
            : "Colours show the highest reported disease status for each area."}
        </p>
      </div>

      {boundaryNotice ? (
        <p className="surveillance-map__notice" role="status">
          {boundaryNotice}
        </p>
      ) : null}
      {!hasGeoJsonFeatures(geoJson) ? (
        <p className="surveillance-map__notice" role="status">
          No map boundaries are available for the selected location.
        </p>
      ) : null}
      <div className="surveillance-map__canvas" hidden={!hasGeoJsonFeatures(geoJson)}>
        <MapContainer
          ref={mapRef}
          center={[1.3733, 32.2903]}
          zoom={7}
          zoomControl={false}
          attributionControl={false}
          dragging={false}
          doubleClickZoom={false}
          boxZoom={false}
          keyboard={false}
          scrollWheelZoom={false}
          className="surveillance-map__leaflet"
        >
          <RefreshMapSize />

          <FitMapToGeoJson geoJson={geoJson} />

          {showWaterLayer && waterGeoJson ? (
            <GeoJSON
              key={waterGeoJsonKey}
              data={waterGeoJson}
              interactive={false}
              style={() => WATER_STYLE}
            />
          ) : null}

          <GeoJSON
            key={geoJsonKey}
            ref={geoJsonRef}
            data={geoJson}
            style={styleFeature}
            onEachFeature={onEachFeature}
          />
        </MapContainer>
      </div>

      {mapLevel === "subcounty" ? (
        <p className="surveillance-map__source">
          Sub-county boundaries: processed{" "}
          <a href="https://statistics.ubos.org/nphc/map" target="_blank" rel="noopener noreferrer">
            UBOS NPHC 2024 data
          </a>
          .
        </p>
      ) : null}
      <div className="surveillance-map__footer">
        <ul
          className="surveillance-map__legend"
          aria-label="Map legend: disease status record counts"
        >
          {LEGEND_ITEMS.map(({ key, label }) => (
            <li key={key} className="surveillance-map__legend-item">
              <span
                className={`surveillance-map__legend-swatch surveillance-map__legend-swatch--${key}`}
                aria-hidden="true"
              />
              {label} {summary[key] ? `(${summary[key]})` : ""}
            </li>
          ))}
          <li className="surveillance-map__legend-item">
            <span
              className="surveillance-map__legend-swatch surveillance-map__legend-swatch--no-data"
              aria-hidden="true"
            />
            No status data
          </li>
        </ul>

        <div className="surveillance-map__actions">
          <button type="button" className="surveillance-map__tool-button" onClick={handleReset}>
            Reset view
          </button>
        </div>
      </div>
    </div>
  );
}
