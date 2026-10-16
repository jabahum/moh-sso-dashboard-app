"""Local bootstrap contract tests; no Docker or running Keycloak required."""
import importlib.util
import base64
import os
import uuid
import json
from pathlib import Path
import stat
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('local_sso', Path(__file__).with_name('local-sso.py'))
bootstrap = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bootstrap)


class FakeKeycloak:
    def __init__(self):
        self.clients = {'data-statistics': {'id': 'roles', 'clientId': 'data-statistics'}}
        self.role = None
        self.grants = set()
        self.scope = set()
        self.creates = 0

    def client(self, name):
        return self.clients.get(name)

    def ensure_client(self, desired):
        return bootstrap.Keycloak.ensure_client(self, desired)

    def call(self, method, path, data=None, missing=False):
        if path == '/clients' and method == 'POST':
            self.clients[data['clientId']] = dict(data, id=data['clientId'])
            self.creates += 1
        elif method == 'PUT':
            self.clients[data['clientId']] = data
        elif path == '/clients/roles/roles/report_scheduler_service':
            return self.role
        elif path == '/clients/roles/roles':
            self.role = dict(data, id='service-role')
        elif path.endswith('/service-account-user'):
            return {'id': 'worker-account'}
        elif '/role-mappings/' in path:
            self.grants.add(data[0]['name'])
        elif '/scope-mappings/' in path:
            self.scope.add(data[0]['name'])
        elif path.endswith('/client-secret'):
            return {'value': 'existing-test-secret'}
        elif path.startswith('/clients?'):
            return list(self.clients.values())
        else:
            raise AssertionError((method, path))


