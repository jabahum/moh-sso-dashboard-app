package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	// ==================================================
	// Global
	// ==================================================
	Environment string `mapstructure:"ENVIRONMENT"`
	GinMode     string `mapstructure:"GIN_MODE"`

	DevAuthBypass bool `mapstructure:"DEV_AUTH_BYPASS"`

	// ==================================================
	// Frontend
	// ==================================================
	FrontendBaseURL             string `mapstructure:"FRONTEND_BASE_URL"`
	FrontendRedirectURI         string `mapstructure:"FRONTEND_REDIRECT_URI"`
	LoginURL                    string `mapstructure:"LOGIN_URL"`
	AuthReturnURLAllowedOrigins string `mapstructure:"AUTH_RETURN_URL_ALLOWED_ORIGINS"`
	CookieDomain                string `mapstructure:"COOKIE_DOMAIN"`

	// ==================================================
	// Keycloak (Infrastructure)
	// ==================================================
	KeycloakVersion   string `mapstructure:"KEYCLOAK_VERSION"`
	KeycloakDB        string `mapstructure:"KEYCLOAK_DB"`
	KeycloakDBName    string `mapstructure:"KEYCLOAK_DB_NAME"`
	KeycloakDBUser    string `mapstructure:"KEYCLOAK_DB_USER"`
	KeycloakDBPass    string `mapstructure:"KEYCLOAK_DB_PASSWORD"`
	KeycloakAdmin     string `mapstructure:"KEYCLOAK_ADMIN"`
	KeycloakAdminPass string `mapstructure:"KEYCLOAK_ADMIN_PASSWORD"`
	KeycloakHostname  string `mapstructure:"KEYCLOAK_HOSTNAME"`

	// ==================================================
	// Keycloak (Application usage)
	// ==================================================
	KeycloakInternalURL string `mapstructure:"KEYCLOAK_INTERNAL_URL"`
	KeycloakExternalURL string `mapstructure:"KEYCLOAK_EXTERNAL_URL"`
	KeycloakBaseURL     string `mapstructure:"KEYCLOAK_BASE_URL"`
	KeycloakRealm       string `mapstructure:"KEYCLOAK_REALM"`

	KeycloakAdminClientID     string `mapstructure:"KEYCLOAK_ADMIN_CLIENT_ID"`
	KeycloakAdminClientSecret string `mapstructure:"KEYCLOAK_ADMIN_CLIENT_SECRET"`

	KeycloakWebClientID     string `mapstructure:"KEYCLOAK_WEB_CLIENT_ID"`
	KeycloakWebClientSecret string `mapstructure:"KEYCLOAK_WEB_CLIENT_SECRET"`
	KeycloakRedirectURI     string `mapstructure:"KEYCLOAK_REDIRECT_URI"`

	// ==================================================
	// Backend
	// ==================================================
	ServerPort string `mapstructure:"SERVER_PORT"`
	AppBaseURL string `mapstructure:"APP_BASE_URL"`

	// ==================================================
	// Health BI report integration
	// ==================================================
	HealthBIBaseURL    string        `mapstructure:"HEALTH_BI_BASE_URL"`
	HealthBIAPIToken   string        `mapstructure:"HEALTH_BI_API_TOKEN"`
	HealthBIAuthHeader string        `mapstructure:"HEALTH_BI_AUTH_HEADER"`
	HealthBITimeout    time.Duration `mapstructure:"HEALTH_BI_TIMEOUT"`

	// ==================================================
	// RBAC / Keycloak startup sync
	// ==================================================
	RBACStartupSyncEnabled         bool   `mapstructure:"RBAC_STARTUP_SYNC_ENABLED"`
	RBACStartupSeedEnabled         bool   `mapstructure:"RBAC_STARTUP_SEED_ENABLED"`
	RBACStartupSeedPath            string `mapstructure:"RBAC_STARTUP_SEED_PATH"`
	RBACStartupSyncRealmExport     bool   `mapstructure:"RBAC_STARTUP_SYNC_REALM_EXPORT"`
	RBACStartupSyncRealmExportPath string `mapstructure:"RBAC_STARTUP_SYNC_REALM_EXPORT_PATH"`
	RBACStartupSyncLiveKeycloak    bool   `mapstructure:"RBAC_STARTUP_SYNC_LIVE_KEYCLOAK"`
	RBACStartupSyncPushToKeycloak  bool   `mapstructure:"RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK"`
	RBACStartupSyncUsers           bool   `mapstructure:"RBAC_STARTUP_SYNC_USERS"`
	RBACStartupSyncFailOnError     bool   `mapstructure:"RBAC_STARTUP_SYNC_FAIL_ON_ERROR"`

	// ==================================================
	// TLS
	// ==================================================
	EnableTLS             bool   `mapstructure:"ENABLE_TLS"`
	TLSCert               string `mapstructure:"TLS_CERT"`
	TLSKey                string `mapstructure:"TLS_KEY"`
	TLSKeycloakCACertPath string `mapstructure:"TLS_KEYCLOAK_CA_CERT_PATH"`

	// ==================================================
	// Database
	// ==================================================
	DBDriver    string `mapstructure:"DB_DRIVER"`
	DBHost      string `mapstructure:"DB_HOST"`
	DBUser      string `mapstructure:"DB_USER"`
	DBPassword  string `mapstructure:"DB_PASSWORD"`
	DBName      string `mapstructure:"DB_NAME"`
	DBPort      string `mapstructure:"DB_PORT"`
	DBEnableSSL bool   `mapstructure:"DB_ENABLE_SSL"`

	// ==================================================
	// Redis
	// ==================================================
	RedisHost     string `mapstructure:"REDIS_HOST"`
	RedisPort     string `mapstructure:"REDIS_PORT"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`

	// ==================================================
	// JWT
	// ==================================================
	TokenSymmetricKey    string        `mapstructure:"TOKEN_SYMMETRIC_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`

	// ==================================================
	// Storage
	// ==================================================
	StorageProvider string `mapstructure:"STORAGE_PROVIDER"`
	LocalBasePath   string `mapstructure:"LOCAL_BASE_PATH"`
	NFSBasePath     string `mapstructure:"NFS_BASE_PATH"`

	S3Bucket          string `mapstructure:"S3_BUCKET"`
	S3Region          string `mapstructure:"S3_REGION"`
	S3AccessKeyID     string `mapstructure:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string `mapstructure:"S3_SECRET_ACCESS_KEY"`

	MinioEndpoint        string `mapstructure:"MINIO_ENDPOINT"`
	MinioRegion          string `mapstructure:"MINIO_REGION"`
	MinioBucket          string `mapstructure:"MINIO_BUCKET"`
	MinioAccessKeyID     string `mapstructure:"MINIO_ACCESS_KEY_ID"`
	MinioSecretAccessKey string `mapstructure:"MINIO_SECRET_ACCESS_KEY"`

	// ==================================================
	// Remote DB
	// ==================================================
	RemoteDBHost     string `mapstructure:"REMOTE_DB_HOST"`
	RemoteDBPort     string `mapstructure:"REMOTE_DB_PORT"`
	RemoteDBUser     string `mapstructure:"REMOTE_DB_USER"`
	RemoteDBPassword string `mapstructure:"REMOTE_DB_PASSWORD"`
	RemoteDBName     string `mapstructure:"REMOTE_DB_NAME"`

	// ==================================================
	// DWH
	// ==================================================
	DWHHost     string `mapstructure:"DWH_HOST"`
	DWHPort     string `mapstructure:"DWH_PORT"`
	DWHUsername string `mapstructure:"DWH_USERNAME"`
	DWHPassword string `mapstructure:"DWH_PASSWORD"`
	DWHDBName   string `mapstructure:"DWH_DB"`

	// DQA results are written by the role that owns the dqa schema, which is
	// not the role used to read source data. Same host/database as the DWH.
	DQADBUsername string `mapstructure:"DQA_DB_USERNAME"`
	DQADBPassword string `mapstructure:"DQA_DB_PASSWORD"`

	// ==================================================
	// SMTP / Retry / Notifications
	// ==================================================
	SMTP                 SMTPConfig                 `mapstructure:",squash"`
	SMTPGM               SMTPGMConfig               `mapstructure:",squash"`
	Email                EmailConfig                `mapstructure:",squash"`
	Announcement         AnnouncementConfig         `mapstructure:",squash"`
	Retry                RetryConfig                `mapstructure:",squash"`
	Notification         NotificationConfig         `mapstructure:",squash"`
	NotificationDelivery NotificationDeliveryConfig `mapstructure:",squash"`
	SMS                  SMSConfig                  `mapstructure:",squash"`
}

type SMTPConfig struct {
	Host           string        `mapstructure:"SMTP_HOST"`
	Port           int           `mapstructure:"SMTP_PORT"`
	Username       string        `mapstructure:"SMTP_USERNAME"`
	Password       string        `mapstructure:"SMTP_PASSWORD"`
	FromEmail      string        `mapstructure:"SMTP_FROM_EMAIL"`
	FromName       string        `mapstructure:"SMTP_FROM_NAME"`
	ConnectTimeout time.Duration `mapstructure:"SMTP_CONNECT_TIMEOUT"`
	SendTimeout    time.Duration `mapstructure:"SMTP_SEND_TIMEOUT"`
}

type SMTPGMConfig struct {
	Host           string        `mapstructure:"SMTP_HOST_GM"`
	Port           int           `mapstructure:"SMTP_PORT_GM"`
	Username       string        `mapstructure:"SMTP_USERNAME_GM"`
	Password       string        `mapstructure:"SMTP_PASSWORD_GM"`
	FromEmail      string        `mapstructure:"SMTP_FROM_EMAIL_GM"`
	FromName       string        `mapstructure:"SMTP_FROM_NAME_GM"`
	ConnectTimeout time.Duration `mapstructure:"SMTP_CONNECT_TIMEOUT_GM"`
	SendTimeout    time.Duration `mapstructure:"SMTP_SEND_TIMEOUT_GM"`
}

type EmailConfig struct {
	MaxAttachments         int    `mapstructure:"EMAIL_MAX_ATTACHMENTS"`
	MaxAttachmentBytes     int64  `mapstructure:"EMAIL_MAX_ATTACHMENT_BYTES"`
	AllowedAttachmentTypes string `mapstructure:"EMAIL_ALLOWED_ATTACHMENT_TYPES"`
}

type AnnouncementConfig struct {
	MaxAttachments         int    `mapstructure:"ANNOUNCEMENT_MAX_ATTACHMENTS"`
	MaxAttachmentBytes     int64  `mapstructure:"ANNOUNCEMENT_MAX_ATTACHMENT_BYTES"`
	AllowedAttachmentTypes string `mapstructure:"ANNOUNCEMENT_ALLOWED_ATTACHMENT_TYPES"`
}

type RetryConfig struct {
	MaxAttempts int           `mapstructure:"RETRY_MAX_ATTEMPTS"`
	BaseDelay   time.Duration `mapstructure:"RETRY_BASE_DELAY"`
}

type NotificationConfig struct {
	PlatformName      string `mapstructure:"PLATFORM_NAME"`
	SystemAdminName   string `mapstructure:"SYSTEM_ADMIN_NAME"`
	SystemAdminEmail  string `mapstructure:"SYSTEM_ADMIN_EMAIL"`
	AdminDashboardURL string `mapstructure:"ADMIN_DASHBOARD_URL"`
}

type NotificationDeliveryConfig struct {
	WorkerEnabled  bool          `mapstructure:"NOTIFICATION_DELIVERY_WORKER_ENABLED"`
	WorkerInterval time.Duration `mapstructure:"NOTIFICATION_DELIVERY_WORKER_INTERVAL"`
	BatchSize      int32         `mapstructure:"NOTIFICATION_DELIVERY_BATCH_SIZE"`
}

type SMSConfig struct {
	Enabled            bool          `mapstructure:"SMS_ENABLED"`
	Provider           string        `mapstructure:"SMS_PROVIDER"`
	DefaultCountryCode string        `mapstructure:"SMS_DEFAULT_COUNTRY_CODE"`
	FromName           string        `mapstructure:"SMS_FROM_NAME"`
	MaxLength          int           `mapstructure:"SMS_MAX_LENGTH"`
	SendTimeout        time.Duration `mapstructure:"SMS_SEND_TIMEOUT"`

	AfricasTalkingUsername string `mapstructure:"AFRICASTALKING_USERNAME"`
	AfricasTalkingAPIKey   string `mapstructure:"AFRICASTALKING_API_KEY"`
	AfricasTalkingSenderID string `mapstructure:"AFRICASTALKING_SENDER_ID"`

	TwilioAccountSID string `mapstructure:"TWILIO_ACCOUNT_SID"`
	TwilioAuthToken  string `mapstructure:"TWILIO_AUTH_TOKEN"`
	TwilioFromNumber string `mapstructure:"TWILIO_FROM_NUMBER"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigName("app")
	viper.SetConfigType("env")
	viper.AddConfigPath(path)

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AllowEmptyEnv(true)

	for key, env := range envBindings() {
		_ = viper.BindEnv(key, env)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	normalizeConfig(&config)

	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func setDefaults() {
	// ==================================================
	// Global defaults
	// ==================================================
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("GIN_MODE", "debug")
	viper.SetDefault("DEV_AUTH_BYPASS", false)

	// ==================================================
	// Frontend defaults
	// ==================================================
	viper.SetDefault("FRONTEND_BASE_URL", "http://localhost:3000")
	viper.SetDefault("FRONTEND_REDIRECT_URI", "http://localhost:3000")
	viper.SetDefault("LOGIN_URL", "http://localhost:9000/api/v1/auth/login")
	viper.SetDefault("AUTH_RETURN_URL_ALLOWED_ORIGINS", "http://localhost:3000")
	viper.SetDefault("COOKIE_DOMAIN", "")

	// ==================================================
	// Keycloak defaults
	// ==================================================
	viper.SetDefault("KEYCLOAK_EXTERNAL_URL", "http://localhost:8081")
	viper.SetDefault("KEYCLOAK_BASE_URL", "http://keycloak:8080")
	viper.SetDefault("KEYCLOAK_INTERNAL_URL", "")
	viper.SetDefault("KEYCLOAK_REALM", "moh-realm")
	viper.SetDefault("KEYCLOAK_REDIRECT_URI", "http://localhost:9000/api/v1/auth/callback")

	// ==================================================
	// Backend defaults
	// ==================================================
	viper.SetDefault("SERVER_PORT", "9000")
	viper.SetDefault("APP_BASE_URL", "http://localhost:9000")
	viper.SetDefault("HEALTH_BI_BASE_URL", "")
	viper.SetDefault("HEALTH_BI_API_TOKEN", "")
	viper.SetDefault("HEALTH_BI_AUTH_HEADER", "Authorization")
	viper.SetDefault("HEALTH_BI_TIMEOUT", "30s")
	viper.SetDefault("RBAC_STARTUP_SYNC_ENABLED", true)
	viper.SetDefault("RBAC_STARTUP_SEED_ENABLED", true)
	viper.SetDefault("RBAC_STARTUP_SEED_PATH", "config/system-rbac.seed.yaml")
	viper.SetDefault("RBAC_STARTUP_SYNC_REALM_EXPORT", true)
	viper.SetDefault("RBAC_STARTUP_SYNC_REALM_EXPORT_PATH", "")
	viper.SetDefault("RBAC_STARTUP_SYNC_LIVE_KEYCLOAK", true)
	viper.SetDefault("RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK", true)
	viper.SetDefault("RBAC_STARTUP_SYNC_USERS", true)
	viper.SetDefault("RBAC_STARTUP_SYNC_FAIL_ON_ERROR", false)

	// ==================================================
	// SMTP defaults for local MailHog
	// ==================================================
	viper.SetDefault("SMTP_HOST", "mailhog")
	viper.SetDefault("SMTP_PORT", 1025)
	viper.SetDefault("SMTP_USERNAME", "")
	viper.SetDefault("SMTP_PASSWORD", "")
	viper.SetDefault("SMTP_FROM_EMAIL", "noreply@moh.go.ug")
	viper.SetDefault("SMTP_FROM_NAME", "MOH Integrated Health Portal")
	viper.SetDefault("SMTP_CONNECT_TIMEOUT", "10s")
	viper.SetDefault("SMTP_SEND_TIMEOUT", "15s")
	viper.SetDefault("EMAIL_MAX_ATTACHMENTS", 5)
	viper.SetDefault("EMAIL_MAX_ATTACHMENT_BYTES", 10485760)
	viper.SetDefault("EMAIL_ALLOWED_ATTACHMENT_TYPES", "pdf,doc,docx,xls,xlsx,csv,png,jpg,jpeg")
	viper.SetDefault("ANNOUNCEMENT_MAX_ATTACHMENTS", 10)
	viper.SetDefault("ANNOUNCEMENT_MAX_ATTACHMENT_BYTES", 10485760)
	viper.SetDefault("ANNOUNCEMENT_ALLOWED_ATTACHMENT_TYPES", "pdf,doc,docx,xls,xlsx,csv,png,jpg,jpeg")

	// ==================================================
	// Retry defaults
	// ==================================================
	viper.SetDefault("RETRY_MAX_ATTEMPTS", 3)
	viper.SetDefault("RETRY_BASE_DELAY", "2s")

	// ==================================================
	// Notification defaults
	// ==================================================
	viper.SetDefault("PLATFORM_NAME", "MOH Integrated Health Portal")
	viper.SetDefault("SYSTEM_ADMIN_NAME", "System Administrator")
	viper.SetDefault("SYSTEM_ADMIN_EMAIL", "admin@example.com")
	viper.SetDefault("ADMIN_DASHBOARD_URL", "http://localhost:3000/admin/home")
	viper.SetDefault("NOTIFICATION_DELIVERY_WORKER_ENABLED", true)
	viper.SetDefault("NOTIFICATION_DELIVERY_WORKER_INTERVAL", "3s")
	viper.SetDefault("NOTIFICATION_DELIVERY_BATCH_SIZE", 20)

	// ==================================================
	// SMS defaults
	// ==================================================
	viper.SetDefault("SMS_ENABLED", false)
	viper.SetDefault("SMS_PROVIDER", "mock")
	viper.SetDefault("SMS_DEFAULT_COUNTRY_CODE", "+256")
	viper.SetDefault("SMS_FROM_NAME", "MOH")
	viper.SetDefault("SMS_MAX_LENGTH", 160)
	viper.SetDefault("SMS_SEND_TIMEOUT", "10s")
}

func envBindings() map[string]string {
	return map[string]string{
		"ENVIRONMENT":                           "ENVIRONMENT",
		"GIN_MODE":                              "GIN_MODE",
		"DEV_AUTH_BYPASS":                       "DEV_AUTH_BYPASS",
		"FRONTEND_BASE_URL":                     "FRONTEND_BASE_URL",
		"FRONTEND_REDIRECT_URI":                 "FRONTEND_REDIRECT_URI",
		"COOKIE_DOMAIN":                         "COOKIE_DOMAIN",
		"LOGIN_URL":                             "LOGIN_URL",
			"AUTH_RETURN_URL_ALLOWED_ORIGINS":       "AUTH_RETURN_URL_ALLOWED_ORIGINS",
			"HEALTH_BI_BASE_URL":                    "HEALTH_BI_BASE_URL",
			"HEALTH_BI_API_TOKEN":                   "HEALTH_BI_API_TOKEN",
			"HEALTH_BI_AUTH_HEADER":                 "HEALTH_BI_AUTH_HEADER",
			"HEALTH_BI_TIMEOUT":                     "HEALTH_BI_TIMEOUT",
		"KEYCLOAK_VERSION":                      "KEYCLOAK_VERSION",
		"KEYCLOAK_DB":                           "KEYCLOAK_DB",
		"KEYCLOAK_DB_NAME":                      "KEYCLOAK_DB_NAME",
		"KEYCLOAK_DB_USER":                      "KEYCLOAK_DB_USER",
		"KEYCLOAK_DB_PASSWORD":                  "KEYCLOAK_DB_PASSWORD",
		"KEYCLOAK_ADMIN":                        "KEYCLOAK_ADMIN",
		"KEYCLOAK_ADMIN_PASSWORD":               "KEYCLOAK_ADMIN_PASSWORD",
		"KEYCLOAK_HOSTNAME":                     "KEYCLOAK_HOSTNAME",
		"KEYCLOAK_INTERNAL_URL":                 "KEYCLOAK_INTERNAL_URL",
		"KEYCLOAK_EXTERNAL_URL":                 "KEYCLOAK_EXTERNAL_URL",
		"KEYCLOAK_BASE_URL":                     "KEYCLOAK_BASE_URL",
		"KEYCLOAK_REALM":                        "KEYCLOAK_REALM",
		"KEYCLOAK_ADMIN_CLIENT_ID":              "KEYCLOAK_ADMIN_CLIENT_ID",
		"KEYCLOAK_ADMIN_CLIENT_SECRET":          "KEYCLOAK_ADMIN_CLIENT_SECRET",
		"KEYCLOAK_WEB_CLIENT_ID":                "KEYCLOAK_WEB_CLIENT_ID",
		"KEYCLOAK_WEB_CLIENT_SECRET":            "KEYCLOAK_WEB_CLIENT_SECRET",
		"KEYCLOAK_REDIRECT_URI":                 "KEYCLOAK_REDIRECT_URI",
		"SERVER_PORT":                           "SERVER_PORT",
		"APP_BASE_URL":                          "APP_BASE_URL",
		"RBAC_STARTUP_SYNC_ENABLED":             "RBAC_STARTUP_SYNC_ENABLED",
		"RBAC_STARTUP_SEED_ENABLED":             "RBAC_STARTUP_SEED_ENABLED",
		"RBAC_STARTUP_SEED_PATH":                "RBAC_STARTUP_SEED_PATH",
		"RBAC_STARTUP_SYNC_REALM_EXPORT":        "RBAC_STARTUP_SYNC_REALM_EXPORT",
		"RBAC_STARTUP_SYNC_REALM_EXPORT_PATH":   "RBAC_STARTUP_SYNC_REALM_EXPORT_PATH",
		"RBAC_STARTUP_SYNC_LIVE_KEYCLOAK":       "RBAC_STARTUP_SYNC_LIVE_KEYCLOAK",
		"RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK":    "RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK",
		"RBAC_STARTUP_SYNC_USERS":               "RBAC_STARTUP_SYNC_USERS",
		"RBAC_STARTUP_SYNC_FAIL_ON_ERROR":       "RBAC_STARTUP_SYNC_FAIL_ON_ERROR",
		"ENABLE_TLS":                            "ENABLE_TLS",
		"TLS_CERT":                              "TLS_CERT",
		"TLS_KEY":                               "TLS_KEY",
		"TLS_KEYCLOAK_CA_CERT_PATH":             "TLS_KEYCLOAK_CA_CERT_PATH",
		"DB_DRIVER":                             "DB_DRIVER",
		"DB_HOST":                               "DB_HOST",
		"DB_PORT":                               "DB_PORT",
		"DB_NAME":                               "DB_NAME",
		"DB_USER":                               "DB_USER",
		"DB_PASSWORD":                           "DB_PASSWORD",
		"DB_ENABLE_SSL":                         "DB_ENABLE_SSL",
		"REDIS_HOST":                            "REDIS_HOST",
		"REDIS_PORT":                            "REDIS_PORT",
		"REDIS_PASSWORD":                        "REDIS_PASSWORD",
		"TOKEN_SYMMETRIC_KEY":                   "TOKEN_SYMMETRIC_KEY",
		"ACCESS_TOKEN_DURATION":                 "ACCESS_TOKEN_DURATION",
		"REFRESH_TOKEN_DURATION":                "REFRESH_TOKEN_DURATION",
		"STORAGE_PROVIDER":                      "STORAGE_PROVIDER",
		"LOCAL_BASE_PATH":                       "LOCAL_BASE_PATH",
		"NFS_BASE_PATH":                         "NFS_BASE_PATH",
		"S3_BUCKET":                             "S3_BUCKET",
		"S3_REGION":                             "S3_REGION",
		"S3_ACCESS_KEY_ID":                      "S3_ACCESS_KEY_ID",
		"S3_SECRET_ACCESS_KEY":                  "S3_SECRET_ACCESS_KEY",
		"MINIO_ENDPOINT":                        "MINIO_ENDPOINT",
		"MINIO_REGION":                          "MINIO_REGION",
		"MINIO_BUCKET":                          "MINIO_BUCKET",
		"MINIO_ACCESS_KEY_ID":                   "MINIO_ACCESS_KEY_ID",
		"MINIO_SECRET_ACCESS_KEY":               "MINIO_SECRET_ACCESS_KEY",
		"REMOTE_DB_HOST":                        "REMOTE_DB_HOST",
		"REMOTE_DB_PORT":                        "REMOTE_DB_PORT",
		"REMOTE_DB_USER":                        "REMOTE_DB_USER",
		"REMOTE_DB_PASSWORD":                    "REMOTE_DB_PASSWORD",
		"REMOTE_DB_NAME":                        "REMOTE_DB_NAME",
		"DWH_HOST":                              "DWH_HOST",
		"DWH_PORT":                              "DWH_PORT",
		"DWH_USERNAME":                          "DWH_USERNAME",
		"DWH_PASSWORD":                          "DWH_PASSWORD",
		"DWH_DB":                                "DWH_DB",
		"DQA_DB_USERNAME":                       "DQA_DB_USERNAME",
		"DQA_DB_PASSWORD":                       "DQA_DB_PASSWORD",
		"SMTP_HOST":                             "SMTP_HOST",
		"SMTP_PORT":                             "SMTP_PORT",
		"SMTP_USERNAME":                         "SMTP_USERNAME",
		"SMTP_PASSWORD":                         "SMTP_PASSWORD",
		"SMTP_FROM_EMAIL":                       "SMTP_FROM_EMAIL",
		"SMTP_FROM_NAME":                        "SMTP_FROM_NAME",
		"SMTP_CONNECT_TIMEOUT":                  "SMTP_CONNECT_TIMEOUT",
		"SMTP_SEND_TIMEOUT":                     "SMTP_SEND_TIMEOUT",
		"SMTP_HOST_GM":                          "SMTP_HOST_GM",
		"SMTP_PORT_GM":                          "SMTP_PORT_GM",
		"SMTP_USERNAME_GM":                      "SMTP_USERNAME_GM",
		"SMTP_PASSWORD_GM":                      "SMTP_PASSWORD_GM",
		"SMTP_FROM_EMAIL_GM":                    "SMTP_FROM_EMAIL_GM",
		"SMTP_FROM_NAME_GM":                     "SMTP_FROM_NAME_GM",
		"SMTP_CONNECT_TIMEOUT_GM":               "SMTP_CONNECT_TIMEOUT_GM",
		"SMTP_SEND_TIMEOUT_GM":                  "SMTP_SEND_TIMEOUT_GM",
		"EMAIL_MAX_ATTACHMENTS":                 "EMAIL_MAX_ATTACHMENTS",
		"EMAIL_MAX_ATTACHMENT_BYTES":            "EMAIL_MAX_ATTACHMENT_BYTES",
		"EMAIL_ALLOWED_ATTACHMENT_TYPES":        "EMAIL_ALLOWED_ATTACHMENT_TYPES",
		"ANNOUNCEMENT_MAX_ATTACHMENTS":          "ANNOUNCEMENT_MAX_ATTACHMENTS",
		"ANNOUNCEMENT_MAX_ATTACHMENT_BYTES":     "ANNOUNCEMENT_MAX_ATTACHMENT_BYTES",
		"ANNOUNCEMENT_ALLOWED_ATTACHMENT_TYPES": "ANNOUNCEMENT_ALLOWED_ATTACHMENT_TYPES",
		"RETRY_MAX_ATTEMPTS":                    "RETRY_MAX_ATTEMPTS",
		"RETRY_BASE_DELAY":                      "RETRY_BASE_DELAY",
		"PLATFORM_NAME":                         "PLATFORM_NAME",
		"SYSTEM_ADMIN_NAME":                     "SYSTEM_ADMIN_NAME",
		"SYSTEM_ADMIN_EMAIL":                    "SYSTEM_ADMIN_EMAIL",
		"ADMIN_DASHBOARD_URL":                   "ADMIN_DASHBOARD_URL",
		"NOTIFICATION_DELIVERY_WORKER_ENABLED":  "NOTIFICATION_DELIVERY_WORKER_ENABLED",
		"NOTIFICATION_DELIVERY_WORKER_INTERVAL": "NOTIFICATION_DELIVERY_WORKER_INTERVAL",
		"NOTIFICATION_DELIVERY_BATCH_SIZE":      "NOTIFICATION_DELIVERY_BATCH_SIZE",
		"SMS_ENABLED":                           "SMS_ENABLED",
		"SMS_PROVIDER":                          "SMS_PROVIDER",
		"SMS_DEFAULT_COUNTRY_CODE":              "SMS_DEFAULT_COUNTRY_CODE",
		"SMS_FROM_NAME":                         "SMS_FROM_NAME",
		"SMS_MAX_LENGTH":                        "SMS_MAX_LENGTH",
		"SMS_SEND_TIMEOUT":                      "SMS_SEND_TIMEOUT",
		"AFRICASTALKING_USERNAME":               "AFRICASTALKING_USERNAME",
		"AFRICASTALKING_API_KEY":                "AFRICASTALKING_API_KEY",
		"AFRICASTALKING_SENDER_ID":              "AFRICASTALKING_SENDER_ID",
		"TWILIO_ACCOUNT_SID":                    "TWILIO_ACCOUNT_SID",
		"TWILIO_AUTH_TOKEN":                     "TWILIO_AUTH_TOKEN",
		"TWILIO_FROM_NUMBER":                    "TWILIO_FROM_NUMBER",
	}
}

func normalizeConfig(c *Config) {
	// ==================================================
	// Global
	// ==================================================
	c.Environment = normalizeEnvironment(c.Environment)
	c.GinMode = strings.TrimSpace(c.GinMode)

	if c.GinMode == "" {
		if c.IsProduction() {
			c.GinMode = "release"
		} else {
			c.GinMode = "debug"
		}
	}

	// ==================================================
	// Frontend / Auth URLs
	// ==================================================
	c.FrontendBaseURL = trimURL(c.FrontendBaseURL)
	c.FrontendRedirectURI = strings.TrimSpace(c.FrontendRedirectURI)
	c.LoginURL = strings.TrimSpace(c.LoginURL)
	c.AuthReturnURLAllowedOrigins = strings.TrimSpace(c.AuthReturnURLAllowedOrigins)
	c.CookieDomain = normalizeCookieDomain(c.CookieDomain, c.Environment)

	// ==================================================
	// Keycloak
	// ==================================================
	c.KeycloakHostname = normalizeKeycloakHostname(c.KeycloakHostname)

	c.KeycloakInternalURL = trimURL(c.KeycloakInternalURL)
	c.KeycloakExternalURL = trimURL(c.KeycloakExternalURL)
	c.KeycloakBaseURL = trimURL(c.KeycloakBaseURL)
	c.KeycloakRealm = strings.TrimSpace(c.KeycloakRealm)

	if c.KeycloakInternalURL == "" {
		c.KeycloakInternalURL = c.KeycloakBaseURL
	}

	if c.KeycloakBaseURL == "" {
		c.KeycloakBaseURL = c.KeycloakInternalURL
	}

	c.KeycloakAdminClientID = strings.TrimSpace(c.KeycloakAdminClientID)
	c.KeycloakAdminClientSecret = strings.TrimSpace(c.KeycloakAdminClientSecret)
	c.KeycloakWebClientID = strings.TrimSpace(c.KeycloakWebClientID)
	c.KeycloakWebClientSecret = strings.TrimSpace(c.KeycloakWebClientSecret)
	c.KeycloakRedirectURI = strings.TrimSpace(c.KeycloakRedirectURI)

	// ==================================================
	// Backend
	// ==================================================
	c.ServerPort = strings.TrimSpace(c.ServerPort)
	c.AppBaseURL = trimURL(c.AppBaseURL)
	c.RBACStartupSeedPath = strings.TrimSpace(c.RBACStartupSeedPath)
	c.RBACStartupSyncRealmExportPath = strings.TrimSpace(c.RBACStartupSyncRealmExportPath)

	// ==================================================
	// Database
	// ==================================================
	c.DBDriver = strings.TrimSpace(c.DBDriver)
	c.DBHost = strings.TrimSpace(c.DBHost)
	c.DBUser = strings.TrimSpace(c.DBUser)
	c.DBName = strings.TrimSpace(c.DBName)
	c.DBPort = strings.TrimSpace(c.DBPort)

	// ==================================================
	// Redis
	// ==================================================
	c.RedisHost = strings.TrimSpace(c.RedisHost)
	c.RedisPort = strings.TrimSpace(c.RedisPort)

	// ==================================================
	// Storage
	// ==================================================
	c.StorageProvider = strings.ToLower(strings.TrimSpace(c.StorageProvider))
	c.LocalBasePath = strings.TrimSpace(c.LocalBasePath)
	c.NFSBasePath = strings.TrimSpace(c.NFSBasePath)

	c.S3Bucket = strings.TrimSpace(c.S3Bucket)
	c.S3Region = strings.TrimSpace(c.S3Region)
	c.S3AccessKeyID = strings.TrimSpace(c.S3AccessKeyID)
	c.S3SecretAccessKey = strings.TrimSpace(c.S3SecretAccessKey)

	c.MinioEndpoint = trimURL(c.MinioEndpoint)
	c.MinioRegion = strings.TrimSpace(c.MinioRegion)
	c.MinioBucket = strings.TrimSpace(c.MinioBucket)
	c.MinioAccessKeyID = strings.TrimSpace(c.MinioAccessKeyID)
	c.MinioSecretAccessKey = strings.TrimSpace(c.MinioSecretAccessKey)

	// ==================================================
	// Remote DB / DWH
	// ==================================================
	c.RemoteDBHost = strings.TrimSpace(c.RemoteDBHost)
	c.RemoteDBPort = strings.TrimSpace(c.RemoteDBPort)
	c.RemoteDBUser = strings.TrimSpace(c.RemoteDBUser)
	c.RemoteDBName = strings.TrimSpace(c.RemoteDBName)

	c.DWHHost = strings.TrimSpace(c.DWHHost)
	c.DWHPort = strings.TrimSpace(c.DWHPort)
	c.DWHUsername = strings.TrimSpace(c.DWHUsername)
	c.DWHDBName = strings.TrimSpace(c.DWHDBName)

	// ==================================================
	// SMTP / Notifications
	// ==================================================
	c.SMTP.Host = strings.TrimSpace(c.SMTP.Host)
	c.SMTP.Username = strings.TrimSpace(c.SMTP.Username)
	c.SMTP.FromEmail = strings.TrimSpace(c.SMTP.FromEmail)
	c.SMTP.FromName = strings.TrimSpace(c.SMTP.FromName)
	c.Email.AllowedAttachmentTypes = strings.TrimSpace(c.Email.AllowedAttachmentTypes)
	if c.Email.MaxAttachments <= 0 {
		c.Email.MaxAttachments = 5
	}
	if c.Email.MaxAttachmentBytes <= 0 {
		c.Email.MaxAttachmentBytes = 10 * 1024 * 1024
	}
	c.Announcement.AllowedAttachmentTypes = strings.TrimSpace(c.Announcement.AllowedAttachmentTypes)
	if c.Announcement.MaxAttachments <= 0 {
		c.Announcement.MaxAttachments = 10
	}
	if c.Announcement.MaxAttachmentBytes <= 0 {
		c.Announcement.MaxAttachmentBytes = 10 * 1024 * 1024
	}

	c.Notification.PlatformName = strings.TrimSpace(c.Notification.PlatformName)
	c.Notification.SystemAdminName = strings.TrimSpace(c.Notification.SystemAdminName)
	c.Notification.SystemAdminEmail = strings.TrimSpace(c.Notification.SystemAdminEmail)
	c.Notification.AdminDashboardURL = strings.TrimSpace(c.Notification.AdminDashboardURL)
	if c.NotificationDelivery.WorkerInterval <= 0 {
		c.NotificationDelivery.WorkerInterval = 3 * time.Second
	}
	if c.NotificationDelivery.BatchSize <= 0 {
		c.NotificationDelivery.BatchSize = 20
	}

	if c.Notification.PlatformName == "" {
		c.Notification.PlatformName = "MOH Integrated Health Portal"
	}
	if c.Notification.SystemAdminName == "" {
		c.Notification.SystemAdminName = "System Administrator"
	}
	if c.Notification.AdminDashboardURL == "" {
		if c.FrontendBaseURL != "" {
			c.Notification.AdminDashboardURL = strings.TrimRight(c.FrontendBaseURL, "/") + "portal/admin/home"
		} else {
			c.Notification.AdminDashboardURL = "http://localhost:3000/admin/home"
		}
	}

	c.SMS.Provider = strings.ToLower(strings.TrimSpace(c.SMS.Provider))
	if c.SMS.Provider == "" {
		c.SMS.Provider = "mock"
	}
	c.SMS.DefaultCountryCode = normalizeCountryCode(c.SMS.DefaultCountryCode)
	if c.SMS.DefaultCountryCode == "" {
		c.SMS.DefaultCountryCode = "+256"
	}
	c.SMS.FromName = strings.TrimSpace(c.SMS.FromName)
	if c.SMS.FromName == "" {
		c.SMS.FromName = "MOH"
	}
	if c.SMS.MaxLength <= 0 {
		c.SMS.MaxLength = 160
	}
	if c.SMS.SendTimeout <= 0 {
		c.SMS.SendTimeout = 10 * time.Second
	}
	c.SMS.AfricasTalkingUsername = strings.TrimSpace(c.SMS.AfricasTalkingUsername)
	c.SMS.AfricasTalkingAPIKey = strings.TrimSpace(c.SMS.AfricasTalkingAPIKey)
	c.SMS.AfricasTalkingSenderID = strings.TrimSpace(c.SMS.AfricasTalkingSenderID)
	c.SMS.TwilioAccountSID = strings.TrimSpace(c.SMS.TwilioAccountSID)
	c.SMS.TwilioAuthToken = strings.TrimSpace(c.SMS.TwilioAuthToken)
	c.SMS.TwilioFromNumber = strings.TrimSpace(c.SMS.TwilioFromNumber)
}

// ==================================================
// Environment helpers
// ==================================================

func (c *Config) IsProduction() bool {
	env := normalizeEnvironment(c.Environment)

	return env == "production" || env == "prod"
}

func (c *Config) IsStaging() bool {
	env := normalizeEnvironment(c.Environment)

	return env == "staging" || env == "stage"
}

func (c *Config) IsDevelopment() bool {
	env := normalizeEnvironment(c.Environment)

	return env == "development" ||
		env == "dev" ||
		env == "local" ||
		env == "test"
}

func (c *Config) CookieSecure() bool {
	return c.IsProduction() || c.IsStaging()
}

func (c *Config) CookieDomainValue() string {
	return normalizeCookieDomain(c.CookieDomain, c.Environment)
}

func (c *Config) CookieSameSiteMode() string {
	// Your current flow uses same-site public domain:
	// frontend: https://dashboards.health.go.ug
	// backend:  https://dashboards.health.go.ug/ssobackend
	//
	// SameSite=Lax is correct for OAuth top-level navigation callbacks.
	return "Lax"
}

func (c *Config) KeycloakTokenBaseURL() string {
	if strings.TrimSpace(c.KeycloakInternalURL) != "" {
		return strings.TrimRight(c.KeycloakInternalURL, "/")
	}

	return strings.TrimRight(c.KeycloakBaseURL, "/")
}

// ==================================================
// DSNs
// ==================================================

func (c *Config) DbSource() string {
	if c.DBUser == "" {
		panic("DB_USER is empty")
	}

	dbPassEscaped := url.QueryEscape(c.DBPassword)

	sslMode := "disable"
	if c.DBEnableSSL {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser,
		dbPassEscaped,
		c.DBHost,
		c.DBPort,
		c.DBName,
		sslMode,
	)
}

func (c *Config) RemoteDbSource() string {
	if c.RemoteDBUser == "" {
		panic("REMOTE_DB_USER is empty")
	}

	dbPassEscaped := url.QueryEscape(c.RemoteDBPassword)

	sslMode := "disable"
	if c.DBEnableSSL {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		c.RemoteDBUser,
		dbPassEscaped,
		c.RemoteDBHost,
		c.RemoteDBPort,
		c.RemoteDBName,
		sslMode,
	)
}

func (c *Config) DwhDbSource() string {
	if c.DWHUsername == "" {
		panic("DWH_USERNAME is empty")
	}

	dbPassEscaped := url.QueryEscape(c.DWHPassword)

	sslMode := "disable"
	if c.DBEnableSSL {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		c.DWHUsername,
		dbPassEscaped,
		c.DWHHost,
		c.DWHPort,
		c.DWHDBName,
		sslMode,
	)
}

// DqaDbSource is the connection the DQA store persists results with. It targets
// the same DWH database as DwhDbSource but authenticates as the role that owns
// the dqa schema; source data is still read over the DWH connection, so this
// role needs no access to the source schemas. Falls back to the DWH credentials
// when DQA_DB_USERNAME is unset.
func (c *Config) DqaDbSource() string {
	if c.DQADBUsername == "" {
		return c.DwhDbSource()
	}

	sslMode := "disable"
	if c.DBEnableSSL {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		c.DQADBUsername,
		url.QueryEscape(c.DQADBPassword),
		c.DWHHost,
		c.DWHPort,
		c.DWHDBName,
		sslMode,
	)
}

// ==================================================
// Validation
// ==================================================

func validateConfig(c *Config) error {
	if err := validateEnvironment(c); err != nil {
		return err
	}
	if c.DevAuthBypass && !c.DevAuthBypassEnabled() {
		return errors.New("DEV_AUTH_BYPASS is only allowed in development, dev, or local environments")
	}

	if err := validateFrontendConfig(c); err != nil {
		return err
	}

	if err := validateKeycloakConfig(c); err != nil {
		return err
	}

	if err := validateDatabaseConfig(c); err != nil {
		return err
	}

	if err := validateStorageConfig(c); err != nil {
		return err
	}

	if err := validateSMTPConfig(c); err != nil {
		return err
	}

	if err := validateRetryConfig(c); err != nil {
		return err
	}

	if err := validateNotificationConfig(c); err != nil {
		return err
	}

	if err := validateNotificationDeliveryConfig(c); err != nil {
		return err
	}

	if err := validateSMSConfig(c); err != nil {
		return err
	}

	return nil
}

// DevAuthBypassEnabled fails closed for missing, test, and deployed environments.
func (c *Config) DevAuthBypassEnabled() bool {
	if c == nil || !c.DevAuthBypass {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(c.Environment)) {
	case "development", "dev", "local":
		return true
	default:
		return false
	}
}

func validateEnvironment(c *Config) error {
	if strings.TrimSpace(c.Environment) == "" {
		return errors.New("ENVIRONMENT is required")
	}

	switch normalizeEnvironment(c.Environment) {
	case "development", "dev", "local", "test", "staging", "stage", "production", "prod":
		return nil
	default:
		return fmt.Errorf("unsupported ENVIRONMENT: %s", c.Environment)
	}
}

func validateFrontendConfig(c *Config) error {
	if strings.TrimSpace(c.FrontendBaseURL) == "" {
		return errors.New("FRONTEND_BASE_URL is required")
	}

	if err := validateHTTPURL("FRONTEND_BASE_URL", c.FrontendBaseURL); err != nil {
		return err
	}

	if strings.TrimSpace(c.LoginURL) != "" {
		if err := validateHTTPURL("LOGIN_URL", c.LoginURL); err != nil {
			return err
		}
	}

	if strings.TrimSpace(c.FrontendRedirectURI) != "" {
		if err := validateHTTPURL("FRONTEND_REDIRECT_URI", c.FrontendRedirectURI); err != nil {
			return err
		}
	}

	for _, rawOrigin := range strings.Split(c.AuthReturnURLAllowedOrigins, ",") {
		origin := strings.TrimSpace(rawOrigin)
		if origin == "" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
			return fmt.Errorf("AUTH_RETURN_URL_ALLOWED_ORIGINS contains invalid origin %q", origin)
		}
	}

	if c.IsProduction() {
		if !strings.HasPrefix(c.FrontendBaseURL, "https://") {
			return errors.New("FRONTEND_BASE_URL must use https:// in production")
		}
		if strings.TrimSpace(c.LoginURL) != "" && !strings.HasPrefix(c.LoginURL, "https://") {
			return errors.New("LOGIN_URL must use https:// in production")
		}
	}

	return nil
}

