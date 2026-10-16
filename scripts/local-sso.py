#!/usr/bin/env python3
"""Local SSO + Health BI lifecycle. Never contacts a non-loopback Keycloak."""
import argparse
import json
import os
from pathlib import Path
import secrets
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
STATE = ROOT / '.local-sso'
BROWSER_CLIENT = 'datawarehouse-report-browser'
WORKER_CLIENT = 'report-scheduler-worker'
ROLE_CLIENT = 'data-statistics'
SERVICE_ROLE = 'report_scheduler_service'


def run(args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def capture(args):
    return subprocess.check_output(args, text=True)


def private_write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    # Open with restrictive mode before writing credentials, including first run.
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    os.fchmod(descriptor, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        output.write(content)


def env_values(path):
    if not path.exists():
        return {}
    return dict(line.split('=', 1) for line in path.read_text().splitlines()
                if line and not line.lstrip().startswith('#') and '=' in line)


def update_env(path, values):
    # Preserve unrelated developer settings and comments. Replace only our keys.
    lines = path.read_text().splitlines() if path.exists() else []
    result = []
    written = set()
    for line in lines:
        key = line.partition('=')[0].strip()
        if key in values:
            if key not in written:
                result.append(key + '=' + values[key])
                written.add(key)
        else:
            result.append(line)
    result.extend(key + '=' + value for key, value in values.items() if key not in written)
    private_write(path, '\n'.join(result) + '\n')


def http_json(url, method='GET', data=None, token=None, form=None, missing=False):
    headers = {'Accept': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if form is not None:
        body = urllib.parse.urlencode(form).encode()
        headers['Content-Type'] = 'application/x-www-form-urlencoded'
    elif data is not None:
        body = json.dumps(data).encode()
        headers['Content-Type'] = 'application/json'
    else:
        body = None
    try:
        with urllib.request.urlopen(urllib.request.Request(url, body, headers, method=method), timeout=10) as response:
            payload = response.read()
            return json.loads(payload) if payload else None
    except urllib.error.HTTPError as error:
        if missing and error.code == 404:
            return None
        # Keycloak responses can contain credential/request details. Never log them.
        raise RuntimeError(f'{method} {urllib.parse.urlsplit(url).path} returned HTTP {error.code}') from None


def http_ready(url):
    with urllib.request.urlopen(url, timeout=10) as response:
        return response.status == 200


def wait_for(check, description, seconds=180):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            if check():
                return
        except (OSError, ValueError, RuntimeError):
            pass
        time.sleep(2)
    raise RuntimeError('Timed out waiting for ' + description)


class Keycloak:
    def __init__(self, base, realm, token):
        parsed = urllib.parse.urlsplit(base)
        if parsed.hostname not in ('localhost', '127.0.0.1', '::1'):
            raise ValueError('Local bootstrap only permits a loopback Keycloak URL')
        self.base = base.rstrip('/')
        self.realm = realm
        self.token = token

    def call(self, method, path, data=None, missing=False):
        return http_json(self.base + '/admin/realms/' + urllib.parse.quote(self.realm, safe='') + path,
                         method, data, self.token, missing=missing)

    def client(self, name):
        clients = self.call('GET', '/clients?' + urllib.parse.urlencode({'clientId': name}))
        return next((client for client in clients if client['clientId'] == name), None)

    def ensure_client(self, desired):
        current = self.client(desired['clientId'])
        if current is None:
            self.call('POST', '/clients', desired)
        else:
            # Retain client UUID, secret and unrelated settings. Add local origins
            # without removing existing redirect URIs on an existing dev client.
            merged = dict(current, **desired)
            for key in ('redirectUris', 'webOrigins'):
                if key in desired:
                    merged[key] = sorted(set(current.get(key, []) + desired[key]))
            self.call('PUT', '/clients/' + current['id'], merged)
        return self.client(desired['clientId'])


def realm_definition():
    return json.loads((ROOT / 'keycloak/realm-export.json').read_text())


def healthbi_clients():
    clients = realm_definition()['clients']
    return {name: next(client for client in clients if client['clientId'] == name)
            for name in (BROWSER_CLIENT, WORKER_CLIENT)}


def imported_credential(kc):
    # Normal startup is read-only: Keycloak's realm import owns client creation.
    for name in (ROLE_CLIENT, BROWSER_CLIENT, WORKER_CLIENT):
        if kc.client(name) is None:
            raise RuntimeError(name + ' is missing. For an existing realm run make sso-reconcile-keycloak once; fresh realms import keycloak/realm-export.json.')
    worker = kc.client(WORKER_CLIENT)
    credential = kc.call('GET', '/clients/' + worker['id'] + '/client-secret')['value']
    if not credential:
        raise RuntimeError('Imported scheduler worker has no credential')
    return credential


def reconcile_clients(kc):
    # Explicit upgrade for persisted realms, which startup import does not replace.
    # Definitions and grants are read from the same realm file as fresh imports.
    role_client = kc.client(ROLE_CLIENT)
    if role_client is None:
        raise RuntimeError('data-statistics is missing; import the portal realm first')
    for desired in healthbi_clients().values():
        kc.ensure_client(desired)
    worker = kc.client(WORKER_CLIENT)
    role_definition = next(role for role in realm_definition()['roles']['client'][ROLE_CLIENT]
                           if role['name'] == SERVICE_ROLE)
    role_path = '/clients/' + role_client['id'] + '/roles/' + SERVICE_ROLE
    role = kc.call('GET', role_path, missing=True)
    if role is None:
        kc.call('POST', '/clients/' + role_client['id'] + '/roles', role_definition)
        role = kc.call('GET', role_path)
    account = kc.call('GET', '/clients/' + worker['id'] + '/service-account-user')
    mapping = '/users/' + account['id'] + '/role-mappings/clients/' + role_client['id']
    kc.call('POST', mapping, [role])
    kc.call('POST', '/clients/' + worker['id'] + '/scope-mappings/clients/' + role_client['id'], [role])
    return imported_credential(kc)


def local_keycloak():
    settings = settings_from_keycloak()
    token = http_json('http://localhost:8081/realms/master/protocol/openid-connect/token', 'POST', form={
        'client_id': 'admin-cli', 'grant_type': 'password',
        'username': os.environ.get('LOCAL_KEYCLOAK_ADMIN_USERNAME', settings.get('KC_BOOTSTRAP_ADMIN_USERNAME', 'admin')),
        'password': os.environ.get('LOCAL_KEYCLOAK_ADMIN_PASSWORD', settings.get('KC_BOOTSTRAP_ADMIN_PASSWORD', '')),
    })['access_token']
    return Keycloak('http://localhost:8081', 'moh-realm', token)


def compose(override=False):
    command = ['docker', 'compose', '-f', str(ROOT / 'docker-compose.dev.yml')]
    if override:
        command += ['-f', str(STATE / 'compose.healthbi.yaml')]
    return command


def settings_from_keycloak():
    settings = dict(value.split('=', 1) for value in json.loads(capture(
        ['docker', 'inspect', '--format', '{{json .Config.Env}}', 'keycloak'])) if '=' in value)
    return settings


def configure_healthbi(repo, credential):
    env = repo / '.local-docker/.env'
    existing = env_values(env)
    values = {
        'AUTH_MODE': 'on', 'KEYCLOAK_URL': 'http://localhost:8081', 'KEYCLOAK_REALM': 'moh-realm',
        'KEYCLOAK_CLIENT_ID': BROWSER_CLIENT, 'KEYCLOAK_ROLES_CLIENT_ID': ROLE_CLIENT,
        'BASE_PATH': '', 'SERVER_PORT': '8080',
        'PORTAL_API_BASE_URL': 'http://localhost:9000',
        'PORTAL_FRAME_ANCESTORS': 'http://localhost:3000',
        'SCHEDULER_PUBLIC_BASE_URL': 'http://localhost:8080',
        'SCHEDULER_KEYCLOAK_CLIENT_ID': WORKER_CLIENT, 'SCHEDULER_KEYCLOAK_AUDIENCE': ROLE_CLIENT,
        'SCHEDULER_KEYCLOAK_ROLE': SERVICE_ROLE,
        'SCHEDULER_ARTIFACT_SIGNING_KEY': existing.get('SCHEDULER_ARTIFACT_SIGNING_KEY') or secrets.token_urlsafe(48),
    }
    update_env(env, values)
    private_write(STATE / 'healthbi.env',
                  'HEALTH_BI_BASE_URL=http://host.docker.internal:8080\n'
                  'HEALTH_BI_KEYCLOAK_CLIENT_ID=' + WORKER_CLIENT + '\n'
                  'HEALTH_BI_KEYCLOAK_CLIENT_SECRET=' + credential + '\n')
    private_write(STATE / 'compose.healthbi.yaml', json.dumps({'services': {
        'backend': {'env_file': [str(STATE / 'healthbi.env')],
                    'extra_hosts': ['host.docker.internal:host-gateway']},
        'frontend': {'environment': {'VITE_HEALTH_BI_BASE_URL': 'http://localhost:8080'}},
    }}, indent=2) + '\n')


def healthbi_ready():
    with urllib.request.urlopen('http://localhost:8080/api/auth/config', timeout=10) as response:
        config = json.load(response)
        frames = response.headers.get('Content-Security-Policy', '')
    kc = config.get('keycloak', {})
    return (config.get('mode') == 'on' and kc.get('clientId') == BROWSER_CLIENT
            and kc.get('realm') == 'moh-realm' and kc.get('rolesClientId') == ROLE_CLIENT
            and 'frame-ancestors http://localhost:3000;' in frames)


def start_healthbi(repo):
    try:
        if healthbi_ready():
            print('Health BI is already running; reusing the local server.', flush=True)
            return
    except (OSError, RuntimeError, ValueError):
        pass
    # A developer-managed Air process may be rebuilding after the env change.
    # Never spawn a second server onto its occupied port.
    try:
        with socket.create_connection(('localhost', 8080), timeout=1):
            pass
    except OSError:
        pass
    else:
        wait_for(healthbi_ready, 'the existing local Health BI server')
        return
    pid_file = STATE / 'healthbi.pid'
    if pid_file.exists():
        try:
            os.kill(int(pid_file.read_text()), 0)
            wait_for(healthbi_ready, 'managed Health BI server')
            return
        except ProcessLookupError:
            pass
    log_path = STATE / 'healthbi.log'
    with log_path.open('ab') as output:
        process = subprocess.Popen(['bash', str(repo / 'scripts/start-docker-local.sh')], cwd=repo,
                                   stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
    private_write(pid_file, str(process.pid))
    wait_for(healthbi_ready, 'Health BI (see .local-sso/healthbi.log)', seconds=240)


def up(repo):
    if not (repo / 'server/scheduler_keycloak.go').exists():
        raise RuntimeError('Set HEALTHBI_PROJECT_DIR to the sibling dwh_visualization checkout')
    startup = repo / 'scripts/start-docker-local.sh'
    if not startup.exists() or 'HEALTHBI_PREPARE_ONLY' not in startup.read_text():
        raise RuntimeError('Update the Health BI checkout: its preparation-only startup mode is required')
    STATE.mkdir(mode=0o700, exist_ok=True)
    STATE.chmod(0o700)
    print('Starting shared SSO infrastructure...', flush=True)
    run(compose() + ['up', '-d', 'keycloak-db', 'keycloak', 'backend-db', 'redis', 'mailhog'])
    wait_for(lambda: http_json('http://localhost:8081/realms/moh-realm/.well-known/openid-configuration'), 'Keycloak realm')
    credential = imported_credential(local_keycloak())
    print('Using imported Health BI clients and existing data-statistics roles.', flush=True)
    prepare_env = dict(os.environ, HEALTHBI_PREPARE_ONLY='1')
    run(['bash', str(repo / 'scripts/start-docker-local.sh')], cwd=repo, env=prepare_env)
    configure_healthbi(repo, credential)
    start_healthbi(repo)
    print('Starting portal backend and frontend...', flush=True)
    command = compose(True) + ['up', '-d']
    if os.environ.get('SSO_BUILD') == '1':
        command.append('--build')
    # Keep buildx metadata in our private runtime directory rather than relying
    # on a possibly root-owned Docker metadata cache from earlier builds.
    compose_env = dict(os.environ)
    compose_env.setdefault('BUILDX_CONFIG', str(STATE / 'buildx'))
    run(command, env=compose_env)
    wait_for(lambda: http_json('http://localhost:9000/health/ready'), 'portal backend', seconds=240)
    wait_for(lambda: http_ready('http://localhost:3000/portal/'), 'portal frontend', seconds=240)
    # Verify the actual service-account token against Health BI's protected API.
    service_token = http_json('http://localhost:8081/realms/moh-realm/protocol/openid-connect/token', 'POST', form={
        'grant_type': 'client_credentials', 'client_id': WORKER_CLIENT, 'client_secret': credential,
    })['access_token']
    wait_for(lambda: http_json('http://localhost:8080/api/v1/reports', token=service_token), 'authenticated Health BI scheduler API')
    print('Ready: portal http://localhost:3000 | Health BI http://localhost:8080 | Keycloak http://localhost:8081', flush=True)


def down(repo):
    if (STATE / 'compose.healthbi.yaml').exists():
        run(compose(True) + ['down'])
    else:
        run(compose() + ['down'])
    pid_file = STATE / 'healthbi.pid'
    if pid_file.exists():
        pid = int(pid_file.read_text())
        try:
            command = capture(['ps', '-p', str(pid), '-o', 'command='])
            if str(repo) not in command:
                raise RuntimeError('Health BI PID identity changed; refusing to stop an unrelated process')
            os.killpg(pid, signal.SIGTERM)
        except (ProcessLookupError, subprocess.CalledProcessError):
            pass
        pid_file.unlink()
    print('Stopped managed local services; database volumes and credentials retained.')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['up', 'down', 'status', 'reconcile-keycloak'])
    args = parser.parse_args()
    repo = Path(os.environ.get('HEALTHBI_PROJECT_DIR', ROOT.parent / 'dwh_visualization')).resolve()
    if args.action == 'up':
        up(repo)
    elif args.action == 'reconcile-keycloak':
        reconcile_clients(local_keycloak())
        print('Health BI clients reconciled from keycloak/realm-export.json; existing secrets retained.')
    elif args.action == 'down':
        down(repo)
    else:
        run(compose((STATE / 'compose.healthbi.yaml').exists()) + ['ps'])
        try:
            ready = healthbi_ready()
        except (OSError, RuntimeError, ValueError):
            ready = False
        print('Health BI: ' + ('ready' if ready else 'not ready'))


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, ValueError, subprocess.CalledProcessError) as error:
        print('Local startup failed: ' + str(error), file=sys.stderr)
        sys.exit(1)
