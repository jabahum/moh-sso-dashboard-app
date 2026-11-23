#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))

def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result

bootstrap = module('bootstrap', 'prepare-keycloak-realm.py')
clients = module('clients', 'reconcile-keycloak-clients.py')

class Environments(unittest.TestCase):
    def test_realms_exclude_humans_credentials_and_external_app_urls(self):
        names = set()
        for environment, name in bootstrap.REALMS.items():
            realm = bootstrap.render(environment, 'https://portal.example.org/portal', 'https://portal.example.org/api/v1')
            names.add(realm['realm'])
            self.assertEqual(realm['realm'], name)
            self.assertTrue(all(u.get('serviceAccountClientId') for u in realm['users']))
            encoded = json.dumps(realm)
            for forbidden in ('"credentials"', '"secret"', '"privateKey"', 'timesheet.health.go.ug', 'dashboards.health.go.ug', 'localhost:3000'):
                self.assertNotIn(forbidden, encoded)
            web = next(c for c in realm['clients'] if c['clientId'] == 'dashboard-web')
            self.assertEqual(web['redirectUris'], ['https://portal.example.org/api/v1/auth/callback'])
        self.assertEqual(len(names), 4)
        with self.assertRaises(ValueError):
            bootstrap.render('production', 'http://localhost:3000', 'http://localhost:9000/api/v1')

    def test_profiles_and_promotion_require_target_environment_review(self):
        spec = {'version': 1, 'clientId': 'fixture-app', 'name': 'Fixture', 'owner': 'Team',
                'supportEmail': 'support@example.invalid', 'profile': 'public-web', 'environment': 'staging',
                'rootUrl': 'https://app.example.org', 'redirectUris': ['https://app.example.org/callback'],
                'logoutUris': ['https://app.example.org/login'], 'webOrigins': ['https://app.example.org']}
        client = clients.render(spec, 'staging')
        self.assertFalse(client['enabled'])
        self.assertEqual(client['attributes']['pkce.code.challenge.method'], 'S256')
        with self.assertRaises(ValueError):
            clients.render(spec, 'production')
        for bad in ('https://app.example.org/*', 'http://app.example.org/callback'):
            with self.assertRaises(ValueError):
                clients.render({**spec, 'redirectUris': [bad]}, 'staging')
        service = clients.render({**spec, 'profile': 'service', 'rootUrl': '', 'redirectUris': [], 'logoutUris': [], 'webOrigins': []}, 'staging')
        self.assertTrue(service['serviceAccountsEnabled'])
        self.assertFalse(service['standardFlowEnabled'])
        self.assertEqual(service['defaultClientScopes'], [])

def origin_attribute(realm):
    """Return the ihp_identity_sync_origin declaration from the realm's user profile."""
    providers = realm.get('components', {}).get('org.keycloak.userprofile.UserProfileProvider', [])
    for provider in providers:
        if provider.get('providerId') != 'declarative-user-profile':
            continue
        for raw in provider.get('config', {}).get('kc.user.profile.config', []):
            profile = json.loads(raw)
            for attribute in profile.get('attributes', []):
                if attribute.get('name') == 'ihp_identity_sync_origin':
                    return profile, attribute
    return None, None


class IdentitySyncUserProfile(unittest.TestCase):
    """Invitations store ihp_identity_sync_origin; Keycloak 24+ drops undeclared attributes."""

    def assert_admin_only(self, realm, where):
        profile, attribute = origin_attribute(realm)
        self.assertIsNotNone(attribute, where + ': ihp_identity_sync_origin is not declared in the user profile')
        self.assertEqual(attribute['permissions'], {'view': ['admin'], 'edit': ['admin']}, where)
        self.assertNotIn('required', attribute, where)
        names = [a['name'] for a in profile['attributes']]
        for builtin in ('username', 'email', 'firstName', 'lastName'):
            self.assertIn(builtin, names, where + ': the default attributes must stay declared')
        # district/facility (token mappers), user_type (dashboard) and the access_* attributes are
        # not declared; without this policy a fresh Keycloak 24+ import silently drops them.
        self.assertEqual(profile.get('unmanagedAttributePolicy'), 'ADMIN_EDIT', where)

    def test_exports_declare_the_origin_attribute_admin_only(self):
        root = Path(__file__).resolve().parents[2]
        for name in ('realm-export.json', 'realm-export-prod.json'):
            self.assert_admin_only(json.loads((root / 'infra/keycloak' / name).read_text()), name)

    def test_generated_environment_realms_keep_the_user_profile(self):
        for environment in bootstrap.REALMS:
            realm = bootstrap.render(environment, 'https://portal.example.org/portal', 'https://portal.example.org/api/v1')
            self.assert_admin_only(realm, environment)
            self.assertEqual(set(realm.get('components', {})), {'org.keycloak.userprofile.UserProfileProvider'})


if __name__ == '__main__':
    unittest.main()
