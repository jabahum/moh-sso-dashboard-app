#!/usr/bin/env python3
"""Exercise built distribution images against a disposable real Keycloak.

Uses generated fixture users/secrets, isolated Compose volumes and loopback ports.
Never reads private operator settings or contacts an existing identity provider.
"""
import argparse
import base64
import csv
import io
import gzip
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

from backend_module_pins import MODULE_PREFIX, selected_modules

ROOT = Path(__file__).resolve().parents[2]
SECRET = 'distribution-auth-fixture-only'
REALM = 'distribution-fixture'


def run(*args, env=None, input=None):
    result = subprocess.run(args, cwd=ROOT, env=env, input=input, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(f'{args[0]} {args[1]} failed: {result.stderr[-2500:]}')
    return result.stdout.strip()


def status_of(url, token=None):
    headers = {'Authorization': 'Bearer ' + token} if token else {}
    try:
        with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=10) as response:
            return response.code
    except urllib.error.HTTPError as error:
        with error:
            return error.code


def request(url, *, method='GET', data=None, token=None, status=200, form=False, extra_headers=None):
    headers = dict(extra_headers or {})
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if data is not None:
        headers['Content-Type'] = 'application/x-www-form-urlencoded' if form else 'application/json'
        data = (urllib.parse.urlencode(data) if form else json.dumps(data)).encode()
    try:
        response = urllib.request.urlopen(urllib.request.Request(url, data=data, headers=headers, method=method), timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        body = response.read()
        if response.code != status:
            # Tokens and request bodies are intentionally excluded from diagnostics.
            raise AssertionError(f'{method} {urllib.parse.urlparse(url).path}: expected {status}, got {response.code}; {body[:500]!r}')
        if not body:
            return None
        content_type = response.headers.get_content_type()
        if content_type == 'application/json' or content_type.endswith('+json'):
            return json.loads(body)
        return body.decode()


def wait_ready(url, seconds=180):
    deadline = time.monotonic() + seconds
    while True:
        try:
            request(url)
            return
        except (OSError, AssertionError):
            if time.monotonic() > deadline:
                raise RuntimeError('Readiness timeout: ' + urllib.parse.urlparse(url).path)
            time.sleep(2)


def realm_fixture():
    users = []
    for name in ('admin', 'viewer', 'outsider', 'scheduler', 'map-denied'):
        roles = {'dashboard-web': ['portal_user']}
        if name == 'map-denied':
            roles = {}
        if name == 'scheduler':
            roles['data-statistics'] = ['report-scheduler_access']
        if name == 'viewer':
            roles['fixture-reports'] = ['viewer']
        users.append({'username': name, 'enabled': True, 'emailVerified': True,
                      'firstName': name, 'lastName': 'Fixture', 'email': name + '@fixture.example.org',
                      'realmRoles': [] if name == 'map-denied' else ['admin' if name == 'admin' else 'user'], 'clientRoles': roles,
                      'credentials': [{'type': 'password', 'value': SECRET, 'temporary': False}]})
    users.append({'username': 'service-account-dashboard-admin', 'enabled': True,
                  'serviceAccountClientId': 'dashboard-admin',
                  'clientRoles': {'realm-management': ['realm-admin']}})
    return {'realm': REALM, 'enabled': True, 'sslRequired': 'none',
            'roles': {'realm': [{'name': 'admin'}, {'name': 'user'}],
                      'client': {'fixture-reports': [{'name': 'viewer'}], 'dashboard-web': [{'name': 'portal_user'}],
                                 'data-statistics': [{'name': 'report-scheduler_access'}]}},
            'clients': [
                {'clientId': 'dashboard-admin', 'secret': SECRET, 'enabled': True,
                 'serviceAccountsEnabled': True, 'publicClient': False, 'protocol': 'openid-connect'},
                {'clientId': 'dashboard-web', 'secret': SECRET, 'enabled': True,
                 'publicClient': False, 'directAccessGrantsEnabled': True,
                 'standardFlowEnabled': True, 'redirectUris': ['https://portal.fixture.example.org/*'],
                 'defaultClientScopes': ['profile', 'email', 'roles'],
                 'protocolMappers': [{'name': 'user_id', 'protocol': 'openid-connect',
                     'protocolMapper': 'oidc-usermodel-property-mapper',
                     'config': {'user.attribute': 'id', 'claim.name': 'user_id', 'jsonType.label': 'String',
                                'userinfo.token.claim': 'true', 'access.token.claim': 'true'}}]},
                {'clientId': 'fixture-reports', 'enabled': True, 'publicClient': True},
                {'clientId': 'data-statistics', 'enabled': True, 'publicClient': True}],
            'users': users}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--backend-image', required=True)
    parser.add_argument('--profile', choices=('full', 'core'), default='full')
    args = parser.parse_args()
    expected_versions = {path.removeprefix(MODULE_PREFIX): version for path, version in selected_modules().items()}
    project = 'moh-auth-fixture-' + uuid.uuid4().hex[:10]
    with tempfile.TemporaryDirectory(prefix=project) as tmp:
        tmp = Path(tmp)
        secret = tmp / 'secret'
        secret.write_text(SECRET)
        secret.chmod(0o600)
        realm = tmp / 'realm.json'
        realm.write_text(json.dumps(realm_fixture()))
        env = {**os.environ, 'APP_ENV_FILE': str(ROOT / 'infra/env/prod.env.example'),
               'BACKEND_TAG': 'fixture', 'FRONTEND_TAG': 'fixture', 'DISTRIBUTION_TAG': '0.1.0-fixture',
               'DB_USER': 'postgres', 'DB_NAME': 'sso_dashboard', 'REDIS_PASSWORD': '',
               'OBSERVATORY_DB_USER': 'surveillance', 'OBSERVATORY_DB_NAME': 'observatory',
               **{key: str(secret) for key in ('DB_PASSWORD_SECRET_FILE', 'OBSERVATORY_DB_PASSWORD_SECRET_FILE',
                                              'KEYCLOAK_ADMIN_SECRET_FILE', 'KEYCLOAK_WEB_SECRET_FILE')}}
        for key in ('COMPOSE_FILE', 'COMPOSE_PROFILES', 'COMPOSE_PROJECT_NAME'):
            env.pop(key, None)
        compose_source = 'infra/compose/docker-compose.distribution.yml'
        if args.profile == 'core':
            token_key = tmp / 'token-key'
            token_key.write_text('01234567890123456789012345678901')
            token_key.chmod(0o600)
            env.update({'REDIS_PASSWORD_SECRET_FILE': str(secret),
                        'TOKEN_SYMMETRIC_KEY_SECRET_FILE': str(token_key)})
            compose_source = 'infra/compose/docker-compose.core.yml'
        model = json.loads(run('docker', 'compose', '--env-file', 'infra/env/prod.env.example',
                               '-f', compose_source, 'config', '--format', 'json', env=env))

        services = model['services']
        services.pop('nginx', None)
        services.pop('frontend', None)
        backend = services['backend']
        backend.pop('build', None)
        backend['image'] = args.backend_image
        backend['restart'] = 'no'
        backend['ports'] = [{'target': 9000, 'published': '0', 'host_ip': '127.0.0.1'}]
        backend['environment'].update({
            'ENVIRONMENT': 'production', 'DEV_AUTH_BYPASS': 'false', 'ALLOW_INSECURE_HTTP': 'true',
            'KEYCLOAK_BASE_URL': 'http://keycloak:8080', 'KEYCLOAK_INTERNAL_URL': 'http://keycloak:8080',
            'KEYCLOAK_EXTERNAL_URL': 'https://identity.fixture.example.org', 'KEYCLOAK_REALM': REALM,
            'KEYCLOAK_ADMIN_CLIENT_ID': 'dashboard-admin', 'KEYCLOAK_WEB_CLIENT_ID': 'dashboard-web',
            'KEYCLOAK_REDIRECT_URI': 'https://portal.fixture.example.org/ssobackend/api/v1/auth/callback',
            'FRONTEND_BASE_URL': 'https://portal.fixture.example.org/portal',
            'FRONTEND_REDIRECT_URI': 'https://portal.fixture.example.org/portal',
            'TOKEN_SYMMETRIC_KEY': '01234567890123456789012345678901',
            'DB_HOST': 'backend-db', 'DB_PORT': '5432', 'DB_USER': 'postgres', 'DB_NAME': 'sso_dashboard',
            'REMOTE_DB_HOST': 'backend-db', 'REMOTE_DB_PORT': '5432', 'REMOTE_DB_USER': 'postgres',
            'REMOTE_DB_NAME': 'sso_dashboard', 'REMOTE_DB_PASSWORD': SECRET,
            'DWH_HOST': 'backend-db', 'DWH_PORT': '5432', 'DWH_USERNAME': 'postgres',
            'DWH_DB': 'sso_dashboard', 'DWH_PASSWORD': SECRET, 'DQA_DB_USERNAME': '', 'DB_ENABLE_SSL': 'false',
            'RBAC_STARTUP_SYNC_ENABLED': 'true', 'RBAC_STARTUP_SEED_ENABLED': 'true',
            'RBAC_STARTUP_SYNC_LIVE_KEYCLOAK': 'false', 'RBAC_STARTUP_SYNC_REALM_EXPORT': 'false',
            'RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK': 'false', 'RBAC_STARTUP_SYNC_USERS': 'true',
            'NOTIFICATION_DELIVERY_WORKER_ENABLED': 'false', 'DISABLED_MODULES': ''})
        if args.profile == 'core':
            services['redis'].setdefault('environment', {})['REDISCLI_AUTH'] = SECRET
            services.pop('observatory-db', None)
            backend.get('depends_on', {}).pop('observatory-db', None)
            backend['secrets'] = [item for item in backend.get('secrets', [])
                                  if (item.get('source') if isinstance(item, dict) else item) != 'observatory_db_password']
            backend.pop('volumes', None)
            backend['environment'].update({
                'PLATFORM_PROFILE': 'core', 'STORAGE_PROVIDER': 'unconfigured-provider',
                'OBSERVATORY_DB_HOST': '', 'OBSERVATORY_DB_PASSWORD_FILE': '/absent/optional-secret',
                'REMOTE_DB_HOST': 'absent-remote.invalid', 'DWH_HOST': 'absent-warehouse.invalid',
                'DQA_DB_HOST': 'absent-dqa.invalid', 'SMTP_HOST': '',
                'SMTP_PASSWORD_FILE': '/absent/optional-secret', 'SMS_ENABLED': 'true',
                'SMS_PROVIDER': 'unconfigured-provider',
                'NOTIFICATION_DELIVERY_WORKER_ENABLED': 'true'})
        services['keycloak'] = {'image': 'quay.io/keycloak/keycloak:26.0.5', 'networks': ['default'] if args.profile == 'core' else ['sso'],
            'environment': {'KC_DB': 'dev-file', 'KC_HOSTNAME': 'http://keycloak:8080',
                            'KC_HTTP_ENABLED': 'true', 'KC_HEALTH_ENABLED': 'true'},
            'command': ['start-dev', '--import-realm'],
            'volumes': [str(realm) + ':/opt/keycloak/data/import/realm.json:ro'],
            'ports': [{'target': 8080, 'published': '0', 'host_ip': '127.0.0.1'}]}
        path = tmp / 'compose.json'
        path.write_text(json.dumps(model))
        compose = ['docker', 'compose', '-p', project, '-f', str(path)]
        try:
            run(*compose, 'up', '-d', 'keycloak', 'backend-db', 'redis',
                *(['observatory-db'] if args.profile == 'full' else []))
            kc_port = run(*compose, 'port', 'keycloak', '8080').rsplit(':', 1)[1]
            kc = 'http://127.0.0.1:' + kc_port + '/realms/' + REALM
            wait_ready(kc + '/.well-known/openid-configuration')
            print('Keycloak fixture ready', flush=True)
            run(*compose, 'up', '-d', '--no-build', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            print('Fresh backend migrations and readiness passed', flush=True)

            def token_bundle(name):
                return request(kc + '/protocol/openid-connect/token', method='POST', form=True,
                    data={'grant_type': 'password', 'client_id': 'dashboard-web', 'client_secret': SECRET,
                          'username': name, 'password': SECRET, 'scope': 'openid profile email'})

            def token(name):
                return token_bundle(name)['access_token']

            admin, viewer, outsider = (token(name) for name in ('admin', 'viewer', 'outsider'))
            inventory = '/api/v1/admin/modules'
            request(api + inventory, status=401)
            request(api + inventory, token=outsider, status=403)
            # Verify host credential/session adapters against real Keycloak and
            # Redis, without returning credentials through the core contracts.
            bundle = token_bundle('admin')
            session_id = uuid.uuid4().hex + uuid.uuid4().hex
            session_key = 'authsession:' + session_id
            cookie = {'Cookie': 'sso_session=' + session_id}
            stored = {key: bundle.get(key, '') for key in ('access_token', 'refresh_token', 'id_token')}
            assert run(*compose, 'exec', '-T', 'redis', 'redis-cli', '-x', 'SETEX',
                       session_key, '60', input=json.dumps(stored)) == 'OK'
            request(api + inventory, extra_headers=cookie)
            request(api + inventory, token='fixture-invalid-token', extra_headers=cookie, status=401)
            request(api + inventory, extra_headers={'Cookie': 'access_token=' + bundle['access_token']})
            request(api + '/api/v1/auth/refresh', method='POST', extra_headers=cookie)
            refreshed = json.loads(run(*compose, 'exec', '-T', 'redis', 'redis-cli', 'GET', session_key))
            assert refreshed['access_token'] and refreshed['refresh_token'], 'Refreshed session was not persisted'
            assert int(run(*compose, 'exec', '-T', 'redis', 'redis-cli', 'TTL', session_key)) > 0
            request(api + inventory, extra_headers=cookie)

            class NoRedirect(urllib.request.HTTPRedirectHandler):
                def redirect_request(self, req, fp, code, msg, headers, newurl):
                    return None

            try:
                logout = urllib.request.build_opener(NoRedirect).open(
                    urllib.request.Request(api + '/api/v1/auth/logout', headers=cookie), timeout=10)
            except urllib.error.HTTPError as error:
                logout = error
            with logout:
                assert logout.code == 307, 'Logout redirect contract changed'
                cleared = logout.headers.get_all('Set-Cookie') or []
                assert any(value.startswith('sso_session=') and 'Max-Age=0' in value for value in cleared), 'Logout did not clear the opaque cookie'
            assert run(*compose, 'exec', '-T', 'redis', 'redis-cli', 'EXISTS', session_key) == '0'
            request(api + inventory, extra_headers=cookie, status=401)
            # Redis expiration must behave like a missing opaque session.
            assert run(*compose, 'exec', '-T', 'redis', 'redis-cli', '-x', 'SETEX',
                       session_key, '60', input=json.dumps(stored)) == 'OK'
            run(*compose, 'exec', '-T', 'redis', 'redis-cli', 'EXPIRE', session_key, '0')
            request(api + inventory, extra_headers=cookie, status=401)
            admin = token('admin')
            request(api + inventory, token=admin, extra_headers=cookie)
            print('Opaque session, bearer precedence, legacy cookie, refresh persistence, logout and expiry checks passed', flush=True)
            modules = request(api + inventory, token=admin)['data']
            assert {item['id']: item['version'] for item in modules} == expected_versions, modules
            assert len(modules) == len(expected_versions), modules
            assert all(item['enabled'] == (args.profile == 'full') for item in modules), modules
            if args.profile == 'core':
                health = request(api + '/health')['data']
                assert health['status'] == 'ok' and health['components']['remoteDB'] == 'disabled', health
                assert not health['components'].get('observatory'), health
                request(api + '/api/v1/admin/rbac/systems', token=admin)
                request(api + '/api/v1/admin/rbac/systems', token=outsider, status=403)
                request(api + '/api/v1/me/health-contexts', token=viewer)
                operations = request(api + '/api/v1/admin/platform/applications/data-statistics/operations', token=admin)['data']
                assert operations['workers'] == [], operations
                for route in ('/api/v1/users', '/api/v1/clients', '/api/v1/documents',
                              '/api/v1/visualizer/admin-units?level=1',
                              '/api/v1/visualizer/adminunits/orgunits',
                              '/api/v1/admin/audit-logs', '/api/v1/admin/metrics',
                              '/api/v1/admin/metrics/overview',
                              '/api/v1/email/templates', '/api/v1/platform/notifications',
                              '/api/v1/geojson/layers', '/api/v1/announcements',
                              '/api/v1/report-scheduler/reports'):
                    request(api + route, token=admin, status=404)
                audit_count = int(run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                                     '-d', 'sso_dashboard', '-At', '-c', 'SELECT count(*) FROM audit_logs'))
                assert audit_count > 0, 'Core audit recorder stopped with optional Audit reader'
                assert run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                           '-d', 'sso_dashboard', '-At', '-c',
                           'SELECT count(*) FROM ihp_platform_worker_heartbeats') == '0', 'Optional worker started in core profile'
                assert run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                           '-d', 'sso_dashboard', '-At', '-c',
                           'SELECT count(*) FROM notification_deliveries') == '0', 'Core queued unavailable delivery work'
                run(*compose, 'stop', '-t', '30', 'backend')
                container = run(*compose, 'ps', '-aq', 'backend')
                assert run('docker', 'inspect', '-f', '{{.State.ExitCode}}', container) == '0', 'Core shutdown was not clean'
                run(*compose, 'start', 'backend')
                api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
                wait_ready(api + '/health/ready')
                assert request(api + inventory, token=token('admin'))['data'] == modules
                assert int(run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                               '-d', 'sso_dashboard', '-At', '-c', 'SELECT count(*) FROM audit_logs')) >= audit_count
                print('PASS: core-only production startup without optional providers, real Keycloak/session checks, '
                      'governance, health context, optional route removal, continued audit recording, '
                      'no optional workers/delivery queue, clean shutdown and restart', flush=True)
                return
            # The fixture's warehouse uses its isolated backend database. Supply
            # only the dimension contract used by Administrative Units.
            warehouse_sql = '''CREATE SCHEMA IF NOT EXISTS dwh;
                CREATE TABLE dwh.dim_org_hierarchy (
                    dim_org_hierarchy_key bigint, org_unit_id text, org_unit_name text, level text,
                    country_uid text, region_uid text, region text, district_uid text, district text,
                    sub_county_uid text, sub_county text, division_uid text, division text,
                    facility_uid text, facility_name text, row_version bigint, is_current boolean);
                INSERT INTO dwh.dim_org_hierarchy
                SELECT n, 'fixture-' || n, (ARRAY['Uganda','Central','District','LocalGovt','Subcounty','Hospital'])[n],
                    n::text, 'ug', 'r1', 'Central', 'd1', 'District', 's1', 'Subcounty',
                    'division1', 'Division', 'f1', 'Hospital', 1, true
                FROM generate_series(1,6) AS n;'''
            run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                '-d', 'sso_dashboard', '-v', 'ON_ERROR_STOP=1', input=warehouse_sql)
            national_id = run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                '-d', 'sso_dashboard', '-At', '-c', "SELECT id FROM health_context_nodes WHERE code='UG' AND enabled=true")
            assert national_id, 'National context must exist in the disposable fixture'
            admin_unit_headers = {'X-Health-Context-ID': national_id}
            admin_unit_paths = [('GET', 'orgunits'), ('GET', 'facilities'), ('GET', 'district'),
                ('POST', 'subcounties'), ('POST', 'localgovt'), ('POST', 'districts'),
                ('GET', 'region'), ('GET', 'national'), ('GET', 'hierarchy')]
            admin_unit_reader = token('outsider')
            admin_unit_data = {}
            for method, suffix in admin_unit_paths:
                url = api + '/api/v1/visualizer/adminunits/' + suffix
                payload = {} if method == 'POST' else None
                request(url, method=method, data=payload, status=401)
                request(url, method=method, data=payload, token=token('map-denied'), status=403)
                request(url, method=method, data=payload, token=admin, extra_headers=admin_unit_headers, status=403)
                request(url, method=method, data=payload, token=admin_unit_reader, status=400)
                response = request(url, method=method, data=payload, token=admin_unit_reader, extra_headers=admin_unit_headers)
                assert response['data'], 'Expected warehouse fixture rows: ' + suffix
                admin_unit_data[suffix] = response['data']
            print('Administrative Units inventory, nine warehouse routes and authentication/context checks passed', flush=True)
            announcement_api = '/api/v1/admin/announcements'
            announcement = request(api + announcement_api, method='POST', token=admin, status=201,
                data={'title': 'Module fixture', 'message': 'Registry delivery check', 'level': 'INFO',
                      'status': 'DRAFT', 'audience_type': 'ALL_USERS', 'notify_by_email': False})['data']
            announcement_id = announcement['id']
            attachment_text = 'name,value\nregistry,fixture\n'
            attachment = request(api + announcement_api + '/' + announcement_id + '/attachments',
                method='POST', token=admin, status=201,
                data={'file_name': 'registry.csv', 'content_type': 'text/csv',
                      'data_base64': base64.b64encode(attachment_text.encode()).decode(),
                      'include_in_email': False})['data']
            announcement_downloads = [
                '/api/v1/announcements/' + announcement_id + '/attachments/' + attachment['id'] + '/download',
                announcement_api + '/' + announcement_id + '/attachments/' + attachment['id'] + '/download']
            for download in announcement_downloads:
                request(api + download, status=401)
                request(api + download, token=token('map-denied'), status=403)
                if download.startswith('/api/v1/announcements/'):
                    request(api + download, token=admin, status=404)
                else:
                    assert request(api + download, token=admin) == attachment_text
            request(api + announcement_api, token=token('map-denied'), status=403)
            request(api + announcement_api, method='POST', data={}, token=outsider, status=403)
            request(api + announcement_api + '/' + announcement_id + '/publish',
                    method='POST', data={}, token=outsider, status=403)
            request(api + announcement_api + '/' + announcement_id + '/publish', method='POST', data={}, token=admin)
            for path_suffix in ('public', 'active'):
                items = request(api + '/api/v1/announcements/' + path_suffix)['data']
                assert any(item['id'] == announcement_id for item in items), items
            request(api + '/api/v1/announcements/me', token=outsider, status=403)
            request(api + '/api/v1/announcements/me', token=admin)
            for download in announcement_downloads:
                assert request(api + download, token=admin) == attachment_text
            private = request(api + announcement_api, method='POST', token=admin, status=201,
                data={'title': 'Private audience fixture', 'message': 'Must remain scoped', 'level': 'INFO',
                      'status': 'PUBLISHED', 'audience_type': 'SPECIFIC_USERS',
                      'user_ids': ['ffffffff-ffff-ffff-ffff-ffffffffffff']})['data']
            for audience_path in ('/api/v1/announcements/public', '/api/v1/announcements/active', '/api/v1/announcements/me'):
                items = request(api + audience_path, token=admin)['data']
                assert not any(item['id'] == private['id'] for item in items), 'Private audience leaked: ' + audience_path
            private_attachment = request(api + announcement_api + '/' + private['id'] + '/attachments',
                method='POST', token=admin, status=201,
                data={'file_name': 'private.csv', 'content_type': 'text/csv',
                      'data_base64': base64.b64encode(attachment_text.encode()).decode(),
                      'include_in_email': False})['data']
            request(api + '/api/v1/announcements/' + private['id'] + '/attachments/' + private_attachment['id'] + '/download',
                    token=admin, status=404)
            print('Announcements routes, permissions, publication, attachment streams and private-audience isolation passed', flush=True)
            request(api + '/api/v1/admin/audit-logs/actions', token=admin)
            request(api + '/api/v1/admin/audit-logs/actions', token=viewer, status=403)
            audit_range = 'from=1970-01-01T00:00:00Z&to=2100-01-01T00:00:00Z'
            audit_list = api + '/api/v1/admin/audit-logs?' + audit_range + '&limit=1'
            first_page = request(audit_list, token=admin)['data']
            assert first_page['items'], 'Expected fixture audit records'
            # Core recording invalidates audit_logs:* on every new event. Seed a
            # distinguishable typed response to prove this request uses Redis.
            first_page['items'][0]['action'] = 'fixture-cached-audit-read'
            cache_key = 'audit_logs:' + urllib.parse.urlparse(audit_list).query
            assert run(*compose, 'exec', '-T', 'redis', 'redis-cli', '-x',
                       'SETEX', cache_key, '20', input=json.dumps(first_page)) == 'OK'
            assert request(audit_list, token=admin)['data'] == first_page, 'Typed Redis cache hit was not preserved'
            request(audit_list, token=viewer, status=403)
            audit_export = api + '/api/v1/admin/audit-logs/export?' + audit_range
            exported = request(audit_export + '&format=json', token=admin)['data']
            assert exported['manifest']['record_count'] == len(exported['items'])
            assert exported['items'], 'Expected exported fixture audit records'
            exported_action = exported['items'][0]['action']
            filtered = request(audit_export + '&format=json&' +
                               urllib.parse.urlencode({'action': exported_action}), token=admin)['data']
            assert filtered['items'] and all(item['action'] == exported_action for item in filtered['items'])
            assert exported['manifest']['hash_algo'] == 'sha256' and len(exported['manifest']['payload_sha256']) == 64
            csv_rows = list(csv.reader(io.StringIO(request(audit_export + '&format=csv', token=admin))))
            assert csv_rows and csv_rows[0] == ['created_at', 'user_id', 'username', 'action', 'ip', 'client_id', 'success', 'metadata_json']
            request(audit_export + '&format=json', token=viewer, status=403)
            request(api + '/api/v1/admin/audit-logs/metrics/overview?' + audit_range, token=admin)
            request(api + '/api/v1/admin/audit-logs/metrics/failed-logins-by-day?' + audit_range, token=admin)
            request(api + '/api/v1/admin/audit-logs/metrics/top-failure-ips?' + audit_range, token=admin)
            request(api + '/api/v1/admin/audit-logs/' + exported['items'][0]['id'], token=admin)
            for _ in range(7):
                request(audit_export + '&format=json', token=admin)
            request(audit_export + '&format=json', token=admin, status=429)
            print('Audit read routes, typed cache, CSV/JSON exports and ten-per-user export limit passed', flush=True)


            geo_url = api + '/api/v1/geojson/'
            request(geo_url + 'geo_water', status=401)
            # Realm user defaults grant map access. Use a roleless principal
            # to verify denial without changing the host authorization policy.
            request(geo_url + 'geo_water', token=outsider)
            request(geo_url + 'geo_water', token=token('map-denied'), status=403)
            unknown = request(geo_url + 'not_allowed', token=admin, status=404)
            assert unknown['error']['code'] == 'NOT_FOUND'
            auth_headers = {'Authorization': 'Bearer ' + admin}
            for layer_name in ('geo_water', 'geo_districts', 'geo_subcounties'):
                with urllib.request.urlopen(urllib.request.Request(geo_url + layer_name, headers=auth_headers), timeout=10) as result:
                    assert result.code == 200 and result.headers.get_content_type() == 'application/geo+json'
                    assert result.headers.get('Cache-Control') == 'public, max-age=86400'
                    assert result.headers.get('X-Content-Type-Options') == 'nosniff'
                    raw_layer = result.read()
                    collection = json.loads(raw_layer)
                    assert collection['type'] == 'FeatureCollection' and collection['features'], layer_name
                    if layer_name == 'geo_water':
                        water = raw_layer
                        etag = result.headers.get('ETag')
            assert etag, 'Missing embedded-map identity header'
            with urllib.request.urlopen(urllib.request.Request(geo_url + 'geo_water',
                    headers={**auth_headers, 'Range': 'bytes=0-31'}), timeout=10) as result:
                assert result.code == 206 and result.read() == water[:32]
                assert result.headers.get('Content-Range') == 'bytes 0-31/' + str(len(water))
            try:
                conditional = urllib.request.urlopen(urllib.request.Request(geo_url + 'geo_water',
                    headers={**auth_headers, 'If-None-Match': etag}), timeout=10)
            except urllib.error.HTTPError as error:
                conditional = error
            with conditional:
                assert conditional.code == 304 and conditional.read() == b''
            print('GeoJSON embedded layers, headers, ranges, conditional reads and permissions passed', flush=True)
            document = {'apiVersion': 'portal.moh.go.ug/v1alpha1', 'kind': 'Application',
                'metadata': {'id': 'fixture-reports', 'name': 'Fixture Reports', 'version': '1.0.0'},
                'spec': {'environment': 'production', 'owner': {'team': 'Fixtures', 'name': 'Test Owner', 'email': 'owner@fixture.example.org'},
                         'supportUrl': 'https://support.fixture.example.org',
                         'launch': {'mode': 'microfrontend', 'route': '/apps/extensions/fixture-reports',
                                    'appName': '@fixture/reports', 'entryUrl': 'https://reports.fixture.example.org/single-spa.js'},
                         'access': {'roles': ['viewer']}}}
            manifests = api + '/api/v1/admin/rbac/application-manifests'
            body = {'document': json.dumps(document)}
            request(manifests + '/preview', method='POST', data=body, token=viewer, status=403)
            preview = request(manifests + '/preview', method='POST', data=body, token=admin)['data']
            request(manifests + '/import', method='POST', data=body, token=admin, status=409)
            body['reviewToken'] = preview['reviewToken']
            request(manifests + '/import', method='POST', data=body, token=admin)
            runtime = api + '/api/v1/applications/fixture-reports/manifest'
            request(runtime, token=viewer, status=403)
            for state in ('configured', 'validated', 'approved', 'active'):
                request(api + '/api/v1/admin/rbac/systems/fixture-reports/lifecycle', method='POST',
                        data={'status': state, 'reason': 'Disposable distribution verification'}, token=admin)
            request(runtime, token=outsider, status=403)
            assert request(runtime, token=viewer)['data']['id'] == 'fixture-reports'
            assert request(manifests + '/import', method='POST', data=body, token=admin)['data']['operation'] == 'noop'
            print('Keycloak roles, manifest review, lifecycle and idempotence passed', flush=True)
            run(*compose, 'exec', '-T', 'backend', 'sh', '-ec',
                'printf preserved > /data/uploads/distribution-fixture-marker')
            backup_hook = tmp / 'backup.sh'
            backup_hook.write_text((ROOT / 'infra/server/hooks/backup-distribution.sh.example').read_text()
                .replace('project=moh-sso-staging', 'project=' + project)
                .replace('backup_root=/var/backups/moh-sso-staging', 'backup_root=' + str(tmp / 'backups')))
            run('bash', str(backup_hook))
            backup = next((tmp / 'backups').iterdir())
            for service in ('backend-db', 'observatory-db'):
                run(*compose, 'exec', '-T', service, 'sh', '-ec',
                    'createdb -U "$POSTGRES_USER" restored_fixture')
                container = run(*compose, 'ps', '-q', service)
                with gzip.open(backup / (service + '.sql.gz'), 'rt') as stream:
                    restored = subprocess.run(['docker', 'exec', '-i', container, 'psql', '-U',
                        'postgres' if service == 'backend-db' else 'surveillance', '-d', 'restored_fixture',
                        '-v', 'ON_ERROR_STOP=1'], input=stream.read(), text=True, capture_output=True)
                assert restored.returncode == 0, 'Database backup restore failed: ' + service
            restored_count = run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                '-d', 'restored_fixture', '-At', '-c',
                "SELECT count(*) FROM ihp_rbac_audit_events WHERE action='application.manifest.imported'")
            assert restored_count == '1'
            uploads = tmp / 'restored-uploads'
            uploads.mkdir()
            run('tar', '-xzf', str(backup / 'uploads.tar.gz'), '-C', str(uploads))
            assert (uploads / 'distribution-fixture-marker').read_text() == 'preserved'
            print('Both database backups and upload restore passed', flush=True)
            run(*compose, 'stop', '--timeout', '60', 'backend')
            container = run(*compose, 'ps', '-a', '-q', 'backend')
            assert run('docker', 'inspect', '--format', '{{.State.ExitCode}}', container) == '0'
            run(*compose, 'start', 'backend')
            # Docker can allocate a different ephemeral host port on restart.
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            runtime = api + '/api/v1/applications/fixture-reports/manifest'
            manifests = api + '/api/v1/admin/rbac/application-manifests'
            wait_ready(api + '/health/ready')
            admin, viewer = token('admin'), token('viewer')
            assert request(runtime, token=viewer)['data']['version'] == '1.0.0'
            assert request(manifests + '/import', method='POST', data=body, token=admin)['data']['operation'] == 'noop'
            run(*compose, 'exec', '-T', 'backend-db', 'sh', '-ec',
                'test "$(psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT count(*) FROM ihp_rbac_audit_events WHERE action=\'application.manifest.imported\'")" = 1')
            def scheduler_runtime():
                return run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                           '-d', 'sso_dashboard', '-At', '-c',
                           "SELECT worker_id || '|' || last_heartbeat_at::text FROM report_scheduler_runtime WHERE singleton=true")

            enabled_runtime = scheduler_runtime()
            assert enabled_runtime, 'Enabled scheduler must complete persistence readiness'
            request(api + '/api/v1/report-scheduler', token=admin, status=403)
            request(api + '/api/v1/report-scheduler', token=token('scheduler'))
            request(api + '/api/v1/report-scheduler/public/deliveries/invalid/download', status=404)
            metric_suffixes = [
                'overview', 'system/count-users', 'system/count-disabled-users',
                'system/active-today', 'system/active-this-week', 'system/login-trend',
                'system/login-trend-range', 'security/failed-logins', 'security/failed-logins-range',
                'security/suspicious-logins', 'clients/count', 'clients/most-accessed',
                'clients/login-count', 'clients/active-today', 'users/new-range', 'users/new-trend',
                'users/never-logged-in', 'users/last-login/', 'users/client-usage/']
            metric_subject = run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                '-d', 'sso_dashboard', '-qAt', '-c', """
                WITH fixture_user AS (
                    INSERT INTO users(id, username, email)
                    VALUES(gen_random_uuid(), 'metrics-fixture', 'metrics-fixture@example.org')
                    RETURNING id
                )
                INSERT INTO audit_logs(user_id, action, metadata)
                SELECT id, 'login', '{"success":true,"client_id":"dashboard-web"}'::jsonb
                FROM fixture_user RETURNING user_id""")
            assert metric_subject, 'Expected isolated Metrics audit subject'
            metric_paths = ['/api/v1/admin/metrics/' + suffix +
                (metric_subject if suffix.endswith('/') else '') +
                '?start=2020-01-01T00:00:00Z&end=2100-01-01T00:00:00Z&client_id=dashboard-web'
                for suffix in metric_suffixes]
            for metric_path in metric_paths:
                request(api + metric_path, status=401)
                request(api + metric_path, token=viewer, status=403)
                request(api + metric_path, token=admin)
            backend['environment']['DISABLED_MODULES'] = 'metrics'
            path.write_text(json.dumps(model))
            run(*compose, 'up', '-d', '--no-deps', '--no-build', '--force-recreate', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            admin = token('admin')
            for metric_path in metric_paths:
                request(api + metric_path, token=admin, status=404)
            modules = request(api + inventory, token=admin)['data']
            assert any(item['id']=='metrics' and not item['enabled'] for item in modules), modules
            request(api + '/api/v1/admin/rbac/systems', token=admin)
            print('Metrics: 19 SQL-backed routes, real authentication/admin checks and complete disablement passed', flush=True)
            # Metrics disablement intentionally restarted the still-enabled scheduler.
            # Compare later scheduler disablement with that current worker.
            enabled_runtime = scheduler_runtime()
            assert enabled_runtime, 'Enabled scheduler must retain persistence readiness'

            backend['environment']['DISABLED_MODULES'] = 'audit,geojson,report-scheduler,announcements,admin-units'
            path.write_text(json.dumps(model))
            run(*compose, 'up', '-d', '--no-deps', '--no-build', '--force-recreate', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            admin = token('admin')
            modules = request(api + inventory, token=admin)['data']
            states = {item['id']: item['enabled'] for item in modules}
            extracted_modules = ('storage-locations', 'visualiser', 'document-templates',
                                 'data-quality', 'documents', 'surveillance')
            assert states == {'metrics': True, 'audit': False, 'geojson': False, 'report-scheduler': False,
                              'announcements': False, 'admin-units': False,
                              **{module: True for module in extracted_modules}}, states
            for metric_path in metric_paths:
                request(api + metric_path, token=admin)
            print('Metrics re-enablement preserves SQL-backed reads', flush=True)
            request(api + '/api/v1/admin/audit-logs/actions', token=admin, status=404)
            for audit_suffix in ('', '/metrics/overview', '/metrics/failed-logins-by-day',
                                 '/metrics/top-failure-ips', '/export', '/' + exported['items'][0]['id']):
                request(api + '/api/v1/admin/audit-logs' + audit_suffix + '?' + audit_range, token=admin, status=404)

            request(api + '/api/v1/geojson/geo_water', token=admin, status=404)
            request(api + '/api/v1/report-scheduler', token=admin, status=404)
            request(api + '/api/v1/report-scheduler', token=token('scheduler'), status=404)
            request(api + '/api/v1/report-scheduler/public/deliveries/invalid/download', status=404)
            for announcement_path in [
                    '/api/v1/announcements/public', '/api/v1/announcements/active',
                    '/api/v1/announcements/me', announcement_api, *announcement_downloads]:
                request(api + announcement_path, token=admin, status=404)
                request(api + announcement_path, status=404)
            request(api + announcement_api, method='POST', data={}, token=admin, status=404)
            request(api + announcement_api + '/' + announcement_id + '/publish',
                    method='POST', data={}, token=admin, status=404)
            for method, suffix in admin_unit_paths:
                url = api + '/api/v1/visualizer/adminunits/' + suffix
                payload = {} if method == 'POST' else None
                request(url, method=method, data=payload, token=admin, extra_headers=admin_unit_headers, status=404)
                request(url, method=method, data=payload, status=404)
            disabled_runtime = scheduler_runtime()
            assert disabled_runtime.split('|')[0] == enabled_runtime.split('|')[0], 'Disabled scheduler started a new worker'


            def audit_count():
                return int(run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                               '-d', 'sso_dashboard', '-At', '-c', 'SELECT count(*) FROM audit_logs'))

            before = audit_count()
            manifests = api + '/api/v1/admin/rbac/application-manifests'
            assert request(manifests + '/import', method='POST', data=body, token=admin)['data']['operation'] == 'noop'
            assert audit_count() > before, 'Disabling audit read APIs must not stop core event recording'
            assert scheduler_runtime() == disabled_runtime, 'Disabled scheduler updated its heartbeat'
            backend['environment']['DISABLED_MODULES'] = 'audit,geojson'
            path.write_text(json.dumps(model))
            run(*compose, 'up', '-d', '--no-deps', '--no-build', '--force-recreate', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            admin = token('admin')
            assert scheduler_runtime().split('|')[0] != disabled_runtime.split('|')[0], 'Re-enabled scheduler did not initialize a fresh worker'
            request(api + '/api/v1/report-scheduler', token=admin, status=403)
            request(api + '/api/v1/report-scheduler', token=token('scheduler'))
            modules = request(api + inventory, token=admin)['data']
            assert any(item['id'] == 'announcements' and item['enabled'] for item in modules), modules
            assert request(api + announcement_api + '/' + announcement_id, token=admin)['data']['status'] == 'PUBLISHED'
            for download in announcement_downloads:
                assert request(api + download, token=admin) == attachment_text
            assert any(item['id'] == announcement_id for item in request(api + '/api/v1/announcements/public')['data'])
            print('Announcements disablement removes every route group; re-enabling preserves records and attachments', flush=True)
            assert any(item['id'] == 'admin-units' and item['enabled'] for item in modules), modules
            admin_unit_reader = token('outsider')
            for method, suffix in admin_unit_paths:
                response = request(api + '/api/v1/visualizer/adminunits/' + suffix,
                    method=method, data={} if method == 'POST' else None,
                    token=admin_unit_reader, extra_headers=admin_unit_headers)
                assert response['data'] == admin_unit_data[suffix], 'Administrative Units data changed after re-enablement'
            print('Administrative Units disablement removes all nine routes; re-enabling preserves warehouse reads', flush=True)
            print('Module disablement preserves core audit recording; scheduler worker stops and resumes', flush=True)
            # Preserve the same primary schema, reviewed manifests and application
            # records while switching the running distribution full -> core -> full.
            full_environment = dict(backend['environment'])
            backend['environment'].update({'PLATFORM_PROFILE': 'core',
                'STORAGE_PROVIDER': 'unconfigured-provider', 'OBSERVATORY_DB_HOST': '',
                'REMOTE_DB_HOST': 'absent-remote.invalid', 'DWH_HOST': 'absent-warehouse.invalid'})
            path.write_text(json.dumps(model))
            run(*compose, 'up', '-d', '--no-deps', '--no-build', '--force-recreate', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            admin = token('admin')
            assert all(not item['enabled'] for item in request(api + inventory, token=admin)['data'])
            request(api + '/api/v1/admin/rbac/systems', token=admin)
            request(api + '/api/v1/announcements/public', status=404)
            retained_runtime = scheduler_runtime()
            request(api + '/api/v1/me/health-contexts', token=token('viewer'))
            assert scheduler_runtime() == retained_runtime, 'Core profile ran the retained scheduler'
            retained_audit = audit_count()
            backend['environment'] = full_environment
            path.write_text(json.dumps(model))
            run(*compose, 'up', '-d', '--no-deps', '--no-build', '--force-recreate', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            admin = token('admin')
            assert request(api + inventory, token=admin)['data'] == modules
            assert audit_count() >= retained_audit, 'Profile switch lost core audit records'
            assert request(api + announcement_api + '/' + announcement_id, token=admin)['data']['status'] == 'PUBLISHED'
            for download in announcement_downloads:
                assert request(api + download, token=admin) == attachment_text
            assert request(api + '/api/v1/applications/fixture-reports/manifest', token=token('viewer'))['data']['version'] == '1.0.0'
            assert scheduler_runtime().split('|')[0] != retained_runtime.split('|')[0], 'Full profile failed to resume the scheduler'
            print('Full -> core -> full profile switch preserves schema, manifests, audit, announcements and attachments', flush=True)
            # Each module extracted from the host switches off on its own.
            extracted_routes = {
                'storage-locations': '/api/v1/storage-locations',
                'visualiser': '/api/v1/visualizer/datasets',
                'document-templates': '/api/v1/document-templates',
                'data-quality': '/api/v1/data-quality/dqa/rules',
                'documents': '/api/v1/documents',
                'surveillance': '/api/v1/surveillance/diseases',
            }
            assert tuple(extracted_routes) == extracted_modules
            for module, route in extracted_routes.items():
                assert status_of(api + route, admin) != 404, f'{module}: {route} must be served while enabled'
            backend['environment']['DISABLED_MODULES'] = ','.join(extracted_modules)
            path.write_text(json.dumps(model))
            run(*compose, 'up', '-d', '--no-deps', '--no-build', '--force-recreate', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            admin = token('admin')
            states = {item['id']: item['enabled'] for item in request(api + inventory, token=admin)['data']}
            assert all(not states[module] for module in extracted_modules), states
            assert states['metrics'] and states['announcements'], states
            for route in extracted_routes.values():
                request(api + route, token=admin, status=404)
            print('Extracted modules (storage-locations, visualiser, document-templates, data-quality, '
                  'documents, surveillance) disable individually and remove their routes', flush=True)
            print('PASS: real Keycloak authentication, module inventory, admin/role authorization, reviewed import, '
                  'lifecycle activation, idempotence, backup/restore, startup preservation, clean shutdown, restart recovery and module disablement')
        finally:
            subprocess.run([*compose, 'down', '-v', '--remove-orphans'], cwd=ROOT, capture_output=True)


if __name__ == '__main__':
    main()
