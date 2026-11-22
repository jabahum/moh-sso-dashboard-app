package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// ---------------------------------------------------------------------------
// Parent command and subcommand registration
// ---------------------------------------------------------------------------

var (
	identitySyncSubcommandsMu sync.Mutex
	identitySyncSubcommands   []func() *cobra.Command
)

// registerIdentitySyncSubcommand adds a subcommand constructor to every parent
// built by newIdentitySyncCommand. Other tracks call it from an init function in
// their own file (for example identity_sync_run.go) so they never edit this one.
func registerIdentitySyncSubcommand(constructor func() *cobra.Command) {
	identitySyncSubcommandsMu.Lock()
	defer identitySyncSubcommandsMu.Unlock()
	identitySyncSubcommands = append(identitySyncSubcommands, constructor)
}

// newIdentitySyncCommand builds the "identity-sync" parent command with the
// registry-based register, describe, preview and apply subcommands plus every
// subcommand registered through registerIdentitySyncSubcommand. Callers may add
// more subcommands with AddCommand.
func newIdentitySyncCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity-sync",
		Short: "Link registered applications' local accounts to Keycloak identities",
		Long: "Registry-based identity sync. Each application is registered once (origin, sync client,\n" +
			"access role and a credential reference); preview and apply then work for any application\n" +
			"that implements the identity-sync contract, with no dashboard code change.",
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newIdentitySyncRegisterCommand(), newIdentitySyncDescribeCommand(),
		newIdentitySyncPreviewCommand(), newIdentitySyncApplyCommand())
	identitySyncSubcommandsMu.Lock()
	constructors := append([]func() *cobra.Command(nil), identitySyncSubcommands...)
	identitySyncSubcommandsMu.Unlock()
	for _, constructor := range constructors {
		cmd.AddCommand(constructor())
	}
	return cmd
}

// ---------------------------------------------------------------------------
// Runtime shared by registry-based subcommands
// ---------------------------------------------------------------------------

// identitySyncRuntime gives subcommands the registry, the Keycloak settings and
// helpers to build a connector and review service for a registered application.
type identitySyncRuntime struct {
	Registry *identityreconcile.RegistryService
	getenv   func(string) string
	close    func()
}

// Close releases the registry's database connection.
func (rt *identitySyncRuntime) Close() {
	if rt != nil && rt.close != nil {
		rt.close()
	}
}

// Keycloak returns the operator credentials, public issuer and token endpoint.
func (rt *identitySyncRuntime) Keycloak() (identitySyncConfig, error) {
	return keycloakSyncConfigFromEnv(rt.getenv)
}

// Connector returns the contract connector and registry row for an enabled application.
func (rt *identitySyncRuntime) Connector(ctx context.Context, app string) (*identityreconcile.HTTPAccounts, identityreconcile.Integration, error) {
	return rt.Registry.Connector(ctx, app)
}

// Service builds the review service for an enabled application: its connector,
// a Keycloak directory granting the registered access role, and the public issuer.
func (rt *identitySyncRuntime) Service(ctx context.Context, app string) (identityreconcile.Service, *identityreconcile.HTTPAccounts, identityreconcile.Integration, error) {
	kc, err := rt.Keycloak()
	if err != nil {
		return identityreconcile.Service{}, nil, identityreconcile.Integration{}, err
	}
	connector, integration, err := rt.Connector(ctx, app)
	if err != nil {
		return identityreconcile.Service{}, nil, identityreconcile.Integration{}, err
	}
	directory := applicationDirectory{
		admin:      keycloak.NewAdminClient(kc.adminBase, kc.realm, kc.adminID, kc.adminSecret),
		issuer:     kc.issuer,
		clientID:   integration.ApplicationClientID,
		accessRole: integration.AccessRole,
	}
	return identityreconcile.Service{Accounts: connector, Directory: directory, Issuer: kc.issuer}, connector, integration, nil
}

// tokenEndpointFromEnv returns the Keycloak client-credentials endpoint
// (through KEYCLOAK_BASE_URL, which may be internal).
func tokenEndpointFromEnv(getenv func(string) string) (string, error) {
	base := strings.TrimRight(getenv("KEYCLOAK_BASE_URL"), "/")
	realm := getenv("KEYCLOAK_REALM")
	if !httpsBaseURL(base) || realm == "" || url.PathEscape(realm) != realm {
		return "", errors.New("configure the HTTPS Keycloak base URL and realm")
	}
	return base + "/realms/" + realm + "/protocol/openid-connect/token", nil
}

