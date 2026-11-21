package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

const poesClientID = "poesgo"

type poesDirectory struct {
	admin  *keycloak.KeyAdminClient
	issuer string
}

func (d poesDirectory) FindEmail(ctx context.Context, email string) ([]identitysync.Identity, error) {
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

func (poesDirectory) EnsureUser(context.Context, identitysync.Account) (identitysync.Identity, error) {
	return identitysync.Identity{}, errors.New("provision new Keycloak users separately before applying POES links")
}

func (d poesDirectory) GrantAccess(ctx context.Context, identity identitysync.Identity) error {
	client, err := d.admin.GetClientByClientID(poesClientID)
	if err != nil {
		return err
	}
	if !client.Enabled || client.ClientID != poesClientID {
		return errors.New("POES Keycloak client is unavailable")
	}
	return d.admin.AssignClientRolesToUser(ctx, identity.Subject, poesClientID, client.ID, []string{"poesgo_access"})
}

func newPOESSyncCommand() *cobra.Command {
	var cursor, accountID, subject string
	var limit int
	cmd := &cobra.Command{
		Use:   "poes-identity-sync",
		Short: "Preview POES identity matches or apply one reviewed existing-user link",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (accountID == "") != (subject == "") {
				return errors.New("apply requires both --apply-account and --approved-subject")
			}
			origin := os.Getenv("POES_SYNC_ORIGIN")
			token := os.Getenv("POES_SYNC_ACCESS_TOKEN")
			base := strings.TrimRight(os.Getenv("KEYCLOAK_BASE_URL"), "/")
			realm := os.Getenv("KEYCLOAK_REALM")
			adminID := os.Getenv("KEYCLOAK_ADMIN_CLIENT_ID")
			secret := os.Getenv("KEYCLOAK_ADMIN_CLIENT_SECRET")
			u, err := url.Parse(base)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" ||
				u.Fragment != "" || realm == "" || url.PathEscape(realm) != realm ||
				adminID == "" || secret == "" || token == "" {
				return errors.New("configure the HTTPS Keycloak base URL, realm, operator credentials and POES service access token")
			}
			accounts, err := identityreconcile.NewPOES(origin, func(context.Context) (string, error) { return token, nil })
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			issuer := base + "/realms/" + realm
			directory := poesDirectory{admin: keycloak.NewAdminClient(base, realm, adminID, secret), issuer: issuer}
			service := identityreconcile.Service{Accounts: accounts, Directory: directory, Issuer: issuer}
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
				fmt.Fprintln(cmd.OutOrStdout(), "POES identity linked and access granted")
				return nil
			}
			return errors.New("approved account is not eligible or is outside the requested page")
		},
	}
	cmd.Flags().StringVar(&cursor, "after", "", "Resume from a preview cursor")
	cmd.Flags().IntVar(&limit, "limit", 25, "Accounts to preview in the requested page")
	cmd.Flags().StringVar(&accountID, "apply-account", "", "Apply one reviewed POES account ID")
	cmd.Flags().StringVar(&subject, "approved-subject", "", "Reviewed Keycloak subject for this account")
	return cmd
}
