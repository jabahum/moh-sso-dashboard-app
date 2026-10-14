import { useEffect, useMemo, useRef, useState, Fragment } from "react";
import {
  Button,
  ComboBox,
  Dropdown,
  Popover,
  PopoverContent,
  Search,
  DataTable,
  Table,
  TableHead,
  TableRow,
  TableHeader,
  TableBody,
  TableCell,
  TableExpandHeader,
  TableExpandRow,
  TableExpandedRow,
  InlineLoading,
} from "@carbon/react";
import {
  ChevronDown,
  ChevronRight,
  Download,
  Filter,
  ArrowRight,
  List,
  Renew,
} from "@carbon/react/icons";
import * as XLSX from "xlsx";
import { useNavigate } from "react-router-dom";
import { PermissionGuard, PERMISSIONS } from "@moh-sso/auth";

import { useGetIssuesQuery, useGetIssuesSummaryByProgramQuery } from "../api";
import type { Issue } from "../types";
import { getAvailablePeriods, periodType } from "../../../data-visualizer/src/pages/Constants.tsx";
import {
  useGetDataSetsQuery,
  type Dataset,
} from "../../../data-visualizer/src/pages/modals/data-model/data-model.ts";
import { useGetHierarchyQuery } from "../../../data-visualizer/src/pages/modals/orgunit/org-unit.ts";

import "./issue-dashboard.scss";

type FacilitySummary = {
  facility: string;
  critical: number;
  open: number;
  resolved: number;
  total: number;
};

type DistrictSummary = {
  district: string;
  critical: number;
  open: number;
  resolved: number;
  total: number;
  facilities: FacilitySummary[];
};

type RegionSummaryRow = {
  id: string;
  region: string;
  critical: number;
  open: number;
  resolved: number;
  total: number;
  percentage: number;
  districts: DistrictSummary[];
  [key: string]: unknown;
};

type SelectEvent<T> = {
  selectedItem?: T | null;
};

type PeriodOption = {
  label: string;
  value?: string;
};

type SortHeader = "region" | "critical" | "open" | "resolved" | "total" | "percentage" | string;

function safeString(val: unknown): string {
  if (val == null) return "";
  return String(val).trim();
}

function safeLower(val: unknown): string {
  return safeString(val).toLowerCase();
}

function isIssueResolved(status?: string | null): boolean {
  if (!status) return false;
  const s = safeLower(status);
  return s === "resolved" || s === "closed" || s === "done";
}

function isCriticalIssue(issue: Issue): boolean {
  const sev = safeLower(issue.severity);
  const prio = safeLower(issue.priority);
  return sev === "critical" || prio === "critical";
}

