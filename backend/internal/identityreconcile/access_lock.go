package identityreconcile

import (
	"context"
	"errors"
)

// Serialising grants with revocations.
//
// A grant checks the revocation list and then assigns the access role; a
// revocation records itself and then removes the role. Without coordination a
// revocation that lands between a grant's check and its role assignment is
// undone by that grant. Both paths therefore take one lock per
// (application, subject):
//
//   - batch apply and single apply hold it across "check revoked -> grant";
//   - RevokeService holds it across "record revocation -> remove role" (and
//     across the role step of resume and re-assertion).
//
// Either the grant finishes first and the revocation's role removal runs after
// it, or the revocation commits first and the grant's check sees it.
// PostgresRevocations implements the lock with a transaction-scoped advisory
// lock. As a second line of defence, apply asks a RevocationEnforcer after the
// grant whether a revocation appeared anyway (for example one written by a
// path that does not take the lock) and, if so, removes the role again.

// AccessLocker serialises grant and revocation for one (application, subject).
// fn runs while the lock is held; the lock is released when it returns. fn
// must not take the same lock again (with Postgres that would wait on itself).
type AccessLocker interface {
	WithAccessLock(ctx context.Context, application, subject string, fn func(context.Context) error) error
}

// RevocationEnforcer re-asserts an active revocation after a grant: it records
// the role removal as pending again and, when it can, removes the role at once.
// enforced reports that an active revocation exists. *RevokeService implements it.
type RevocationEnforcer interface {
	EnforceRevocation(ctx context.Context, application, issuer, subject, actor string) (enforced bool, err error)
}

// accessLockerOf returns the lock a revocation checker offers, or nil.
func accessLockerOf(checker RevocationChecker) AccessLocker {
	if locker, ok := checker.(AccessLocker); ok {
		return locker
	}
	return nil
}

// enforcerOf returns the enforcer a revocation checker offers, or nil.
func enforcerOf(checker RevocationChecker) RevocationEnforcer {
	if enforcer, ok := checker.(RevocationEnforcer); ok {
		return enforcer
	}
	return nil
}

// withAccessLock runs fn under locker, or directly when locker is nil.
func withAccessLock(ctx context.Context, locker AccessLocker, application, subject string, fn func(context.Context) error) error {
	if locker == nil {
		return fn(ctx)
	}
	return locker.WithAccessLock(ctx, application, subject, fn)
}

// ErrRevokedDuringApply reports that a revocation was found right after a grant;
// the role was removed again (or its removal recorded as pending).
var ErrRevokedDuringApply = errors.New("identity access was revoked while access was being granted; the access role was removed again")
