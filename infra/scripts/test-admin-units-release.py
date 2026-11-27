#!/usr/bin/env python3
"""Verify the pinned Administrative Units release in a fresh module cache."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

from backend_module_pins import module_pins
from module_release import artifact_differences, skip_if_pending

ROOT = Path(__file__).resolve().parents[2]
MODULE = 'github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units'
VERSION = module_pins()[MODULE]
CONSUMER = 'example.com/admin-units-release-consumer'


def main():
    if skip_if_pending(MODULE):
        return
    go = os.environ.get('GO_CMD', 'go')
    with tempfile.TemporaryDirectory(prefix='moh-admin-units-release-') as tmp:
        root = Path(tmp)
        consumer = root / 'consumer'
        consumer.mkdir()
        env = {**os.environ, 'GOWORK': 'off', 'GOTOOLCHAIN': 'local',
               'GOMODCACHE': str(root / 'fresh-module-cache'), 'GOSUMDB': 'sum.golang.org',
               'GONOSUMDB': '', 'GOPRIVATE': ''}
        (consumer / 'go.mod').write_text(
            'module ' + CONSUMER + '\n\ngo 1.25.4\n\nrequire ' + MODULE + ' ' + VERSION + '\n')
        checksums = {}
        for line in (ROOT / 'backend/go.sum').read_text().splitlines():
            path, version, checksum = line.split()
            if path == MODULE and version in (VERSION, VERSION + '/go.mod'):
                checksums[version] = checksum
        assert len(checksums) == 2, 'Reviewed release checksums are required'
        (consumer / 'go.sum').write_text(''.join(
            f'{MODULE} {version} {checksums[version]}\n'
            for version in (VERSION, VERSION + '/go.mod')))
        source = (ROOT / 'backend/modules/admin-units/testdata/consumer/main.go').read_text()
        assert 'units.Version() != "dev"' in source
        source = source.replace('units.Version() != "dev"', 'units.Version() != "' + VERSION + '"')
        source = source.replace('consumer (dev)', 'consumer (' + VERSION + ')')
        (consumer / 'main.go').write_text(source)
        before = {name: (consumer / name).read_bytes() for name in ('go.mod', 'go.sum')}

        def run(*args):
            result = subprocess.run([go, *args], cwd=consumer, env=env,
                                    text=True, capture_output=True, check=True)
            return result.stdout.strip()

        downloaded = json.loads(run('mod', 'download', '-json', MODULE + '@' + VERSION))
        assert downloaded['Sum'] == checksums[VERSION]
        assert downloaded['GoModSum'] == checksums[VERSION + '/go.mod']
        selected = json.loads(run('list', '-mod=readonly', '-m', '-json', MODULE))
        assert selected['Version'] == VERSION and not selected.get('Replace'), selected
        modules = set(run('list', '-mod=readonly', '-m', '-f', '{{.Path}}', 'all').splitlines())
        assert modules == {CONSUMER, MODULE}, modules
        run('mod', 'verify')
        run('build', '-mod=readonly', './...')
        run('vet', '-mod=readonly', './...')
        output = run('run', '-race', '-mod=readonly', '.')
        assert 'separate Administrative Units consumer (' + VERSION + ')' in output, output
        print(output, flush=True)
        assert all((consumer / name).read_bytes() == value for name, value in before.items())
        released = Path(downloaded['Dir'])
        local = ROOT / 'backend/modules/admin-units'
        # Shared comparison: tolerates the root LICENSE Go adds to module zips.
        differences = artifact_differences(local, released, ROOT / 'LICENSE')
        assert not differences, 'Release drift: ' + '; '.join(differences)
        source = (local / 'testdata/http-consumer/main.go').read_text()
        source = source.replace('if err != nil {', 'if units.Version() != "' + VERSION + '" { panic("unexpected published version") }; if err != nil {', 1)
        (consumer / 'main.go').write_text(source)
        run('vet', '-mod=readonly', './...')
        binary = root / 'http-consumer'
        run('build', '-race', '-mod=readonly', '-o', str(binary), '.')
        empty = root / 'empty'
        empty.mkdir()
        output = subprocess.run([str(binary)], cwd=empty, env=env, text=True,
                                capture_output=True, check=True).stdout.strip()
        assert output == 'PASS: standalone Administrative Units HTTP consumer, scoped lookup and package-owned routes', output
        print(output, flush=True)
        assert all((consumer / name).read_bytes() == value for name, value in before.items())
        print(f'PASS: published {VERSION} download/checksums, exact dependency graph, no replacement, fresh-cache HTTP consumer and complete artifact parity', flush=True)


if __name__ == '__main__':
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f'Published Administrative Units verification failed: {error.stdout}\\n{error.stderr}') from None
