# @moh-sso/ui

## 1.12.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.
- e752ec8: Add `PageLoading` and `useFirstLoad` to `@moh-sso/ui`, and use them for each app's first-load indicator instead of per-app `InlineLoading` code.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.12.0

<!-- scoped-source-changes -->
### Source changes

Changes since `frontend/v1.10.0`.

- Share the first-load indicator as PageLoading and useFirstLoad (`e752ec86`)

## 1.11.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.11.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.9.0`.

- Serve the portal from portals.health.go.ug (`ba02bee7`)
- Serve the portal at the site root instead of /portal/ (`0dde9e56`)

## 1.10.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.10.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.8.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.9.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.9.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.7.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.8.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.8.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.6.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.7.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.
- 95a3edd: Dock shared side panels alongside portal content on desktop. Disable docking globally in runtime configuration or for an individual panel.

### Patch Changes

- d83ac9c: Render `PublicFooter` in normal page flow instead of fixed to the viewport, so it no longer covers the end of long pages.
- 77ef3f4: Make the user header logo a link to the portal home, so home stays reachable when the title is hidden on small screens.
- Updated dependencies
  - @moh-sso/types@1.7.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.5.1`.

- Unify shell header actions across admin and user layouts (#342) (`77ef3f40`)
- Keep the portal footer in page flow so it stops covering content (#344) (`d83ac9cd`)

## 1.6.0

### Minor Changes

- e017093: Dock shared side panels alongside portal content on desktop. Disable docking globally in runtime configuration or for an individual panel.

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/types@1.6.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.5.0`.

- feat(frontend): make desktop side panel docking configurable (#335) (`e017093`)

## 1.5.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.5.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.4.0`.

- fix(frontend): release shared React runtime patch 1.4.1 (#308) (`fdd4fe8`)

## 1.4.1

### Patch Changes

- 864c86e: Keep React and React DOM subpaths, including the client renderer and JSX runtimes,
  external in published shared packages. Microfrontends now use the import map's
  matching React runtime instead of loading a renderer bundled at build time, which
  could fail every published application with React version mismatch error 527.
- Updated dependencies [864c86e]
  - @moh-sso/types@1.4.1

## 1.4.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.4.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.3.2`.

- No direct source changes; this release updates version or dependency metadata.

## 1.3.2

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/types@1.3.2

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.3.1`.

- ci(frontend): publish npm modules on release tags (#300) (`851c9bc8`)
- Release/frontend modules npm (#298) (`fcb6d3a4`)

## 1.3.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/types@1.3.1

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.3.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.3.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.3.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.3`.

- fix: avoid secure-context UUID APIs for UI identifiers (#291) (`d35ba08a`)
- feat: add health-context access control and administration (#289) (`0a35faab`)
- Refine admin workspaces and fix onboarding recovery (#288) (`df845ca2`)
- Align nested sidebar folder and page indentation (#285) (`89891b52`)
- Isolate surveillance database and import the root CSV datasets (#279) (`e0c7a922`)

## 1.2.3

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/types@1.2.3

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.2`.

- Improve overview density and notification table, feed and details (#276) (`3dc88570`)
- Group application launcher entries by purpose (#274) (`10b1babe`)

## 1.2.2

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/types@1.2.2

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.1`.

- Refine shell navigation, launcher and account experience (#272) (`aaa66d4b`)

## 1.2.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/types@1.2.1

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.0`.

- Align all application root pages with the shared page layout (#269) (`cffeb506`)

## 1.2.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.2.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.1.0`.

- Feat/admin rbac ux (#254) (`0bbf6bf6`)
- Platform/phase 1 application registry (#252) (`3dcac948`)
- Show one application version in the footer and About page (#264) (`aa9e7968`)

## 1.1.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/types@1.1.1

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.1.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.1.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.1.0

<!-- scoped-source-changes -->

### Source changes

Initial release; includes available source history.

- Unpush commits (#247) (`362f5f97`)
- chore : fix frontend modules and packages versioning gaps (#246) (`b334746d`)
- Add Issue Summary Table to Issue Tracking Dropdown (#243) (`889be7f0`)
- remove breadcrumb icon and also change the sidenav icon from menu to forward and backward arrows (#237) (`7015d4a5`)
- add modules to the app menu (`c85d0a8f`)
- Clean inline styles (#202) (`1e501c12`)
- Document UI alignment (#201) (`d2f40df4`)
- add mobile responsivenes (#200) (`c6613ef0`)
- Improve settings (#188) (`fa9d9387`)
- Fix sidenav bug (#186) (`4779a85a`)
- add version on footer (#181) (`f8c12018`)
- Sidenav improve (#177) (`693b6b77`)
- fix production css (#161) (`99607986`)
- Add data quality checks and validation rules (#132) (`4bb5c8e6`)
- Fix forgot password (#122) (`df457dd0`)
- fix state invalid (#111) (`e61bcf9f`)
- UI improvement (#105) (`8001d0ff`)
- UI improvement (#104) (`8c0da0f4`)
- UI improvement (#103) (`0fc21858`)
- add theme styles (#102) (`f0868a60`)
- improve ui (#101) (`c629dbe5`)
- Improve newfeeds (#99) (`11c780ea`)
- Improve announcements (#96) (`882994d0`)
- Frontend docker prod deployment (#93) (`757cb3de`)
- add documentation and dependencies (#90) (`2f6fc1cf`)
- fix docker build error (#89) (`15472b87`)
- Feat email notification dispatch (#87) (`79e072cd`)

## 1.0.0

### Major Changes

- Automatic major release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/types@1.0.0

<!-- scoped-source-changes -->

### Source changes

Initial release; includes available source history.

- chore : fix frontend modules and packages versioning gaps (`936e7178`)
- Add Issue Summary Table to Issue Tracking Dropdown (#243) (`889be7f0`)
- remove breadcrumb icon and also change the sidenav icon from menu to forward and backward arrows (#237) (`7015d4a5`)
- add modules to the app menu (`c85d0a8f`)
- Clean inline styles (#202) (`1e501c12`)
- Document UI alignment (#201) (`d2f40df4`)
- add mobile responsivenes (#200) (`c6613ef0`)
- Improve settings (#188) (`fa9d9387`)
- Fix sidenav bug (#186) (`4779a85a`)
- add version on footer (#181) (`f8c12018`)
- Sidenav improve (#177) (`693b6b77`)
- fix production css (#161) (`99607986`)
- Add data quality checks and validation rules (#132) (`4bb5c8e6`)
- Fix forgot password (#122) (`df457dd0`)
- fix state invalid (#111) (`e61bcf9f`)
- UI improvement (#105) (`8001d0ff`)
- UI improvement (#104) (`8c0da0f4`)
- UI improvement (#103) (`0fc21858`)
- add theme styles (#102) (`f0868a60`)
- improve ui (#101) (`c629dbe5`)
- Improve newfeeds (#99) (`11c780ea`)
- Improve announcements (#96) (`882994d0`)
- Frontend docker prod deployment (#93) (`757cb3de`)
- add documentation and dependencies (#90) (`2f6fc1cf`)
- fix docker build error (#89) (`15472b87`)
- Feat email notification dispatch (#87) (`79e072cd`)
