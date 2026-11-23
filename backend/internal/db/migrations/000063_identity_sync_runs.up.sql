-- Reviewed batch identity sync (docs/architecture/identity-sync.md, Track B).
-- A run is one walk of an application's eligible accounts with a proposal per
-- account; operators approve or reject rows, and apply/resume work through the
-- approved rows step by step. last_error holds a stable code, never an
-- upstream response body.
CREATE TABLE ihp_identity_sync_runs (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_client_id TEXT NOT NULL REFERENCES ihp_identity_sync_integrations(application_client_id),
    issuer                TEXT NOT NULL CHECK (issuer ~ '^https://'),
    status                TEXT NOT NULL CHECK (status IN ('previewed', 'applying', 'completed', 'failed', 'cancelled')),
    created_by            TEXT NOT NULL CHECK (btrim(created_by) <> ''),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ihp_identity_sync_runs_application_idx ON ihp_identity_sync_runs (application_client_id, created_at DESC);

CREATE TABLE ihp_identity_sync_rows (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id           UUID NOT NULL REFERENCES ihp_identity_sync_runs(id) ON DELETE CASCADE,
    account_id       TEXT NOT NULL CHECK (account_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    email_hint       TEXT NOT NULL DEFAULT '',
    proposal         TEXT NOT NULL CHECK (proposal IN ('reuse', 'create', 'review', 'linked', 'linked_no_access', 'revoked')),
    proposed_subject TEXT CHECK (proposed_subject IS NULL OR (proposed_subject <> '' AND length(proposed_subject) <= 255)),
    decision         TEXT NOT NULL DEFAULT 'pending' CHECK (decision IN ('pending', 'approved', 'rejected')),
    reviewed_by      TEXT,
    reviewed_at      TIMESTAMPTZ,
    review_note      TEXT CHECK (review_note IS NULL OR length(review_note) <= 1000),
    user_status      TEXT NOT NULL DEFAULT 'not_required' CHECK (user_status IN ('not_required', 'pending', 'invited', 'failed')),
    link_status      TEXT NOT NULL DEFAULT 'pending' CHECK (link_status IN ('pending', 'linked', 'conflict', 'ineligible', 'failed')),
    grant_status     TEXT NOT NULL DEFAULT 'pending' CHECK (grant_status IN ('pending', 'granted', 'failed', 'not_required')),
    attempts         INT  NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    -- Stable code only (for example link_conflict), never upstream bodies.
    last_error       TEXT CHECK (last_error IS NULL OR last_error ~ '^[a-z][a-z0-9_]{0,63}$'),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, account_id),
    -- A decision always names its reviewer; apply refuses rows without one.
    CONSTRAINT ihp_identity_sync_rows_reviewer CHECK (
        decision = 'pending' OR (reviewed_by IS NOT NULL AND btrim(reviewed_by) <> '' AND reviewed_at IS NOT NULL)),
    -- Rows shown as linked without access, or revoked, are never granted by a sync.
    CONSTRAINT ihp_identity_sync_rows_no_grant CHECK (
        NOT (proposal IN ('linked_no_access', 'revoked') AND grant_status = 'granted'))
);

CREATE INDEX ihp_identity_sync_rows_run_idx ON ihp_identity_sync_rows (run_id, decision, proposal);
