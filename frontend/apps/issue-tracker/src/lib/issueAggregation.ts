import * as XLSX from "xlsx";
import type {
  ActionableAgingIssue,
  AgingBracketCounts,
  AgingBucketDefinition,
  AgingBucketId,
  AgingDistrictSummary,
  AgingFacilitySummary,
  AgingGrandTotals,
  AgingRegionSummaryRow,
  DashboardGrandTotals,
  DistrictSummary,
  FacilitySummary,
  Issue,
  RegionSummaryRow,
} from "../types";
import {
  convertPeriodToCode,
  formatReportingPeriod,
  isPeriodMatching,
} from "./periodUtils";
import {
  normalizeDistrictKey,
  normalizeRegionKey,
} from "../component/issue-choropleth-map.component";

export const AGING_BUCKETS: AgingBucketDefinition[] = [
  {
    id: "0-7",
    label: "0–7 Days",
    shortLabel: "0–7d",
    minDays: 0,
    maxDays: 7,
    color: "#0f62fe",
    badgeClass: "aging-pill--fresh",
  },
  {
    id: "8-14",
    label: "8–14 Days",
    shortLabel: "8–14d",
    minDays: 8,
    maxDays: 14,
    color: "#007d79",
    badgeClass: "aging-pill--recent",
  },
  {
    id: "15-30",
    label: "15–30 Days",
    shortLabel: "15–30d",
    minDays: 15,
    maxDays: 30,
    color: "#d2a106",
    badgeClass: "aging-pill--aging",
  },
  {
    id: "31-60",
    label: "31–60 Days",
    shortLabel: "31–60d",
    minDays: 31,
    maxDays: 60,
    color: "#eb6200",
    badgeClass: "aging-pill--overdue",
  },
  {
    id: "61-90",
    label: "61–90 Days",
    shortLabel: "61–90d",
    minDays: 61,
    maxDays: 90,
    color: "#da1e28",
    badgeClass: "aging-pill--stale",
  },
  {
    id: "90+",
    label: "> 90 Days",
    shortLabel: "> 90d",
    minDays: 91,
    maxDays: Number.POSITIVE_INFINITY,
    color: "#a2191f",
    badgeClass: "aging-pill--critical",
  },
];

export function safeString(val: unknown): string {
  if (val == null) return "";
  return String(val).trim();
}

export function safeLower(val: unknown): string {
  return safeString(val).toLowerCase();
}

export function isIssueResolved(status?: string | null): boolean {
  if (!status) return false;
  const s = safeLower(status);
  return s === "resolved" || s === "closed" || s === "done";
}

export function isCriticalIssue(issue: Issue): boolean {
  const sev = safeLower(issue.severity);
  const prio = safeLower(issue.priority);
  return sev === "critical" || prio === "critical";
}

export function parseIssueDate(issue: Issue): Date | null {
  const candidates = [safeString(issue.date_reported), safeString(issue.updated_date)];

  for (const raw of candidates) {
    if (!raw) continue;

    const isoMatch = raw.match(/^(\d{4})-(0[1-9]|1[0-2])-(\d{2})/);
    if (isoMatch) {
      const parsed = new Date(
        Number(isoMatch[1]),
        Number(isoMatch[2]) - 1,
        Number(isoMatch[3]),
      );
      if (!Number.isNaN(parsed.getTime())) return parsed;
    }

    const dmyMatch = raw.match(/^(\d{1,2})[/-](0?[1-9]|1[0-2])[/-](\d{4})$/);
    if (dmyMatch) {
      const parsed = new Date(
        Number(dmyMatch[3]),
        Number(dmyMatch[2]) - 1,
        Number(dmyMatch[1]),
      );
      if (!Number.isNaN(parsed.getTime())) return parsed;
    }

    const direct = new Date(raw);
    if (!Number.isNaN(direct.getTime())) return direct;
  }

  const rawPeriod = safeString(issue.time_Period || issue.time_period);
  if (rawPeriod) {
    const code = convertPeriodToCode(rawPeriod);
    const mMatch = code.match(/^(\d{4})(0[1-9]|1[0-2])$/);
    if (mMatch) {
      return new Date(Number(mMatch[1]), Number(mMatch[2]) - 1, 1);
    }
    const qMatch = code.match(/^(\d{4})Q([1-4])$/i);
    if (qMatch) {
      const startMonth = (Number(qMatch[2]) - 1) * 3;
      return new Date(Number(qMatch[1]), startMonth, 1);
    }
    const yMatch = rawPeriod.match(/\b(19\d\d|20\d\d)\b/);
    if (yMatch) {
      return new Date(Number(yMatch[1]), 0, 1);
    }
  }

  return null;
}

