#!/usr/bin/env python3
"""Generate a credential-free fresh realm; never modify an existing realm."""
import argparse
import copy
import json
from pathlib import Path
import urllib.parse

ROOT = Path(__file__).resolve().parents[2]
REALMS = json.loads((ROOT / 'backend/internal/keycloak/identity-environments.json').read_text())


def render(environment, portal_url, api_url):
    for raw in (portal_url, api_url):
        uri = urllib.parse.urlsplit(raw)
        if not uri.hostname or uri.username or uri.password or uri.query or uri.fragment or '*' in raw:
            raise ValueError('Use an absolute portal/API URL without credentials, query, fragment or wildcard')
        if uri.scheme != 'https' and not (uri.scheme == 'http' and environment in ('development', 'test') and uri.hostname in ('127.0.0.1', 'localhost', '::1')):
            raise ValueError('HTTPS is required except for development/test loopback')
    realm = copy.deepcopy(json.loads((ROOT / 'infra/keycloak/realm-export-prod.json').read_text()))
    realm['realm'] = REALMS[environment]
    realm['displayName'] = 'Ministry of Health — ' + environment.title()
    # Service identities keep their role bindings; no human users or credentials are copied.
    realm['users'] = [u for u in realm.get('users', []) if u.get('serviceAccountClientId')]
    realm['sslRequired'] = 'external' if environment in ('development', 'test') else 'all'
    realm['smtpServer'] = {}
    # Keep only the declarative user profile (it declares the admin-only
    # ihp_identity_sync_origin attribute identity-sync invitations store); drop
    # every other component (keys, LDAP, etc.).
    components = realm.pop('components', {}) or {}
    profile = components.get('org.keycloak.userprofile.UserProfileProvider')
    if profile:
        realm['components'] = {'org.keycloak.userprofile.UserProfileProvider': profile}
    for client in realm['clients']:
        client['enabled'] = client['clientId'] in ('dashboard-web', 'dashboard-admin', 'data-statistics')
        client['redirectUris'] = []
        client['webOrigins'] = []
        client['rootUrl'] = ''
        client['baseUrl'] = ''
        attrs = client.setdefault('attributes', {})
        for key in list(attrs):
            if any(term in key.lower() for term in ('url', 'uri', 'home')):
                attrs.pop(key)
        if client['clientId'] == 'dashboard-web':
            origin = urllib.parse.urlsplit(portal_url)
            client.update(rootUrl=portal_url.rstrip('/'), baseUrl=portal_url.rstrip('/'),
                          redirectUris=[api_url.rstrip('/') + '/auth/callback'],
                          webOrigins=[origin.scheme + '://' + origin.netloc])
            attrs['pkce.code.challenge.method'] = 'S256'
            attrs['post.logout.redirect.uris'] = portal_url.rstrip('/')
    # The external applications are onboarded independently for each environment.
    external = {'digital-timesheets', 'digital-timesheets-test', 'datawarehouse-report-browser'}
    realm['clients'] = [c for c in realm['clients'] if c['clientId'] not in external]
    def scrub(value):
        if isinstance(value, dict):
            return {k: scrub(v) for k, v in value.items() if k not in (
                'id', 'secret', 'credentials', 'privateKey', 'certificate', 'registrationAccessToken')}
        if isinstance(value, list):
            return [scrub(v) for v in value]
        return value
    return scrub(realm)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--environment', required=True, choices=REALMS)
    parser.add_argument('--portal-url', required=True)
    parser.add_argument('--api-url', required=True, help='API base URL including /api/v1')
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    result = render(args.environment, args.portal_url, args.api_url)
    with args.output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print('Prepared fresh realm ' + result['realm'] + '; existing realms are never replaced.')


if __name__ == '__main__':
    main()
