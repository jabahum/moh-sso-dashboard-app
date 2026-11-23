package identityreconcile

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// The revoke-versus-grant interleaving on the real store: a revocation started
// between single apply's revocation check and its grant must still leave the
// person without the access role.
func TestIdentitySyncGrantRevokeInterleavingPostgres(t *testing.T) {
	db := registryTestDatabase(t)
	ctx := context.Background()
	for _, name := range []string{
		"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql",
		"000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql",
		"000038_add_rbac_groups.up.sql", "000048_application_registry_lifecycle.up.sql",
		"000049_application_identities.up.sql", "000061_service_only_system_roles.up.sql",
		"000062_identity_sync_integrations.up.sql", "000064_identity_sync_revocations.up.sql",
	} {
		data, err := os.ReadFile(filepath.Join("..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, query := range []string{
		`INSERT INTO ihp_systems (client_id, display_name) VALUES ('alerts', 'Alerts')`,
		`INSERT INTO ihp_system_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`,
		`INSERT INTO ihp_system_access_roles (system_id, role_name) SELECT id, 'alerts_access' FROM ihp_systems WHERE client_id = 'alerts'`,
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_application_identities (oauth_client_id, application_client_id, subject, enabled) VALUES ('alerts-identity-sync', 'alerts', $1, TRUE)`, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ihp_identity_sync_integrations (application_client_id, origin, sync_client_id, access_role, credential_ref, account_id_format, enabled)
		VALUES ('alerts', 'https://alerts.example.test', 'alerts-identity-sync', 'alerts_access', 'env:ALERTS_SYNC_CLIENT_SECRET', 'opaque', TRUE)`); err != nil {
		t.Fatal(err)
	}

	identity := identitysync.Identity{Issuer: batchIssuer, Subject: uuid.NewString()}
	roles := newRoleState()
	revoker := raceRevokeService(batchIssuer, roles, &PostgresRevocations{DB: db})
	beforeGrant, wait := interleaveRevoke(t, revoker, identity.Subject)
	accounts := &linkRecorder{account: identitysync.Account{ID: "acct-1", Email: "single@example.org"}}
	service := Service{
		Accounts:    accounts,
		Directory:   &interleavingServiceDirectory{identity: identity, roles: roles, beforeGrant: beforeGrant},
		Issuer:      batchIssuer,
		Application: "alerts",
		Revocations: revoker,
	}
	_ = service.Apply(ctx, Decision{Account: accounts.account, Action: Reuse, Identity: identity}, accounts.account)
	revocation := wait()
	if roles.has(identity.Subject) {
		t.Fatal("the grant undid a revocation that ran during the apply")
	}
	if revocation.RoleRemoved != StepDone || !revocation.Complete() {
		t.Fatalf("revocation = %+v", revocation)
	}
}

// Proves PostgresRevocations.WithAccessLock serialises holders of the same
// (application, subject) across separate connections, and leaves other
// subjects and applications alone. Needs MOH_REGISTRY_TEST_DATABASE_URL.
func TestIdentitySyncAccessLockPostgres(t *testing.T) {
	db := registryTestDatabase(t)
	var locker AccessLocker = &PostgresRevocations{DB: db}
	ctx := context.Background()

	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- locker.WithAccessLock(ctx, "alerts", "subject-1", func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	// Another subject and another application are not blocked.
	for _, key := range [][2]string{{"alerts", "subject-2"}, {"clims", "subject-1"}} {
		done := make(chan error, 1)
		go func() { done <- locker.WithAccessLock(ctx, key[0], key[1], func(context.Context) error { return nil }) }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s/%s was blocked by the lock on alerts/subject-1", key[0], key[1])
		}
	}

	// The same key waits until the first holder releases.
	var secondRan atomic.Bool
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- locker.WithAccessLock(ctx, "alerts", "subject-1", func(context.Context) error {
			secondRan.Store(true)
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)
	if secondRan.Load() {
		t.Fatal("a second holder entered while the first held the lock")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-secondDone:
		if err != nil || !secondRan.Load() {
			t.Fatalf("second holder: ran=%v err=%v", secondRan.Load(), err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the lock was not released when the first holder returned")
	}

	// A cancelled wait gives up instead of blocking forever.
	hold := make(chan struct{})
	held := make(chan struct{})
	go func() {
		_ = locker.WithAccessLock(ctx, "alerts", "subject-3", func(context.Context) error {
			close(held)
			<-hold
			return nil
		})
	}()
	<-held
	waitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if err := locker.WithAccessLock(waitCtx, "alerts", "subject-3", func(context.Context) error {
		t.Error("entered a held lock")
		return nil
	}); err == nil {
		t.Fatal("a cancelled wait for the lock returned no error")
	}
	close(hold)
}