export function getAgingBucketForDays(ageDays: number): AgingBucketDefinition {
  for (const bucket of AGING_BUCKETS) {
    if (ageDays >= bucket.minDays && ageDays <= bucket.maxDays) {
      return bucket;
    }
  }
  return AGING_BUCKETS[AGING_BUCKETS.length - 1];
}

export function doesIssueMatchReportingPeriod(
  issue: Issue,
  selectedPeriod: string,
  selectedYear: number,
  isYearFilterActive: boolean,
): boolean {
  const rawPeriod = safeString(issue.time_Period || issue.time_period);

  if (selectedPeriod) {
    if (!rawPeriod) return false;

    if (isPeriodMatching(selectedPeriod, rawPeriod, selectedYear)) {
      return true;
    }

    const formattedTarget = safeLower(formatReportingPeriod(selectedPeriod));
    const formattedIssue = safeLower(formatReportingPeriod(rawPeriod));
    if (formattedTarget && formattedIssue && formattedTarget === formattedIssue) {
      return true;
    }

    const targetCode = convertPeriodToCode(selectedPeriod, selectedYear).toUpperCase();
    const issueCode = convertPeriodToCode(rawPeriod, selectedYear).toUpperCase();

    const qMatch = targetCode.match(/^(\d{4})Q([1-4])$/);
    if (qMatch) {
      const targetYear = qMatch[1];
      const targetQuarter = Number(qMatch[2]);

      const mMatch = issueCode.match(/^(\d{4})(0[1-9]|1[0-2])$/);
      if (mMatch && mMatch[1] === targetYear) {
        const issueQuarter = Math.ceil(Number(mMatch[2]) / 3);
        if (issueQuarter === targetQuarter) return true;
      }

      const dateMatch = rawPeriod.match(/^(\d{4})-(0[1-9]|1[0-2])-\d{2}/);
      if (dateMatch && dateMatch[1] === targetYear) {
        const issueQuarter = Math.ceil(Number(dateMatch[2]) / 3);
        if (issueQuarter === targetQuarter) return true;
      }
    }

    const targetMonthMatch = targetCode.match(/^(\d{4})(0[1-9]|1[0-2])$/);
    if (targetMonthMatch) {
      const dateMatch = rawPeriod.match(/^(\d{4})-(0[1-9]|1[0-2])-\d{2}/);
      if (
        dateMatch &&
        dateMatch[1] === targetMonthMatch[1] &&
        dateMatch[2] === targetMonthMatch[2]
      ) {
        return true;
      }
    }

    return false;
  }

  if (isYearFilterActive) {
    if (!rawPeriod) return false;
    const yearStr = String(selectedYear);
    const formattedIssue = formatReportingPeriod(rawPeriod);
    const issueCode = convertPeriodToCode(rawPeriod);
    return (
      rawPeriod.includes(yearStr) ||
      formattedIssue.includes(yearStr) ||
      issueCode.includes(yearStr)
    );
  }

  return true;
}

