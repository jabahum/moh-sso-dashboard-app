#!/usr/bin/env python3
"""Smoke-test built distribution images in an isolated Compose project.

Uses only generated fixture secrets and example settings. Starts the real
frontend, primary/Observatory databases and Redis, without contacting Keycloak,
warehouse databases or an existing deployment. Does not test authenticated SSO.
"""
import argparse
import http.client
import json
import os
from pathlib import Path
import subprocess
import tempfile
import uuid

ROOT = Path(__file__).resolve().parents[2]


def run(*args, env=None):
    result = subprocess.run(args, env=env, cwd=ROOT, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(result.stderr[-5000:])
    return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--backend-image', required=True)
    parser.add_argument('--frontend-image', required=True)
    args = parser.parse_args()
    project = 'moh-distribution-fixture-' + uuid.uuid4().hex[:10]
    with tempfile.TemporaryDirectory(prefix=project) as directory:
        directory = Path(directory)
        secret = directory / 'fixture-secret'
        secret.write_text('isolated-distribution-fixture-only\n')
        secret.chmod(0o600)
        env = {
            **os.environ,
            'APP_ENV_FILE': str(ROOT / 'infra/env/prod.env.example'),
            'BACKEND_TAG': 'fixture', 'FRONTEND_TAG': 'fixture',
            'DISTRIBUTION_TAG': '0.1.0-fixture',
            'DB_USER': 'fixture_primary', 'DB_NAME': 'sso_dashboard',
            'OBSERVATORY_DB_USER': 'fixture_observatory',
            'OBSERVATORY_DB_NAME': 'fixture_observatory',
            'REDIS_PASSWORD': 'isolated-redis-fixture-only',
            **{key: str(secret) for key in (
                'DB_PASSWORD_SECRET_FILE', 'OBSERVATORY_DB_PASSWORD_SECRET_FILE',
                'KEYCLOAK_ADMIN_SECRET_FILE', 'KEYCLOAK_WEB_SECRET_FILE')},
        }
        for key in ('COMPOSE_FILE', 'COMPOSE_PROFILES', 'COMPOSE_PROJECT_NAME'):
            env.pop(key, None)
        model = json.loads(run('docker', 'compose', '--env-file',
                               'infra/env/prod.env.example', '-f',
                               'infra/compose/docker-compose.distribution.yml',
                               'config', '--format', 'json', env=env))
        model['services']['frontend']['image'] = args.frontend_image
        model['services']['backend']['image'] = args.backend_image
        model['services']['frontend']['ports'] = [
            {'target': 3000, 'published': '0', 'host_ip': '127.0.0.1', 'protocol': 'tcp'}]
        model['services']['frontend']['environment']['API_BASE_URL'] = 'https://fixture.example.org/ssobackend'
        # The exact runtime services/settings are retained, with local images and
        # a unique project. Do not pull registry application images or start SSO.
        for service in model['services'].values():
            service.pop('build', None)
        configuration = directory / 'compose.json'
        configuration.write_text(json.dumps(model))
        compose = ['docker', 'compose', '-p', project, '-f', str(configuration)]
        try:
            version = run('docker', 'run', '--rm', args.backend_image, '--version')
            assert 'moh-sso-dashboard-backend' in version, version
            run(*compose, 'up', '-d', '--no-build', '--no-deps', '--wait', '--wait-timeout', '120',
                'backend-db', 'observatory-db', 'redis', 'frontend')
            port = int(run(*compose, 'port', 'frontend', '3000').rsplit(':', 1)[1])

            def get(path):
                connection = http.client.HTTPConnection('127.0.0.1', port, timeout=5)
                try:
                    connection.request('GET', path)
                    response = connection.getresponse()
                    return response.status, response.headers, response.read().decode()
                finally:
                    connection.close()

            assert get('/nginx/health')[0] == 200
            assert get('/portal/apps/extensions/example-reports')[0] == 200
            status, headers, text = get('/portal/config.js')
            assert status == 200 and headers['Cache-Control'] == 'no-store'
            assert 'https://fixture.example.org/ssobackend' in text
            assert 'isolated-distribution-fixture-only' not in text
            status, _, text = get('/portal/version-manifest.json')
            assert status == 200
            manifest = json.loads(text)
            image = json.loads(run('docker', 'image', 'inspect', args.frontend_image))[0]
            assert manifest['service']['version'] == image['Config']['Labels']['org.opencontainers.image.version']
            pins = json.loads((ROOT / 'frontend/module-sources.json').read_text())['production']['versions']
            for group in ('apps', 'packages'):
                for name, item in manifest[group].items():
                    if group == 'apps' and name == 'shell':
                        continue  # The shell is built from this checkout, not npm.
                    assert item['version'] == pins['@moh-sso/' + name], (group, name)
            assert get('/portal/mf/announcements/single-spa.js')[0] == 200
            assert get('/portal/mf/announcements/missing.js')[0] == 404
            run(*compose, 'exec', '-T', 'backend-db', 'sh', '-ec',
                'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 '
                '-c "CREATE TABLE distribution_fixture (value text); INSERT INTO distribution_fixture VALUES (\'preserved\');"')
            run(*compose, 'up', '-d', '--no-deps', '--force-recreate', '--wait', 'backend-db')
            value = run(*compose, 'exec', '-T', 'backend-db', 'sh', '-ec',
                        'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT value FROM distribution_fixture"')
            assert value == 'preserved', value
            run(*compose, 'exec', '-T', 'observatory-db', 'sh', '-ec',
                'test "$POSTGRES_DB" != sso_dashboard; pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"')
            print('PASS: built images, Compose readiness, mounted secrets, published frontend modules, '
                  'public runtime configuration, database persistence and Observatory isolation')
        finally:
            subprocess.run([*compose, 'down', '-v', '--remove-orphans'],
                           cwd=ROOT, capture_output=True)


if __name__ == '__main__':
    main()
