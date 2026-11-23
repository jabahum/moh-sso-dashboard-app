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
//
// The returned *RevokeService also gives apply the per-(application, subject)
// access lock (PostgresRevocations' advisory lock) and, when the Keycloak
// operator settings are present, the full removal steps, so a revocation found
// right after a grant removes the role and ends sessions at once.
func revocationsFromRuntime(ctx context.Context, rt *identitySyncRuntime) (identityreconcile.RevocationChecker, error) {
	if rt == nil || rt.Registry == nil {
		return nil, errors.New("identity-sync revocations need the registry runtime")
	}
	registry, ok := rt.Registry.Repository.(*identityreconcile.PostgresRegistry)
	if !ok || registry == nil || registry.DB == nil {
		return nil, errors.New("identity-sync revocations need the dashboard database")
	}
	if rt.getenv != nil {
		if _, err := rt.Keycloak(); err == nil {
			if full, err := openIdentitySyncRevokeService(ctx, rt); err == nil {
				return full, nil
			}
		}
	}
	// Without operator settings a re-asserted revocation is recorded as pending
	// for 'identity-sync revocations --resume'.
	return &identityreconcile.RevokeService{Repository: &identityreconcile.PostgresRevocations{DB: registry.DB}}, nil
}
