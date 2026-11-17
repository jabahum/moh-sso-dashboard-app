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
import os
import subprocess
import textwrap
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
    def test_distribution_metadata_binds_exact_pins_and_dependency_checksums(self):
        data = bundle.distribution_metadata()
        self.assertEqual(data['frontend']['versions']['@moh-sso/announcements'], '1.3.2')
        self.assertEqual(data['backend']['modules']['github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics'], 'v0.1.0')
        for field in ('version', 'checksum', 'module'):
            changed = json.loads(json.dumps(data))
            if field == 'version':
                changed['frontend']['versions']['@moh-sso/announcements'] = '^1.3.2'
            elif field == 'checksum':
                changed['frontend']['lockSha256'] = 'unverified'
            else:
                changed['backend']['modules'] = {'local/metrics': 'dev'}
            with self.assertRaises(ValueError):
                bundle.validate_distribution(changed)

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

    def test_distribution_rejects_runtime_frontend_versions_without_matching_pins(self):
        self.record['manifest']['distribution'] = bundle.distribution_metadata()
        with self.assertRaisesRegex(ValueError, 'runtime modules differ'):
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


class DistributionDeployment(unittest.TestCase):
    def test_candidate_records_project_and_requires_dependency_metadata(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            env_file = root / 'prod.env'
            env_file.write_text('ENVIRONMENT=production\n')
            config = root / 'config.json'
            config.write_text(json.dumps({'schema_version': 1, 'mode': 'distribution',
                'project_name': 'moh-staging', 'deploy_root': str(root),
                'env_file': str(env_file), 'verify_public_edge': False,
                'gateway_trusted_proxies_file': str(root / 'proxies'), 'secret_files': {}}))
            server = Server(config)
            release = root / 'releases' / manifest()['release_id']
            release.mkdir(parents=True)
            (release / 'deployment.json').write_text(json.dumps(manifest()))
            with self.assertRaisesRegex(ValueError, 'pinned dependency metadata'):
                server.candidate(release)
            data = manifest()
            data['distribution'] = bundle.distribution_metadata()
            (release / 'deployment.json').write_text(json.dumps(data))
            record = server.candidate(release)
            self.assertEqual(record['project_name'], 'moh-staging')
            self.assertEqual(server.environment(record)['DISTRIBUTION_TAG'], 'sha-' + COMMIT[:7])
            self.assertIn('nginx', json.loads(Path(record['override_file']).read_text())['services'])
            server.config['project_name'] = 'new-setting-after-deploy'
            calls = []
            server.runner = lambda args, **kwargs: calls.append(args) or ''
            server.compose(record, 'config', '--quiet')
            self.assertEqual(calls[0][calls[0].index('--project-name') + 1], 'moh-staging')

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
        (release / 'environment-contract.json').write_text(json.dumps({'schema_version': 1, 'settings': []}))
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
        for component, image in manifest('2')['images'].items():
            self.assertEqual(override['services'][component]['image'],
                             f"{image['repository']}:{image['version']}@{image['digest']}")
        self.assertEqual(Path(state['current']['env_file']).stat().st_mode & 0o777, 0o600)
        self.assertNotIn('private-value', (self.root / 'state/history.jsonl').read_text())

    def test_existing_installation_cannot_change_project_or_topology(self):
        self.server.deploy(self.release('1'), 'tester')
        original = self.state()
        candidate = self.release('2')
        self.commands.clear()
        for mode, project in [('distribution', 'moh-sso-distribution'), ('prod', 'another-project')]:
            self.server.config['mode'] = mode
            self.server.project = project
            with self.assertRaisesRegex(ValueError, 'topology/project'):
                self.server.deploy(candidate, 'tester')
            self.assertEqual(self.state(), original)
            self.assertEqual(self.commands, [])

    def test_duplicate_automatic_event_verifies_current_without_rollout_or_losing_rollback(self):
        self.server.deploy(self.release('1'), 'tester')
        release = self.release('2')
        self.server.deploy(release, 'tester')
        saved = self.state()
        self.commands.clear()
        with patch.object(self.server, 'verify') as verify:
            self.server.deploy(release, 'automatic', automatic=True)
            verify.assert_called_once_with(saved['current'])
        self.assertEqual(self.state(), saved)
        self.assertEqual(self.commands, [])

    def test_delayed_automatic_release_never_downgrades_current(self):
        older = self.release('9')
        self.server.deploy(older, 'tester')
        self.server.deploy(self.release('10'), 'tester')
        saved = self.state()
        self.commands.clear()
        self.server.deploy(older, 'automatic', automatic=True)
        self.assertEqual(self.state(), saved)
        self.assertEqual(self.commands, [])
        # An operator can still deliberately promote/redeploy the selected build.
        self.server.deploy(older, 'operator')
        self.assertEqual(self.state()['current']['manifest']['release_id'], manifest('9')['release_id'])

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


class EnvironmentUpgrades(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.env = self.root / 'prod.env'
        self.env.write_text('EXISTING="private existing value"\nEMPTY=\n')
        config = self.root / 'deploy.json'
        config.write_text(json.dumps({'schema_version': 1, 'deploy_root': str(self.root),
                                     'mode': 'prod', 'env_file': str(self.env), 'secret_files': {},
                                     'verify_public_edge': False}))
        self.server = Server(config)
        self.release = self.root / 'releases' / manifest()['release_id']
        self.release.mkdir(parents=True)
        (self.release / 'deployment.json').write_text(json.dumps(manifest()))

    def contract(self, settings):
        (self.release / 'environment-contract.json').write_text(json.dumps({'schema_version': 1, 'settings': settings}))

    def setting(self, name, default='', required=False, alternatives=None):
        return {'name': name, 'default': default, 'required': required, 'alternatives': alternatives or []}

    def test_upgrade_preserves_values_and_empty_settings_and_is_idempotent(self):
        original = self.env.read_bytes()
        self.contract([self.setting('EXISTING', 'ignored'), self.setting('EMPTY', 'ignored'),
                       self.setting('OBSERVATORY_DB_PORT', '5432'), self.setting('NEW_OPTION')])
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            self.server.prepare_environment(self.release)
            first = self.env.read_bytes()
            self.server.prepare_environment(self.release)
        self.assertTrue(first.startswith(original))
        self.assertIn(b'OBSERVATORY_DB_PORT=5432\nNEW_OPTION=\n', first)
        self.assertEqual(first, self.env.read_bytes())
        backups = list(self.root.glob('prod.env.before-upgrade-*'))
        self.assertEqual(len(backups), 1)
        self.assertEqual(backups[0].read_bytes(), original)
        for path in [self.env, backups[0]]:
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        self.assertNotIn('private existing value', output.getvalue())

    def test_missing_required_values_are_added_but_block_before_candidate_or_rollout(self):
        self.contract([self.setting('OBSERVATORY_DB_HOST', required=True),
                       self.setting('OBSERVATORY_DB_PASSWORD', required=True)])
        self.server.preflight = lambda: None
        with patch.object(self.server, 'candidate') as candidate, contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaisesRegex(ValueError, 'OBSERVATORY_DB_HOST, OBSERVATORY_DB_PASSWORD'):
                self.server.deploy(self.release, 'tester')
            candidate.assert_not_called()
        self.assertIn('OBSERVATORY_DB_HOST=\n', self.env.read_text())
        self.assertFalse((self.root / 'state/pending.json').exists())
        self.assertFalse((self.root / 'state/deployment.json').exists())

    def test_file_backed_secret_is_accepted_without_copying_secret_values(self):
        self.env.write_text('OBSERVATORY_DB_PASSWORD_FILE=/run/secrets/private-file\n')
        self.contract([self.setting('OBSERVATORY_DB_PASSWORD', required=True,
                                    alternatives=['OBSERVATORY_DB_PASSWORD_FILE'])])
        with contextlib.redirect_stdout(io.StringIO()):
            self.server.prepare_environment(self.release)
        self.assertIn('OBSERVATORY_DB_PASSWORD=\n', self.env.read_text())

    def test_placeholder_or_quoted_empty_required_values_fail_without_exposing_values(self):
        self.contract([self.setting('REQUIRED', required=True)])
        for value in ['"" # empty', "'   '", 'replace-with-a-private-host', ' # empty']:
            self.env.write_text('REQUIRED=' + value + '\n')
            with self.assertRaisesRegex(ValueError, 'settings need values: REQUIRED') as error:
                self.server.prepare_environment(self.release)
            self.assertNotIn(value, str(error.exception))

    def test_contract_never_uses_example_values_and_new_names_are_discovered(self):
        contract = bundle.build_environment_contract('PASSWORD=private-canary\nHOST=operator-edit\nNEW_NAME=unsafe-placeholder\nPORT=1\n',
                    {'schema_version': 1, 'defaults': {'PORT': '5432'}, 'required': ['HOST', 'PASSWORD']})
        self.assertNotIn('private-canary', json.dumps(contract))
        self.assertNotIn('operator-edit', json.dumps(contract))
        self.assertNotIn('unsafe-placeholder', json.dumps(contract))
        self.assertEqual([item['name'] for item in contract['settings']], ['PASSWORD', 'HOST', 'NEW_NAME', 'PORT'])
        for name, default in [('PASSWORD', 'do-not-leak'), ('HOST', 'bad\nINJECTED=value')]:
            with self.assertRaises(ValueError):
                bundle.build_environment_contract(name+'=\n', {'schema_version': 1, 'defaults': {name: default}})

    def test_missing_contract_or_pending_recovery_does_not_modify_private_environment(self):
        original = self.env.read_bytes()
        with self.assertRaisesRegex(ValueError, 'contract is missing'):
            self.server.prepare_environment(self.release)
        self.contract([self.setting('NEW_NAME')])
        (self.root / 'state').mkdir()
        (self.root / 'state/pending.json').write_text('{}')
        with self.assertRaisesRegex(RuntimeError, 'pending deployment'):
            self.server.prepare_environment(self.release)
        self.assertEqual(self.env.read_bytes(), original)

    def test_workflow_propagates_ssh_failure_through_tee(self):
        workflow = (bundle.ROOT / '.github/workflows/deploy-to-server.yaml').read_text()
        step = workflow.split('      - name: Pull digests, back up, roll out, verify and automatically roll back on failure\n', 1)[1]
        step = step.split('      - name:', 1)[0]
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        ssh = self.root / 'ssh'
        ssh.write_text('#!/bin/sh\necho deployment-failed\nexit 23\n')
        ssh.chmod(0o700)
        env = {**os.environ, 'PATH': str(self.root)+':'+os.environ['PATH'], 'DEPLOY_ACTOR': 'tester',
               'AUTOMATIC': 'true', 'DEPLOY_PYTHON': 'python3', 'DEPLOY_CONFIG': '/private/config.json',
               'CD_SSH_CONFIG': '/private/ssh', 'deploy_helper': '/private/helper', 'release_dir': '/private/release'}
        result = subprocess.run(['bash', '-e', '-c', script], cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 23)
        self.assertIn('deployment-failed', (self.root / 'deployment-result.log').read_text())


if __name__ == '__main__':
    unittest.main()
