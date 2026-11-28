# @moh-sso/rbac

## 1.11.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- 230c5fe: Show only a loading indicator until the first data load finishes, then the full page, as the Report Scheduler does.
- e752ec8: Add `PageLoading` and `useFirstLoad` to `@moh-sso/ui`, and use them for each app's first-load indicator instead of per-app `InlineLoading` code.
- Updated dependencies
- Updated dependencies [230c5fe]
- Updated dependencies [e752ec8]
  - @moh-sso/users@1.11.0
  - @moh-sso/api@1.12.0
  - @moh-sso/auth@1.12.0
  - @moh-sso/microfrontend@1.12.0
  - @moh-sso/types@1.12.0
  - @moh-sso/ui@1.12.0

<!-- scoped-source-changes -->
### Source changes

Changes since `frontend/v1.10.0`.

- Share the first-load indicator as PageLoading and useFirstLoad (`e752ec86`)
- Show only a loading indicator until each app's first data load (`230c5fe3`)
- Add a Machine clients tab for binding platform scopes (`456757e1`)
- Add Check network to the identity sync page (`9be28d58`)

## 1.10.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.10.0
  - @moh-sso/api@1.11.0
  - @moh-sso/auth@1.11.0
  - @moh-sso/microfrontend@1.11.0
  - @moh-sso/types@1.11.0
  - @moh-sso/ui@1.11.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.9.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.9.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.9.0
  - @moh-sso/api@1.10.0
  - @moh-sso/auth@1.10.0
  - @moh-sso/microfrontend@1.10.0
  - @moh-sso/types@1.10.0
  - @moh-sso/ui@1.10.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.8.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.8.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.8.0
  - @moh-sso/api@1.9.0
  - @moh-sso/auth@1.9.0
  - @moh-sso/microfrontend@1.9.0
  - @moh-sso/types@1.9.0
  - @moh-sso/ui@1.9.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.7.0`.

- Merge pull request #363 from jabahum/fix/identity-sync-null-integration (`9ea08e6b`)
- Answer an unregistered identity-sync integration with 200 and data: null (`54e650f0`)
- Read identity-sync client secrets from Keycloak with a keycloak: credential reference (`234b07d2`)

## 1.7.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.7.0
  - @moh-sso/api@1.8.0
  - @moh-sso/auth@1.8.0
  - @moh-sso/microfrontend@1.8.0
  - @moh-sso/types@1.8.0
  - @moh-sso/ui@1.8.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.6.0`.

- Use tabs for governance tasks and dialogs for destructive governance actions (`2040bbfd`)
- Rebuild the RBAC page around application and portal-wide scopes (`363d111e`)
- Add the portal-wide realm permission and group panels (`6db3d5ee`)
- Add the application panels of the RBAC workspace (`67e71aff`)
- Stage permission changes and save them after review (`6b49198c`)
- Add shared RBAC workspace helpers for confirmations, unsaved changes and inline errors (`aa02bacd`)

## 1.6.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
- Updated dependencies [95a3edd]
- Updated dependencies [1f62724]
- Updated dependencies [d83ac9c]
- Updated dependencies [77ef3f4]
- Updated dependencies [bce82d0]
  - @moh-sso/users@1.6.0
  - @moh-sso/api@1.7.0
  - @moh-sso/auth@1.7.0
  - @moh-sso/microfrontend@1.7.0
  - @moh-sso/types@1.7.0
  - @moh-sso/ui@1.7.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.5.1`.

- Merge docs/content-structure (with upstream/main) into docs/accuracy (`50b0444f`)
- Correct frontend docs and READMEs to match the code (`2a18c23c`)
- Register an application's service identity from the Identity sync page (`963f8621`)
- Test the identity-sync screens end to end and fit their tables to the workspace (`14e25999`)
- Add identity-sync screens to the RBAC workspace (`1a5fd228`)
- Prevent service-account roles from being granted to people (`5ce06be3`)

## 1.5.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
- Updated dependencies [e017093]
  - @moh-sso/users@1.5.1
  - @moh-sso/api@1.6.0
  - @moh-sso/auth@1.6.0
  - @moh-sso/microfrontend@1.6.0
  - @moh-sso/types@1.6.0
  - @moh-sso/ui@1.6.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.5.0`.

