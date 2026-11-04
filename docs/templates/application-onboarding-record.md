# Application onboarding record

Copy this record into the integration/change ticket. Do not record client secrets,
access/refresh tokens, or real message/patient content here. Use the
[onboarding handbook](../platform-onboarding.md) for the procedure.

## Ownership and identifiers

| Item                                              | Value |
| ------------------------------------------------- | ----- |
| Application display name and purpose              |       |
| Registry / parent client ID                       |       |
| Integration type / system type / contract version |       |
| Environment and criticality                       |       |
| Owner team, owner name, business email            |       |
| Support and documentation URLs                    |       |
| Technical contact and approver                    |       |
| Change ticket and rollout window                  |       |

## Identity and launch configuration

| Item                                                             | Value/evidence |
| ---------------------------------------------------------------- | -------------- |
| Keycloak realm / public issuer                                   |                |
| Interactive client ID                                            |                |
| Callback, origin and logout allowlists                           |                |
| Launch URL, optional authenticated launch URL, launch mode       |                |
| Parent/module launcher and side-navigation flags                 |                |
| Client roles, system roles, access roles                         |                |
| Group/user assignment plan                                       |                |
| Backend permissions / enforcement location                       |                |
| Audience and capability scopes, if platform APIs are used        |                |
| Machine client ID and service-account subject, if used           |                |
| Secret-store reference (reference only, never the secret)        |                |
| Hosted package, route and build/import-map change, if applicable |                |

## Completion evidence

- [ ] Schema/version prerequisites verified; restore plan reviewed.
- [ ] Registration saved; owner/support/launch/type fields verified.
- [ ] Enabled system roles and matching access roles configured.
- [ ] Configure, Validate, Approve and Activate actions recorded with reasons.
- [ ] Active/enabled state and authorized pilot user's Effective Access verified.
- [ ] Ordinary user's launcher and protected action pass.
- [ ] Unauthorized user is denied on direct route/API, not just hidden in UI.
- [ ] External OIDC login/logout and callback validation verified, if applicable.
- [ ] Hosted lifecycle, deep-link/reload, local and remote production paths verified, if applicable.
- [ ] Read-only machine integration/readiness verified, if applicable.
- [ ] Approved test recipient and notification replay/status evidence recorded, if applicable.
- [ ] Suspension/revocation test completed; restoration deliberately approved.
- [ ] Logs/request IDs and application operations evidence linked.
- [ ] Monitoring, support ownership, credential rotation and rollback agreed.

## Outcome and approval

| Item                                                         | Value |
| ------------------------------------------------------------ | ----- |
| Evidence links / request IDs (no tokens)                     |       |
| Known limitations and accepted risks                         |       |
| Outstanding work and owner                                   |       |
| Approver, decision, date                                     |       |
| Release artifact/image/version                               |       |
| Deployment authorization and outcome (separate from release) |       |
| Rollback reference and responsible operator                  |       |
