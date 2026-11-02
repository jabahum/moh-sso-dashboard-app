"""Small standard-library reference client; contains no portal implementation imports."""
import json
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener


class PlatformError(Exception):
    pass


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def safe_url(value):
    parsed = urlparse(value)
    local = parsed.hostname in ("localhost", "127.0.0.1", "::1")
    if not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise PlatformError("invalid configured URL")
    if parsed.scheme != "https" and not (local and parsed.scheme == "http"):
        raise PlatformError("HTTPS is required outside loopback development")
    return value.rstrip("/")


def request_json(url, *, token=None, payload=None, form=None, headers=None):
    safe_url(url)
    request_headers = {"Accept": "application/json", **(headers or {})}
    if token:
        request_headers["Authorization"] = "Bearer " + token
    data = None
    if payload is not None:
        data = json.dumps(payload).encode()
        request_headers["Content-Type"] = "application/json"
    if form is not None:
        data = urlencode(form).encode()
        request_headers["Content-Type"] = "application/x-www-form-urlencoded"
    req = Request(url, data=data, headers=request_headers)
    try:
        with build_opener(NoRedirect()).open(req, timeout=5) as response:
            body = response.read(1048577)
            if len(body) > 1048576:
                raise PlatformError("response too large")
            result = json.loads(body)
    except HTTPError as error:
        # Do not print provider response bodies: they may contain sensitive data.
        raise PlatformError("remote request rejected (HTTP %d)" % error.code) from None
    except (URLError, TimeoutError, ValueError):
        raise PlatformError("remote service unavailable or invalid response") from None
    if not isinstance(result, dict):
        raise PlatformError("invalid response envelope")
    return result


class PlatformClient:
    def __init__(self, base_url):
        self.base = safe_url(base_url)

    def call(self, path, token, payload=None, key=None):
        result = request_json(self.base + path, token=token, payload=payload,
                              headers={"Idempotency-Key": key} if key else None)
        if not result.get("success") or not isinstance(result.get("data"), dict):
            raise PlatformError("platform request did not return success")
        return result["data"]

    def identity(self, token):
        return self.call("/platform/identity", token)

    def access(self, application_id, token):
        from urllib.parse import quote
        return self.call("/platform/applications/" + quote(application_id, safe="") + "/access", token)

    def notify(self, token, key, recipient, title, message):
        return self.call("/platform/notifications", token,
                         {"title": title, "message": message, "recipient": {"email": recipient}}, key)

    def status(self, token, notification_id):
        from uuid import UUID
        return self.call("/platform/notifications/" + str(UUID(notification_id)), token)


def discover(issuer):
    issuer = safe_url(issuer)
    metadata = request_json(issuer + "/.well-known/openid-configuration")
    if metadata.get("issuer") != issuer:
        raise PlatformError("discovery issuer mismatch")
    for key in ("authorization_endpoint", "token_endpoint", "end_session_endpoint"):
        if key not in metadata:
            raise PlatformError("discovery is missing " + key)
        safe_url(metadata[key])
        if urlparse(metadata[key]).netloc != urlparse(issuer).netloc:
            raise PlatformError("reference example requires same-origin identity endpoints")
    return metadata


def machine_token(token_endpoint, client_id, secret, scope="identity:read"):
    result = request_json(token_endpoint, form={"grant_type": "client_credentials",
                          "client_id": client_id, "client_secret": secret,
                          "scope": scope})
    if str(result.get("token_type", "")).lower() != "bearer" or not result.get("access_token"):
        raise PlatformError("token endpoint did not return a bearer access token")
    return result["access_token"]
