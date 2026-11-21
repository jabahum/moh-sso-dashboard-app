package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/spf13/cobra"
)

var (
	baseURL     string
	realm       string
	adminID     string
	adminSecret string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "moh-sso",
		Short: "MOH SSO Management CLI",
		Long:  "CLI tool for managing Keycloak realms, clients, and users for the MOH SSO Dashboard",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if isLocalToolCommand(cmd) {
				return nil
			}
			if baseURL == "" || realm == "" || adminID == "" || adminSecret == "" {
				return fmt.Errorf(
					"please provide all Keycloak admin connection details (base-url, realm, admin-client-id, admin-client-secret)",
				)
			}
			return nil
		},
	}

	// ------------------------------------------------------------------
	// Global flags (ADMIN ONLY)
	// ------------------------------------------------------------------
	rootCmd.PersistentFlags().StringVar(
		&baseURL,
		"base-url",
		os.Getenv("KEYCLOAK_BASE_URL"),
		"Keycloak base URL (e.g. http://keycloak:8080)",
	)

	rootCmd.PersistentFlags().StringVar(
		&realm,
		"realm",
		os.Getenv("KEYCLOAK_REALM"),
		"Keycloak realm name",
	)

	rootCmd.PersistentFlags().StringVar(
		&adminID,
		"admin-client-id",
		os.Getenv("KEYCLOAK_ADMIN_CLIENT_ID"),
		"Keycloak ADMIN client ID",
	)

	rootCmd.PersistentFlags().StringVar(
		&adminSecret,
		"admin-client-secret",
		os.Getenv("KEYCLOAK_ADMIN_CLIENT_SECRET"),
		"Keycloak ADMIN client secret",
	)

	// ------------------------------------------------------------------
	// Commands
	// ------------------------------------------------------------------
	rootCmd.AddCommand(initRealmCmd)
	rootCmd.AddCommand(createClientCmd)
	rootCmd.AddCommand(listClientsCmd)
	rootCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(listUsersCmd)
	rootCmd.AddCommand(systemRBACCmd)
	rootCmd.AddCommand(newPOESSyncCommand())
	rootCmd.AddCommand(newCLIMSSyncCommand())
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(openAPICmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("❌", err)
		os.Exit(1)
	}
}

// ------------------------------------------------------------------
// Helpers
// ------------------------------------------------------------------
func isLocalToolCommand(cmd *cobra.Command) bool {
	commandPath := cmd.CommandPath()
	localPrefixes := []string{
		"moh-sso system-rbac",
		"moh-sso doctor",
		"moh-sso config",
		"moh-sso dev",
		"moh-sso openapi",
		"moh-sso version",
	}
	for _, prefix := range localPrefixes {
		if strings.HasPrefix(commandPath, prefix) {
			return true
		}
	}
	return false
}

func newAdminKC() *keycloak.KeyAdminClient {
	kc := keycloak.NewAdminClient(
		baseURL,
		realm,
		adminID,
		adminSecret,
	)

	if err := kc.Authenticate(); err != nil {
		log.Fatalf("❌ Keycloak admin auth failed: %v", err)
	}

	return kc
}

// ------------------------------------------------------------------
// Realm Commands
// ------------------------------------------------------------------
var initRealmCmd = &cobra.Command{
	Use:   "init-realm",
	Short: "Initialize the Keycloak realm",
	Run: func(cmd *cobra.Command, args []string) {
		kc := newAdminKC()

		if err := kc.EnsureRealmExists(realm); err != nil {
			log.Fatalf("❌ Failed to initialize realm: %v", err)
		}

		fmt.Printf("✅ Realm '%s' is ready\n", realm)
	},
}

// ------------------------------------------------------------------
// Client Commands
// ------------------------------------------------------------------
var (
	clientName    string
	redirectURI   string
	clientBaseURL string
)

var createClientCmd = &cobra.Command{
	Use:   "create-client",
	Short: "Create a new Keycloak client",
	Run: func(cmd *cobra.Command, args []string) {
		kc := newAdminKC()

		params := keycloak.CreateClientParams{
			ClientID:                  clientName,
			Name:                      clientName,
			Description:               "Created via CLI",
			BaseURL:                   clientBaseURL,
			RootURL:                   clientBaseURL,
			RedirectURIs:              []string{redirectURI},
			WebOrigins:                []string{clientBaseURL},
			PublicClient:              true,
			Protocol:                  "openid-connect",
			StandardFlowEnabled:       true,
			ImplicitFlowEnabled:       false,
			DirectAccessGrantsEnabled: false,
			ServiceAccountsEnabled:    false,
			Enabled:                   true,
		}

		if _, err := kc.CreateClient(params); err != nil {
			log.Fatalf("❌ Error creating client: %v", err)
		}

		fmt.Printf("✅ Client '%s' created successfully\n", clientName)
	},
}

var listClientsCmd = &cobra.Command{
	Use:   "list-clients",
	Short: "List all Keycloak clients",
	Run: func(cmd *cobra.Command, args []string) {
		kc := newAdminKC()

		clients, err := kc.ListClients()
		if err != nil {
			log.Fatalf("❌ Error fetching clients: %v", err)
		}

		for _, c := range clients {
			fmt.Printf("• %s (%s)\n", c.ClientID, c.ID)
		}
	},
}

// ------------------------------------------------------------------
// User Commands
// ------------------------------------------------------------------
var (
	username string
	password string
	role     string
)

var createUserCmd = &cobra.Command{
	Use:   "create-user",
	Short: "Create a new Keycloak user",
	Run: func(cmd *cobra.Command, args []string) {
		kc := newAdminKC()

		user := model.User{
			Username: username,
			Email:    username + "@example.com",
			Enabled:  true,
		}

		if _, err := kc.CreateUser(&user); err != nil {
			log.Fatalf("❌ Failed to create user: %v", err)
		}

		fmt.Printf("✅ User '%s' created successfully\n", username)
	},
}

var listUsersCmd = &cobra.Command{
	Use:   "list-users",
	Short: "List all Keycloak users",
	Run: func(cmd *cobra.Command, args []string) {
		kc := newAdminKC()

		users, err := kc.ListUsers()
		if err != nil {
			log.Fatalf("❌ Error fetching users: %v", err)
		}

		for _, u := range users {
			fmt.Printf("• %s (%s)\n", u.Username, u.ID)
		}
	},
}

func init() {
	createClientCmd.Flags().StringVar(&clientName, "name", "", "Client name")
	createClientCmd.Flags().StringVar(&redirectURI, "redirect-uri", "", "Redirect URI")
	createClientCmd.Flags().StringVar(&clientBaseURL, "base-url", "", "Base URL")
	_ = createClientCmd.MarkFlagRequired("name")

	createUserCmd.Flags().StringVar(&username, "username", "", "Username")
	createUserCmd.Flags().StringVar(&password, "password", "", "Password (ignored by Keycloak API)")
	createUserCmd.Flags().StringVar(&role, "role", "user", "User role")
	_ = createUserCmd.MarkFlagRequired("username")
}
