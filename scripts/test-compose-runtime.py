#!/usr/bin/env python3
"""Build the real production runtime stages with tiny fixtures; requires Docker.

Checks container entrypoints, non-root uploads, public config and Keycloak health.
Application compilation and external services are deliberately not involved.
"""
import http.client
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]


def docker(*args):
    result = subprocess.run(['docker', *args], capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(f"docker {args[0]} failed: {result.stderr[-4000:]}")
    return result.stdout.strip()


def main():
    prefix = 'moh-compose-check-' + uuid.uuid4().hex[:10]
    images, containers = [], []
    try:
        with tempfile.TemporaryDirectory(prefix='moh-compose-runtime-') as tmp:
            tmp = Path(tmp)
            # Supply only build outputs; execute the actual checked-in runtime stages.
            frontend = tmp / 'frontend'
            (frontend / 'docker').mkdir(parents=True)
            (frontend / 'fixture/portal').mkdir(parents=True)
            (frontend / 'fixture/portal/index.html').write_text('<!doctype html><title>Runtime fixture</title>')
            (frontend / 'fixture/index.html').write_text('<!doctype html><title>Runtime fixture</title>')
            (frontend / 'docker/40-runtime-config.sh').write_bytes((ROOT / 'frontend/docker/40-runtime-config.sh').read_bytes())
            (frontend / 'nginx.conf').write_bytes((ROOT / 'frontend/nginx.conf').read_bytes())
            stage = (ROOT / 'frontend/Dockerfile').read_text().split('FROM nginx:', 1)[1]
            (frontend / 'Dockerfile').write_text('FROM nginx:1.30.6-alpine AS build\nCOPY fixture /app/dist\nFROM nginx:' + stage)
            image = prefix + ':frontend'
            images.append(image)
            docker('build', '-q', '-t', image, str(frontend))
            container = prefix + '-frontend'
            containers.append(container)
            docker('run', '-d', '--name', container, '-p', '127.0.0.1::3000',
                   '-e', 'API_BASE_URL=https://edge.example.org/ssobackend',
                   '-e', 'DB_PASSWORD=never-publish', image)
            port = int(docker('port', container, '3000/tcp').rsplit(':', 1)[1])

            def get(path):
                conn = http.client.HTTPConnection('127.0.0.1', port, timeout=3)
                try:
                    conn.request('GET', path)
                    response = conn.getresponse()
                    return response.status, response.headers, response.read().decode()
                finally:
                    conn.close()

            deadline = time.monotonic() + 30
            while True:
                try:
                    assert get('/nginx/health')[0] == 200
                    break
                except (OSError, http.client.HTTPException, AssertionError):
                    if time.monotonic() > deadline:
                        raise RuntimeError('Frontend failed startup: ' + docker('logs', container))
                    time.sleep(.25)
            for path in ('/config.js', '/portal/config.js'):
                status, headers, text = get(path)
                assert status == 200 and headers['Cache-Control'] == 'no-store'
                data = json.loads(text.split('window.__APP_CONFIG__ = ', 1)[1].strip().removesuffix(';'))
                assert data['API_BASE_URL'] == 'https://edge.example.org/ssobackend'
                assert data['singleSpaOrchestration'] is True
                assert 'never-publish' not in text

            backend = tmp / 'backend'
            for path in ('scripts', 'fixture/internal/db/migrations', 'fixture/config'):
                (backend / path).mkdir(parents=True)
            entrypoint = ROOT / 'backend/scripts/container-entrypoint.sh'
            (backend / 'scripts/container-entrypoint.sh').write_bytes(entrypoint.read_bytes())
            executable = backend / 'fixture/sso-server'
            executable.write_text('''#!/bin/sh
set -eu
test "$DB_PASSWORD" = 'fixture-password'
test "$(id -u)" != 0
touch /data/uploads/runtime-permission-check
printf 'secret-loading-and-nonroot-upload-ok'
''')
            executable.chmod(0o755)
            stage = (ROOT / 'backend/Dockerfile').read_text().split('FROM alpine:', 1)[1]
            (backend / 'Dockerfile').write_text('FROM nginx:1.30.6-alpine AS builder\nCOPY fixture /app\nFROM alpine:' + stage)
            image = prefix + ':backend'
            images.append(image)
            docker('build', '-q', '-t', image, str(backend))
            secret = tmp / 'password'
            secret.write_text('fixture-password\n')
            secret.chmod(0o644)
            result = docker('run', '--rm', '-v', f'{secret}:/run/secrets/db:ro',
                            '-v', '/data/uploads', '-e', 'DB_PASSWORD=placeholder',
                            '-e', 'DB_PASSWORD_FILE=/run/secrets/db', image)
            assert result == 'secret-loading-and-nonroot-upload-ok'

            # Use the exact development healthcheck against a fresh local IdP.
            spec = importlib.util.spec_from_file_location('compose_contracts', ROOT / 'scripts/test-compose.py')
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            service = module.config('docker-compose.dev.yml', 'backend/app.env.example')['services']['keycloak']
            container = prefix + '-keycloak'
            containers.append(container)
            docker('run', '-d', '--name', container, '-e', 'KC_DB=dev-file',
                   '-e', 'KC_HEALTH_ENABLED=true', '-e', 'KC_HTTP_ENABLED=true',
                   '-e', 'KC_HOSTNAME=http://localhost:8081', service['image'], 'start-dev')
            deadline = time.monotonic() + 180
            while True:
                result = subprocess.run(['docker', 'exec', container, *service['healthcheck']['test'][1:]],
                                        capture_output=True, text=True)
                if result.returncode == 0:
                    break
                if time.monotonic() > deadline:
                    raise RuntimeError('Keycloak health check failed: ' + docker('logs', container))
                time.sleep(2)
            print('PASS: real frontend/backend runtime stages, runtime config, secret loading, non-root upload permissions and development Keycloak health check')
    finally:
        for container in containers:
            subprocess.run(['docker', 'rm', '-f', container], capture_output=True)
        for image in images:
            subprocess.run(['docker', 'image', 'rm', image], capture_output=True)


if __name__ == '__main__':
    main()
