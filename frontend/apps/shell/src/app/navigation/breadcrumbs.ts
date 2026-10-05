import type { AppBreadcrumbItem } from "@moh-sso/ui";

const LABEL_MAP: Record<string, string> = {
  portal: "Portal",
  apps: "Applications",

  news: "News & Updates",

  dwh: "Data & Statistics",
  "data-visualizer": "Data Visualizer",
  "data-validation": "Data Validation",
  dashboards: "Dashboards",
  documents: "Documents",
  filesvr: "Documents",
  surveillance: "Surveillance",
  "issue-tracker": "Issue Tracker",
  "report-scheduler": "Report Scheduler",

  eservices: "E-Services",
  ihris: "iHRIS",
  "meeting-manager": "Meeting Manager",
  "action-tracker": "Action Tracker",
  "clinician-outputs": "Clinician Outputs",
  "leave-absence-management": "Leave & Absence Management",
  workplans: "Workplans",
  "budget-tracker": "Budget Tracker",
  "activity-reporting": "Activity Reporting",
  "partner-management": "Partner Management",
  "observatory-uploads": "Observatory Uploads",

  "research-studies": "Research Studies",
  studies: "Studies",
  datasets: "Datasets",
  ethics: "Ethics & Approvals",
  publications: "Publications",

  "case-registers": "Case Registers",
  "external-referrals": "External Referrals",
  "disease-registers": "Disease Registers",

  "outbreak-management": "Outbreak Management",
  "signals-alerts": "Signals & Alerts",
  "poe-management": "PoE Management",
  "case-management": "Case Management",

  "reference-registers": "Reference Registers",
  "facility-register": "Facility Register",
  terminology: "Terminology",
  "test-menu": "Test Menu",
  pharmaceuticals: "Pharmaceuticals",
  procedures: "Procedures",
  equipment: "Equipment",

  utilities: "Utilities",
  "self-service": "Self Service",
  timesheet: "Timesheet",
  elearning: "E-learning",
  "leave-plan": "Leave Plan",
  "absence-requests": "Absence Requests",
  "absence-dashboard": "Absence Dashboard",
  eservice: "E-Service",
  "document-upload": "Document Upload",
  "service-access": "Service Access",
  "equipment-request": "Equipment Request",

  settings: "Settings",
  profile: "Profile",
  sessions: "Active Sessions",
  security: "Security",
};

function toTitleCase(segment: string): string {
  return segment
    .split("-")
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function getLabel(segment: string): string {
  return LABEL_MAP[segment] ?? toTitleCase(segment);
}

export function buildBreadcrumbs(pathname: string): AppBreadcrumbItem[] {
  const cleanPath = pathname.split("?")[0].split("#")[0];

  const segments = cleanPath.split("/").filter(Boolean);

  const portalIndex = segments.indexOf("portal");
  const effectiveSegments = portalIndex >= 0 ? segments.slice(portalIndex) : segments;

  const items: AppBreadcrumbItem[] = [];
  let currentPath = "";

  effectiveSegments.forEach((segment, index) => {
    currentPath += `/${segment}`;

    const isLast = index === effectiveSegments.length - 1;

    items.push({
      label: getLabel(segment),
      href: isLast ? undefined : currentPath,
      current: isLast,
    });
  });

  return items;
}
