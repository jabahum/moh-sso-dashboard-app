-- SYNTHETIC e2e accounts for POES. Keycloak subjects are the fixed user IDs in
-- keycloak/realm-e2e-overlay.json. Issuer: https://sso.e2e.test:18443/realms/moh-realm
\set issuer 'https://sso.e2e.test:18443/realms/moh-realm'
INSERT INTO users (id, username, email, full_name, role_key, country_code, account_type, is_active, locked_until) VALUES
    (1,  'both.apps',        'both.apps@e2e.test',        'Both Apps (POES)', 'poe_officer',  'UG', 'staff', true,  NULL),
    (2,  'poes.linked',      'poes.linked@e2e.test',      'Poes Linked',      'supervisor',   'UG', 'staff', true,  NULL),
    (3,  'poes.unlinked',    'poes.unlinked@e2e.test',    'Poes Unlinked',    'poe_officer',  'UG', 'staff', true,  NULL),
    (4,  'poes.noaccess',    'poes.noaccess@e2e.test',    'Poes NoAccess',    'poe_officer',  'UG', 'staff', true,  NULL),
    (5,  'poes.disabled',    'poes.disabled@e2e.test',    'Poes Disabled',    'poe_officer',  'UG', 'staff', false, NULL),
    (6,  'poes.invitee',     'poes.invitee@e2e.test',     'Poes Invitee',     'poe_officer',  'UG', 'staff', true,  NULL),
    (7,  'poes.locked',      'poes.locked@e2e.test',      'Poes Locked',      'poe_officer',  'UG', 'staff', true,  now() + interval '30 days'),
    (8,  'poes.conf1',       NULL,                        'Conformance One',  'poe_officer',  'UG', 'staff', true,  NULL),
    (9,  'poes.conf2',       NULL,                        'Conformance Two',  'poe_officer',  'UG', 'staff', true,  NULL),
    (10, 'poes.confinelig',  NULL,                        'Conformance Inel', 'poe_officer',  'UG', 'staff', false, NULL),
    (11, 'poes.filler1',     NULL,                        'Filler One',       'poe_officer',  'UG', 'staff', true,  NULL),
    (12, 'poes.filler2',     NULL,                        'Filler Two',       'poe_officer',  'UG', 'staff', true,  NULL),
    (13, 'poes.filler3',     NULL,                        'Filler Three',     'poe_officer',  'UG', 'staff', true,  NULL),
    (14, 'poes.roleless',    'poes.roleless@e2e.test',    'Poes Roleless',    '',             'UG', 'staff', true,  NULL),
    -- Per-suite fixtures (e2e/identity-sync): removal suite, browser suite.
    (15, 'removal.both',     'removal.both@e2e.test',     'Removal Both',     'poe_officer',  'UG', 'staff', true,  NULL),
    (16, 'browser.unlinked', 'browser.unlinked@e2e.test', 'Browser Unlinked', 'poe_officer',  'UG', 'staff', true,  NULL),
    (17, 'browser.noaccess', 'browser.noaccess@e2e.test', 'Browser NoAccess', 'poe_officer',  'UG', 'staff', true,  NULL);

INSERT INTO user_assignments (id, user_id, country_code, district_code, poe_code, is_primary) VALUES
    (1, 1, 'UG', 'D-E2E-01', 'POE-E2E-ALPHA', true),
    (2, 2, 'UG', 'D-E2E-02', 'POE-E2E-BRAVO', true),
    (3, 3, 'UG', 'D-E2E-01', 'POE-E2E-ALPHA', true),
    (4, 15, 'UG', 'D-E2E-03', 'POE-E2E-CHARLIE', true);

INSERT INTO user_oidc_identities (issuer, subject, user_id) VALUES
    (:'issuer', 'e2e00000-0000-4000-8000-000000000010', 1),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000201', 2),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000203', 4),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000204', 5),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000011', 15),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000013', 17);