- Fix/dashboard local errors (#332) (`5210efc`)

## 1.5.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.5.0
  - @moh-sso/api@1.5.0
  - @moh-sso/auth@1.5.0
  - @moh-sso/microfrontend@1.5.0
  - @moh-sso/types@1.5.0
  - @moh-sso/ui@1.5.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.4.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.4.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.4.0
  - @moh-sso/api@1.4.0
  - @moh-sso/auth@1.4.0
  - @moh-sso/microfrontend@1.4.0
  - @moh-sso/types@1.4.0
  - @moh-sso/ui@1.4.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.3.2`.

- No direct source changes; this release updates version or dependency metadata.

## 1.3.2

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/users@1.3.2
  - @moh-sso/api@1.3.2
  - @moh-sso/auth@1.3.2
  - @moh-sso/microfrontend@1.3.2
  - @moh-sso/types@1.3.2
  - @moh-sso/ui@1.3.2

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.3.1`.

- ci(frontend): publish npm modules on release tags (#300) (`851c9bc8`)
- Release/frontend modules npm (#298) (`fcb6d3a4`)

## 1.3.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/users@1.3.1
  - @moh-sso/api@1.3.1
  - @moh-sso/auth@1.3.1
  - @moh-sso/microfrontend@1.3.1
  - @moh-sso/types@1.3.1
  - @moh-sso/ui@1.3.1

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.3.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.3.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.3.0
  - @moh-sso/api@1.3.0
  - @moh-sso/auth@1.3.0
  - @moh-sso/microfrontend@1.3.0
  - @moh-sso/types@1.3.0
  - @moh-sso/ui@1.3.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.3`.

- feat: add health-context access control and administration (#289) (`0a35faab`)
- Refine admin workspaces and fix onboarding recovery (#288) (`df845ca2`)

## 1.2.3

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/users@1.2.3
  - @moh-sso/api@1.2.3
  - @moh-sso/auth@1.2.3
  - @moh-sso/microfrontend@1.2.3
  - @moh-sso/types@1.2.3
  - @moh-sso/ui@1.2.3

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.2`.

- No direct source changes; this release updates version or dependency metadata.

## 1.2.2

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/users@1.2.2
  - @moh-sso/api@1.2.2
  - @moh-sso/auth@1.2.2
  - @moh-sso/microfrontend@1.2.2
  - @moh-sso/types@1.2.2
  - @moh-sso/ui@1.2.2

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.1`.

- No direct source changes; this release updates version or dependency metadata.

## 1.2.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/users@1.2.1
  - @moh-sso/api@1.2.1
  - @moh-sso/auth@1.2.1
  - @moh-sso/microfrontend@1.2.1
  - @moh-sso/types@1.2.1
  - @moh-sso/ui@1.2.1

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.0`.

- Align all application root pages with the shared page layout (#269) (`cffeb506`)

## 1.2.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.2.0
  - @moh-sso/api@1.2.0
  - @moh-sso/auth@1.2.0
  - @moh-sso/microfrontend@1.2.0
  - @moh-sso/types@1.2.0
  - @moh-sso/ui@1.2.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.1.0`.

- Feat/admin rbac ux (#254) (`0bbf6bf6`)
- Platform/phase 1 application registry (#252) (`3dcac948`)

## 1.1.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/users@1.1.1
  - @moh-sso/api@1.1.1
  - @moh-sso/auth@1.1.1
  - @moh-sso/microfrontend@1.1.1
  - @moh-sso/types@1.1.1
  - @moh-sso/ui@1.1.1

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.1.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.1.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/users@1.1.0
  - @moh-sso/api@1.1.0
  - @moh-sso/auth@1.1.0
  - @moh-sso/microfrontend@1.1.0
  - @moh-sso/types@1.1.0
  - @moh-sso/ui@1.1.0

<!-- scoped-source-changes -->

### Source changes

Initial release; includes available source history.

- Unpush commits (#247) (`362f5f97`)
- chore : fix frontend modules and packages versioning gaps (#246) (`b334746d`)
- add authenticated launch url (`d1a1423f`)
- add modules to the app menu (`c85d0a8f`)
- Feat keycloak authorization groups (#203) (`622dbd94`)
- add mobile responsivenes (#200) (`c6613ef0`)
- Extract apis to features (#166) (`50f1700d`)
- Add data quality checks and validation rules (#132) (`4bb5c8e6`)
- Feat rbac (#128) (`a0420950`)

## 1.0.0

### Major Changes

- Automatic major release of all frontend apps and shared packages.

### Patch Changes

- 936e717: Use the shell version in frontend runtime build metadata, keep internal dependency ranges compatible, and clear expired authentication when an API retry remains unauthorized.
- Updated dependencies
- Updated dependencies [936e717]
  - @moh-sso/users@1.0.0
  - @moh-sso/api@1.0.0
  - @moh-sso/auth@1.0.0
  - @moh-sso/microfrontend@1.0.0
  - @moh-sso/types@1.0.0
  - @moh-sso/ui@1.0.0

<!-- scoped-source-changes -->

### Source changes

Initial release; includes available source history.

- chore : fix frontend modules and packages versioning gaps (`936e7178`)
- add authenticated launch url (`d1a1423f`)
- add modules to the app menu (`c85d0a8f`)
- Feat keycloak authorization groups (#203) (`622dbd94`)
- add mobile responsivenes (#200) (`c6613ef0`)
- Extract apis to features (#166) (`50f1700d`)
- Add data quality checks and validation rules (#132) (`4bb5c8e6`)
- Feat rbac (#128) (`a0420950`)
