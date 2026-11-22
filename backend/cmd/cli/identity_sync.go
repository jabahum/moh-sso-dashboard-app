package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

type applicationDirectory struct {
	admin    *keycloak.KeyAdminClient
	issuer   string
	clientID string
	// accessRole is the application's Keycloak client role granted after linking.
	accessRole string
}

func (d applicationDirectory) FindEmail(ctx context.Context, email string) ([]identitysync.Identity, error) {
	query := url.Values{"email": {email}, "exact": {"true"}, "max": {"2"}}
	resp, err := d.admin.GetWithContext(ctx, "users?"+query.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Keycloak user lookup: HTTP %d", resp.StatusCode)
	}
	var users []keycloak.UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	matches := make([]identitysync.Identity, 0, len(users))
	for _, user := range users {
		if !strings.EqualFold(user.Email, email) {
			return nil, errors.New("Keycloak returned an unexpected email match")
		}
		var identity identitysync.Identity
		if user.Enabled && user.EmailVerified {
			identity = identitysync.Identity{Issuer: d.issuer, Subject: user.ID}
		}
		matches = append(matches, identity)
	}
	return matches, nil
}

func (applicationDirectory) EnsureUser(context.Context, identitysync.Account) (identitysync.Identity, error) {
	return identitysync.Identity{}, errors.New("provision new Keycloak users separately before applying identity links")
}

func (d applicationDirectory) GrantAccess(ctx context.Context, identity identitysync.Identity) error {
	if d.accessRole == "" {
		return errors.New("application access role is not configured")
	}
	client, err := d.admin.GetClientByClientID(d.clientID)
	if err != nil {
		return err
	}
	if !client.Enabled || client.ClientID != d.clientID {
		return errors.New("application Keycloak client is unavailable")
	}
	return d.admin.AssignClientRolesToUser(ctx, identity.Subject, d.clientID, client.ID, []string{d.accessRole})
}

// newPOESSyncCommand is the deprecated alias for "identity-sync preview|apply --app poesgo".
func newPOESSyncCommand() *cobra.Command {
	return newLegacyIdentitySyncCommand("poes-identity-sync", "POES", "poesgo", identityreconcile.NewPOES)
}

// newCLIMSSyncCommand is the deprecated alias for "identity-sync preview|apply --app clims".
func newCLIMSSyncCommand() *cobra.Command {
	return newLegacyIdentitySyncCommand("clims-identity-sync", "CLIMS", "clims", identityreconcile.NewCLIMS)
}

// newLegacyIdentitySyncCommand keeps the per-application commands working for
// one release. When the application has an enabled registry row it uses the
// registry; otherwise it uses the <APP>_SYNC_* environment configuration exactly
// as before.
func newLegacyIdentitySyncCommand(name, app, clientID string, connect func(string, identityreconcile.AccessToken) (*identityreconcile.HTTPAccounts, error)) *cobra.Command {
	var cursor, accountID, subject string
	var limit int
	cmd := &cobra.Command{
		Use:   name,
		Short: "Deprecated: use identity-sync preview|apply --app " + clientID,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			printDeprecation(cmd.ErrOrStderr(), name, clientID)
			if (accountID == "") != (subject == "") {
				return errors.New("apply requires both --apply-account and --approved-subject")
			}
			if service, issuer, ok := registeredIdentitySyncService(cmd.Context(), clientID); ok {
				fmt.Fprintln(cmd.ErrOrStderr(), app+" identity sync uses the registry entry for "+clientID)
				fmt.Fprintln(cmd.ErrOrStderr(), app+" identity links use issuer "+issuer)
				return runPagePreviewOrApply(cmd, app, service, cursor, limit, accountID, subject)
			}
			cfg, err := identitySyncConfigFromEnv(app, os.Getenv)
			if err != nil {
				return err
			}
			token, err := identitySyncToken(app, cfg.tokenEndpoint, cfg.syncClientID, cfg.syncClientSecret, cfg.staticToken)
			if err != nil {
				return err
			}
			accounts, err := connect(cfg.origin, token)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), app+" identity links use issuer "+cfg.issuer)
			directory := applicationDirectory{admin: keycloak.NewAdminClient(cfg.adminBase, cfg.realm, cfg.adminID, cfg.adminSecret), issuer: cfg.issuer, clientID: clientID, accessRole: clientID + "_access"}
			service := identityreconcile.Service{Accounts: accounts, Directory: directory, Issuer: cfg.issuer}
			return runPagePreviewOrApply(cmd, app, service, cursor, limit, accountID, subject)
		},
	}
	cmd.Flags().StringVar(&cursor, "after", "", "Resume from a preview cursor")
	cmd.Flags().IntVar(&limit, "limit", 25, "Accounts to preview in the requested page")
	cmd.Flags().StringVar(&accountID, "apply-account", "", "Apply one reviewed "+app+" account ID")
	cmd.Flags().StringVar(&subject, "approved-subject", "", "Reviewed Keycloak subject for this account")
	return cmd
}

