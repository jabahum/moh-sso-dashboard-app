package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	router "github.com/moh-sso-dashboard/internal/api"
)

func TestModuleCatalogMatchesCompiledInventory(t *testing.T) {
	path := filepath.Join(repoRoot(t), "..", "docs", "module-catalog.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Backend []struct {
			ID       string `json:"id"`
			Stage    string `json:"stage"`
			ModuleID string `json:"moduleId"`
		} `json:"backend"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	registry, err := router.NewModuleRegistry(router.HandlerSet{}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{}
	for _, feature := range catalog.Backend {
		if feature.Stage == "published" || feature.Stage == "registry-hosted" {
			if feature.ModuleID == "" || expected[feature.ModuleID] {
				t.Fatalf("missing or duplicate module ID for %s", feature.ID)
			}
			expected[feature.ModuleID] = true
		}
	}
	if len(expected) != len(registry.Inventory()) {
		t.Fatalf("catalogue and compiled inventory differ: %v vs %v", expected, registry.Inventory())
	}
	for _, info := range registry.Inventory() {
		if !expected[info.ID] {
			t.Errorf("compiled module %s is not tracked in the catalogue", info.ID)
		}
	}
}
