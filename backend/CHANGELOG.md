# Changelog

## 1.5.1

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.5.0`.

- Feat/external system rbac seed (#334) (`c59af98`)
- Consume Report Scheduler as a self-contained backend module (#331) (`e25d69a`)
- Expose local delivery worker polling status (#330) (`9006dfe`)
- Standardize request metadata and module HTTP errors (#328) (`87920fc`)
- Give Report Scheduler a module-owned HTTP contract (#326) (`d9bb6f8`)
- Centralize application transition validation policy in core (`a92cd98`)
- Reuse core permission decisions in host middleware (`c897b60`)
- Move shared background worker shutdown to core (`82efcd1`)
- Share an audit event contract across module adapters (`a57b0c2`)
- Define core request access for standalone modules (`7126ba2`)
- Move report scheduler HTTP API into standalone module (#324) (`1fd8214`)

## 1.5.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.4.0`.

- feat(announcements): integrate self-contained v0.2.0 (`8526d84`)
- release(announcements): prepare self-contained v0.2.0 (`e79ca82`)
- feat(announcements): own HTTP routes and provider composition (`fcdf300`)
- feat(announcements): own standalone PostgreSQL persistence (`cc46893`)
- feat(audit): integrate published v0.2.0 and retain core recorder (`47f76ad`)
- chore(audit): prepare self-contained v0.2.0 release (`fe1e6f6`)
- feat(audit): build self-contained HTTP and read-query package (`fc7df55`)
- feat(metrics): integrate published v0.2.0 and remove host implementation (`bc6463c`)
- chore(metrics): prepare self-contained v0.2.0 release (`ce93be8`)
- feat(metrics): build self-contained HTTP and PostgreSQL package (`c011831`)
- feat(admin-units): integrate published self-contained v0.2.0 (`138e217`)
- chore(admin-units): prepare self-contained v0.2.0 release (`ddbc5a3`)
- feat(admin-units): build self-contained HTTP and warehouse package (`d6e5433`)
- refactor(geojson): pin v0.2.0 and remove host adapter and assets (`269bf38`)
- feat(geojson): finalize v0.2.0 self-contained API contract (`2c78cc7`)
- feat(geojson): add self-contained HTTP module and embedded maps (`4d828b7`)
- refactor(modules): separate published host adapters from features (`4ad8569`)
- feat(core): add core-only startup and conditional provider wiring (`4d2ef90`)
- refactor(core): add governance and contextual policy contracts (`2f9a341`)
- refactor(core): add identity session and authorization contracts (`8419fb5`)
- refactor(core): establish portable runtime and platform boundaries (`c5bac09`)
- feat(admin-units): pin v0.1.0 and replace host core with adapters (`e57a503`)
- feat(admin-units): build and verify standalone Go package (`d299ea7`)
- feat(admin-units): register module and verify route disablement (`95d062b`)
- refactor(admin-units): isolate portable core and visibility ports (`2af6e60`)
- feat(announcements): pin v0.1.0 and replace host core with adapters (`09d0a5a`)
- feat(announcements): build and verify standalone Go package (`e384437`)
- feat(announcements): register module and enforce route disablement (`9df7f32`)
- refactor(announcements): isolate portable persistence and delivery models (`1a7a9df`)
- refactor(announcements): isolate audience storage and delivery ports (`72aeffa`)
- feat(geojson): integrate published v0.1.0 through host adapters (`64af27a`)
- release(geojson): prepare standalone v0.1.0 (`f6d681c`)
- feat(geojson): prepare verified standalone package (`547f3f5`)
- refactor(geojson): extract portable layer resolution and read errors (`ac77fc3`)
- refactor(geojson): isolate asset reader and host file adapter (`15c74dc`)
- feat(audit): integrate published v0.1.0 through host adapters (`477680d`)
- release(audit): prepare standalone v0.1.0 (`bd1bc5d`)
- feat(audit): prepare verified standalone read package (`646d147`)
- refactor(audit): separate portable cached reads and export formatting (`f15f9a5`)
- refactor(audit): isolate read repository and cache contracts (`b51766c`)
- feat(report-scheduler): integrate published v0.1.0 through host adapters (`05d0c58`)
- release(report-scheduler): prepare standalone v0.1.0 (`7589a53`)
- feat(report-scheduler): prepare verified standalone Go package (`1d38828`)
- feat(report-scheduler): own routes and worker through module lifecycle (`e1995ff`)
- refactor(report-scheduler): isolate dependency ports for extraction (`46835e2`)

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

