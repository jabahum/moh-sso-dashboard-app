# @moh-sso/microfrontend

## 1.6.0

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
- Updated dependencies [e017093]
  - @moh-sso/auth@1.6.0
  - @moh-sso/state@1.6.0
  - @moh-sso/ui@1.6.0

<!-- scoped-source-changes -->
### Source changes

Changes since `frontend/v1.5.0`.

- Standardize frontend module router basenames (#329) (`35f0b49`)

## 1.5.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/auth@1.5.0
  - @moh-sso/state@1.5.0
  - @moh-sso/ui@1.5.0

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
  - @moh-sso/auth@1.4.1
  - @moh-sso/state@1.4.1
  - @moh-sso/ui@1.4.1

## 1.4.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/auth@1.4.0
  - @moh-sso/state@1.4.0
  - @moh-sso/ui@1.4.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.3.2`.

- No direct source changes; this release updates version or dependency metadata.

## 1.3.2

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/auth@1.3.2
  - @moh-sso/state@1.3.2
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
  - @moh-sso/auth@1.3.1
  - @moh-sso/state@1.3.1
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
  - @moh-sso/auth@1.3.0
  - @moh-sso/state@1.3.0
  - @moh-sso/ui@1.3.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.3`.

- feat: add health-context access control and administration (#289) (`0a35faab`)
- Isolate surveillance database and import the root CSV datasets (#279) (`e0c7a922`)

## 1.2.3

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/auth@1.2.3
  - @moh-sso/state@1.2.3
  - @moh-sso/ui@1.2.3

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.2`.

- No direct source changes; this release updates version or dependency metadata.

## 1.2.2

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/auth@1.2.2
  - @moh-sso/state@1.2.2
  - @moh-sso/ui@1.2.2

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.1`.

- No direct source changes; this release updates version or dependency metadata.

## 1.2.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/auth@1.2.1
  - @moh-sso/state@1.2.1
  - @moh-sso/ui@1.2.1

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.2.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.2.0

### Minor Changes

- Automatic minor release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/auth@1.2.0
  - @moh-sso/state@1.2.0
  - @moh-sso/ui@1.2.0

<!-- scoped-source-changes -->

### Source changes

Changes since `frontend/v1.1.0`.

- No direct source changes; this release updates version or dependency metadata.

## 1.1.1

### Patch Changes

- Automatic patch release of all frontend apps and shared packages.
- Updated dependencies
  - @moh-sso/auth@1.1.1
  - @moh-sso/state@1.1.1
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
  - @moh-sso/auth@1.1.0
  - @moh-sso/state@1.1.0
  - @moh-sso/ui@1.1.0

<!-- scoped-source-changes -->

### Source changes

Initial release; includes available source history.

- Unpush commits (#247) (`362f5f97`)
- chore : fix frontend modules and packages versioning gaps (#246) (`b334746d`)
- Refactored the file upload to work with the custom tables in the landing zone instead of the template tables (#224) (`386d42a5`)
- load permissions gaurds (#199) (`8e038864`)
- Fix self service routing (#182) (`e2bf3966`)
- Add data quality checks and validation rules (#132) (`4bb5c8e6`)
- Feat rbac (#128) (`a0420950`)
- Fix forgot password (#122) (`df457dd0`)
- Frontend docker prod deployment (#93) (`757cb3de`)
- add documentation and dependencies (#90) (`2f6fc1cf`)
- fix docker build error (#89) (`15472b87`)
- Feat email notification dispatch (#87) (`79e072cd`)

## 1.0.0

### Major Changes

- Automatic major release of all frontend apps and shared packages.

### Patch Changes

- Updated dependencies
  - @moh-sso/auth@1.0.0
  - @moh-sso/state@1.0.0
  - @moh-sso/ui@1.0.0

<!-- scoped-source-changes -->

### Source changes

Initial release; includes available source history.

- chore : fix frontend modules and packages versioning gaps (`936e7178`)
- Refactored the file upload to work with the custom tables in the landing zone instead of the template tables (#224) (`386d42a5`)
- load permissions gaurds (#199) (`8e038864`)
- Fix self service routing (#182) (`e2bf3966`)
- Add data quality checks and validation rules (#132) (`4bb5c8e6`)
- Feat rbac (#128) (`a0420950`)
- Fix forgot password (#122) (`df457dd0`)
- Frontend docker prod deployment (#93) (`757cb3de`)
- add documentation and dependencies (#90) (`2f6fc1cf`)
- fix docker build error (#89) (`15472b87`)
- Feat email notification dispatch (#87) (`79e072cd`)
