import { Fragment, useMemo, useState } from "react";
import {
  Button,
  DataTable,
  InlineLoading,
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
import type {
  DashboardGrandTotals,
  DistrictSummary,
  RegionSummaryRow,
} from "../types";
import {
  exportRegionalSummaryToExcel,
  safeLower,
} from "../lib/issueAggregation";

type SortHeader =
  | "region"
  | "critical"
  | "open"
  | "resolved"
  | "total"
  | "percentage"
  | string;

export interface DashboardRegionalTableProps {
  regionalSummary: RegionSummaryRow[];
  filteredGrandTotals: DashboardGrandTotals;
  regionSearchTerm: string;
  onRegionSearchChange: (term: string) => void;
  isLoading: boolean;
  hasError: boolean;
  hasActiveFilters: boolean;
  onRetry: () => void;
  onResetFilters: () => void;
  onNavigateToRegion: (regionName: string) => void;
  onNavigateToDistrict: (districtName: string) => void;
  onNavigateToFacility: (facilityName: string) => void;
}

const TABLE_HEADERS = [
  { key: "region", header: "Region" },
  { key: "critical", header: "Critical Issues" },
  { key: "open", header: "Open Issues" },
  { key: "resolved", header: "Resolved Issues" },
  { key: "total", header: "Total Issues" },
  { key: "percentage", header: "Share of Total" },
];

export const DashboardRegionalTable = ({
  regionalSummary,
  filteredGrandTotals,
  regionSearchTerm,
  onRegionSearchChange,
  isLoading,
  hasError,
  hasActiveFilters,
  onRetry,
  onResetFilters,
  onNavigateToRegion,
  onNavigateToDistrict,
  onNavigateToFacility,
}: DashboardRegionalTableProps) => {
  const [sortHeader, setSortHeader] = useState<SortHeader>("total");
  const [sortDirection, setSortDirection] = useState<"ASC" | "DESC">("DESC");
  const [expandedDistricts, setExpandedDistricts] = useState<Record<string, boolean>>({});

  const handleSort = (headerKey: string) => {
    if (sortHeader === headerKey) {
      setSortDirection((prev) => (prev === "ASC" ? "DESC" : "ASC"));
    } else {
      setSortHeader(headerKey);
      setSortDirection("DESC");
    }
  };

  const isDistrictExpanded = (region: string, d: DistrictSummary) => {
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

  const toggleDistrict = (region: string, d: DistrictSummary) => {
    const districtKey = `${region}-${d.district}`;
    const currentlyExpanded = isDistrictExpanded(region, d);
    setExpandedDistricts((prev) => ({
      ...prev,
      [districtKey]: !currentlyExpanded,
    }));
  };

  const isEveryDistrictExpanded = (region: string, districts: DistrictSummary[]) => {
    if (districts.length === 0) return false;
    return districts.every((d) => isDistrictExpanded(region, d));
  };

  const toggleAllDistrictsInRegion = (region: string, districts: DistrictSummary[]) => {
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
    let result = [...regionalSummary];

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
  }, [regionalSummary, regionSearchTerm, sortHeader, sortDirection]);

  const tableGrandTotals = useMemo(() => {
    if (!regionSearchTerm.trim()) {
      return filteredGrandTotals;
    }
    return displayedRows.reduce(
      (acc, row) => ({
        critical: acc.critical + row.critical,
        open: acc.open + row.open,
        resolved: acc.resolved + row.resolved,
        total: acc.total + row.total,
        activeRegionsCount: displayedRows.length,
      }),
      {
        critical: 0,
        open: 0,
        resolved: 0,
        total: 0,
        activeRegionsCount: 0,
      },
    );
  }, [regionSearchTerm, filteredGrandTotals, displayedRows]);

  return (
    <>
      <div className="dashboard-toolbar-row">
        <div className="dashboard-search">
          <Search
            labelText="Search regions, districts, or facilities"
            placeholder="Search by region, district, or facility name..."
            value={regionSearchTerm}
            onChange={(e) => onRegionSearchChange(e.target.value)}
            size="sm"
            closeButtonLabelText="Clear search"
          />
        </div>

        <Button
          size="sm"
          kind="secondary"
          renderIcon={Download}
          disabled={displayedRows.length === 0 || isLoading}
          onClick={() =>
            exportRegionalSummaryToExcel(displayedRows, tableGrandTotals)
          }
        >
          Export Summary (.xlsx)
        </Button>
      </div>

      {isLoading ? (
        <div className="dashboard-loading-state">
          <InlineLoading
            status="active"
            description="Loading regional issue summaries..."
          />
        </div>
      ) : hasError ? (
        <div className="dashboard-empty-state">
          <h4>Failed to Load Issue Data</h4>
          <p>An error occurred while retrieving issue data from the server.</p>
          <Button size="sm" kind="tertiary" onClick={onRetry}>
            Retry
          </Button>
        </div>
      ) : displayedRows.length === 0 ? (
        <div className="dashboard-empty-state">
          <h4>No Regional Issues Found</h4>
          <p>
            {hasActiveFilters || regionSearchTerm
              ? "No issues match the selected filters or search criteria. Try adjusting or resetting the filters."
              : "There are currently no recorded issues to summarize."}
          </p>
          {(hasActiveFilters || regionSearchTerm) && (
            <Button size="sm" kind="tertiary" onClick={onResetFilters}>
              Reset Filters
            </Button>
          )}
        </div>
      ) : (
        <div className="summary-table-container">
          <DataTable rows={displayedRows} headers={TABLE_HEADERS}>
            {({
              rows,
              headers,
              getTableProps,
              getHeaderProps,
              getRowProps,
              getExpandHeaderProps,
            }) => (
              <Table {...getTableProps()} aria-label="Regional issue summary table">
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
                    const summaryItem = displayedRows.find((item) => item.id === row.id);
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

                            if (headerKey === "critical") {
                              const count = Number(cell.value || 0);
                              return (
                                <TableCell key={cell.id}>
                                  <span
                                    className={`severity-pill ${
                                      count > 0
                                        ? "severity-pill--critical"
                                        : "severity-pill--zero"
                                    }`}
                                  >
                                    {count > 0 ? count : "-"}
                                  </span>
                                </TableCell>
                              );
                            }

                            if (headerKey === "open") {
                              const count = Number(cell.value || 0);
                              return (
                                <TableCell key={cell.id}>
                                  <span className="stat-cell">
                                    <span className="stat-dot stat-dot--open" />
                                    {count}
                                  </span>
                                </TableCell>
                              );
                            }

                            if (headerKey === "resolved") {
                              const count = Number(cell.value || 0);
                              return (
                                <TableCell key={cell.id}>
                                  <span className="stat-cell">
                                    <span className="stat-dot stat-dot--resolved" />
                                    {count}
                                  </span>
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

                            if (headerKey === "percentage") {
                              const pct = Number(cell.value || 0);
                              return (
                                <TableCell key={cell.id}>
                                  <div className="share-bar-wrapper">
                                    <progress
                                      className="share-progress"
                                      max={100}
                                      value={Math.min(100, pct)}
                                    />
                                    <span className="share-percent-text">
                                      {pct.toFixed(1)}%
                                    </span>
                                  </div>
                                </TableCell>
                              );
                            }

                            return (
                              <TableCell key={cell.id}>{cell.value ?? "-"}</TableCell>
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
                                    District &amp; Facility Breakdown for{" "}
                                    {summaryItem.region} Region
                                  </h4>
                                  <p className="expanded-panel-subtitle">
                                    {summaryItem.districts.length} district(s) |{" "}
                                    {summaryItem.districts.reduce(
                                      (acc, d) => acc + d.facilities.length,
                                      0,
                                    )}{" "}
                                    facility(ies) affected |{" "}
                                    <strong>{summaryItem.total}</strong> total issues (
                                    {summaryItem.critical > 0 && (
                                      <>
                                        <span className="stat-text--critical">
                                          {summaryItem.critical} Critical
                                        </span>
                                        {", "}
                                      </>
                                    )}
                                    <span className="stat-text--open">
                                      {summaryItem.open} Open
                                    </span>
                                    ,{" "}
                                    <span className="stat-text--resolved">
                                      {summaryItem.resolved} Resolved
                                    </span>
                                    )
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
                                      <th>Critical Issues</th>
                                      <th>Open</th>
                                      <th>Resolved</th>
                                      <th>Total Issues</th>
                                      <th className="table-action-header">Actions</th>
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
                                              expanded ? "district-row--expanded" : ""
                                            }`}
                                          >
                                            <td>
                                              <button
                                                type="button"
                                                aria-expanded={expanded}
                                                className="district-expand-btn"
                                                onClick={() =>
                                                  toggleDistrict(summaryItem.region, d)
                                                }
                                                title={
                                                  expanded
                                                    ? "Collapse facility breakdown"
                                                    : "Expand facility breakdown"
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
                                            <td>
                                              <span
                                                className={`severity-pill ${
                                                  d.critical > 0
                                                    ? "severity-pill--critical"
                                                    : "severity-pill--zero"
                                                }`}
                                              >
                                                {d.critical > 0 ? d.critical : "-"}
                                              </span>
                                            </td>
                                            <td>
                                              <span className="stat-cell">
                                                <span className="stat-dot stat-dot--open" />
                                                {d.open}
                                              </span>
                                            </td>
                                            <td>
                                              <span className="stat-cell">
                                                <span className="stat-dot stat-dot--resolved" />
                                                {d.resolved}
                                              </span>
                                            </td>
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
                                                  onNavigateToDistrict(d.district)
                                                }
                                              />
                                            </td>
                                          </tr>

                                          {expanded && (
                                            <tr className="facility-breakdown-row">
                                              <td colSpan={6}>
                                                <div className="facility-breakdown-panel">
                                                  <div className="facility-breakdown-header">
                                                    <span className="facility-breakdown-title">
                                                      Facilities in {d.district} (
                                                      {d.facilities.length})
                                                    </span>
                                                  </div>
                                                  {d.facilities.length === 0 ? (
                                                    <p className="facility-empty-note">
                                                      No health facilities recorded for
                                                      this district.
                                                    </p>
                                                  ) : (
                                                    <table className="facility-table">
                                                      <thead>
                                                        <tr>
                                                          <th>
                                                            Health Facility / Org Unit
                                                          </th>
                                                          <th>Critical Issues</th>
                                                          <th>Open</th>
                                                          <th>Resolved</th>
                                                          <th>Total Issues</th>
                                                          <th className="table-action-header">
                                                            Actions
                                                          </th>
                                                        </tr>
                                                      </thead>
                                                      <tbody>
                                                        {d.facilities.map((f) => (
                                                          <tr key={f.facility}>
                                                            <td>
                                                              <span className="facility-name-cell">
                                                                {f.facility}
                                                              </span>
                                                            </td>
                                                            <td>
                                                              <span
                                                                className={`severity-pill ${
                                                                  f.critical > 0
                                                                    ? "severity-pill--critical"
                                                                    : "severity-pill--zero"
                                                                }`}
                                                              >
                                                                {f.critical > 0
                                                                  ? f.critical
                                                                  : "-"}
                                                              </span>
                                                            </td>
                                                            <td>
                                                              <span className="stat-cell">
                                                                <span className="stat-dot stat-dot--open" />
                                                                {f.open}
                                                              </span>
                                                            </td>
                                                            <td>
                                                              <span className="stat-cell">
                                                                <span className="stat-dot stat-dot--resolved" />
                                                                {f.resolved}
                                                              </span>
                                                            </td>
                                                            <td>
                                                              <strong>{f.total}</strong>
                                                            </td>
                                                            <td className="table-action-cell">
                                                              <Button
                                                                size="sm"
                                                                kind="ghost"
                                                                hasIconOnly
                                                                renderIcon={ArrowRight}
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
                                                        ))}
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
                    <TableCell>
                      <span
                        className={`severity-pill ${
                          tableGrandTotals.critical > 0
                            ? "severity-pill--critical"
                            : "severity-pill--zero"
                        }`}
                      >
                        <strong>{tableGrandTotals.critical}</strong>
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="stat-cell">
                        <span className="stat-dot stat-dot--open" />
                        <strong>{tableGrandTotals.open}</strong>
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="stat-cell">
                        <span className="stat-dot stat-dot--resolved" />
                        <strong>{tableGrandTotals.resolved}</strong>
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="total-pill">{tableGrandTotals.total}</span>
                    </TableCell>
                    <TableCell>
                      <span className="grand-total-percent">100.0%</span>
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
