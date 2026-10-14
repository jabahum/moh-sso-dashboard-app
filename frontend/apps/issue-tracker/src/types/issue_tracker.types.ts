export type Issue = {
  id?: string;
  issue_id: number;
  issue_code: string;
  dataset: string;
  data_element: string;
  org_unit: string;
  region?: string;
  district?: string;
  issue: string;
  date_reported: string;
  reported_by: string;
  status: string;
  issue_type: string;
  updated_by: string;
  updated_date: string;
  priority: string;
  severity: string;
  time_period: string;
  time_Period: string;
  assigned_to?: string;
};

export type IssuePayload = {
  dataset: string;
  data_element: string;
  org_unit: string;
  region?: string;
  district?: string;
  issue: string;
  issue_type: string;
  reported_by?: string;
  updated_by?: string;
  priority?: string;
  severity?: string;
  time_period: string;
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


