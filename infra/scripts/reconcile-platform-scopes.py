#!/usr/bin/env python3
"""Create the platform API's capability scopes in Keycloak from the catalogue.

Reads backend/internal/features/platform/scopes.json and makes sure each scope
exists as an OpenID Connect client scope with "Include in token scope" on and
"Display on consent screen" off. With --client it also attaches the given
scopes to that machine client as optional client scopes, so the client
receives them only when it requests them.

Dry-run by default; --apply writes. It never deletes or renames client scopes,
and only changes the protocol and those two attributes on existing ones.
Credentials come from the existing KEYCLOAK_* environment (see
reconcile-keycloak-user-defaults.py). No credentials are printed.
"""
import argparse
import json
import os
from pathlib import Path

from keycloak_client_provisioning import ROOT, Keycloak, quote

CATALOGUE = ROOT / 'backend/internal/features/platform/scopes.json'
WANTED_ATTRIBUTES = {'include.in.token.scope': 'true', 'display.on.consent.screen': 'false'}


def load_catalogue(path=CATALOGUE):
    data = json.loads(Path(path).read_text())
    if data.get('version') != 1 or not data.get('scopes'):
        raise ValueError('Unsupported platform scope catalogue')
    return data['scopes']


def build_plan(api, catalogue, client_id=None, grants=()):
    """Return the writes needed, as (action, name, method, path, body) tuples."""
    known = {scope['name']: scope for scope in catalogue}
    unknown = [name for name in grants if name not in known or 'machine' not in known[name]['tokens']]
    if unknown:
        raise ValueError('Not machine-grantable platform scopes: ' + ', '.join(unknown))
    existing = {scope['name']: scope for scope in api.get('/client-scopes')}
    plan = []
    for scope in catalogue:
        name = scope['name']
        current = existing.get(name)
        if current is None:
            plan.append(('create scope', name, 'POST', '/client-scopes', {
                'name': name, 'description': scope['description'], 'protocol': 'openid-connect',
                'attributes': dict(WANTED_ATTRIBUTES),
            }))
            continue
        attributes = dict(current.get('attributes') or {})
        if current.get('protocol') != 'openid-connect' or any(attributes.get(k) != v for k, v in WANTED_ATTRIBUTES.items()):
            attributes.update(WANTED_ATTRIBUTES)
            updated = dict(current, protocol='openid-connect', attributes=attributes)
            plan.append(('update scope', name, 'PUT', '/client-scopes/' + quote(current['id']), updated))
    if client_id:
        clients = api.get('/clients?clientId=' + quote(client_id))
        if len(clients) != 1:
            raise ValueError('Client not found: ' + client_id)
        client = clients[0]
        if not client.get('serviceAccountsEnabled'):
            raise ValueError('Platform scopes are granted to service-account clients only')
        path = '/clients/' + quote(client['id'])
        assigned = {s['name'] for s in api.get(path + '/optional-client-scopes')}
        assigned |= {s['name'] for s in api.get(path + '/default-client-scopes')}
        for name in grants:
            if name not in assigned:
                # The scope's ID is only known after it exists; resolve it at apply time.
                plan.append(('attach optional scope', name, 'PUT', path + '/optional-client-scopes/{%s}' % name, None))
    return plan


def apply_plan(api, plan):
    ids = {}
    for action, name, method, path, body in plan:
        if '{' in path:
            if not ids:
                ids = {scope['name']: scope['id'] for scope in api.get('/client-scopes')}
            path = path.replace('{%s}' % name, quote(ids[name]))
        api.request(method, path, body)
        if action == 'create scope':
            ids = {}


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--catalogue', type=Path, default=CATALOGUE)
    parser.add_argument('--client', help='Machine client ID to attach scopes to, for example alerts-platform')
    parser.add_argument('--scopes', default='', help='Comma-separated scopes to attach to --client')
    parser.add_argument('--apply', action='store_true', help='Write the changes (default: print the plan)')
    args = parser.parse_args()
    grants = [s.strip() for s in args.scopes.split(',') if s.strip()]
    if bool(args.client) != bool(grants):
        parser.error('--client and --scopes go together')
    api = Keycloak(os.environ)
    plan = build_plan(api, load_catalogue(args.catalogue), args.client, grants)
    if not plan:
        print('Platform scopes are up to date.')
        return
    for action, name, *_ in plan:
        print('%s: %s' % (action, name))
    if args.apply:
        apply_plan(api, plan)
        print('Applied %d change(s).' % len(plan))
    else:
        print('Dry run: %d change(s). Re-run with --apply to write them.' % len(plan))


if __name__ == '__main__':
    main()