export function resolveLevelOfCare(
  rawLevel?: string | null,
  facilityName?: string | null,
): string {
  const explicit = safeLower(rawLevel);
  if (explicit) {
    if (explicit === "nrh" || explicit.includes("national referral")) return "NRH";
    if (explicit === "rrh" || explicit.includes("regional referral")) return "RRH";
    if (explicit.includes("hospital") || explicit === "gh") return "General Hospital";
    if (explicit === "hciv" || explicit === "hc iv" || explicit === "hc 4") return "HC IV";
    if (explicit === "hciii" || explicit === "hc iii" || explicit === "hc 3") return "HC III";
    if (explicit === "hcii" || explicit === "hc ii" || explicit === "hc 2") return "HC II";
    if (explicit.includes("clinic")) return "Clinic";
    if (explicit.includes("drug shop")) return "Drug Shop";
    if (explicit === "rbb") return "RBB";
    if (explicit === "nbb") return "NBB";
    if (explicit === "bcdp") return "BCDP";
    return safeString(rawLevel);
  }

  const combined = `${safeString(rawLevel)} ${safeString(facilityName)}`;
  if (/\bnrh\b|national\s+referral/i.test(combined)) return "NRH";
  if (/\brrh\b|regional\s+referral/i.test(combined)) return "RRH";
  if (/\bhospital\b|\bgh\b/i.test(combined)) return "General Hospital";
  if (/\bhc\s*(?:iv|4)\b|health\s+cent(?:re|er)\s+(?:iv|4)\b/i.test(combined)) return "HC IV";
  if (/\bhc\s*(?:iii|3)\b|health\s+cent(?:re|er)\s+(?:iii|3)\b/i.test(combined)) return "HC III";
  if (/\bhc\s*(?:ii|2)\b|health\s+cent(?:re|er)\s+(?:ii|2)\b/i.test(combined)) return "HC II";
  if (/\bclinic\b|medical\s+cent(?:re|er)|maternity|dispensary|health\s+unit/i.test(combined)) {
    return "Clinic";
  }
  if (/\bdrug\s*shop\b|pharmacy/i.test(combined)) return "Drug Shop";
  if (/\brbb\b|regional\s+blood\s+bank/i.test(combined)) return "RBB";
  if (/\bnbb\b|national\s+blood\s+bank/i.test(combined)) return "NBB";
  if (/\bbcdp\b/i.test(combined)) return "BCDP";

  return "Other / Unspecified";
}

export type OrgHierarchyMaps = {
  orgToRegionMap: Map<string, string>;
  orgToDistrictMap: Map<string, string>;
  orgIdToNameMap: Map<string, string>;
};

export function buildOrgHierarchyMaps(hierarchyData: unknown): OrgHierarchyMaps {
  const regionMap = new Map<string, string>();
  const districtMap = new Map<string, string>();
  const idToNameMap = new Map<string, string>();

  if (!hierarchyData) {
    return {
      orgToRegionMap: regionMap,
      orgToDistrictMap: districtMap,
      orgIdToNameMap: idToNameMap,
    };
  }

  const rootList: unknown[] = Array.isArray(hierarchyData)
    ? hierarchyData
    : typeof hierarchyData === "object"
      ? [hierarchyData]
      : [];

  function traverseTree(
    nodes: unknown[],
    currentRegion: string | null,
    currentDistrict: string | null,
  ) {
    if (!Array.isArray(nodes)) return;

    for (const rawNode of nodes) {
      if (!rawNode || typeof rawNode !== "object") continue;
      const node = rawNode as { id?: string; name?: string; children?: unknown[] };

      const nodeName = safeString(node.name);
      const nodeId = safeString(node.id);

      if (nodeName && nodeId) {
        idToNameMap.set(nodeId.toLowerCase(), nodeName);
      }

      if (!currentRegion && Array.isArray(node.children) && node.children.length > 0) {
        for (const rawRegionNode of node.children) {
          if (!rawRegionNode || typeof rawRegionNode !== "object") continue;
          const regionNode = rawRegionNode as {
            id?: string;
            name?: string;
            children?: unknown[];
          };

          const regName = safeString(regionNode.name);
          const regId = safeString(regionNode.id);

          if (regName) {
            regionMap.set(regName.toLowerCase(), regName);
            if (regId) {
              regionMap.set(regId.toLowerCase(), regName);
              idToNameMap.set(regId.toLowerCase(), regName);
            }
          }

          if (Array.isArray(regionNode.children) && regionNode.children.length > 0) {
            for (const rawDistNode of regionNode.children) {
              if (!rawDistNode || typeof rawDistNode !== "object") continue;
              const distNode = rawDistNode as {
                id?: string;
                name?: string;
                children?: unknown[];
              };

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

              if (Array.isArray(distNode.children) && distNode.children.length > 0) {
                traverseTree(distNode.children, regName, distName);
              }
            }
          }
        }
        continue;
      }

      if (nodeName) {
        if (currentRegion) regionMap.set(nodeName.toLowerCase(), currentRegion);
        if (currentDistrict) districtMap.set(nodeName.toLowerCase(), currentDistrict);
      }
      if (nodeId) {
        if (currentRegion) regionMap.set(nodeId.toLowerCase(), currentRegion);
        if (currentDistrict) districtMap.set(nodeId.toLowerCase(), currentDistrict);
      }

      if (Array.isArray(node.children) && node.children.length > 0) {
        traverseTree(node.children, currentRegion, currentDistrict);
      }
    }
  }

  traverseTree(rootList, null, null);
  return {
    orgToRegionMap: regionMap,
    orgToDistrictMap: districtMap,
    orgIdToNameMap: idToNameMap,
  };
}

