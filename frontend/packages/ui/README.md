# MOH SSO UI Package

## External application header

Use `ExternalAppHeader` in independently hosted React apps to share the MOH header
while setting `appName` to the current application's name. The host supplies its
verified username, authorized launch links, profile action and logout flow.
No portal Redux store or authentication provider is required. Import Carbon's
stylesheet and `@moh-sso/ui/styles.css` once in the host application.

See [the external header integration guide](../../../docs/external-app-header.md)
for a full example, session ownership and acceptance checks. `UserHeader` also
accepts optional `appName` and `onProfile` props; the portal title remains the default.

## Rules

- Do not import from `@moh-sso/api`, `@moh-sso/state`, or app folders.
- Components should be presentation-only.
- Business data fetching stays in apps.
- Use Carbon tokens and MOH CSS variables.
- Avoid inline styles.
- Use `PageShell`, `SectionCard`, `EmptyState`, and `ErrorState` for consistency.

## Theme Usage

Wrap shell and independently mounted modules with `MohThemeProvider`.
The shell owns the global provider, while `@moh-sso/microfrontend` wraps standalone app lifecycles so microfrontends receive the same MOH/Carbon theme in local, hybrid, and remote modes.

Use the default `theme="white"` unless a product requirement explicitly calls for another Carbon theme.

## Data Display

Use `DataTableShell` for feature pages that share the standard table frame:

- title and description
- filter region
- loading state
- empty table state
- table tile/container
- custom row rendering through a render prop

Use `TableStatusTag` for simple status tags, `RowActionsCell` for row action alignment, and `DataTablePagination` for standard paginated tables.

Keep feature-specific behavior in the feature app:

- API hooks and mutations
- search/filter state
- row action menus
- bulk action business logic
- drawers, modals, and feature-specific panels
