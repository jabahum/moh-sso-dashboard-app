#!/usr/bin/env python3
"""Exercise the real deployment transaction with isolated local image/container fixtures.

Only registry transport and the Compose project name are adapted for isolation.
The server's preflight, snapshots, up, verification, state and rollback run intact.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import uuid

from cd_server import Server

ROOT = Path(__file__).resolve().parents[2]
COMMIT = 'a' * 40


def docker(*args, **kwargs):
    result = subprocess.run(['docker', *args], text=True, capture_output=True, **kwargs)
    if result.returncode:
        raise RuntimeError(result.stderr[:5000])
    return result.stdout.strip()


class LocalTransport:
    def __init__(self, project, references):
        self.project, self.references = project, references
        self.last_compose = None

    def __call__(self, args, **kwargs):
        args = list(args)
        if args[:2] == ['docker', 'pull'] and args[2] in self.references:
            return docker('image', 'inspect', self.references[args[2]])
        if args[:3] == ['docker', 'image', 'inspect']:
            args[3] = self.references.get(args[3], args[3])
        if args[:2] == ['docker', 'compose'] and '--project-name' in args:
            args[args.index('--project-name') + 1] = self.project
            override_index = max(i for i, arg in enumerate(args) if arg == '-f') + 1
            original = Path(args[override_index])
            model = json.loads(original.read_text())
            for service in model['services'].values():
                service['image'] = self.references[service['image']]
                service['pull_policy'] = 'never'
            translated = original.with_name('images.fixture.json')
            translated.write_text(json.dumps(model))
            args[override_index] = str(translated)
            self.last_compose = args[:override_index + 1]
        return Server._run(args, **kwargs)


def main():
    project = 'moh-cd-fixture-' + uuid.uuid4().hex[:12]
    tags, transport = [], None
    with tempfile.TemporaryDirectory(prefix=project) as tmp:
        root = Path(tmp)
        try:
            imports = json.loads((ROOT / 'frontend/public/import-map.json').read_text())['imports']
            imports = {name: '/portal/mf/' + name.split('/')[-1] + '.js' for name in imports if name.startswith('@moh-sso/')}
            (root / 'imports.json').write_text(json.dumps({'imports': imports}))
            (root / 'server.cjs').write_text('''
const http = require('node:http');
const fs = require('node:fs');
const version = process.env.FIXTURE_VERSION;
const commit = process.env.FIXTURE_COMMIT;
if (process.env.ROLE === 'backend' && !fs.existsSync('/data/marker')) fs.writeFileSync('/data/marker', 'preserved');
http.createServer((req,res) => {
  if (req.url === '/version') { res.setHeader('Content-Type','application/json'); return res.end(JSON.stringify({version:process.env.BROKEN_VERSION==='1'?'0.0.0':version,commit})); }
  if (req.url === '/portal/version-manifest.json') { res.setHeader('Content-Type','application/json'); return res.end(JSON.stringify({service:{version,commit}})); }
  if (req.url === '/setting') return res.end(process.env.FIXTURE_SETTING);
  if (req.url === '/portal/import-map.json') {res.setHeader('Content-Type','application/json'); return res.end(fs.readFileSync('/fixture/imports.json'));}
  if (req.url === '/portal/') {res.setHeader('Content-Type','text/html'); return res.end('<!doctype html><div id="root"></div><script src="/portal/assets/index-fixture.js"></script>');}
  if (req.url.endsWith('.js')) {res.setHeader('Content-Type','application/javascript');return res.end('window.__APP_CONFIG__ = {};');}
  res.end('ready');
}).listen(process.env.ROLE === 'backend' ? 9000 : 3000, '0.0.0.0');
''')
            (root / 'Dockerfile').write_text('''FROM node:22-alpine
ARG VERSION
ARG COMMIT
LABEL org.opencontainers.image.version=$VERSION org.opencontainers.image.revision=$COMMIT
ENV FIXTURE_VERSION=$VERSION FIXTURE_COMMIT=$COMMIT
WORKDIR /fixture
COPY server.cjs imports.json ./
CMD ["node", "server.cjs"]
''')
            references, manifests = {}, []
            for number in (1, 2):
                version = f'{number}.0.0'
                tag = project + ':' + version
                tags.append(tag)
                docker('build', '-q', '-t', tag, '--build-arg', 'VERSION=' + version, '--build-arg', 'COMMIT=' + COMMIT, str(root))
                identity = json.loads(docker('image', 'inspect', tag))[0]['Id']
                images = {}
                for component in ('backend', 'frontend'):
                    image = {'repository': f'ghcr.io/fixture/portal-{component}', 'digest': identity,
                             'version': version, 'commit': COMMIT, 'tag': version}
                    images[component] = image
                    references[image['repository'] + '@' + identity] = identity
                manifests.append({'schema_version': 1, 'source_commit': COMMIT, 'release_id': f'{COMMIT}-{number}-1', 'images': images})
            transport = LocalTransport(project, references)
            env_file = root / 'fixture.env'
            env_file.write_text('FIXTURE_SETTING=A\nBROKEN_VERSION=0\n')
            secret = root / 'secret'
            secret.write_text('fixture-only')
            config = root / 'config.json'
            config.write_text(json.dumps({'schema_version': 1, 'mode': 'prod', 'deploy_root': str(root),
                                         'env_file': str(env_file), 'verify_public_edge': False, 'wait_timeout': 30,
                                         'secret_files': {key: str(secret) for key in ('DB_PASSWORD_SECRET_FILE', 'KEYCLOAK_ADMIN_SECRET_FILE', 'KEYCLOAK_WEB_SECRET_FILE')}}))
            server = Server(config, runner=transport)
            releases = []
            for manifest in manifests:
                release = root / 'releases' / manifest['release_id']
                scripts, compose = release / 'infra/scripts', release / 'infra/compose'
                scripts.mkdir(parents=True)
                compose.mkdir()
                (release / 'deployment.json').write_text(json.dumps(manifest))
                (scripts / 'verify-frontend-deployment.mjs').write_bytes((ROOT / 'infra/scripts/verify-frontend-deployment.mjs').read_bytes())
                services = {component: {'image': 'overridden-by-digest', 'env_file': ['${APP_ENV_FILE}'], 'environment': {'ROLE': component},
                            'healthcheck': {'test': ['CMD', 'wget', '-qO-', f'http://127.0.0.1:{9000 if component == "backend" else 3000}/health/ready'], 'interval': '1s', 'timeout': '2s', 'retries': 10}}
                            for component in ('backend', 'frontend')}
                services['backend']['volumes'] = ['fixture_data:/data']
                (compose / 'docker-compose.prod.yml').write_text(json.dumps({'services': services, 'volumes': {'fixture_data': {}}}))
                releases.append(release)
            server.deploy(releases[0], 'fixture')
            before = json.loads((server.state / 'deployment.json').read_text())
            env_file.write_text('FIXTURE_SETTING=B\nBROKEN_VERSION=1\n')
            try:
                server.deploy(releases[1], 'fixture')
            except ValueError as error:
                assert 'version/commit mismatch' in str(error), str(error)
            else:
                raise AssertionError('Invalid runtime version was accepted')
            after = json.loads((server.state / 'deployment.json').read_text())
            assert after == before
            current = after['current']
            server.verify(current)
            assert server.compose(current, 'exec', '-T', 'backend', 'wget', '-qO-', 'http://127.0.0.1:9000/setting').strip() == 'A'
            assert server.compose(current, 'exec', '-T', 'backend', 'cat', '/data/marker').strip() == 'preserved'
            container = server.compose(current, 'ps', '-q', 'backend').strip()
            actual = json.loads(docker('inspect', container))[0]['Image']
            assert actual == manifests[0]['images']['backend']['digest']
            assert not (server.state / 'pending.json').exists()
            assert 'automatic-rollback' in (server.state / 'history.jsonl').read_text()
            env_file.write_text('FIXTURE_SETTING=B\nBROKEN_VERSION=0\n')
            server.deploy(releases[1], 'fixture')
            server.rollback('fixture')
            assert json.loads((server.state / 'deployment.json').read_text())['current']['manifest'] == manifests[0]
            print('PASS: real container rollout, failed-version detection, automatic/operator rollback, image identity, env snapshots and persistent data')
        finally:
            if transport and transport.last_compose:
                subprocess.run([*transport.last_compose, 'down', '-v', '--remove-orphans'], env={**os.environ, 'APP_ENV_FILE': str(root / 'fixture.env')}, capture_output=True)
            for tag in tags:
                subprocess.run(['docker', 'image', 'rm', tag], capture_output=True)


if __name__ == '__main__':
    main()