func validateKeycloakConfig(c *Config) error {
	if strings.TrimSpace(c.KeycloakExternalURL) == "" {
		return errors.New("KEYCLOAK_EXTERNAL_URL is required")
	}
	if err := validateHTTPURL("KEYCLOAK_EXTERNAL_URL", c.KeycloakExternalURL); err != nil {
		return err
	}

	if strings.TrimSpace(c.KeycloakTokenBaseURL()) == "" {
		return errors.New("KEYCLOAK_BASE_URL or KEYCLOAK_INTERNAL_URL is required")
	}
	if err := validateHTTPURL("KEYCLOAK_BASE_URL/KEYCLOAK_INTERNAL_URL", c.KeycloakTokenBaseURL()); err != nil {
		return err
	}

	if strings.TrimSpace(c.KeycloakRealm) == "" {
		return errors.New("KEYCLOAK_REALM is required")
	}

	if strings.TrimSpace(c.KeycloakAdminClientID) == "" {
		return errors.New("KEYCLOAK_ADMIN_CLIENT_ID is required")
	}

	if strings.TrimSpace(c.KeycloakWebClientID) == "" {
		return errors.New("KEYCLOAK_WEB_CLIENT_ID is required")
	}

	if strings.TrimSpace(c.KeycloakRedirectURI) == "" {
		return errors.New("KEYCLOAK_REDIRECT_URI is required")
	}

	if err := validateHTTPURL("KEYCLOAK_REDIRECT_URI", c.KeycloakRedirectURI); err != nil {
		return err
	}

	if c.IsProduction() {
		if !strings.HasPrefix(c.KeycloakExternalURL, "https://") {
			return errors.New("KEYCLOAK_EXTERNAL_URL must use https:// in production")
		}
		if !strings.HasPrefix(c.KeycloakRedirectURI, "https://") {
			return errors.New("KEYCLOAK_REDIRECT_URI must use https:// in production")
		}
	}

	return nil
}

