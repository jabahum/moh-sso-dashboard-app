# Responsive Audit

This audit captures the current mobile hardening work for the MOH Integrated Health Portal.

## Viewports

Use these viewports when checking the portal:

- 320px wide small mobile
- 375px wide mobile
- 430px wide large mobile
- 768px tablet
- 1024px tablet landscape
- desktop

## Shared Fixes Applied

- Admin and user side navigation now behaves as an overlay drawer below tablet width.
- Mobile side navigation includes a backdrop, Escape-to-close, and route-change close behavior.
- Content padding now accounts for the fixed footer.
- Shared `DataTableShell` wraps tables in a horizontal scroll container.
- Shared side panels become full-width, scrollable mobile drawers with stacked footer actions.
- Shared page shells and breadcrumbs handle narrow widths and long labels more safely.
- App launcher width and grid behavior have mobile-specific rules.
- Every standalone microfrontend root component is wrapped in a defensive
  `.moh-microfrontend-root` container to prevent child modules from forcing
  horizontal page overflow.
- Shared header panels and app launcher panels now trap focus while open.
- Admin and user mobile side navigation drawers now trap focus while open.
- Documents now has a feature stylesheet for responsive tabs, stats, toolbar,
  and table overflow.
- Data visualizer, RBAC, surveillance, and issue tracker now have additional
  mobile containment rules for dense controls, maps, popovers, and wide tables.

## Areas To Manually Check

- Public news page: `/portal/`
- Admin users: `/portal/admin/users`
- Admin clients: `/portal/admin/clients`
- Admin systems: `/portal/admin/systems`
- RBAC: `/portal/admin/rbac`
- Email outbox: `/portal/admin/emails`
- Audit logs: `/portal/admin/audit-logs`
- Documents: `/portal/apps/dwh/documents`
- Data validation: `/portal/apps/dwh/data-validation`
- Data visualizer: `/portal/apps/dwh/data-visualizer`
- Issue tracker: `/portal/apps/dwh/issue-tracker`
- Surveillance: `/portal/apps/dwh/surveillance`
- Utilities/settings: `/portal/apps/utilities/self-service`

## Remaining Manual QA

- Check the listed routes manually at 320, 375, 430, 768, and 1024px.
- Confirm data visualizer chart and pivot-table widgets remain usable with real
  data. The current pass contains overflow and stacking rules, not a full
  visualization redesign.
- Confirm RBAC governance panels with large Keycloak/client role payloads.
- Confirm surveillance charts/maps with production-sized geography and alert
  data.
- Confirm documents and issue tracker with production-sized filenames,
  statuses, users, and IDs.
- Confirm focus returns to the trigger after closing mobile drawers and panels.

## Acceptance Checklist

- Navigation can open, close, and route on mobile.
- No page content is hidden behind the footer.
- Tables scroll horizontally inside their own container.
- Panels are usable on a phone viewport.
- Long URLs, IDs, emails, and role names wrap or truncate without breaking layout.
- Desktop behavior remains unchanged.
