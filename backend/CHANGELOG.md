# Changelog

## 1.11.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.10.0`.

- Document the platform API conformance suite (`9ac8a351`)
- Run the platform API conformance suite against a real stack (`92be3910`)
- Add moh-sso platform conformance (`7d9c5d70`)
- Add a conformance suite for the platform API (`af5394f1`)
- Merge main into feat/platform-scope-catalogue (`94a5ee28`)
- Merge pull request #403 from jabahum/feat/platform-go-client (`96d207c7`)
- Add the platform scope catalogue (`c03c2f3f`)
- Add a Go client for the platform API (`46987f55`)
- Publish the platform API contract with a breaking-change check (`22c58102`)
- Add an admin network-check endpoint for identity sync (`b1a05691`)
- Add a network check for identity-sync applications (`6efaae75`)
- Explain network failures when saving identity sync (`a10967bd`)
- Merge remote-tracking branch 'upstream/main' into feat/module-migration-streams (`8833a588`)
- Format the module migrator integration test (`1bbd6e33`)
- Let compiled modules own their schema through migration streams (`e7692b40`)
- Add a standard platform capability set for compiled modules (`88ad430d`)
- feat(announcements): integrate v0.3.0 and run the scheduled publisher (`b75850b3`)
- Ship the project license inside the announcements module (`fd1f2487`)
- release(announcements): prepare v0.3.0 (`dc81063c`)
- Merge branch 'fix/announcement-remaining' into release/announcements-v0.3.0 (`73d4d52a`)
- Merge branch 'fix/announcement-create-status' into release/announcements-v0.3.0 (`c4511ff0`)
- Merge branch 'fix/announcement-scheduling' into release/announcements-v0.3.0 (`dd53af40`)
- Remove Service.ListMyAnnouncements, which ignored audience targeting (`d83f07c5`)
- Stop attaching the same file twice to announcement emails (`2425b0f8`)
- Hide only the affected announcement when its audience check fails (`b3c97c33`)
- Mark announcement email/SMS as sent only after they are handed off (`4616b602`)
- Apply the status sent with an announcement update (`dff9c962`)
- Stop announcement updates from silently widening the audience (`3fd81371`)
- Require the publish permission to create published or scheduled announcements (`a89bd4e0`)
- Publish scheduled announcements when due and stop pre-queuing their notifications (`ed3a1f48`)
- Stop email attachments from reading arbitrary server files (`ddc79c55`)
- Let a deployment declare its realm for client onboarding (`8f7164d9`)
- Serve the backend at /api instead of /ssobackend (`236e8a3f`)

## 1.10.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.9.0`.

- Serve the portal from portals.health.go.ug (`ba02bee7`)
- Serve the portal at the site root instead of /portal/ (`0dde9e56`)

## 1.9.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.8.0`.

- Add opt-in identity-sync auto-link at backend startup (`6e63753d`)

## 1.8.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.7.0`.

- Allow keycloak: credential references in the integrations table (`d745bb3c`)
- Merge pull request #364 from jabahum/fix/service-client-roles-for-manifest-apps (`3a59fbd4`)
- Create service-client roles for manifest-imported systems and push every service client (`ce3730ff`)
- Merge pull request #363 from jabahum/fix/identity-sync-null-integration (`9ea08e6b`)
- Answer an unregistered identity-sync integration with 200 and data: null (`54e650f0`)
- Read identity-sync client secrets from Keycloak with a keycloak: credential reference (`234b07d2`)

## 1.7.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.6.0`.

- Create the identity-sync service clients in Keycloak from the system RBAC seed (`9816975e`)

## 1.6.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.5.1`.

