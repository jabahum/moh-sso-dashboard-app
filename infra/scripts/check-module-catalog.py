#!/usr/bin/env python3
"""Validate complete module coverage and keep the generated status guide current."""
import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND_FIELDS = {'id', 'stage', 'scope', 'moduleId', 'dependencies', 'workers', 'migrations', 'next'}
FRONTEND_FIELDS = {'package', 'kind', 'stage', 'backendFeatures'}
STAGES = {'core', 'planned', 'registry-hosted', 'published'}
INFRASTRUCTURE = {'primary-db', 'redis', 'keycloak', 'blob-storage', 'warehouse', 'observatory-db',
                  'host-audit-recorder', 'geojson-assets', 'upload-db', 'smtp', 'sms', 'health-bi'}


def strings(value):
    return isinstance(value, list) and all(isinstance(item, str) and item for item in value) and len(set(value)) == len(value)


def load_catalog(root=ROOT):
    data = json.loads((root / 'docs/module-catalog.json').read_text())
    if set(data) != {'schemaVersion', 'backend', 'frontend'} or data['schemaVersion'] != 1:
        raise ValueError('Unsupported module catalogue schema')
    if not isinstance(data['backend'], list) or not isinstance(data['frontend'], list):
        raise ValueError('Module catalogue entries must be lists')
    backend = {}
    runtime = set()
    for entry in data['backend']:
        if set(entry) != BACKEND_FIELDS or not all(isinstance(entry.get(key), str) and entry[key]
                for key in ('id', 'stage', 'scope', 'migrations', 'next')):
            raise ValueError('Backend entry requires status, scope, migration owner and next work')
        if entry['id'] in backend or entry['stage'] not in STAGES or not strings(entry['workers']) or not strings(entry['dependencies']):
            raise ValueError('Duplicate or invalid backend entry: ' + entry['id'])
        module_id = entry['moduleId']
        if not isinstance(module_id, str) or bool(module_id) != (entry['stage'] in {'registry-hosted', 'published'}):
            raise ValueError('Only registry-integrated features declare runtime module IDs')
        if module_id and module_id in runtime:
            raise ValueError('Duplicate runtime module ID: ' + module_id)
        if module_id:
            runtime.add(module_id)
        backend[entry['id']] = entry
    actual = {path.name for path in (root / 'backend/internal/features').iterdir() if path.is_dir() and not path.name.startswith('.')}
    if actual != set(backend):
        raise ValueError('Backend catalogue drift: missing=' + str(sorted(actual - backend.keys())) + ', stale=' + str(sorted(backend.keys() - actual)))
    for entry in backend.values():
        unknown = set(entry['dependencies']) - backend.keys() - INFRASTRUCTURE
        if unknown:
            raise ValueError('Unknown dependency for ' + entry['id'] + ': ' + str(sorted(unknown)))
    workspaces = {}
    for group, kind in [('apps', 'application'), ('packages', 'shared')]:
        for path in (root / 'frontend' / group).glob('*/package.json'):
            package = json.loads(path.read_text())
            if not package.get('private'):
                workspaces[package['name']] = kind
    frontend = {}
    for entry in data['frontend']:
        if set(entry) != FRONTEND_FIELDS or entry.get('stage') != 'published' or not strings(entry.get('backendFeatures')):
            raise ValueError('Invalid frontend catalogue entry')
        name = entry['package']
        if name in frontend or workspaces.get(name) != entry['kind'] or set(entry['backendFeatures']) - backend.keys():
            raise ValueError('Invalid frontend package/dependency mapping: ' + name)
        frontend[name] = entry
    if set(frontend) != set(workspaces):
        raise ValueError('Frontend catalogue must cover every publishable app and shared package')
    production = json.loads((root / 'frontend/module-sources.json').read_text())['production']
    if production['source'] != 'npm' or set(production['versions']) != set(frontend):
        raise ValueError('Published frontend catalogue must match production npm selection')
    lock = json.loads((root / 'frontend' / production['lockfile']).read_text())
    if lock.get('packages', {}).get('', {}).get('dependencies') != production['versions']:
        raise ValueError('Production module pins and npm lock differ')
    for name, version in production['versions'].items():
        package = lock['packages'].get('node_modules/' + name, {})
        if package.get('version') != version or not package.get('integrity') or package.get('link'):
            raise ValueError('Published package is not locked to a registry artifact: ' + name)
    return data, production['versions']


def render(data, pins):
    def cell(value):
        return str(value).replace('|', '\\|').replace('\n', ' ')
    lines = ['# Module coverage', '',
        'Generated from `docs/module-catalog.json`; update with `python3 infra/scripts/check-module-catalog.py --write`.', '',
        '`core` stays in the trusted host. `planned` still uses legacy wiring. `registry-hosted` uses the compiled registry but ships with the host version. `published` has independently versioned package artifacts.', '',
        'Registry integration does not imply independent Go packaging. Planned domains are tracked expansion work; their workers and migrations have not been transferred to the registry.', '',
        '## Backend', '',
        '| Feature | Stage | Runtime ID | Scope | Dependencies | Workers | Migration owner | Next work |',
        '| --- | --- | --- | --- | --- | --- | --- | --- |']
    for item in sorted(data['backend'], key=lambda entry: entry['id']):
        values = [item['id'], item['stage'], item['moduleId'] or '—', item['scope'],
                  ', '.join(item['dependencies']) or '—', ', '.join(item['workers']) or 'none', item['migrations'], item['next']]
        lines.append('| ' + ' | '.join(cell(value) for value in values) + ' |')
    lines += ['', '## Frontend', '',
        'The shell is built from the checkout. This table covers every publishable application and shared package; versions come from the reviewed production selection.', '',
        '| Package | Kind | Stage | Production version | Backend features |', '| --- | --- | --- | --- | --- |']
    for item in sorted(data['frontend'], key=lambda entry: entry['package']):
        values = [item['package'], item['kind'], item['stage'], pins[item['package']], ', '.join(item['backendFeatures']) or 'frontend/shared or external integration']
        lines.append('| ' + ' | '.join(cell(value) for value in values) + ' |')
    return '\n'.join(lines) + '\n'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--write', action='store_true', help='Regenerate docs/module-coverage.md after reviewing catalogue changes')
    args = parser.parse_args()
    data, pins = load_catalog()
    expected = render(data, pins)
    path = ROOT / 'docs/module-coverage.md'
    if args.write:
        path.write_text(expected)
    elif not path.is_file() or path.read_text() != expected:
        raise SystemExit('Module coverage guide is stale; run check-module-catalog.py --write')
    print(f'Module coverage passed: {len(data["backend"])} backend features, {len(data["frontend"])} frontend packages')


if __name__ == '__main__':
    main()
