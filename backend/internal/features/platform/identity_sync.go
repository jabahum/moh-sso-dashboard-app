package platform

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// IdentitySyncRegistry is the identity-sync app registry used by the admin API.
// *identityreconcile.RegistryService implements it.
type IdentitySyncRegistry interface {
	Get(ctx context.Context, application string) (identityreconcile.Integration, error)
	Save(ctx context.Context, integration identityreconcile.Integration, actor, reason string) (identityreconcile.Integration, error)
	Delete(ctx context.Context, application, actor, reason string) error
}

// IdentitySyncInput registers or updates an application's identity-sync integration.
// CredentialRef names where the sync client's secret lives (env:NAME,
// file:/absolute/path, or keycloak:<sync client ID> to read it from Keycloak
// when needed); the secret itself is never sent or stored.
type IdentitySyncInput struct {
	Origin          string `json:"origin" binding:"required"`
	SyncClientID    string `json:"syncClientId" binding:"required"`
	AccessRole      string `json:"accessRole" binding:"required"`
	CredentialRef   string `json:"credentialRef" binding:"required"`
	AccountIDFormat string `json:"accountIdFormat" binding:"required"`
	Enabled         bool   `json:"enabled"`
	Reason          string `json:"reason" binding:"required"`
	// AllowContractDowngrade accepts that an application registered at 1.1 or
	// newer now answers like 1.0 (no GET / descriptor).
	AllowContractDowngrade bool `json:"allowContractDowngrade"`
}

// IdentitySyncDeleteInput records why an integration is removed.
type IdentitySyncDeleteInput struct {
	Reason string `json:"reason" binding:"required"`
}

