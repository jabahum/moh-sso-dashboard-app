#!/usr/bin/env python3
import copy
import importlib.util
import json
from pathlib import Path
import stat
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('shared_bootstrap', Path(__file__).with_name('bootstrap-keycloak-realms.py'))
bootstrap = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bootstrap)
URLS = {'portalUrl': 'https://portal.example.invalid', 'apiUrl': 'https://portal.example.invalid/api/v1'}


class FakeAdmin:
    base = 'https://fixture.example.invalid'
    def __init__(self):
        self.realms = {'moh-realm': {'realm': 'moh-realm', 'marker': 'production'}, 'moh-staging': {'realm': 'moh-staging', 'marker': 'existing'}}
        self.calls = []
        self.post_status = 201

    def request(self, method, path, body=None):
        self.calls.append((method, path, copy.deepcopy(body)))
        if method == 'GET':
            name = path.removeprefix('/realms/')
            return (200, self.realms[name]) if name in self.realms else (404, None)
        if method == 'POST' and self.post_status == 201:
            self.realms[body['realm']] = copy.deepcopy(body)
        return self.post_status, None


class SharedBootstrap(unittest.TestCase):
    def test_dry_run_create_only_idempotence_and_private_record(self):
        api = FakeAdmin()
        protected = copy.deepcopy(api.realms)
        plan = bootstrap.build_plan(api, {key: URLS for key in bootstrap.NON_PRODUCTION})
        self.assertFalse(any(call[0] != 'GET' for call in api.calls))
        self.assertEqual([row['action'] for row in plan], ['create', 'create', 'skip-existing'])
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'created.json'
            bootstrap.apply_plan(api, plan, path)
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            self.assertEqual([r['status'] for r in json.loads(path.read_text())['realms']], ['created-verified', 'created-verified', 'untouched'])
            with self.assertRaises(FileExistsError):
                bootstrap.apply_plan(api, plan, path)
            api.calls.clear()
            repeat = bootstrap.build_plan(api, {key: URLS for key in bootstrap.NON_PRODUCTION})
            bootstrap.apply_plan(api, repeat, Path(tmp) / 'repeat.json')
        self.assertTrue(all(method == 'GET' for method, _, _ in api.calls))
        self.assertTrue(all('moh-realm' not in path for _, path, _ in api.calls))
        self.assertEqual(api.realms['moh-realm'], protected['moh-realm'])
        self.assertEqual(api.realms['moh-staging'], protected['moh-staging'])

    def test_production_bad_urls_and_foreign_plans_rejected_before_writes(self):
        api = FakeAdmin()
        for configuration in ({'production': URLS}, {'unknown': URLS}, {'development': {'portalUrl': 'invalid', 'apiUrl': 'invalid'}}):
            with self.assertRaises(ValueError):
                bootstrap.build_plan(api, configuration)
        self.assertEqual(api.calls, [])
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(ValueError):
                bootstrap.apply_plan(api, [{'environment': 'production', 'realm': 'moh-realm', 'action': 'create', 'definition': {'realm': 'moh-realm'}}], Path(tmp) / 'bad.json')
            self.assertFalse((Path(tmp) / 'bad.json').exists())

    def test_concurrent_creation_and_rejections_do_not_overwrite(self):
        api = FakeAdmin()
        plan = bootstrap.build_plan(api, {'development': URLS})
        api.realms['moh-dev'] = {'marker': 'created-concurrently'}
        with tempfile.TemporaryDirectory() as tmp:
            bootstrap.apply_plan(api, plan, Path(tmp) / 'race.json')
            self.assertFalse(any(method == 'POST' for method, _, _ in api.calls))
            api = FakeAdmin()
            plan = bootstrap.build_plan(api, {'development': URLS})
            api.post_status = 403
            path = Path(tmp) / 'denied.json'
            with self.assertRaisesRegex(RuntimeError, 'HTTP 403'):
                bootstrap.apply_plan(api, plan, path)
            self.assertEqual(json.loads(path.read_text())['realms'][0]['status'], 'creation-rejected')

    def test_operator_credentials_and_transport_never_target_production(self):
        env = {'KEYCLOAK_BASE_URL': 'https://auth.example.invalid', 'KEYCLOAK_BOOTSTRAP_CLIENT_ID': 'fixture', 'KEYCLOAK_BOOTSTRAP_CLIENT_SECRET': 'fixture-secret'}
        api = bootstrap.ServerAdmin(env)
        calls = []
        def request(method, url, payload=None, headers=None):
            calls.append((method, url))
            return (200, {'access_token': 'fixture-token', 'expires_in': 60}) if '/token' in url else (404, None)
        api._request = request
        self.assertEqual(api.request('GET', '/realms/moh-dev')[0], 404)
        for method, path, body in [('GET', '/realms/moh-realm', None), ('PUT', '/realms/moh-dev', {}), ('DELETE', '/realms/moh-test', None), ('POST', '/realms', {'realm': 'moh-realm'})]:
            before = len(calls)
            with self.assertRaises(ValueError):
                api.request(method, path, body)
            self.assertEqual(len(calls), before)
        for url in ('http://auth.example.invalid', 'https://user:password@auth.example.invalid', 'https://auth.example.invalid?secret=value'):
            with self.assertRaises(ValueError):
                bootstrap.ServerAdmin({**env, 'KEYCLOAK_BASE_URL': url})
        with self.assertRaises(ValueError):
            bootstrap.ServerAdmin({**env, 'KEYCLOAK_BOOTSTRAP_USERNAME': 'admin', 'KEYCLOAK_BOOTSTRAP_PASSWORD': 'fixture'})


if __name__ == '__main__':
    unittest.main()