func validateDatabaseConfig(c *Config) error {
	if strings.TrimSpace(c.DBUser) == "" {
		return errors.New("DB_USER is required")
	}
	if strings.TrimSpace(c.DBPassword) == "" {
		return errors.New("DB_PASSWORD is required")
	}
	if strings.TrimSpace(c.DBHost) == "" {
		return errors.New("DB_HOST is required")
	}
	if strings.TrimSpace(c.DBName) == "" {
		return errors.New("DB_NAME is required")
	}
	if strings.TrimSpace(c.DBPort) == "" {
		return errors.New("DB_PORT is required")
	}

	return nil
}

func validateStorageConfig(c *Config) error {
	if c.StorageProvider == "" {
		return nil
	}

	switch c.StorageProvider {
	case "local":
		if strings.TrimSpace(c.LocalBasePath) == "" {
			return errors.New("LOCAL_BASE_PATH is required when STORAGE_PROVIDER=local")
		}
		if strings.TrimSpace(c.AppBaseURL) == "" {
			return errors.New("APP_BASE_URL is required when STORAGE_PROVIDER=local")
		}
	case "nfs":
		if strings.TrimSpace(c.NFSBasePath) == "" {
			return errors.New("NFS_BASE_PATH is required when STORAGE_PROVIDER=nfs")
		}
		if strings.TrimSpace(c.AppBaseURL) == "" {
			return errors.New("APP_BASE_URL is required when STORAGE_PROVIDER=nfs")
		}
	case "s3":
		if strings.TrimSpace(c.S3Bucket) == "" {
			return errors.New("S3_BUCKET is required when STORAGE_PROVIDER=s3")
		}
		if strings.TrimSpace(c.S3Region) == "" {
			return errors.New("S3_REGION is required when STORAGE_PROVIDER=s3")
		}
	case "minio":
		if strings.TrimSpace(c.MinioEndpoint) == "" {
			return errors.New("MINIO_ENDPOINT is required when STORAGE_PROVIDER=minio")
		}
		if strings.TrimSpace(c.MinioBucket) == "" {
			return errors.New("MINIO_BUCKET is required when STORAGE_PROVIDER=minio")
		}
	default:
		return fmt.Errorf("unsupported STORAGE_PROVIDER: %s", c.StorageProvider)
	}

	return nil
}

