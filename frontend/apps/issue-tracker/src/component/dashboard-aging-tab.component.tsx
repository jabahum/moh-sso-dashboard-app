import { Fragment, useMemo, useState } from "react";
import {
  Button,
  DataTable,
  Search,
  Table,
  TableBody,
  TableCell,
  TableExpandHeader,
  TableExpandedRow,
  TableExpandRow,
  TableHead,
  TableHeader,
  TableRow,
} from "@carbon/react";
import {
  ArrowRight,
  ChevronDown,
  ChevronRight,
  Download,
} from "@carbon/react/icons";
import { AreaChart, LineChart } from "@carbon/charts-react";
import { ScaleTypes } from "@carbon/charts";

import type {
  ActionableAgingIssue,
  AgingBucketId,
  AgingDistrictSummary,
} from "../types";
import {
  AGING_BUCKETS,
  aggregateAgingIssuesByRegion,
  computeAgingMetrics,
  exportAgingSummaryToExcel,
  safeLower,
} from "../lib/issueAggregation";

export interface DashboardAgingTabProps {
  unresolvedAgingIssues: ActionableAgingIssue[];
  chartViewMode: "total" | "status";
  activeLocationFilter: string;
  defaultLocationLabel: string;
  chartToolbarConfig: Record<string, unknown>;
  hasActiveFilters: boolean;
  onResetFilters: () => void;
  onNavigateToRegion: (regionName: string) => void;
  onNavigateToDistrict: (districtName: string) => void;
  onNavigateToFacility: (facilityName: string) => void;
}

type AgingSliceFilter = "all" | "unassigned";

type SortHeader =
  | "region"
  | "0-7"
  | "8-14"
  | "15-30"
  | "31-60"
  | "61-90"
  | "90+"
  | "avgAgeDays"
  | "total"
  | string;

const AGING_SUMMARY_HEADERS = [
  { key: "region", header: "Region" },
  { key: "0-7", header: "0–7 Days" },
  { key: "8-14", header: "8–14 Days" },
  { key: "15-30", header: "15–30 Days" },
  { key: "31-60", header: "31–60 Days" },
  { key: "61-90", header: "61–90 Days" },
  { key: "90+", header: "> 90 Days" },
  { key: "avgAgeDays", header: "Avg Days" },
  { key: "total", header: "Total Unresolved" },
];

const BUCKET_IDS: AgingBucketId[] = [
  "0-7",
  "8-14",
  "15-30",
  "31-60",
  "61-90",
  "90+",
];

function renderDaysCount(bucketId: AgingBucketId, count: number) {
  if (count <= 0) {
    return <span className="severity-pill severity-pill--zero">-</span>;
  }
  const bucketDef = AGING_BUCKETS.find((b) => b.id === bucketId);
  return (
    <span className={`aging-pill ${bucketDef?.badgeClass ?? ""}`}>{count}</span>
  );
}

