#!/usr/bin/env python3
"""Release workflow for the published backend Go modules.

A pull request that changes backend/modules/<id> declares the version it will
become in infra/modules/releases.json. The tooling then:

  check   (every PR)  each module either matches its pinned release exactly, or
                      has a pending release with a higher, untagged version.
  tag     (on main)   tags every pending module at HEAD and pushes the tag.
  pin     (on main)   pins the backend to a tagged version, verifies the
                      published artifact matches the source, regenerates the
                      OpenAPI contract and removes the pending entry.

Run from anywhere; paths are resolved from the repository root.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parent))
from backend_module_pins import MODULE_PREFIX, module_pins  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'backend'
MODULES = BACKEND / 'modules'
RELEASES = ROOT / 'infra/modules/releases.json'
SEMVER = re.compile(r'^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$')
# Base release of a pseudo-version such as v0.1.1-0.20261120164120-d9bb6f8c819f
PSEUDO_BASE = re.compile(r'^v(\d+)\.(\d+)\.(\d+)-(?:0\.)?\d{14}-[0-9a-f]{12}$')


class ReleaseError(Exception):
    pass


def tag_name(module_id, version):
    return f'backend/modules/{module_id}/{version}'


def module_ids(modules=MODULES):
    return sorted(path.parent.name for path in modules.glob('*/go.mod'))


def load_pending(path=RELEASES, known=None):
    """Return {module_id: version} for declared pending releases."""
    if not path.exists():
        return {}
    data = json.loads(path.read_text())
    releases = data.get('releases', {}) if isinstance(data, dict) else None
    if not isinstance(releases, dict):
        raise ReleaseError(f'{path.relative_to(ROOT)} must contain {{"releases": {{...}}}}')
    pending = {}
    for module_id, entry in releases.items():
        if known is not None and module_id not in known:
            raise ReleaseError(f'pending release for unknown module {module_id!r}')
        version = entry.get('version') if isinstance(entry, dict) else None
        if not isinstance(version, str) or not SEMVER.match(version):
            raise ReleaseError(f'{module_id}: version must look like v1.2.3, got {version!r}')
        if not isinstance(entry.get('notes'), str) or not entry['notes'].strip():
            raise ReleaseError(f'{module_id}: release notes are required')
        pending[module_id] = version
    return pending


def write_pending_without(module_id, path=RELEASES):
    data = json.loads(path.read_text())
    data['releases'].pop(module_id, None)
    path.write_text(json.dumps(data, indent=2, sort_keys=True) + '\n')


def version_key(version):
    match = SEMVER.match(version) or PSEUDO_BASE.match(version)
    if not match:
        raise ReleaseError(f'unrecognised module version {version!r}')
    key = tuple(int(part) for part in match.groups()[:3])
    if PSEUDO_BASE.match(version):
        # A pseudo-version vX.Y.(Z+1)-... sorts before vX.Y.(Z+1).
        key = (key[0], key[1], key[2] - 1, 1)
    else:
        key = (*key, 0)
    return key


def artifact_differences(local, published, root_license):
    """Files that differ between a module directory and its published zip.

    Go adds the repository root LICENSE to a nested module's zip when the
    module has none, so a LICENSE identical to the root one is ignored on
    either side.
    """
    license_bytes = root_license.read_bytes() if root_license.exists() else None

    def inventory(base):
        files = {}
        for path in base.rglob('*'):
            if path.is_file() and '.git' not in path.relative_to(base).parts:
                relative = path.relative_to(base).as_posix()
                data = path.read_bytes()
                if relative == 'LICENSE' and data == license_bytes:
                    continue
                files[relative] = data
        return files

    mine, theirs = inventory(local), inventory(published)
    differences = [f'only in source: {name}' for name in sorted(mine.keys() - theirs.keys())]
    differences += [f'only in release: {name}' for name in sorted(theirs.keys() - mine.keys())]
    differences += [f'changed: {name}' for name in sorted(mine.keys() & theirs.keys()) if mine[name] != theirs[name]]
    return differences


def go_env(cache):
    env = {**os.environ, 'GOWORK': 'off', 'GOFLAGS': '-mod=mod', 'GOMODCACHE': str(cache),
           'GOSUMDB': 'sum.golang.org', 'GONOSUMDB': '', 'GOPRIVATE': '', 'GO111MODULE': 'on'}
    env.setdefault('GOTOOLCHAIN', 'local')
    return env


def download(module_path, version, cache, attempts=1):
    go = os.environ.get('GO_CMD', 'go')
    last = None
    for attempt in range(attempts):
        result = subprocess.run([go, 'mod', 'download', '-json', f'{module_path}@{version}'], cwd=tempfile.gettempdir(),
                                env=go_env(cache), text=True, capture_output=True)
        if result.returncode == 0:
            return Path(json.loads(result.stdout)['Dir'])
        last = result.stdout + result.stderr
        if attempt + 1 < attempts:
            time.sleep(min(60, 10 * (attempt + 1)))
    raise ReleaseError(f'cannot download {module_path}@{version}: {last.strip()}')


def git(*args, check=True):
    return subprocess.run(['git', *args], cwd=ROOT, text=True, capture_output=True, check=check)


def remote():
    return os.environ.get('MODULE_RELEASE_REMOTE', 'origin')


def tag_exists(name):
    if git('rev-parse', '-q', '--verify', f'refs/tags/{name}', check=False).returncode == 0:
        return True
    listed = git('ls-remote', '--tags', remote(), f'refs/tags/{name}', check=False)
    if listed.returncode != 0:
        raise ReleaseError(f'cannot list tags on {remote()}: {listed.stderr.strip()}')
    return bool(listed.stdout.strip())


def module_status(module_id, pins, pending, cache):
    module_path = MODULE_PREFIX + module_id
    pinned = pins.get(module_path)
    differences = None
    if pinned:
        differences = artifact_differences(MODULES / module_id, download(module_path, pinned, cache), ROOT / 'LICENSE')
    return pinned, pending.get(module_id), differences


def check(only=None):
    ids = module_ids()
    pending = load_pending(known=set(ids))
    pins = module_pins(BACKEND)
    failures = []
    with tempfile.TemporaryDirectory(prefix='module-release-check-') as cache:
        for module_id in ids:
            if only and module_id != only:
                continue
            pinned, version, differences = module_status(module_id, pins, pending, cache)
            if version is None:
                if pinned is None:
                    failures.append(f'{module_id}: not pinned in backend/go.mod and no pending release declared')
                elif differences:
                    shown = '; '.join(differences[:5]) + (' …' if len(differences) > 5 else '')
                    failures.append(
                        f'{module_id}: source differs from the pinned {pinned} ({shown}). Declare the release this '
                        f'change becomes in infra/modules/releases.json, for example '
                        f'{{"releases": {{"{module_id}": {{"version": "vX.Y.Z", "notes": "..."}}}}}}')
                else:
                    print(f'OK {module_id}: matches pinned {pinned}')
                continue
            if pinned and version_key(version) <= version_key(pinned):
                failures.append(f'{module_id}: pending {version} must be newer than the pinned {pinned}')
            elif tag_exists(tag_name(module_id, version)):
                failures.append(f'{module_id}: tag {tag_name(module_id, version)} already exists; choose a new version')
            elif pinned and not differences:
                failures.append(f'{module_id}: pending {version} declared but the source still matches {pinned}')
            else:
                print(f'OK {module_id}: pending release {version} (pinned {pinned or "none"}); tagged after merge')
    if failures:
        raise ReleaseError('\n'.join(failures))


def skip_if_pending(module_path):
    """For per-module release verifiers: a module awaiting release is verified
    by `module_release.py check` now and by `tag`/`pin` after merge."""
    module_id = module_path.removeprefix(MODULE_PREFIX)
    version = load_pending().get(module_id)
    if version:
        print(f'SKIP: {module_id} has pending release {version}; the pinned release is not expected to match the source')
        return True
    return False


def tag(push):
    if git('status', '--porcelain', '--untracked-files=no', '--', 'backend/modules').stdout.strip():
        raise ReleaseError('backend/modules has uncommitted changes; tags must point at committed source')
    ids = module_ids()
    pending = load_pending(known=set(ids))
    tagged = []
    for module_id, version in sorted(pending.items()):
        name = tag_name(module_id, version)
        if tag_exists(name):
            git('fetch', '-q', remote(), f'refs/tags/{name}:refs/tags/{name}', check=False)
            tagged_tree = git('rev-parse', f'{name}^{{commit}}:backend/modules/{module_id}', check=False).stdout.strip()
            head_tree = git('rev-parse', f'HEAD:backend/modules/{module_id}').stdout.strip()
            if tagged_tree != head_tree:
                raise ReleaseError(f'{name} exists but does not match the module source at HEAD')
            print(f'{name} already tagged at this source')
        else:
            git('tag', '-a', name, '-m', f'{module_id} module {version}')
            if push:
                git('push', remote(), f'refs/tags/{name}')
            print(f'tagged {name}' + (' and pushed' if push else ' (not pushed)'))
        tagged.append({'module': module_id, 'version': version})
    return tagged


def pin(module_id):
    pending = load_pending(known=set(module_ids()))
    version = pending.get(module_id)
    if not version:
        raise ReleaseError(f'{module_id} has no pending release')
    module_path = MODULE_PREFIX + module_id
    go = os.environ.get('GO_CMD', 'go')
    with tempfile.TemporaryDirectory(prefix='module-release-pin-') as cache:
        # The proxy may need a moment to see a freshly pushed tag.
        published = download(module_path, version, cache, attempts=6)
        differences = artifact_differences(MODULES / module_id, published, ROOT / 'LICENSE')
        if differences:
            raise ReleaseError(f'published {module_path}@{version} differs from the source: {differences[:5]}')
    env = {**os.environ, 'GOWORK': 'off'}
    subprocess.run([go, 'get', f'{module_path}@{version}'], cwd=BACKEND, env=env, check=True)
    subprocess.run([go, 'run', './cmd/cli', 'openapi', 'generate'], cwd=BACKEND, env=env, check=True)
    write_pending_without(module_id)
    print(f'pinned {module_path} {version}')


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    commands = parser.add_subparsers(dest='command', required=True)
    check_parser = commands.add_parser('check', help='verify every module is released or has a valid pending release')
    check_parser.add_argument('--module')
    tag_parser = commands.add_parser('tag', help='tag pending modules at HEAD')
    tag_parser.add_argument('--push', action='store_true')
    tag_parser.add_argument('--output', help='write the tagged modules as JSON to this file')
    pin_parser = commands.add_parser('pin', help='pin the backend to a tagged pending release')
    pin_parser.add_argument('--module', required=True)
    args = parser.parse_args(argv)
    try:
        if args.command == 'check':
            check(args.module)
        elif args.command == 'tag':
            tagged = tag(args.push)
            if args.output:
                Path(args.output).write_text(json.dumps(tagged))
        else:
            pin(args.module)
    except (ReleaseError, subprocess.CalledProcessError) as error:
        print(f'FAIL: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
