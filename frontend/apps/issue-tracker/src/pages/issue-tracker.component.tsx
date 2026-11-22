import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  Button,
  ComboBox,
  Dropdown,
  InlineLoading,
  Popover,
  PopoverContent,
  Search,
  Tab,
  TabList,
  Tabs,
  TreeView,
} from "@carbon/react";
import {
  Add,
  ChevronDown,
  Download,
  Filter,
  List,
  Upload,
  User,
} from "@carbon/react/icons";
import * as XLSX from "xlsx";
import { useSelector } from "react-redux";
import { useLocation, useSearchParams } from "react-router-dom";
import { PERMISSIONS, PermissionGuard, selectUser } from "@moh-sso/auth";
import { PageShell } from "@moh-sso/ui";

import {
  useGetIssueByCodeQuery,
  useGetIssueDataSetsQuery,
  useGetIssueOrgHierarchyQuery,
  useGetIssuesQuery,
  useGetIssuesSummaryByProgramQuery,
  useLazyGetIssueDataSetElementsQuery,
  useLazyGetIssuesQuery,
} from "../api";
import { IssuesDataTable } from "../component/issues-data-table.component";
import { IssueModal } from "../component/issue-modal.component";
import { ImportIssuesModal } from "../component/import-issues-modal.component";
import { AssignModal } from "../component/assign-modal.component";
import { OrgUnitNode, type OrgUnitNodeData } from "../component/tree-node.component";
import {
  headers,
  IMPORT_TEMPLATE_EXAMPLE_ROW,
  IMPORT_TEMPLATE_HEADERS,
  periodType,
} from "../lib/constants";
import {
  formatReportingPeriod,
  getAvailablePeriods,
} from "../lib/periodUtils";
import IssueDetail from "./issue-detail/issue-detail.component";
import type {
  Issue,
  PeriodOption,
  SelectEvent,
  ThemeElement,
} from "../types";

export {
  formatReportingPeriod,
  formatReportingPeriod as convertReportingPeriod,
} from "../lib/periodUtils";
export type { Issue } from "../types";

import "./issue-tracker.scss";

function extractIssueCodeFromPath(pathname: string): string {
  const parts = pathname.split("/").filter(Boolean);
  const lastPart = parts[parts.length - 1];
  if (lastPart && /^HMIS-\d+$/i.test(lastPart)) {
    return lastPart.trim();
  }
  return "";
}

function normalizeIssueRows(rows: Issue[] | undefined): Issue[] {
  return (
    rows?.map((item) => {
      const formattedPeriod = formatReportingPeriod(
        item.time_Period ?? item.time_period,
      );
      return {
        ...item,
        id: String(item.issue_id),
        time_Period: formattedPeriod,
        time_period: formattedPeriod,
      };
    }) ?? []
  );
}

const EXPORT_COLUMNS: { key: string; header: string }[] = [
  { key: "program", header: "Program" },
  ...headers,
  { key: "region", header: "Region" },
  { key: "time_period", header: "Reporting Period" },
  { key: "issue_type", header: "Issue Type" },
  { key: "priority", header: "Priority" },
  { key: "severity", header: "Severity" },
  { key: "reported_by", header: "Reported By" },
];

