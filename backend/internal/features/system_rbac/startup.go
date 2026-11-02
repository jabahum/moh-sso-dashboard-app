package system_rbac

import "context"

type startupDiscoveryKey struct{}

// WithStartupDiscovery makes automatic discovery/seed runs insert-only for system
// configuration. Explicit administrator sync/import keeps its existing semantics.
// Role and permission discovery continues for both existing and new systems.
func WithStartupDiscovery(ctx context.Context) context.Context {
	return context.WithValue(ctx, startupDiscoveryKey{}, true)
}
func IsStartupDiscovery(ctx context.Context) bool {
	value, _ := ctx.Value(startupDiscoveryKey{}).(bool)
	return value
}
