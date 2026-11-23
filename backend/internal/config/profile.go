package config

import (
	"fmt"
	"strings"
)

const ProfileFull = "full"
const ProfileCore = "core"

// OptionalModuleIDs lists the currently packaged applications. Legacy feature
// switches are not inferred from this inventory.
func OptionalModuleIDs() []string {
	return []string{"metrics", "audit", "geojson", "report-scheduler", "announcements", "admin-units", "storage-locations", "visualiser", "document-templates", "data-quality", "documents", "surveillance"}
}
func (c *Config) CoreOnly() bool { return c != nil && c.PlatformProfile == ProfileCore }
func (c *Config) EffectiveDisabledModules() string {
	if c.CoreOnly() {
		return strings.Join(OptionalModuleIDs(), ",")
	}
	if c == nil {
		return ""
	}
	return c.DisabledModules
}
func (c *Config) ModuleEnabled(id string) bool {
	known := false
	for _, candidate := range OptionalModuleIDs() {
		if candidate == id {
			known = true
			break
		}
	}
	if !known || c.CoreOnly() {
		return false
	}
	for _, disabled := range strings.Split(c.EffectiveDisabledModules(), ",") {
		if strings.TrimSpace(disabled) == id {
			return false
		}
	}
	return true
}
func validateProfile(c *Config) error {
	if c.PlatformProfile != "" && c.PlatformProfile != ProfileFull && c.PlatformProfile != ProfileCore {
		return fmt.Errorf("PLATFORM_PROFILE must be full or core")
	}
	for _, id := range strings.Split(c.DisabledModules, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		known := false
		for _, candidate := range OptionalModuleIDs() {
			if id == candidate {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("unknown disabled module %q", id)
		}
	}
	return nil
}