const IssueTracker = () => {
  const user = useSelector(selectUser);
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const currentYear = new Date().getFullYear();

  const [showModal, setShowModal] = useState(false);
  const [showImportModal, setShowImportModal] = useState(false);
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [selectedRowIds, setSelectedRowIds] = useState<string[]>([]);

  const [activeTab, setActiveTab] = useState<"assigned_to_me" | "all_issues">(() => {
    const hasUrlFilter = Boolean(
      searchParams.get("region") ||
        searchParams.get("district") ||
        searchParams.get("orgUnit") ||
        searchParams.get("program") ||
        searchParams.get("view") === "issues" ||
        searchParams.get("view") === "registered-issues" ||
        searchParams.get("tab") === "issues",
    );
    return hasUrlFilter ? "all_issues" : "assigned_to_me";
  });

  const [selectedIssue, setSelectedIssue] = useState<Issue>();
  const [isViewIssueDetail, setIsViewIssueDetail] = useState(false);

  const [urlIssueCode, setUrlIssueCode] = useState<string>(() => {
    const codeParam =
      searchParams.get("issueCode") ||
      searchParams.get("issue") ||
      searchParams.get("code");
    if (codeParam && codeParam.trim()) return codeParam.trim();
    return extractIssueCodeFromPath(location.pathname);
  });

  /*
   * Filter State
   */
  const [tableSearchTerm, setTableSearchTerm] = useState("");
  const [selectedProgram, setSelectedProgram] = useState<string | undefined>(() => {
    const prog = searchParams.get("program");
    return prog ? prog.trim() : undefined;
  });

  const [selectedYear, setSelectedYear] = useState(currentYear);
  const [selectedPeriodType, setSelectedPeriodType] = useState("Monthly");
  const [selectedPeriod, setSelectedPeriod] = useState("");
  const [availablePeriods, setAvailablePeriods] = useState<PeriodOption[]>(() =>
    getAvailablePeriods("Monthly", currentYear.toString()),
  );
  const [isPeriodPopoverOpen, setIsPeriodPopoverOpen] = useState(false);
  const periodPopoverRef = useRef<HTMLDivElement>(null);

  const [selectedDataset, setSelectedDataset] = useState("");
  const [selectedDataElement, setSelectedDataElement] = useState("");
  const [dataElements, setDataElements] = useState<ThemeElement[]>([]);
  const [isDataPopoverOpen, setIsDataPopoverOpen] = useState(false);
  const dataPopoverRef = useRef<HTMLDivElement>(null);

  const [selectedRegion, setSelectedRegion] = useState<string>(
    () => searchParams.get("region")?.trim() || "",
  );
  const [selectedDistrict, setSelectedDistrict] = useState<string>(
    () => searchParams.get("district")?.trim() || "",
  );
  const [selectedOrgUnit, setSelectedOrgUnit] = useState<string>(
    () => searchParams.get("orgUnit")?.trim() || "",
  );
  const [orgSearchTerm, setOrgSearchTerm] = useState("");
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());
  const [isOrgPopoverOpen, setIsOrgPopoverOpen] = useState(false);
  const orgPopoverRef = useRef<HTMLDivElement>(null);

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  /* Sync URL Search Params on Navigation (e.g. from Issue Summary Dashboard) */
  useEffect(() => {
    const codeParam =
      searchParams.get("issueCode") ||
      searchParams.get("issue") ||
      searchParams.get("code") ||
      extractIssueCodeFromPath(location.pathname);

    if (codeParam && codeParam.trim()) {
      setUrlIssueCode(codeParam.trim());
    }

    const regionParam = searchParams.get("region")?.trim() ?? "";
    const districtParam = searchParams.get("district")?.trim() ?? "";
    const orgUnitParam = searchParams.get("orgUnit")?.trim() ?? "";
    const programParam = searchParams.get("program")?.trim();
    const viewParam = searchParams.get("view") || searchParams.get("tab");

    if (regionParam) setSelectedRegion(regionParam);
    if (districtParam) setSelectedDistrict(districtParam);
    if (orgUnitParam) setSelectedOrgUnit(orgUnitParam);
    if (programParam) setSelectedProgram(programParam);

    if (
      regionParam ||
      districtParam ||
      orgUnitParam ||
      programParam ||
      viewParam === "issues" ||
      viewParam === "registered-issues" ||
      viewParam === "dashboard"
    ) {
      setActiveTab("all_issues");
    }
  }, [searchParams, location.pathname]);

  /*
   * API Hooks
   */
  const { data: directIssueData, isLoading: isLoadingDirectIssue } =
    useGetIssueByCodeQuery(urlIssueCode, { skip: !urlIssueCode });

  useEffect(() => {
    if (directIssueData && urlIssueCode) {
      setSelectedIssue(directIssueData);
      setIsViewIssueDetail(true);
    }
  }, [directIssueData, urlIssueCode]);

  const [triggerGetIssues] = useLazyGetIssuesQuery();
  const { data: summaryData = [] } = useGetIssuesSummaryByProgramQuery();
  const { data: datasets = [] } = useGetIssueDataSetsQuery();
  const [triggerGetDataSetElements] = useLazyGetIssueDataSetElementsQuery();
  const { data: hierarchyData = [] } = useGetIssueOrgHierarchyQuery();

  const programOptions = useMemo(() => {
    const names = summaryData
      .map((item) => item.program || "Unspecified")
      .filter(Boolean);
    return Array.from(new Set(names));
  }, [summaryData]);

  const years = useMemo(
    () => Array.from({ length: 10 }, (_, index) => currentYear - index),
    [currentYear],
  );

  const datasetNames = useMemo(
    () => datasets.map((dataset) => dataset.display_name).filter(Boolean),
    [datasets],
  );

  const dataElementNames = useMemo(
    () =>
      dataElements
        .map((element) => element.data_element_short_name ?? "")
        .filter(Boolean),
    [dataElements],
  );

  const hasActiveFilters =
    selectedYear !== currentYear ||
    selectedPeriodType !== "Monthly" ||
    Boolean(selectedPeriod) ||
    Boolean(selectedDataset) ||
    Boolean(selectedDataElement) ||
    Boolean(selectedRegion) ||
    Boolean(selectedDistrict) ||
    Boolean(selectedOrgUnit) ||
    Boolean(selectedProgram);

  useEffect(() => {
    setPage(1);
  }, [
    selectedProgram,
    selectedYear,
    selectedPeriod,
    selectedDataset,
    selectedDataElement,
    selectedRegion,
    selectedDistrict,
    selectedOrgUnit,
    tableSearchTerm,
    activeTab,
  ]);

  const userIdentifier = user?.email || user?.username || user?.id || "";

  const effectivePeriod = useMemo(() => {
    if (selectedPeriod) return selectedPeriod;
    if (selectedYear !== currentYear) return String(selectedYear);
    return undefined;
  }, [selectedPeriod, selectedYear, currentYear]);

  const queryParams = useMemo(() => {
    return {
      limit: pageSize,
      offset: (page - 1) * pageSize,
      program: selectedProgram,
      region: selectedRegion || undefined,
      district: selectedDistrict || undefined,
      orgUnit: selectedOrgUnit || undefined,
      dataset: selectedDataset || undefined,
      dataElement: selectedDataElement || undefined,
      period: effectivePeriod,
      assignedTo: activeTab === "assigned_to_me" ? userIdentifier : undefined,
      search: tableSearchTerm.trim() || undefined,
    };
  }, [
    pageSize,
    page,
    selectedProgram,
    selectedRegion,
    selectedDistrict,
    selectedOrgUnit,
    selectedDataset,
    selectedDataElement,
    effectivePeriod,
    activeTab,
    userIdentifier,
    tableSearchTerm,
  ]);

  const { data, isLoading, error } = useGetIssuesQuery(queryParams);

  const { data: assignedData } = useGetIssuesQuery(
    {
      limit: 1,
      offset: 0,
      assignedTo: userIdentifier,
    },
    { skip: !userIdentifier },
  );

  const assignedToMeCount = assignedData?.totalCount ?? 0;

  const allIssuesCount = useMemo(() => {
    if (summaryData.length > 0) {
      return summaryData.reduce((sum, item) => sum + (item.issue_count || 0), 0);
    }
    return data?.totalCount ?? 0;
  }, [summaryData, data?.totalCount]);

  const closeAllPopovers = useCallback(() => {
    setIsPeriodPopoverOpen(false);
    setIsDataPopoverOpen(false);
    setIsOrgPopoverOpen(false);
  }, []);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Node;

      if (periodPopoverRef.current && !periodPopoverRef.current.contains(target)) {
        setIsPeriodPopoverOpen(false);
      }
      if (dataPopoverRef.current && !dataPopoverRef.current.contains(target)) {
        setIsDataPopoverOpen(false);
      }
      if (orgPopoverRef.current && !orgPopoverRef.current.contains(target)) {
        setIsOrgPopoverOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, []);

  useEffect(() => {
    if (error) {
      console.error("Error encountered while fetching issues:", error);
    }
  }, [error]);

  const issues = useMemo(() => normalizeIssueRows(data?.items), [data?.items]);

  /* -----------------------------
   * Excel template & export
   * ----------------------------- */
  const downloadTemplate = () => {
    const workbook = XLSX.utils.book_new();
    const worksheet = XLSX.utils.aoa_to_sheet([
      IMPORT_TEMPLATE_HEADERS,
      IMPORT_TEMPLATE_EXAMPLE_ROW,
    ]);
    worksheet["!cols"] = IMPORT_TEMPLATE_HEADERS.map(() => ({ wch: 22 }));
    XLSX.utils.book_append_sheet(workbook, worksheet, "Issues Template");
    XLSX.writeFile(workbook, "issue-import-template.xlsx");
  };

  const downloadIssues = async () => {
    try {
      const allIssuesResponse = await triggerGetIssues({
        limit: 10000,
        offset: 0,
        program: selectedProgram,
        region: selectedRegion || undefined,
        district: selectedDistrict || undefined,
        orgUnit: selectedOrgUnit || undefined,
        dataset: selectedDataset || undefined,
        dataElement: selectedDataElement || undefined,
        period: effectivePeriod,
        assignedTo: activeTab === "assigned_to_me" ? userIdentifier : undefined,
        search: tableSearchTerm.trim() || undefined,
      }).unwrap();

      const issuesToDownload = normalizeIssueRows(allIssuesResponse.items);
      const headerRow = EXPORT_COLUMNS.map((col) => col.header);

      const dataRows = issuesToDownload.map((issue) =>
        EXPORT_COLUMNS.map((col) => {
          if (col.key === "program") {
            const rawProgram = issue.program;
            if (typeof rawProgram === "string" && rawProgram.trim()) {
              return rawProgram.trim();
            }
            if (selectedProgram !== undefined) {
              return selectedProgram || "Unspecified";
            }
            return "Unspecified";
          }
          if (col.key === "time_period") {
            return issue.time_period ?? issue.time_Period ?? "";
          }
          return (issue as Record<string, unknown>)[col.key] ?? "";
        }),
      );

      const workbook = XLSX.utils.book_new();
      const worksheet = XLSX.utils.aoa_to_sheet([headerRow, ...dataRows]);
      worksheet["!cols"] = EXPORT_COLUMNS.map(() => ({ wch: 22 }));
      XLSX.utils.book_append_sheet(workbook, worksheet, "Issues");
      XLSX.writeFile(
        workbook,
        `issues-${new Date().toISOString().slice(0, 10)}.xlsx`,
      );
    } catch (err) {
      console.error("Failed to download issues:", err);
    }
  };

  /* -----------------------------
   * Selection & Detail Handlers
   * ----------------------------- */
  const handleSelectRow = useCallback((rowId: string, checked: boolean) => {
    setSelectedRowIds((prev) => {
      if (checked) {
        return prev.includes(rowId) ? prev : [...prev, rowId];
      }
      return prev.filter((id) => id !== rowId);
    });
  }, []);

  const handleSelectAll = useCallback(
    (checked: boolean) => {
      if (checked) {
        const allIds = issues.map((item) => String(item.id ?? item.issue_id));
        setSelectedRowIds(allIds);
      } else {
        setSelectedRowIds([]);
      }
    },
    [issues],
  );

  const selectedIssueObjects = useMemo(
    () =>
      issues.filter((item) =>
        selectedRowIds.includes(String(item.id ?? item.issue_id)),
      ),
    [issues, selectedRowIds],
  );

  const selectedIssueCodes = useMemo(
    () => selectedIssueObjects.map((item) => item.issue_code).filter(Boolean),
    [selectedIssueObjects],
  );

  const areAllSelectedAssigned = useMemo(
    () =>
      selectedIssueObjects.length > 0 &&
      selectedIssueObjects.every((item) => Boolean(item.assigned_to)),
    [selectedIssueObjects],
  );

  const handleIssueClick = (row: { id?: string }) => {
    const selectedItem = issues.find(
      (item) => String(item.issue_id) === String(row.id ?? ""),
    );
    if (!selectedItem) return;

    setSelectedIssue(selectedItem);
    setIsViewIssueDetail(true);

    if (selectedItem.issue_code) {
      const next = new URLSearchParams(searchParams);
      next.set("issueCode", selectedItem.issue_code);
      setSearchParams(next, { replace: true });
    }
  };

  const handleBackFromDetail = () => {
    setSelectedIssue(undefined);
    setIsViewIssueDetail(false);
    setUrlIssueCode("");
    const next = new URLSearchParams(searchParams);
    next.delete("issueCode");
    next.delete("issue");
    next.delete("code");
    setSearchParams(next, { replace: true });
  };

  /* -----------------------------
   * Filter Control Handlers
   * ----------------------------- */
  const handleYearChange = ({ selectedItem }: SelectEvent<number>) => {
    if (selectedItem == null) return;
    setSelectedYear(selectedItem);
    setAvailablePeriods(
      getAvailablePeriods(selectedPeriodType, selectedItem.toString()),
    );
    setSelectedPeriod("");
  };

  const handlePeriodTypeChange = ({
    selectedItem,
  }: SelectEvent<{ label: string; value: string }>) => {
    if (!selectedItem) return;
    setSelectedPeriodType(selectedItem.value);
    setAvailablePeriods(
      getAvailablePeriods(selectedItem.value, selectedYear.toString()),
    );
    setSelectedPeriod("");
  };

  const handlePeriodChange = ({ selectedItem }: SelectEvent<PeriodOption>) => {
    setSelectedPeriod(selectedItem?.label ?? "");
  };

  const handleDatasetChange = async ({ selectedItem }: SelectEvent<string>) => {
    const datasetName = selectedItem ?? "";
    setSelectedDataset(datasetName);
    setSelectedDataElement("");

    if (!datasetName) {
      setDataElements([]);
      return;
    }

    const dataset = datasets.find((item) => item.display_name === datasetName);
    if (!dataset) {
      setDataElements([]);
      return;
    }

    try {
      const elements = await triggerGetDataSetElements(dataset.dataset_id).unwrap();
      setDataElements(elements);
    } catch (requestError) {
      console.error("Failed to fetch data elements:", requestError);
      setDataElements([]);
    }
  };

  const nodeMatchesSearch = useCallback(
    (node: OrgUnitNodeData, term: string): boolean => {
      if (node.name.toLowerCase().includes(term.toLowerCase())) {
        return true;
      }
      return Boolean(
        node.children?.some((child) => nodeMatchesSearch(child, term)),
      );
    },
    [],
  );

  const handleOrgSelect = (name: string) => {
    setSelectedOrgUnit((prev) => (prev === name ? "" : name));
    setSelectedRegion("");
    setSelectedDistrict("");
  };

  const handleToggleNode = useCallback((nodeId: string, isExpanded: boolean) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev);
      if (isExpanded) {
        next.add(nodeId);
      } else {
        next.delete(nodeId);
      }
      return next;
    });
  }, []);

  const renderRecursive = (
    nodes: OrgUnitNodeData[],
    idPrefix = "filter-org",
  ): ReactNode[] => {
    if (!Array.isArray(nodes)) return [];

    return nodes
      .filter((node) => !orgSearchTerm || nodeMatchesSearch(node, orgSearchTerm))
      .map((node) => (
        <OrgUnitNode
          key={node.id}
          node={node}
          searchTerm={orgSearchTerm}
          selectedOrgUnit={selectedOrgUnit}
          expandedNodes={expandedNodes}
          onSelect={handleOrgSelect}
          onToggleNode={handleToggleNode}
          renderRecursive={renderRecursive}
          idPrefix={idPrefix}
        />
      ));
  };

  const handleFilter = (
    overrides: {
      period?: string;
      year?: number;
      dataset?: string;
      dataElement?: string;
      orgUnit?: string;
    } = {},
  ) => {
    closeAllPopovers();
    setPage(1);

    if (overrides.period !== undefined) setSelectedPeriod(overrides.period);
    if (overrides.year !== undefined) setSelectedYear(overrides.year);
    if (overrides.dataset !== undefined) setSelectedDataset(overrides.dataset);
    if (overrides.dataElement !== undefined) {
      setSelectedDataElement(overrides.dataElement);
    }
    if (overrides.orgUnit !== undefined) {
      setSelectedOrgUnit(overrides.orgUnit);
      if (!overrides.orgUnit) {
        setSelectedRegion("");
        setSelectedDistrict("");
      }
    }
  };

  const handleReset = () => {
    setSelectedYear(currentYear);
    setSelectedPeriodType("Monthly");
    setSelectedPeriod("");
    setAvailablePeriods(getAvailablePeriods("Monthly", currentYear.toString()));

    setSelectedDataset("");
    setSelectedDataElement("");
    setDataElements([]);

    setSelectedRegion("");
    setSelectedDistrict("");
    setSelectedOrgUnit("");
    setOrgSearchTerm("");
    setTableSearchTerm("");
    setSelectedProgram(undefined);
    setPage(1);

    const next = new URLSearchParams(searchParams);
    next.delete("region");
    next.delete("district");
    next.delete("orgUnit");
    next.delete("program");
    setSearchParams(next, { replace: true });

    closeAllPopovers();
  };

  if (isLoadingDirectIssue && urlIssueCode && !selectedIssue) {
    return (
      <PermissionGuard permission={PERMISSIONS.issueTrackerRead}>
        <div className="issue-direct-loading">
          <InlineLoading
            status="active"
            description={`Fetching issue [${urlIssueCode}] from server...`}
          />
        </div>
      </PermissionGuard>
    );
  }

  if (isViewIssueDetail && selectedIssue) {
    return (
      <PermissionGuard permission={PERMISSIONS.issueTrackerRead}>
        <IssueDetail selectedIssue={selectedIssue} goToBack={handleBackFromDetail} />
      </PermissionGuard>
    );
  }

  const activeLocationLabel =
    selectedOrgUnit || selectedDistrict || selectedRegion || "";

  const activeFilterSummary = [
    selectedProgram !== undefined &&
      `Program: ${selectedProgram || "Unspecified"}`,
    selectedPeriod && `Period: ${selectedPeriod}`,
    selectedYear !== currentYear && !selectedPeriod && `Year: ${selectedYear}`,
    selectedDataset && `Dataset: ${selectedDataset}`,
    selectedDataElement && `Element: ${selectedDataElement}`,
    selectedRegion && `Region: ${selectedRegion}`,
    selectedDistrict && `District: ${selectedDistrict}`,
    selectedOrgUnit && `Org Unit: ${selectedOrgUnit}`,
  ]
    .filter(Boolean)
    .join(" • ");

  return (
    <PermissionGuard permission={PERMISSIONS.issueTrackerRead}>
      <PageShell
        title="Registered Issues"
        subtitle="Review, filter, assign, and resolve reported data quality issues."
        actions={
          <div className="issue-toolbar-actions">
            <PermissionGuard permission={PERMISSIONS.issueTrackerAssign}>
              <Button
                size="sm"
                kind="tertiary"
                disabled={selectedIssueCodes.length === 0}
                onClick={() => setShowAssignModal(true)}
              >
                {areAllSelectedAssigned
                  ? "Re-assign Selected"
                  : "Assign Selected"}{" "}
                {selectedIssueCodes.length > 0
                  ? `(${selectedIssueCodes.length})`
                  : ""}
              </Button>
            </PermissionGuard>

            <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
              <Button
                size="sm"
                kind="ghost"
                renderIcon={Download}
                onClick={downloadTemplate}
              >
                Template
              </Button>
            </PermissionGuard>

            <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
              <Button
                size="sm"
                kind="secondary"
                renderIcon={Upload}
                onClick={() => setShowImportModal(true)}
              >
                Import Issues
              </Button>
            </PermissionGuard>

            <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
              <Button
                size="sm"
                kind="primary"
                renderIcon={Add}
                className="dwh-btn-width"
                onClick={() => setShowModal(true)}
              >
                New Issue
              </Button>
            </PermissionGuard>
          </div>
        }
      >
        <div className="issue-tracker-page">
          <div className="issue-tracker-assignments">
            <Tabs
              selectedIndex={activeTab === "assigned_to_me" ? 0 : 1}
              onChange={({ selectedIndex }) => {
                setActiveTab(
                  selectedIndex === 0 ? "assigned_to_me" : "all_issues",
                );
                setSelectedRowIds([]);
              }}
            >
              <TabList aria-label="Issue tracker view options" contained>
                <Tab renderIcon={User}>
                  Assigned to me ({assignedToMeCount})
                </Tab>
                <Tab renderIcon={List}>All Issues ({allIssuesCount})</Tab>
              </TabList>
            </Tabs>
          </div>

          <div className="issue-filter-container">
            {programOptions.length > 0 && (
              <div className="issue-filter-combobox">
                <ComboBox
                  id="issue-filter-program"
                  size="md"
                  titleText=""
                  aria-label="Filter by Program"
                  placeholder="Filter by Program"
                  items={programOptions}
                  selectedItem={
                    selectedProgram === undefined
                      ? null
                      : selectedProgram || "Unspecified"
                  }
                  shouldFilterItem={({ item, inputValue }) =>
                    !inputValue ||
                    String(item ?? "")
                      .toLowerCase()
                      .includes(inputValue.toLowerCase().trim())
                  }
                  onChange={({ selectedItem }: SelectEvent<string>) => {
                    if (!selectedItem) {
                      setSelectedProgram(undefined);
                    } else {
                      setSelectedProgram(
                        selectedItem === "Unspecified" ? "Unspecified" : selectedItem,
                      );
                    }
                  }}
                />
              </div>
            )}

            <div ref={periodPopoverRef} className="issue-filter-wrapper">
              <Popover open={isPeriodPopoverOpen} align="bottom-left" dropShadow>
                <Button
                  size="md"
                  kind="tertiary"
                  renderIcon={ChevronDown}
                  onClick={() => {
                    setIsPeriodPopoverOpen((current) => !current);
                    setIsDataPopoverOpen(false);
                    setIsOrgPopoverOpen(false);
                  }}
                >
                  {selectedPeriod ? `Period: ${selectedPeriod}` : "Period"}
                </Button>

                <PopoverContent className="filter-popover-content">
                  <div className="popover-inner">
                    <Dropdown
                      id="issue-filter-year"
                      titleText="Year"
                      label="Select year"
                      items={years}
                      selectedItem={selectedYear}
                      itemToString={(item) =>
                        item == null ? "" : String(item)
                      }
                      onChange={handleYearChange}
                    />

                    <Dropdown
                      id="issue-filter-period-type"
                      titleText="Period type"
                      label="Select period type"
                      items={periodType}
                      selectedItem={periodType.find(
                        (item) => item.value === selectedPeriodType,
                      )}
                      itemToString={(item) => item?.label ?? ""}
                      onChange={handlePeriodTypeChange}
                    />

                    <ComboBox
                      id="issue-filter-period"
                      titleText="Period"
                      placeholder="Select period"
                      items={availablePeriods}
                      selectedItem={
                        availablePeriods.find(
                          (item) => item.label === selectedPeriod,
                        ) ?? null
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
                          handleFilter({ period: "" });
                        }}
                      >
                        Clear
                      </Button>

                      <Button size="sm" onClick={() => handleFilter()}>
                        Update
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>

            <div ref={dataPopoverRef} className="issue-filter-wrapper">
              <Popover open={isDataPopoverOpen} align="bottom-left" dropShadow>
                <Button
                  size="md"
                  kind="tertiary"
                  renderIcon={ChevronDown}
                  onClick={() => {
                    setIsDataPopoverOpen((current) => !current);
                    setIsPeriodPopoverOpen(false);
                    setIsOrgPopoverOpen(false);
                  }}
                >
                  {selectedDataset ? `Data: ${selectedDataset}` : "Data"}
                </Button>

                <PopoverContent className="filter-popover-content">
                  <div className="popover-inner">
                    <ComboBox
                      id="issue-filter-dataset"
                      titleText="Dataset"
                      placeholder="Select dataset"
                      items={datasetNames}
                      selectedItem={selectedDataset || null}
                      onChange={handleDatasetChange}
                    />

                    <ComboBox
                      id="issue-filter-data-element"
                      titleText="Data element"
                      placeholder="Select data element"
                      items={dataElementNames}
                      selectedItem={selectedDataElement || null}
                      disabled={!selectedDataset || dataElementNames.length === 0}
                      onChange={({ selectedItem }: SelectEvent<string>) =>
                        setSelectedDataElement(selectedItem ?? "")
                      }
                    />

                    <div className="popover-footer">
                      <Button
                        size="sm"
                        kind="ghost"
                        onClick={() => {
                          setSelectedDataset("");
                          setSelectedDataElement("");
                          setDataElements([]);
                          handleFilter({ dataset: "", dataElement: "" });
                        }}
                      >
                        Clear
                      </Button>

                      <Button size="sm" onClick={() => handleFilter()}>
                        Update
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>

            <div ref={orgPopoverRef} className="issue-filter-wrapper">
              <Popover open={isOrgPopoverOpen} align="bottom-left" dropShadow>
                <Button
                  size="md"
                  kind="tertiary"
                  renderIcon={ChevronDown}
                  onClick={() => {
                    setIsOrgPopoverOpen((current) => !current);
                    setIsPeriodPopoverOpen(false);
                    setIsDataPopoverOpen(false);
                  }}
                >
                  {activeLocationLabel
                    ? `Location: ${activeLocationLabel}`
                    : "Organisation Unit"}
                </Button>

                <PopoverContent className="filter-popover-content">
                  <div className="popover-inner org-filter-inner">
                    <Search
                      labelText="Search organisation unit"
                      placeholder="Search..."
                      value={orgSearchTerm}
                      onChange={(event) => setOrgSearchTerm(event.target.value)}
                      size="sm"
                    />

                    <div className="org-tree-container">
                      <TreeView label="Organisation Units" hideLabel>
                        {renderRecursive(hierarchyData, "filter-org")}
                      </TreeView>
                    </div>

                    <div className="popover-footer">
                      <Button
                        size="sm"
                        kind="ghost"
                        onClick={() => {
                          setSelectedOrgUnit("");
                          setSelectedRegion("");
                          setSelectedDistrict("");
                          setOrgSearchTerm("");
                          handleFilter({ orgUnit: "" });
                        }}
                      >
                        Clear
                      </Button>

                      <Button size="sm" onClick={() => handleFilter()}>
                        Update
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>

            <Button
              size="md"
              kind="ghost"
              renderIcon={Filter}
              disabled={!hasActiveFilters}
              onClick={handleReset}
            >
              Reset Filters
            </Button>

            {hasActiveFilters && activeFilterSummary && (
              <span className="issue-active-filters-badge">
                {activeFilterSummary}
              </span>
            )}
          </div>

          <div className="issue-table-search">
            <div className="issue-table-search-bar">
              <Search
                labelText="Search issues"
                placeholder="Search by issue code, dataset, data element, issue, status, or organisation unit"
                value={tableSearchTerm}
                onChange={(event) => setTableSearchTerm(event.target.value)}
                size="lg"
                closeButtonLabelText="Clear search"
              />
              <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
                <Button
                  size="md"
                  kind="ghost"
                  renderIcon={Download}
                  onClick={downloadIssues}
                >
                  Download
                </Button>
              </PermissionGuard>
            </div>
          </div>

          {activeTab === "assigned_to_me" &&
            issues.length === 0 &&
            !isLoading && (
              <div className="issue-assigned-empty-banner">
                <strong>No issues currently assigned to you.</strong> Switch to
                the <em>All Issues</em> tab to view all registered issues.
              </div>
            )}

          <div
            className="issue-tracker-table"
            role="region"
            aria-label="Registered issues table"
            tabIndex={0}
          >
            <IssuesDataTable
              columns={headers}
              data={issues}
              totalItems={data?.totalCount ?? 0}
              currentPage={page}
              currentPageSize={pageSize}
              onPageChange={(newPage, newPageSize) => {
                setPage(newPage);
                setPageSize(newPageSize);
              }}
              handleIssueClick={handleIssueClick}
              closeView={() => {
                setSelectedIssue(undefined);
                setIsViewIssueDetail(false);
              }}
              selectedRowIds={selectedRowIds}
              onSelectRow={handleSelectRow}
              onSelectAll={handleSelectAll}
            />
          </div>

          {showModal && (
            <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
              <IssueModal onClose={() => setShowModal(false)} />
            </PermissionGuard>
          )}

          {showImportModal && (
            <PermissionGuard permission={PERMISSIONS.issueTrackerWrite}>
              <ImportIssuesModal onClose={() => setShowImportModal(false)} />
            </PermissionGuard>
          )}

          {showAssignModal && selectedIssueCodes.length > 0 && (
            <PermissionGuard permission={PERMISSIONS.issueTrackerAssign}>
              <AssignModal
                issueCodes={selectedIssueCodes}
                isReassign={areAllSelectedAssigned}
                onClose={() => setShowAssignModal(false)}
                onSuccess={() => setSelectedRowIds([])}
              />
            </PermissionGuard>
          )}
        </div>
      </PageShell>
    </PermissionGuard>
  );
};

export default IssueTracker;
