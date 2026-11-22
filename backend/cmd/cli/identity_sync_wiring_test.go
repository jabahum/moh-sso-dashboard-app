package main

import (
	"context"
	"database/sql"
	"testing"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
)

func TestIdentitySyncParentHasEveryTrackSubcommand(t *testing.T) {
	cmd := newIdentitySyncCommand()
	counts := map[string]int{}
	for _, sub := range cmd.Commands() {
		counts[sub.Name()]++
	}
	for _, want := range []string{"register", "describe", "preview", "apply", "run", "revoke", "restore", "revocations", "conformance"} {
		if counts[want] != 1 {
			t.Errorf("subcommand %q attached %d times (have %v)", want, counts[want], counts)
		}
	}
}

func TestRevocationsFromRuntimeUsesRevocationTable(t *testing.T) {
	ctx := context.Background()
	if _, err := revocationsFromRuntime(ctx, nil); err == nil {
		t.Fatal("nil runtime must fail closed")
	}
	memory := &identitySyncRuntime{Registry: identityreconcile.NewRegistryService(&memoryRegistry{}, "https://sso.example.test/token")}
	if _, err := revocationsFromRuntime(ctx, memory); err == nil {
		t.Fatal("a registry without the dashboard database must fail closed")
	}
	db := &sql.DB{} // never used: only the wiring is checked here
	rt := &identitySyncRuntime{Registry: identityreconcile.NewRegistryService(&identityreconcile.PostgresRegistry{DB: db}, "https://sso.example.test/token")}
	checker, err := revocationsFromRuntime(ctx, rt)
	if err != nil {
		t.Fatal(err)
	}
	service, ok := checker.(*identityreconcile.RevokeService)
	if !ok {
		t.Fatalf("checker is %T, want *identityreconcile.RevokeService", checker)
	}
	repo, ok := service.Repository.(*identityreconcile.PostgresRevocations)
	if !ok || repo.DB != db {
		t.Fatalf("revocation repository is %T, want PostgresRevocations on the registry database", service.Repository)
	}
}
