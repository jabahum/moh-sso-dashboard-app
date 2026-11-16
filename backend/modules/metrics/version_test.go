package metrics

import (
	"runtime/debug"
	"testing"
)

func TestVersionUsesSelectedDependencyAndIdentifiesLocalSource(t *testing.T) {
	for _, tc := range []struct {
		dependency *debug.Module
		want       string
	}{
		{&debug.Module{Path: ModulePath, Version: "v0.1.0"}, "v0.1.0"},
		{&debug.Module{Path: ModulePath, Version: "v0.1.0", Replace: &debug.Module{Path: "./modules/metrics"}}, "dev"},
		{&debug.Module{Path: ModulePath, Version: "v0.1.0", Replace: &debug.Module{Path: ModulePath, Version: "v0.2.0"}}, "v0.2.0"},
		{&debug.Module{Path: "unrelated", Version: "v9.0.0"}, "dev"},
	} {
		if got := versionFromBuild(&debug.BuildInfo{Deps: []*debug.Module{tc.dependency}}); got != tc.want {
			t.Fatalf("expected %s, got %s", tc.want, got)
		}
	}
}
