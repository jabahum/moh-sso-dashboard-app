#!/usr/bin/env python3
"""Compose/environment contracts; Python 3, Docker Compose 2.24.4+ and jq."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


def config(filename, env_file='infra/env/prod.env.example', **values):
    env = {key: value for key, value in os.environ.items()
           if key not in {'COMPOSE_PROFILES', 'COMPOSE_FILE', 'DEV_ENV_FILE', 'APP_BIND_ADDRESS'}}
    env.update(APP_ENV_FILE=str(ROOT / env_file), BACKEND_TAG='1.2.3', FRONTEND_TAG='sha-1234567')
    env.update(values)
    result = subprocess.run(['docker', 'compose', '--env-file', str(ROOT / env_file),
                             '-f', str(ROOT / filename), 'config', '--format', 'json'],
                            env=env, cwd=ROOT, text=True, capture_output=True)
    if result.returncode:
        raise AssertionError(result.stderr)
    return json.loads(result.stdout)


class ComposeTests(unittest.TestCase):
    def test_launcher_resolves_paths_from_outside_the_repository(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            fake = tmp / 'docker'
            fake.write_text('#!/usr/bin/env python3\nimport json,os,sys\n'
                            'print(json.dumps({"args":sys.argv[1:],"cwd":os.getcwd(),'
                            '"app":os.environ.get("APP_ENV_FILE"),"dev":os.environ.get("DEV_ENV_FILE")}))\n')
            fake.chmod(0o755)
            marker = tmp / 'must-not-exist'
            private_env = tmp / 'settings with spaces.env'
            private_env.write_text(f'BACKEND_TAG=1.2.3\nDONT_SOURCE=$(touch "{marker}")\n')
            remote_env = tmp / 'remote.env'
            remote_env.write_text('APP_BIND_ADDRESS=10.0.0.20\n')
            env = {**os.environ, 'PATH': str(tmp) + os.pathsep + os.environ['PATH'],
                   'ENV_FILE': str(private_env), 'REMOTE_ENV_FILE': str(remote_env)}
            for mode in ('prod', 'dev', 'gateway', 'remote', 'external'):
                result = subprocess.run(['sh', str(ROOT / 'infra/scripts/compose.sh'), mode, 'config', '--quiet'],
                                        cwd=tmp, env=env, text=True, capture_output=True, check=True)
                call = json.loads(result.stdout)
                self.assertEqual(call['cwd'], str(ROOT))
                self.assertEqual(call['args'][-2:], ['config', '--quiet'])
                self.assertEqual(call['args'][call['args'].index('--env-file') + 1], str(private_env))
                if mode == 'remote':
                    self.assertEqual(call['args'].count('--env-file'), 2)
                self.assertEqual(call['dev' if mode in ('dev', 'external') else 'app'], str(private_env))
            self.assertFalse(marker.exists(), 'Environment files must never execute as shell code')

    def test_relocated_build_and_bind_mount_paths_exist(self):
        model = config('infra/compose/docker-compose.dev.yml', 'infra/env/dev.env.example')
        for service in model['services'].values():
            if 'build' in service:
                context = Path(service['build']['context'])
                self.assertEqual(context, ROOT)
                self.assertTrue((context / service['build']['dockerfile']).is_file())
            for volume in service.get('volumes', []):
                if volume['type'] == 'bind':
                    self.assertTrue(Path(volume['source']).exists(), volume['source'])
        self.assertFalse((ROOT / 'docker-compose.yml').exists())
        self.assertTrue((ROOT / '.dockerignore').is_file())

    def test_production_entry_points_match(self):
        default, prod = config('infra/compose/docker-compose.yml'), config('infra/compose/docker-compose.prod.yml')
        for field in ('name', 'services', 'networks', 'volumes', 'secrets'):
            self.assertEqual(default[field], prod[field], field)

    def test_all_application_envs_reach_backend_and_frontend_is_public_only(self):
        for filename, example in [('infra/compose/docker-compose.yml', 'infra/env/prod.env.example'),
                                  ('infra/compose/docker-compose.dev.yml', 'infra/env/dev.env.example')]:
            services = config(filename, example)['services']
            declared = set(re.findall(r'^([A-Z][A-Z0-9_]+)=', (ROOT / example).read_text(), re.M))
            backend = services['backend']['environment']
            self.assertTrue(declared <= backend.keys(), sorted(declared - backend.keys()))
            frontend = services['frontend']['environment']
            self.assertFalse(any(key.endswith(('PASSWORD', 'SECRET', 'TOKEN', 'KEY')) for key in frontend))
            self.assertNotIn('DB_USER', frontend)
        # Application config fields must appear in both examples; Keycloak container
        # infrastructure belongs only to development because production uses an IdP.
        fields = set(re.findall(r'mapstructure:"([A-Z][A-Z0-9_]+)"', (ROOT / 'backend/internal/config/config.go').read_text()))
        infrastructure = {'KEYCLOAK_VERSION', 'KEYCLOAK_DB', 'KEYCLOAK_DB_NAME', 'KEYCLOAK_DB_USER',
                          'KEYCLOAK_DB_PASSWORD', 'KEYCLOAK_ADMIN', 'KEYCLOAK_ADMIN_PASSWORD', 'KEYCLOAK_HOSTNAME'}
        for example in ('infra/env/prod.env.example', 'infra/env/dev.env.example'):
            declared = set(re.findall(r'^([A-Z][A-Z0-9_]+)=', (ROOT / example).read_text(), re.M))
            self.assertFalse(fields - infrastructure - declared, (example, fields - infrastructure - declared))

    def test_production_database_redis_secrets_and_remote_binding(self):
        services = config('infra/compose/docker-compose.prod.yml', DB_USER='custom_owner', DB_NAME='custom_database',
                          REDIS_PASSWORD='fixture-password', APP_BIND_ADDRESS='10.0.0.20')['services']
        db = services['backend-db']['environment']
        api = services['backend']['environment']
        self.assertEqual(db['POSTGRES_USER'], api['DB_USER'])
        self.assertEqual(db['POSTGRES_DB'], api['DB_NAME'])
        self.assertEqual(db['POSTGRES_PASSWORD_FILE'], api['DB_PASSWORD_FILE'])
        self.assertEqual(services['redis']['environment']['REDIS_PASSWORD'], api['REDIS_PASSWORD'])
        self.assertEqual(api['ENVIRONMENT'], 'production')
        self.assertEqual(api['DEV_AUTH_BYPASS'], 'false')
        self.assertEqual(len(services['backend']['secrets']), 3)
        for name in ('frontend', 'backend'):
            self.assertEqual(services[name]['ports'][0]['host_ip'], '10.0.0.20')
        for name in ('redis', 'backend-db'):
            self.assertNotIn('ports', services[name])
        gateway = config('infra/compose/docker-compose-nginx.yml')['services']
        for name in ('frontend', 'backend'):
            self.assertFalse(gateway[name].get('ports'))
        for name in ('backend', 'frontend', 'redis', 'backend-db'):
            expected = config('infra/compose/docker-compose.yml')['services'][name]
            for field in ('environment', 'healthcheck', 'volumes', 'secrets'):
                self.assertEqual(gateway[name].get(field), expected.get(field), (name, field))

    def test_development_topology_and_new_module_ports(self):
        model = config('infra/compose/docker-compose.dev.yml', 'infra/env/dev.env.example')
        self.assertEqual(model['name'], 'moh-sso-dashboard-dev')
        self.assertFalse(model['networks']['sso_network'].get('external'))
        services = model['services']
        backend = services['backend']
        self.assertEqual(backend['environment']['DWH_HOST'], 'remote-postgres')
        self.assertEqual(backend['environment']['SMTP_HOST'], 'mailhog')
        self.assertEqual(backend['environment']['ENVIRONMENT'], 'development')
        for name in ('backend-db', 'remote-postgres', 'redis', 'keycloak'):
            self.assertEqual(backend['depends_on'][name]['condition'], 'service_healthy')
        published = {int(port['published']) for port in services['frontend']['ports']}
        self.assertTrue({3000, *range(4101, 4116)} <= published)
        for service in services.values():
            for port in service.get('ports', []):
                self.assertEqual(port['host_ip'], '127.0.0.1')

    def test_user_development_env_wins_over_fallback(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'dev.env'
            path.write_text('NOTIFICATION_DELIVERY_BATCH_SIZE=57\nSMTP_SEND_TIMEOUT=29s\n')
            model = config('infra/compose/docker-compose.dev.yml', 'infra/env/dev.env.example', DEV_ENV_FILE=str(path))
            env = model['services']['backend']['environment']
            self.assertEqual(env['NOTIFICATION_DELIVERY_BATCH_SIZE'], '57')
            self.assertEqual(env['SMTP_SEND_TIMEOUT'], '29s')

    def test_external_keycloak_mode_has_no_local_idp_dependency(self):
        model = config('infra/compose/docker-compose-without-keycloak.yml', 'infra/env/dev.env.example',
                       KEYCLOAK_INTERNAL_URL='https://identity.example.org',
                       KEYCLOAK_BASE_URL='https://identity.example.org',
                       KEYCLOAK_EXTERNAL_URL='https://identity.example.org', KEYCLOAK_REALM='external',
                       KEYCLOAK_ADMIN_CLIENT_ID='external-admin', KEYCLOAK_ADMIN_CLIENT_SECRET='fixture-admin',
                       KEYCLOAK_WEB_CLIENT_ID='external-web', KEYCLOAK_WEB_CLIENT_SECRET='fixture-web')
        backend = model['services']['backend']
        self.assertNotIn('keycloak', backend['depends_on'])
        self.assertEqual(backend['environment']['KEYCLOAK_REALM'], 'external')
        self.assertNotIn('keycloak', model['services'])
        self.assertNotIn('keycloak-db', model['services'])

    def test_file_secrets_override_placeholders_and_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'password'
            path.write_text('fixture-secret-with-$-and-quotes"\n')
            env = {**os.environ, 'DB_PASSWORD': 'placeholder', 'DB_PASSWORD_FILE': str(path)}
            entrypoint = ROOT / 'infra/docker/backend/entrypoint.sh'
            result = subprocess.run(['sh', str(entrypoint), 'sh', '-ec',
                                     'test "$DB_PASSWORD" = \'fixture-secret-with-$-and-quotes"\''], env=env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, b'')
            for value in (None, ''):
                if value is None:
                    path.unlink()
                else:
                    path.write_text(value)
                result = subprocess.run(['sh', str(entrypoint), 'true'], env=env, capture_output=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn(b'fixture-secret', result.stderr)

    def test_browser_config_is_escaped_and_excludes_backend_secrets(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'config.js'
            api = 'https://api.example.org/path?label="quoted"'
            env = {**os.environ, 'API_BASE_URL': api, 'DB_PASSWORD': 'never-publish',
                   'KEYCLOAK_WEB_CLIENT_SECRET': 'never-publish',
                   'FRONTEND_MICROFRONTEND_MODE': 'local', 'FRONTEND_SINGLE_SPA_ORCHESTRATION': 'false'}
            subprocess.run(['sh', str(ROOT / 'infra/docker/frontend/40-runtime-config.sh'), str(path)],
                           env=env, check=True, capture_output=True)
            text = path.read_text()
            data = json.loads(text.split('window.__APP_CONFIG__ = ', 1)[1].rstrip().removesuffix(';'))
            self.assertEqual(data['API_BASE_URL'], api)
            self.assertFalse(data['singleSpaOrchestration'])
            self.assertEqual(data['microfrontendMode'], 'local')
            self.assertNotIn('never-publish', text)


if __name__ == '__main__':
    unittest.main()
