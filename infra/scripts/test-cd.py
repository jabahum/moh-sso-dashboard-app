#!/usr/bin/env python3
"""CD contracts and transaction failure tests; no server, Docker or credentials required."""
import hashlib
import contextlib
import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import cd_bundle as bundle
from cd_server import Server, atomic_json
from image_version import image_version

COMMIT = 'a' * 40


def manifest(run='1'):
    return {'schema_version': 1, 'source_commit': COMMIT, 'release_id': f'{COMMIT}-{run}-1',
            'images': {component: {'repository': f'ghcr.io/example/portal-{component}',
                                  'digest': 'sha256:' + ('b' if component == 'backend' else 'c') * 64,
                                  'tag': 'sha-' + COMMIT[:7], 'version': '1.2.3', 'commit': COMMIT}
                       for component in ('backend', 'frontend')}}


class ImageVersions(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / 'backend').mkdir()
        (self.root / 'frontend/apps/shell').mkdir(parents=True)
        self.write_versions('1.2.3', '4.5.6')

    def write_versions(self, backend, frontend):
        (self.root / 'backend/VERSION').write_text(backend + '\n')
        (self.root / 'frontend/apps/shell/package.json').write_text(json.dumps({'version': frontend}))

    def test_main_images_use_each_components_prepared_numeric_version(self):
        self.assertEqual(image_version('backend', 'refs/heads/main', self.root), '1.2.3')
        self.assertEqual(image_version('frontend', 'refs/heads/main', self.root), '4.5.6')

    def test_component_tags_must_match_the_prepared_versions(self):
        for component, version in [('backend', '1.2.3'), ('frontend', '4.5.6')]:
            self.assertEqual(image_version(component, f'refs/tags/{component}/v{version}', self.root), version)
            with self.assertRaisesRegex(ValueError, 'does not match'):
                image_version(component, f'refs/tags/{component}/v9.9.9', self.root)

    def test_production_rejects_development_prerelease_and_build_suffixes(self):
        for version in ('dev', '1.2.3-dev.abc1234', '1.2.3-rc.1', '1.2.3+abc1234', 'v1.2.3', '01.2.3'):
            with self.subTest(version=version):
                self.write_versions(version, version)
                for component in ('backend', 'frontend'):
                    with self.assertRaisesRegex(ValueError, 'numeric'):
                        image_version(component, 'refs/heads/main', self.root)