- Format Go files introduced since 1.5.1 with gofmt (`b8cd6daf`)
- License the project under MIT (`10848d29`)
- Merge docs/content-structure (with upstream/main) into docs/accuracy (`50b0444f`)
- Apply migrations 38 and 61 in the application manifest PostgreSQL tests (`4f0fb963`)
- Correct module and platform docs to match the code (`088b1440`)
- Prepare the backend/core module release with pre- and post-tag checks (`894b57e6`)
- Register an application's service identity from the Identity sync page (`963f8621`)
- Make the identity-sync local-command test independent of map order (`dda23113`)
- Check retry safety in the identity-sync conformance suite (`cd263903`)
- Add the identity-sync run and revocation admin API for the portal (`55929e8a`)
- Read user role mappings explicitly when loading a user profile (`3605a66c`)
- Harden identity-sync connectors against proxies and silent contract downgrades (`cd94e57b`)
- Let identity-sync rows for permanently missing accounts become final (`9598f4c6`)
- Let database-only identity-sync commands run without Keycloak admin flags (`8f11f82c`)
- Fail closed in deprecated identity-sync aliases when the registry is unusable (`c3da64fe`)
- Regenerate OpenAPI for identity-sync access-change warnings (`9cfaa334`)
- Surface incomplete portal revocations and restore revocations on re-grant (`cbbb7790`)
- Refuse identity-sync origins that resolve to non-public addresses (`f78861d7`)
- Keep identity-sync rows retryable when the account walk skipped them (`28245794`)
- Allow disabling an identity-sync integration after its binding broke (`abde5466`)
- Refuse linked accounts in single apply and show revoked identities in preview (`56a769da`)
- Keep a registered 1.1 identity-sync application at 1.1 when GET / returns 404 (`d5923735`)
- Serialise identity-sync grants with revocations per application and subject (`0c4a528c`)
- Let identity-sync conformance run without Keycloak admin credentials (`daa38372`)
- Regenerate sqlc models for identity-sync migrations 62-64 (`94b98af6`)
- Refuse revoked identities in single-account identity-sync apply (`f26e0a3e`)
- Wire the identity-sync tracks together in the CLI (`5a8714da`)
- Merge idsync/track-d: core adapter, conformance suite and CLI (`42681401`)
- Fix duplicate identifiers after merging Tracks B and C (`a8d16bfc`)
- Merge idsync/track-c: immediate revocation and portal hooks (`35f0b870`)
- Add identity-sync revoke, restore and revocations CLI commands (`b7983047`)
- Revoke identity-sync access when the access role is removed in the portal (`a4f1f24c`)
- Add the Keycloak access directory and portal revocation hook (`5f29d0fd`)
- Add identity-sync revocations (migration 64) and the revoke service (`07624400`)
- Add Keycloak helpers to remove one client role and revoke one client's consents (`c72dec3f`)
- Add the identity-sync run CLI (start, show, approve, reject, apply, resume) (`20c107c5`)
- Add reviewed batch identity sync and Keycloak invitations (`7d6df27c`)
- Add the registry-based identity-sync CLI (`75a3f48f`)
- Add the identity-sync registry admin API (`c2ac1fc9`)
- Document the identity-sync adapter, conformance suite and core release (`3978f580`)
- Document the test-only identity-sync session probe (`11bfffc6`)
- Add identity-sync conformance CLI command (not yet registered) (`dd44ff00`)
- Add the identity-sync app registry (migration 62) and registry service (`91688a97`)
- Upgrade the identity-sync connector to contract 1.1 with 1.0 fallback (`73d4f899`)
- Add identity-sync conformance runner to core (`053141bb`)
- Add identity-sync contract 1.1 adapter and in-memory store to core (`1306e4d6`)
- Define identity-sync contract v1.1 and the shared integration design (`7eb450e4`)
- Test service-only role migration and triggers on PostgreSQL (`a6283731`)
- Prevent service-account roles from being granted to people (`5ce06be3`)
- Use the public Keycloak issuer for CLIMS/POES identity links (`8e3714cb`)
- Add identity-sync service clients and client-credentials tokens for CLIMS/POES reconcile (`82de7d5d`)
- Feat/platform identity sync (#339) (`95a3edd7`)
- Feature Unresolved Aging Issues (#338) (`97d5fe98`)

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

