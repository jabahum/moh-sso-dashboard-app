// Package module adapts compiled platform modules to the host's Gin routes.
// Portable lifecycle and inventory contracts live in core.
package module

import (
	"context"
	"io/fs"

	"github.com/gin-gonic/gin"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// Info retains the host API schema name while core owns its field contract.
type Info core.ModuleInfo
type Stop = core.Stop

// Routes contains host groups. Public has no authentication; its routes must
// validate their own access tokens and apply appropriate rate limits.
// Modules must apply their resource permissions to the routes they register.
type Routes struct {
	Public    *gin.RouterGroup
	Protected *gin.RouterGroup
	Admin     *gin.RouterGroup
}

// Definition receives domain dependencies through constructor closures, keeping
// this public API independent of the host's internal implementation types.
type Definition struct {
	Info
	Requires []string
	// Initialize returns after readiness. Its cleanup must also be returned on
	// partial failure. Workers must observe ctx and cleanup must wait for them.
	Initialize func(context.Context) (Stop, error)
	Register   func(Routes)
	// Migrations optionally holds the module's own schema migrations; see
	// core.ValidateModuleMigrations. They run before any module initializes.
	Migrations fs.FS
}

// Registry combines the portable runtime with host-owned route registration.
type Registry struct {
	runtime     *core.Runtime
	definitions []Definition
}

func New(definitions ...Definition) (*Registry, error) {
	lifecycle := make([]core.ModuleDefinition, len(definitions))
	for i, definition := range definitions {
		lifecycle[i] = core.ModuleDefinition{ModuleInfo: core.ModuleInfo(definition.Info), Requires: definition.Requires, Initialize: definition.Initialize, Migrations: definition.Migrations}
	}
	runtime, err := core.NewRuntime(lifecycle...)
	if err != nil {
		return nil, err
	}
	return &Registry{runtime: runtime, definitions: append([]Definition(nil), definitions...)}, nil
}

func (r *Registry) Inventory() []Info {
	items := r.runtime.Inventory()
	out := make([]Info, len(items))
	for i, item := range items {
		out[i] = Info(item)
	}
	return out
}
func (r *Registry) Initialize(ctx context.Context) (Stop, error) { return r.runtime.Initialize(ctx) }

// UseMigrator supplies the host migrator that applies module migration
// streams during Initialize.
func (r *Registry) UseMigrator(m core.Migrator) { r.runtime.UseMigrator(m) }

// MigrationStreams lists enabled modules' migration streams in order.
func (r *Registry) MigrationStreams() []core.MigrationStream { return r.runtime.MigrationStreams() }

func (r *Registry) RegisterRoutes(routes Routes) {
	for _, definition := range r.definitions {
		if definition.Enabled && definition.Register != nil {
			definition.Register(routes)
		}
	}
}
