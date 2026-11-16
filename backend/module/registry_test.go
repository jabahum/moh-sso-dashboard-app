package module_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/moh-sso-dashboard/module"
)

func TestCompositionRejectsInvalidDependencies(t *testing.T) {
	a := module.Definition{Info: module.Info{ID: "a", Version: "dev", Enabled: true}}
	b := module.Definition{Info: module.Info{ID: "b", Version: "dev", Enabled: true}, Requires: []string{"a"}}
	disabled := a
	disabled.Enabled = false
	for _, definitions := range [][]module.Definition{{a, a}, {b, a}, {disabled, b}, {{Info: module.Info{ID: "bad id", Version: "dev"}}}, {{Info: module.Info{ID: "a"}}}} {
		if _, err := module.New(definitions...); err == nil {
			t.Fatalf("accepted invalid composition: %+v", definitions)
		}
	}
	if _, err := module.New(a, b); err != nil {
		t.Fatal(err)
	}
}

func TestPartialInitializationRollsBackAndCancels(t *testing.T) {
	var events []string
	var lifetime context.Context
	failure := errors.New("initialization failed")
	definition := func(id string, fail bool) module.Definition {
		return module.Definition{
			Info: module.Info{ID: id, Version: "dev", Enabled: true},
			Initialize: func(ctx context.Context) (module.Stop, error) {
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
	r, err := module.New(definition("a", false), definition("b", true), definition("c", false))
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

func TestDisabledModuleAndIdempotentShutdown(t *testing.T) {
	initializations, registrations, stops := 0, 0, 0
	definition := module.Definition{
		Info: module.Info{ID: "enabled", Version: "dev", Enabled: true},
		Initialize: func(context.Context) (module.Stop, error) {
			initializations++
			return func(context.Context) error { stops++; return nil }, nil
		},
		Register: func(module.Routes) { registrations++ },
	}
	disabled := definition
	disabled.ID, disabled.Enabled = "disabled", false
	r, err := module.New(definition, disabled)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := r.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r.RegisterRoutes(module.Routes{})
	if err := stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if initializations != 1 || registrations != 1 || stops != 1 {
		t.Fatalf("incorrect lifecycle counts: %d %d %d", initializations, registrations, stops)
	}
	items := r.Inventory()
	items[0].ID = "mutated"
	if r.Inventory()[0].ID != "enabled" || len(items) != 2 || items[1].Enabled {
		t.Fatal("invalid inventory or mutable inventory leaked")
	}
}