class BundleTests(unittest.TestCase):
    def test_manifest_rejects_incomplete_mutable_or_mixed_commit_images(self):
        bundle.validate_manifest(manifest())
        for mutation in ('digest', 'tag', 'commit', 'pair'):
            data = manifest()
            if mutation == 'pair':
                del data['images']['frontend']
            else:
                data['images']['backend'][mutation] = {'digest': 'latest', 'tag': 'latest', 'commit': 'd' * 40}[mutation]
            with self.assertRaises(ValueError):
                bundle.validate_manifest(data)

    def test_package_contains_only_public_runtime_files_and_can_be_reused(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            metadata = tmp / 'metadata'
            metadata.mkdir()
            for component, image in manifest()['images'].items():
                (metadata / f'{component}.json').write_text(json.dumps(image))
            bundle.package(metadata, tmp / 'out', COMMIT, '1', '1')
            archive, checksum = tmp / 'out/deployment.tar.gz', tmp / 'out/deployment.tar.gz.sha256'
            with tarfile.open(archive) as tar:
                self.assertFalse(any('/env/' in n or '/secrets/' in n or '/certs/' in n for n in tar.getnames()))
            first = bundle.extract(archive, checksum, tmp / 'release')
            self.assertEqual(first, bundle.extract(archive, checksum, tmp / 'release'))
            (tmp / 'release/deployment.json').write_text('{}')
            with self.assertRaises(ValueError):
                bundle.extract(archive, checksum, tmp / 'release')

    def test_rejects_checksum_failure_and_archive_traversal(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            archive, checksum = tmp / 'bad.tar.gz', tmp / 'checksum'
            with tarfile.open(archive, 'w:gz') as tar:
                entry = tarfile.TarInfo('../escape')
                entry.size = 3
                tar.addfile(entry, io.BytesIO(b'bad'))
            checksum.write_text('0' * 64)
            with self.assertRaises(ValueError):
                bundle.extract(archive, checksum, tmp / 'dest')
            checksum.write_text(hashlib.sha256(archive.read_bytes()).hexdigest())
            with self.assertRaises(ValueError):
                bundle.extract(archive, checksum, tmp / 'dest')
            self.assertFalse((tmp / 'escape').exists())


class RuntimeVerification(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        config = Path(self.tmp.name) / 'config.json'
        config.write_text(json.dumps({'schema_version': 1, 'mode': 'prod',
                                     'deploy_root': self.tmp.name,
                                     'env_file': str(Path(self.tmp.name) / 'prod.env'),
                                     'verify_public_edge': False, 'secret_files': {}}))
        self.expected = manifest()['images']['backend']
        self.payload = {'success': True, 'data': {'version': self.expected['version'],
                                                'commit': self.expected['commit']},
                        'meta': {'requestId': 'fixture'}}
        self.server = Server(config, runner=self.run_command)
        self.record = {'manifest': manifest(), 'mode': 'prod',
                       'release_dir': str(Path(__file__).resolve().parents[2]),
                       'env_file': str(Path(self.tmp.name) / 'prod.env'),
                       'override_file': str(Path(self.tmp.name) / 'override.json')}

    def run_command(self, args, **kwargs):
        if args[-1] == 'http://127.0.0.1:9000/version':
            return json.dumps(self.payload)
        if args[-1] == 'http://127.0.0.1:3000/portal/version-manifest.json':
            return json.dumps({'service': self.expected})
        if args[-3:] == ['ps', '-q', 'frontend']:
            return 'fixture-frontend'
        return ''

    def test_actual_backend_envelope_passes_runtime_verification(self):
        self.server.verify(self.record)

    def test_failed_or_malformed_version_responses_are_rejected(self):
        for payload in ({'success': False, 'data': self.payload['data']},
                        {'success': True, 'data': []}, self.payload['data'], []):
            with self.subTest(payload=payload):
                self.payload = payload
                with self.assertRaisesRegex(ValueError, 'Invalid backend version response'):
                    self.server.verify(self.record)

    def test_enveloped_identity_must_match_both_version_and_commit(self):
        for key in ('version', 'commit'):
            with self.subTest(key=key):
                self.payload['data'][key] = 'wrong'
                with self.assertRaisesRegex(ValueError, 'Backend runtime version/commit mismatch'):
                    self.server.verify(self.record)
                self.payload['data'][key] = self.expected[key]

    def test_public_edge_checks_the_enveloped_identity(self):
        origin = 'https://dashboard.example.org'
        self.server.config.update(verify_public_edge=True, public_base_url=origin)
        edge_payload = dict(self.payload, data=dict(self.payload['data']))

        def response(url, **kwargs):
            stream = io.BytesIO(json.dumps(edge_payload).encode())
            stream.status, stream.url = 200, url
            return stream

        with patch('cd_server.urllib.request.urlopen', side_effect=response):
            self.server.verify(self.record)
            edge_payload['data']['commit'] = 'wrong'
            with self.assertRaisesRegex(ValueError, 'Public API runtime metadata mismatch'):
                self.server.verify(self.record)


class Transactions(unittest.TestCase):
    def setUp(self):
        self.output = contextlib.redirect_stdout(io.StringIO())
        self.output.__enter__()
        self.addCleanup(self.output.__exit__, None, None, None)
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.addCleanup(self.tmp.cleanup)
        env = self.root / 'prod.env'
        env.write_text('DB_PASSWORD=private-value\n')
        self.config = self.root / 'config.json'
        self.config.write_text(json.dumps({'schema_version': 1, 'deploy_root': str(self.root),
                                          'mode': 'prod', 'env_file': str(env), 'secret_files': {},
                                          'verify_public_edge': False, 'wait_timeout': 60}))
        self.commands = []
        self.server = Server(self.config, runner=self.run_command)
        self.server.preflight = lambda: None
        self.server.pull = lambda record: None
        self.server.verify = lambda record: None

    def run_command(self, args, **kwargs):
        self.commands.append((args, kwargs))
        return ''

    def release(self, run):
        release = self.root / 'releases' / manifest(run)['release_id']
        release.mkdir(parents=True)
        (release / 'deployment.json').write_text(json.dumps(manifest(run)))
        return release

    def state(self):
        return json.loads((self.root / 'state/deployment.json').read_text())

    def test_success_pins_digests_and_records_both_releases(self):
        self.server.deploy(self.release('1'), 'tester')
        self.server.deploy(self.release('2'), 'tester')
        state = self.state()
        self.assertEqual(state['current']['manifest']['release_id'], manifest('2')['release_id'])
        self.assertEqual(state['previous']['manifest']['release_id'], manifest('1')['release_id'])
        self.assertEqual((self.root / 'current').resolve(), Path(state['current']['release_dir']))
        override = json.loads(Path(state['current']['override_file']).read_text())
        self.assertIn('@sha256:', override['services']['backend']['image'])
        self.assertEqual(Path(state['current']['env_file']).stat().st_mode & 0o777, 0o600)
        self.assertNotIn('private-value', (self.root / 'state/history.jsonl').read_text())

    def test_failed_verification_restores_previous_image_and_configuration_state(self):
        self.server.deploy(self.release('1'), 'tester')
        old = self.state()
        def verify(record):
            if record['manifest']['release_id'] == manifest('2')['release_id']:
                raise ValueError('candidate failed')
        self.server.verify = verify
        with self.assertRaisesRegex(ValueError, 'candidate failed'):
            self.server.deploy(self.release('2'), 'tester')
        self.assertEqual(self.state(), old)
        self.assertFalse((self.root / 'state/pending.json').exists())
        self.assertIn('automatic-rollback', (self.root / 'state/history.jsonl').read_text())

    def test_failed_rollback_retains_pending_state_and_blocks_new_deployment(self):
        self.server.deploy(self.release('1'), 'tester')
        self.server.verify = lambda record: (_ for _ in ()).throw(ValueError('failure'))
        with self.assertRaisesRegex(RuntimeError, 'rollback failed'):
            self.server.deploy(self.release('2'), 'tester')
        self.assertTrue((self.root / 'state/pending.json').exists())
        with self.assertRaisesRegex(RuntimeError, 'requires rollback'):
            self.server.deploy(self.release('3'), 'tester')
        self.server.verify = lambda record: None
        self.server.rollback('operator')
        self.assertFalse((self.root / 'state/pending.json').exists())
        self.assertEqual(self.state()['current']['manifest']['release_id'], manifest('1')['release_id'])

    def test_operator_rollback_can_toggle_verified_releases(self):
        self.server.deploy(self.release('1'), 'tester')
        self.server.deploy(self.release('2'), 'tester')
        self.server.rollback('operator')
        self.assertEqual(self.state()['current']['manifest']['release_id'], manifest('1')['release_id'])
        self.server.rollback('operator')
        self.assertEqual(self.state()['current']['manifest']['release_id'], manifest('2')['release_id'])

    def test_interruption_after_state_commit_restores_complete_previous_state(self):
        self.server.deploy(self.release('1'), 'tester')
        original = self.state()
        audit = self.server.audit
        self.server.audit = lambda *args: (_ for _ in ()).throw(KeyboardInterrupt())
        with self.assertRaises(KeyboardInterrupt):
            self.server.deploy(self.release('2'), 'tester')
        self.assertEqual(self.state()['current']['manifest']['release_id'], manifest('2')['release_id'])
        self.server.audit = audit
        self.server.rollback('operator')
        self.assertEqual(self.state(), original)
        self.assertEqual((self.root / 'current').resolve(), Path(original['current']['release_dir']))
        self.assertFalse((self.root / 'state/pending.json').exists())

    def test_operator_rollback_failure_can_retry_same_target(self):
        self.server.deploy(self.release('1'), 'tester')
        self.server.deploy(self.release('2'), 'tester')
        self.server.verify = lambda record: (_ for _ in ()).throw(ValueError('unready'))
        with self.assertRaises(ValueError):
            self.server.rollback('operator')
        self.assertTrue((self.root / 'state/pending.json').exists())
        self.server.verify = lambda record: None
        self.server.rollback('operator')
        self.assertEqual(self.state()['current']['manifest']['release_id'], manifest('1')['release_id'])
        self.assertFalse((self.root / 'state/pending.json').exists())

    def test_first_failure_reports_no_previous_release_and_same_artifact_can_retry(self):
        release = self.release('1')
        self.server.verify = lambda record: (_ for _ in ()).throw(ValueError('unready'))
        with self.assertRaises(ValueError):
            self.server.deploy(release, 'tester')
        self.server.verify = lambda record: None
        self.server.deploy(release, 'tester')
        self.assertEqual(self.state()['current']['manifest']['release_id'], manifest('1')['release_id'])

    def test_lock_and_environment_isolation(self):
        with self.server.lock():
            with self.assertRaisesRegex(RuntimeError, 'Another deployment'):
                with self.server.lock():
                    pass
        self.server.deploy(self.release('1'), 'tester')
        command, kwargs = next((a, k) for a, k in self.commands if 'compose' in a)
        self.assertEqual(kwargs['env']['APP_ENV_FILE'], self.state()['current']['env_file'])
        self.assertNotIn('DB_PASSWORD', kwargs['env'])
        self.assertIn('moh-sso-dashboard', command)

    def test_pre_deploy_hook_failure_never_starts_candidate(self):
        self.server.config['pre_deploy_hook'] = '/fixture/backup'
        def fail_hook(args, **kwargs):
            if args == ['/fixture/backup']:
                raise RuntimeError('backup failed')
            return self.run_command(args, **kwargs)
        self.server.runner = fail_hook
        with self.assertRaisesRegex(RuntimeError, 'backup failed'):
            self.server.deploy(self.release('1'), 'tester')
        self.assertFalse(any('up' in args for args, kwargs in self.commands))


if __name__ == '__main__':
    unittest.main()
