package rbac

import (
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	"github.com/moh-sso-dashboard/module"
	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/semver"
)

const ManifestAPIVersion = "portal.moh.go.ug/v1alpha1"
const maxManifestBytes = 128 * 1024

type ApplicationManifest struct {
	APIVersion string           `json:"apiVersion" yaml:"apiVersion"`
	Kind       string           `json:"kind" yaml:"kind"`
	Metadata   ManifestMetadata `json:"metadata" yaml:"metadata"`
	Spec       ManifestSpec     `json:"spec" yaml:"spec"`
}

type ManifestMetadata struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type ManifestSpec struct {
	Environment string               `json:"environment" yaml:"environment"`
	Owner       ManifestOwner        `json:"owner" yaml:"owner"`
	SupportURL  string               `json:"supportUrl" yaml:"supportUrl"`
	Icon        string               `json:"icon,omitempty" yaml:"icon,omitempty"`
	Category    string               `json:"category,omitempty" yaml:"category,omitempty"`
	Launch      ManifestLaunch       `json:"launch" yaml:"launch"`
	Access      ManifestAccess       `json:"access" yaml:"access"`
	Navigation  []ManifestNavigation `json:"navigation,omitempty" yaml:"navigation,omitempty"`
}

type ManifestOwner struct {
	Team  string `json:"team" yaml:"team"`
	Name  string `json:"name" yaml:"name"`
	Email string `json:"email" yaml:"email"`
}

type ManifestLaunch struct {
	Mode     string `json:"mode" yaml:"mode"`
	URL      string `json:"url,omitempty" yaml:"url,omitempty"`
	Route    string `json:"route,omitempty" yaml:"route,omitempty"`
	Module   string `json:"module,omitempty" yaml:"module,omitempty"`
	AppName  string `json:"appName,omitempty" yaml:"appName,omitempty"`
	EntryURL string `json:"entryUrl,omitempty" yaml:"entryUrl,omitempty"`
}

// Roles are an any-of gate using this application's client ID. This maps to
// the existing system access-role model and does not assign roles to users.
type ManifestAccess struct {
	Roles []string `json:"roles" yaml:"roles"`
}

type ManifestNavigation struct {
	ID                  string   `json:"id" yaml:"id"`
	Label               string   `json:"label" yaml:"label"`
	Path                string   `json:"path" yaml:"path"`
	Icon                string   `json:"icon,omitempty" yaml:"icon,omitempty"`
	RequiredPermissions []string `json:"requiredPermissions,omitempty" yaml:"requiredPermissions,omitempty"`
}

type ManifestRequest struct {
	Document    string `json:"document" binding:"required"`
	ReviewToken string `json:"reviewToken,omitempty"`
}

var manifestID = regexp.MustCompile(`^[a-z][a-z0-9-]{1,99}$`)
var manifestRole = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]{0,99}$`)
var manifestVersionShape = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+([-+].*)?$`)
var manifestAppName = regexp.MustCompile(`^@[a-z0-9-]+/[a-z0-9-]+$`)

