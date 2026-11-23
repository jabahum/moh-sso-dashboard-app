-- SYNTHETIC e2e schema for POES. Production POES gets its base schema from the
-- legacy MySQL dump (backend/cmd/migrate-dump); the e2e stack never loads a dump.
-- These are only the columns the POES backend reads or writes for sign-in,
-- identity sync and the account page. The app's own migrations
-- (infra/database/migrations/000002, 000003) run after this file. Upstream's
-- later migrations (000004 onwards) alter legacy tables this stand-in does not
-- have; the parts sign-in and identity sync need are declared here instead.
-- scheduler_jobs is left out on purpose: without it poesGo's scheduler logs an
-- error each tick and runs no maintenance job against tables that don't exist.
CREATE TABLE users (
    id bigint PRIMARY KEY,
    username text,
    email text,
    full_name text,
    name text,
    role_key text,
    country_code text,
    account_type text,
    is_active boolean,
    must_change_password boolean NOT NULL DEFAULT false,
    locked_until timestamptz,
    failed_login_count integer NOT NULL DEFAULT 0,
    last_failed_login_at timestamptz,
    password_hash text,
    password text,
    password_changed_at timestamptz,
    last_login_at timestamptz,
    last_activity_at timestamptz,
    last_login_ip text,
    last_login_ua text,
    updated_at timestamptz,
    -- Account fields poesGo reads since upstream's accounts work (phone, locale,
    -- suspension and invitation state).
    phone text,
    locale text,
    suspended_at timestamptz,
    suspension_reason text,
    invitation_token_hash text,
    invitation_expires_at timestamptz,
    invitation_accepted_at timestamptz
);

-- Sign-in sessions, as in poesGo migration 000006_accounts_security. Identity
-- sync's POST /sessions/revoke revokes these too.
CREATE TABLE auth_sessions (
    id varchar(64) PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    refresh_jti varchar(64) NOT NULL,
    previous_jti varchar(64),
    rotated_at timestamptz,
    ip varchar(45),
    user_agent varchar(300),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    revoked_reason varchar(40)
);

CREATE TABLE user_assignments (
    id bigint PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    country_code text NOT NULL,
    province_code text,
    pheoc_code text,
    district_code text,
    poe_code text,
    is_primary boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    starts_at timestamptz,
    ends_at timestamptz
);