func validateSMTPConfig(c *Config) error {
	if c.SMTP.Host == "" {
		return nil
	}

	if c.SMTP.Port <= 0 {
		return errors.New("SMTP_PORT must be greater than 0")
	}
	if strings.TrimSpace(c.SMTP.FromEmail) == "" {
		return errors.New("SMTP_FROM_EMAIL is required when SMTP is enabled")
	}
	if strings.TrimSpace(c.SMTP.FromName) == "" {
		return errors.New("SMTP_FROM_NAME is required when SMTP is enabled")
	}

	return nil
}

func validateRetryConfig(c *Config) error {
	if c.Retry.MaxAttempts < 0 {
		return errors.New("RETRY_MAX_ATTEMPTS cannot be negative")
	}
	if c.Retry.BaseDelay < 0 {
		return errors.New("RETRY_BASE_DELAY cannot be negative")
	}

	return nil
}

func validateNotificationConfig(c *Config) error {
	if strings.TrimSpace(c.Notification.PlatformName) == "" {
		return errors.New("PLATFORM_NAME is required")
	}
	if strings.TrimSpace(c.Notification.SystemAdminName) == "" {
		return errors.New("SYSTEM_ADMIN_NAME is required")
	}
	if strings.TrimSpace(c.Notification.SystemAdminEmail) == "" {
		return errors.New("SYSTEM_ADMIN_EMAIL is required")
	}
	if strings.TrimSpace(c.Notification.AdminDashboardURL) == "" {
		return errors.New("ADMIN_DASHBOARD_URL is required")
	}

	return nil
}

