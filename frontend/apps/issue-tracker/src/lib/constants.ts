export const IssueTypes = [
  "Outliers",
  "Incomplete Data",
  "Invalid Formats",
  "Inconsistent Values",
];

export const Priority = ["Low", "Moderate", "High", "Critical"];

export const Severity = ["Low", "Moderate", "High", "Critical"];

export const periodType = [
  { label: "Weekly", value: "Weekly" },
  { label: "Monthly", value: "Monthly" },
  { label: "Quarterly", value: "Quarterly" },
];

export const headers = [
  {
    key: "dataset",
    header: "DataSet",
  },
  {
    key: "data_element",
    header: "Data Element",
  },
  {
    key: "issue",
    header: "Issue",
  },
  {
    key: "org_unit",
    header: "Facility Name",
  },
  {
    key: "district",
    header: "District",
  },
  {
    key: "time_Period",
    header: "Reporting Period",
  },
  {
    key: "status",
    header: "Status",
  },
  {
    key: "assigned_to",
    header: "Assigned To",
  },
  {
    key: "date_reported",
    header: "Date Reported",
  },
];

/** Headers used in the downloadable import template */
export const IMPORT_TEMPLATE_HEADERS = [
  "dataset",
  "data_element",
  "org_unit",
  "region",
  "district",
  "issue",
  "issue_type",
  "priority",
  "severity",
  "time_period",
];

export const IMPORT_TEMPLATE_EXAMPLE_ROW = [
  "Example Dataset",
  "Example Data Element",
  "Example Org Unit",
  "Central Region",
  "Kampala District",
  "Example issue description",
  "Outliers",
  "High",
  "Moderate",
  "2025Q1",
];
