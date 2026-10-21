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
from cd_bundle import validate_manifest


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


class Server:
    def __init__(self, config, runner=None):
        self.config = json.loads(Path(config).read_text())
        c = self.config
        if c.get('schema_version') != 1 or c.get('mode') not in ('prod', 'gateway', 'remote'):
            raise ValueError('Unsupported server configuration schema/mode')
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
        return env

    def compose(self, record, *args):
        file = 'docker-compose-nginx.yml' if record['mode'] == 'gateway' else 'docker-compose.prod.yml'
        command = ['docker', 'compose', '--project-name', 'moh-sso-dashboard', '--env-file', record['env_file']]
        if record['mode'] == 'remote':
            command += ['--env-file', record['remote_env_file']]
        command += ['-f', str(Path(record['release_dir']) / 'infra/compose' / file),
                    '-f', record['override_file'], *args]
        return self.runner(command, env=self.environment(record))

    def candidate(self, release):
        release = Path(release).resolve()
        if not release.is_relative_to((self.root / 'releases').resolve()):
            raise ValueError('Release must be inside deploy_root/releases')
        manifest = validate_manifest(json.loads((release / 'deployment.json').read_text()))
        if release.name != manifest['release_id']:
            raise ValueError('Release directory/manifest ID mismatch')
        snapshot = self.state / 'snapshots' / (manifest['release_id'] + '-' + uuid.uuid4().hex[:12])
        snapshot.mkdir(parents=True, mode=0o700)
        env_file = snapshot / 'prod.env'
        shutil.copyfile(self.config['env_file'], env_file)
        env_file.chmod(0o600)
        record = {'release_dir': str(release), 'manifest': manifest, 'mode': self.config['mode'],
                  'env_file': str(env_file), 'override_file': str(snapshot / 'images.json')}
        if record['mode'] == 'remote':
            remote = snapshot / 'remote.env'
            shutil.copyfile(self.config['remote_env_file'], remote)
            remote.chmod(0o600)
            record['remote_env_file'] = str(remote)
        override = {'services': {key: {'image': f"{value['repository']}@{value['digest']}"}
                                 for key, value in manifest['images'].items()}}
        if record['mode'] == 'gateway':
            override['services']['nginx'] = {'volumes': [{
                'type': 'bind', 'source': self.config['gateway_trusted_proxies_file'],
                'target': '/etc/nginx/trusted-proxies.conf', 'read_only': True}]}
        atomic_json(Path(record['override_file']), override)
        return record

    def preflight(self):
        for command in ('docker',):
            if self.runner == self._run and not shutil.which(command):
                raise RuntimeError('Docker CLI is not installed')
        if set(self.config.get('secret_files', {})) != {'DB_PASSWORD_SECRET_FILE', 'KEYCLOAK_ADMIN_SECRET_FILE', 'KEYCLOAK_WEB_SECRET_FILE'}:
            raise ValueError('All three production secret paths must be configured')
        required = [self.config['env_file'], *self.config['secret_files'].values()]
        if self.config['mode'] == 'remote':
            required.append(self.config.get('remote_env_file', ''))
        if self.config['mode'] == 'gateway':
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
        actual = json.loads(self.compose(record, 'exec', '-T', 'backend', 'wget', '-T', '10', '-qO-', 'http://127.0.0.1:9000/version'))
        backend = manifest['images']['backend']
        if actual.get('version') != backend['version'] or actual.get('commit') != backend['commit']:
            raise ValueError('Backend runtime version/commit mismatch')
        frontend = manifest['images']['frontend']
        actual = json.loads(self.compose(record, 'exec', '-T', 'frontend', 'wget', '-T', '10', '-qO-', 'http://127.0.0.1:3000/portal/version-manifest.json'))['service']
        if actual.get('version') != frontend['version'] or actual.get('commit') != frontend['commit']:
            raise ValueError('Frontend runtime version/commit mismatch')
        container = self.compose(record, 'ps', '-q', 'frontend').strip()
        if not container or '\n' in container:
            raise ValueError('Expected exactly one frontend container')
        verifier = (Path(record['release_dir']) / 'infra/scripts/verify-frontend-deployment.mjs').read_text()
        self.runner(['docker', 'run', '--rm', '-i', '--network', 'container:' + container,
                     self.config.get('verifier_image', 'node:22-alpine'), 'node', '--input-type=module', '-'], input=verifier)
        if self.config.get('verify_public_edge', True):
            origin = self.config['public_base_url'].rstrip('/')
            for path in ('/portal/', '/ssobackend/health/ready', '/ssobackend/swagger/openapi.json'):
                with urllib.request.urlopen(origin + path, timeout=15) as response:
                    if response.status != 200 or response.url != origin + path:
                        raise ValueError('Public HTTPS routing/redirect check failed')
            with urllib.request.urlopen(origin + '/ssobackend/version', timeout=15) as response:
                actual = json.load(response)
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

    def deploy(self, release, actor):
        with self.lock():
            self.preflight()
            pending = self.state / 'pending.json'
            if pending.exists():
                raise RuntimeError('An interrupted deployment requires rollback/recovery first')
            saved = read_json(self.state / 'deployment.json', {})
            previous = saved.get('current')
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
    args = parser.parse_args()
    os.umask(0o077)
    server = Server(args.config)
    if args.action == 'deploy':
        if not args.release:
            parser.error('--release is required for deploy')
        server.deploy(args.release, args.actor)
    elif args.action == 'rollback':
        server.rollback(args.actor)
    elif args.action == 'preflight':
        server.preflight()
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
