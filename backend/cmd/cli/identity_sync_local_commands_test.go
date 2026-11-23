package main

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
)

// Database-only identity-sync subcommands must not demand Keycloak admin
// credentials from the root PersistentPreRunE.
func TestDatabaseOnlyIdentitySyncCommandsNeedNoKeycloakAdmin(t *testing.T) {
	root := &cobra.Command{Use: "moh-sso"}
	root.AddCommand(newIdentitySyncCommand())
	for args, wantLocal := range map[string]bool{
		"run show":             true,
		"run approve":          true,
		"run reject":           true,
		"revocations":          true,
		"revocations --resume": false,
		"run start":            false,
		"run apply":            false,
		"run resume":           false,
		"revoke":               false,
		"restore":              false,
	} {
		fields := append([]string{"identity-sync"}, strings.Fields(args)...)
		sub, rest, err := root.Find(fields)
		if err != nil {
			t.Fatal(err)
		}
		if err := sub.ParseFlags(rest); err != nil {
			t.Fatal(err)
		}
		if got := isLocalToolCommand(sub); got != wantLocal {
			t.Errorf("identity-sync %s: local tool = %v, want %v", args, got, wantLocal)
		}
	}
}

// Listing revocations reads only the database: it works with no Keycloak
// settings at all.
func TestRevocationListingNeedsNoKeycloak(t *testing.T) {
	for _, env := range []string{"KEYCLOAK_BASE_URL", "KEYCLOAK_REALM", "KEYCLOAK_ADMIN_CLIENT_ID", "KEYCLOAK_ADMIN_CLIENT_SECRET"} {
		t.Setenv(env, "")
	}
	repo := &cliRevocations{rows: []identityreconcile.Revocation{{ID: "rev-1", ApplicationClientID: "alerts", Subject: cliRevokeSubject,
		RoleRemoved: identityreconcile.StepDone, ConsentsRevoked: identityreconcile.StepDone, SessionsRevoked: identityreconcile.StepFailed}}}
	previous := openIdentitySyncRevocationList
	openIdentitySyncRevocationList = func(context.Context) (*identityreconcile.RevokeService, func(), error) {
		return &identityreconcile.RevokeService{Repository: repo}, func() {}, nil
	}
	t.Cleanup(func() { openIdentitySyncRevocationList = previous })
	stdout, _, err := runCommand(t, newIdentitySyncCommand(), "revocations", "--app", "alerts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"id": "rev-1"`) {
		t.Fatalf("stdout = %s", stdout)
	}
}
