# Changelog

## 1.4.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.3.1`.

- Feat/modular distribution phase 6 (#305) (`a1984449`)
- Feat/modular distribution (#302) (`290a0bce`)
- feat: own shared geography in primary and replicate it to Observatory (#297) (`9bb0936e`)

## 1.3.1

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.3.0`.

- fix: recover Observatory connectivity without blocking portal startup (`7bfb7d93`)

## 1.3.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.2.3`.

- fix: keep existing clients active outside new-client onboarding (#293) (`6f214b25`)
- fix: provide National health context to every user by default (#292) (`39eb762f`)
- Feat/observatory reference geography (#290) (`599b8655`)
- feat: add health-context access control and administration (#289) (`0a35faab`)
- feat(identity): add guided client onboarding and isolated environments (#287) (`0f1cf71b`)
- Isolate surveillance database and import the root CSV datasets (#279) (`e0c7a922`)
- Automatically update generated OpenAPI contracts on pull requests (#278) (`34e056ce`)

## 1.2.3

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.2.2`.

- Align standard human-user role defaults and Keycloak provisioning (#275) (`39eab1b9`)

## 1.2.2

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.2.1`.

- No direct source changes; this release updates version or dependency metadata.

## 1.2.1

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.2.0`.

- Fix Issue Summary (#270) (`44dc912c`)

## 1.2.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.1.0`.

- Platform/phase 1 application registry (#252) (`3dcac948`)
- Allow explicit private HTTP deployments (#257) (`d6fea6c7`)

## 1.1.1

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.1.0`.

- Allow explicit private HTTP deployments (#257) (`d6fea6c7`)

## 1.1.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v0.0.1`.

- fix(ci): refresh data quality OpenAPI contract (`515dbe7f`)
- Merge remote-tracking branch 'upstream/main' into improve-nginx-config (`70ffe3c3`)
- Feature Issue Summary Charts (#253) (`83df0c04`)
- add scheduler  env generation (`64801d72`)
- add more tests (`ee60e24d`)
- move all deploy files and scripts to infro folder (`c4dd12b6`)
- move all deploy files and scripts to infro folder (`14cc00f8`)
- add align stacks (`751bffe7`)
- align compose files and separate prod from dev (`a55fe11e`)
- Feat/report scheduler v1 (#239) (`340ceb3b`)
- Add swagger documentation to backend endpoints (#250) (`99fd059b`)
- Unpush commits (#247) (`362f5f97`)
- Logout on 401 (#245) (`73081201`)
- chore : fix frontend modules and packages  versioning gaps (#246) (`b334746d`)
- Fixing Issue Tracker issues (#244) (`ce47fa20`)
- Apply the run scope to raw\_sql rules and drop the date picker from the modal (#242) (`5925284a`)
- Add Issue Summary Table to Issue Tracking Dropdown (#243) (`889be7f0`)
- Merge branch 'mohuganda:main' into main (`bf4ea743`)
- Save a table's column configuration instead of rejecting the request (`98e38b05`)
- Add disaggregation by facility (#240) (`e157a564`)
- Add the DQA changes to existing files that the previous commit omitted (`2a8c90c9`)
- feat(data\_quality): implement run scope management and scheduling for data quality scans (`f7ab5228`)
- Optmize data querying (`b8e5f19c`)
- Store DQA results in the dqa schema and promote flag filter dimensions (`f5c8a2c7`)
- Remove legacy Validation Rules feature (DQA engine = Data Validation) (`3c1c199e`)
- refactor(ApplicationTile): streamline onLaunch handling in handleLaunch function (`7ad55d1e`)
- feat(dqa): implement DQA engine components and API integration (`dcc74c9c`)
- Changes on transactions history & links on emails (#232) (`259f68d3`)
- Feature assigning an issue to a user (#231) (`d69cecf1`)
- Fix Region Filter on Issue Tracker (#230) (`b4bd1a28`)
- make backend auto reload on code change (#223) (`d035aaaa`)
- Fix series upload (#229) (`ad200e37`)
- Group issues returned by region and district (#227) (`d4912454`)
- Merge pull request #222 from jabahum/add-url-referer (`426d3825`)
- Refactored the file upload to work with the custom tables in the landing zone instead of the template tables  (#224) (`386d42a5`)
- make sure visualiser is shown on the app menu (`d5c50133`)
- add data visuliser to app menu (`db8311b6`)
- add authenticated launch url (`d1a1423f`)
- Merge pull request #221 from akileng56/feat/issue-summary (`c784b82a`)
- Add summaries of issues open & resolved per program (`6ee8a86c`)
- Merge pull request #219 from jabahum/add-modules-sidenav (`8065c56d`)
- Add Issue summary & all issues tabs (`8bed1ec7`)
- Add an endpoint to return a list of issues reported by program (`dd2d81f4`)
- Update Facility Details & Changes from data-values retrieval  (#220) (`4ca31292`)
- add modules to the app menu (`c85d0a8f`)

## 1.0.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v0.0.1`.

- add auto release of both frontend and backend for traceability (`634f0363`)
- update project and folder make files to provide release scripts (`d4a655ab`)
- chore : fix frontend modules and packages  versioning gaps (`936e7178`)
- Fixing Issue Tracker issues (#244) (`ce47fa20`)
- Apply the run scope to raw\_sql rules and drop the date picker from the modal (#242) (`5925284a`)
- Add Issue Summary Table to Issue Tracking Dropdown (#243) (`889be7f0`)
- Merge branch 'mohuganda:main' into main (`bf4ea743`)
- Save a table's column configuration instead of rejecting the request (`98e38b05`)
- Add disaggregation by facility (#240) (`e157a564`)
- Add the DQA changes to existing files that the previous commit omitted (`2a8c90c9`)
- feat(data\_quality): implement run scope management and scheduling for data quality scans (`f7ab5228`)
- Optmize data querying (`b8e5f19c`)
- Store DQA results in the dqa schema and promote flag filter dimensions (`f5c8a2c7`)
- Remove legacy Validation Rules feature (DQA engine = Data Validation) (`3c1c199e`)
- refactor(ApplicationTile): streamline onLaunch handling in handleLaunch function (`7ad55d1e`)
- feat(dqa): implement DQA engine components and API integration (`dcc74c9c`)
- Changes on transactions history & links on emails (#232) (`259f68d3`)
- Feature assigning an issue to a user (#231) (`d69cecf1`)
- Fix Region Filter on Issue Tracker (#230) (`b4bd1a28`)
- make backend auto reload on code change (#223) (`d035aaaa`)
- Fix series upload (#229) (`ad200e37`)
- Group issues returned by region and district (#227) (`d4912454`)
- Merge pull request #222 from jabahum/add-url-referer (`426d3825`)
- Refactored the file upload to work with the custom tables in the landing zone instead of the template tables  (#224) (`386d42a5`)
- make sure visualiser is shown on the app menu (`d5c50133`)
- add data visuliser to app menu (`db8311b6`)
- add authenticated launch url (`d1a1423f`)
- Merge pull request #221 from akileng56/feat/issue-summary (`c784b82a`)
- Add summaries of issues open & resolved per program (`6ee8a86c`)
- Merge pull request #219 from jabahum/add-modules-sidenav (`8065c56d`)
- Add Issue summary & all issues tabs (`8bed1ec7`)
- Add an endpoint to return a list of issues reported by program (`dd2d81f4`)
- Update Facility Details & Changes from data-values retrieval  (#220) (`4ca31292`)
- add modules to the app menu (`c85d0a8f`)

