CREATE TABLE ihp_application_identities (
    oauth_client_id TEXT PRIMARY KEY,
    application_client_id TEXT NOT NULL REFERENCES ihp_systems(client_id),
    subject TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ihp_application_identities_application_idx ON ihp_application_identities(application_client_id);
