"""Local reference BFF: code+PKCE, opaque sessions, fresh platform access checks.

It consumes access tokens through the platform decision endpoint; it never decodes
or trusts an ID token. Production deployments should use a maintained OIDC library
and a shared encrypted session store. This single-process sample uses no framework.
"""
import base64
import hashlib
import html
import hmac
import json
import os
import secrets
import time
from http.cookies import SimpleCookie
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlencode, urlparse

from platform_client import PlatformClient, PlatformError, discover, machine_token, request_json, safe_url


class Demo:
    def __init__(self):
        self.public = safe_url(os.environ.get("APP_PUBLIC_URL", "http://127.0.0.1:8082"))
        if urlparse(self.public).path not in ("", "/"):
            raise PlatformError("sample APP_PUBLIC_URL must be an origin")
        self.issuer = safe_url(os.environ["OIDC_ISSUER"])
        self.client_id = os.environ["APPLICATION_ID"]
        self.client = PlatformClient(os.environ["PLATFORM_BASE_URL"])
        self.oidc = discover(self.issuer)
        self.pending = {}
        self.sessions = {}
        self.secure_cookie = self.public.startswith("https://")

    def clean(self):
        now = time.time()
        self.pending = {k: v for k, v in self.pending.items() if v["expires"] > now}
        self.sessions = {k: v for k, v in self.sessions.items() if v["expires"] > now}


