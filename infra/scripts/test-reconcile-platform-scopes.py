import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).parent))
spec = importlib.util.spec_from_file_location(
    'platform_scopes', Path(__file__).with_name('reconcile-platform-scopes.py'))
scopes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scopes)


class FakeKeycloak:
    def __init__(self, client_scopes, service_accounts=True):
        self.client_scopes = client_scopes
        self.service_accounts = service_accounts
        self.optional = []
        self.writes = []

    def get(self, path):
        if path == '/client-scopes':
            return [dict(s) for s in self.client_scopes]
        if path == '/clients?clientId=alerts-platform':
            return [{'id': 'client-1', 'clientId': 'alerts-platform', 'serviceAccountsEnabled': self.service_accounts}]
        if path.startswith('/clients?clientId='):
            return []
        if path == '/clients/client-1/optional-client-scopes':
            return [{'name': n} for n in self.optional]
        if path == '/clients/client-1/default-client-scopes':
            return [{'name': 'profile'}]
        raise AssertionError('unexpected GET ' + path)

    def request(self, method, path, body=None):
        self.writes.append((method, path, body))
        if method == 'POST' and path == '/client-scopes':
            self.client_scopes.append(dict(body, id='id-' + body['name']))
        if method == 'PUT' and path.startswith('/client-scopes/'):
            self.client_scopes = [body if s['id'] == body['id'] else s for s in self.client_scopes]


class PlatformScopesTest(unittest.TestCase):
    def test_catalogue_is_the_backend_file(self):
        names = [s['name'] for s in scopes.load_catalogue()]
        self.assertIn('identity:read', names)
        self.assertIn('notifications:write', names)

    def test_creates_missing_and_fixes_misconfigured_scopes_only(self):
        api = FakeKeycloak([
            {'id': 'a', 'name': 'identity:read', 'protocol': 'openid-connect', 'description': 'kept',
             'attributes': {'include.in.token.scope': 'false', 'gui.order': '7'}},
            {'id': 'b', 'name': 'notifications:read', 'protocol': 'openid-connect',
             'attributes': dict(scopes.WANTED_ATTRIBUTES)},
            {'id': 'c', 'name': 'unrelated', 'protocol': 'openid-connect', 'attributes': {}},
        ])
        plan = scopes.build_plan(api, scopes.load_catalogue())
        self.assertEqual([(p[0], p[1]) for p in plan],
                         [('update scope', 'identity:read'), ('create scope', 'notifications:write')])
        update = plan[0][4]
        self.assertEqual(update['attributes'], {'include.in.token.scope': 'true', 'gui.order': '7',
                                                'display.on.consent.screen': 'false'})
        self.assertEqual(update['description'], 'kept')
        create = plan[1][4]
        self.assertEqual(create['protocol'], 'openid-connect')
        self.assertTrue(create['description'])
        scopes.apply_plan(api, plan)
        self.assertEqual([(m, p) for m, p, _ in api.writes],
                         [('PUT', '/client-scopes/a'), ('POST', '/client-scopes')])
        self.assertEqual(scopes.build_plan(api, scopes.load_catalogue()), [])

    def test_attaches_requested_scopes_to_a_service_client(self):
        api = FakeKeycloak([])
        api.optional = ['identity:read']
        plan = scopes.build_plan(api, scopes.load_catalogue(), 'alerts-platform', ['identity:read', 'notifications:write'])
        attach = [p for p in plan if p[0] == 'attach optional scope']
        self.assertEqual([p[1] for p in attach], ['notifications:write'])
        scopes.apply_plan(api, plan)
        self.assertIn(('PUT', '/clients/client-1/optional-client-scopes/id-notifications%3Awrite', None), api.writes)

    def test_refuses_unknown_scopes_missing_clients_and_non_service_clients(self):
        catalogue = scopes.load_catalogue()
        with self.assertRaisesRegex(ValueError, 'admin:all'):
            scopes.build_plan(FakeKeycloak([]), catalogue, 'alerts-platform', ['admin:all'])
        with self.assertRaisesRegex(ValueError, 'not found'):
            scopes.build_plan(FakeKeycloak([]), catalogue, 'missing', ['identity:read'])
        with self.assertRaisesRegex(ValueError, 'service-account'):
            scopes.build_plan(FakeKeycloak([], service_accounts=False), catalogue, 'alerts-platform', ['identity:read'])

    def test_rejects_an_unsupported_catalogue(self):
        with tempfile.NamedTemporaryFile('w', suffix='.json') as handle:
            json.dump({'version': 2, 'scopes': []}, handle)
            handle.flush()
            with self.assertRaises(ValueError):
                scopes.load_catalogue(handle.name)


if __name__ == '__main__':
    unittest.main()
