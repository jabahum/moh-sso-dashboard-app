#!/usr/bin/env python3
"""Verify the complete portable platform core without host files or downloads."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
MODULE = 'github.com/mohuganda/moh-sso-dashboard/backend/core'


def main():
    with tempfile.TemporaryDirectory(prefix='moh-platform-core-') as tmp:
        root = Path(tmp) / 'core'
        shutil.copytree(ROOT / 'backend/core', root)
        before = (root / 'go.mod').read_bytes()
        env = {**os.environ, 'GOWORK': 'off', 'GOTOOLCHAIN': 'local',
               'GOPROXY': 'off', 'GOMODCACHE': str(Path(tmp) / 'empty-module-cache')}
        go = os.environ.get('GO_CMD', 'go')

        def run(*args):
            return subprocess.run([go, *args], cwd=root, env=env, text=True,
                                  capture_output=True, check=True).stdout.strip()

        modules = set(run('list', '-mod=readonly', '-deps', '-test', '-f',
                          '{{if .Module}}{{.Module.Path}}{{end}}', './...').splitlines()) - {''}
        assert modules == {MODULE}, 'Host/provider dependency entered core: ' + repr(modules)
        run('mod', 'verify')
        run('build', '-mod=readonly', './...')
        run('vet', '-mod=readonly', './...')
        print(run('test', '-race', '-mod=readonly', '-count=1', '-timeout=2m', './...'), flush=True)
        assert (root / 'go.mod').read_bytes() == before
        assert not any((root / name).exists() for name in ('go.sum', 'go.work', 'go.work.sum'))
        print('PASS: entire platform core builds, vets and race-tests with standard library only, empty cache and downloads disabled', flush=True)


if __name__ == '__main__':
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f'Platform core verification failed: {error.stdout}\n{error.stderr}') from None
