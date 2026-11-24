-- Accept keycloak:<sync client ID> credential references: the dashboard reads
-- the sync client's secret from Keycloak when it needs it. The reference must
-- name the integration's own sync client, as the registry also checks.
ALTER TABLE ihp_identity_sync_integrations
    DROP CONSTRAINT ihp_identity_sync_integrations_credential_ref_check;
ALTER TABLE ihp_identity_sync_integrations
    ADD CONSTRAINT ihp_identity_sync_integrations_credential_ref_check CHECK (
        credential_ref ~ '^(env:[A-Z][A-Z0-9_]*|file:/[^\s]+)$'
        OR credential_ref = 'keycloak:' || sync_client_id
    );