export const DashboardAgingTab = ({
  unresolvedAgingIssues,
  chartViewMode,
  activeLocationFilter,
  defaultLocationLabel,
  chartToolbarConfig,
  hasActiveFilters,
  onResetFilters,
  onNavigateToRegion,
  onNavigateToDistrict,
  onNavigateToFacility,
}: DashboardAgingTabProps) => {
  const [agingSliceFilter, setAgingSliceFilter] =
    useState<AgingSliceFilter>("all");
  const [regionSearchTerm, setRegionSearchTerm] = useState<string>("");
  const [sortHeader, setSortHeader] = useState<SortHeader>("total");
  const [sortDirection, setSortDirection] = useState<"ASC" | "DESC">("DESC");
  const [expandedDistricts, setExpandedDistricts] = useState<
    Record<string, boolean>
  >({});

  const overallMetrics = useMemo(
    () => computeAgingMetrics(unresolvedAgingIssues),
    [unresolvedAgingIssues],
  );

  const slicedAgingIssues = useMemo(() => {
    if (agingSliceFilter === "unassigned") {
      return unresolvedAgingIssues.filter((item) => item.isUnassigned);
    }
    return unresolvedAgingIssues;
  }, [unresolvedAgingIssues, agingSliceFilter]);

  const slicedAgingMetrics = useMemo(
    () => computeAgingMetrics(slicedAgingIssues),
    [slicedAgingIssues],
  );

  const { regionalAgingSummary, grandTotals: agingGrandTotals } = useMemo(
    () => aggregateAgingIssuesByRegion(slicedAgingIssues),
    [slicedAgingIssues],
  );

  const agingChartData = useMemo(() => {
    if (chartViewMode === "status") {
      return AGING_BUCKETS.flatMap((bucket) => {
        const stats = slicedAgingMetrics.bucketMap.get(bucket.id) ?? {
          total: 0,
          critical: 0,
          unassigned: 0,
          assigned: 0,
        };
        return [
          { group: "Critical Open", key: bucket.label, value: stats.critical },
          {
            group: "Unassigned Open",
            key: bucket.label,
            value: stats.unassigned,
          },
          { group: "Assigned Open", key: bucket.label, value: stats.assigned },
        ];
      });
    }

    return AGING_BUCKETS.flatMap((bucket) => {
      const stats = slicedAgingMetrics.bucketMap.get(bucket.id);
      return [
        {
          group: "Total Unresolved",
          key: bucket.label,
          value: stats?.total ?? 0,
        },
        {
          group: "Critical Open",
          key: bucket.label,
          value: stats?.critical ?? 0,
        },
      ];
    });
  }, [slicedAgingMetrics, chartViewMode]);

  const sliceTitleSuffix =
    agingSliceFilter === "unassigned" ? " — Unassigned Only" : "";

  const agingChartOptions = useMemo(
    () => ({
      title:
        activeLocationFilter && activeLocationFilter !== defaultLocationLabel
          ? `Unresolved Issue Aging Series (${activeLocationFilter})${sliceTitleSuffix}`
          : `Unresolved Issue Aging Series${sliceTitleSuffix}`,
      axes: {
        left: {
          mapsTo: "value",
          title: "Unresolved Issues",
          scaleType: ScaleTypes.LINEAR,
        },
        bottom: {
          mapsTo: "key",
          title: "Days",
          scaleType: ScaleTypes.LABELS,
        },
      },
      curve: "curveMonotoneX",
      points: {
        enabled: true,
        radius: 5,
        filled: true,
      },
      color: {
        scale:
          chartViewMode === "status"
            ? {
                "Critical Open": "#a2191f",
                "Unassigned Open": "#eb6200",
                "Assigned Open": "#0f62fe",
              }
            : {
                "Total Unresolved": "#0f62fe",
                "Critical Open": "#a2191f",
              },
      },
      height: "360px",
      toolbar: chartToolbarConfig,
      legend: {
        enabled: true,
      },
    }),
    [
      activeLocationFilter,
      defaultLocationLabel,
      sliceTitleSuffix,
      chartViewMode,
      chartToolbarConfig,
    ],
  );

  const handleSort = (headerKey: string) => {
    if (sortHeader === headerKey) {
      setSortDirection((prev) => (prev === "ASC" ? "DESC" : "ASC"));
    } else {
      setSortHeader(headerKey);
      setSortDirection("DESC");
    }
  };

  const isDistrictExpanded = (region: string, d: AgingDistrictSummary) => {
    const districtKey = `${region}-${d.district}`;
    if (expandedDistricts[districtKey] !== undefined) {
      return expandedDistricts[districtKey];
    }
    if (regionSearchTerm.trim()) {
      const term = safeLower(regionSearchTerm);
      return (
        safeLower(d.district).includes(term) ||
        d.facilities.some((f) => safeLower(f.facility).includes(term))
      );
    }
    return false;
  };

  const toggleDistrict = (region: string, d: AgingDistrictSummary) => {
    const districtKey = `${region}-${d.district}`;
    const currentlyExpanded = isDistrictExpanded(region, d);
    setExpandedDistricts((prev) => ({
      ...prev,
      [districtKey]: !currentlyExpanded,
    }));
  };

  const isEveryDistrictExpanded = (
    region: string,
    districts: AgingDistrictSummary[],
  ) => {
    if (districts.length === 0) return false;
    return districts.every((d) => isDistrictExpanded(region, d));
  };

  const toggleAllDistrictsInRegion = (
    region: string,
    districts: AgingDistrictSummary[],
  ) => {
    const allExpanded = isEveryDistrictExpanded(region, districts);
    setExpandedDistricts((prev) => {
      const next = { ...prev };
      for (const d of districts) {
        next[`${region}-${d.district}`] = !allExpanded;
      }
      return next;
    });
  };

  const displayedRows = useMemo(() => {
    let result = [...regionalAgingSummary];

    if (regionSearchTerm.trim()) {
      const term = safeLower(regionSearchTerm);
      result = result.filter(
        (r) =>
          safeLower(r.region).includes(term) ||
          r.districts.some(
            (d) =>
              safeLower(d.district).includes(term) ||
              d.facilities.some((f) => safeLower(f.facility).includes(term)),
          ),
      );
    }

    result.sort((a, b) => {
      const aVal = a[sortHeader];
      const bVal = b[sortHeader];

      if (typeof aVal === "string") {
        const aStr = aVal.toLowerCase();
        const bStr = String(bVal ?? "").toLowerCase();
        return sortDirection === "ASC"
          ? aStr.localeCompare(bStr)
          : bStr.localeCompare(aStr);
      }

      const aNum = Number(aVal ?? 0);
      const bNum = Number(bVal ?? 0);
      return sortDirection === "ASC" ? aNum - bNum : bNum - aNum;
    });

    return result;
  }, [regionalAgingSummary, regionSearchTerm, sortHeader, sortDirection]);

  const tableGrandTotals = useMemo(() => {
    if (!regionSearchTerm.trim()) {
      return agingGrandTotals;
    }

    let weightedAgeSum = 0;
    let maxAge = 0;

    const acc = displayedRows.reduce(
      (sum, row) => {
        weightedAgeSum += row.avgAgeDays * row.total;
        if (row.maxAgeDays > maxAge) maxAge = row.maxAgeDays;
        return {
          "0-7": sum["0-7"] + row["0-7"],
          "8-14": sum["8-14"] + row["8-14"],
          "15-30": sum["15-30"] + row["15-30"],
          "31-60": sum["31-60"] + row["31-60"],
          "61-90": sum["61-90"] + row["61-90"],
          "90+": sum["90+"] + row["90+"],
          overdue: sum.overdue + row.overdue,
          critical: sum.critical + row.critical,
          unassigned: sum.unassigned + row.unassigned,
          total: sum.total + row.total,
        };
      },
      {
        "0-7": 0,
        "8-14": 0,
        "15-30": 0,
        "31-60": 0,
        "61-90": 0,
        "90+": 0,
        overdue: 0,
        critical: 0,
        unassigned: 0,
        total: 0,
      },
    );

    return {
      ...acc,
      avgAgeDays: acc.total > 0 ? Math.round(weightedAgeSum / acc.total) : 0,
      maxAgeDays: maxAge,
      activeRegionsCount: displayedRows.length,
    };
  }, [regionSearchTerm, agingGrandTotals, displayedRows]);

  const overduePct =
    slicedAgingMetrics.totalUnresolved > 0
      ? Math.round(
          (slicedAgingMetrics.overdueCount /
            slicedAgingMetrics.totalUnresolved) *
            100,
        )
      : 0;

  return (
    <>
      {/* ================= Full-Width Aging Series Chart ================= */}
      <div className="dashboard-chart-card dashboard-chart-card--full">
        {chartViewMode === "status" ? (
          <LineChart data={agingChartData} options={agingChartOptions} />
        ) : (
          <AreaChart data={agingChartData} options={agingChartOptions} />
        )}
      </div>

      {/* ================= Clean Toolbar & Quick Slice Toggles ================= */}
      <div className="aging-analytics-toolbar">
        <div
          className="aging-triage-filters"
          role="group"
          aria-label="Filter aging analytics by backlog slice"
        >
          <button
            type="button"
            className={`aging-triage-btn ${
              agingSliceFilter === "all" ? "active" : ""
            }`}
            onClick={() => setAgingSliceFilter("all")}
          >
            All Unresolved ({overallMetrics.totalUnresolved})
          </button>
          <button
            type="button"
            className={`aging-triage-btn ${
              agingSliceFilter === "unassigned" ? "active" : ""
            }`}
            onClick={() =>
              setAgingSliceFilter((prev) =>
                prev === "unassigned" ? "all" : "unassigned",
              )
            }
          >
            Unassigned Only ({overallMetrics.unassignedCount})
          </button>
        </div>

        <div className="aging-triage-meta">
          <span className="aging-summary-indicator aging-summary-indicator--overdue">
            Overdue (&gt;30 Days):{" "}
            <strong>
              {slicedAgingMetrics.overdueCount} ({overduePct}%)
            </strong>
          </span>
          <span className="aging-summary-indicator">
            Avg / Oldest Days:{" "}
            <strong>
              {slicedAgingMetrics.avgAgeDays}d / {slicedAgingMetrics.maxAgeDays}
              d
            </strong>
          </span>
        </div>
      </div>

      {/* ================= Search & Export Bar ================= */}
      <div className="dashboard-toolbar-row">
        <div className="dashboard-search">
          <Search
            labelText="Search regions, districts, or facilities in aging breakdown"
            placeholder="Search by region, district, or facility name..."
            value={regionSearchTerm}
            onChange={(e) => setRegionSearchTerm(e.target.value)}
            size="sm"
            closeButtonLabelText="Clear search"
          />
        </div>

        <Button
          size="sm"
          kind="secondary"
          renderIcon={Download}
          disabled={displayedRows.length === 0}
          onClick={() =>
            exportAgingSummaryToExcel(displayedRows, tableGrandTotals)
          }
        >
          Download (.xlsx)
        </Button>
      </div>

      {/* ================= 3-Level Regional Aging Breakdown Table ================= */}
      {displayedRows.length === 0 ? (
        <div className="dashboard-empty-state">
          <h4>No Unresolved Aging Issues Found</h4>
          <p>
            {hasActiveFilters ||
            regionSearchTerm ||
            agingSliceFilter !== "all"
              ? "No unresolved issues match the selected filters, slice, or search criteria."
              : "There are currently no unresolved issues in the backlog."}
          </p>
          {(hasActiveFilters ||
            regionSearchTerm ||
            agingSliceFilter !== "all") && (
            <Button
              size="sm"
              kind="tertiary"
              onClick={() => {
                setAgingSliceFilter("all");
                setRegionSearchTerm("");
                if (hasActiveFilters) {
                  onResetFilters();
                }
              }}
            >
              Reset Aging Filters
            </Button>
          )}
        </div>
      ) : (
        <div className="summary-table-container">
          <DataTable rows={displayedRows} headers={AGING_SUMMARY_HEADERS}>
            {({
              rows,
              headers,
              getTableProps,
              getHeaderProps,
              getRowProps,
              getExpandHeaderProps,
            }) => (
              <Table
                {...getTableProps()}
                aria-label="Regional unresolved issue aging breakdown table"
              >
                <TableHead>
                  <TableRow>
                    <TableExpandHeader {...getExpandHeaderProps()} />
                    {headers.map((header) => {
                      const isSorted = sortHeader === header.key;
                      const { key: headerKey, ...headerProps } = getHeaderProps({
                        header,
                        isSortable: true,
                        onClick: () => handleSort(header.key),
                      });
                      return (
                        <TableHeader
                          key={headerKey}
                          {...headerProps}
                          isSortable
                          isSortHeader={isSorted}
                          sortDirection={isSorted ? sortDirection : "NONE"}
                        >
                          {header.header}
                        </TableHeader>
                      );
                    })}
                  </TableRow>
                </TableHead>

                <TableBody>
                  {rows.map((row) => {
                    const summaryItem = displayedRows.find(
                      (item) => item.id === row.id,
                    );
                    const { key: rowKey, ...rowProps } = getRowProps({ row });

                    return (
                      <Fragment key={row.id}>
                        <TableExpandRow key={rowKey} {...rowProps}>
                          {row.cells.map((cell) => {
                            const headerKey = cell.info.header;

                            if (headerKey === "region") {
                              return (
                                <TableCell key={cell.id}>
                                  <div className="region-cell">
                                    <span>{cell.value}</span>
                                  </div>
                                </TableCell>
                              );
                            }

                            if (
                              BUCKET_IDS.includes(headerKey as AgingBucketId)
                            ) {
                              const count = Number(cell.value || 0);
                              return (
                                <TableCell key={cell.id}>
                                  {renderDaysCount(
                                    headerKey as AgingBucketId,
                                    count,
                                  )}
                                </TableCell>
                              );
                            }

                            if (headerKey === "avgAgeDays") {
                              const avg = Number(cell.value || 0);
                              return (
                                <TableCell key={cell.id}>
                                  <span className="stat-cell">{avg}d</span>
                                </TableCell>
                              );
                            }

                            if (headerKey === "total") {
                              const count = Number(cell.value || 0);
                              return (
                                <TableCell key={cell.id}>
                                  <span className="total-pill">{count}</span>
                                </TableCell>
                              );
                            }

                            return (
                              <TableCell key={cell.id}>
                                {cell.value ?? "-"}
                              </TableCell>
                            );
                          })}
                        </TableExpandRow>

                        <TableExpandedRow
                          key={`expanded-${row.id}`}
                          colSpan={headers.length + 1}
                        >
                          {row.isExpanded && summaryItem && (
                            <div className="expanded-panel">
                              <div className="expanded-panel-header">
                                <div>
                                  <h4 className="expanded-panel-title">
                                    District &amp; Facility Aging Breakdown for{" "}
                                    {summaryItem.region} Region
                                  </h4>
                                  <p className="expanded-panel-subtitle">
                                    {summaryItem.districts.length} district(s) |{" "}
                                    {summaryItem.districts.reduce(
                                      (acc, d) => acc + d.facilities.length,
                                      0,
                                    )}{" "}
                                    facility(ies) affected |{" "}
                                    <strong>{summaryItem.total}</strong>{" "}
                                    unresolved issues
                                  </p>
                                </div>

                                <div className="expanded-panel-actions">
                                  <Button
                                    size="sm"
                                    kind="ghost"
                                    onClick={() =>
                                      toggleAllDistrictsInRegion(
                                        summaryItem.region,
                                        summaryItem.districts,
                                      )
                                    }
                                  >
                                    {isEveryDistrictExpanded(
                                      summaryItem.region,
                                      summaryItem.districts,
                                    )
                                      ? "Collapse All Facilities"
                                      : "Expand All Facilities"}
                                  </Button>

                                  <Button
                                    size="sm"
                                    kind="tertiary"
                                    renderIcon={ArrowRight}
                                    onClick={() =>
                                      onNavigateToRegion(summaryItem.region)
                                    }
                                  >
                                    View Issues in {summaryItem.region}
                                  </Button>
                                </div>
                              </div>

                              <div className="district-table-wrapper">
                                <table>
                                  <thead>
                                    <tr>
                                      <th>District</th>
                                      <th>0–7 Days</th>
                                      <th>8–14 Days</th>
                                      <th>15–30 Days</th>
                                      <th>31–60 Days</th>
                                      <th>61–90 Days</th>
                                      <th>&gt; 90 Days</th>
                                      <th>Avg Days</th>
                                      <th>Total Unresolved</th>
                                      <th className="table-action-header">
                                        Actions
                                      </th>
                                    </tr>
                                  </thead>
                                  <tbody>
                                    {summaryItem.districts.map((d) => {
                                      const expanded = isDistrictExpanded(
                                        summaryItem.region,
                                        d,
                                      );

                                      return (
                                        <Fragment key={d.district}>
                                          <tr
                                            className={`district-row ${
                                              expanded
                                                ? "district-row--expanded"
                                                : ""
                                            }`}
                                          >
                                            <td>
                                              <button
                                                type="button"
                                                aria-expanded={expanded}
                                                className="district-expand-btn"
                                                onClick={() =>
                                                  toggleDistrict(
                                                    summaryItem.region,
                                                    d,
                                                  )
                                                }
                                                title={
                                                  expanded
                                                    ? "Collapse facility aging breakdown"
                                                    : "Expand facility aging breakdown"
                                                }
                                              >
                                                {expanded ? (
                                                  <ChevronDown
                                                    size={16}
                                                    className="district-expand-icon"
                                                  />
                                                ) : (
                                                  <ChevronRight
                                                    size={16}
                                                    className="district-expand-icon"
                                                  />
                                                )}
                                                <span className="district-name-text">
                                                  {d.district}
                                                </span>
                                                <span className="district-facility-badge">
                                                  {d.facilities.length}{" "}
                                                  {d.facilities.length === 1
                                                    ? "facility"
                                                    : "facilities"}
                                                </span>
                                              </button>
                                            </td>
                                            {BUCKET_IDS.map((bId) => (
                                              <td key={bId}>
                                                {renderDaysCount(
                                                  bId,
                                                  d.buckets[bId],
                                                )}
                                              </td>
                                            ))}
                                            <td>{d.avgAgeDays}d</td>
                                            <td>
                                              <strong>{d.total}</strong>
                                            </td>
                                            <td className="table-action-cell">
                                              <Button
                                                size="sm"
                                                kind="ghost"
                                                hasIconOnly
                                                renderIcon={ArrowRight}
                                                iconDescription={`View issues in ${d.district}`}
                                                tooltipPosition="left"
                                                onClick={() =>
                                                  onNavigateToDistrict(
                                                    d.district,
                                                  )
                                                }
                                              />
                                            </td>
                                          </tr>

                                          {expanded && (
                                            <tr className="facility-breakdown-row">
                                              <td colSpan={10}>
                                                <div className="facility-breakdown-panel">
                                                  <div className="facility-breakdown-header">
                                                    <span className="facility-breakdown-title">
                                                      Facilities in {d.district}{" "}
                                                      ({d.facilities.length})
                                                    </span>
                                                  </div>
                                                  {d.facilities.length === 0 ? (
                                                    <p className="facility-empty-note">
                                                      No health facilities
                                                      recorded for this
                                                      district.
                                                    </p>
                                                  ) : (
                                                    <table className="facility-table">
                                                      <thead>
                                                        <tr>
                                                          <th>
                                                            Health Facility /
                                                            Org Unit
                                                          </th>
                                                          <th>0–7 Days</th>
                                                          <th>8–14 Days</th>
                                                          <th>15–30 Days</th>
                                                          <th>31–60 Days</th>
                                                          <th>61–90 Days</th>
                                                          <th>&gt; 90 Days</th>
                                                          <th>Avg Days</th>
                                                          <th>
                                                            Total Unresolved
                                                          </th>
                                                          <th className="table-action-header">
                                                            Actions
                                                          </th>
                                                        </tr>
                                                      </thead>
                                                      <tbody>
                                                        {d.facilities.map(
                                                          (f) => (
                                                            <tr
                                                              key={f.facility}
                                                            >
                                                              <td>
                                                                <span className="facility-name-cell">
                                                                  {f.facility}
                                                                </span>
                                                              </td>
                                                              {BUCKET_IDS.map(
                                                                (bId) => (
                                                                  <td key={bId}>
                                                                    {renderDaysCount(
                                                                      bId,
                                                                      f.buckets[
                                                                        bId
                                                                      ],
                                                                    )}
                                                                  </td>
                                                                ),
                                                              )}
                                                              <td>
                                                                {f.avgAgeDays}d
                                                              </td>
                                                              <td>
                                                                <strong>
                                                                  {f.total}
                                                                </strong>
                                                              </td>
                                                              <td className="table-action-cell">
                                                                <Button
                                                                  size="sm"
                                                                  kind="ghost"
                                                                  hasIconOnly
                                                                  renderIcon={
                                                                    ArrowRight
                                                                  }
                                                                  iconDescription={`View issues in ${f.facility}`}
                                                                  tooltipPosition="left"
                                                                  onClick={() =>
                                                                    onNavigateToFacility(
                                                                      f.facility,
                                                                    )
                                                                  }
                                                                />
                                                              </td>
                                                            </tr>
                                                          ),
                                                        )}
                                                      </tbody>
                                                    </table>
                                                  )}
                                                </div>
                                              </td>
                                            </tr>
                                          )}
                                        </Fragment>
                                      );
                                    })}
                                  </tbody>
                                </table>
                              </div>
                            </div>
                          )}
                        </TableExpandedRow>
                      </Fragment>
                    );
                  })}

                  <TableRow className="grand-total-row">
                    <TableCell />
                    <TableCell>
                      <strong>TOTAL (All Regions)</strong>
                    </TableCell>
                    {BUCKET_IDS.map((bId) => (
                      <TableCell key={bId}>
                        {renderDaysCount(bId, tableGrandTotals[bId])}
                      </TableCell>
                    ))}
                    <TableCell>
                      <strong>{tableGrandTotals.avgAgeDays}d</strong>
                    </TableCell>
                    <TableCell>
                      <span className="total-pill">
                        {tableGrandTotals.total}
                      </span>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            )}
          </DataTable>
        </div>
      )}
    </>
  );
};
