-- CI only: the test application and its machine-client bindings, as the admin API
-- would create them. The subjects are the fixed service-account user IDs in
-- keycloak/realm-moh-test.json. conformance-narrow's binding grants every scope,
-- but its Keycloak client can't request notifications:write, so its effective
-- scopes leave it out (token scope ∩ binding).
\set ON_ERROR_STOP on
BEGIN;
INSERT INTO ihp_systems (client_id, display_name, lifecycle_status, enabled)
VALUES ('conformance-app', 'Platform conformance', 'active', true)
ON CONFLICT (client_id) DO UPDATE SET lifecycle_status = 'active', enabled = true;

INSERT INTO ihp_application_identities (application_client_id, oauth_client_id, subject, scopes, enabled) VALUES
    ('conformance-app', 'conformance-bound',  'c0000000-0000-4000-8000-000000000001', '{identity:read,notifications:read,notifications:write}', true),
    ('conformance-app', 'conformance-narrow', 'c0000000-0000-4000-8000-000000000002', '{identity:read,notifications:read,notifications:write}', true)
ON CONFLICT (oauth_client_id) DO UPDATE SET subject = EXCLUDED.subject, scopes = EXCLUDED.scopes, enabled = true, updated_at = now();
COMMIT;
