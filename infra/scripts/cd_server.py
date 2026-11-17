#!/usr/bin/env python3
"""Server-side locked deployment, verification and rollback of a validated bundle."""
import argparse
import contextlib
import fcntl
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import sys
import time
import urllib.parse
import urllib.request
import uuid

sys.dont_write_bytecode = True
from cd_bundle import validate_manifest, validate_environment_contract


def atomic_json(path, data):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o750)
    temporary = path.with_suffix('.tmp')
    with temporary.open('w') as stream:
        os.chmod(temporary, 0o600)
        json.dump(data, stream, indent=2)
        stream.write('\n')
    os.replace(temporary, path)


def read_json(path, default=None):
    return json.loads(path.read_text()) if path.exists() else default


def backend_version(payload):
    """Read the API success envelope without accepting failed/malformed responses."""
    if (not isinstance(payload, dict) or payload.get('success') is not True
            or not isinstance(payload.get('data'), dict)):
        raise ValueError('Invalid backend version response')
    return payload['data']


class Server:
    def __init__(self, config, runner=None):
        self.config = json.loads(Path(config).read_text())
        c = self.config
        if c.get('schema_version') != 1 or c.get('mode') not in ('prod', 'gateway', 'remote', 'distribution'):
            raise ValueError('Unsupported server configuration schema/mode')
        default_project = 'moh-sso-distribution' if c['mode'] == 'distribution' else 'moh-sso-dashboard'
        self.project = c.get('project_name', default_project)
        if not re.fullmatch(r'[a-z0-9][a-z0-9_-]{0,62}', self.project):
            raise ValueError('Invalid Compose project name')
        for key in ('deploy_root', 'env_file'):
            if not Path(c[key]).is_absolute():
                raise ValueError(f'{key} must be absolute')
        self.root = Path(c['deploy_root'])
        self.state = self.root / 'state'
        self.runner = runner or self._run
        if not 30 <= c.get('wait_timeout', 360) <= 900:
            raise ValueError('wait_timeout must be between 30 and 900 seconds')
        public = urllib.parse.urlparse(c.get('public_base_url', ''))
        if c.get('verify_public_edge', True) and (public.scheme != 'https' or not public.netloc or public.username or public.path not in ('', '/')):
            raise ValueError('public_base_url must be an HTTPS origin')
        if c.get('verifier_image', 'node:22-alpine') != 'node:22-alpine':
            raise ValueError('Use the supported Node 22 verifier image')

    @staticmethod
    def _run(args, *, env=None, input=None):
        result = subprocess.run(args, env=env, input=input, text=True, capture_output=True)
        if result.returncode:
            # Do not publish container logs or expanded Compose configuration.
            raise RuntimeError(f'{args[0]} {args[1] if len(args) > 1 else ""} failed (exit {result.returncode})')
        return result.stdout

    @contextlib.contextmanager
    def lock(self):
        self.state.mkdir(parents=True, exist_ok=True, mode=0o750)
        with (self.state / 'deploy.lock').open('a') as stream:
            try:
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as exc:
                raise RuntimeError('Another deployment/rollback is running') from exc
            yield

    def environment(self, record):
        # Prevent login-shell variables from overriding the selected private env file.
        env = {key: os.environ[key] for key in ('PATH', 'HOME', 'DOCKER_CONFIG', 'XDG_RUNTIME_DIR') if key in os.environ}
        env.update(APP_ENV_FILE=record['env_file'], BACKEND_TAG=record['manifest']['images']['backend']['tag'],
                   FRONTEND_TAG=record['manifest']['images']['frontend']['tag'])
        env.update(self.config['secret_files'])
        if record['mode'] == 'distribution':
            env['DISTRIBUTION_TAG'] = record['manifest']['images']['backend']['tag']
        return env

    def compose(self, record, *args):
        file = 'docker-compose-nginx.yml' if record['mode'] == 'gateway' else 'docker-compose.prod.yml'
        if record['mode'] == 'distribution':
            file = 'docker-compose.distribution.release.yml'
        default_project = 'moh-sso-distribution' if record['mode'] == 'distribution' else 'moh-sso-dashboard'
        command = ['docker', 'compose', '--project-name', record.get('project_name', default_project), '--env-file', record['env_file']]
        if record['mode'] == 'remote':
            command += ['--env-file', record['remote_env_file']]
        command += ['-f', str(Path(record['release_dir']) / 'infra/compose' / file),
                    '-f', record['override_file'], *args]
        return self.runner(command, env=self.environment(record))

    def prepare_environment(self, release):
        release = Path(release).resolve()
        if not release.is_relative_to((self.root / 'releases').resolve()):
            raise ValueError('Release must be inside deploy_root/releases')
        if (self.state / 'pending.json').exists():
            raise RuntimeError('Resolve pending deployment recovery before updating environment settings')
        contract_path = release / 'environment-contract.json'
        if not contract_path.is_file():
            raise ValueError('Release environment contract is missing; build a new deployment bundle')
        contract = validate_environment_contract(json.loads(contract_path.read_text()))
        path = Path(self.config['env_file']).resolve(strict=True)
        original_bytes = path.read_bytes()
        original = original_bytes.decode()
        values = {}
        for line in original.splitlines():
            match = re.match(r"^\s*(?:export\s+)?([A-Z_][A-Z0-9_]*)\s*=(.*)$", line)
            if match:
                value = match[2].strip()
                if value.startswith(('"', "'")):
                    quote = value[0]
                    value = value[1:value.rfind(quote)] if value.rfind(quote) else value[1:]
                else:
                    value = re.split(r'\s+#', match[2], maxsplit=1)[0].strip()
                values[match[1]] = value
        missing = [setting for setting in contract['settings'] if setting['name'] not in values]
        if missing:
            # Preserve the original bytes, comments, ownership and every existing value.
            backup = path.with_name(path.name + '.before-upgrade-' + uuid.uuid4().hex)
            with os.fdopen(os.open(backup, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'wb') as stream:
                stream.write(original_bytes)
            updated = original + ('' if original.endswith('\n') else '\n') + '\n# Settings added by deployment ' + release.name + '\n'
            updated += ''.join(item['name'] + '=' + item['default'] + '\n' for item in missing)
            temporary = path.with_name(path.name + '.next-' + uuid.uuid4().hex)
            try:
                with os.fdopen(os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as stream:
                    stream.write(updated)
                stat = path.stat()
                if temporary.stat().st_uid != stat.st_uid or temporary.stat().st_gid != stat.st_gid:
                    os.chown(temporary, stat.st_uid, stat.st_gid)
                if path.read_bytes() != original_bytes:
                    raise RuntimeError('Private environment changed during upgrade; retry preflight')
                os.replace(temporary, path)
            finally:
                temporary.unlink(missing_ok=True)
            print('Added environment setting names: ' + ', '.join(item['name'] for item in missing))
            values.update({item['name']: item['default'] for item in missing})
        if self.config['mode'] == 'distribution':
            # This topology supplies the managed Observatory connection and
            # mounted password. Validate operator settings for external services.
            values.update(OBSERVATORY_DB_HOST='observatory-db', OBSERVATORY_DB_PORT='5432',
                          OBSERVATORY_DB_USER=values.get('OBSERVATORY_DB_USER') or 'surveillance',
                          OBSERVATORY_DB_NAME=values.get('OBSERVATORY_DB_NAME') or 'observatory',
                          OBSERVATORY_DB_PASSWORD_FILE='/run/secrets/observatory_db_password')
        def supplied(key):
            value = values.get(key, '').strip()
            return bool(value) and not value.lower().startswith(('replace-', 'change-me', 'changeme', '<'))
        required = [item['name'] for item in contract['settings'] if item['required'] and
                    not any(supplied(key) for key in [item['name'], *item.get('alternatives', [])])]
        if required:
            raise ValueError('Required production environment settings need values: ' + ', '.join(required))

    def candidate(self, release):
        release = Path(release).resolve()
        if not release.is_relative_to((self.root / 'releases').resolve()):
            raise ValueError('Release must be inside deploy_root/releases')
        manifest = validate_manifest(json.loads((release / 'deployment.json').read_text()))
        if self.config['mode'] == 'distribution' and 'distribution' not in manifest:
            raise ValueError('Distribution deployment requires pinned dependency metadata')
        if release.name != manifest['release_id']:
            raise ValueError('Release directory/manifest ID mismatch')
        snapshot = self.state / 'snapshots' / (manifest['release_id'] + '-' + uuid.uuid4().hex[:12])
        snapshot.mkdir(parents=True, mode=0o700)
        env_file = snapshot / 'prod.env'
        shutil.copyfile(self.config['env_file'], env_file)
        env_file.chmod(0o600)
        record = {'release_dir': str(release), 'manifest': manifest, 'mode': self.config['mode'],
                  'project_name': self.project,
                  'env_file': str(env_file), 'override_file': str(snapshot / 'images.json')}
        if record['mode'] == 'remote':
            remote = snapshot / 'remote.env'
            shutil.copyfile(self.config['remote_env_file'], remote)
            remote.chmod(0o600)
            record['remote_env_file'] = str(remote)
        override = {'services': {key: {'image': f"{value['repository']}:{value['version']}@{value['digest']}"}
                                 for key, value in manifest['images'].items()}}
        if record['mode'] in ('gateway', 'distribution'):
            override['services']['nginx'] = {'volumes': [{
                'type': 'bind', 'source': self.config['gateway_trusted_proxies_file'],
                'target': '/etc/nginx/trusted-proxies.conf', 'read_only': True}]}
        atomic_json(Path(record['override_file']), override)
        return record

    def preflight(self):
        for command in ('docker',):
            if self.runner == self._run and not shutil.which(command):
                raise RuntimeError('Docker CLI is not installed')
        secrets = {'DB_PASSWORD_SECRET_FILE', 'KEYCLOAK_ADMIN_SECRET_FILE', 'KEYCLOAK_WEB_SECRET_FILE'}
        if self.config['mode'] == 'distribution':
            secrets.add('OBSERVATORY_DB_PASSWORD_SECRET_FILE')
        if set(self.config.get('secret_files', {})) != secrets:
            raise ValueError('All production secret paths for the selected topology must be configured')
        required = [self.config['env_file'], *self.config['secret_files'].values()]
        if self.config['mode'] == 'remote':
            required.append(self.config.get('remote_env_file', ''))
        if self.config['mode'] in ('gateway', 'distribution'):
            required.append(self.config.get('gateway_trusted_proxies_file', ''))
        for value in required:
            path = Path(value)
            if not path.is_absolute() or not path.is_file() or not os.access(path, os.R_OK) or path.stat().st_size == 0:
                raise ValueError('A required private configuration/secret file is missing, empty or unreadable')
        hook = self.config.get('pre_deploy_hook')
        if hook and (not Path(hook).is_absolute() or not os.access(hook, os.X_OK)):
            raise ValueError('pre_deploy_hook must be an absolute executable file')
        self.runner(['docker', 'info', '--format', '{{.ServerVersion}}'])
        version = self.runner(['docker', 'compose', 'version', '--short']).strip().lstrip('v')
        match = re.match(r'(\d+)\.(\d+)\.(\d+)', version)
        if not match or tuple(int(x) for x in match.groups()) < (2, 24, 4):
            raise ValueError('Docker Compose 2.24.4+ is required')
        self.root.joinpath('releases').mkdir(parents=True, exist_ok=True, mode=0o750)

    def pull(self, record):
        for image in record['manifest']['images'].values():
            reference = f"{image['repository']}@{image['digest']}"
            self.runner(['docker', 'pull', reference])
            inspection = json.loads(self.runner(['docker', 'image', 'inspect', reference]))[0]
            labels = inspection['Config'].get('Labels') or {}
            if labels.get('org.opencontainers.image.version') != image['version'] or labels.get('org.opencontainers.image.revision') != image['commit']:
                raise ValueError('Pulled image metadata does not match the approved deployment manifest')
        self.runner(['docker', 'pull', self.config.get('verifier_image', 'node:22-alpine')])

    def up(self, record):
        self.compose(record, 'up', '-d', '--no-build', '--wait', '--wait-timeout', str(self.config.get('wait_timeout', 360)))

    def verify(self, record):
        manifest = record['manifest']
        self.compose(record, 'exec', '-T', 'backend', 'wget', '-T', '10', '-qO-', 'http://127.0.0.1:9000/health/ready')
        actual = backend_version(json.loads(self.compose(record, 'exec', '-T', 'backend', 'wget', '-T', '10', '-qO-', 'http://127.0.0.1:9000/version')))
        backend = manifest['images']['backend']
        if actual.get('version') != backend['version'] or actual.get('commit') != backend['commit']:
            raise ValueError('Backend runtime version/commit mismatch')
        frontend = manifest['images']['frontend']
        frontend_manifest = json.loads(self.compose(record, 'exec', '-T', 'frontend', 'wget', '-T', '10', '-qO-', 'http://127.0.0.1:3000/portal/version-manifest.json'))
        actual = frontend_manifest['service']
        if actual.get('version') != frontend['version'] or actual.get('commit') != frontend['commit']:
            raise ValueError('Frontend runtime version/commit mismatch')
        distribution = manifest.get('distribution')
        if distribution:
            actual_modules = {item['name']: item['version'] for group in ('apps', 'packages')
                              for item in frontend_manifest.get(group, {}).values()
                              if item['name'] != '@moh-sso/shell'}
            if actual_modules != distribution['frontend']['versions']:
                raise ValueError('Frontend runtime modules differ from the approved distribution pins')
        container = self.compose(record, 'ps', '-q', 'frontend').strip()
        if not container or '\n' in container:
            raise ValueError('Expected exactly one frontend container')
        # Operator/recovery helpers carry their matching verifier without modifying
        # an immutable release. Older helper uploads can use the release copy.
        verifier_path = Path(__file__).with_name('verify-frontend-deployment.mjs')
        if not verifier_path.is_file():
            verifier_path = Path(record['release_dir']) / 'infra/scripts/verify-frontend-deployment.mjs'
        verifier = verifier_path.read_text()
        self.runner(['docker', 'run', '--rm', '-i', '--network', 'container:' + container,
                     self.config.get('verifier_image', 'node:22-alpine'), 'node', '--input-type=module', '-'], input=verifier)
        if self.config.get('verify_public_edge', True):
            origin = self.config['public_base_url'].rstrip('/')
            for path in ('/portal/', '/ssobackend/health/ready', '/ssobackend/swagger/openapi.json'):
                with urllib.request.urlopen(origin + path, timeout=15) as response:
                    if response.status != 200 or response.url != origin + path:
                        raise ValueError('Public HTTPS routing/redirect check failed')
            with urllib.request.urlopen(origin + '/ssobackend/version', timeout=15) as response:
                actual = backend_version(json.load(response))
                if actual.get('version') != backend['version'] or actual.get('commit') != backend['commit']:
                    raise ValueError('Public API runtime metadata mismatch')

    def audit(self, action, record, actor):
        event = {'time': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()), 'action': action,
                 'release_id': record['manifest']['release_id'], 'source_commit': record['manifest']['source_commit'],
                 'actor': actor, 'images': record['manifest']['images']}
        with (self.state / 'history.jsonl').open('a') as stream:
            os.chmod(stream.name, 0o600)
            stream.write(json.dumps(event) + '\n')
        print(json.dumps(event))

    def activate(self, record):
        link = self.root / '.current-next'
        link.unlink(missing_ok=True)
        link.symlink_to(record['release_dir'], target_is_directory=True)
        os.replace(link, self.root / 'current')

    def deploy(self, release, actor, automatic=False):
        with self.lock():
            self.preflight()
            pending = self.state / 'pending.json'
            if pending.exists():
                raise RuntimeError('An interrupted deployment requires rollback/recovery first')
            saved = read_json(self.state / 'deployment.json', {})
            previous = saved.get('current')
            if previous and (previous['mode'] != self.config['mode'] or
                             previous.get('project_name', 'moh-sso-distribution' if previous['mode'] == 'distribution' else 'moh-sso-dashboard') != self.project):
                raise ValueError('Changing deployment topology/project requires a separate migration plan')
            if automatic and previous:
                incoming = validate_manifest(json.loads((Path(release) / 'deployment.json').read_text()))
                current = previous['manifest']
                if incoming['release_id'] == current['release_id']:
                    if incoming != current:
                        raise ValueError('Current release identity has different image metadata')
                    self.verify(previous)
                    print('Release already current and verified; no rollout needed.')
                    return
                def build_order(item):
                    return tuple(map(int, item['release_id'].rsplit('-', 2)[1:]))
                if build_order(incoming) <= build_order(current):
                    print('Skipping older automatic release; current deployment retained.')
                    return
            self.prepare_environment(release)
            record = self.candidate(release)
            self.compose(record, 'config', '--quiet')
            self.pull(record)
            hook = self.config.get('pre_deploy_hook')
            if hook:
                self.runner([hook]) # failure blocks rollout, before migrations can start
            atomic_json(pending, {'previous': previous, 'candidate': record, 'restore_state': saved})
            try:
                self.up(record)
                self.verify(record)
                self.activate(record)
                atomic_json(self.state / 'deployment.json', {'current': record, 'previous': previous})
                self.audit('deployed', record, actor)
                pending.unlink()
            except Exception:
                self.audit('deployment-failed', record, actor)
                if previous:
                    try:
                        self.up(previous)
                        self.verify(previous)
                        self.activate(previous)
                        atomic_json(self.state / 'deployment.json', saved)
                        self.audit('automatic-rollback', previous, actor)
                        pending.unlink()
                    except Exception as error:
                        raise RuntimeError('Deployment and rollback failed; pending state retained for operator recovery') from error
                else:
                    pending.unlink()
                    print('First deployment failed: no verified previous release is available.', file=sys.stderr)
                raise

    def rollback(self, actor):
        with self.lock():
            saved = read_json(self.state / 'deployment.json', {})
            pending = read_json(self.state / 'pending.json')
            target = pending.get('previous') if pending else saved.get('previous')
            if not target:
                raise ValueError('No verified previous release is available')
            restored = pending['restore_state'] if pending else {'current': target, 'previous': saved.get('current')}
            if not pending:
                atomic_json(self.state / 'pending.json', {'previous': target, 'restore_state': restored})
            self.up(target)
            self.verify(target)
            self.activate(target)
            atomic_json(self.state / 'deployment.json', restored)
            self.audit('operator-rollback', target, actor)
            self.state.joinpath('pending.json').unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['preflight', 'deploy', 'verify', 'rollback', 'status'])
    parser.add_argument('--config', type=Path, default=Path('/etc/moh-sso/deploy.json'))
    parser.add_argument('--release', type=Path)
    parser.add_argument('--actor', default='operator')
    parser.add_argument('--automatic', action='store_true', help='Skip current or older builds under the deployment lock')
    args = parser.parse_args()
    os.umask(0o077)
    server = Server(args.config)
    if args.action == 'deploy':
        if not args.release:
            parser.error('--release is required for deploy')
        server.deploy(args.release, args.actor, automatic=args.automatic)
    elif args.action == 'rollback':
        server.rollback(args.actor)
    elif args.action == 'preflight':
        with server.lock():
            server.preflight()
            if args.release:
                server.prepare_environment(args.release)
        print('Server prerequisites and private configuration are ready.')
    else:
        saved = read_json(server.state / 'deployment.json', {})
        if args.action == 'status':
            print(json.dumps({key: value['manifest'] if value else None for key, value in saved.items()}, indent=2))
        elif not saved.get('current'):
            raise ValueError('No verified current release')
        else:
            server.verify(saved['current'])
            print('Current release verified.')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(f'Deployment error: {error}', file=sys.stderr)
        sys.exit(1)
