-- E2E only: register each identity-sync client as an enabled service identity
-- (ihp_application_identities) of its application, with an audit event, as
-- PUT /api/v1/admin/platform/applications/:clientId/identities/:oauthClientId
-- would. The subjects are the fixed service-account user IDs in
-- keycloak/realm-e2e-overlay.json. Idempotent.
\set ON_ERROR_STOP on
BEGIN;
INSERT INTO ihp_application_identities (application_client_id, oauth_client_id, subject, scopes, enabled) VALUES
    ('clims',  'clims-identity-sync',  'e2e00000-0000-4000-8000-000000000901', '{}', true),
    ('poesgo', 'poesgo-identity-sync', 'e2e00000-0000-4000-8000-000000000902', '{}', true)
ON CONFLICT (oauth_client_id) DO UPDATE SET subject = EXCLUDED.subject, enabled = true, updated_at = now()
WHERE ihp_application_identities.application_client_id = EXCLUDED.application_client_id;

INSERT INTO ihp_rbac_audit_events (actor_user_id, action, resource_type, resource_id, system_client_id, details)
SELECT 'e2e-seed', 'application.identity.updated', 'application_identity', oauth_client_id, application_client_id,
       jsonb_build_object('clientId', oauth_client_id, 'subject', subject, 'enabled', enabled, 'reason', 'identity-sync e2e seed')
FROM ihp_application_identities WHERE oauth_client_id IN ('clims-identity-sync', 'poesgo-identity-sync');
COMMIT;

SELECT application_client_id, oauth_client_id, enabled FROM ihp_application_identities ORDER BY 1;
