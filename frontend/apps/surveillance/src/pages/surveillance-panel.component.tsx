import React, { useMemo } from "react";
import type { Alert, FacilityWeeklyMetric, WeeklyStatusDetailed } from "../types";

type SurveillanceRankItem = {
  label: string;
  value: number | string;
};

interface SurveillancePanelsProps {
  selectedWeekId?: string;
  selectedWeek?: {
    id?: string;
    year?: number;
    week?: number;
  };
  regionId?: string;
  districtId?: string;
  subCountyId?: string;
  weeklyStatusesDetailed?: WeeklyStatusDetailed[];
  facilityMetrics?: FacilityWeeklyMetric[];
  alerts?: Alert[];
  loading?: boolean;
  mapContent?: React.ReactNode;
}

interface RankingPanelProps {
  title: string;
  items: SurveillanceRankItem[];
  loading?: boolean;
  emptyText?: string;
}

function RankingPanel({ title, items, loading, emptyText }: RankingPanelProps) {
  return (
    <section className="surveillance-panel surveillance-panel--side">
      <h3 className="surveillance-panel__title">{title}</h3>

      {loading ? (
        <p className="surveillance-panel__empty">Loading...</p>
      ) : items.length > 0 ? (
        <ol className="surveillance-panel__ranking-list">
          {items.map((item, index) => (
            <li
              key={`${title}-${item.label}-${index}`}
              className="surveillance-panel__ranking-item"
            >
              <span className="surveillance-panel__ranking-label">
                {item.label}: {item.value}
              </span>
            </li>
          ))}
        </ol>
      ) : (
        <p className="surveillance-panel__empty">{emptyText ?? "No data available."}</p>
      )}
    </section>
  );
}

function countStatuses(rows: WeeklyStatusDetailed[]) {
  return rows.reduce(
    (acc, item) => {
      switch (item.status) {
        case "MAROON":
          acc.maroon += 1;
          break;
        case "RED":
          acc.red += 1;
          break;
        case "YELLOW":
          acc.yellow += 1;
          break;
        case "GREEN":
          acc.green += 1;
          break;
        default:
          break;
      }
      return acc;
    },
    { maroon: 0, red: 0, yellow: 0, green: 0 },
  );
}

const FALLBACK_LEGEND_ITEMS = [
  { key: "maroon", tone: "immediate-action", label: "Immediate action" },
  { key: "red", tone: "take-action", label: "Take action" },
  { key: "yellow", tone: "alert", label: "Alert" },
  { key: "green", tone: "watch", label: "Watch" },
] as const;

function normalizeLabel(value: unknown) {
  return typeof value === "string" && value.trim() ? value.trim() : "";
}

function DefaultMapFallback({
  weeklyStatuses,
  selectedWeek,
}: {
  weeklyStatuses: WeeklyStatusDetailed[];
  selectedWeek?: { year?: number; week?: number };
}) {
  const summary = useMemo(() => countStatuses(weeklyStatuses), [weeklyStatuses]);

  return (
    <div className="surveillance-map">
      <div className="surveillance-map__toolbar">
        <button
          type="button"
          className="surveillance-map__tool-button"
          disabled
          aria-disabled="true"
          title="Map not available"
        >
          Reset
        </button>
      </div>

      <div className="surveillance-map__canvas surveillance-map__canvas--placeholder">
        <div className="surveillance-map__placeholder-shape">Uganda Map</div>
      </div>

      <ul
        className="surveillance-map__legend"
        aria-label="Map legend: disease status record counts"
      >
        {FALLBACK_LEGEND_ITEMS.map(({ key, tone, label }) => (
          <li key={key} className="surveillance-map__legend-item">
            <span
              className={`surveillance-map__legend-swatch surveillance-map__legend-swatch--${tone}`}
              aria-hidden="true"
            />
            {label} {summary[key] ? `(${summary[key]})` : ""}
          </li>
        ))}
      </ul>

      <div className="surveillance-map__meta">
        <p>
          {selectedWeek?.year && selectedWeek?.week
            ? `Week ${selectedWeek.week}, ${selectedWeek.year}`
            : "Current reporting week"}
        </p>
      </div>
    </div>
  );
}