class App(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass  # Never put callback codes, token responses or recipients in access logs.

    def cookie(self, name):
        try:
            cookies = SimpleCookie(self.headers.get("Cookie", ""))
            return cookies[name].value if name in cookies else ""
        except Exception:
            return ""

    def set_cookie(self, name, value, age):
        secure = "; Secure" if self.server.demo.secure_cookie else ""
        self.send_header("Set-Cookie", f"{name}={value}; Path=/; HttpOnly; SameSite=Lax; Max-Age={age}{secure}")

    def redirect(self, target, cookie=None):
        self.send_response(303)
        self.send_header("Location", target)
        self.send_header("Cache-Control", "no-store")
        if cookie:
            self.set_cookie(*cookie)
        self.end_headers()

    def page(self, body, status=200):
        data = ("<!doctype html><html lang='en'><meta charset='utf-8'><title>MOH platform pilot</title><h1>Independent application pilot</h1>" + body + "</html>").encode()
        self.send_response(status)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        issuer = urlparse(self.server.demo.issuer)
        auth_origin = issuer.scheme + "://" + issuer.netloc
        self.send_header("Content-Security-Policy", "default-src 'none'; form-action 'self' " + auth_origin + "; frame-ancestors 'none'; base-uri 'none'")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def current(self):
        demo = self.server.demo
        session = demo.sessions.get(self.cookie("pilot_session"))
        if not session:
            raise PlatformError("sign in required")
        # Every protected request gets a fresh application-specific authorization decision.
        access = demo.client.access(demo.client_id, session["token"])
        if access.get("subject") != session["subject"]:
            raise PlatformError("session identity mismatch")
        return session, access

    def do_GET(self):
        demo = self.server.demo
        demo.clean()
        path = urlparse(self.path)
        try:
            if path.path == "/login":
                if len(demo.pending) >= 1000:
                    raise PlatformError("too many pending sign-ins")
                state, verifier = secrets.token_urlsafe(32), secrets.token_urlsafe(48)
                challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()
                demo.pending[state] = {"verifier": verifier, "expires": time.time() + 300}
                query = urlencode({"client_id": demo.client_id, "redirect_uri": demo.public + "/callback", "response_type": "code", "scope": "openid identity:read", "state": state, "code_challenge": challenge, "code_challenge_method": "S256"})
                self.redirect(demo.oidc["authorization_endpoint"] + "?" + query, ("pilot_state", state, 300))
            elif path.path == "/callback":
                query = parse_qs(path.query)
                state = query.get("state", [""])[0]
                pending = demo.pending.pop(state, None)
                if not pending or not hmac.compare_digest(state, self.cookie("pilot_state")):
                    raise PlatformError("invalid sign-in state")
                if query.get("iss", [demo.issuer])[0] != demo.issuer or not query.get("code"):
                    raise PlatformError("invalid sign-in response")
                form = {"grant_type": "authorization_code", "client_id": demo.client_id, "redirect_uri": demo.public + "/callback", "code_verifier": pending["verifier"], "code": query["code"][0]}
                if os.environ.get("APP_CLIENT_SECRET"):
                    form["client_secret"] = os.environ["APP_CLIENT_SECRET"]
                tokens = request_json(demo.oidc["token_endpoint"], form=form)
                if str(tokens.get("token_type", "")).lower() != "bearer" or not tokens.get("access_token"):
                    raise PlatformError("invalid token response")
                access = demo.client.access(demo.client_id, tokens["access_token"])
                if not access.get("subject") or len(demo.sessions) >= 1000:
                    raise PlatformError("session cannot be established")
                sid = secrets.token_urlsafe(32)
                demo.sessions[sid] = {"token": tokens["access_token"], "subject": access["subject"], "csrf": secrets.token_urlsafe(32), "expires": time.time() + 600}
                self.redirect("/", ("pilot_session", sid, 600))
            elif path.path == "/":
                if not self.cookie("pilot_session"):
                    self.page("<p><a href='/login'>Sign in with MOH</a></p>")
                    return
                session, access = self.current()
                csrf = html.escape(session["csrf"], quote=True)
                notify = ""
                if "operator" in access.get("roles", []):
                    notify = f"<form method='post' action='/notify'><input type='hidden' name='csrf' value='{csrf}'><label>Test recipient <input type='email' name='email' required></label><button>Send test notification</button></form>"
                self.page("<p>Signed in. Application access verified.</p>" + notify + f"<form method='post' action='/logout'><input type='hidden' name='csrf' value='{csrf}'><button>Sign out</button></form>")
            else:
                self.page("<p>Not found.</p>", 404)
        except (PlatformError, KeyError, ValueError):
            self.page("<p>Access unavailable. <a href='/login'>Sign in again</a>.</p>", 403)

    def do_POST(self):
        demo = self.server.demo
        demo.clean()
        try:
            if self.headers.get("Origin") != demo.public:
                raise PlatformError("invalid origin")
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 < length <= 4096:
                raise PlatformError("invalid form size")
            values = parse_qs(self.rfile.read(length).decode())
            session, access = self.current()
            if not hmac.compare_digest(session["csrf"], values.get("csrf", [""])[0]):
                raise PlatformError("invalid form token")
            if self.path == "/logout":
                demo.sessions.pop(self.cookie("pilot_session"), None)
                self.redirect(demo.oidc["end_session_endpoint"] + "?" + urlencode({"client_id": demo.client_id, "post_logout_redirect_uri": demo.public + "/"}), ("pilot_session", "", 0))
            elif self.path == "/notify" and "operator" in access.get("roles", []):
                token = machine_token(demo.oidc["token_endpoint"], os.environ["MACHINE_CLIENT_ID"], os.environ["MACHINE_CLIENT_SECRET"], scope="identity:read notifications:write")
                identity = demo.client.identity(token)
                if identity.get("applicationId") != demo.client_id:
                    raise PlatformError("incorrect integration binding")
                result = demo.client.notify(token, secrets.token_hex(16), values.get("email", [""])[0], "Pilot integration test", "Your independent application can use the platform.")
                self.page("<p>Notification accepted: " + html.escape(result["id"]) + "</p><a href='/'>Return</a>")
            else:
                self.page("<p>Action not allowed.</p>", 403)
        except (PlatformError, KeyError, ValueError):
            self.page("<p>Request denied or service unavailable.</p>", 403)


if __name__ == "__main__":
    try:
        demo = Demo()
        server = HTTPServer(("127.0.0.1", int(os.environ.get("APP_PORT", "8082"))), App)
        server.demo = demo
        print("Pilot listening on loopback; public URL: " + demo.public)
        server.serve_forever()
    except (PlatformError, KeyError):
        raise SystemExit("Configure the example environment and identity provider before starting.")
