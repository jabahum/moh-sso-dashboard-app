package core_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

func TestConcurrentShutdownCancelsBeforeReverseCleanupAndRetainsAllErrors(t *testing.T) {
	var lifetime context.Context
	var stopped atomic.Int32
	var order []string
	first, second := errors.New("first cleanup failed"), errors.New("second cleanup failed")
	definition := func(id string, cause error) core.ModuleDefinition {
		return core.ModuleDefinition{ModuleInfo: core.ModuleInfo{ID: id, Version: "dev", Enabled: true},
			Initialize: func(ctx context.Context) (core.Stop, error) {
				lifetime = ctx
				return func(context.Context) error {
					if lifetime.Err() == nil {
						t.Error("cleanup ran before module context cancellation")
					}
					stopped.Add(1)
					order = append(order, id)
					return cause
				}, nil
			}}
	}
	runtime, err := core.NewRuntime(definition("first", first), definition("second", second))
	if err != nil {
		t.Fatal(err)
	}
	stop, err := runtime.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			err := stop(context.Background())
			if !errors.Is(err, first) || !errors.Is(err, second) {
				t.Errorf("cleanup causes lost: %v", err)
			}
		}()
	}
	group.Wait()
	if stopped.Load() != 2 || !reflect.DeepEqual(order, []string{"second", "first"}) {
		t.Fatalf("duplicate or unordered cleanup: %v", order)
	}
}

func TestCleanupReceivesShutdownCancellation(t *testing.T) {
	runtime, err := core.NewRuntime(core.ModuleDefinition{ModuleInfo: core.ModuleInfo{ID: "worker", Version: "dev", Enabled: true},
		Initialize: func(context.Context) (core.Stop, error) {
			return func(ctx context.Context) error { return ctx.Err() }, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	stop, err := runtime.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := stop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown cancellation lost: %v", err)
	}
}
