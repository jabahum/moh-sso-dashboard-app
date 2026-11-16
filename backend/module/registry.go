// Package module defines the public contract for compiled dashboard modules.
// Construct a registry at startup, initialize it once, then register its routes.
package module

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Info struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

// Routes contains groups with the host's authentication, audit and rate limits.
// Modules must apply their resource permissions to the routes they register.
type Routes struct {
	Protected *gin.RouterGroup
	Admin     *gin.RouterGroup
}

type Stop func(context.Context) error

// Definition receives domain dependencies through constructor closures, keeping
// this public API independent of the host's internal implementation types.
type Definition struct {
	Info
	Requires []string
	// Initialize returns after readiness. Its cleanup must also be returned on
	// partial failure. Workers must observe ctx and cleanup must wait for them.
	Initialize func(context.Context) (Stop, error)
	Register   func(Routes)
}

type Registry struct {
	definitions []Definition
	mu          sync.Mutex
	initialized bool
}

var validID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// New validates explicit composition order before any module has side effects.
// Required modules must precede their consumers and be enabled when used.
func New(definitions ...Definition) (*Registry, error) {
	seen := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		if !validID.MatchString(definition.ID) || definition.Version == "" {
			return nil, fmt.Errorf("module ID and version required: %q", definition.ID)
		}
		if _, exists := seen[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate module %q", definition.ID)
		}
		for _, required := range definition.Requires {
			enabled, exists := seen[required]
			if !exists || (definition.Enabled && !enabled) {
				return nil, fmt.Errorf("module %q requires preceding enabled module %q", definition.ID, required)
			}
		}
		seen[definition.ID] = definition.Enabled
	}
	return &Registry{definitions: append([]Definition(nil), definitions...)}, nil
}

func (r *Registry) Inventory() []Info {
	items := make([]Info, 0, len(r.definitions))
	for _, definition := range r.definitions {
		items = append(items, definition.Info)
	}
	return items
}

// Initialize runs once. Failure cancels module contexts and rolls back acquired
// resources in reverse order. A failed registry must be discarded.
func (r *Registry) Initialize(ctx context.Context) (Stop, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initialized {
		return nil, errors.New("module registry already initialized")
	}
	r.initialized = true
	moduleCtx, cancel := context.WithCancel(ctx)
	var cleanups []Stop
	var once sync.Once
	var stopErr error
	stop := func(ctx context.Context) error {
		once.Do(func() {
			cancel()
			for i := len(cleanups) - 1; i >= 0; i-- {
				stopErr = errors.Join(stopErr, cleanups[i](ctx))
			}
		})
		return stopErr
	}
	for _, definition := range r.definitions {
		if !definition.Enabled || definition.Initialize == nil {
			continue
		}
		cleanup, err := definition.Initialize(moduleCtx)
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
		if err != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer rollbackCancel()
			return nil, errors.Join(fmt.Errorf("initialize module %q: %w", definition.ID, err), stop(rollbackCtx))
		}
	}
	return stop, nil
}

func (r *Registry) RegisterRoutes(routes Routes) {
	for _, definition := range r.definitions {
		if definition.Enabled && definition.Register != nil {
			definition.Register(routes)
		}
	}
}
