#!/usr/bin/env python3
"""Apply fresh/previous-schema migrations in disposable PostgreSQL only."""
import os
from pathlib import Path
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[2]


def docker(*args):
    return subprocess.check_output(['docker', *args], text=True).strip()


def main():
    name = 'moh-migrations-' + uuid.uuid4().hex[:12]
    # Loopback only. Empty means an ephemeral port; MIGRATIONS_CHECK_HOST_PORT pins
    # it (the identity-sync e2e run keeps every host port in 18400-18499).
    host_port = os.environ.get('MIGRATIONS_CHECK_HOST_PORT', '')
    if host_port and not (host_port.isdigit() and 1 <= int(host_port) <= 65535):
        raise SystemExit('MIGRATIONS_CHECK_HOST_PORT must be a port number')
    try:
        docker('run', '-d', '--name', name, '--label', 'moh.fixture=migrations',
               '--tmpfs', '/var/lib/postgresql/data', '-p', f'127.0.0.1:{host_port}:5432',
               '-e', 'POSTGRES_PASSWORD=fixture-only', 'postgres:15-alpine')
        deadline = time.monotonic() + 60
        while subprocess.run(['docker', 'exec', name, 'pg_isready', '-U', 'postgres'], capture_output=True).returncode:
            if time.monotonic() > deadline:
                raise RuntimeError('Disposable PostgreSQL did not become ready')
            time.sleep(.5)
        port = docker('port', name, '5432/tcp').rsplit(':', 1)[1]
        env = {**os.environ, 'TEST_MIGRATION_DATABASE_URL': f'postgres://postgres:fixture-only@127.0.0.1:{port}/postgres?sslmode=disable'}
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', '-tags=integration', './internal/migrate', '-run', '^(TestMigrationPaths|TestStandardUserDefaultsMigration)$', '-count=1', '-v', '-timeout=5m'],
                       cwd=ROOT / 'backend', env=env, check=True)
        env['MOH_OBSERVATORY_TEST_DATABASE_URL'] = env['TEST_MIGRATION_DATABASE_URL']
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', './internal/bootstrap', '-run', '^TestObservatoryReconnectPostgres$', '-count=1', '-v', '-timeout=3m'],
                       cwd=ROOT / 'backend', env=env, check=True)
        env['MOH_HEALTH_CONTEXT_TEST_DATABASE_URL'] = env['TEST_MIGRATION_DATABASE_URL']
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', './internal/features/health_context', '-run', '^(TestGeographySyncAcrossDatabases|TestNationalDefaultPostgres)$', '-count=1', '-v', '-timeout=3m'],
                       cwd=ROOT / 'backend', env=env, check=True)
        env['MOH_REGISTRY_TEST_DATABASE_URL'] = env['TEST_MIGRATION_DATABASE_URL']
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', './internal/features/rbac', '-run', '^(TestApplicationRegistryPostgres|TestServiceOnlyRolesPostgres)$', '-count=1', '-v', '-timeout=5m'],
                       cwd=ROOT / 'backend', env=env, check=True)
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', './internal/features/platform', '-run', '^TestPlatformPostgres$', '-count=1', '-v', '-timeout=5m'],
                       cwd=ROOT / 'backend', env=env, check=True)
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', './internal/repository/notification_delivery', '-run', '^TestNotificationDeliveryPostgres$', '-count=1', '-v', '-timeout=3m'],
                       cwd=ROOT / 'backend', env=env, check=True)
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', './internal/identityreconcile', '-run', '^(TestIdentitySyncRegistryPostgres|TestIdentitySyncBatchPostgres|TestIdentitySyncRevocationsPostgres|TestIdentitySyncAccessLockPostgres|TestIdentitySyncGrantRevokeInterleavingPostgres|TestIdentitySyncIntegrationDisableAfterBindingBreaksPostgres)$', '-count=1', '-v', '-timeout=5m'],
                       cwd=ROOT / 'backend', env=env, check=True)
    finally:
        subprocess.run(['docker', 'rm', '-f', name], capture_output=True)


if __name__ == '__main__':
    main()
