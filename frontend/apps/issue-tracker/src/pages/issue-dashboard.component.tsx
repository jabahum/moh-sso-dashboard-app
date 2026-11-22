import { useEffect, useMemo, useRef, useState } from "react";
import {
  Button,
  ComboBox,
  Dropdown,
  InlineLoading,
  Popover,
  PopoverContent,
} from "@carbon/react";
import { ChevronDown, Filter } from "@carbon/react/icons";
import {
  GroupedBarChart,
  PieChart,
  SimpleBarChart,
} from "@carbon/charts-react";
import { Alignments, ScaleTypes, ToolbarControlTypes } from "@carbon/charts";
import "@carbon/charts/styles.css";
import { useNavigate } from "react-router-dom";
import { PermissionGuard, PERMISSIONS } from "@moh-sso/auth";
import { PageShell } from "@moh-sso/ui";

import {
  useGetIssueDataSetsQuery,
  useGetIssueGeoJsonQuery,
  useGetIssueOrgHierarchyQuery,
  useGetIssuesQuery,
  useGetIssuesSummaryByProgramQuery,
} from "../api";
import {
  IssueChoroplethMap,
  normalizeDistrictKey,
  type DistrictMapMetric,
} from "../component/issue-choropleth-map.component";
import { DashboardKpiGrid } from "../component/dashboard-kpi-grid.component";
import { DashboardAgingTab } from "../component/dashboard-aging-tab.component";
import { DashboardRegionalTable } from "../component/dashboard-regional-table.component";
import type { Issue, PeriodOption, SelectEvent } from "../types";
import { periodType } from "../lib/constants";
import { fetchAvailablePeriods } from "../lib/periodUtils";
import {
  aggregateIssuesByRegion,
  buildOrgHierarchyMaps,
  buildUnresolvedAgingIssues,
  doesIssueMatchLocationFilter,
  doesIssueMatchReportingPeriod,
  safeLower,
} from "../lib/issueAggregation";

import "./issue-dashboard.scss";

