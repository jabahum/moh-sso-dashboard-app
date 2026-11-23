#!/usr/bin/env python3
"""Check a backend/core release before and after tagging.

  test-core-release.py --version v0.1.0 --source local
      Before tagging. Checks the version, go.mod, changelog and a clean tree,
      runs the standard-library-only core check, then builds and runs the
      independent consumer (identitysync/testdata/consumer) in a scratch module
      outside the repository against the local tree, offline. Prints the tag
      and push commands; it never creates or pushes a tag.

  test-core-release.py --version v0.1.0 --source proxy
      After the tag is pushed. Downloads the version through the Go module
      proxy into a fresh cache, checks the checksum database accepted it,
      compares every published file with the tagged tree, and builds and runs
      the same consumer against the published version with no replacement.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
CORE = ROOT / 'backend/core'
MODULE = 'github.com/mohuganda/moh-sso-dashboard/backend/core'
TAG_PREFIX = 'backend/core/'
CONSUMER = 'example.com/core-release-consumer'
CONSUMER_SOURCE = CORE / 'identitysync/testdata/consumer/main.go'
EXPECTED_OUTPUT = 'PASS: independent consumer served contract'


def fail(message):
    raise SystemExit('FAIL: ' + message)


def run(args, cwd, env=None, check=True):
    result = subprocess.run(args, cwd=cwd, env=env, text=True, capture_output=True)
    if check and result.returncode != 0:
        fail(f"{' '.join(args)} (in {cwd}) exited {result.returncode}:\n{result.stdout}{result.stderr}")
    return result


def go_env(cache, offline):
    env = {**os.environ, 'GOWORK': 'off', 'GOTOOLCHAIN': 'local', 'GOMODCACHE': str(cache), 'GOFLAGS': ''}
    if offline:
        env.update({'GOPROXY': 'off', 'GOSUMDB': 'off'})
    else:
        env.update({'GOPROXY': 'https://proxy.golang.org', 'GOSUMDB': 'sum.golang.org', 'GONOSUMDB': '', 'GOPRIVATE': ''})
    return env


def check_metadata(version):
    if not re.fullmatch(r'v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)', version):
        fail(f'version {version!r} must look like v0.1.0')
    gomod = (CORE / 'go.mod').read_text()
    if not re.search(r'^module ' + re.escape(MODULE) + r'$', gomod, re.M):
        fail(f'backend/core/go.mod must declare module {MODULE}')
    if re.search(r'^\s*require\b', gomod, re.M):
        fail('backend/core/go.mod must have no require lines (standard library only)')
    changelog = (CORE / 'CHANGELOG.md').read_text()
    if not re.search(r'^## ' + re.escape(version) + r'\b', changelog, re.M):
        fail(f'backend/core/CHANGELOG.md has no "## {version}" section')
    print(f'PASS: version {version}, module path, no requirements, changelog section')


def check_local(version):
    check_metadata(version)
    tag = TAG_PREFIX + version
    if run(['git', 'tag', '-l', tag], ROOT).stdout.strip():
        fail(f'tag {tag} already exists')
    status = run(['git', 'status', '--porcelain', '--untracked-files=all', '--', 'backend/core'], ROOT).stdout.strip()
    if status:
        fail('backend/core has uncommitted or untracked files; only committed files are published:\n' + status)
    unformatted = run(['gofmt', '-l', '.'], CORE).stdout.strip()
    if unformatted:
        fail('gofmt reports unformatted files:\n' + unformatted)
    core_check = run([sys.executable, str(ROOT / 'infra/scripts/test-platform-core.py')], ROOT)
    print(core_check.stdout.strip().splitlines()[-1])
    with tempfile.TemporaryDirectory(prefix='moh-core-release-') as directory:
        root = Path(directory)
        consumer = write_consumer(root, f'v0.0.0-00010101000000-000000000000\nreplace {MODULE} => {CORE}\n')
        env = go_env(root / 'cache', offline=True)
        output = build_and_run(consumer, root, env, readonly=False)
    print(output)
    print(f'PASS: backend/core {version} is ready to tag. After the change is merged, on the merged commit:')
    print(f'  git tag -a {tag} -m "backend/core {version}"')
    print(f'  git push origin {tag}')
    print(f'  python3 infra/scripts/test-core-release.py --version {version} --source proxy')


def write_consumer(root, requirement):
    consumer = root / 'consumer'
    consumer.mkdir()
    go_line = re.search(r'^go (\S+)$', (CORE / 'go.mod').read_text(), re.M).group(1)
    (consumer / 'go.mod').write_text(f'module {CONSUMER}\n\ngo {go_line}\n\nrequire {MODULE} {requirement}')
    (consumer / 'main.go').write_text(CONSUMER_SOURCE.read_text())
    return consumer


def build_and_run(consumer, root, env, readonly):
    mod = ['-mod=readonly'] if readonly else ['-mod=mod']
    run(['go', 'vet', *mod, '.'], consumer, env)
    binary = root / 'consumer-bin'
    run(['go', 'build', '-race', *mod, '-o', str(binary), '.'], consumer, env)
    empty = root / 'empty'
    empty.mkdir()
    output = run([str(binary)], empty, env).stdout.strip()
    if not output.startswith(EXPECTED_OUTPUT):
        fail('consumer output: ' + output)
    return output


def check_proxy(version):
    check_metadata(version)
    tag = TAG_PREFIX + version
    if not run(['git', 'tag', '-l', tag], ROOT).stdout.strip():
        fail(f'tag {tag} does not exist locally; fetch tags first')
    with tempfile.TemporaryDirectory(prefix='moh-core-release-') as directory:
        root = Path(directory)
        env = go_env(root / 'cache', offline=False)
        consumer = write_consumer(root, f'{version}\n')
        downloaded = json.loads(run(['go', 'mod', 'download', '-json', f'{MODULE}@{version}'], consumer, env).stdout)
        if downloaded.get('Error') or not downloaded.get('Sum'):
            fail(f'download failed: {downloaded.get("Error")}')
        run(['go', 'mod', 'tidy'], consumer, env)
        selected = json.loads(run(['go', 'list', '-mod=readonly', '-m', '-json', MODULE], consumer, env).stdout)
        if selected.get('Version') != version or selected.get('Replace'):
            fail(f'consumer selected {selected}')
        graph = run(['go', 'list', '-mod=readonly', '-m', 'all'], consumer, env).stdout.split('\n')
        if sorted(line.split()[0] for line in graph if line.strip()) != sorted([CONSUMER, MODULE]):
            fail('unexpected dependency graph: ' + ', '.join(graph))
        run(['go', 'mod', 'verify'], consumer, env)
        # Every published file equals the tagged tree.
        released = Path(downloaded['Dir'])
        tagged = set(run(['git', 'ls-tree', '-r', '--name-only', tag, '--', 'backend/core'], ROOT).stdout.split())
        tagged = {p.removeprefix('backend/core/') for p in tagged}
        published = {str(p.relative_to(released)) for p in released.rglob('*') if p.is_file()}
        if tagged != published:
            fail(f'published files differ from {tag}: {sorted(tagged ^ published)}')
        for relative in sorted(tagged):
            content = subprocess.run(['git', 'show', f'{tag}:backend/core/{relative}'], cwd=ROOT, capture_output=True, check=True).stdout
            if content != (released / relative).read_bytes():
                fail(f'published {relative} differs from {tag}')
        output = build_and_run(consumer, root, env, readonly=True)
    print(output)
    print(f'PASS: backend/core {version} downloads from the proxy with a verified checksum, matches {tag} file for file, '
          'and an independent consumer builds and passes with no replacement')


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--version', required=True)
    parser.add_argument('--source', choices=['local', 'proxy'], required=True)
    args = parser.parse_args()
    if args.source == 'local':
        check_local(args.version)
    else:
        check_proxy(args.version)


if __name__ == '__main__':
    main()