export function resolveIssueGeography(
  issue: Issue,
  maps: OrgHierarchyMaps,
): { regionName: string; districtName: string; facilityName: string } {
  let regionName = safeString(issue.region);
  if (regionName && maps.orgIdToNameMap.has(safeLower(regionName))) {
    regionName = maps.orgIdToNameMap.get(safeLower(regionName))!;
  }
  if (!regionName && issue.org_unit) {
    regionName = maps.orgToRegionMap.get(safeLower(issue.org_unit)) || "";
  }
  if (!regionName && issue.district) {
    regionName = maps.orgToRegionMap.get(safeLower(issue.district)) || "";
  }
  if (!regionName) {
    regionName = "Unspecified";
  }

  let districtName = safeString(issue.district);
  if (districtName && maps.orgIdToNameMap.has(safeLower(districtName))) {
    districtName = maps.orgIdToNameMap.get(safeLower(districtName))!;
  }
  if (!districtName && issue.org_unit) {
    districtName =
      maps.orgToDistrictMap.get(safeLower(issue.org_unit)) || safeString(issue.org_unit);
  }
  if (!districtName) {
    districtName = "Unspecified";
  }

  let facilityName = safeString(issue.org_unit);
  if (!facilityName) {
    facilityName = "Unspecified Facility";
  } else {
    const mappedName = maps.orgIdToNameMap.get(safeLower(facilityName));
    if (mappedName) {
      facilityName = mappedName;
    }
  }

  return { regionName, districtName, facilityName };
}

export function doesIssueMatchLocationFilter(
  issue: Issue,
  locationFilter: string,
  maps: OrgHierarchyMaps,
): boolean {
  if (
    !locationFilter ||
    safeLower(locationFilter) === "all districts" ||
    safeLower(locationFilter) === "all regions"
  ) {
    return true;
  }

  const { regionName, districtName } = resolveIssueGeography(issue, maps);
  const targetRegKey = normalizeRegionKey(locationFilter);
  const targetDistKey = normalizeDistrictKey(locationFilter);

  return (
    normalizeRegionKey(regionName) === targetRegKey ||
    normalizeDistrictKey(districtName) === targetDistKey ||
    safeLower(regionName) === safeLower(locationFilter) ||
    safeLower(districtName) === safeLower(locationFilter)
  );
}