func validateNotificationDeliveryConfig(c *Config) error {
	if c.NotificationDelivery.WorkerInterval <= 0 {
		return errors.New("NOTIFICATION_DELIVERY_WORKER_INTERVAL must be greater than 0")
	}
	if c.NotificationDelivery.BatchSize <= 0 {
		return errors.New("NOTIFICATION_DELIVERY_BATCH_SIZE must be greater than 0")
	}

	return nil
}

func validateSMSConfig(c *Config) error {
	if !c.SMS.Enabled {
		return nil
	}

	switch c.SMS.Provider {
	case "mock", "noop":
	case "africastalking":
		if strings.TrimSpace(c.SMS.AfricasTalkingUsername) == "" {
			return errors.New("AFRICASTALKING_USERNAME is required when SMS_PROVIDER=africastalking")
		}
		if strings.TrimSpace(c.SMS.AfricasTalkingAPIKey) == "" {
			return errors.New("AFRICASTALKING_API_KEY is required when SMS_PROVIDER=africastalking")
		}
	case "twilio":
		if strings.TrimSpace(c.SMS.TwilioAccountSID) == "" {
			return errors.New("TWILIO_ACCOUNT_SID is required when SMS_PROVIDER=twilio")
		}
		if strings.TrimSpace(c.SMS.TwilioAuthToken) == "" {
			return errors.New("TWILIO_AUTH_TOKEN is required when SMS_PROVIDER=twilio")
		}
		if strings.TrimSpace(c.SMS.TwilioFromNumber) == "" {
			return errors.New("TWILIO_FROM_NUMBER is required when SMS_PROVIDER=twilio")
		}
	default:
		return fmt.Errorf("unsupported SMS_PROVIDER: %s", c.SMS.Provider)
	}

	if c.SMS.MaxLength <= 0 {
		return errors.New("SMS_MAX_LENGTH must be greater than 0")
	}

	return nil
}

