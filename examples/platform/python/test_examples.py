import json
import threading
import unittest
from http.server import HTTPServer
from types import SimpleNamespace
from unittest.mock import Mock, patch
from urllib.error import HTTPError
from urllib.parse import parse_qs, urlparse
from urllib.request import Request, build_opener

from external_app import App
from platform_client import NoRedirect, PlatformClient, PlatformError, safe_url


class ClientTests(unittest.TestCase):
    def test_client_uses_public_contract_and_scoped_paths(self):
        with patch("platform_client.request_json", return_value={"success": True, "data": {"subject": "user"}}) as request:
            client = PlatformClient("https://portal.test/api/v1")
            self.assertEqual(client.access("pilot/other", "token")["subject"], "user")
            self.assertEqual(request.call_args.args[0], "https://portal.test/api/v1/platform/applications/pilot%2Fother/access")
            self.assertEqual(request.call_args.kwargs["token"], "token")

    def test_configuration_rejects_credential_urls_and_cleartext_remote(self):
        for url in ("http://remote.test", "https://user:secret@remote.test", "https://remote.test?secret=value"):
            with self.assertRaises(PlatformError):
                safe_url(url)
        self.assertEqual(safe_url("http://127.0.0.1:8081"), "http://127.0.0.1:8081")


class ExternalAppTests(unittest.TestCase):
    def setUp(self):
        self.server = HTTPServer(("127.0.0.1", 0), App)
        self.base = "http://127.0.0.1:%d" % self.server.server_port
        self.client = Mock()
        self.client.access.return_value = {"subject": "user", "permissions": []}
        self.server.demo = SimpleNamespace(public=self.base, issuer="https://identity.test/realms/moh", client_id="pilot", client=self.client, pending={}, sessions={}, secure_cookie=False, clean=lambda: None, oidc={"authorization_endpoint": "https://identity.test/auth", "token_endpoint": "https://identity.test/token", "end_session_endpoint": "https://identity.test/logout"})
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.opener = build_opener(NoRedirect())

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def request(self, path, *, headers=None, data=None):
        try:
            return self.opener.open(Request(self.base + path, headers=headers or {}, data=data), timeout=3)
        except HTTPError as response:
            return response

    def test_login_pkce_callback_and_one_use_state(self):
        with self.request("/login") as login:
            self.assertEqual(login.code, 303)
            params = parse_qs(urlparse(login.headers["Location"]).query)
            self.assertEqual(params["code_challenge_method"], ["S256"])
            state = params["state"][0]
            cookie = login.headers["Set-Cookie"].split(";", 1)[0]
        with patch("external_app.request_json", return_value={"token_type": "Bearer", "access_token": "server-only-token"}) as exchange:
            with self.request("/callback?state=" + state + "&code=code", headers={"Cookie": cookie}) as callback:
                self.assertEqual(callback.code, 303)
                self.assertIn("HttpOnly", callback.headers["Set-Cookie"])
                self.assertNotIn("server-only-token", callback.headers["Set-Cookie"])
            self.assertIn("code_verifier", exchange.call_args.kwargs["form"])
        self.client.access.assert_called_with("pilot", "server-only-token")
        with self.request("/callback?state=" + state + "&code=code", headers={"Cookie": cookie}) as replay:
            self.assertEqual(replay.code, 403)

    def test_mutations_require_origin_and_csrf(self):
        self.server.demo.sessions["session"] = {"token": "token", "subject": "user", "csrf": "expected"}
        with self.request("/logout", data=b"csrf=expected", headers={"Cookie": "pilot_session=session", "Origin": "https://attacker.test"}) as result:
            self.assertEqual(result.code, 403)
        with self.request("/logout", data=b"csrf=wrong", headers={"Cookie": "pilot_session=session", "Origin": self.base}) as result:
            self.assertEqual(result.code, 403)
        self.assertIn("session", self.server.demo.sessions)

    def test_current_access_failure_denies_existing_session(self):
        self.server.demo.sessions["session"] = {"token": "token", "subject": "user", "csrf": "expected"}
        self.client.access.side_effect = PlatformError("revoked")
        with self.request("/", headers={"Cookie": "pilot_session=session"}) as result:
            self.assertEqual(result.code, 403)
            self.assertNotIn(b"token", result.read())


if __name__ == "__main__":
    unittest.main()
