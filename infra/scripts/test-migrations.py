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
    try:
        docker('run', '-d', '--name', name, '--label', 'moh.fixture=migrations',
               '--tmpfs', '/var/lib/postgresql/data', '-p', '127.0.0.1::5432',
               '-e', 'POSTGRES_PASSWORD=fixture-only', 'postgres:15-alpine')
        deadline = time.monotonic() + 60
        while subprocess.run(['docker', 'exec', name, 'pg_isready', '-U', 'postgres'], capture_output=True).returncode:
            if time.monotonic() > deadline:
                raise RuntimeError('Disposable PostgreSQL did not become ready')
            time.sleep(.5)
        port = docker('port', name, '5432/tcp').rsplit(':', 1)[1]
        env = {**os.environ, 'TEST_MIGRATION_DATABASE_URL': f'postgres://postgres:fixture-only@127.0.0.1:{port}/postgres?sslmode=disable'}
        subprocess.run([os.environ.get('GO_CMD', 'go'), 'test', '-tags=integration', './internal/migrate', '-run', '^TestMigrationPaths$', '-count=1', '-v', '-timeout=5m'],
                       cwd=ROOT / 'backend', env=env, check=True)
    finally:
        subprocess.run(['docker', 'rm', '-f', name], capture_output=True)


if __name__ == '__main__':
    main()
