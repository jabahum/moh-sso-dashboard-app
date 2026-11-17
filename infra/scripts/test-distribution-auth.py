#!/usr/bin/env python3
"""Exercise built distribution images against a disposable real Keycloak.

Uses generated fixture users/secrets, isolated Compose volumes and loopback ports.
Never reads private operator settings or contacts an existing identity provider.
"""
import argparse
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

ROOT = Path(__file__).resolve().parents[2]
SECRET = 'distribution-auth-fixture-only'
REALM = 'distribution-fixture'


def run(*args, env=None):
    result = subprocess.run(args, cwd=ROOT, env=env, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(f'{args[0]} {args[1]} failed: {result.stderr[-2500:]}')
    return result.stdout.strip()


def request(url, *, method='GET', data=None, token=None, status=200, form=False):
    headers = {}
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
    for name in ('admin', 'viewer', 'outsider'):
        roles = {'dashboard-web': ['portal_user']}
        if name == 'viewer':
            roles['fixture-reports'] = ['viewer']
        users.append({'username': name, 'enabled': True, 'emailVerified': True,
                      'firstName': name, 'lastName': 'Fixture', 'email': name + '@fixture.example.org',
                      'realmRoles': ['admin' if name == 'admin' else 'user'], 'clientRoles': roles,
                      'credentials': [{'type': 'password', 'value': SECRET, 'temporary': False}]})
    users.append({'username': 'service-account-dashboard-admin', 'enabled': True,
                  'serviceAccountClientId': 'dashboard-admin',
                  'clientRoles': {'realm-management': ['realm-admin']}})
    return {'realm': REALM, 'enabled': True, 'sslRequired': 'none',
            'roles': {'realm': [{'name': 'admin'}, {'name': 'user'}],
                      'client': {'fixture-reports': [{'name': 'viewer'}], 'dashboard-web': [{'name': 'portal_user'}]}},
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
                {'clientId': 'fixture-reports', 'enabled': True, 'publicClient': True}],
            'users': users}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--backend-image', required=True)
    args = parser.parse_args()
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
        model = json.loads(run('docker', 'compose', '--env-file', 'infra/env/prod.env.example',
                               '-f', 'infra/compose/docker-compose.distribution.yml', 'config', '--format', 'json', env=env))
        services = model['services']
        services.pop('nginx')
        services.pop('frontend')
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
        services['keycloak'] = {'image': 'quay.io/keycloak/keycloak:26.0.5', 'networks': ['sso'],
            'environment': {'KC_DB': 'dev-file', 'KC_HOSTNAME': 'http://keycloak:8080',
                            'KC_HTTP_ENABLED': 'true', 'KC_HEALTH_ENABLED': 'true'},
            'command': ['start-dev', '--import-realm'],
            'volumes': [str(realm) + ':/opt/keycloak/data/import/realm.json:ro'],
            'ports': [{'target': 8080, 'published': '0', 'host_ip': '127.0.0.1'}]}
        path = tmp / 'compose.json'
        path.write_text(json.dumps(model))
        compose = ['docker', 'compose', '-p', project, '-f', str(path)]
        try:
            run(*compose, 'up', '-d', 'keycloak', 'backend-db', 'observatory-db', 'redis')
            kc_port = run(*compose, 'port', 'keycloak', '8080').rsplit(':', 1)[1]
            kc = 'http://127.0.0.1:' + kc_port + '/realms/' + REALM
            wait_ready(kc + '/.well-known/openid-configuration')
            print('Keycloak fixture ready', flush=True)
            run(*compose, 'up', '-d', '--no-build', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            print('Fresh backend migrations and readiness passed', flush=True)

            def token(name):
                return request(kc + '/protocol/openid-connect/token', method='POST', form=True,
                    data={'grant_type': 'password', 'client_id': 'dashboard-web', 'client_secret': SECRET,
                          'username': name, 'password': SECRET, 'scope': 'openid profile email'})['access_token']

            admin, viewer, outsider = (token(name) for name in ('admin', 'viewer', 'outsider'))
            inventory = '/api/v1/admin/modules'
            request(api + inventory, status=401)
            request(api + inventory, token=outsider, status=403)
            modules = request(api + inventory, token=admin)['data']
            assert any(item['id'] == 'metrics' and item['version'] == 'v0.1.0' and item['enabled'] for item in modules), modules
            for module_id in ('audit', 'geojson'):
                assert any(item['id'] == module_id and item['enabled'] for item in modules), module_id
            request(api + '/api/v1/admin/audit-logs/actions', token=admin)
            request(api + '/api/v1/admin/audit-logs/actions', token=viewer, status=403)
            request(api + '/api/v1/geojson/geo_water', token=admin)
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
            backend['environment']['DISABLED_MODULES'] = 'audit,geojson'
            path.write_text(json.dumps(model))
            run(*compose, 'up', '-d', '--no-deps', '--no-build', '--force-recreate', 'backend')
            api = 'http://127.0.0.1:' + run(*compose, 'port', 'backend', '9000').rsplit(':', 1)[1]
            wait_ready(api + '/health/ready')
            admin = token('admin')
            modules = request(api + inventory, token=admin)['data']
            states = {item['id']: item['enabled'] for item in modules}
            assert states == {'metrics': True, 'audit': False, 'geojson': False}, states
            request(api + '/api/v1/admin/audit-logs/actions', token=admin, status=404)
            request(api + '/api/v1/geojson/geo_water', token=admin, status=404)

            def audit_count():
                return int(run(*compose, 'exec', '-T', 'backend-db', 'psql', '-U', 'postgres',
                               '-d', 'sso_dashboard', '-At', '-c', 'SELECT count(*) FROM audit_logs'))

            before = audit_count()
            manifests = api + '/api/v1/admin/rbac/application-manifests'
            assert request(manifests + '/import', method='POST', data=body, token=admin)['data']['operation'] == 'noop'
            assert audit_count() > before, 'Disabling audit read APIs must not stop core event recording'
            print('Module disablement preserves core audit recording', flush=True)
            print('PASS: real Keycloak authentication, module inventory, admin/role authorization, reviewed import, '
                  'lifecycle activation, idempotence, backup/restore, startup preservation, clean shutdown, restart recovery and module disablement')
        finally:
            subprocess.run([*compose, 'down', '-v', '--remove-orphans'], cwd=ROOT, capture_output=True)


if __name__ == '__main__':
    main()
