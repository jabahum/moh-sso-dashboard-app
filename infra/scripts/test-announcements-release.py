#!/usr/bin/env python3
"""Verify the pinned published Announcements release with a fresh module cache."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

from backend_module_pins import module_pins
from module_release import artifact_differences, skip_if_pending

ROOT = Path(__file__).resolve().parents[2]
MODULE = 'github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements'
VERSION = module_pins()[MODULE]


def main():
    if skip_if_pending(MODULE):
        return
    go = os.environ.get('GO_CMD', 'go')
    with tempfile.TemporaryDirectory(prefix='moh-announcements-release-') as tmp:
        root = Path(tmp)
        consumer = root / 'consumer'
        consumer.mkdir()
        env = {**os.environ, 'GOWORK': 'off', 'GOTOOLCHAIN': 'local',
               'GOMODCACHE': str(root / 'fresh-module-cache'), 'GOSUMDB':'sum.golang.org', 'GONOSUMDB':'', 'GOPRIVATE':''}
        # This is a release verification, so normal checksum verification and
        # registry downloads remain enabled. There is no local replacement.
        (consumer / 'go.mod').write_text(
            'module example.com/announcements-release-consumer\n\ngo 1.25.4\n\n'
            'require ' + MODULE + ' ' + VERSION + '\n'
            'require github.com/google/uuid v1.6.0\nrequire github.com/lib/pq v1.10.9\n')
        sums = [line for line in (ROOT / 'backend/go.sum').read_text().splitlines()
                if line.startswith((MODULE + ' ' + VERSION, 'github.com/google/uuid v1.6.0', 'github.com/lib/pq v1.10.9'))]
        assert len(sums) == 6, 'Reviewed release and dependency checksums are required'
        (consumer / 'go.sum').write_text('\n'.join(sums) + '\n')
        source = (ROOT / 'backend/modules/announcements/testdata/consumer/main.go').read_text()
        source = source.replace('announcements.Version() != "dev"', 'announcements.Version() != "' + VERSION + '"')
        source = source.replace('consumer (dev)', 'consumer (' + VERSION + ')')
        (consumer / 'main.go').write_text(source)
        before = {name: (consumer / name).read_bytes() for name in ('go.mod', 'go.sum')}

        def run(*args):
            result = subprocess.run([go, *args], cwd=consumer, env=env,
                                    text=True, capture_output=True, check=True)
            return result.stdout.strip()

        downloaded = json.loads(run('mod', 'download', '-json', MODULE + '@' + VERSION))
        assert downloaded['Sum'] in (consumer / 'go.sum').read_text()
        assert downloaded['GoModSum'] in (consumer / 'go.sum').read_text()
        selected = json.loads(run('list', '-mod=readonly', '-m', '-json', MODULE))
        assert selected['Version'] == VERSION and not selected.get('Replace'), selected
        run('mod', 'download', 'github.com/google/uuid', 'github.com/lib/pq')
        run('mod', 'verify')
        run('build', '-mod=readonly', './...')
        run('vet', '-mod=readonly', './...')
        output = run('run', '-race', '-mod=readonly', '.')
        assert 'separate Announcements consumer (' + VERSION + ')' in output, output
        print(output, flush=True)
        assert all((consumer / name).read_bytes() == content for name, content in before.items())
        # Compare the entire public artifact, not only source files.
        local = ROOT / 'backend/modules/announcements'
        released = Path(downloaded['Dir'])
        # Shared comparison: tolerates the root LICENSE Go adds to module zips.
        differences = artifact_differences(local, released, ROOT / 'LICENSE')
        assert not differences, 'Release drift: ' + '; '.join(differences)
        graph = {line.strip() for line in run('list', '-mod=readonly', '-m', '-f', '{{.Path}} {{.Version}}', 'all').splitlines()}
        assert graph == {'example.com/announcements-release-consumer',
                         MODULE + ' ' + VERSION, 'github.com/google/uuid v1.6.0',
                         'github.com/lib/pq v1.10.9'}, graph
        assert not run('list', '-mod=readonly', '-m', '-f', '{{if .Replace}}{{.Path}}{{end}}', 'all')
        # Build the independent HTTP consumer against the downloaded release.
        (consumer / 'main.go').write_text((local/'testdata/http-consumer/main.go').read_text())
        run('build', '-race', '-mod=readonly', '-o', str(root/'http-consumer'), '.')
        run('vet', '-mod=readonly', './...')
        print(f'PASS: published {VERSION} download/checksums, fresh-cache consumer, exact graph, HTTP build and complete artifact parity', flush=True)


if __name__ == '__main__':
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f'Published Announcements verification failed: {error.stdout}\n{error.stderr}') from None