class LocalBootstrapTests(unittest.TestCase):
    def test_client_provision_is_idempotent_and_keeps_scope_separate(self):
        kc = FakeKeycloak()
        first = bootstrap.reconcile_clients(kc)
        second = bootstrap.reconcile_clients(kc)
        self.assertEqual(first, second)
        self.assertEqual(kc.creates, 2)
        self.assertEqual(set(kc.clients), {'data-statistics', 'datawarehouse-report-browser', 'report-scheduler-worker'})
        browser = kc.clients[bootstrap.BROWSER_CLIENT]
        worker = kc.clients[bootstrap.WORKER_CLIENT]
        self.assertTrue(browser['publicClient'])
        self.assertTrue(browser['standardFlowEnabled'])
        self.assertFalse(browser['serviceAccountsEnabled'])
        self.assertFalse(worker['publicClient'])
        self.assertFalse(worker['standardFlowEnabled'])
        self.assertFalse(worker['directAccessGrantsEnabled'])
        self.assertFalse(worker['fullScopeAllowed'])
        self.assertTrue(worker['serviceAccountsEnabled'])
        self.assertEqual(kc.grants, {'report_scheduler_service'})
        self.assertEqual(kc.scope, {'report_scheduler_service'})
        self.assertEqual(worker['protocolMappers'][0]['config']['included.client.audience'], 'data-statistics')

    def test_existing_browser_redirects_retained(self):
        kc = FakeKeycloak()
        kc.clients[bootstrap.BROWSER_CLIENT] = {'id': 'browser', 'clientId': bootstrap.BROWSER_CLIENT, 'redirectUris': ['http://localhost:8888/*']}
        bootstrap.reconcile_clients(kc)
        self.assertIn('http://localhost:8888/*', kc.clients[bootstrap.BROWSER_CLIENT]['redirectUris'])

    def test_refuses_nonlocal_keycloak(self):
        with self.assertRaises(ValueError):
            bootstrap.Keycloak('https://auth.example.org', 'moh-realm', 'test')

    def test_runtime_secrets_are_private_and_existing_settings_preserved(self):
        with tempfile.TemporaryDirectory() as temp:
            env = Path(temp) / 'runtime.env'
            env.write_text('# Keep comment\nother=untouched\nAUTH_MODE=off\nAUTH_MODE=off\n')
            bootstrap.update_env(env, {'AUTH_MODE': 'on', 'new': 'value'})
            self.assertIn('# Keep comment', env.read_text())
            self.assertIn('other=untouched', env.read_text())
            self.assertEqual(env.read_text().count('AUTH_MODE='), 1)
            self.assertEqual(stat.S_IMODE(env.stat().st_mode), 0o600)

    def test_realm_export_contains_complete_healthbi_contract(self):
        for name in ['keycloak/realm-export.json', 'charts/moh-sso/files/dev/realm-export.json', 'keycloak/realm-export-prod.json']:
            realm = json.loads((bootstrap.ROOT / name).read_text())
            clients = {client['clientId']: client for client in realm['clients']}
            self.assertTrue(clients['datawarehouse-report-browser']['publicClient'])
            self.assertEqual(clients['datawarehouse-report-browser']['attributes']['pkce.code.challenge.method'], 'S256')
            self.assertTrue(clients['report-scheduler-worker']['serviceAccountsEnabled'])
            self.assertNotIn('secret', clients['report-scheduler-worker'])
            worker = next(user for user in realm['users'] if user.get('serviceAccountClientId') == 'report-scheduler-worker')
            self.assertEqual(worker['clientRoles'], {'data-statistics': ['report_scheduler_service']})
            self.assertIn({'client': 'report-scheduler-worker', 'roles': ['report_scheduler_service']}, realm['clientScopeMappings']['data-statistics'])

    def test_normal_startup_does_not_mutate_imported_clients(self):
        kc = FakeKeycloak()
        bootstrap.reconcile_clients(kc)
        original = kc.call
        calls = []
        def read_only(method, *args, **kwargs):
            calls.append(method)
            self.assertEqual(method, 'GET')
            return original(method, *args, **kwargs)
        kc.call = read_only
        bootstrap.imported_credential(kc)
        self.assertTrue(calls)

    def test_missing_imported_client_requires_explicit_reconciliation(self):
        with self.assertRaisesRegex(RuntimeError, 'sso-reconcile-keycloak'):
            bootstrap.imported_credential(FakeKeycloak())


    @unittest.skipUnless(os.environ.get('TEST_KEYCLOAK_IMPORT') == '1', 'set TEST_KEYCLOAK_IMPORT=1 for disposable local realm integration')
    def test_fresh_realm_import_issues_worker_token_without_reconciliation(self):
        admin = bootstrap.local_keycloak()
        source = bootstrap.realm_definition()
        name = 'healthbi-import-test-' + uuid.uuid4().hex[:12]
        payload = {
            'realm': name, 'enabled': True, 'sslRequired': 'none',
            'clientScopes': source['clientScopes'],
            'clients': [client for client in source['clients'] if client['clientId'] in
                        [bootstrap.BROWSER_CLIENT, bootstrap.WORKER_CLIENT, bootstrap.ROLE_CLIENT]],
            'roles': {'client': {bootstrap.ROLE_CLIENT: [role for role in source['roles']['client'][bootstrap.ROLE_CLIENT]
                                                       if role['name'] == bootstrap.SERVICE_ROLE]}},
            'users': [user for user in source['users'] if user.get('serviceAccountClientId') == bootstrap.WORKER_CLIENT],
            'clientScopeMappings': source['clientScopeMappings'],
        }
        created = False
        try:
            bootstrap.http_json(admin.base + '/admin/realms', 'POST', data=payload, token=admin.token)
            created = True
            kc = bootstrap.Keycloak(admin.base, name, admin.token)
            credential = bootstrap.imported_credential(kc)
            token = bootstrap.http_json(admin.base + '/realms/' + name + '/protocol/openid-connect/token', 'POST', form={
                'grant_type': 'client_credentials', 'client_id': bootstrap.WORKER_CLIENT, 'client_secret': credential,
            })['access_token']
            segment = token.split('.')[1]
            claims = json.loads(base64.urlsafe_b64decode(segment + '=' * ((-len(segment)) % 4)))
            self.assertIn(bootstrap.SERVICE_ROLE, claims['resource_access'][bootstrap.ROLE_CLIENT]['roles'])
            audience = [claims['aud']] if isinstance(claims['aud'], str) else claims['aud']
            self.assertIn(bootstrap.ROLE_CLIENT, audience)
            self.assertEqual(claims['azp'], bootstrap.WORKER_CLIENT)
        finally:
            if created:
                bootstrap.http_json(admin.base + '/admin/realms/' + name, 'DELETE', token=admin.token)


if __name__ == '__main__':
    unittest.main()
