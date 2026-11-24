# Backend 1.8.0

<!-- scoped-source-changes -->
### Source changes

Changes since `backend/v1.7.0`.

- Allow keycloak: credential references in the integrations table (`d745bb3c`)
- Merge pull request #364 from jabahum/fix/service-client-roles-for-manifest-apps (`3a59fbd4`)
- Create service-client roles for manifest-imported systems and push every service client (`ce3730ff`)
- Merge pull request #363 from jabahum/fix/identity-sync-null-integration (`9ea08e6b`)
- Answer an unregistered identity-sync integration with 200 and data: null (`54e650f0`)
- Read identity-sync client secrets from Keycloak with a keycloak: credential reference (`234b07d2`)