export default function SurveillancePanels({
  selectedWeekId,
  selectedWeek,
  regionId = "",
  districtId = "",
  subCountyId = "",
  weeklyStatusesDetailed = [],
  facilityMetrics = [],
  alerts = [],
  loading = false,
  mapContent,
}: SurveillancePanelsProps) {
  const filteredWeeklyStatuses = useMemo(() => {
    return weeklyStatusesDetailed.filter((item) => {
      const matchesWeek = !selectedWeekId || item.epi_week_id === selectedWeekId;
      const matchesRegion = !regionId || item.region_id === regionId;
      const matchesDistrict = !districtId || item.district_id === districtId;
      const matchesSubCounty = !subCountyId || item.sub_county_id === subCountyId;

      return matchesWeek && matchesRegion && matchesDistrict && matchesSubCounty;
    });
  }, [weeklyStatusesDetailed, selectedWeekId, regionId, districtId, subCountyId]);

  const filteredFacilityMetrics = useMemo(() => {
    return facilityMetrics.filter((item) => {
      const matchesRegion =
        !regionId || ("region_id" in item && String(item.region_id ?? "") === regionId);

      const matchesDistrict =
        !districtId || ("district_id" in item && String(item.district_id ?? "") === districtId);

      const matchesSubCounty =
        !subCountyId ||
        ("sub_county_id" in item && String(item.sub_county_id ?? "") === subCountyId);

      return matchesRegion && matchesDistrict && matchesSubCounty;
    });
  }, [facilityMetrics, regionId, districtId, subCountyId]);

  const filteredAlerts = useMemo(() => {
    return alerts.filter((item) => {
      const matchesWeek = !selectedWeekId || item.epi_week_id === selectedWeekId;
      const matchesDistrict = !districtId || String(item.district_id ?? "") === districtId;

      return matchesWeek && matchesDistrict;
    });
  }, [alerts, selectedWeekId, districtId]);

  const facilitiesReporting = useMemo<SurveillanceRankItem[]>(() => {
    const grouped = new Map<string, number>();

    for (const item of filteredFacilityMetrics) {
      const label =
        normalizeLabel(item.facility_name) || normalizeLabel(item.district_name) || "Unknown";

      const current = grouped.get(label) ?? 0;
      grouped.set(label, current + Number(item.metric_value ?? 0));
    }

    return Array.from(grouped.entries())
      .map(([label, value]) => ({ label, value }))
      .sort((a, b) => Number(b.value) - Number(a.value))
      .slice(0, 10);
  }, [filteredFacilityMetrics]);

  const eidsrAlerts = useMemo<SurveillanceRankItem[]>(() => {
    const grouped = new Map<string, number>();

    for (const item of filteredAlerts) {
      const diseaseName = normalizeLabel(item.disease_name) || "Unknown disease";
      const current = grouped.get(diseaseName) ?? 0;
      grouped.set(diseaseName, current + 1);
    }

    return Array.from(grouped.entries())
      .map(([label, value]) => ({ label, value }))
      .sort((a, b) => Number(b.value) - Number(a.value))
      .slice(0, 10);
  }, [filteredAlerts]);

  return (
    <div className="surveillance-panels">
      <div className="surveillance-panels__grid">
        <RankingPanel
          title="Facilities Reporting"
          items={facilitiesReporting}
          loading={loading}
          emptyText="No facility reports for the selected week and location."
        />

        <section className="surveillance-panel surveillance-panel--map">
          {mapContent ? (
            mapContent
          ) : (
            <DefaultMapFallback
              weeklyStatuses={filteredWeeklyStatuses}
              selectedWeek={selectedWeek}
            />
          )}
        </section>

        <RankingPanel
          title="EIDSR Alerts"
          items={eidsrAlerts}
          loading={loading}
          emptyText="No EIDSR alerts for the selected week and location. Disease status flags are reported separately."
        />
      </div>
    </div>
  );
}
