#!/usr/bin/env python3
"""Exercise client reconciliation and real OIDC flows in disposable Keycloak."""
import base64
import hashlib
import html
import http.cookiejar
import importlib.util
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

spec = importlib.util.spec_from_file_location(
    'timesheets', Path(__file__).with_name('reconcile-digital-timesheets.py'))
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(url, data=None, headers=None, opener=None, method=None):
    req = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
    try:
        response = opener.open(req, timeout=10) if opener else urllib.request.urlopen(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.code, response.headers, response.read().decode()


def form(values):
    return urllib.parse.urlencode(values).encode()


def jwt_payload(token):
    part = token.split('.')[1]
    return json.loads(base64.urlsafe_b64decode(part + '=' * (-len(part) % 4)))


def run():
    container = 'timesheets-oidc-test-' + secrets.token_hex(5)
    password = 'Fixture-Aa1!' + secrets.token_urlsafe(24)
    subprocess.run(['docker', 'run', '--detach', '--name', container,
                    '-p', '127.0.0.1::8080',
                    '-e', 'KC_BOOTSTRAP_ADMIN_USERNAME=fixture-admin',
                    '-e', 'KC_BOOTSTRAP_ADMIN_PASSWORD=' + password,
                    'quay.io/keycloak/keycloak:26.0.5', 'start-dev'],
                   check=True, stdout=subprocess.DEVNULL)
    try:
        port = subprocess.check_output(['docker', 'port', container, '8080/tcp'], text=True).strip().rsplit(':', 1)[1]
        base = 'http://127.0.0.1:' + port
        deadline = time.monotonic() + 100
        while True:
            try:
                if request(base + '/realms/master')[0] == 200:
                    break
            except (OSError, urllib.error.URLError):
                pass
            if time.monotonic() > deadline:
                raise RuntimeError('Disposable Keycloak did not become ready')
            time.sleep(1)

        def admin(method, path, body=None):
            status, _, raw = request(base + '/realms/master/protocol/openid-connect/token', form({
                'grant_type': 'password', 'client_id': 'admin-cli',
                'username': 'fixture-admin', 'password': password}))
            assert status == 200, 'Fixture administrator authentication failed'
            token = json.loads(raw)['access_token']
            status, _, raw = request(base + '/admin/' + path,
                                    None if body is None else json.dumps(body).encode(),
                                    {'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'},
                                    method=method)
            assert status < 300, 'Fixture admin request failed: %s %s (%s)' % (method, path, status)
            return json.loads(raw) if raw else None

        bootstrap_spec = importlib.util.spec_from_file_location(
            'bootstrap', Path(__file__).with_name('prepare-keycloak-realm.py'))
        bootstrap = importlib.util.module_from_spec(bootstrap_spec)
        bootstrap_spec.loader.exec_module(bootstrap)
        # Mimic the user's single live instance: production already exists.
        admin('POST', 'realms', {'realm': 'moh-realm', 'enabled': True, 'sslRequired': 'none',
                                 'accessTokenLifespan': 300})
        admin('POST', 'realms/moh-realm/users', {'username': 'production-sentinel', 'enabled': False})
        production_before = admin('GET', 'realms/moh-realm')
        production_users = admin('GET', 'realms/moh-realm/users')
        shared_spec = importlib.util.spec_from_file_location(
            'shared_bootstrap', Path(__file__).with_name('bootstrap-keycloak-realms.py'))
        shared = importlib.util.module_from_spec(shared_spec)
        shared_spec.loader.exec_module(shared)
        operator = shared.ServerAdmin({'KEYCLOAK_BASE_URL': base,
                                       'KEYCLOAK_BOOTSTRAP_USERNAME': 'fixture-admin',
                                       'KEYCLOAK_BOOTSTRAP_PASSWORD': password})
        environments = {name: {'portalUrl': 'https://portal.example.invalid',
                               'apiUrl': 'https://portal.example.invalid/api/v1'}
                        for name in ('development', 'test', 'staging')}
        plan = shared.build_plan(operator, environments)
        assert all(row['action'] == 'create' for row in plan)
        with tempfile.TemporaryDirectory() as directory:
            shared.apply_plan(operator, plan, Path(directory) / 'created.json')
            repeat = shared.build_plan(operator, environments)
            assert all(row['action'] == 'skip-existing' for row in repeat)
            shared.apply_plan(operator, repeat, Path(directory) / 'repeat.json')
        assert admin('GET', 'realms/moh-realm') == production_before, 'Production realm configuration changed'
        assert admin('GET', 'realms/moh-realm/users') == production_users, 'Production users changed'
        key_ids = set()
        for environment in environments:
            realm_name = bootstrap.REALMS[environment]
            users = admin('GET', 'realms/' + realm_name + '/users')
            assert all(u.get('serviceAccountClientId') for u in users), 'Human users copied between environments'
            keys = admin('GET', 'realms/' + realm_name + '/keys')['keys']
            ids = {key['kid'] for key in keys}
            assert ids and not (ids & key_ids), 'Signing keys were shared between realms'
            key_ids.update(ids)
        print('PASS: shared-instance dev/test/staging creation, repeat skip, independent keys and untouched production', flush=True)

        class API:
            realm = 'moh-realm'

            def request(self, method, path, body=None):
                return admin(method, 'realms/' + self.realm + path, body)

            def get(self, path):
                return self.request('GET', path)

        admin('POST', 'realms/moh-realm/clients', {
            'clientId': 'fixture-provisioner', 'enabled': True, 'publicClient': False,
            'serviceAccountsEnabled': True, 'secret': password})
        provisioner = admin('GET', 'realms/moh-realm/clients?clientId=fixture-provisioner')[0]
        service_user = admin('GET', 'realms/moh-realm/clients/' + provisioner['id'] + '/service-account-user')
        management = admin('GET', 'realms/moh-realm/clients?clientId=realm-management')[0]
        role = admin('GET', 'realms/moh-realm/clients/' + management['id'] + '/roles/realm-admin')
        admin('POST', 'realms/moh-realm/users/' + service_user['id'] + '/role-mappings/clients/' + management['id'], [role])
        subprocess.run(['go', 'test', './internal/keycloak', '-run', '^TestOnboardingKeycloakRuntime$', '-count=1'],
                       cwd=setup.ROOT / 'backend', check=True,
                       env={**os.environ, 'ONBOARDING_KEYCLOAK_URL': base, 'ONBOARDING_KEYCLOAK_SECRET': password})
        print('PASS: Go provisioning create/update verification for public, confidential and service profiles', flush=True)

        api = API()
        manifests = [json.loads(setup.MANIFEST.read_text()),
                     json.loads(setup.MANIFEST.with_name('digital-timesheets-test.json').read_text())]
        for manifest in manifests:
            api.realm = manifest['realm']
            with tempfile.TemporaryDirectory() as directory:
                setup.apply_plan(api, setup.build_plan(api, manifest, True), Path(directory) / 'before.json')
                plan = setup.summary(setup.build_plan(api, manifest, True))
                assert not plan['clientsToCreate'] and not plan['clientsToUpdate'] and not plan['testUsersToCreate']
            for verified in (True, False):
                api.request('POST', '/users', {
                    'username': 'fixture-' + str(verified), 'enabled': True,
                    'firstName': 'Fixture', 'lastName': 'User',
                    'email': 'fixture-' + str(verified).lower() + '@example.invalid',
                    'emailVerified': verified,
                    'credentials': [{'type': 'password', 'value': password, 'temporary': False}]})
        print('PASS: client reconciliation, exact scopes, disabled test users and idempotence', flush=True)

        issuer = base + '/realms/moh-realm'
        oidc = issuer + '/protocol/openid-connect'
        verifier = secrets.token_urlsafe(48)
        challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).decode().rstrip('=')
        no_redirect = urllib.request.build_opener(NoRedirect())

        def auth(client, **overrides):
            values = {'client_id': client['clientId'], 'response_type': 'code', 'scope': 'openid profile email',
                      'redirect_uri': client['redirectUris'][0], 'code_challenge': challenge,
                      'code_challenge_method': 'S256', **overrides}
            return oidc + '/auth?' + urllib.parse.urlencode(values)

        def exchange(client, code, proof=verifier):
            return request(oidc + '/token', form({
                'grant_type': 'authorization_code', 'client_id': client['clientId'],
                'redirect_uri': client['redirectUris'][0], 'code': code, 'code_verifier': proof}),
                {'Origin': client['webOrigins'][0]})

        def login(client, verified):
            # Keycloak treats loopback as a secure context and sets Secure cookies.
            # Match browser loopback behavior in this isolated HTTP fixture only.
            cookies = http.cookiejar.CookieJar(http.cookiejar.DefaultCookiePolicy(
                secure_protocols=('https', 'wss', 'http')))
            browser = urllib.request.build_opener(
                urllib.request.HTTPCookieProcessor(cookies), NoRedirect())
            status, _, page = request(auth(client), opener=browser)
            assert status == 200, 'Authorization login form missing'
            action = html.unescape(re.search(r'<form[^>]+action="([^"]+)"', page).group(1))
            status, headers, page = request(action, form({'username': 'fixture-' + str(verified),
                                                         'password': password}), opener=browser)
            errors = re.findall(r'(?:id="input-error"|class="kc-feedback-text")[^>]*>(.*?)<', page, re.S)
            title = re.search(r'<title>(.*?)</title>', page, re.S)
            if status != 302:
                visible = re.sub(r'<(?:script|style)\b[^>]*>.*?</(?:script|style)>', '', page, flags=re.S)
                errors.append(' '.join(html.unescape(re.sub(r'<[^>]+>', ' ', visible)).split())[:700])
            assert status == 302, 'Fixture browser login failed (%s): %s %s' % (
                status, title.group(1).strip() if title else '', ', '.join(errors).strip())
            location = headers['Location']
            assert location.startswith(client['redirectUris'][0] + '?')
            code = urllib.parse.parse_qs(urllib.parse.urlsplit(location).query)['code'][0]
            status, headers, raw = exchange(client, code)
            assert status == 200, 'Code exchange failed'
            assert headers['Access-Control-Allow-Origin'] == client['webOrigins'][0]
            tokens = json.loads(raw)
            claims = jwt_payload(tokens['access_token'])
            header = json.loads(base64.urlsafe_b64decode(tokens['access_token'].split('.')[0] + '=='))
            assert header['alg'] == 'RS256'
            assert claims['iss'] == issuer and claims['azp'] == client['clientId']
            assert claims['sub'] and claims['email_verified'] is verified and claims['email'].endswith('@example.invalid')
            assert claims['typ'] == 'Bearer' and claims['exp'] - claims['iat'] == 300
            assert 'realm_access' not in claims and 'resource_access' not in claims
            assert request(oidc + '/userinfo', headers={'Authorization': 'Bearer ' + tokens['access_token']})[0] == 200
            assert exchange(client, code)[0] == 400, 'Authorization code was reusable'
            return browser, tokens

        for manifest in manifests:
            issuer = base + "/realms/" + manifest["realm"]
            oidc = issuer + "/protocol/openid-connect"
            for client in manifest['clients']:
                status, headers, _ = request(auth(client, code_challenge_method='plain'), opener=no_redirect)
                assert status == 400 or (status == 302 and 'error=' in headers['Location']), 'Plain PKCE was allowed'
                status, headers, _ = request(auth(client, redirect_uri=client['redirectUris'][0] + '/invalid'), opener=no_redirect)
                assert status == 400 and 'Location' not in headers, 'Unregistered redirect was allowed'
                status, headers, _ = request(auth(client, code_challenge='', code_challenge_method=''), opener=no_redirect)
                assert status == 400 or (status == 302 and 'error=' in headers['Location']), 'Missing PKCE was allowed'
                for grant in ('password', 'client_credentials'):
                    assert request(oidc + '/token', form({'client_id': client['clientId'], 'grant_type': grant,
                                                         'username': 'fixture-True', 'password': password}))[0] >= 400
                browser, tokens = login(client, True)
                status, headers, _ = request(auth(client, prompt='none'), opener=browser)
                assert status == 302 and 'code=' in headers['Location'], 'Realm SSO session was unavailable'
                code = urllib.parse.parse_qs(urllib.parse.urlsplit(headers['Location']).query)['code'][0]
                assert exchange(client, code, secrets.token_urlsafe(48))[0] == 400, 'Incorrect PKCE verifier was allowed'
                logout = oidc + '/logout?' + urllib.parse.urlencode({
                    'id_token_hint': tokens['id_token'],
                    'post_logout_redirect_uri': client['attributes']['post.logout.redirect.uris']})
                status, headers, _ = request(logout, opener=browser)
                assert status == 302 and headers['Location'] == client['attributes']['post.logout.redirect.uris']
                status, headers, _ = request(auth(client, prompt='none'), opener=browser)
                assert status == 302 and 'error=login_required' in headers['Location'], 'Logout retained the SSO session'
                login(client, False)
                print('PASS: %s S256/code flow, claims, email negative case, CORS, SSO, logout and grant restrictions' % client['clientId'], flush=True)
    finally:
        subprocess.run(['docker', 'rm', '--force', container], check=True, stdout=subprocess.DEVNULL)


if __name__ == '__main__':
    run()
