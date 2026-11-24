-- Restore migration 62's credential check (env: and file: only). Refuse while
-- an integration still uses a keycloak: reference, rather than leave it
-- unusable: re-register it with env: or file: first.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM ihp_identity_sync_integrations WHERE credential_ref LIKE 'keycloak:%') THEN
        RAISE EXCEPTION 'identity-sync integrations still use keycloak: credential references; re-register them with env: or file: before rolling back';
    END IF;
END;
$$;
ALTER TABLE ihp_identity_sync_integrations
    DROP CONSTRAINT ihp_identity_sync_integrations_credential_ref_check;
ALTER TABLE ihp_identity_sync_integrations
    ADD CONSTRAINT ihp_identity_sync_integrations_credential_ref_check CHECK (
        credential_ref ~ '^(env:[A-Z][A-Z0-9_]*|file:/[^\s]+)$'
    );
