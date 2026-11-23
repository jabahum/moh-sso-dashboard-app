// Package accesswarnings carries non-fatal warnings from access changes (for
// example an identity-sync revocation that could not end every session) back
// to the HTTP handler that made the change, without changing service method
// signatures. A handler puts a Collector in the request context; services add
// warnings to it. Every function is nil-safe.
package accesswarnings

import (
	"context"
	"sync"
)

// Stable warning codes.
const (
	// CodeRevocationIncomplete: the role was removed but a revocation step
	// (consents, application sessions) did not finish; resume retries it.
	CodeRevocationIncomplete = "identity_sync_revocation_incomplete"
	// CodeSessionsNotEnded: the application (contract 1.0) cannot end sessions.
	CodeSessionsNotEnded = "identity_sync_sessions_not_ended"
	// CodeRestoreFailed: a portal re-grant could not record the restore of an
	// active revocation.
	CodeRestoreFailed = "identity_sync_restore_failed"
)

// Warning is one non-fatal problem reported with a successful change.
type Warning struct {
	Code     string `json:"code"`
	ClientID string `json:"clientId,omitempty"`
	UserID   string `json:"userId,omitempty"`
	Message  string `json:"message"`
}

// Collector gathers warnings for one request.
type Collector struct {
	mu       sync.Mutex
	warnings []Warning
}

type contextKey struct{}

// WithCollector returns a context carrying a new collector.
func WithCollector(ctx context.Context) (context.Context, *Collector) {
	c := &Collector{}
	return context.WithValue(ctx, contextKey{}, c), c
}

// From returns the collector in ctx, or nil.
func From(ctx context.Context) *Collector {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(contextKey{}).(*Collector)
	return c
}

// Add records w in ctx's collector. It reports whether a collector was present.
func Add(ctx context.Context, w Warning) bool {
	c := From(ctx)
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.warnings = append(c.warnings, w)
	return true
}

// Warnings returns a copy of the collected warnings (nil when none).
func (c *Collector) Warnings() []Warning {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.warnings) == 0 {
		return nil
	}
	return append([]Warning(nil), c.warnings...)
}