// runPagePreviewOrApply previews one page, or applies one reviewed link for an
// account within that page (the deprecated commands' behaviour).
func runPagePreviewOrApply(cmd *cobra.Command, app string, service identityreconcile.Service, cursor string, limit int, accountID, subject string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
	defer cancel()
	page, err := service.Preview(ctx, cursor, limit)
	if err != nil {
		return err
	}
	if accountID == "" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(page)
	}
	for _, decision := range page.Decisions {
		if decision.Account.ID != accountID {
			continue
		}
		if decision.Action != identityreconcile.Reuse || decision.Identity.Subject != subject {
			return errors.New("current match differs from the approved Keycloak subject")
		}
		if err := service.Apply(ctx, decision, decision.Account); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), app+" identity linked and access granted")
		return nil
	}
	return errors.New("approved account is not eligible or is outside the requested page")
}

type identitySyncConfig struct {
	adminBase, realm, adminID, adminSecret string
	// issuer is the public issuer written into application identity links. It must match
	// the iss claim users' ID tokens carry, so it comes from KEYCLOAK_EXTERNAL_URL.
	issuer string
	// tokenEndpoint is reached through KEYCLOAK_BASE_URL, which may be an internal address.
	tokenEndpoint                                       string
	origin, syncClientID, syncClientSecret, staticToken string
}

// keycloakSyncConfigFromEnv reads the Keycloak settings every identity-sync
// command needs: operator credentials, the public issuer and the token endpoint.
func keycloakSyncConfigFromEnv(getenv func(string) string) (identitySyncConfig, error) {
	cfg := identitySyncConfig{
		adminBase:   strings.TrimRight(getenv("KEYCLOAK_BASE_URL"), "/"),
		realm:       getenv("KEYCLOAK_REALM"),
		adminID:     getenv("KEYCLOAK_ADMIN_CLIENT_ID"),
		adminSecret: getenv("KEYCLOAK_ADMIN_CLIENT_SECRET"),
	}
	if !httpsBaseURL(cfg.adminBase) || cfg.realm == "" || url.PathEscape(cfg.realm) != cfg.realm ||
		cfg.adminID == "" || cfg.adminSecret == "" {
		return identitySyncConfig{}, errors.New("configure the HTTPS Keycloak base URL, realm and operator credentials")
	}
	public := strings.TrimRight(getenv("KEYCLOAK_EXTERNAL_URL"), "/")
	if public == "" {
		public = cfg.adminBase
	} else if !httpsBaseURL(public) {
		return identitySyncConfig{}, errors.New("KEYCLOAK_EXTERNAL_URL must be an HTTPS base URL")
	}
	cfg.issuer = public + "/realms/" + cfg.realm
	cfg.tokenEndpoint = cfg.adminBase + "/realms/" + cfg.realm + "/protocol/openid-connect/token"
	return cfg, nil
}

// identitySyncConfigFromEnv adds the deprecated per-application <APP>_SYNC_* settings.
func identitySyncConfigFromEnv(app string, getenv func(string) string) (identitySyncConfig, error) {
	cfg, err := keycloakSyncConfigFromEnv(getenv)
	if err != nil {
		return identitySyncConfig{}, err
	}
	cfg.origin = getenv(app + "_SYNC_ORIGIN")
	cfg.syncClientID = getenv(app + "_SYNC_CLIENT_ID")
	cfg.syncClientSecret = getenv(app + "_SYNC_CLIENT_SECRET")
	cfg.staticToken = getenv(app + "_SYNC_ACCESS_TOKEN")
	return cfg, nil
}

func httpsBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil &&
		u.RawQuery == "" && u.Fragment == ""
}

// identitySyncToken prefers the application's dedicated identity-sync service account
// (<APP>_SYNC_CLIENT_ID / <APP>_SYNC_CLIENT_SECRET) and falls back to a pre-minted
// <APP>_SYNC_ACCESS_TOKEN only when no service client is configured.
func identitySyncToken(app, tokenEndpoint, clientID, clientSecret, staticToken string) (identityreconcile.AccessToken, error) {
	if clientID != "" || clientSecret != "" {
		return identityreconcile.ClientCredentials(tokenEndpoint, clientID, clientSecret)
	}
	if staticToken != "" {
		return func(context.Context) (string, error) { return staticToken, nil }, nil
	}
	return nil, fmt.Errorf("configure %[1]s_SYNC_CLIENT_ID and %[1]s_SYNC_CLIENT_SECRET (or %[1]s_SYNC_ACCESS_TOKEN)", app)
}

// printDeprecation writes the alias deprecation note to stderr.
func printDeprecation(w io.Writer, name, clientID string) {
	fmt.Fprintf(w, "Note: %s is deprecated and will be removed after one release; use 'moh-sso identity-sync preview|apply --app %s'.\n", name, clientID)
}