func parseApplicationManifest(document, environment string, modules []module.Info) (ApplicationManifest, UpsertSystemInput, error) {
	var manifest ApplicationManifest
	invalid := func(message string) (ApplicationManifest, UpsertSystemInput, error) {
		return manifest, UpsertSystemInput{}, fmt.Errorf("%w: %s", ErrInvalidInput, message)
	}
	if len(document) == 0 || len(document) > maxManifestBytes {
		return invalid("manifest must contain 1–131072 bytes")
	}
	decoder := yaml.NewDecoder(strings.NewReader(document))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return invalid("invalid manifest: " + err.Error())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return invalid("provide exactly one application manifest")
	}
	if manifest.APIVersion != ManifestAPIVersion || manifest.Kind != "Application" {
		return invalid("unsupported apiVersion or kind")
	}
	if !manifestID.MatchString(manifest.Metadata.ID) || strings.TrimSpace(manifest.Metadata.Name) == "" {
		return invalid("metadata requires a lowercase ID and a display name")
	}
	if !manifestVersionShape.MatchString(manifest.Metadata.Version) || !semver.IsValid("v"+strings.TrimPrefix(manifest.Metadata.Version, "v")) {
		return invalid("metadata.version requires a full semantic version")
	}
	if environment == "" || manifest.Spec.Environment != environment {
		return invalid("manifest environment must match this deployment")
	}
	if strings.TrimSpace(manifest.Spec.Owner.Team) == "" || strings.TrimSpace(manifest.Spec.Owner.Name) == "" {
		return invalid("owner team and name are required")
	}
	address, err := mail.ParseAddress(manifest.Spec.Owner.Email)
	if err != nil || address.Address != manifest.Spec.Owner.Email {
		return invalid("owner email must be an email address")
	}
	if err := manifestURL(manifest.Spec.SupportURL, environment); err != nil {
		return invalid("supportUrl: " + err.Error())
	}
	if len(manifest.Spec.Access.Roles) == 0 || len(manifest.Spec.Access.Roles) > 50 {
		return invalid("access.roles requires 1–50 client role names")
	}
	seenRoles := map[string]bool{}
	for _, role := range manifest.Spec.Access.Roles {
		if !manifestRole.MatchString(role) || seenRoles[role] {
			return invalid("access.roles contains an invalid or duplicate role")
		}
		seenRoles[role] = true
	}
	sort.Strings(manifest.Spec.Access.Roles)
	launch := manifest.Spec.Launch
	launcher, sideNav, enabled := true, len(manifest.Spec.Navigation) > 0, false
	input := UpsertSystemInput{ClientID: manifest.Metadata.ID, DisplayName: strings.TrimSpace(manifest.Metadata.Name), Description: manifest.Metadata.Description,
		Icon: manifest.Spec.Icon, Category: manifest.Spec.Category, OwnerTeam: manifest.Spec.Owner.Team, OwnerName: manifest.Spec.Owner.Name, OwnerEmail: manifest.Spec.Owner.Email,
		SupportURL: manifest.Spec.SupportURL, Environment: environment, SystemType: "platform", IntegrationType: "hosted_module", ContractVersion: "1", LaunchMode: "internal",
		DisplayInLauncher: &launcher, DisplayInSideNav: &sideNav, Enabled: &enabled, LifecycleManaged: true}
	switch launch.Mode {
	case "external":
		if launch.Route != "" || launch.Module != "" || launch.EntryURL != "" || launch.AppName != "" || sideNav {
			return invalid("external launch accepts a URL and no portal navigation or module fields")
		}
		if err := manifestURL(launch.URL, environment); err != nil {
			return invalid("launch.url: " + err.Error())
		}
		input.SystemType, input.IntegrationType, input.LaunchMode, input.LaunchURL = "external", "external", "new_tab", launch.URL
	case "internal":
		if launch.URL != "" || launch.EntryURL != "" || launch.AppName != "" || !manifestPortalPath(launch.Route) {
			return invalid("internal launch requires a portal route and a compiled module")
		}
		available := false
		for _, item := range modules {
			if item.ID == launch.Module && item.Enabled {
				available = true
			}
		}
		if !available {
			return invalid("launch.module must identify an enabled compiled backend module")
		}
		input.LaunchURL = launch.Route
	case "microfrontend":
		if launch.URL != "" || launch.Module != "" || launch.Route != "/apps/extensions/"+manifest.Metadata.ID || !manifestAppName.MatchString(launch.AppName) || strings.HasPrefix(launch.AppName, "@moh-sso/") {
			return invalid("microfrontend requires a unique scoped appName and route /apps/extensions/<metadata.id>")
		}
		if err := manifestURL(launch.EntryURL, environment); err != nil {
			return invalid("launch.entryUrl: " + err.Error())
		}
		input.LaunchURL = launch.Route
	default:
		return invalid("launch.mode must be external, internal or microfrontend")
	}
	if len(manifest.Spec.Navigation) > 50 {
		return invalid("navigation is limited to 50 entries")
	}
	if len(manifest.Spec.Navigation) == 0 {
		manifest.Spec.Navigation = nil
	}
	for index := range manifest.Spec.Navigation {
		if len(manifest.Spec.Navigation[index].RequiredPermissions) == 0 {
			manifest.Spec.Navigation[index].RequiredPermissions = nil
		}
	}
	for _, item := range manifest.Spec.Navigation {
		if !manifestPortalPath(item.Path) {
			return invalid("navigation paths must be canonical portal paths")
		}
		if launch.Mode == "microfrontend" && item.Path != launch.Route && !strings.HasPrefix(item.Path, launch.Route+"/") {
			return invalid("microfrontend navigation must remain within its application route")
		}
	}
	if sideNav {
		raw, _ := json.Marshal(manifest.Spec.Navigation)
		input.Navigation = string(raw)
	}
	behavior := systemrbac.SeedSystem{SystemType: input.SystemType, LaunchURL: input.LaunchURL, LaunchMode: input.LaunchMode, Navigation: input.Navigation, DisplayInLauncher: &launcher, DisplayInSideNav: &sideNav}
	if err := systemrbac.ValidateSystemBehavior(behavior); err != nil {
		return invalid(err.Error())
	}
	return manifest, input, nil
}

func manifestPortalPath(value string) bool {
	return value != "" && (strings.HasPrefix(value, "/apps/") || strings.HasPrefix(value, "/portal/")) && path.Clean(value) == value && !strings.ContainsAny(value, "%?#\\\r\n\t ")
}

func manifestURL(value, environment string) error {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || strings.ContainsAny(value, "*\\\r\n\t ") {
		return fmt.Errorf("use an absolute HTTPS URL without credentials, query or fragment")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (environment == "development" || environment == "test") && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return nil
	}
	return fmt.Errorf("HTTPS is required, with loopback HTTP allowed for development and test")
}
