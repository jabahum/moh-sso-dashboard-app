#!/usr/bin/env python3
"""Verify the pinned Audit public artifact and a fresh-cache HTTP consumer."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
from backend_module_pins import module_pins
from module_release import artifact_differences, skip_if_pending

ROOT = Path(__file__).resolve().parents[2]
MODULE = 'github.com/mohuganda/moh-sso-dashboard/backend/modules/audit'
VERSION = module_pins()[MODULE]
CONSUMER = 'example.com/audit-release-consumer'

def main():
    if skip_if_pending(MODULE):
        return
    go = os.environ.get('GO_CMD', 'go')
    local = ROOT / 'backend/modules/audit'
    baseline = subprocess.run([go, 'list', '-mod=readonly', '-m', '-f', '{{.Path}} {{.Version}}', 'all'],
        cwd=local, env={**os.environ, 'GOWORK':'off'}, text=True, capture_output=True, check=True).stdout.splitlines()
    expected = {line for line in baseline if line.strip() != MODULE}
    expected |= {CONSUMER, MODULE + ' ' + VERSION, 'github.com/lib/pq v1.10.9'}
    with tempfile.TemporaryDirectory(prefix='moh-audit-release-') as directory:
        root = Path(directory)
        consumer = root / 'consumer'
        consumer.mkdir()
        manifest = (local / 'go.mod').read_text().replace('module ' + MODULE, 'module ' + CONSUMER, 1)
        manifest += '\nrequire ' + MODULE + ' ' + VERSION + '\nrequire github.com/lib/pq v1.10.9\n'
        (consumer / 'go.mod').write_text(manifest)
        sums = [line for line in (ROOT / 'backend/go.sum').read_text().splitlines()
                if line.startswith(MODULE + ' ' + VERSION)]
        assert len(sums) == 2, 'Both reviewed release checksums required'
        driver_sums=[line for line in (ROOT/'backend/go.sum').read_text().splitlines() if line.startswith('github.com/lib/pq v1.10.9')]
        (consumer / 'go.sum').write_text((local / 'go.sum').read_text() + '\n'.join(sums+driver_sums) + '\n')
        source = (local / 'testdata/http-consumer/main.go').read_text()
        source = source.replace('func main() {', 'func main() {\n check(audit.Version()=="' + VERSION + '","published version mismatch")', 1)
        (consumer / 'main.go').write_text(source)
        before = {name:(consumer / name).read_bytes() for name in ('go.mod','go.sum')}
        env = {**os.environ, 'GOWORK':'off', 'GOTOOLCHAIN':'local',
               'GOMODCACHE':str(root / 'fresh-cache'), 'GOSUMDB':'sum.golang.org', 'GONOSUMDB':'', 'GOPRIVATE':''}
        env.pop('AUDIT_TEST_DSN', None)
        def run(*args):
            return subprocess.run([go,*args],cwd=consumer,env=env,text=True,capture_output=True,check=True).stdout.strip()
        downloaded = json.loads(run('mod','download','-json',MODULE+'@'+VERSION))
        checksums = {line.split()[1]:line.split()[2] for line in sums}
        assert downloaded['Sum']==checksums[VERSION] and downloaded['GoModSum']==checksums[VERSION+'/go.mod']
        selected = json.loads(run('list','-mod=readonly','-m','-json',MODULE))
        assert selected['Version']==VERSION and not selected.get('Replace')
        graph = {line.strip() for line in run('list','-mod=readonly','-m','-f','{{.Path}} {{.Version}}','all').splitlines()}
        assert graph==expected, 'Release graph drift: '+str(graph^expected)
        assert not run('list','-mod=readonly','-m','-f','{{if .Replace}}{{.Path}}{{end}}','all')
        run('mod','download')
        run('mod','verify')
        run('vet','-mod=readonly','./...')
        binary = root / 'consumer-bin'
        run('build','-race','-mod=readonly','-o',str(binary),'.')
        empty = root / 'empty'
        empty.mkdir()
        output = subprocess.run([str(binary)],cwd=empty,env=env,text=True,capture_output=True,check=True).stdout.strip()
        assert output=='PASS: standalone Audit HTTP consumer and module-owned contract',output
        assert all((consumer/name).read_bytes()==data for name,data in before.items())
        released = Path(downloaded['Dir'])
        # Shared comparison: tolerates the root LICENSE Go adds to module zips.
        differences = artifact_differences(local, released, ROOT / 'LICENSE')
        assert not differences, 'Release drift: ' + '; '.join(differences)
        print(output,flush=True)
        print(f'PASS: Audit {VERSION} fresh public download/checksums, exact dependency graph, no replacements, independent HTTP consumer and complete artifact parity',flush=True)
if __name__=='__main__':
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise RuntimeError('Audit release verification failed:\n'+(error.stdout or '')+'\n'+(error.stderr or '')) from None
