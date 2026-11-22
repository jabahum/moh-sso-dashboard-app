export type Issue = {
  id?: string;
  issue_id: number;
  issue_code: string;
  program?: string;
  dataset: string;
  data_element: string;
  org_unit: string;
  region?: string;
  district?: string;
  level_of_care?: string;
  issue: string;
  date_reported: string;
  reported_by: string;
  status: string;
  issue_type: string;
  updated_by?: string;
  updated_date?: string;
  priority?: string;
  severity?: string;
  time_period?: string;
  time_Period?: string;
  assigned_to?: string;
};

export type IssuePayload = {
  program?: string;
  dataset: string;
  data_element: string;
  org_unit: string;
  region?: string;
  district?: string;
  level_of_care?: string;
  issue: string;
  issue_type: string;
  reported_by?: string;
  updated_by?: string;
  priority?: string;
  severity?: string;
  time_period?: string;
  time_Period?: string;
  assigned_to?: string;
};

export type IssueTransaction = {
  id?: string | number;
  issue_id?: number;
  issue_code?: string;

  resolution_date?: string | null;
  resolved_by?: string | null;
  resolution_action?: string | null;

  comment?: string | null;
  status?: string | null;
  assigned_to?: string | null;
  created_by?: string | null;
  updated_by?: string | null;
  created_at?: string | null;
  updated_at?: string | null;
};

export type IssueTransactionPayload = {
  comment?: string;
  status?: string;
  resolution?: string;
  resolution_action?: string;
  resolved_by?: string;
  updated_by?: string;
  assigned_to?: string;
  [key: string]: unknown;
};

export type KeycloakGroup = {
  id: string;
  name: string;
  path: string;
};

export type KeycloakGroupMember = {
  id: string;
  username: string;
  email: string;
  firstName: string;
  lastName: string;
};

export type AssignIssuesPayload = {
  issue_codes?: string[];
  issue_code?: string;
  assigned_to: string;
  comment?: string;
};

export type IssueProgramSummary = {
  program: string;
  issue_count: number;
  open_count: number;
  resolved_count: number;
};

export type GetIssuesParams = {
  limit: number;
  offset: number;
  program?: string;
  dataset?: string;
  dataElement?: string;
  period?: string;
  region?: string;
  district?: string;
  orgUnit?: string;
  assignedTo?: string;
  search?: string;
};

export type GetIssuesResponse = {
  items: Issue[];
  totalCount: number;
};

export type Dataset = {
  dataset_id: string;
  display_name: string;
};

export type ThemeElement = {
  theme_category_id: number;
  theme_id: string;
  theme_name: string;
  data_element_key: number;
  data_element_id: string;
  data_element_short_name: string;
};

export type PeriodOption = {
  id?: string;
  label: string;
  value?: string;
  disabled?: boolean;
};

export type SelectEvent<T> = {
  selectedItem?: T | null;
};

export type FacilitySummary = {
  facility: string;
  levelOfCare: string;
  critical: number;
  open: number;
  resolved: number;
  total: number;
};

export type DistrictSummary = {
  district: string;
  critical: number;
  open: number;
  resolved: number;
  total: number;
  facilities: FacilitySummary[];
};

export type RegionSummaryRow = {
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

export type DashboardGrandTotals = {
  critical: number;
  open: number;
  resolved: number;
  total: number;
  activeRegionsCount: number;
};

export type AgingBucketId = "0-7" | "8-14" | "15-30" | "31-60" | "61-90" | "90+";

export type AgingBucketDefinition = {
  id: AgingBucketId;
  label: string;
  shortLabel: string;
  minDays: number;
  maxDays: number;
  color: string;
  badgeClass: string;
};

export type ActionableAgingIssue = {
  issue: Issue;
  issueCode: string;
  ageDays: number;
  bucket: AgingBucketDefinition;
  regionName: string;
  districtName: string;
  facilityName: string;
  isCritical: boolean;
  isUnassigned: boolean;
  reportedDateLabel: string;
};

export type AgingBracketCounts = Record<AgingBucketId, number>;

export type AgingFacilitySummary = {
  facility: string;
  levelOfCare: string;
  buckets: AgingBracketCounts;
  overdue: number;
  critical: number;
  unassigned: number;
  total: number;
  avgAgeDays: number;
  maxAgeDays: number;
};

export type AgingDistrictSummary = {
  district: string;
  buckets: AgingBracketCounts;
  overdue: number;
  critical: number;
  unassigned: number;
  total: number;
  avgAgeDays: number;
  maxAgeDays: number;
  facilities: AgingFacilitySummary[];
};

export type AgingRegionSummaryRow = {
  id: string;
  region: string;
  "0-7": number;
  "8-14": number;
  "15-30": number;
  "31-60": number;
  "61-90": number;
  "90+": number;
  overdue: number;
  critical: number;
  unassigned: number;
  total: number;
  avgAgeDays: number;
  maxAgeDays: number;
  districts: AgingDistrictSummary[];
  [key: string]: unknown;
};

export type AgingGrandTotals = {
  "0-7": number;
  "8-14": number;
  "15-30": number;
  "31-60": number;
  "61-90": number;
  "90+": number;
  overdue: number;
  critical: number;
  unassigned: number;
  total: number;
  avgAgeDays: number;
  maxAgeDays: number;
  activeRegionsCount: number;
};

