export const IssueTypes = ["Outliers", "Incomplete Data", "Invalid Formats", "Inconsistent Values"];

export const Priority = ["Low", "Moderate", "High"];

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

export const ORG_UNIT_DATA = [
  {
    id: "ug-1",
    name: "MOH - Uganda",
    children: [
      {
        id: "acholi-1",
        name: "Acholi",
        children: [
          { id: "agago-1", name: "Agago District" },
          { id: "amuru-1", name: "Amuru District" },
          { id: "gulu-c", name: "Gulu City" },
          { id: "gulu-d", name: "Gulu District" },
          { id: "kitgum-d", name: "Kitgum District" },
        ],
      },
      {
        id: "ankole-1",
        name: "Ankole",
        children: [
          { id: "mbarara-c", name: "Mbarara City" },
          { id: "bushenyi-d", name: "Bushenyi District" },
        ],
      },
      {
        id: "bugisu-1",
        name: "Bugisu",
        children: [
          { id: "mbale-c", name: "Mbale City" },
          { id: "sironko-d", name: "Sironko District" },
        ],
      },
    ],
  },
];
