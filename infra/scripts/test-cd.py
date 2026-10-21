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

sys.path.insert(0, str(Path(__file__).resolve().parent))
import cd_bundle as bundle
from cd_server import Server, atomic_json

COMMIT = 'a' * 40


def manifest(run='1'):
    return {'schema_version': 1, 'source_commit': COMMIT, 'release_id': f'{COMMIT}-{run}-1',
            'images': {component: {'repository': f'ghcr.io/example/portal-{component}',
                                  'digest': 'sha256:' + ('b' if component == 'backend' else 'c') * 64,
                                  'tag': 'sha-' + COMMIT[:7], 'version': '1.2.3', 'commit': COMMIT}
                       for component in ('backend', 'frontend')}}


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