// IdentitySyncIntegration is a registered identity-sync integration.
type IdentitySyncIntegration struct {
	ApplicationID   string    `json:"applicationId"`
	Origin          string    `json:"origin"`
	SyncClientID    string    `json:"syncClientId"`
	AccessRole      string    `json:"accessRole"`
	CredentialRef   string    `json:"credentialRef"`
	AccountIDFormat string    `json:"accountIdFormat"`
	ContractVersion string    `json:"contractVersion"`
	Enabled         bool      `json:"enabled"`
	UpdatedBy       string    `json:"updatedBy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// NewIdentitySyncRegistry builds the Postgres-backed registry service. Sync
// clients mint tokens at the realm's token endpoint under keycloakTokenBaseURL.
// secrets (the Keycloak admin client) resolves keycloak: credential
// references; nil refuses them.
func NewIdentitySyncRegistry(db *sql.DB, keycloakTokenBaseURL, realm string, secrets identityreconcile.ClientSecretSource) *identityreconcile.RegistryService {
	endpoint := strings.TrimRight(keycloakTokenBaseURL, "/") + "/realms/" + url.PathEscape(realm) + "/protocol/openid-connect/token"
	return identityreconcile.NewRegistryService(&identityreconcile.PostgresRegistry{DB: db}, endpoint).WithClientSecrets(secrets)
}

func identitySyncDTO(i identityreconcile.Integration) IdentitySyncIntegration {
	return IdentitySyncIntegration{
		ApplicationID: i.ApplicationClientID, Origin: i.Origin, SyncClientID: i.SyncClientID, AccessRole: i.AccessRole,
		CredentialRef: i.CredentialRef, AccountIDFormat: string(i.AccountIDFormat), ContractVersion: i.ContractVersion,
		Enabled: i.Enabled, UpdatedBy: i.UpdatedBy, CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt,
	}
}

func (h *Handler) GetIdentitySync(c *gin.Context) {
	if h.IdentitySync == nil {
		fail(c, ErrUnavailable)
		return
	}
	// An application without an integration is a normal state (the admin UI
	// shows "Not registered"), so it answers 200 with data: null, like the
	// identities, runs and revocations lists answer an empty list.
	var integration *IdentitySyncIntegration
	result, err := h.IdentitySync.Get(c.Request.Context(), c.Param("clientId"))
	switch {
	case err == nil:
		dto := identitySyncDTO(result)
		integration = &dto
	case !errors.Is(err, identityreconcile.ErrIntegrationNotFound):
		failIdentitySync(c, err)
		return
	}
	response.OK(c, http.StatusOK, integration)
}

// SaveIdentitySync registers or updates an integration. Enabling it first reads
// the application's GET / descriptor (a 1.0 application answers 404) and stores
// the contract version; an application that cannot be described is not enabled.
func (h *Handler) SaveIdentitySync(c *gin.Context) {
	if h.IdentitySync == nil {
		fail(c, ErrUnavailable)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	var input IdentitySyncInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, ErrInvalid)
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	integration := identityreconcile.Integration{
		ApplicationClientID: c.Param("clientId"),
		Origin:              input.Origin,
		SyncClientID:        input.SyncClientID,
		AccessRole:          input.AccessRole,
		CredentialRef:       input.CredentialRef,
		AccountIDFormat:     identitysync.AccountIDFormat(input.AccountIDFormat),
		Enabled:             input.Enabled,

		AllowContractDowngrade: input.AllowContractDowngrade,
	}
	if input.Reason == "" || len(input.Reason) > 2000 || !identityreconcile.ValidOrigin(input.Origin) || integration.Validate() != nil {
		fail(c, ErrInvalid)
		return
	}
	result, err := h.IdentitySync.Save(c.Request.Context(), integration, c.GetString("user_id"), input.Reason)
	if err != nil {
		failIdentitySync(c, err)
		return
	}
	response.OK(c, http.StatusOK, identitySyncDTO(result))
}

func (h *Handler) DeleteIdentitySync(c *gin.Context) {
	if h.IdentitySync == nil {
		fail(c, ErrUnavailable)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	var input IdentitySyncDeleteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, ErrInvalid)
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" || len(input.Reason) > 2000 {
		fail(c, ErrInvalid)
		return
	}
	if err := h.IdentitySync.Delete(c.Request.Context(), c.Param("clientId"), c.GetString("user_id"), input.Reason); err != nil {
		failIdentitySync(c, err)
		return
	}
	response.OK(c, http.StatusOK, gin.H{"applicationId": c.Param("clientId"), "deleted": true})
}

// failIdentitySync maps registry errors to stable responses without echoing
// upstream bodies, credential paths or secrets.
func failIdentitySync(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identityreconcile.ErrIntegrationNotFound):
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "identity sync integration or application not found")
	case errors.Is(err, identityreconcile.ErrInvalidIntegration):
		response.Fail(c, http.StatusBadRequest, "INVALID_IDENTITY_SYNC", "the sync client must be an enabled service identity of this application and the access role a person-facing access role")
	case errors.Is(err, identityreconcile.ErrIntegrationInUse):
		response.Fail(c, http.StatusConflict, "IDENTITY_SYNC_IN_USE", "identity sync integration is still referenced")
	case errors.Is(err, identityreconcile.ErrContractDowngrade):
		response.Fail(c, http.StatusConflict, "IDENTITY_SYNC_CONTRACT_DOWNGRADE", "the application was registered at contract 1.1 or newer but now answers like 1.0; retry later, or set allowContractDowngrade to accept it")
	case errors.Is(err, identityreconcile.ErrOriginUnresolvable):
		response.Fail(c, http.StatusUnprocessableEntity, "IDENTITY_SYNC_ORIGIN_UNRESOLVED", "the application's host name does not resolve from the portal server; add it to the DNS the portal server uses, then try again")
	case errors.Is(err, identityreconcile.ErrPrivateOrigin):
		response.Fail(c, http.StatusUnprocessableEntity, "IDENTITY_SYNC_PRIVATE_ORIGIN", "the application's host resolves to a private address; list it in "+identityreconcile.AllowedPrivateHostsEnv+" if it is meant to be reached on a private network, then try again")
	case errors.Is(err, identityreconcile.ErrDescriptorUnavailable):
		message := "the application's identity-sync endpoint could not be described; check the origin, credential reference and sync client"
		if problem := identityreconcile.ReachabilityProblem(err); problem != "" {
			message = "the application's identity-sync endpoint could not be reached: " + problem
		}
		response.Fail(c, http.StatusUnprocessableEntity, "IDENTITY_SYNC_UNREACHABLE", message)
	case identityreconcile.ReachabilityProblem(err) != "":
		response.Fail(c, http.StatusUnprocessableEntity, "IDENTITY_SYNC_UNREACHABLE", "the application's identity-sync endpoint could not be reached: "+identityreconcile.ReachabilityProblem(err))
	default:
		fail(c, err)
		return
	}
	c.Abort()
}
