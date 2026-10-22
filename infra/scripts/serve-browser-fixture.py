#!/usr/bin/env python3
"""Serve built portal assets locally, with SPA fallback and a local-only API URL."""
import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[2] / 'frontend/dist'


class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):
        path = urlsplit(self.path).path
        if path == '/portal/config.js':
            content = b'window.__APP_CONFIG__={API_BASE_URL:"/ssobackend",microfrontendMode:"remote",singleSpaOrchestration:true,microfrontendMountMode:"orchestrated"};'
            self.send_response(200)
            self.send_header('Content-Type', 'application/javascript')
            self.send_header('Content-Length', str(len(content)))
            self.end_headers()
            self.wfile.write(content)
            return
        if path.startswith('/portal/') and not Path(path).suffix:
            self.path = '/portal/index.html'
        super().do_GET()

    def log_message(self, *args):
        pass


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--port', type=int, default=5417)
    args = parser.parse_args()
    if not (ROOT / 'portal/index.html').is_file():
        raise SystemExit('Run the production frontend build before browser tests')
    ThreadingHTTPServer(('127.0.0.1', args.port), partial(Handler, directory=str(ROOT))).serve_forever()
