# @moh-sso/config

## 1.0.0

### Major Changes

- Automatic major release of all frontend apps and shared packages.

### Patch Changes

- 936e717: Use the shell version in frontend runtime build metadata, keep internal dependency ranges compatible, and clear expired authentication when an API retry remains unauthorized.
- Updated dependencies
  - @moh-sso/types@1.0.0

<!-- scoped-source-changes -->
### Source changes

Initial release; includes available source history.

- chore : fix frontend modules and packages  versioning gaps (`936e7178`)
- Add the DQA changes to existing files that the previous commit omitted (`2a8c90c9`)
- fix(config): remove duplicate dataValidation.dqa key left by merge conflict (`91836667`)
- Merge branch 'main' of https://github.com/mohuganda/moh-sso-dashboard into feat-dqa (`25ddcd52`)
- Remove legacy Validation Rules feature (DQA engine = Data Validation) (`3c1c199e`)
- feat(dqa): implement DQA engine components and API integration (#233) (`f1f8d9ab`)
- feat(dqa): implement DQA engine components and API integration (`dcc74c9c`)
- add authenticated launch url (`d1a1423f`)
- Improve search of orgunits (`57be0128`)
- add data validation foundation (#190) (`d02b4f7e`)
- add sms foundation (#185) (`3aee2595`)
- fix version 404 (#184) (`92b8f761`)
- Extract apis to features (#166) (`50f1700d`)
- Add data quality checks and validation rules  (#132) (`4bb5c8e6`)
- Update Api Usage for Issues and Visualizer (#136) (`b7223506`)
- Fix merge (#130) (`9b5192ad`)
- UI improvement (#103) (`0fc21858`)
- Improve newfeeds (#99) (`11c780ea`)
- Improve announcements (#96) (`882994d0`)
- Frontend docker prod deployment (#93) (`757cb3de`)
- add documentation and dependencies (#90) (`2f6fc1cf`)
- fix docker build error (#89) (`15472b87`)
- Feat email notification dispatch (#87) (`79e072cd`)

