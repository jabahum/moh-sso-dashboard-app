package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// Registry errors. Handlers map ErrIntegrationNotFound to 404 and
// ErrInvalidIntegration to 400.
var (
	ErrIntegrationNotFound = errors.New("identity sync integration not found")
	ErrIntegrationDisabled = errors.New("identity sync integration is disabled")
	ErrInvalidIntegration  = errors.New("invalid identity sync integration")
	// ErrDescriptorUnavailable reports that an integration could not be enabled
	// because its credential, token or GET / descriptor check failed.
	ErrDescriptorUnavailable = errors.New("identity sync application could not be described")
)

// Integration is one row of ihp_identity_sync_integrations. CredentialRef is a
// reference (env:NAME or file:/path) to the sync client's secret, never the secret.
type Integration struct {
	ApplicationClientID string                       `json:"applicationId"`
	Origin              string                       `json:"origin"`
	SyncClientID        string                       `json:"syncClientId"`
	AccessRole          string                       `json:"accessRole"`
	CredentialRef       string                       `json:"credentialRef"`
	AccountIDFormat     identitysync.AccountIDFormat `json:"accountIdFormat"`
	// ContractVersion is the version last read from GET /; empty until checked.
	ContractVersion string    `json:"contractVersion"`
	Enabled         bool      `json:"enabled"`
	UpdatedBy       string    `json:"updatedBy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// RegistryRepository persists integrations. Save and Delete write the audit
// event (with actor and reason) in the same transaction as the change.
type RegistryRepository interface {
	GetIntegration(ctx context.Context, application string) (Integration, error)
	SaveIntegration(ctx context.Context, integration Integration, actor, reason string) (Integration, error)
	DeleteIntegration(ctx context.Context, application, actor, reason string) error
	// CheckIntegrationBinding re-checks, at use time, that the sync client is
	// still the application's enabled service identity and the access role is
	// still a person-facing access role.
	CheckIntegrationBinding(ctx context.Context, integration Integration) error
}

var (
	applicationIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	credentialRefPattern = regexp.MustCompile(`^(env:[A-Z][A-Z0-9_]*|file:/[^\s]+)$`)
	originPattern        = regexp.MustCompile(`^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$`)
)

// ValidOrigin reports whether origin is https://host[:port] exactly, as the
// registry and the contract require.
func ValidOrigin(origin string) bool {
	if !originPattern.MatchString(origin) {
		return false
	}
	_, ok := normalizeOrigin(origin)
	return ok
}

// Validate checks the fields an operator supplies.
func (i Integration) Validate() error {
	switch {
	case !applicationIDPattern.MatchString(i.ApplicationClientID):
		return fmt.Errorf("%w: application client ID", ErrInvalidIntegration)
	case !ValidOrigin(i.Origin):
		return fmt.Errorf("%w: origin must be https://host[:port] only", ErrInvalidIntegration)
	case !applicationIDPattern.MatchString(i.SyncClientID) || i.SyncClientID == i.ApplicationClientID:
		return fmt.Errorf("%w: sync client ID", ErrInvalidIntegration)
	case strings.TrimSpace(i.AccessRole) == "" || strings.TrimSpace(i.AccessRole) != i.AccessRole || len(i.AccessRole) > 255:
		return fmt.Errorf("%w: access role", ErrInvalidIntegration)
	case !credentialRefPattern.MatchString(i.CredentialRef) || len(i.CredentialRef) > 1024:
		return fmt.Errorf("%w: credential reference must be env:NAME or file:/absolute/path", ErrInvalidIntegration)
	}
	switch i.AccountIDFormat {
	case identitysync.AccountIDUUID, identitysync.AccountIDPositiveInteger, identitysync.AccountIDOpaque:
	default:
		return fmt.Errorf("%w: account ID format", ErrInvalidIntegration)
	}
	return nil
}

// ResolveCredential reads the secret a credential reference points to:
// "env:NAME" reads an environment variable and "file:/abs/path" reads a file.
// A single trailing newline (LF or CRLF) is trimmed and an empty secret is
// refused. Errors never include the secret.
func ResolveCredential(ref string, getenv func(string) string, readFile func(string) ([]byte, error)) (string, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if readFile == nil {
		readFile = os.ReadFile
	}
	if !credentialRefPattern.MatchString(ref) {
		return "", fmt.Errorf("%w: credential reference must be env:NAME or file:/absolute/path", ErrInvalidIntegration)
	}
	var secret string
	switch {
	case strings.HasPrefix(ref, "env:"):
		secret = getenv(strings.TrimPrefix(ref, "env:"))
	case strings.HasPrefix(ref, "file:"):
		data, err := readFile(strings.TrimPrefix(ref, "file:"))
		if err != nil {
			// The path is configuration, not a secret. Report only the OS cause.
			cause := err
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				cause = pathErr.Err
			}
			return "", fmt.Errorf("read identity sync credential %s: %w", ref, cause)
		}
		secret = string(data)
	}
	secret = strings.TrimSuffix(secret, "\n")
	secret = strings.TrimSuffix(secret, "\r")
	if secret == "" {
		return "", fmt.Errorf("identity sync credential %s is empty", ref)
	}
	return secret, nil
}

// RegistryService manages registered identity-sync integrations and builds
// contract connectors from them.
type RegistryService struct {
	Repository RegistryRepository
	// TokenEndpoint is the Keycloak client-credentials token endpoint, reached
	// through the (possibly internal) Keycloak base URL.
	TokenEndpoint string
	// Getenv and ReadFile resolve credential references; nil uses the OS.
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
	// HTTPClient, when set, supplies the transport for connectors and token
	// requests (tests use it to trust httptest TLS servers).
	HTTPClient *http.Client
	// Sleeper overrides the connector's retry sleeper (tests).
	Sleeper Sleeper
}

// NewRegistryService returns a registry service backed by the given repository.
func NewRegistryService(repository RegistryRepository, tokenEndpoint string) *RegistryService {
	return &RegistryService{Repository: repository, TokenEndpoint: tokenEndpoint}
}

// Get returns the integration for an application.
func (s *RegistryService) Get(ctx context.Context, application string) (Integration, error) {
	if !applicationIDPattern.MatchString(application) {
		return Integration{}, ErrIntegrationNotFound
	}
	return s.Repository.GetIntegration(ctx, application)
}

// Save validates and stores an integration. When it is enabled, the
// application's descriptor is read first (a 1.0 application is accepted when
// GET / returns 404) and its contract version is stored; an application that
// cannot be described is not enabled. A disabled save keeps the last known
// contract version.
func (s *RegistryService) Save(ctx context.Context, integration Integration, actor, reason string) (Integration, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 2000 {
		return Integration{}, fmt.Errorf("%w: a reason is required", ErrInvalidIntegration)
	}
	if err := integration.Validate(); err != nil {
		return Integration{}, err
	}
	integration.ContractVersion = ""
	if existing, err := s.Repository.GetIntegration(ctx, integration.ApplicationClientID); err == nil {
		if existing.Origin == integration.Origin && existing.AccountIDFormat == integration.AccountIDFormat {
			integration.ContractVersion = existing.ContractVersion
		}
	} else if !errors.Is(err, ErrIntegrationNotFound) {
		return Integration{}, err
	}
	if integration.Enabled {
		if err := s.Repository.CheckIntegrationBinding(ctx, integration); err != nil {
			return Integration{}, err
		}
		// An explicit (re-)register reads the descriptor afresh: it is the only
		// place a 1.1 registration may be downgraded to 1.0 on a 404.
		fresh := integration
		fresh.ContractVersion = ""
		descriptor, err := s.Probe(ctx, fresh)
		if err != nil {
			return Integration{}, fmt.Errorf("%w: enable %s: %v", ErrDescriptorUnavailable, integration.ApplicationClientID, err)
		}
		integration.ContractVersion = descriptor.Version
	}
	return s.Repository.SaveIntegration(ctx, integration, actor, reason)
}

// Delete removes an integration and records the reason.
func (s *RegistryService) Delete(ctx context.Context, application, actor, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 2000 {
		return fmt.Errorf("%w: a reason is required", ErrInvalidIntegration)
	}
	if !applicationIDPattern.MatchString(application) {
		return ErrIntegrationNotFound
	}
	return s.Repository.DeleteIntegration(ctx, application, actor, reason)
}

// Probe builds a connector for an integration (enabled or not) and reads its
// descriptor. With a stored contract version of 1.1 or newer a 404 is an error.
func (s *RegistryService) Probe(ctx context.Context, integration Integration) (identitysync.Descriptor, error) {
	connector, err := s.ConnectorFor(integration)
	if err != nil {
		return identitysync.Descriptor{}, err
	}
	return connector.Describe(ctx)
}

// Connector returns a contract connector for an enabled, registered
// application, after re-checking its service-identity and access-role binding.
func (s *RegistryService) Connector(ctx context.Context, application string) (*HTTPAccounts, Integration, error) {
	integration, err := s.Get(ctx, application)
	if err != nil {
		return nil, Integration{}, err
	}
	if !integration.Enabled {
		return nil, Integration{}, ErrIntegrationDisabled
	}
	if err := s.Repository.CheckIntegrationBinding(ctx, integration); err != nil {
		return nil, Integration{}, err
	}
	connector, err := s.ConnectorFor(integration)
	if err != nil {
		return nil, Integration{}, err
	}
	return connector, integration, nil
}

// ConnectorFor builds a connector from a registry row and its resolved
// credential. It does not check that the integration is enabled.
func (s *RegistryService) ConnectorFor(integration Integration) (*HTTPAccounts, error) {
	if err := integration.Validate(); err != nil {
		return nil, err
	}
	secret, err := ResolveCredential(integration.CredentialRef, s.Getenv, s.ReadFile)
	if err != nil {
		return nil, err
	}
	token, err := newClientCredentials(s.TokenEndpoint, integration.SyncClientID, secret, s.tokenClient())
	if err != nil {
		return nil, err
	}
	connector, err := NewHTTPAccounts(HTTPAccountsConfig{
		Application:           integration.ApplicationClientID,
		Origin:                integration.Origin,
		LegacyAccountIDFormat: integration.AccountIDFormat,
		StoredContractVersion: integration.ContractVersion,
		Token:                 token,
	})
	if err != nil {
		return nil, err
	}
	if s.HTTPClient != nil && s.HTTPClient.Transport != nil {
		connector.client.Transport = s.HTTPClient.Transport
	}
	if s.Sleeper != nil {
		connector.sleep = s.Sleeper
	}
	return connector, nil
}

func (s *RegistryService) tokenClient() *http.Client {
	client := &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if s.HTTPClient != nil && s.HTTPClient.Transport != nil {
		client.Transport = s.HTTPClient.Transport
	}
	return client
}
