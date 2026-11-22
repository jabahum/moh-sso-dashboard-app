# Backend 1.5.1

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
