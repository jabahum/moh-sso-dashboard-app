#!/usr/bin/env python3
"""Exercise production Nginx; requires Python 3, Docker and the OpenSSL CLI."""
import http.client
import json
import os
import shutil
import ssl
from pathlib import Path
import subprocess
import tempfile
import time
import uuid

ROOT = Path(__file__).resolve().parents[2]
IMAGE = "nginx:1.30.6-alpine"


def docker(*args):
    result = subprocess.run(["docker", *args], text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(f"docker {' '.join(args)}\n{result.stderr}")
    return result.stdout.strip()


def main():
    # Check actual Compose interpolation without reading real deployment secrets.
    for bind_address in ('', '10.0.0.20'):
        env = {**os.environ, 'APP_BIND_ADDRESS': bind_address,
               'APP_ENV_FILE': str(ROOT / 'infra/env/prod.env.example'),
               'BACKEND_TAG': '1.2.3', 'FRONTEND_TAG': '1.2.3'}
        result = subprocess.run(['docker', 'compose', '-f', str(ROOT / 'infra/compose/docker-compose.yml'),
                                 'config', '--format', 'json'],
                                env=env, text=True, capture_output=True, check=True)
        services = json.loads(result.stdout)['services']
        for service, port in (('frontend', 3000), ('backend', 9000)):
            bindings = services[service]['ports']
            assert len(bindings) == 1
            assert bindings[0]['host_ip'] == (bind_address or '127.0.0.1')
            assert int(bindings[0]['published']) == port and bindings[0]['target'] == port
        for service in ('redis', 'backend-db'):
            assert not services[service].get('ports')
    name = "moh-nginx-test-" + uuid.uuid4().hex[:10]
    containers = []
    docker("network", "create", name)
    try:
        with tempfile.TemporaryDirectory(prefix="moh-nginx-") as tmp:
            tmp = Path(tmp)
            html = tmp / "html"
            for prefix in ("", "portal/"):
                fixtures = {
                    "index.html": "<!doctype html><title>Portal fixture</title>",
                    "config.js": "window.__APP_CONFIG__ = {};",
                    "config.production.js": "window.__APP_CONFIG__ = {};",
                    "import-map.json": '{"imports":{}}',
                    "version-manifest.json": '{}',
                    "sw.js": "self.addEventListener('fetch', () => {});",
                    "assets/index-abcdefgh.js": "export const hashed = true;",
                    "assets/stable.js": "export const stable = true;",
                    "mf/users/single-spa.js": "export const mount = () => {};",
                    "packages/auth/index.js": "export const auth = {};",
                    ".env": "MUST_NOT_BE_SERVED",
                    "assets/private.js.map": "MUST_NOT_BE_SERVED",
                }
                for path, contents in fixtures.items():
                    target = html / (prefix + path)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text(contents)
            # Use the exact port transformation performed by infra/docker/frontend/Dockerfile.
            frontend = tmp / "frontend.conf"
            frontend.write_text((ROOT / 'infra/docker/frontend/nginx.conf').read_text().replace(
                'listen 80 default_server;', 'listen 3000 default_server;'))
            backend = tmp / "backend.conf"
            backend.write_text('''server {
 listen 9000;
 absolute_redirect off;
 set $cookie_chunk ''' + 'x' * 3000 + ''';
 location = /swagger { return 307 swagger/; }
 location = /root-redirect { return 302 /api/v1/target; }
 location = /api/v1/auth/callback {
   add_header Set-Cookie "sso_session=fixture; Path=/; HttpOnly; Secure; SameSite=Lax" always;
   add_header Set-Cookie "legacy=$cookie_chunk$cookie_chunk$cookie_chunk; Path=/; HttpOnly; Secure" always;
   return 302 https://identity.example/login;
 }
 location = /api/v1/ws {
   add_header Upgrade $http_upgrade always;
   add_header Connection upgrade always;
   return 101;
 }
 location / {
   default_type application/json;
   return 200 '{"uri":"$request_uri","method":"$request_method","length":"$http_content_length","proto":"$http_x_forwarded_proto","ip":"$http_x_forwarded_for","forwarded":"$http_forwarded","connection":"$http_connection","request_id":"$http_x_request_id"}';
 }
}''')
            trusted = tmp / "trusted-proxies.conf"
            trusted.write_text((ROOT / "infra/nginx/trusted-proxies.conf").read_text())

            def start(label, alias, mounts, port=False, ip=None, tls=False):
                container = name + "-" + label
                containers.append(container)
                args = ["run", "-d", "--name", container, "--network", name,
                        "--network-alias", alias]
                if port:
                    args += ["-p", "127.0.0.1::80"]
                if tls:
                    args += ["-p", "127.0.0.1::443"]
                if ip:
                    args += ["--ip", ip]
                for source, target in mounts:
                    args += ["-v", f"{source}:{target}:ro"]
                docker(*args, IMAGE)
                return container

            backend_mounts = [(backend, "/etc/nginx/conf.d/default.conf")]
            start("backend", "backend", backend_mounts)
            start("frontend", "frontend", [(frontend, "/etc/nginx/conf.d/default.conf"),
                                             (html, "/usr/share/nginx/html")])
            gateway = start("gateway", "gateway", [
                (ROOT / "infra/nginx/nginx.conf", "/etc/nginx/nginx.conf"),
                (ROOT / "infra/nginx/app.conf", "/etc/nginx/conf.d/default.conf"),
                (trusted, "/etc/nginx/trusted-proxies.conf")], port=True)
            docker("exec", gateway, "nginx", "-t")
            docker("exec", name + "-backend", "nginx", "-t")
            docker("exec", name + "-frontend", "nginx", "-t")
            port = int(docker("port", gateway, "80/tcp").rsplit(":", 1)[1])
            use_tls = False

            def request(path, method="GET", body=None, headers=None):
                conn = (http.client.HTTPSConnection("127.0.0.1", port, timeout=5,
                        context=ssl._create_unverified_context()) if use_tls else
                        http.client.HTTPConnection("127.0.0.1", port, timeout=5))
                try:
                    conn.request(method, path, body=body, headers=headers or {})
                    response = conn.getresponse()
                    data = response.read() if response.status != 101 else b""
                    return response.status, response.headers, data
                finally:
                    conn.close()

            def wait_for(path, predicate, seconds=25):
                deadline = time.monotonic() + seconds
                while time.monotonic() < deadline:
                    try:
                        result = request(path)
                        if predicate(result):
                            return result
                    except (OSError, http.client.HTTPException):
                        pass
                    time.sleep(0.25)
                raise AssertionError(f"Timed out waiting for {path}")

            wait_for("/portal/", lambda r: r[0] == 200)
            wait_for("/health/ready", lambda r: r[0] == 200)
            for prefix in ("", "/portal"):
                for path, expected_cache in (
                    ("/config.js", "no-store"), ("/config.production.js", "no-store"),
                    ("/import-map.json", "no-store"), ("/version-manifest.json", "no-store"),
                    ("/sw.js", "no-cache"), ("/assets/stable.js", "no-cache"),
                    ("/mf/users/single-spa.js", "no-cache"), ("/packages/auth/index.js", "no-cache"),
                    ("/assets/index-abcdefgh.js", "public, max-age=31536000, immutable"),
                    ("/apps/documents", "no-store"),
                ):
                    status, headers, body = request(prefix + path)
                    assert status == 200, (path, status, body)
                    assert headers['Cache-Control'] == expected_cache, (path, headers)
                    assert headers.get_all('X-Content-Type-Options') == ['nosniff']
                    assert headers.get_all('X-Frame-Options') == ['SAMEORIGIN']
                for path in ("/assets/missing-abcdefgh.js", "/mf/missing", "/missing.json",
                             "/.env", "/assets/private.js.map"):
                    status, headers, _ = request(prefix + path)
                    assert status == 404, (path, status)
                    assert headers['Cache-Control'] == 'no-store'
                    assert headers['X-Content-Type-Options'] == 'nosniff'

            for prefix in ("", "/backend", "/ssobackend"):
                for path in ("/api/v1/probe?batch=2", "/health/ready", "/version", "/swagger/openapi.json"):
                    status, headers, body = request(prefix + path)
                    assert status == 200 and json.loads(body)['uri'] == path, (prefix, path, body)
                    assert headers['Cache-Control'] == 'no-store'
                status, headers, _ = request(prefix + '/swagger')
                assert status == 307 and headers['Location'] == 'swagger/'
                status, headers, _ = request(prefix + '/api/v1/auth/callback')
                assert status == 302 and headers['Location'] == 'https://identity.example/login'
                assert len(headers.get_all('Set-Cookie')) == 2
                assert 'Secure' in headers.get_all('Set-Cookie')[0]
                status, headers, body = request(prefix + '/api/v1/probe?batch=2', 'POST', b'payload')
                data = json.loads(body)
                assert data['method'] == 'POST' and data['length'] == '7'
                status, headers, _ = request(prefix + '/api/v1/ws', headers={'Upgrade': 'websocket', 'Connection': 'upgrade'})
                assert status == 101 and headers['Upgrade'] == 'websocket'
            for prefix in ('/backend', '/ssobackend'):
                assert request(prefix + '/root-redirect')[1]['Location'] == prefix + '/api/v1/target'
            assert request('/portal?keep=1')[:2][1]['Location'] == '/portal/?keep=1'
            assert request('/ssobackend/api/v1/probe', 'POST', headers={'Content-Length': str(50 * 1024 * 1024 + 1)})[0] == 413

            spoof = {'X-Forwarded-Proto': 'https', 'X-Forwarded-For': '203.0.113.99',
                     'Forwarded': 'for=203.0.113.99;proto=https'}
            status, headers, body = request('/api/v1/probe', headers=spoof)
            data = json.loads(body)
            assert data['proto'] == 'http' and data['ip'] != '203.0.113.99'
            assert data['forwarded'] == '' and data['connection'] == ''
            assert data['request_id'] == headers['X-Request-ID']
            # Opt in to the actual connecting proxy and check scheme/client restoration.
            trusted.write_text(f'''set_real_ip_from {data['ip']}/32;
geo $realip_remote_addr $trusted_proxy {{ default 0; {data['ip']}/32 1; }}
''')
            docker('exec', gateway, 'nginx', '-s', 'reload')
            wait_for('/api/v1/probe', lambda r: r[0] == 200)
            time.sleep(0.5)
            data = json.loads(request('/api/v1/probe', headers=spoof)[2])
            assert data['proto'] == 'https' and data['ip'] == '203.0.113.99', data
            request('/api/v1/probe?code=SHOULD_NOT_APPEAR_IN_LOG')
            assert 'SHOULD_NOT_APPEAR_IN_LOG' not in docker('logs', gateway)

            # Force a different backend IP and verify Docker DNS refresh without reload.
            old_backend = name + '-backend'
            old_ip = json.loads(docker('inspect', old_backend))[0]['NetworkSettings']['Networks'][name]['IPAddress']
            docker('rm', '-f', old_backend)
            start('ip-holder', 'unused', [], ip=old_ip)
            start('backend-replaced', 'backend', backend_mounts)
            wait_for('/ssobackend/health/ready', lambda r: r[0] == 200)

            # Emulate host processes sharing one loopback interface: Nginx on
            # 443, frontend on 3000 and backend on 9000. No Docker DNS upstreams.
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048',
                            '-nodes', '-keyout', str(tmp / 'key.pem'),
                            '-out', str(tmp / 'cert.pem'), '-days', '1',
                            '-subj', '/CN=localhost'], check=True, capture_output=True)
            host_site = tmp / 'host-site.conf'
            host_site.write_text('''include /etc/nginx/moh-sso/http.conf;
server {
 listen 443 ssl default_server;
 ssl_certificate /fixtures/cert.pem;
 ssl_certificate_key /fixtures/key.pem;
 add_header Strict-Transport-Security "max-age=31536000" always;
 add_header X-Host-Security "retained" always;
 include /etc/nginx/moh-sso/portal.locations.conf;
 location = / { return 200 "report-browser-root"; }
 location /report-browser/ { return 200 "report-browser-route"; }
}
''')
            host = start('host', 'host', [
                (host_site, '/etc/nginx/conf.d/default.conf'),
                (backend, '/etc/nginx/conf.d/backend.conf'),
                (frontend, '/etc/nginx/conf.d/frontend.conf'),
                (html, '/usr/share/nginx/html'),
                (ROOT / 'infra/nginx/host', '/etc/nginx/moh-sso'),
                (tmp / 'cert.pem', '/fixtures/cert.pem'),
                (tmp / 'key.pem', '/fixtures/key.pem')], tls=True)
            docker('exec', host, 'nginx', '-t')
            port = int(docker('port', host, '443/tcp').rsplit(':', 1)[1])
            use_tls = True
            wait_for('/portal/', lambda r: r[0] == 200)
            for path, expected in (('/', b'report-browser-root'),
                                   ('/report-browser/reports', b'report-browser-route')):
                assert request(path)[2] == expected
            for path, code, cache in (
                ('/portal/apps/documents', 200, 'no-store'),
                ('/portal/import-map.json', 200, 'no-store'),
                ('/portal/assets/index-abcdefgh.js', 200, 'public, max-age=31536000, immutable'),
                ('/portal/mf/users/single-spa.js', 200, 'no-cache'),
                ('/portal/assets/missing.js', 404, 'no-store'),
                ('/ssobackend/swagger/openapi.json', 200, 'no-store'),
            ):
                status, headers, _ = request(path)
                assert status == code and headers['Cache-Control'] == cache, (path, status, headers)
                assert headers['X-Host-Security'] == 'retained'
                assert headers.get_all('Strict-Transport-Security') == ['max-age=31536000']
            for path in ('/portal', '/ssobackend'):
                status, headers, _ = request(path + '?keep=1')
                assert status == 308 and headers['Location'] == path + '/?keep=1'
                assert headers['X-Host-Security'] == 'retained'
            status, headers, body = request('/ssobackend/api/v1/probe?batch=2', 'POST', b'payload',
                                            {**spoof, 'X-Forwarded-Proto': 'http'})
            data = json.loads(body)
            assert data['uri'] == '/api/v1/probe?batch=2' and data['length'] == '7'
            assert data['method'] == 'POST' and data['proto'] == 'https'
            assert data['ip'] != '203.0.113.99' and data['forwarded'] == ''
            assert data['request_id'] == headers['X-Request-ID']
            assert request('/ssobackend/swagger')[1]['Location'] == 'swagger/'
            assert request('/ssobackend/root-redirect')[1]['Location'] == '/ssobackend/api/v1/target'
            status, headers, _ = request('/ssobackend/api/v1/auth/callback')
            assert status == 302 and headers['Location'] == 'https://identity.example/login'
            assert len(headers.get_all('Set-Cookie')) == 2
            assert headers['Strict-Transport-Security'] == 'max-age=31536000'
            assert request('/ssobackend/api/v1/probe', 'POST', headers={'Content-Length': str(50 * 1024 * 1024 + 1)})[0] == 413
            assert request('/ssobackend/api/v1/ws', headers={'Upgrade': 'websocket', 'Connection': 'upgrade'})[0] == 101
            request('/ssobackend/api/v1/probe?code=HOST_LOG_SECRET')
            assert 'HOST_LOG_SECRET' not in docker('exec', host, 'cat', '/var/log/nginx/moh-sso-access.log')

            # A separate proxy process/network address reaches the application
            # through its IP, using the committed remote upstream template.
            app_ip = json.loads(docker('inspect', host))[0]['NetworkSettings']['Networks'][name]['IPAddress']
            remote_config = tmp / 'remote-nginx'
            shutil.copytree(ROOT / 'infra/nginx/host', remote_config)
            (remote_config / 'http.conf').write_text(
                (ROOT / 'infra/nginx/remote/http.conf').read_text().replace('10.0.0.20', app_ip))
            remote = start('remote', 'remote', [
                (host_site, '/etc/nginx/conf.d/default.conf'),
                (remote_config, '/etc/nginx/moh-sso'),
                (tmp / 'cert.pem', '/fixtures/cert.pem'),
                (tmp / 'key.pem', '/fixtures/key.pem')], tls=True)
            docker('exec', remote, 'nginx', '-t')
            port = int(docker('port', remote, '443/tcp').rsplit(':', 1)[1])
            wait_for('/portal/import-map.json', lambda r: r[0] == 200)
            assert request('/')[2] == b'report-browser-root'
            assert request('/report-browser/reports')[2] == b'report-browser-route'
            for path, expected_cache in (
                ('/portal/import-map.json', 'no-store'),
                ('/portal/assets/index-abcdefgh.js', 'public, max-age=31536000, immutable'),
                ('/portal/mf/users/single-spa.js', 'no-cache'),
                ('/ssobackend/swagger/openapi.json', 'no-store'),
            ):
                status, headers, _ = request(path)
                assert status == 200 and headers['Cache-Control'] == expected_cache
                assert headers['X-Host-Security'] == 'retained'
            status, headers, body = request('/ssobackend/api/v1/probe?batch=2', 'POST', b'payload',
                                            {**spoof, 'X-Forwarded-Proto': 'http'})
            data = json.loads(body)
            assert status == 200 and data['proto'] == 'https' and data['method'] == 'POST'
            assert data['uri'] == '/api/v1/probe?batch=2' and data['length'] == '7'
            assert data['ip'] != '203.0.113.99' and data['forwarded'] == ''
            assert data['request_id'] == headers['X-Request-ID']
            assert request('/ssobackend/swagger')[1]['Location'] == 'swagger/'
            status, headers, _ = request('/ssobackend/api/v1/auth/callback')
            assert status == 302 and headers['Location'] == 'https://identity.example/login'
            assert len(headers.get_all('Set-Cookie')) == 2
            assert request('/ssobackend/api/v1/ws', headers={'Upgrade': 'websocket', 'Connection': 'upgrade'})[0] == 101
            print('PASS: Compose bindings, gateway, same-host and remote-IP HTTPS Nginx; routing, SSO, uploads, WebSocket, caching, security headers, proxy trust, safe logs and DNS recovery')
    finally:
        for container in reversed(containers):
            subprocess.run(['docker', 'rm', '-f', container], capture_output=True)
        subprocess.run(['docker', 'network', 'rm', name], capture_output=True)


if __name__ == '__main__':
    main()
