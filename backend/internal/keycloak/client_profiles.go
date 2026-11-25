package keycloak

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/moh-sso-dashboard/internal/model"
)

//go:embed client-profiles.json identity-environments.json
var profileFiles embed.FS

type ClientProfile struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Settings model.Client `json:"settings"`
}

// OnboardingSpec is a portable, credential-free application configuration.
// Environment URLs must be reviewed again when importing into another deployment.
type OnboardingSpec struct {
	Version      int      `json:"version"`
	ClientID     string   `json:"clientId"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Owner        string   `json:"owner"`
	SupportEmail string   `json:"supportEmail"`
	Profile      string   `json:"profile"`
	Environment  string   `json:"environment"`
	RootURL      string   `json:"rootUrl"`
	RedirectURIs []string `json:"redirectUris"`
	LogoutURIs   []string `json:"logoutUris"`
	WebOrigins   []string `json:"webOrigins"`
	Launcher     bool     `json:"launcher"`
}

func ClientProfiles() []ClientProfile {
	data, _ := profileFiles.ReadFile("client-profiles.json")
	var profiles []ClientProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		panic(err)
	}
	return profiles
}

func BuildOnboardingClient(spec OnboardingSpec, environment string) (*model.Client, error) {
	if spec.Version != 1 {
		return nil, fmt.Errorf("unsupported onboarding document version")
	}
	if environment == "" || spec.Environment != environment {
		return nil, fmt.Errorf("select this deployment's environment before reviewing")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,99}$`).MatchString(spec.ClientID) {
		return nil, fmt.Errorf("client ID must be 2–100 lowercase letters, digits or dashes")
	}
	if strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.Owner) == "" {
		return nil, fmt.Errorf("application name and owner are required")
	}
	email := strings.Split(spec.SupportEmail, "@")
	if len(email) != 2 || email[0] == "" || !strings.Contains(email[1], ".") {
		return nil, fmt.Errorf("a support email is required")
	}
	var client *model.Client
	for _, profile := range ClientProfiles() {
		if profile.ID == spec.Profile {
			copy := profile.Settings
			client = &copy
			break
		}
	}
	if client == nil {
		return nil, fmt.Errorf("unknown client profile")
	}
	validateURL := func(raw string, origin bool) error {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "*\\\r\n ") || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("URLs must be absolute HTTP(S) URLs without wildcards, credentials or fragments")
		}
		if origin && (u.Path != "" || u.RawQuery != "") {
			return fmt.Errorf("web origins must contain only scheme, host and optional port")
		}
		if u.Scheme == "http" && ((environment != "development" && environment != "test") || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1")) {
			return fmt.Errorf("HTTP is allowed only for loopback URLs in development or test")
		}
		return nil
	}
	if spec.Profile != "service" {
		if len(spec.RedirectURIs) == 0 || len(spec.LogoutURIs) == 0 || len(spec.WebOrigins) == 0 {
			return nil, fmt.Errorf("web applications require exact callback, logout and origin URLs")
		}
		if err := validateURL(spec.RootURL, false); err != nil {
			return nil, err
		}
	} else if spec.Launcher || spec.RootURL != "" || len(spec.RedirectURIs)+len(spec.LogoutURIs)+len(spec.WebOrigins) > 0 {
		return nil, fmt.Errorf("service integrations do not use browser URLs or launcher entries")
	}
	for _, values := range [][]string{spec.RedirectURIs, spec.LogoutURIs} {
		for _, value := range values {
			if err := validateURL(value, false); err != nil {
				return nil, err
			}
		}
	}
	for _, value := range spec.WebOrigins {
		if err := validateURL(value, true); err != nil {
			return nil, err
		}
	}
	client.ClientID = spec.ClientID
	client.Name = strings.TrimSpace(spec.Name)
	client.Description = spec.Description
	client.RootURL = spec.RootURL
	client.BaseURL = spec.RootURL
	client.RedirectUris = spec.RedirectURIs
	client.WebOrigins = spec.WebOrigins
	if client.RedirectUris == nil {
		client.RedirectUris = []string{}
	}
	if client.WebOrigins == nil {
		client.WebOrigins = []string{}
	}
	client.Attributes["ui.launchUrl"] = ""
	client.Attributes["ui.home"] = ""
	client.Attributes["portal.onboarding.profile"] = spec.Profile
	client.Attributes["portal.onboarding.environment"] = environment
	client.Attributes["portal.owner"] = spec.Owner
	client.Attributes["portal.supportEmail"] = spec.SupportEmail
	if len(spec.LogoutURIs) > 0 {
		client.Attributes["post.logout.redirect.uris"] = strings.Join(spec.LogoutURIs, "##")
	}
	if spec.Launcher {
		client.Attributes["portal.system"] = "true"
		client.Attributes["ui.systemType"] = "external"
		client.Attributes["ui.launchMode"] = "new_tab"
		client.Attributes["ui.launchUrl"] = spec.RootURL
		client.Attributes["ui.displayInLauncher"] = "true"
	}
	return client, nil
}

// ValidateIdentityEnvironment prevents a deployment administering another environment's realm.
// override, when set, names this environment's realm instead of the default in
// identity-environments.json; it may not be another environment's default realm.
func ValidateIdentityEnvironment(environment, realm, override string) error {
	data, _ := profileFiles.ReadFile("identity-environments.json")
	var realms map[string]string
	if err := json.Unmarshal(data, &realms); err != nil {
		return err
	}
	expected := realms[environment]
	if expected == "" {
		return fmt.Errorf("identity environment/realm mismatch: configure this deployment's isolated realm")
	}
	if override = strings.TrimSpace(override); override != "" {
		for other, otherRealm := range realms {
			if other != environment && otherRealm == override {
				return fmt.Errorf("IDENTITY_ENVIRONMENT_REALM %q belongs to the %s environment", override, other)
			}
		}
		expected = override
	}
	if expected != realm {
		return fmt.Errorf("identity environment/realm mismatch: configure this deployment's isolated realm")
	}
	return nil
}
