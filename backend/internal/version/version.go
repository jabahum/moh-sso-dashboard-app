package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

const defaultServiceName = "moh-sso-dashboard-backend"

// These values are intentionally variables so release builds can inject them
// with -ldflags -X. Application code should read them through Get.
var (
	Service   = defaultServiceName
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
	Dirty     = "unknown"
)

type BuildInfo struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	Dirty     bool   `json:"dirty"`
	GoVersion string `json:"goVersion"`
}

func Get() BuildInfo {
	dirty, dirtyExplicit := parseOptionalBool(Dirty)
	info := BuildInfo{
		Service:   valueOrDefault(Service, defaultServiceName),
		Version:   normalizeVersion(Version),
		Commit:    valueOrDefault(Commit, "none"),
		BuildTime: valueOrDefault(BuildTime, "unknown"),
		Dirty:     dirty,
		GoVersion: runtime.Version(),
	}

	applyVCSFallback(&info, !dirtyExplicit)
	return info
}

func String() string {
	return Get().String()
}

func IsDevelopment() bool {
	return Get().IsDevelopment()
}

func (info BuildInfo) String() string {
	return fmt.Sprintf(
		"%s %s (commit %s, built %s, dirty=%t, %s)",
		info.Service,
		info.Version,
		info.Commit,
		info.BuildTime,
		info.Dirty,
		info.GoVersion,
	)
}

func (info BuildInfo) IsDevelopment() bool {
	value := normalizeVersion(info.Version)
	return value == "dev" || strings.HasPrefix(value, "0.0.0-dev.")
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "backend/")
	value = strings.TrimPrefix(value, "v")
	if value == "" || value == "(devel)" {
		return "dev"
	}
	return value
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func parseBool(value string) bool {
	parsed, _ := parseOptionalBool(value)
	return parsed
}

func parseOptionalBool(value string) (bool, bool) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return parsed, err == nil
}

func applyVCSFallback(info *BuildInfo, deriveDirty bool) {
	if info == nil {
		return
	}

	build, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if info.Version == "dev" && build.Main.Version != "" && build.Main.Version != "(devel)" {
		info.Version = normalizeVersion(build.Main.Version)
	}

	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "none" && strings.TrimSpace(setting.Value) != "" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.BuildTime == "unknown" && strings.TrimSpace(setting.Value) != "" {
				info.BuildTime = setting.Value
			}
		case "vcs.modified":
			if deriveDirty && parseBool(setting.Value) {
				info.Dirty = true
			}
		}
	}
}
