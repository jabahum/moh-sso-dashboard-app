# Microfrontend dependency audit

This audit tracks direct cross-app imports and whether they should remain. It is for frontend developers planning dependency cleanup.

## Current intentional dependencies

Declared in each app's `package.json` and used in its `src/`:

- `users` imports `@moh-sso/clients/types` (client role and role-assignment types) and the `@moh-sso/rbac` API hooks and types for user access.
- `rbac` imports `useListUsersQuery` from `@moh-sso/users`, so `users` and `rbac` depend on each other.
- `clients` imports user-access hooks from `@moh-sso/rbac/api`.
- `announcements` imports from `@moh-sso/rbac` and `useListUsersQuery` from `@moh-sso/users`.
- `email` imports `useListRbacGroupsQuery` from `@moh-sso/rbac`.
- `documents` imports `useGetUserQuery` from `@moh-sso/users/api`.
- `surveillance` imports the documents API and types for disease detail documents.
- `issue-tracker` declares `@moh-sso/data-visualizer` as a dependency but no longer imports it.
- `home` is shell-owned and lazy-loads panels from users (`UserFormPanel`), clients (`ClientFormPanel`) and announcements (`ManageAnnouncementsPanel`).

## Target direction

- Shell should import apps through lifecycle modules or public index exports.
- Apps should avoid importing shell internals.
- Reusable components should move to `frontend/packages/ui`.
- Shared types should move to `frontend/packages/types`.
- Shared API hooks should stay in `frontend/packages/api`.

## Next cleanup candidates

- Move the user, RBAC and client access hooks and types shared by `users`, `rbac`, `clients`, `announcements`, `email` and `documents` into a shared package to break the `users`/`rbac` cycle.
- Remove the unused `@moh-sso/data-visualizer` dependency from `issue-tracker`.
- Keep surveillance/document API coupling documented until domain ownership is clearer.
