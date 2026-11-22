package main

import (
	"context"
	"errors"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
)

// Integration wiring for the identity-sync tracks: it attaches the conformance
// command (Track D) to the identity-sync parent (Track A) and makes the batch
// run (Track B) consult the access-removal revocations (Track C).

func init() {
	registerIdentitySyncSubcommand(newIdentitySyncConformanceCommand)
	identitySyncRevocations = revocationsFromRuntime
}

// revocationsFromRuntime returns Track C's revocation list on the registry's
// database. It fails closed: without the database there is no batch session,
// rather than a run that silently treats every identity as not revoked.
func revocationsFromRuntime(_ context.Context, rt *identitySyncRuntime) (identityreconcile.RevocationChecker, error) {
	if rt == nil || rt.Registry == nil {
		return nil, errors.New("identity-sync revocations need the registry runtime")
	}
	registry, ok := rt.Registry.Repository.(*identityreconcile.PostgresRegistry)
	if !ok || registry == nil || registry.DB == nil {
		return nil, errors.New("identity-sync revocations need the dashboard database")
	}
	return &identityreconcile.RevokeService{Repository: &identityreconcile.PostgresRevocations{DB: registry.DB}}, nil
}