// ==================================================
// Normalization helpers
// ==================================================

func normalizeEnvironment(env string) string {
	env = strings.ToLower(strings.TrimSpace(env))

	if env == "" {
		return "development"
	}

	return env
}

func trimURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func normalizeCountryCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "+") {
		return value
	}
	return "+" + strings.TrimLeft(value, "+")
}

func normalizeCookieDomain(domain string, environment string) string {
	domain = strings.TrimSpace(domain)

	if domain == "" {
		return ""
	}

	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimRight(domain, "/")

	env := normalizeEnvironment(environment)

	// Never set Domain=localhost.
	// Host-only cookies work better for localhost.
	if env == "development" || env == "dev" || env == "local" || env == "test" {
		if domain == "localhost" ||
			strings.HasPrefix(domain, "localhost:") ||
			domain == "127.0.0.1" ||
			strings.HasPrefix(domain, "127.0.0.1:") ||
			domain == "::1" {
			return ""
		}
	}

	return domain
}

func normalizeKeycloakHostname(hostname string) string {
	hostname = strings.TrimSpace(hostname)

	if hostname == "" {
		return ""
	}

	hostname = strings.TrimPrefix(hostname, "https://")
	hostname = strings.TrimPrefix(hostname, "http://")
	hostname = strings.TrimRight(hostname, "/")

	return hostname
}

func validateHTTPURL(name string, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("%s is required", name)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", name, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must start with http:// or https://", name)
	}

	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", name)
	}

	return nil
}