const IssueDashboard = () => {
  const navigate = useNavigate();
  const currentYear = new Date().getFullYear();

  /* -------------------------------------------------------------
   * API Queries
   * ------------------------------------------------------------- */
  const [selectedProgram, setSelectedProgram] = useState<string | undefined>();

  const {
    data: issuesData,
    isLoading: isLoadingIssues,
    error: issuesError,
    refetch: refetchIssues,
  } = useGetIssuesQuery({
    limit: 10000,
    offset: 0,
    program: selectedProgram,
  });

  const { data: summaryPrograms = [] } = useGetIssuesSummaryByProgramQuery();
  const { data: datasets = [] } = useGetDataSetsQuery();
  const { data: hierarchyData = [] } = useGetHierarchyQuery();

  /* -------------------------------------------------------------
   * Filters State
   * ------------------------------------------------------------- */
  const [selectedYear, setSelectedYear] = useState<number>(currentYear);
  const [selectedPeriodType, setSelectedPeriodType] = useState<string>("Monthly");
  const [selectedPeriod, setSelectedPeriod] = useState<string>("");
  const [availablePeriods, setAvailablePeriods] = useState<PeriodOption[]>(
    getAvailablePeriods("Monthly", currentYear.toString()),
  );
  const [isPeriodPopoverOpen, setIsPeriodPopoverOpen] = useState<boolean>(false);
  const periodPopoverRef = useRef<HTMLDivElement>(null);

  const [selectedDataset, setSelectedDataset] = useState<string>("");
  const [regionSearchTerm, setRegionSearchTerm] = useState<string>("");

  /* Sorting state */
  const [sortHeader, setSortHeader] = useState<SortHeader>("total");
  const [sortDirection, setSortDirection] = useState<"ASC" | "DESC">("DESC");

  /* Expanded districts state for facility breakdown */
  const [expandedDistricts, setExpandedDistricts] = useState<Record<string, boolean>>({});

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

  const years = useMemo(
    () => Array.from({ length: 10 }, (_, index) => currentYear - index),
    [currentYear],
  );

  const datasetNames = useMemo(
    () => (datasets as Dataset[]).map((d) => d.display_name).filter(Boolean),
    [datasets],
  );

  const programNames = useMemo(
    () => summaryPrograms.map((p) => p.program).filter(Boolean),
    [summaryPrograms],
  );

  const hasActiveFilters =
    selectedYear !== currentYear ||
    selectedPeriodType !== "Monthly" ||
    Boolean(selectedPeriod) ||
    Boolean(selectedDataset) ||
    Boolean(selectedProgram);

  /* Close popover on outside click */
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Node;
      if (periodPopoverRef.current && !periodPopoverRef.current.contains(target)) {
        setIsPeriodPopoverOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, []);

  /* -------------------------------------------------------------
   * Hierarchy Org-to-Region & Org-to-District mapping
   * ------------------------------------------------------------- */
  const { orgToRegionMap, orgToDistrictMap, orgIdToNameMap } = useMemo(() => {
    const regionMap = new Map<string, string>();
    const districtMap = new Map<string, string>();
    const idToNameMap = new Map<string, string>();

    if (!hierarchyData) {
      return { orgToRegionMap: regionMap, orgToDistrictMap: districtMap, orgIdToNameMap: idToNameMap };
    }

    const rootList: any[] = Array.isArray(hierarchyData)
      ? hierarchyData
      : typeof hierarchyData === "object"
      ? [hierarchyData]
      : [];

    function traverseTree(
      nodes: any[],
      currentRegion: string | null,
      currentDistrict: string | null,
    ) {
      if (!Array.isArray(nodes)) return;

      for (const node of nodes) {
        if (!node || typeof node !== "object") continue;

        const nodeName = safeString(node.name);
        const nodeId = safeString(node.id);

        if (nodeName && nodeId) {
          idToNameMap.set(nodeId.toLowerCase(), nodeName);
        }

        const nodeRegion = currentRegion;
        const nodeDistrict = currentDistrict;

        // If top root (Level 1 e.g. "MOH - Uganda"), its children are Level 2 Regions
        if (!currentRegion) {
          if (node.children && Array.isArray(node.children) && node.children.length > 0) {
            for (const regionNode of node.children) {
              if (!regionNode || typeof regionNode !== "object") continue;

              const regName = safeString(regionNode.name);
              const regId = safeString(regionNode.id);

              if (regName) {
                regionMap.set(regName.toLowerCase(), regName);
                if (regId) {
                  regionMap.set(regId.toLowerCase(), regName);
                  idToNameMap.set(regId.toLowerCase(), regName);
                }
              }

              // Children of region are Level 3 Districts
              if (regionNode.children && Array.isArray(regionNode.children) && regionNode.children.length > 0) {
                for (const distNode of regionNode.children) {
                  if (!distNode || typeof distNode !== "object") continue;

                  const distName = safeString(distNode.name);
                  const distId = safeString(distNode.id);

                  if (distName) {
                    districtMap.set(distName.toLowerCase(), distName);
                    if (regName) regionMap.set(distName.toLowerCase(), regName);
                    if (distId) {
                      districtMap.set(distId.toLowerCase(), distName);
                      if (regName) regionMap.set(distId.toLowerCase(), regName);
                      idToNameMap.set(distId.toLowerCase(), distName);
                    }
                  }
                  // Any facility or sub-county under district
                  if (distNode.children && Array.isArray(distNode.children) && distNode.children.length > 0) {
                    traverseTree(distNode.children, regName, distName);
                  }
                }
              }
            }
            continue;
          }
        }

        if (nodeName) {
          if (nodeRegion) regionMap.set(nodeName.toLowerCase(), nodeRegion);
          if (nodeDistrict) districtMap.set(nodeName.toLowerCase(), nodeDistrict);
        }
        if (nodeId) {
          if (nodeRegion) regionMap.set(nodeId.toLowerCase(), nodeRegion);
          if (nodeDistrict) districtMap.set(nodeId.toLowerCase(), nodeDistrict);
        }

        if (node.children && Array.isArray(node.children) && node.children.length > 0) {
          traverseTree(node.children, nodeRegion, nodeDistrict);
        }
      }
    }

    traverseTree(rootList, null, null);
    return { orgToRegionMap: regionMap, orgToDistrictMap: districtMap, orgIdToNameMap: idToNameMap };
  }, [hierarchyData]);

  /* -------------------------------------------------------------
   * Filtered Issues list
   * ------------------------------------------------------------- */
  const rawIssues: Issue[] = useMemo(() => {
    return issuesData?.items ?? [];
  }, [issuesData]);

  const filteredIssues = useMemo(() => {
    return rawIssues.filter((issue) => {
      // Dataset filter
      if (selectedDataset) {
        if (safeLower(issue.dataset) !== safeLower(selectedDataset)) {
          return false;
        }
      }

      // Period filter
      if (selectedPeriod) {
        const targetPeriod = safeLower(selectedPeriod);
        const p1 = safeLower(issue.time_period);
        const p2 = safeLower(issue.time_Period);
        if (p1 !== targetPeriod && p2 !== targetPeriod) {
          return false;
        }
      } else if (selectedYear && selectedYear !== currentYear) {
        const yearStr = String(selectedYear);
        const p1 = safeString(issue.time_period);
        const p2 = safeString(issue.time_Period);
        if (!p1.includes(yearStr) && !p2.includes(yearStr)) {
          return false;
        }
      }

      return true;
    });
  }, [rawIssues, selectedDataset, selectedPeriod, selectedYear, currentYear]);

  /* -------------------------------------------------------------
   * Group and Aggregate by Region & Status
   * ------------------------------------------------------------- */
  const { regionalSummary, grandTotals } = useMemo(() => {
    const regionGroups = new Map<
      string,
      {
        critical: number;
        open: number;
        resolved: number;
        total: number;
        districtGroups: Map<
          string,
          {
            critical: number;
            open: number;
            resolved: number;
            total: number;
            facilityGroups: Map<
              string,
              {
                critical: number;
                open: number;
                resolved: number;
                total: number;
              }
            >;
          }
        >;
      }
    >();

    let totalCritical = 0;
    let totalOpen = 0;
    let totalResolved = 0;
    let totalAll = 0;

    for (const issue of filteredIssues) {
      // 1. Resolve Region
      let regionName = safeString(issue.region);
      if (regionName && orgIdToNameMap.has(safeLower(regionName))) {
        regionName = orgIdToNameMap.get(safeLower(regionName))!;
      }
      if (!regionName && issue.org_unit) {
        regionName = orgToRegionMap.get(safeLower(issue.org_unit)) || "";
      }
      if (!regionName && issue.district) {
        regionName = orgToRegionMap.get(safeLower(issue.district)) || "";
      }
      if (!regionName) {
        regionName = "Unspecified";
      }

      // 2. Resolve District
      let districtName = safeString(issue.district);
      if (districtName && orgIdToNameMap.has(safeLower(districtName))) {
        districtName = orgIdToNameMap.get(safeLower(districtName))!;
      }
      if (!districtName && issue.org_unit) {
        districtName = orgToDistrictMap.get(safeLower(issue.org_unit)) || safeString(issue.org_unit);
      }
      if (!districtName) {
        districtName = "Unspecified";
      }

      // 3. Resolve Facility
      let facilityName = safeString(issue.org_unit);
      if (!facilityName) {
        facilityName = "Unspecified Facility";
      } else {
        const mappedName = orgIdToNameMap.get(safeLower(facilityName));
        if (mappedName) {
          facilityName = mappedName;
        }
      }

      // 4. Resolve Status & Critical (Open)
      const resolved = isIssueResolved(issue.status);
      const isCriticalOpen = !resolved && isCriticalIssue(issue);

      // Init Region group if absent
      if (!regionGroups.has(regionName)) {
        regionGroups.set(regionName, {
          critical: 0,
          open: 0,
          resolved: 0,
          total: 0,
          districtGroups: new Map(),
        });
      }

      const rGroup = regionGroups.get(regionName)!;
      rGroup.total += 1;
      totalAll += 1;

      if (resolved) {
        rGroup.resolved += 1;
        totalResolved += 1;
      } else {
        rGroup.open += 1;
        totalOpen += 1;
      }

      if (isCriticalOpen) {
        rGroup.critical += 1;
        totalCritical += 1;
      }

      // District aggregation inside Region
      if (!rGroup.districtGroups.has(districtName)) {
        rGroup.districtGroups.set(districtName, {
          critical: 0,
          open: 0,
          resolved: 0,
          total: 0,
          facilityGroups: new Map(),
        });
      }

      const dGroup = rGroup.districtGroups.get(districtName)!;
      dGroup.total += 1;
      if (resolved) {
        dGroup.resolved += 1;
      } else {
        dGroup.open += 1;
      }
      if (isCriticalOpen) {
        dGroup.critical += 1;
      }

      // Facility aggregation inside District
      if (!dGroup.facilityGroups.has(facilityName)) {
        dGroup.facilityGroups.set(facilityName, {
          critical: 0,
          open: 0,
          resolved: 0,
          total: 0,
        });
      }

      const fGroup = dGroup.facilityGroups.get(facilityName)!;
      fGroup.total += 1;
      if (resolved) {
        fGroup.resolved += 1;
      } else {
        fGroup.open += 1;
      }
      if (isCriticalOpen) {
        fGroup.critical += 1;
      }
    }

    // Build rows list
    const rows: RegionSummaryRow[] = [];
    regionGroups.forEach((val, region) => {
      const districtsList: DistrictSummary[] = [];
      val.districtGroups.forEach((dVal, dName) => {
        const facilitiesList: FacilitySummary[] = [];
        dVal.facilityGroups.forEach((fVal, fName) => {
          facilitiesList.push({
            facility: fName,
            critical: fVal.critical,
            open: fVal.open,
            resolved: fVal.resolved,
            total: fVal.total,
          });
        });

        // Sort facilities by total descending, then critical descending
        facilitiesList.sort((a, b) => b.total - a.total || b.critical - a.critical);

        districtsList.push({
          district: dName,
          critical: dVal.critical,
          open: dVal.open,
          resolved: dVal.resolved,
          total: dVal.total,
          facilities: facilitiesList,
        });
      });

      // Sort districts by total descending
      districtsList.sort((a, b) => b.total - a.total);

      const percentage = totalAll > 0 ? (val.total / totalAll) * 100 : 0;

      rows.push({
        id: `region-${region}`,
        region,
        critical: val.critical,
        open: val.open,
        resolved: val.resolved,
        total: val.total,
        percentage,
        districts: districtsList,
      });
    });

    return {
      regionalSummary: rows,
      grandTotals: {
        critical: totalCritical,
        open: totalOpen,
        resolved: totalResolved,
        total: totalAll,
        activeRegionsCount: regionGroups.size,
      },
    };
  }, [filteredIssues, orgToRegionMap, orgToDistrictMap, orgIdToNameMap]);

  /* -------------------------------------------------------------
   * Table Columns Definition
   * ------------------------------------------------------------- */
  const tableHeaders = useMemo(
    () => [
      { key: "region", header: "Region" },
      { key: "critical", header: "Critical Issues" },
      { key: "open", header: "Open Issues" },
      { key: "resolved", header: "Resolved Issues" },
      { key: "total", header: "Total Issues" },
      { key: "percentage", header: "Share of Total" },
    ],
    [],
  );

  /* -------------------------------------------------------------
   * Sorted & Filtered Rows for Display
   * ------------------------------------------------------------- */
  const displayedRows = useMemo(() => {
    let result = [...regionalSummary];

    // Filter by Region, District, or Facility search
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

    // Sort
    result.sort((a, b) => {
      let aVal = a[sortHeader];
      let bVal = b[sortHeader];

      if (typeof aVal === "string") {
        aVal = (aVal as string).toLowerCase();
        bVal = String(bVal ?? "").toLowerCase();
        if (sortDirection === "ASC") {
          return (aVal as string).localeCompare(bVal as string);
        }
        return (bVal as string).localeCompare(aVal as string);
      }

      const aNum = Number(aVal ?? 0);
      const bNum = Number(bVal ?? 0);
      if (sortDirection === "ASC") {
        return aNum - bNum;
      }
      return bNum - aNum;
    });

    return result;
  }, [regionalSummary, regionSearchTerm, sortHeader, sortDirection]);

  /* -------------------------------------------------------------
   * Handlers
   * ------------------------------------------------------------- */
  const handleSort = (headerKey: string) => {
    if (sortHeader === headerKey) {
      setSortDirection((prev) => (prev === "ASC" ? "DESC" : "ASC"));
    } else {
      setSortHeader(headerKey);
      setSortDirection("DESC");
    }
  };

  const handleYearChange = ({ selectedItem }: SelectEvent<number>) => {
    if (selectedItem == null) return;
    setSelectedYear(selectedItem);
    setAvailablePeriods(getAvailablePeriods(selectedPeriodType, selectedItem.toString()));
    setSelectedPeriod("");
  };

  const handlePeriodTypeChange = ({
    selectedItem,
  }: SelectEvent<{ label: string; value: string }>) => {
    if (!selectedItem) return;
    setSelectedPeriodType(selectedItem.value);
    setAvailablePeriods(getAvailablePeriods(selectedItem.value, selectedYear.toString()));
    setSelectedPeriod("");
  };

  const handlePeriodChange = ({ selectedItem }: SelectEvent<PeriodOption>) => {
    setSelectedPeriod(selectedItem?.label ?? "");
  };

  const handleResetFilters = () => {
    setSelectedYear(currentYear);
    setSelectedPeriodType("Monthly");
    setSelectedPeriod("");
    setAvailablePeriods(getAvailablePeriods("Monthly", currentYear.toString()));
    setSelectedDataset("");
    setSelectedProgram(undefined);
    setRegionSearchTerm("");
    setIsPeriodPopoverOpen(false);
  };

  /* -------------------------------------------------------------
   * Excel Export
   * ------------------------------------------------------------- */
  const handleExportExcel = () => {
    const exportHeaders = [
      "Region",
      "Critical Issues (Open)",
      "Open Issues",
      "Resolved Issues",
      "Total Issues",
      "Share (%)",
    ];

    const dataRows = regionalSummary.map((row) => [
      row.region,
      row.critical,
      row.open,
      row.resolved,
      row.total,
      `${row.percentage.toFixed(1)}%`,
    ]);

    // Grand total row
    const totalRow = [
      "TOTAL",
      grandTotals.critical,
      grandTotals.open,
      grandTotals.resolved,
      grandTotals.total,
      "100%",
    ];

    const sheetData = [exportHeaders, ...dataRows, [], totalRow];
    const worksheet = XLSX.utils.aoa_to_sheet(sheetData);

    worksheet["!cols"] = exportHeaders.map(() => ({ wch: 18 }));

    const workbook = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(workbook, worksheet, "Regional Issue Summary");

    // District Breakdown Sheet
    const districtHeaders = [
      "Region",
      "District",
      "Facilities Affected",
      "Critical Issues (Open)",
      "Open Issues",
      "Resolved Issues",
      "Total Issues",
    ];

    const districtDataRows: (string | number)[][] = [];
    for (const r of regionalSummary) {
      for (const d of r.districts) {
        districtDataRows.push([
          r.region,
          d.district,
          d.facilities.length,
          d.critical,
          d.open,
          d.resolved,
          d.total,
        ]);
      }
    }

    const districtWorksheet = XLSX.utils.aoa_to_sheet([
      districtHeaders,
      ...districtDataRows,
    ]);
    districtWorksheet["!cols"] = districtHeaders.map(() => ({ wch: 22 }));
    XLSX.utils.book_append_sheet(workbook, districtWorksheet, "District Breakdown");

    // District & Facility Breakdown Sheet
    const facilityHeaders = [
      "Region",
      "District",
      "Health Facility / Org Unit",
      "Critical Issues (Open)",
      "Open Issues",
      "Resolved Issues",
      "Total Issues",
    ];

    const facilityDataRows: (string | number)[][] = [];
    for (const r of regionalSummary) {
      for (const d of r.districts) {
        for (const f of d.facilities) {
          facilityDataRows.push([
            r.region,
            d.district,
            f.facility,
            f.critical,
            f.open,
            f.resolved,
            f.total,
          ]);
        }
      }
    }

    const facilityWorksheet = XLSX.utils.aoa_to_sheet([
      facilityHeaders,
      ...facilityDataRows,
    ]);
    facilityWorksheet["!cols"] = facilityHeaders.map(() => ({ wch: 24 }));
    XLSX.utils.book_append_sheet(workbook, facilityWorksheet, "Facility Breakdown");

    const dateStr = new Date().toISOString().slice(0, 10);
    XLSX.writeFile(workbook, `issue-regional-summary-${dateStr}.xlsx`);
  };

  const handleNavigateToIssues = (filter?: string) => {
    if (filter && !filter.toLowerCase().startsWith("unspecified")) {
      navigate(`/issues?region=${encodeURIComponent(filter)}`);
    } else {
      navigate("/issues");
    }
  };

  /* Resolution rate */
  const resolutionRate =
    grandTotals.total > 0
      ? Math.round((grandTotals.resolved / grandTotals.total) * 100)
      : 0;

  return (
    <PermissionGuard permission={PERMISSIONS.issueTrackerRead}>
      <div className="issue-dashboard-page">
        {/* ================= Header ================= */}
        <div className="dashboard-header">
          <div className="dashboard-header-text">
            <h2 className="dashboard-title">Issue Summary Dashboard</h2>
            <p className="dashboard-subtitle">
              Aggregated overview of reported data quality issues categorized by resolution status and
              grouped by region.
            </p>
          </div>

          <div className="dashboard-actions">
            <Button
              size="md"
              kind="ghost"
              renderIcon={Renew}
              onClick={() => refetchIssues()}
              disabled={isLoadingIssues}
            >
              Refresh
            </Button>

            <Button
              size="md"
              kind="secondary"
              renderIcon={Download}
              onClick={handleExportExcel}
              disabled={regionalSummary.length === 0 || isLoadingIssues}
            >
              Export Summary (.xlsx)
            </Button>

            <Button
              size="md"
              kind="primary"
              renderIcon={List}
              onClick={() => handleNavigateToIssues()}
            >
              View Registered Issues
            </Button>
          </div>
        </div>

        {/* ================= KPI Metric Cards ================= */}
        <div className="kpi-grid">
          <div className="kpi-card kpi-card--total">
            <div className="kpi-header">
              <span className="kpi-label">Total Issues</span>
            </div>
            <div className="kpi-value">{isLoadingIssues ? "..." : grandTotals.total}</div>
            <p className="kpi-subtext">
              Across <strong>{grandTotals.activeRegionsCount}</strong> affected regions
            </p>
          </div>

          <div className="kpi-card kpi-card--critical">
            <div className="kpi-header">
              <span className="kpi-label">Critical Issues</span>
              <span className="kpi-badge kpi-badge--critical">Critical</span>
            </div>
            <div className="kpi-value" style={{ color: "#a2191f" }}>
              {isLoadingIssues ? "..." : grandTotals.critical}
            </div>
            <p className="kpi-subtext">
              <strong>
                {grandTotals.open > 0
                  ? `${((grandTotals.critical / grandTotals.open) * 100).toFixed(1)}%`
                  : "0%"}
              </strong>{" "}
              of open issues
            </p>
          </div>

          <div className="kpi-card kpi-card--open">
            <div className="kpi-header">
              <span className="kpi-label">Open Issues</span>
              <span className="kpi-badge kpi-badge--open">Open</span>
            </div>
            <div className="kpi-value" style={{ color: "#da1e28" }}>
              {isLoadingIssues ? "..." : grandTotals.open}
            </div>
            <p className="kpi-subtext">
              <strong>
                {grandTotals.total > 0
                  ? `${((grandTotals.open / grandTotals.total) * 100).toFixed(1)}%`
                  : "0%"}
              </strong>{" "}
              of total issues
            </p>
          </div>

          <div className="kpi-card kpi-card--resolved">
            <div className="kpi-header">
              <span className="kpi-label">Resolved Issues</span>
              <span className="kpi-badge kpi-badge--resolved">Resolved</span>
            </div>
            <div className="kpi-value" style={{ color: "#198038" }}>
              {isLoadingIssues ? "..." : grandTotals.resolved}
            </div>
            <p className="kpi-subtext">
              <strong>
                {grandTotals.total > 0
                  ? `${((grandTotals.resolved / grandTotals.total) * 100).toFixed(1)}%`
                  : "0%"}
              </strong>{" "}
              of total issues
            </p>
          </div>

          <div className="kpi-card kpi-card--rate">
            <div className="kpi-header">
              <span className="kpi-label">Resolution Rate</span>
              <span className="kpi-badge kpi-badge--rate">
                {resolutionRate}%
              </span>
            </div>
            <div className="kpi-value" style={{ color: "#0f62fe" }}>
              {isLoadingIssues ? "..." : `${resolutionRate}%`}
            </div>
            <p className="kpi-subtext">
              Resolved vs <strong>{grandTotals.open}</strong> open issues
            </p>
          </div>
        </div>

        {/* ================= Filters Bar ================= */}
        <div className="dashboard-filter-container">
          {/* Period Filter Popover */}
          <div ref={periodPopoverRef} className="issue-filter-wrapper">
            <Popover open={isPeriodPopoverOpen} align="bottom-left" dropShadow>
              <Button
                size="md"
                kind="tertiary"
                renderIcon={ChevronDown}
                onClick={() => setIsPeriodPopoverOpen((prev) => !prev)}
              >
                {selectedPeriod
                  ? `Period: ${selectedPeriod}`
                  : selectedYear !== currentYear
                  ? `Year: ${selectedYear}`
                  : "Reporting Period"}
              </Button>

              <PopoverContent className="filter-popover-content">
                <div className="popover-inner">
                  <Dropdown
                    id="dashboard-filter-year"
                    titleText="Year"
                    label="Select year"
                    items={years}
                    selectedItem={selectedYear}
                    itemToString={(item) => (item == null ? "" : String(item))}
                    onChange={handleYearChange}
                  />

                  <Dropdown
                    id="dashboard-filter-period-type"
                    titleText="Period Type"
                    label="Select period type"
                    items={periodType}
                    selectedItem={periodType.find((item) => item.value === selectedPeriodType)}
                    itemToString={(item) => item?.label ?? ""}
                    onChange={handlePeriodTypeChange}
                  />

                  <ComboBox
                    id="dashboard-filter-period"
                    titleText="Period"
                    placeholder="Select period"
                    items={availablePeriods}
                    selectedItem={
                      availablePeriods.find((item) => item.label === selectedPeriod) ?? null
                    }
                    itemToString={(item) => item?.label ?? ""}
                    onChange={handlePeriodChange}
                  />

                  <div className="popover-footer">
                    <Button
                      size="sm"
                      kind="ghost"
                      onClick={() => {
                        setSelectedPeriod("");
                        setIsPeriodPopoverOpen(false);
                      }}
                    >
                      Clear
                    </Button>
                    <Button size="sm" onClick={() => setIsPeriodPopoverOpen(false)}>
                      Apply
                    </Button>
                  </div>
                </div>
              </PopoverContent>
            </Popover>
          </div>

          {/* Dataset Filter */}
          <div style={{ minWidth: "15rem" }}>
            <ComboBox
              id="dashboard-dataset-filter"
              size="md"
              titleText=""
              placeholder="Filter by Dataset"
              items={datasetNames}
              selectedItem={selectedDataset || null}
              onChange={({ selectedItem }: SelectEvent<string>) =>
                setSelectedDataset(selectedItem ?? "")
              }
            />
          </div>

          {/* Program Filter */}
          {programNames.length > 0 && (
            <div style={{ minWidth: "15rem" }}>
              <ComboBox
                id="dashboard-program-filter"
                size="md"
                titleText=""
                placeholder="Filter by Program"
                items={programNames}
                selectedItem={selectedProgram || null}
                onChange={({ selectedItem }: SelectEvent<string>) =>
                  setSelectedProgram(selectedItem ?? undefined)
                }
              />
            </div>
          )}

          {/* Reset Filters */}
          <Button
            size="md"
            kind="ghost"
            renderIcon={Filter}
            disabled={!hasActiveFilters}
            onClick={handleResetFilters}
          >
            Reset Filters
          </Button>

          {hasActiveFilters && (
            <span className="filter-active-indicator">
              Active Filters:{" "}
              {[
                selectedPeriod && `Period: ${selectedPeriod}`,
                selectedYear !== currentYear && !selectedPeriod && `Year: ${selectedYear}`,
                selectedDataset && `Dataset: ${selectedDataset}`,
                selectedProgram && `Program: ${selectedProgram}`,
              ]
                .filter(Boolean)
                .join(", ")}
            </span>
          )}
        </div>

        {/* ================= Search & Toolbar Row ================= */}
        <div className="dashboard-toolbar-row">
          <div className="dashboard-search">
            <Search
              labelText="Search regions, districts, or facilities"
              placeholder="Search by region, district, or facility name..."
              value={regionSearchTerm}
              onChange={(e) => setRegionSearchTerm(e.target.value)}
              size="md"
              closeButtonLabelText="Clear search"
            />
          </div>
        </div>

        {/* ================= Data Table Section ================= */}
        {isLoadingIssues ? (
          <div className="dashboard-loading-state">
            <InlineLoading status="active" description="Loading regional issue summaries..." />
          </div>
        ) : issuesError ? (
          <div className="dashboard-empty-state">
            <h4>Failed to Load Issue Data</h4>
            <p>An error occurred while retrieving issue data from the server.</p>
            <Button size="sm" kind="tertiary" onClick={() => refetchIssues()}>
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
            {hasActiveFilters && (
              <Button size="sm" kind="tertiary" onClick={handleResetFilters}>
                Reset Filters
              </Button>
            )}
          </div>
        ) : (
          <div className="summary-table-container">
            <DataTable rows={displayedRows} headers={tableHeaders}>
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
                      <TableExpandHeader
                        {...getExpandHeaderProps()}
                      />
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
                          <TableExpandRow
                            key={rowKey}
                            {...rowProps}
                          >
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
                                      <div className="share-bar-bg">
                                        <div
                                          className="share-bar-fill"
                                          style={{ width: `${Math.min(100, pct)}%` }}
                                        />
                                      </div>
                                      <span className="share-percent-text">
                                        {pct.toFixed(1)}%
                                      </span>
                                    </div>
                                  </TableCell>
                                );
                              }

                              return <TableCell key={cell.id}>{cell.value ?? "-"}</TableCell>;
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
                                      District & Facility Breakdown for {summaryItem.region} Region
                                    </h4>
                                    <p style={{ fontSize: "0.8125rem", color: "#525252", margin: "0.25rem 0 0" }}>
                                      {summaryItem.districts.length} district(s) |{" "}
                                      {summaryItem.districts.reduce((acc, d) => acc + d.facilities.length, 0)} facility(ies) affected |{" "}
                                      <strong>{summaryItem.total}</strong> total issues (
                                      {summaryItem.critical > 0 && (
                                        <>
                                          <span style={{ color: "#a2191f", fontWeight: 600 }}>
                                            {summaryItem.critical} Critical
                                          </span>
                                          {", "}
                                        </>
                                      )}
                                      <span style={{ color: "#da1e28", fontWeight: 600 }}>
                                        {summaryItem.open} Open
                                      </span>
                                      ,{" "}
                                      <span style={{ color: "#198038", fontWeight: 600 }}>
                                        {summaryItem.resolved} Resolved
                                      </span>
                                      )
                                    </p>
                                  </div>

                                  <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                                    <Button
                                      size="sm"
                                      kind="ghost"
                                      onClick={() => toggleAllDistrictsInRegion(summaryItem.region, summaryItem.districts)}
                                    >
                                      {isEveryDistrictExpanded(summaryItem.region, summaryItem.districts)
                                        ? "Collapse All Facilities"
                                        : "Expand All Facilities"}
                                    </Button>

                                    <Button
                                      size="sm"
                                      kind="tertiary"
                                      renderIcon={ArrowRight}
                                      onClick={() => handleNavigateToIssues(summaryItem.region)}
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
                                        <th style={{ width: "4rem", textAlign: "center" }}>Actions</th>
                                      </tr>
                                    </thead>
                                    <tbody>
                                      {summaryItem.districts.map((d) => {
                                        const expanded = isDistrictExpanded(summaryItem.region, d);

                                        return (
                                          <Fragment key={d.district}>
                                            <tr className={`district-row ${expanded ? "district-row--expanded" : ""}`}>
                                              <td>
                                                <button
                                                  type="button"
                                                  aria-expanded={expanded}
                                                  className="district-expand-btn"
                                                  onClick={() => toggleDistrict(summaryItem.region, d)}
                                                  title={expanded ? "Collapse facility breakdown" : "Expand facility breakdown"}
                                                >
                                                  {expanded ? (
                                                    <ChevronDown size={16} className="district-expand-icon" />
                                                  ) : (
                                                    <ChevronRight size={16} className="district-expand-icon" />
                                                  )}
                                                  <span className="district-name-text">{d.district}</span>
                                                  <span className="district-facility-badge">
                                                    {d.facilities.length} {d.facilities.length === 1 ? "facility" : "facilities"}
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
                                                  onClick={() => handleNavigateToIssues(d.district)}
                                                />
                                              </td>
                                            </tr>

                                            {expanded && (
                                              <tr className="facility-breakdown-row">
                                                <td colSpan={6}>
                                                  <div className="facility-breakdown-panel">
                                                    <div className="facility-breakdown-header">
                                                      <span className="facility-breakdown-title">
                                                        Facilities in {d.district} ({d.facilities.length})
                                                      </span>
                                                    </div>
                                                    {d.facilities.length === 0 ? (
                                                      <p style={{ margin: "0.5rem 0", color: "#6f6f6f", fontSize: "0.8125rem" }}>
                                                        No health facilities recorded for this district.
                                                      </p>
                                                    ) : (
                                                      <table className="facility-table">
                                                        <thead>
                                                          <tr>
                                                            <th>Health Facility / Org Unit</th>
                                                            <th>Critical Issues</th>
                                                            <th>Open</th>
                                                            <th>Resolved</th>
                                                            <th>Total Issues</th>
                                                            <th style={{ width: "4rem", textAlign: "center" }}>Actions</th>
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
                                                                  {f.critical > 0 ? f.critical : "-"}
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
                                                                  onClick={() => handleNavigateToIssues(f.facility)}
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

                    {/* ================= Grand Total Summary Row ================= */}
                    <TableRow className="grand-total-row">
                      <TableCell />
                      <TableCell>
                        <strong>TOTAL (All Regions)</strong>
                      </TableCell>
                      <TableCell>
                        <span
                          className={`severity-pill ${
                            grandTotals.critical > 0
                              ? "severity-pill--critical"
                              : "severity-pill--zero"
                          }`}
                        >
                          <strong>{grandTotals.critical}</strong>
                        </span>
                      </TableCell>
                      <TableCell>
                        <span className="stat-cell">
                          <span className="stat-dot stat-dot--open" />
                          <strong>{grandTotals.open}</strong>
                        </span>
                      </TableCell>
                      <TableCell>
                        <span className="stat-cell">
                          <span className="stat-dot stat-dot--resolved" />
                          <strong>{grandTotals.resolved}</strong>
                        </span>
                      </TableCell>
                      <TableCell>
                        <span className="total-pill">{grandTotals.total}</span>
                      </TableCell>
                      <TableCell>
                        <span style={{ fontSize: "0.8125rem", fontWeight: 700 }}>
                          100.0%
                        </span>
                      </TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              )}
            </DataTable>
          </div>
        )}
      </div>
    </PermissionGuard>
  );
};

export default IssueDashboard;
