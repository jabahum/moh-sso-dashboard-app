-- Registry of applications the dashboard reconciles through the identity-sync
-- contract (docs/integrations/identity-sync-contract.md). The registry stores a
-- reference to the sync client's secret (env:NAME or file:/path), never the secret.
CREATE TABLE ihp_identity_sync_integrations (
    application_client_id TEXT PRIMARY KEY REFERENCES ihp_systems(client_id),
    origin                TEXT NOT NULL CHECK (origin ~ '^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$'),
    sync_client_id        TEXT NOT NULL UNIQUE REFERENCES ihp_application_identities(oauth_client_id),
    access_role           TEXT NOT NULL CHECK (access_role <> ''),
    credential_ref        TEXT NOT NULL CHECK (credential_ref ~ '^(env:[A-Z][A-Z0-9_]*|file:/[^\s]+)$'),
    -- Account ID format used for 1.0 applications, which have no GET / descriptor.
    account_id_format     TEXT NOT NULL CHECK (account_id_format IN ('uuid', 'positive-integer', 'opaque')),
    -- Last version read from GET /; NULL until checked.
    contract_version      TEXT CHECK (contract_version ~ '^1\.[0-9]+$'),
    enabled               BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by            TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The sync client must be the application's own enabled service identity, and the
-- access role must be one of the application's person-facing access roles. This
-- holds for every writer (admin API, CLI, ad-hoc SQL).
CREATE FUNCTION ihp_check_identity_sync_integration() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM ihp_application_identities
        WHERE oauth_client_id = NEW.sync_client_id
          AND application_client_id = NEW.application_client_id
          AND enabled
    ) THEN
        RAISE EXCEPTION 'sync client % is not an enabled service identity of application %', NEW.sync_client_id, NEW.application_client_id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'ihp_identity_sync_sync_client';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM ihp_system_access_roles ar JOIN ihp_systems s ON s.id = ar.system_id
        WHERE s.client_id = NEW.application_client_id AND ar.role_name = NEW.access_role
    ) OR EXISTS (
        SELECT 1 FROM ihp_system_roles sr JOIN ihp_systems s ON s.id = sr.system_id
        WHERE s.client_id = NEW.application_client_id AND sr.role_name = NEW.access_role AND sr.service_only
    ) THEN
        RAISE EXCEPTION 'role % is not a person-facing access role of application %', NEW.access_role, NEW.application_client_id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'ihp_identity_sync_access_role';
    END IF;
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER ihp_identity_sync_integration_guard BEFORE INSERT OR UPDATE ON ihp_identity_sync_integrations
    FOR EACH ROW EXECUTE FUNCTION ihp_check_identity_sync_integration();
