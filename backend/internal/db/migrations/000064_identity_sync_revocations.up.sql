-- Access removal for identity-sync applications (docs/architecture/identity-sync.md,
-- Track C). One row records that a person's access to one application was removed,
-- and the status of each removal step. While a row is active (restored_at IS NULL)
-- no sync may grant that application's access role to the subject again; only an
-- explicit restore clears it, and a restore never re-grants access by itself.
CREATE TABLE ihp_identity_sync_revocations (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_client_id TEXT NOT NULL REFERENCES ihp_identity_sync_integrations(application_client_id),
    issuer                TEXT NOT NULL CHECK (issuer ~ '^https?://[^\s]+$'),
    subject               TEXT NOT NULL CHECK (subject <> '' AND subject = btrim(subject) AND length(subject) <= 255),
    revoked_by            TEXT NOT NULL CHECK (revoked_by <> ''),
    reason                TEXT NOT NULL CHECK (reason <> '' AND length(reason) <= 2000),
    revoked_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    restored_by           TEXT,
    restore_reason        TEXT,
    restored_at           TIMESTAMPTZ,
    -- Step status: pending (not tried yet), done, failed (retried by resume) or
    -- unsupported (the application cannot do it; reported, not retried).
    role_removed          TEXT NOT NULL DEFAULT 'pending' CHECK (role_removed IN ('pending', 'done', 'failed', 'unsupported')),
    consents_revoked      TEXT NOT NULL DEFAULT 'pending' CHECK (consents_revoked IN ('pending', 'done', 'failed', 'unsupported')),
    sessions_revoked      TEXT NOT NULL DEFAULT 'pending' CHECK (sessions_revoked IN ('pending', 'done', 'failed', 'unsupported')),
    attempts              INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    -- Stable error code of the last failed step, never an upstream body or message.
    last_error            TEXT CHECK (last_error IS NULL OR last_error ~ '^[a-z][a-z0-9_]{0,63}$'),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ihp_identity_sync_revocations_restore CHECK (
        (restored_at IS NULL AND restored_by IS NULL AND restore_reason IS NULL)
        OR (restored_at IS NOT NULL AND restored_by IS NOT NULL AND restored_by <> ''
            AND restore_reason IS NOT NULL AND restore_reason <> '' AND length(restore_reason) <= 2000)
    )
);

-- At most one active revocation per application and identity.
CREATE UNIQUE INDEX ihp_identity_sync_revocations_active
    ON ihp_identity_sync_revocations (application_client_id, issuer, subject)
    WHERE restored_at IS NULL;

-- Resume scans active revocations with a step still to do.
CREATE INDEX ihp_identity_sync_revocations_unfinished
    ON ihp_identity_sync_revocations (application_client_id, revoked_at)
    WHERE restored_at IS NULL
      AND (role_removed IN ('pending', 'failed')
        OR consents_revoked IN ('pending', 'failed')
        OR sessions_revoked IN ('pending', 'failed'));
