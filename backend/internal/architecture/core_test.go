package architecture

import (
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The trusted portable core must not depend on host adapters, providers,
// frameworks or optional features. Apply the boundary to every Go file,
// including public contract tests and future subpackages.
func TestPlatformCoreUsesOnlyStandardLibraryImports(t *testing.T) {
	root := filepath.Join(repoRoot(t), "core")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if name == "github.com/mohuganda/moh-sso-dashboard/backend/core" || strings.HasPrefix(name, "github.com/mohuganda/moh-sso-dashboard/backend/core/") {
				continue
			}
			dependency, err := build.Default.Import(name, "", build.FindOnly)
			if err != nil || !dependency.Goroot {
				t.Errorf("%s imports non-standard core dependency %q", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHTTPModuleAdapterDoesNotImportHostFeaturesOrPublishedApplications(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(repoRoot(t), "module", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(name, "github.com/moh-sso-dashboard/internal/") ||
				strings.HasPrefix(name, "github.com/mohuganda/moh-sso-dashboard/backend/modules/") {
				t.Errorf("module route adapter %s depends on feature %q; inject it through host composition", path, name)
			}
		}
	}
}

// The shared registrar may consume trusted platform identity/trace packages;
// it must never grow a module-specific implementation or import.
func TestSharedHTTPRegistrarDoesNotDependOnApplications(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(repoRoot(t), "internal", "modulehttp", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(name, "github.com/mohuganda/moh-sso-dashboard/backend/modules/") ||
				strings.HasPrefix(name, "github.com/moh-sso-dashboard/internal/adapters/modules/") ||
				strings.HasPrefix(name, "github.com/moh-sso-dashboard/internal/features/") {
				t.Errorf("shared registrar imports application implementation %q", name)
			}
		}
	}
}