// openIdentitySyncDB opens the dashboard database from the usual app.env /
// environment configuration. Tests replace it.
var openIdentitySyncDB = func(ctx context.Context) (*sql.DB, error) {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		return nil, err
	}
	if cfg.DBUser == "" {
		return nil, errors.New("DB_USER is not configured")
	}
	return openToolDB(ctx, cfg)
}

// openIdentitySyncRuntime connects to the registry. Tests replace it.
var openIdentitySyncRuntime = func(ctx context.Context) (*identitySyncRuntime, error) {
	endpoint, err := tokenEndpointFromEnv(os.Getenv)
	if err != nil {
		return nil, err
	}
	db, err := openIdentitySyncDB(ctx)
	if err != nil {
		return nil, fmt.Errorf("open the dashboard database: %w", err)
	}
	return &identitySyncRuntime{
		Registry: identityreconcile.NewRegistryService(&identityreconcile.PostgresRegistry{DB: db}, endpoint),
		getenv:   os.Getenv,
		close:    func() { db.Close() },
	}, nil
}

// registeredIdentitySyncService is used by the deprecated aliases: it returns a
// registry-based service only when the application has an enabled registry
// row. Any failure (no database, no row, disabled row) means "not registered",
// and the alias keeps its environment-based behaviour.
func registeredIdentitySyncService(ctx context.Context, app string) (identityreconcile.Service, string, bool) {
	lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rt, err := openIdentitySyncRuntime(lookupCtx)
	if err != nil {
		return identityreconcile.Service{}, "", false
	}
	integration, err := rt.Registry.Get(lookupCtx, app)
	if err != nil || !integration.Enabled {
		rt.Close()
		return identityreconcile.Service{}, "", false
	}
	service, _, _, err := rt.Service(ctx, app)
	if err != nil {
		rt.Close()
		return identityreconcile.Service{}, "", false
	}
	// The process exits after the command; the connection is released then.
	return service, service.Issuer, true
}

func identitySyncActor() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return "cli:" + u.Username
	}
	return "cli"
}

// ---------------------------------------------------------------------------
// Subcommands
// ---------------------------------------------------------------------------

func newIdentitySyncRegisterCommand() *cobra.Command {
	var input identityreconcile.Integration
	var format, reason string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register or update an application in the identity-sync registry",
		Long: "Stores the application's origin, dedicated sync client, access role and a credential\n" +
			"reference (env:NAME or file:/absolute/path; never the secret). With --enable the\n" +
			"application's GET / descriptor is read first (a 1.0 application answers 404).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			input.AccountIDFormat = identitysync.AccountIDFormat(format)
			if strings.TrimSpace(reason) == "" {
				return errors.New("--reason is required")
			}
			if err := input.Validate(); err != nil {
				return err
			}
			rt, err := openIdentitySyncRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.Close()
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			saved, err := rt.Registry.Save(ctx, input, identitySyncActor(), reason)
			if err != nil {
				return err
			}
			return writeJSON(cmd, saved)
		},
	}
	f := cmd.Flags()
	f.StringVar(&input.ApplicationClientID, "app", "", "Application Keycloak client ID (required)")
	f.StringVar(&input.Origin, "origin", "", "Application HTTPS origin, https://host[:port] only (required)")
	f.StringVar(&input.SyncClientID, "sync-client", "", "Dedicated identity-sync Keycloak client ID (required)")
	f.StringVar(&input.AccessRole, "access-role", "", "Application access role granted after linking (required)")
	f.StringVar(&input.CredentialRef, "credential", "", "Sync client secret reference: env:NAME or file:/absolute/path (required)")
	f.StringVar(&format, "account-id-format", string(identitysync.AccountIDOpaque), "Account ID format for a 1.0 application: uuid, positive-integer or opaque")
	f.BoolVar(&input.Enabled, "enable", false, "Enable the integration after reading its descriptor")
	f.StringVar(&reason, "reason", "", "Why this change is made, recorded in the audit log (required)")
	for _, name := range []string{"app", "origin", "sync-client", "access-role", "credential", "reason"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}

