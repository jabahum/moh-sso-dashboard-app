#!/usr/bin/env python3
"""Verify the pinned self-contained GeoJSON artifact in a fresh module cache."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile

from backend_module_pins import module_pins
from module_release import artifact_differences, skip_if_pending

ROOT = Path(__file__).resolve().parents[2]
MODULE = 'github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson'
VERSION = module_pins()[MODULE]
CONSUMER = 'example.com/geojson-release-consumer'

def main():
    if skip_if_pending(MODULE):
        return
    spec = importlib.util.spec_from_file_location('geojson_package', Path(__file__).with_name('test-geojson-package.py'))
    package = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(package)
    with tempfile.TemporaryDirectory(prefix='moh-geojson-release-') as temporary:
        root = Path(temporary)
        consumer = root / 'consumer'
        consumer.mkdir()
        env = {**os.environ, 'GOWORK': 'off', 'GOTOOLCHAIN': 'local',
               'GOMODCACHE': str(root / 'fresh-cache'), 'GOSUMDB': 'sum.golang.org',
               'GONOSUMDB': '', 'GOPRIVATE': ''}
        (consumer / 'go.mod').write_text('module ' + CONSUMER + '\n\ngo 1.25.4\n\nrequire ' + MODULE + ' ' + VERSION + '\n')
        sums = {}
        for line in (ROOT / 'backend/go.sum').read_text().splitlines():
            module, version, checksum = line.split()
            if module == MODULE and version in (VERSION, VERSION + '/go.mod'):
                sums[version] = checksum
        assert len(sums) == 2, 'Both reviewed checksums are required'
        (consumer / 'go.sum').write_text(''.join(f'{MODULE} {version} {sums[version]}\n' for version in (VERSION, VERSION + '/go.mod')))
        source = package.CONSUMER_SOURCE.replace('geojson.Version() != "dev"', 'geojson.Version() != "' + VERSION + '"')
        source = source.replace('consumer (dev)', 'consumer (' + VERSION + ')')
        (consumer / 'main.go').write_text(source)
        before = package.metadata(consumer)
        def run(*args):
            return subprocess.run([os.environ.get('GO_CMD', 'go'), *args], cwd=consumer, env=env,
                                  text=True, capture_output=True, check=True).stdout.strip()
        downloaded = json.loads(run('mod', 'download', '-json', MODULE + '@' + VERSION))
        assert downloaded['Sum'] == sums[VERSION] and downloaded['GoModSum'] == sums[VERSION + '/go.mod']
        selected = json.loads(run('list', '-mod=readonly', '-m', '-json', MODULE))
        assert selected['Version'] == VERSION and not selected.get('Replace')
        assert set(run('list', '-mod=readonly', '-m', '-f', '{{.Path}}', 'all').splitlines()) == {CONSUMER, MODULE}
        run('mod', 'verify')
        run('vet', '-mod=readonly', './...')
        binary = root / 'consumer-binary'
        run('build', '-race', '-mod=readonly', '-o', str(binary), '.')
        empty = root / 'empty-working-directory'
        empty.mkdir()
        result = subprocess.run([str(binary)], cwd=empty, env=env, text=True, capture_output=True, check=True).stdout.strip()
        assert result == 'PASS: separate self-contained GeoJSON consumer (' + VERSION + ')', result
        assert package.metadata(consumer) == before
        released = Path(downloaded['Dir'])
        local = ROOT / 'backend/modules/geojson'
        # Shared comparison: tolerates the root LICENSE Go adds to module zips.
        differences = artifact_differences(local, released, ROOT / 'LICENSE')
        assert not differences, 'Release drift: ' + '; '.join(differences)
        print(result, flush=True)
        print('PASS: fresh public download, checksum verification, exact dependency graph, no replacement, embedded HTTP consumer and complete release artifact parity', flush=True)

if __name__ == '__main__':
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f'GeoJSON release verification failed: {error.stdout}\n{error.stderr}') from None
