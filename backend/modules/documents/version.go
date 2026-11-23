package documents

import "runtime/debug"

const ModulePath = "github.com/mohuganda/moh-sso-dashboard/backend/modules/documents"

// Version reports the module version selected by the consuming Go build.
// Workspace replacements report dev because their contents can differ from a tag.
func Version() string {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return versionFromBuild(build)
}

func versionFromBuild(build *debug.BuildInfo) string {
	for _, dependency := range build.Deps {
		if dependency.Path == ModulePath {
			if dependency.Replace != nil {
				if dependency.Replace.Version != "" && dependency.Replace.Version != "(devel)" {
					return dependency.Replace.Version
				}
				return "dev"
			}
			if dependency.Version == "" || dependency.Version == "(devel)" {
				return "dev"
			}
			return dependency.Version
		}
	}
	return "dev"
}
