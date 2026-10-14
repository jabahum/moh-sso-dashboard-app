package version

import (
	"strings"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := map[string]string{
		"1.2.3":          "1.2.3",
		"v1.2.3":         "1.2.3",
		"backend/v1.2.3": "1.2.3",
		"":               "dev",
		"(devel)":        "dev",
	}

	for input, expected := range tests {
		if actual := normalizeVersion(input); actual != expected {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestGetInjectedBuildInfo(t *testing.T) {
	restore := setBuildVariables("backend/v1.2.3", "abc123", "2026-06-19T12:00:00Z", "true")
	defer restore()

	info := Get()
	if info.Version != "1.2.3" {
		t.Fatalf("Version = %q, want 1.2.3", info.Version)
	}
	if info.Commit != "abc123" || info.BuildTime != "2026-06-19T12:00:00Z" || !info.Dirty {
		t.Fatalf("unexpected build info: %+v", info)
	}
	if info.GoVersion == "" {
		t.Fatal("GoVersion must not be empty")
	}
	if strings.Contains(info.String(), "vv1.2.3") {
		t.Fatalf("version was prefixed twice: %s", info.String())
	}
}

func TestDevelopmentBuild(t *testing.T) {
	restore := setBuildVariables("dev", "none", "unknown", "false")
	defer restore()

	info := Get()
	if info.Version != "dev" {
		t.Fatalf("Version = %q, want dev", info.Version)
	}
	if !info.IsDevelopment() || !IsDevelopment() {
		t.Fatal("development build was not detected")
	}
	if !strings.Contains(String(), defaultServiceName+" dev") {
		t.Fatalf("unexpected version string: %s", String())
	}
}

func TestParseBool(t *testing.T) {
	if !parseBool("true") || !parseBool("1") {
		t.Fatal("truthy dirty values were not parsed")
	}
	if parseBool("invalid") || parseBool("") {
		t.Fatal("invalid dirty values must be false")
	}
}

func TestDevelopmentPrerelease(t *testing.T) {
	for _, value := range []string{"dev", "0.0.0-dev.abc123", "backend/v0.0.0-dev.abc123"} {
		if !(BuildInfo{Version: value}).IsDevelopment() {
			t.Fatalf("%q must be detected as development", value)
		}
	}
	for _, value := range []string{"0.0.1", "1.2.3", "1.2.3-rc.1"} {
		if (BuildInfo{Version: value}).IsDevelopment() {
			t.Fatalf("%q must not be detected as development", value)
		}
	}
}

func setBuildVariables(version, commit, buildTime, dirty string) func() {
	oldService, oldVersion, oldCommit := Service, Version, Commit
	oldBuildTime, oldDirty := BuildTime, Dirty
	Service = defaultServiceName
	Version = version
	Commit = commit
	BuildTime = buildTime
	Dirty = dirty
	return func() {
		Service = oldService
		Version = oldVersion
		Commit = oldCommit
		BuildTime = oldBuildTime
		Dirty = oldDirty
	}
}
