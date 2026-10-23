#!/usr/bin/env python3
"""Private scheduler configuration generation and credential lifecycle contracts."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('scheduler_env', Path(__file__).with_name('scheduler-env.py'))
env = importlib.util.module_from_spec(spec)
spec.loader.exec_module(env)


class SchedulerEnvTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.portal = Path(self.tmp.name) / 'portal.env'
        self.healthbi = Path(self.tmp.name) / 'healthbi.env'
        self.portal.write_text('KEYCLOAK_EXTERNAL_URL=https://identity.example.org\nKEYCLOAK_REALM=shared\nSMTP_PASSWORD=keep-this-private\nHEALTH_BI_BASE_URL=\n')

    def test_generates_separate_private_settings_without_fake_oauth_credentials(self):
        self.assertFalse(env.generate(self.portal, self.healthbi, 'external'))
        portal, peer = env.values(self.portal), env.values(self.healthbi)
        self.assertEqual(portal['HEALTH_BI_KEYCLOAK_CLIENT_SECRET'], '')
        self.assertEqual(portal['REPORT_SCHEDULER_WORKER_INTERVAL'], '30s')
        self.assertEqual(peer['KEYCLOAK_REALM'], 'shared')
        self.assertEqual(peer['BASE_PATH'], '/report-browser')
        self.assertGreaterEqual(len(peer['SCHEDULER_ARTIFACT_SIGNING_KEY']), 32)
        self.assertNotIn('SCHEDULER_ARTIFACT_SIGNING_KEY', portal)
        self.assertNotIn('SMTP_PASSWORD', peer)
        for path in (self.portal, self.healthbi):
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_rerun_preserves_keys_credentials_and_unrelated_application_settings(self):
        self.portal.write_text(self.portal.read_text() + "HEALTH_BI_KEYCLOAK_CLIENT_SECRET='existing-$-credential'\nHEALTH_BI_TIMEOUT=45s\n")
        self.assertTrue(env.generate(self.portal, self.healthbi, 'external'))
        before = self.healthbi.read_bytes()
        env.generate(self.portal, self.healthbi, 'external')
        self.assertEqual(before, self.healthbi.read_bytes())
        self.assertIn("HEALTH_BI_KEYCLOAK_CLIENT_SECRET='existing-$-credential'", self.portal.read_text())
        self.assertEqual(env.values(self.portal)['SMTP_PASSWORD'], 'keep-this-private')
        self.assertEqual(env.values(self.portal)['HEALTH_BI_TIMEOUT'], '45s')

    def test_reads_issued_secret_from_file_without_sharing_it_with_healthbi(self):
        self.portal.write_text(self.portal.read_text() + 'HEALTH_BI_KEYCLOAK_CLIENT_SECRET_FILE=/run/secrets/previous-worker\n')
        secret = self.portal.parent / 'credential'
        secret.write_text('issued-$-credential\n')
        self.assertTrue(env.generate(self.portal, self.healthbi, 'dev', 'http://host.docker.internal:8080', client_secret_file=secret))
        self.assertEqual(env.values(self.portal)['HEALTH_BI_KEYCLOAK_CLIENT_SECRET'], 'issued-$-credential')
        self.assertEqual(env.values(self.portal)['HEALTH_BI_KEYCLOAK_CLIENT_SECRET_FILE'], '')
        self.assertEqual(env.values(self.healthbi)['SCHEDULER_PUBLIC_BASE_URL'], 'http://localhost:8080')
        self.assertNotIn('issued-$-credential', self.healthbi.read_text())

    def test_reuses_legacy_signing_key_instead_of_invalidating_download_links(self):
        key = 'legacy-' + 'a' * 40
        self.healthbi.write_text('SCHEDULER_SERVICE_API_KEY=' + key + '\n')
        env.generate(self.portal, self.healthbi, 'external')
        self.assertEqual(env.values(self.healthbi)['SCHEDULER_ARTIFACT_SIGNING_KEY'], key)

    def test_bootstraps_complete_private_env_from_each_mode_template(self):
        for mode, name in [('prod', 'prod'), ('dev', 'dev'), ('external', 'external-dev')]:
            with self.subTest(mode=mode):
                portal = self.portal.parent / f'{mode}.env'
                peer = self.portal.parent / f'healthbi-{mode}.env'
                template = env.ROOT / f'infra/env/{name}.env.example'
                env.generate(portal, peer, mode, template=template)
                original, generated = env.values(template), env.values(portal)
                for key, value in original.items():
                    if not key.startswith(('HEALTH_BI_', 'REPORT_SCHEDULER_')):
                        self.assertEqual(generated[key], value)
                self.assertIn(template.read_text().splitlines()[0], portal.read_text())
                self.assertEqual(portal.stat().st_mode & 0o777, 0o600)
                portal.write_text(portal.read_text() + 'CUSTOM_SETTING=preserve\n')
                before = peer.read_bytes()
                env.generate(portal, peer, mode, template=template)
                self.assertEqual(env.values(portal)['CUSTOM_SETTING'], 'preserve')
                self.assertEqual(peer.read_bytes(), before)

    def test_missing_custom_env_and_invalid_bootstrap_leave_no_files(self):
        portal = self.portal.parent / 'missing.env'
        with self.assertRaisesRegex(ValueError, 'Missing application env file'):
            env.generate(portal, self.healthbi, 'prod')
        with self.assertRaises(ValueError):
            env.generate(portal, self.healthbi, 'prod', 'https://bi.example.org/api/v1',
                         template=env.ROOT / 'infra/env/prod.env.example')
        self.assertFalse(portal.exists())
        self.assertFalse(self.healthbi.exists())

    def test_rejects_invalid_destinations_and_urls_without_modifying_files(self):
        before = self.portal.read_bytes()
        for url in ('https://bi.example.org/?report=1', 'https://bi.example.org/api/v1', 'https://user:secret@bi.example.org'):
            with self.assertRaises(ValueError):
                env.generate(self.portal, self.healthbi, 'external', url)
        with self.assertRaises(ValueError):
            env.generate(self.portal, self.portal, 'external')
        with self.assertRaises(ValueError):
            env.generate(self.portal, self.healthbi.with_suffix('.example'), 'external')
        self.assertEqual(before, self.portal.read_bytes())
        self.assertFalse(self.healthbi.exists())


if __name__ == '__main__':
    unittest.main()
