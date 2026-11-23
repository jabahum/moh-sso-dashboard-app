-- SYNTHETIC e2e schema for POES. Production POES gets its base schema from the
-- legacy MySQL dump (backend/cmd/migrate-dump); the e2e stack never loads a dump.
-- These are only the columns the POES backend reads or writes for sign-in,
-- identity sync and the account page. The app's own migrations
-- (infra/database/migrations/000002, 000003) run after this file.
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
    updated_at timestamptz
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
    ends_at timestamptz
);
