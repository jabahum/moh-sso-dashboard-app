#!/usr/bin/env python3
"""Serve built portal assets locally, with SPA fallback and a local-only API URL."""
import argparse
import json
import re
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit

FRONTEND = Path(__file__).resolve().parents[2] / 'frontend'
ROOT = FRONTEND / 'dist'
DEPENDENCIES = FRONTEND / 'node_modules/.cache/browser-dependencies'


class FixtureServer(ThreadingHTTPServer):
    # ES module graphs request many same-origin dependencies concurrently.
    request_queue_size = 128


class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):
        path = urlsplit(self.path).path
        if path == '/config.js':
            content = b'window.__APP_CONFIG__={API_BASE_URL:"/ssobackend",microfrontendMode:"remote",singleSpaOrchestration:true,microfrontendMountMode:"orchestrated"};'
            self.send_response(200)
            self.send_header('Content-Type', 'application/javascript')
            self.send_header('Content-Length', str(len(content)))
            self.end_headers()
            self.wfile.write(content)
            return
        if path.startswith('/__browser_dependencies__/'):
            filename = path.removeprefix('/__browser_dependencies__/')
            if '/' in filename or not re.fullmatch(r'dependency-[0-9]+\.js', filename):
                self.send_error(404)
                return
            target = DEPENDENCIES / filename
            if not target.is_file():
                self.send_error(404)
                return
            self.send_content(target.read_bytes(), 'application/javascript')
            return
        if path == '/index.html' or not Path(path).suffix:
            html = (ROOT / 'index.html').read_text()
            local_imports = json.loads((DEPENDENCIES / 'import-map.json').read_text())['imports']
            def replace_import_map(match):
                import_map = json.loads(match.group(1))
                import_map['imports'].update(local_imports)
                return '<script type="importmap">' + json.dumps(import_map) + '</script>'
            html, replacements = re.subn(r'<script type="importmap">(.*?)</script>', replace_import_map, html, flags=re.S)
            if replacements != 1:
                self.send_error(500, 'Expected one production import map')
                return
            self.send_content(html.encode(), 'text/html')
            return
        super().do_GET()

    def send_content(self, content, content_type):
        self.send_response(200)
        self.send_header('Content-Type', content_type)
        self.send_header('Content-Length', str(len(content)))
        self.end_headers()
        self.wfile.write(content)

    def log_message(self, *args):
        pass


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--port', type=int, default=5417)
    args = parser.parse_args()
    if not (ROOT / 'index.html').is_file():
        raise SystemExit('Run the production frontend build before browser tests')
    FixtureServer(('127.0.0.1', args.port), partial(Handler, directory=str(ROOT))).serve_forever()
