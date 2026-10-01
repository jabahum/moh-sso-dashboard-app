package config

import (
	"strings"
	"testing"
)

func TestDevAuthBypassEnvironmentGuard(t *testing.T) {
	var missing *Config
	if missing.DevAuthBypassEnabled() {
		t.Fatal("nil configuration must not enable authentication bypass")
	}
	for _, environment := range []string{"development", "dev", "local", " Development "} {
		cfg := &Config{Environment: environment, DevAuthBypass: true}
		if !cfg.DevAuthBypassEnabled() {
			t.Errorf("expected explicit local bypass for %q", environment)
		}
		cfg.DevAuthBypass = false
		if cfg.DevAuthBypassEnabled() {
			t.Errorf("bypass must require opt-in for %q", environment)
		}
	}
	for _, environment := range []string{"", "production", "prod", "staging", "stage", "test", "unknown"} {
		cfg := &Config{Environment: environment, DevAuthBypass: true}
		if cfg.DevAuthBypassEnabled() {
			t.Errorf("unsafe bypass enabled for %q", environment)
		}
	}
}

func TestValidateConfigRejectsDeployedAuthBypassBeforeDependencies(t *testing.T) {
	for _, environment := range []string{"production", "prod", "staging", "stage", "test"} {
		err := validateConfig(&Config{Environment: environment, DevAuthBypass: true})
		if err == nil || !strings.Contains(err.Error(), "DEV_AUTH_BYPASS") {
			t.Errorf("expected bypass validation failure for %q, got %v", environment, err)
		}
	}
}