func newIdentitySyncDescribeCommand() *cobra.Command {
	var app string
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Show an application's registry entry and its live GET / descriptor",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := openIdentitySyncRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.Close()
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			integration, err := rt.Registry.Get(ctx, app)
			if err != nil {
				return err
			}
			result := struct {
				Integration     identityreconcile.Integration `json:"integration"`
				Descriptor      *identitysync.Descriptor      `json:"descriptor,omitempty"`
				DescriptorError string                        `json:"descriptorError,omitempty"`
			}{Integration: integration}
			descriptor, probeErr := rt.Registry.Probe(ctx, integration)
			if probeErr != nil {
				result.DescriptorError = probeErr.Error()
			} else {
				result.Descriptor = &descriptor
			}
			if err := writeJSON(cmd, result); err != nil {
				return err
			}
			if probeErr != nil {
				return errors.New("the application could not be described")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Application Keycloak client ID (required)")
	_ = cmd.MarkFlagRequired("app")
	return cmd
}

func newIdentitySyncPreviewCommand() *cobra.Command {
	var app, cursor string
	var limit int
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview identity matches for one page of a registered application's accounts (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := openIdentitySyncRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.Close()
			service, _, _, err := rt.Service(cmd.Context(), app)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), app+" identity links use issuer "+service.Issuer)
			return runPagePreviewOrApply(cmd, app, service, cursor, limit, "", "")
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Application Keycloak client ID (required)")
	cmd.Flags().StringVar(&cursor, "after", "", "Resume from a preview cursor")
	cmd.Flags().IntVar(&limit, "limit", 25, "Accounts to preview in the requested page (1-100)")
	_ = cmd.MarkFlagRequired("app")
	return cmd
}

// maxApplyPages bounds the account walk apply performs to find one account.
const maxApplyPages = 1000

func newIdentitySyncApplyCommand() *cobra.Command {
	var app, accountID, subject, cursor string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply one reviewed existing-user link and grant the application's access role",
		Long: "Finds the account among the application's eligible accounts, re-checks that the directory\n" +
			"still matches exactly the approved Keycloak subject, links it and grants the access role.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(subject) == "" || subject != strings.TrimSpace(subject) {
				return errors.New("--approved-subject must be a Keycloak user ID")
			}
			rt, err := openIdentitySyncRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.Close()
			service, connector, _, err := rt.Service(cmd.Context(), app)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), app+" identity links use issuer "+service.Issuer)
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			account, err := findEligibleAccount(ctx, connector, cursor, accountID)
			if err != nil {
				return err
			}
			reviewed := identityreconcile.Decision{
				Account:  account,
				Action:   identityreconcile.Reuse,
				Identity: identitysync.Identity{Issuer: service.Issuer, Subject: subject},
			}
			if err := service.Apply(ctx, reviewed, account); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), app+" identity linked and access granted")
			return nil
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Application Keycloak client ID (required)")
	cmd.Flags().StringVar(&accountID, "account", "", "Reviewed application account ID (required)")
	cmd.Flags().StringVar(&subject, "approved-subject", "", "Reviewed Keycloak subject for this account (required)")
	cmd.Flags().StringVar(&cursor, "after", "", "Start the account search after this preview cursor")
	for _, name := range []string{"app", "account", "approved-subject"} {
		_ = cmd.MarkFlagRequired(name)
	}
	return cmd
}

// findEligibleAccount walks the application's eligible accounts from cursor
// until it finds accountID.
func findEligibleAccount(ctx context.Context, accounts identityreconcile.Accounts, cursor, accountID string) (identitysync.Account, error) {
	for range maxApplyPages {
		page, err := accounts.ListEligibleAccounts(ctx, cursor, identitysync.MaxPageSize)
		if err != nil {
			return identitysync.Account{}, err
		}
		for _, account := range page.Accounts {
			if account.ID == accountID {
				return account, nil
			}
		}
		if page.NextCursor == "" {
			return identitysync.Account{}, errors.New("approved account is not eligible")
		}
		if page.NextCursor == cursor {
			return identitysync.Account{}, errors.New("application returned a repeating cursor")
		}
		cursor = page.NextCursor
	}
	return identitysync.Account{}, fmt.Errorf("approved account not found within %d pages; pass --after to narrow the search", maxApplyPages)
}

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