const DEFAULT_CHART_REGION = "All Districts";

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
  });

  const {
    data: programIssuesData,
    isFetching: isFetchingProgramIssues,
    refetch: refetchProgramIssues,
  } = useGetIssuesQuery(
    {
      limit: 10000,
      offset: 0,
      program: selectedProgram,
    },
    { skip: !selectedProgram },
  );

  const { data: summaryPrograms = [] } = useGetIssuesSummaryByProgramQuery();
  const { data: datasets = [] } = useGetIssueDataSetsQuery();
  const { data: hierarchyData = [] } = useGetIssueOrgHierarchyQuery();
  const { data: districtGeoJson, isLoading: isLoadingDistrictGeoJson } =
    useGetIssueGeoJsonQuery("geo_districts");
  const { data: waterGeoJson } = useGetIssueGeoJsonQuery("geo_water");

  /* -------------------------------------------------------------
   * Global Filters State
   * ------------------------------------------------------------- */
  const [selectedYear, setSelectedYear] = useState<number>(currentYear);
  const [isYearFilterApplied, setIsYearFilterApplied] = useState<boolean>(false);
  const [selectedPeriodType, setSelectedPeriodType] = useState<string>("Monthly");
  const [selectedPeriod, setSelectedPeriod] = useState<string>("");
  const [availablePeriods, setAvailablePeriods] = useState<PeriodOption[]>(
    fetchAvailablePeriods("Monthly", currentYear.toString()),
  );
  const [isPeriodPopoverOpen, setIsPeriodPopoverOpen] = useState<boolean>(false);
  const periodPopoverRef = useRef<HTMLDivElement>(null);

  const [selectedDataset, setSelectedDataset] = useState<string>("");
  const [selectedChartRegion, setSelectedChartRegion] =
    useState<string>(DEFAULT_CHART_REGION);
  const [regionSearchTerm, setRegionSearchTerm] = useState<string>("");

  /* Visual tabs & chart view controls */
  const [activeVisualTab, setActiveVisualTab] = useState<
    "distribution" | "aging" | "table"
  >("distribution");
  const [chartViewMode, setChartViewMode] = useState<"total" | "status">("total");

  const years = useMemo(
    () => Array.from({ length: 10 }, (_, index) => currentYear - index),
    [currentYear],
  );

  const datasetNames = useMemo(
    () => datasets.map((d) => d.display_name).filter(Boolean),
    [datasets],
  );

  const programNames = useMemo(
    () => summaryPrograms.map((p) => p.program).filter(Boolean),
    [summaryPrograms],
  );

  const isLocationFilterActive =
    Boolean(selectedChartRegion) &&
    safeLower(selectedChartRegion) !== safeLower(DEFAULT_CHART_REGION) &&
    safeLower(selectedChartRegion) !== "all regions";

  const hasActiveFilters =
    selectedYear !== currentYear ||
    isYearFilterApplied ||
    selectedPeriodType !== "Monthly" ||
    Boolean(selectedPeriod) ||
    Boolean(selectedDataset) ||
    Boolean(selectedProgram) ||
    isLocationFilterActive;

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
   * Hierarchy Maps & Unified Filtered Datasets
   * ------------------------------------------------------------- */
  const hierarchyMaps = useMemo(
    () => buildOrgHierarchyMaps(hierarchyData),
    [hierarchyData],
  );

  const baseIssues: Issue[] = useMemo(() => {
    if (selectedProgram) {
      return programIssuesData?.items ?? [];
    }
    return issuesData?.items ?? [];
  }, [selectedProgram, programIssuesData, issuesData]);

  const periodAndDatasetFilteredIssues = useMemo(() => {
    const isYearActive = isYearFilterApplied || selectedYear !== currentYear;

    return baseIssues.filter((issue) => {
      if (selectedDataset && safeLower(issue.dataset) !== safeLower(selectedDataset)) {
        return false;
      }
      if (
        !doesIssueMatchReportingPeriod(
          issue,
          selectedPeriod,
          selectedYear,
          isYearActive,
        )
      ) {
        return false;
      }
      return true;
    });
  }, [
    baseIssues,
    selectedDataset,
    selectedPeriod,
    selectedYear,
    currentYear,
    isYearFilterApplied,
  ]);

  const { regionalSummary: mapRegionalSummary } = useMemo(
    () => aggregateIssuesByRegion(periodAndDatasetFilteredIssues, hierarchyMaps),
    [periodAndDatasetFilteredIssues, hierarchyMaps],
  );

  const districtMetricsMap = useMemo(() => {
    const map = new Map<string, DistrictMapMetric>();
    for (const regionRow of mapRegionalSummary) {
      for (const d of regionRow.districts) {
        const key = normalizeDistrictKey(d.district);
        if (!key || key === "unspecified") continue;
        const existing = map.get(key);
        if (existing) {
          existing.critical += d.critical;
          existing.open += d.open;
          existing.resolved += d.resolved;
          existing.total += d.total;
          existing.facilitiesCount += d.facilities.length;
        } else {
          map.set(key, {
            district: d.district,
            region: regionRow.region,
            critical: d.critical,
            open: d.open,
            resolved: d.resolved,
            total: d.total,
            facilitiesCount: d.facilities.length,
          });
        }
      }
    }
    return map;
  }, [mapRegionalSummary]);

  const chartRegionOptions = useMemo(() => {
    const districtEntries = Array.from(districtMetricsMap.values())
      .filter((d) => d.district && safeLower(d.district) !== "unspecified")
      .sort((a, b) => a.district.localeCompare(b.district))
      .map((d) => d.district);

    const regionNames = mapRegionalSummary
      .map((r) => r.region)
      .filter((r) => r && safeLower(r) !== "unspecified")
      .sort((a, b) => a.localeCompare(b));

    const seen = new Set<string>(["all districts"]);
    const options: string[] = [DEFAULT_CHART_REGION];

    for (const rName of regionNames) {
      const k = safeLower(rName);
      if (!seen.has(k)) {
        seen.add(k);
        options.push(rName);
      }
    }

    for (const dName of districtEntries) {
      const k = safeLower(dName);
      if (!seen.has(k)) {
        seen.add(k);
        options.push(dName);
      }
    }

    return options;
  }, [districtMetricsMap, mapRegionalSummary]);

  const activeChartRegion = useMemo(() => {
    if (!selectedChartRegion || safeLower(selectedChartRegion) === "all districts") {
      return DEFAULT_CHART_REGION;
    }
    const exactMatch = chartRegionOptions.find(
      (opt) =>
        safeLower(opt) === safeLower(selectedChartRegion) ||
        normalizeDistrictKey(opt) === normalizeDistrictKey(selectedChartRegion),
    );
    return exactMatch || selectedChartRegion;
  }, [chartRegionOptions, selectedChartRegion]);

  const filteredIssues = useMemo(() => {
    if (!isLocationFilterActive) {
      return periodAndDatasetFilteredIssues;
    }
    return periodAndDatasetFilteredIssues.filter((issue) =>
      doesIssueMatchLocationFilter(issue, activeChartRegion, hierarchyMaps),
    );
  }, [
    periodAndDatasetFilteredIssues,
    isLocationFilterActive,
    activeChartRegion,
    hierarchyMaps,
  ]);

  const { regionalSummary, grandTotals: filteredGrandTotals } = useMemo(
    () => aggregateIssuesByRegion(filteredIssues, hierarchyMaps),
    [filteredIssues, hierarchyMaps],
  );

  const unresolvedAgingIssues = useMemo(
    () => buildUnresolvedAgingIssues(filteredIssues, hierarchyMaps),
    [filteredIssues, hierarchyMaps],
  );

  /* -------------------------------------------------------------
   * Charts Data & Options
   * ------------------------------------------------------------- */
  const chartToolbarConfig = useMemo(
    () => ({
      enabled: true,
      numberOfIcons: 3,
      controls: [
        { type: ToolbarControlTypes.MAKE_FULLSCREEN },
        { type: ToolbarControlTypes.SHOW_AS_DATATABLE },
        { type: ToolbarControlTypes.EXPORT_CSV },
        { type: ToolbarControlTypes.EXPORT_PNG },
        { type: ToolbarControlTypes.EXPORT_JPG },
      ],
    }),
    [],
  );

  const regionChartData = useMemo(() => {
    const sortedRegions = [...regionalSummary].sort((a, b) => b.total - a.total);
    if (chartViewMode === "status") {
      return sortedRegions.flatMap((r) => [
        { group: "Critical Issues", key: r.region, value: r.critical },
        { group: "Open Issues", key: r.region, value: r.open },
        { group: "Resolved Issues", key: r.region, value: r.resolved },
      ]);
    }
    return sortedRegions.map((r) => ({
      group: "Total Issues",
      key: r.region,
      value: r.total,
    }));
  }, [regionalSummary, chartViewMode]);

  const regionChartOptions = useMemo(
    () => ({
      title: "Issues by Region",
      axes: {
        left: {
          mapsTo: "value",
          title: "Number of Issues",
          scaleType: ScaleTypes.LINEAR,
        },
        bottom: {
          mapsTo: "key",
          title: "Region",
          scaleType: ScaleTypes.LABELS,
        },
      },
      color: {
        scale: {
          "Total Issues": "#0f62fe",
          "Critical Issues": "#a2191f",
          "Open Issues": "#da1e28",
          "Resolved Issues": "#198038",
        },
      },
      height: "480px",
      toolbar: chartToolbarConfig,
      legend: {
        enabled: chartViewMode === "status",
      },
    }),
    [chartViewMode, chartToolbarConfig],
  );

  const levelOfCareChartData = useMemo(() => {
    const careMap = new Map<string, number>();

    for (const regionRow of regionalSummary) {
      for (const d of regionRow.districts) {
        for (const f of d.facilities) {
          const careLevel = f.levelOfCare || "Other / Unspecified";
          careMap.set(careLevel, (careMap.get(careLevel) ?? 0) + f.total);
        }
      }
    }

    return Array.from(careMap.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([group, value]) => ({ group, value }));
  }, [regionalSummary]);

  const levelOfCareChartOptions = useMemo(
    () => ({
      title: isLocationFilterActive
        ? `Issues by Level of Care (${activeChartRegion})`
        : "Issues by Level of Care",
      resizable: true,
      pie: {
        alignment: Alignments.CENTER,
      },
      height: "480px",
      toolbar: chartToolbarConfig,
      legend: {
        enabled: true,
        alignment: Alignments.CENTER,
      },
    }),
    [isLocationFilterActive, activeChartRegion, chartToolbarConfig],
  );

  /* -------------------------------------------------------------
   * Filter & Navigation Handlers
   * ------------------------------------------------------------- */
  const handleYearChange = ({ selectedItem }: SelectEvent<number>) => {
    if (selectedItem == null) return;
    setSelectedYear(selectedItem);
    setIsYearFilterApplied(true);
    setAvailablePeriods(
      fetchAvailablePeriods(selectedPeriodType, selectedItem.toString()),
    );
    setSelectedPeriod("");
  };

  const handlePeriodTypeChange = ({
    selectedItem,
  }: SelectEvent<{ label: string; value: string }>) => {
    if (!selectedItem) return;
    setSelectedPeriodType(selectedItem.value);
    setAvailablePeriods(
      fetchAvailablePeriods(selectedItem.value, selectedYear.toString()),
    );
    setSelectedPeriod("");
  };

  const handlePeriodChange = ({ selectedItem }: SelectEvent<PeriodOption>) => {
    setSelectedPeriod(selectedItem?.label ?? "");
  };

  const handleResetFilters = () => {
    setSelectedYear(currentYear);
    setIsYearFilterApplied(false);
    setSelectedPeriodType("Monthly");
    setSelectedPeriod("");
    setAvailablePeriods(fetchAvailablePeriods("Monthly", currentYear.toString()));
    setSelectedDataset("");
    setSelectedProgram(undefined);
    setSelectedChartRegion(DEFAULT_CHART_REGION);
    setRegionSearchTerm("");
    setIsPeriodPopoverOpen(false);
  };

  const handleNavigateToRegion = (regionName?: string) => {
    if (regionName && !regionName.toLowerCase().startsWith("unspecified")) {
      navigate(`/issues?region=${encodeURIComponent(regionName)}`);
    } else {
      navigate("/issues");
    }
  };

  const handleNavigateToDistrict = (districtName?: string) => {
    if (districtName && !districtName.toLowerCase().startsWith("unspecified")) {
      navigate(`/issues?district=${encodeURIComponent(districtName)}`);
    } else {
      navigate("/issues");
    }
  };

  const handleNavigateToFacility = (facilityName?: string) => {
    if (facilityName && !facilityName.toLowerCase().startsWith("unspecified")) {
      navigate(`/issues?orgUnit=${encodeURIComponent(facilityName)}`);
    } else {
      navigate("/issues");
    }
  };

  const handleRefreshAll = () => {
    refetchIssues();
    if (selectedProgram) {
      refetchProgramIssues();
    }
  };

  const isDataLoading =
    isLoadingIssues || (Boolean(selectedProgram) && isFetchingProgramIssues);

  const activeFilterSummary = [
    selectedPeriod && `Period: ${selectedPeriod}`,
    (isYearFilterApplied || selectedYear !== currentYear) &&
      !selectedPeriod &&
      `Year: ${selectedYear}`,
    selectedProgram && `Program: ${selectedProgram}`,
    selectedDataset && `Dataset: ${selectedDataset}`,
    isLocationFilterActive && `Location: ${activeChartRegion}`,
  ]
    .filter(Boolean)
    .join(" • ");

  return (
    <PermissionGuard permission={PERMISSIONS.issueTrackerRead}>
      <PageShell
        title="Issue Summary"
        subtitle="Aggregated overview of reported data quality issues categorized by resolution status and grouped by region."
      >
        <div className="issue-dashboard-page">
          {/* ================= Unified Global Filter Bar ================= */}
          <div
            className="dashboard-filter-container"
            role="region"
            aria-label="Dashboard global filters"
          >
            <div ref={periodPopoverRef} className="issue-filter-wrapper">
              <Popover open={isPeriodPopoverOpen} align="bottom-left" dropShadow>
                <Button
                  size="sm"
                  kind="tertiary"
                  renderIcon={ChevronDown}
                  onClick={() => setIsPeriodPopoverOpen((prev) => !prev)}
                >
                  {selectedPeriod
                    ? `Period: ${selectedPeriod}`
                    : isYearFilterApplied || selectedYear !== currentYear
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
                      selectedItem={periodType.find(
                        (item) => item.value === selectedPeriodType,
                      )}
                      itemToString={(item) => item?.label ?? ""}
                      onChange={handlePeriodTypeChange}
                    />

                    <ComboBox
                      id="dashboard-filter-period"
                      titleText="Period"
                      placeholder="Search or select period..."
                      items={availablePeriods}
                      selectedItem={
                        availablePeriods.find(
                          (item) => item.label === selectedPeriod,
                        ) ?? null
                      }
                      itemToString={(item) => item?.label ?? ""}
                      shouldFilterItem={({ item, inputValue }) => {
                        if (!inputValue) return true;
                        const query = inputValue.toLowerCase().trim();
                        const label = (item?.label ?? "").toLowerCase();
                        const id = (item?.id ?? "").toLowerCase();
                        return label.includes(query) || id.includes(query);
                      }}
                      onChange={handlePeriodChange}
                    />

                    <div className="popover-footer">
                      <Button
                        size="sm"
                        kind="ghost"
                        onClick={() => {
                          setSelectedPeriod("");
                          setIsYearFilterApplied(false);
                          setSelectedYear(currentYear);
                          setAvailablePeriods(
                            fetchAvailablePeriods(
                              selectedPeriodType,
                              currentYear.toString(),
                            ),
                          );
                          setIsPeriodPopoverOpen(false);
                        }}
                      >
                        Clear
                      </Button>
                      <Button
                        size="sm"
                        onClick={() => {
                          if (!selectedPeriod) {
                            setIsYearFilterApplied(true);
                          }
                          setIsPeriodPopoverOpen(false);
                        }}
                      >
                        Apply
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>

            {programNames.length > 0 && (
              <div className="dashboard-filter-field">
                <ComboBox
                  id="dashboard-program-filter"
                  size="sm"
                  titleText=""
                  aria-label="Filter by Program"
                  placeholder="Filter by Program"
                  items={programNames}
                  selectedItem={selectedProgram || null}
                  shouldFilterItem={({ item, inputValue }) =>
                    !inputValue ||
                    String(item ?? "")
                      .toLowerCase()
                      .includes(inputValue.toLowerCase().trim())
                  }
                  onChange={({ selectedItem }: SelectEvent<string>) =>
                    setSelectedProgram(selectedItem ?? undefined)
                  }
                />
              </div>
            )}

            <div className="dashboard-filter-field">
              <ComboBox
                id="dashboard-dataset-filter"
                size="sm"
                titleText=""
                aria-label="Filter by Dataset"
                placeholder="Filter by Dataset"
                items={datasetNames}
                selectedItem={selectedDataset || null}
                shouldFilterItem={({ item, inputValue }) =>
                  !inputValue ||
                  String(item ?? "")
                    .toLowerCase()
                    .includes(inputValue.toLowerCase().trim())
                }
                onChange={({ selectedItem }: SelectEvent<string>) =>
                  setSelectedDataset(selectedItem ?? "")
                }
              />
            </div>

            <div className="dashboard-filter-field">
              <ComboBox
                id="chart-region-filter-dropdown"
                size="sm"
                titleText=""
                aria-label="Filter by Region or District"
                placeholder={DEFAULT_CHART_REGION}
                items={chartRegionOptions}
                selectedItem={
                  activeChartRegion === DEFAULT_CHART_REGION
                    ? null
                    : activeChartRegion
                }
                shouldFilterItem={({ item, inputValue }) =>
                  !inputValue ||
                  String(item ?? "")
                    .toLowerCase()
                    .includes(inputValue.toLowerCase().trim())
                }
                onChange={({ selectedItem }: SelectEvent<string>) =>
                  setSelectedChartRegion(selectedItem || DEFAULT_CHART_REGION)
                }
              />
            </div>

            <Button
              size="sm"
              kind="ghost"
              renderIcon={Filter}
              disabled={!hasActiveFilters}
              onClick={handleResetFilters}
            >
              Reset Filters
            </Button>

            {hasActiveFilters && activeFilterSummary && (
              <span className="filter-active-indicator">
                {activeFilterSummary}
              </span>
            )}
          </div>

          {/* ================= KPI Metric Cards ================= */}
          <DashboardKpiGrid
            grandTotals={filteredGrandTotals}
            isLoading={isDataLoading}
          />

          {/* ================= Visual Issue Distribution & Tabbed Views ================= */}
          <div className="dashboard-charts-section">
            <div className="dashboard-charts-header">
              <div>
                <h3 className="dashboard-charts-title">
                  Visual Issue Distribution
                </h3>
                <p className="dashboard-charts-subtitle">
                  Comparison of reported issues across districts, regions, days
                  open, and health facilities.
                </p>
              </div>

              {activeVisualTab !== "table" && (
                <div className="dashboard-charts-controls">
                  <div
                    className="chart-mode-toggle"
                    role="group"
                    aria-label="Chart metric mode"
                  >
                    <Button
                      size="sm"
                      kind={chartViewMode === "total" ? "primary" : "tertiary"}
                      onClick={() => setChartViewMode("total")}
                    >
                      Total Issues
                    </Button>
                    <Button
                      size="sm"
                      kind={chartViewMode === "status" ? "primary" : "tertiary"}
                      onClick={() => setChartViewMode("status")}
                    >
                      By Status
                    </Button>
                  </div>
                </div>
              )}
            </div>

            <div
              className="dashboard-visual-tabs"
              role="tablist"
              aria-label="Issue dashboard visual tabs"
            >
              <button
                type="button"
                role="tab"
                aria-selected={activeVisualTab === "distribution"}
                className={`dashboard-visual-tab ${
                  activeVisualTab === "distribution" ? "active" : ""
                }`}
                onClick={() => setActiveVisualTab("distribution")}
              >
                <span>Geographic Issue Distribution</span>
                <span className="dashboard-visual-tab-badge">
                  {filteredGrandTotals.total}
                </span>
              </button>

              <button
                type="button"
                role="tab"
                aria-selected={activeVisualTab === "aging"}
                className={`dashboard-visual-tab ${
                  activeVisualTab === "aging" ? "active" : ""
                }`}
                onClick={() => setActiveVisualTab("aging")}
              >
                <span>Unresolved Issue Aging</span>
                <span className="dashboard-visual-tab-badge dashboard-visual-tab-badge--alert">
                  {unresolvedAgingIssues.length}
                </span>
              </button>

              <button
                type="button"
                role="tab"
                aria-selected={activeVisualTab === "table"}
                className={`dashboard-visual-tab ${
                  activeVisualTab === "table" ? "active" : ""
                }`}
                onClick={() => setActiveVisualTab("table")}
              >
                <span>Regional Summary Table</span>
                <span className="dashboard-visual-tab-badge">
                  {regionalSummary.length}
                </span>
              </button>
            </div>

            {activeVisualTab === "distribution" && (
              <div className="dashboard-visual-tab-panel" role="tabpanel">
                {isDataLoading ? (
                  <div className="dashboard-loading-state">
                    <InlineLoading
                      status="active"
                      description="Loading issue distribution visuals..."
                    />
                  </div>
                ) : issuesError ? (
                  <div className="dashboard-empty-state">
                    <h4>Failed to Load Issue Data</h4>
                    <p>
                      An error occurred while retrieving issue data from the
                      server.
                    </p>
                    <Button size="sm" kind="tertiary" onClick={handleRefreshAll}>
                      Retry
                    </Button>
                  </div>
                ) : (
                  <>
                    <div className="dashboard-chart-card dashboard-chart-card--full dashboard-chart-card--map">
                      <IssueChoroplethMap
                        geoJson={districtGeoJson}
                        waterGeoJson={waterGeoJson}
                        isLoadingGeoJson={isLoadingDistrictGeoJson}
                        districtMetricsMap={districtMetricsMap}
                        selectedFilter={activeChartRegion}
                        chartViewMode={chartViewMode}
                        onSelectFilter={(district) =>
                          setSelectedChartRegion(
                            district || DEFAULT_CHART_REGION,
                          )
                        }
                        onNavigateToDistrict={handleNavigateToDistrict}
                      />
                    </div>

                    <div className="dashboard-charts-grid">
                      <div className="dashboard-chart-card">
                        {chartViewMode === "status" ? (
                          <GroupedBarChart
                            data={regionChartData}
                            options={regionChartOptions}
                          />
                        ) : (
                          <SimpleBarChart
                            data={regionChartData}
                            options={regionChartOptions}
                          />
                        )}
                      </div>

                      <div className="dashboard-chart-card">
                        <PieChart
                          data={levelOfCareChartData}
                          options={levelOfCareChartOptions}
                        />
                      </div>
                    </div>
                  </>
                )}
              </div>
            )}

            {activeVisualTab === "aging" && (
              <div className="dashboard-visual-tab-panel" role="tabpanel">
                {isDataLoading ? (
                  <div className="dashboard-loading-state">
                    <InlineLoading
                      status="active"
                      description="Loading unresolved issue aging..."
                    />
                  </div>
                ) : issuesError ? (
                  <div className="dashboard-empty-state">
                    <h4>Failed to Load Issue Data</h4>
                    <p>
                      An error occurred while retrieving issue data from the
                      server.
                    </p>
                    <Button size="sm" kind="tertiary" onClick={handleRefreshAll}>
                      Retry
                    </Button>
                  </div>
                ) : (
                  <DashboardAgingTab
                    unresolvedAgingIssues={unresolvedAgingIssues}
                    chartViewMode={chartViewMode}
                    activeLocationFilter={activeChartRegion}
                    defaultLocationLabel={DEFAULT_CHART_REGION}
                    chartToolbarConfig={chartToolbarConfig}
                    hasActiveFilters={hasActiveFilters}
                    onResetFilters={handleResetFilters}
                    onNavigateToRegion={handleNavigateToRegion}
                    onNavigateToDistrict={handleNavigateToDistrict}
                    onNavigateToFacility={handleNavigateToFacility}
                  />
                )}
              </div>
            )}

            {activeVisualTab === "table" && (
              <div className="dashboard-visual-tab-panel" role="tabpanel">
                <DashboardRegionalTable
                  regionalSummary={regionalSummary}
                  filteredGrandTotals={filteredGrandTotals}
                  regionSearchTerm={regionSearchTerm}
                  onRegionSearchChange={setRegionSearchTerm}
                  isLoading={isDataLoading}
                  hasError={Boolean(issuesError)}
                  hasActiveFilters={hasActiveFilters}
                  onRetry={handleRefreshAll}
                  onResetFilters={handleResetFilters}
                  onNavigateToRegion={handleNavigateToRegion}
                  onNavigateToDistrict={handleNavigateToDistrict}
                  onNavigateToFacility={handleNavigateToFacility}
                />
              </div>
            )}
          </div>
        </div>
      </PageShell>
    </PermissionGuard>
  );
};

export default IssueDashboard;
