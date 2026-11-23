-- SYNTHETIC e2e accounts for CLIMS, applied after the CLIMS backend has run its
-- own migrations. Keycloak subjects are the fixed user IDs in
-- keycloak/realm-e2e-overlay.json. Nobody signs in with a password: the
-- password column holds a value no hash check can match.
\set ON_ERROR_STOP on
\set issuer 'https://sso.e2e.test:18443/realms/moh-realm'
BEGIN;
INSERT INTO users (id, firstname, lastname, email, username, password, phone_number, status, created_at, updated_at) VALUES
    ('c1000000-0000-4000-8000-000000000001', 'Both',   'Apps',      'both.apps@e2e.test',        'both.apps',        '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000002', 'Clims',  'Linked',    'clims.linked@e2e.test',     'clims.linked',     '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000003', 'Clims',  'Unlinked',  'clims.unlinked@e2e.test',   'clims.unlinked',   '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000004', 'Clims',  'NoAccess',  'clims.noaccess@e2e.test',   'clims.noaccess',   '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000005', 'Clims',  'Disabled',  'clims.disabled@e2e.test',   'clims.disabled',   '!e2e-no-password', '', 'inactive', now(), now()),
    ('c1000000-0000-4000-8000-000000000006', 'Clims',  'Invitee',   'clims.invitee@e2e.test',    'clims.invitee',    '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000007', 'Clims',  'KcDisabled','clims.kcdisabled@e2e.test', 'clims.kcdisabled', '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000008', 'Conf',   'One',       NULL,                        'clims.conf1',      '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000009', 'Conf',   'Two',       NULL,                        'clims.conf2',      '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-00000000000a', 'Conf',   'Ineligible',NULL,                        'clims.confinelig', '!e2e-no-password', '', 'inactive', now(), now()),
    ('c1000000-0000-4000-8000-00000000000b', 'Filler', 'One',       NULL,                        'clims.filler1',    '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-00000000000c', 'Filler', 'Two',       NULL,                        'clims.filler2',    '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-00000000000d', 'Filler', 'Three',     NULL,                        'clims.filler3',    '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-00000000000e', 'Clims',  'Roleless',  'clims.roleless@e2e.test',   'clims.roleless',   '!e2e-no-password', '', 'active',   now(), now()),
    -- Per-suite fixtures (e2e/identity-sync): removal suite, browser suite.
    ('c1000000-0000-4000-8000-00000000000f', 'Removal', 'Both',     'removal.both@e2e.test',     'removal.both',     '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000010', 'Browser', 'Unlinked', 'browser.unlinked@e2e.test', 'browser.unlinked', '!e2e-no-password', '', 'active',   now(), now()),
    ('c1000000-0000-4000-8000-000000000011', 'Browser', 'NoAccess', 'browser.noaccess@e2e.test', 'browser.noaccess', '!e2e-no-password', '', 'active',   now(), now());

-- Every account except the roleless one holds one existing CLIMS role.
INSERT INTO role_user (user_id, role_id)
SELECT u.id, r.id FROM users u
JOIN roles r ON r.name = CASE u.username WHEN 'clims.linked' THEN 'store incharge' WHEN 'both.apps' THEN 'moh' ELSE 'vht' END
WHERE u.id::text LIKE 'c1000000-%' AND u.username <> 'clims.roleless';

INSERT INTO user_oidc_identities (issuer, subject, user_id) VALUES
    (:'issuer', 'e2e00000-0000-4000-8000-000000000010', 'c1000000-0000-4000-8000-000000000001'),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000101', 'c1000000-0000-4000-8000-000000000002'),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000103', 'c1000000-0000-4000-8000-000000000004'),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000104', 'c1000000-0000-4000-8000-000000000005'),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000011', 'c1000000-0000-4000-8000-00000000000f'),
    (:'issuer', 'e2e00000-0000-4000-8000-000000000013', 'c1000000-0000-4000-8000-000000000011');
COMMIT;

SELECT u.username, u.status, string_agg(r.name, ',') AS roles, i.subject IS NOT NULL AS linked
FROM users u LEFT JOIN role_user ru ON ru.user_id = u.id LEFT JOIN roles r ON r.id = ru.role_id
LEFT JOIN user_oidc_identities i ON i.user_id = u.id
GROUP BY u.id, u.username, u.status, i.subject ORDER BY u.id;
