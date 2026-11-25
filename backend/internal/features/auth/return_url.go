package auth

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

const (
	cookieOAuthReturnTo = "oauth_return_to"
	oauthFlowCookieTTL  = 300
)

var errInvalidReturnURL = errors.New("invalid returnTo URL")

func (h *Handler) normalizeReturnURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	lower := strings.ToLower(raw)
	if strings.ContainsAny(raw, "\\\r\n\x00") || strings.HasPrefix(raw, "//") ||
		strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || strings.Contains(lower, "%0d") ||
		strings.Contains(lower, "%0a") || strings.Contains(lower, "%00") ||
		strings.Contains(lower, "%25") {
		return "", errInvalidReturnURL
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Opaque != "" {
		return "", errInvalidReturnURL
	}
	for _, segment := range strings.Split(parsed.EscapedPath(), "/") {
		decoded, err := url.PathUnescape(segment)
		if err != nil || decoded == "." || decoded == ".." {
			return "", errInvalidReturnURL
		}
	}

	parsed.Path = stripLegacyPortalPrefix(parsed.Path)
	parsed.RawPath = ""
	if !isPortalPath(parsed.Path) {
		return "", errInvalidReturnURL
	}

	frontend, err := url.Parse(h.config.FrontendBaseURL)
	if err != nil || frontend.Scheme == "" || frontend.Host == "" {
		return "", errInvalidReturnURL
	}

	if parsed.IsAbs() {
		if !h.isAllowedReturnOrigin(parsed) {
			return "", errInvalidReturnURL
		}
		return parsed.String(), nil
	}

	if parsed.Host != "" || parsed.Scheme != "" {
		return "", errInvalidReturnURL
	}

	return (&url.URL{
		Scheme:   frontend.Scheme,
		Host:     frontend.Host,
		Path:     parsed.Path,
		RawQuery: parsed.RawQuery,
		Fragment: parsed.Fragment,
	}).String(), nil
}

// backendPathPrefixes are served by the API on the same host, so a login must
// never land on them (for example a GET to a logout or launch endpoint).
var backendPathPrefixes = []string{"/api", "/ssobackend", "/backend", "/swagger", "/health", "/version"}

// isPortalPath accepts root-relative portal routes. The portal is served at the
// host root, so any absolute path is a portal route unless the API owns it.
func isPortalPath(path string) bool {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return false
	}
	for _, prefix := range backendPathPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	return true
}

// stripLegacyPortalPrefix maps links from before the portal moved to the host
// root (/portal, /portal/x) onto their current paths (/, /x).
func stripLegacyPortalPrefix(path string) string {
	if path == "/portal" {
		return "/"
	}
	if strings.HasPrefix(path, "/portal/") {
		return strings.TrimPrefix(path, "/portal")
	}
	return path
}

func (h *Handler) isAllowedReturnOrigin(target *url.URL) bool {
	allowed := make(map[string]struct{})
	if frontend, err := url.Parse(h.config.FrontendBaseURL); err == nil {
		allowed[strings.ToLower(frontend.Scheme+"://"+frontend.Host)] = struct{}{}
	}
	for _, rawOrigin := range strings.Split(h.config.AuthReturnURLAllowedOrigins, ",") {
		origin, err := url.Parse(strings.TrimSpace(rawOrigin))
		if err == nil && origin.Scheme != "" && origin.Host != "" {
			allowed[strings.ToLower(origin.Scheme+"://"+origin.Host)] = struct{}{}
		}
	}

	_, ok := allowed[strings.ToLower(target.Scheme+"://"+target.Host)]
	return ok
}

func encodeReturnCookie(state, target string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(state + "\n" + target))
}

func decodeReturnCookie(value, expectedState string) string {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(decoded), "\n", 2)
	if len(parts) != 2 || parts[0] != expectedState {
		return ""
	}
	return parts[1]
}
