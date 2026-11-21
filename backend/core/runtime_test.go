package core_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

func TestCompositionRejectsInvalidDependencies(t *testing.T) {
	a := core.ModuleDefinition{ModuleInfo: core.ModuleInfo{ID: "a", Version: "dev", Enabled: true}}
	b := core.ModuleDefinition{ModuleInfo: core.ModuleInfo{ID: "b", Version: "dev", Enabled: true}, Requires: []string{"a"}}
	disabled := a
	disabled.Enabled = false
	for _, definitions := range [][]core.ModuleDefinition{{a, a}, {b, a}, {disabled, b}, {{ModuleInfo: core.ModuleInfo{ID: "bad id", Version: "dev"}}}, {{ModuleInfo: core.ModuleInfo{ID: "a"}}}} {
		if _, err := core.NewRuntime(definitions...); err == nil {
			t.Fatalf("accepted invalid composition: %+v", definitions)
		}
	}
	if _, err := core.NewRuntime(a, b); err != nil {
		t.Fatal(err)
	}
}

func TestPartialInitializationRollsBackAndCancels(t *testing.T) {
	var events []string
	var lifetime context.Context
	failure := errors.New("initialization failed")
	definition := func(id string, fail bool) core.ModuleDefinition {
		return core.ModuleDefinition{
			ModuleInfo: core.ModuleInfo{ID: id, Version: "dev", Enabled: true},
			Initialize: func(ctx context.Context) (core.Stop, error) {
				lifetime = ctx
				events = append(events, "init-"+id)
				cleanup := func(ctx context.Context) error {
					if ctx.Err() != nil {
						t.Error("rollback context already cancelled")
					}
					events = append(events, "stop-"+id)
					return nil
				}
				if fail {
					return cleanup, failure
				}
				return cleanup, nil
			},
		}
	}
	r, err := core.NewRuntime(definition("a", false), definition("b", true), definition("c", false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Initialize(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("expected initialization failure, got %v", err)
	}
	if lifetime.Err() == nil || !reflect.DeepEqual(events, []string{"init-a", "init-b", "stop-b", "stop-a"}) {
		t.Fatalf("rollback failed: %v, context: %v", events, lifetime.Err())
	}
	if _, err := r.Initialize(context.Background()); err == nil {
		t.Fatal("failed registry accepted a retry")
	}
}

func TestDisabledModulesAreSkippedAndInventoryIsDetached(t *testing.T) {
	starts, stops := 0, 0
	enabled := core.ModuleDefinition{ModuleInfo: core.ModuleInfo{ID: "enabled", Version: "dev", Enabled: true},
		Initialize: func(context.Context) (core.Stop, error) {
			starts++
			return func(context.Context) error { stops++; return nil }, nil
		}}
	disabled := enabled
	disabled.ID = "disabled"
	disabled.Enabled = false
	definitions := []core.ModuleDefinition{enabled, disabled}
	runtime, err := core.NewRuntime(definitions...)
	if err != nil {
		t.Fatal(err)
	}
	definitions[0].Enabled = false
	stop, err := runtime.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || stops != 1 {
		t.Fatalf("lifecycle counts: %d %d", starts, stops)
	}
	inventory := runtime.Inventory()
	inventory[0].ID = "mutated"
	if runtime.Inventory()[0].ID != "enabled" || len(inventory) != 2 || inventory[1].Enabled {
		t.Fatal("inventory aliases runtime state")
	}
}
