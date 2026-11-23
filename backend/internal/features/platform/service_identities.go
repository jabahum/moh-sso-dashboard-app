package platform

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

// ServiceAccountResolver finds the Keycloak user ID of a client's service
// account (*keycloak.KeyAdminClient implements it), so an administrator never
// has to copy it out of Keycloak when registering a service identity.
type ServiceAccountResolver interface {
	ServiceAccountUserID(ctx context.Context, clientID string) (string, error)
}

// ServiceAccount is the resolved service account of a Keycloak client.
type ServiceAccount struct {
	ClientID string `json:"clientId"`
	Subject  string `json:"subject"`
}

// ListIdentities returns the application's registered service identities.
func (h *Handler) ListIdentities(c *gin.Context) {
	identities, err := h.Repository.ListIdentities(c.Request.Context(), c.Param("clientId"))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, gin.H{"identities": identities})
}

// GetServiceAccount resolves a Keycloak client's service-account user ID.
func (h *Handler) GetServiceAccount(c *gin.Context) {
	if h.ServiceAccounts == nil {
		fail(c, ErrUnavailable)
		return
	}
	client := c.Param("oauthClientId")
	subject, err := h.ServiceAccounts.ServiceAccountUserID(c.Request.Context(), client)
	if errors.Is(err, keycloak.ErrServiceAccountNotFound) {
		response.Fail(c, http.StatusNotFound, "SERVICE_ACCOUNT_NOT_FOUND", "the Keycloak client does not exist or has no service account")
		c.Abort()
		return
	}
	if err != nil {
		fail(c, ErrUnavailable)
		return
	}
	response.OK(c, http.StatusOK, ServiceAccount{ClientID: client, Subject: subject})
}
