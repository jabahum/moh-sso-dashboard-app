package config

import (
	"testing"

	"github.com/spf13/viper"
)

// Pushing the RBAC registry to Keycloak creates clients and roles and rewrites
// client attributes, so it must be opt-in: unset means off, as in every shipped
// infra/env template.
func TestRBACStartupPushToKeycloakIsOptIn(t *testing.T) {
	for _, tc := range []struct {
		env  string
		set  bool
		want bool
	}{
		{want: false},
		{env: "true", set: true, want: true},
		{env: "false", set: true, want: false},
	} {
		viper.Reset()
		t.Cleanup(viper.Reset)
		if tc.set {
			t.Setenv("RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK", tc.env)
		}
		setDefaults()
		viper.AutomaticEnv()
		for key, env := range envBindings() {
			if err := viper.BindEnv(key, env); err != nil {
				t.Fatal(err)
			}
		}
		var c Config
		if err := viper.Unmarshal(&c); err != nil {
			t.Fatal(err)
		}
		if c.RBACStartupSyncPushToKeycloak != tc.want {
			t.Fatalf("RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=%q (set=%t) gave %t, want %t", tc.env, tc.set, c.RBACStartupSyncPushToKeycloak, tc.want)
		}
		// The rest of the startup-sync posture keeps its compiled defaults.
		if !c.RBACStartupSyncEnabled || !c.RBACStartupSyncLiveKeycloak || !c.RBACStartupSyncUsers || c.RBACStartupSyncFailOnError {
			t.Fatal("other startup sync defaults changed")
		}
	}
}