export function aggregateIssuesByRegion(
  issues: Issue[],
  maps: OrgHierarchyMaps,
): { regionalSummary: RegionSummaryRow[]; grandTotals: DashboardGrandTotals } {
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
              levelOfCare: string;
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

  for (const issue of issues) {
    const { regionName, districtName, facilityName } = resolveIssueGeography(issue, maps);
    const resolved = isIssueResolved(issue.status);
    const isCriticalOpen = !resolved && isCriticalIssue(issue);

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

    const levelOfCare = resolveLevelOfCare(issue.level_of_care, facilityName);
    if (!dGroup.facilityGroups.has(facilityName)) {
      dGroup.facilityGroups.set(facilityName, {
        levelOfCare,
        critical: 0,
        open: 0,
        resolved: 0,
        total: 0,
      });
    }

    const fGroup = dGroup.facilityGroups.get(facilityName)!;
    if (
      fGroup.levelOfCare === "Other / Unspecified" &&
      levelOfCare !== "Other / Unspecified"
    ) {
      fGroup.levelOfCare = levelOfCare;
    }
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

  const rows: RegionSummaryRow[] = [];
  regionGroups.forEach((val, region) => {
    const districtsList: DistrictSummary[] = [];
    val.districtGroups.forEach((dVal, dName) => {
      const facilitiesList: FacilitySummary[] = [];
      dVal.facilityGroups.forEach((fVal, fName) => {
        facilitiesList.push({
          facility: fName,
          levelOfCare: fVal.levelOfCare,
          critical: fVal.critical,
          open: fVal.open,
          resolved: fVal.resolved,
          total: fVal.total,
        });
      });

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
}

export function buildUnresolvedAgingIssues(
  issues: Issue[],
  maps: OrgHierarchyMaps,
): ActionableAgingIssue[] {
  const nowMs = Date.now();
  const dayMs = 1000 * 60 * 60 * 24;
  const items: ActionableAgingIssue[] = [];

  for (const issue of issues) {
    if (isIssueResolved(issue.status)) {
      continue;
    }

    const { regionName, districtName, facilityName } = resolveIssueGeography(issue, maps);
    const parsedDate = parseIssueDate(issue);
    const ageDays = parsedDate
      ? Math.max(0, Math.floor((nowMs - parsedDate.getTime()) / dayMs))
      : 0;
    const bucket = getAgingBucketForDays(ageDays);
    const isCritical = isCriticalIssue(issue);
    const isUnassigned = !safeString(issue.assigned_to);
    const issueCode =
      safeString(issue.issue_code) ||
      (issue.issue_id ? `HMIS-${String(issue.issue_id).padStart(4, "0")}` : "ISSUE");
    const reportedDateLabel =
      safeString(issue.date_reported).slice(0, 10) ||
      formatReportingPeriod(issue.time_Period || issue.time_period) ||
      "N/A";

    items.push({
      issue,
      issueCode,
      ageDays,
      bucket,
      regionName,
      districtName,
      facilityName,
      isCritical,
      isUnassigned,
      reportedDateLabel,
    });
  }

  items.sort((a, b) => {
    if (b.ageDays !== a.ageDays) return b.ageDays - a.ageDays;
    if (a.isCritical !== b.isCritical) return a.isCritical ? -1 : 1;
    return a.isUnassigned === b.isUnassigned ? 0 : a.isUnassigned ? -1 : 1;
  });

  return items;
}

export type AgingMetricsSummary = {
  totalUnresolved: number;
  overdueCount: number;
  criticalCount: number;
  unassignedCount: number;
  avgAgeDays: number;
  maxAgeDays: number;
  bucketMap: Map<
    AgingBucketId,
    { total: number; critical: number; unassigned: number; assigned: number }
  >;
};

export function computeAgingMetrics(
  unresolvedAgingIssues: ActionableAgingIssue[],
): AgingMetricsSummary {
  const bucketMap = new Map<
    AgingBucketId,
    { total: number; critical: number; unassigned: number; assigned: number }
  >();

  for (const b of AGING_BUCKETS) {
    bucketMap.set(b.id, { total: 0, critical: 0, unassigned: 0, assigned: 0 });
  }

  let overdueCount = 0;
  let criticalCount = 0;
  let unassignedCount = 0;
  let totalAgeDays = 0;
  let maxAgeDays = 0;

  for (const item of unresolvedAgingIssues) {
    const bStat = bucketMap.get(item.bucket.id)!;
    bStat.total += 1;
    if (item.isCritical) {
      bStat.critical += 1;
      criticalCount += 1;
    }
    if (item.isUnassigned) {
      bStat.unassigned += 1;
      unassignedCount += 1;
    } else {
      bStat.assigned += 1;
    }
    if (item.ageDays > 30) {
      overdueCount += 1;
    }
    totalAgeDays += item.ageDays;
    if (item.ageDays > maxAgeDays) {
      maxAgeDays = item.ageDays;
    }
  }

  const totalUnresolved = unresolvedAgingIssues.length;
  const avgAgeDays = totalUnresolved > 0 ? Math.round(totalAgeDays / totalUnresolved) : 0;

  return {
    totalUnresolved,
    overdueCount,
    criticalCount,
    unassignedCount,
    avgAgeDays,
    maxAgeDays,
    bucketMap,
  };
}

function createEmptyAgingBuckets(): AgingBracketCounts {
  return {
    "0-7": 0,
    "8-14": 0,
    "15-30": 0,
    "31-60": 0,
    "61-90": 0,
    "90+": 0,
  };
}

type MutableAgingAccumulator = {
  buckets: AgingBracketCounts;
  overdue: number;
  critical: number;
  unassigned: number;
  total: number;
  totalAgeDays: number;
  maxAgeDays: number;
};

function createAgingAccumulator(): MutableAgingAccumulator {
  return {
    buckets: createEmptyAgingBuckets(),
    overdue: 0,
    critical: 0,
    unassigned: 0,
    total: 0,
    totalAgeDays: 0,
    maxAgeDays: 0,
  };
}

function addIssueToAgingAccumulator(
  acc: MutableAgingAccumulator,
  item: ActionableAgingIssue,
) {
  acc.buckets[item.bucket.id] += 1;
  acc.total += 1;
  acc.totalAgeDays += item.ageDays;
  if (item.ageDays > acc.maxAgeDays) {
    acc.maxAgeDays = item.ageDays;
  }
  if (item.ageDays > 30) {
    acc.overdue += 1;
  }
  if (item.isCritical) {
    acc.critical += 1;
  }
  if (item.isUnassigned) {
    acc.unassigned += 1;
  }
}

export function aggregateAgingIssuesByRegion(
  agingIssues: ActionableAgingIssue[],
): {
  regionalAgingSummary: AgingRegionSummaryRow[];
  grandTotals: AgingGrandTotals;
} {
  const regionGroups = new Map<
    string,
    MutableAgingAccumulator & {
      districtGroups: Map<
        string,
        MutableAgingAccumulator & {
          facilityGroups: Map<
            string,
            MutableAgingAccumulator & { levelOfCare: string }
          >;
        }
      >;
    }
  >();

  const grandAcc = createAgingAccumulator();

  for (const item of agingIssues) {
    const { regionName, districtName, facilityName } = item;

    addIssueToAgingAccumulator(grandAcc, item);

    if (!regionGroups.has(regionName)) {
      regionGroups.set(regionName, {
        ...createAgingAccumulator(),
        districtGroups: new Map(),
      });
    }
    const rGroup = regionGroups.get(regionName)!;
    addIssueToAgingAccumulator(rGroup, item);

    if (!rGroup.districtGroups.has(districtName)) {
      rGroup.districtGroups.set(districtName, {
        ...createAgingAccumulator(),
        facilityGroups: new Map(),
      });
    }
    const dGroup = rGroup.districtGroups.get(districtName)!;
    addIssueToAgingAccumulator(dGroup, item);

    const levelOfCare = resolveLevelOfCare(
      item.issue.level_of_care,
      facilityName,
    );
    if (!dGroup.facilityGroups.has(facilityName)) {
      dGroup.facilityGroups.set(facilityName, {
        ...createAgingAccumulator(),
        levelOfCare,
      });
    }
    const fGroup = dGroup.facilityGroups.get(facilityName)!;
    if (
      fGroup.levelOfCare === "Other / Unspecified" &&
      levelOfCare !== "Other / Unspecified"
    ) {
      fGroup.levelOfCare = levelOfCare;
    }
    addIssueToAgingAccumulator(fGroup, item);
  }

  const rows: AgingRegionSummaryRow[] = [];

  regionGroups.forEach((rVal, region) => {
    const districtsList: AgingDistrictSummary[] = [];

    rVal.districtGroups.forEach((dVal, district) => {
      const facilitiesList: AgingFacilitySummary[] = [];

      dVal.facilityGroups.forEach((fVal, facility) => {
        facilitiesList.push({
          facility,
          levelOfCare: fVal.levelOfCare,
          buckets: { ...fVal.buckets },
          overdue: fVal.overdue,
          critical: fVal.critical,
          unassigned: fVal.unassigned,
          total: fVal.total,
          avgAgeDays:
            fVal.total > 0 ? Math.round(fVal.totalAgeDays / fVal.total) : 0,
          maxAgeDays: fVal.maxAgeDays,
        });
      });

      facilitiesList.sort(
        (a, b) =>
          b.total - a.total ||
          b.overdue - a.overdue ||
          b.avgAgeDays - a.avgAgeDays,
      );

      districtsList.push({
        district,
        buckets: { ...dVal.buckets },
        overdue: dVal.overdue,
        critical: dVal.critical,
        unassigned: dVal.unassigned,
        total: dVal.total,
        avgAgeDays:
          dVal.total > 0 ? Math.round(dVal.totalAgeDays / dVal.total) : 0,
        maxAgeDays: dVal.maxAgeDays,
        facilities: facilitiesList,
      });
    });

    districtsList.sort(
      (a, b) =>
        b.total - a.total ||
        b.overdue - a.overdue ||
        b.avgAgeDays - a.avgAgeDays,
    );

    rows.push({
      id: `aging-region-${region}`,
      region,
      "0-7": rVal.buckets["0-7"],
      "8-14": rVal.buckets["8-14"],
      "15-30": rVal.buckets["15-30"],
      "31-60": rVal.buckets["31-60"],
      "61-90": rVal.buckets["61-90"],
      "90+": rVal.buckets["90+"],
      overdue: rVal.overdue,
      critical: rVal.critical,
      unassigned: rVal.unassigned,
      total: rVal.total,
      avgAgeDays:
        rVal.total > 0 ? Math.round(rVal.totalAgeDays / rVal.total) : 0,
      maxAgeDays: rVal.maxAgeDays,
      districts: districtsList,
    });
  });

  rows.sort(
    (a, b) =>
      b.total - a.total ||
      b.overdue - a.overdue ||
      b.avgAgeDays - a.avgAgeDays,
  );

  return {
    regionalAgingSummary: rows,
    grandTotals: {
      "0-7": grandAcc.buckets["0-7"],
      "8-14": grandAcc.buckets["8-14"],
      "15-30": grandAcc.buckets["15-30"],
      "31-60": grandAcc.buckets["31-60"],
      "61-90": grandAcc.buckets["61-90"],
      "90+": grandAcc.buckets["90+"],
      overdue: grandAcc.overdue,
      critical: grandAcc.critical,
      unassigned: grandAcc.unassigned,
      total: grandAcc.total,
      avgAgeDays:
        grandAcc.total > 0
          ? Math.round(grandAcc.totalAgeDays / grandAcc.total)
          : 0,
      maxAgeDays: grandAcc.maxAgeDays,
      activeRegionsCount: regionGroups.size,
    },
  };
}

export function exportRegionalSummaryToExcel(
  displayedRows: RegionSummaryRow[],
  tableGrandTotals: DashboardGrandTotals,
) {
  const exportHeaders = [
    "Region",
    "Critical Issues (Open)",
    "Open Issues",
    "Resolved Issues",
    "Total Issues",
    "Share (%)",
  ];

  const dataRows = displayedRows.map((row) => [
    row.region,
    row.critical,
    row.open,
    row.resolved,
    row.total,
    `${row.percentage.toFixed(1)}%`,
  ]);

  const totalRow = [
    "TOTAL",
    tableGrandTotals.critical,
    tableGrandTotals.open,
    tableGrandTotals.resolved,
    tableGrandTotals.total,
    "100%",
  ];

  const sheetData = [exportHeaders, ...dataRows, [], totalRow];
  const worksheet = XLSX.utils.aoa_to_sheet(sheetData);
  worksheet["!cols"] = exportHeaders.map(() => ({ wch: 18 }));

  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, worksheet, "Regional Issue Summary");

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
  for (const r of displayedRows) {
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

  const districtWorksheet = XLSX.utils.aoa_to_sheet([districtHeaders, ...districtDataRows]);
  districtWorksheet["!cols"] = districtHeaders.map(() => ({ wch: 22 }));
  XLSX.utils.book_append_sheet(workbook, districtWorksheet, "District Breakdown");

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
  for (const r of displayedRows) {
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

  const facilityWorksheet = XLSX.utils.aoa_to_sheet([facilityHeaders, ...facilityDataRows]);
  facilityWorksheet["!cols"] = facilityHeaders.map(() => ({ wch: 24 }));
  XLSX.utils.book_append_sheet(workbook, facilityWorksheet, "Facility Breakdown");

  const dateStr = new Date().toISOString().slice(0, 10);
  XLSX.writeFile(workbook, `issue-regional-summary-${dateStr}.xlsx`);
}

export function exportAgingSummaryToExcel(
  displayedRows: AgingRegionSummaryRow[],
  tableGrandTotals: AgingGrandTotals,
) {
  const exportHeaders = [
    "Region",
    "0–7 Days",
    "8–14 Days",
    "15–30 Days",
    "31–60 Days",
    "61–90 Days",
    "> 90 Days",
    "Avg Days",
    "Total Unresolved",
  ];

  const dataRows = displayedRows.map((row) => [
    row.region,
    row["0-7"],
    row["8-14"],
    row["15-30"],
    row["31-60"],
    row["61-90"],
    row["90+"],
    row.avgAgeDays,
    row.total,
  ]);

  const totalRow = [
    "TOTAL",
    tableGrandTotals["0-7"],
    tableGrandTotals["8-14"],
    tableGrandTotals["15-30"],
    tableGrandTotals["31-60"],
    tableGrandTotals["61-90"],
    tableGrandTotals["90+"],
    tableGrandTotals.avgAgeDays,
    tableGrandTotals.total,
  ];

  const sheetData = [exportHeaders, ...dataRows, [], totalRow];
  const worksheet = XLSX.utils.aoa_to_sheet(sheetData);
  worksheet["!cols"] = exportHeaders.map(() => ({ wch: 18 }));

  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, worksheet, "Regional Aging Summary");

  const districtHeaders = [
    "Region",
    "District",
    "Facilities Affected",
    "0–7 Days",
    "8–14 Days",
    "15–30 Days",
    "31–60 Days",
    "61–90 Days",
    "> 90 Days",
    "Avg Days",
    "Total Unresolved",
  ];

  const districtDataRows: (string | number)[][] = [];
  for (const r of displayedRows) {
    for (const d of r.districts) {
      districtDataRows.push([
        r.region,
        d.district,
        d.facilities.length,
        d.buckets["0-7"],
        d.buckets["8-14"],
        d.buckets["15-30"],
        d.buckets["31-60"],
        d.buckets["61-90"],
        d.buckets["90+"],
        d.avgAgeDays,
        d.total,
      ]);
    }
  }

  const districtWorksheet = XLSX.utils.aoa_to_sheet([
    districtHeaders,
    ...districtDataRows,
  ]);
  districtWorksheet["!cols"] = districtHeaders.map(() => ({ wch: 20 }));
  XLSX.utils.book_append_sheet(workbook, districtWorksheet, "District Aging Breakdown");

  const facilityHeaders = [
    "Region",
    "District",
    "Health Facility / Org Unit",
    "0–7 Days",
    "8–14 Days",
    "15–30 Days",
    "31–60 Days",
    "61–90 Days",
    "> 90 Days",
    "Avg Days",
    "Total Unresolved",
  ];

  const facilityDataRows: (string | number)[][] = [];
  for (const r of displayedRows) {
    for (const d of r.districts) {
      for (const f of d.facilities) {
        facilityDataRows.push([
          r.region,
          d.district,
          f.facility,
          f.buckets["0-7"],
          f.buckets["8-14"],
          f.buckets["15-30"],
          f.buckets["31-60"],
          f.buckets["61-90"],
          f.buckets["90+"],
          f.avgAgeDays,
          f.total,
        ]);
      }
    }
  }

  const facilityWorksheet = XLSX.utils.aoa_to_sheet([
    facilityHeaders,
    ...facilityDataRows,
  ]);
  facilityWorksheet["!cols"] = facilityHeaders.map(() => ({ wch: 22 }));
  XLSX.utils.book_append_sheet(workbook, facilityWorksheet, "Facility Aging Breakdown");

  const dateStr = new Date().toISOString().slice(0, 10);
  XLSX.writeFile(workbook, `issue-unresolved-aging-${dateStr}.xlsx`);
}
