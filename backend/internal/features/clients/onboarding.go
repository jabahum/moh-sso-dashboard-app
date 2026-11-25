package clients

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/model"
)

type onboardingRequest struct {
	Spec        keycloak.OnboardingSpec `json:"spec"`
	ReviewToken string                  `json:"reviewToken"`
}

type onboardingPreview struct {
	Spec        keycloak.OnboardingSpec `json:"spec"`
	Realm       string                  `json:"realm"`
	Issuer      string                  `json:"issuer"`
	Operation   string                  `json:"operation"`
	Client      ClientResponse          `json:"client"`
	ReviewToken string                  `json:"reviewToken"`
	Existing    *ClientResponse         `json:"existing,omitempty"`
}

func (s *Service) previewOnboarding(spec keycloak.OnboardingSpec) (*onboardingPreview, *model.Client, error) {
	if s.cfg == nil || s.cfg.KeycloakRealm == "" {
		return nil, nil, fmt.Errorf("deployment Keycloak configuration is missing")
	}
	if err := keycloak.ValidateIdentityEnvironment(s.cfg.Environment, s.cfg.KeycloakRealm, s.cfg.IdentityEnvironmentRealm); err != nil {
		return nil, nil, err
	}
	client, err := keycloak.BuildOnboardingClient(spec, s.cfg.Environment)
	if err != nil {
		return nil, nil, err
	}
	preview := &onboardingPreview{Spec: spec, Realm: s.cfg.KeycloakRealm, Operation: "create", Client: toClientResponse(client)}
	preview.Issuer = strings.TrimRight(s.cfg.KeycloakExternalURL, "/") + "/realms/" + s.cfg.KeycloakRealm
	clients, err := s.repo.ListClients()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot check existing client configuration")
	}
	for _, item := range clients {
		if item.ClientID == spec.ClientID {
			if item.Attributes["portal.onboarding.profile"] != spec.Profile || item.Attributes["portal.onboarding.environment"] != s.cfg.Environment {
				return nil, nil, fmt.Errorf("existing client is not managed by this onboarding profile; use its existing administration workflow")
			}
			id, parseErr := uuid.Parse(item.ID)
			if parseErr != nil {
				return nil, nil, parseErr
			}
			existing, getErr := s.repo.GetClientByID(id)
			if getErr != nil {
				return nil, nil, fmt.Errorf("cannot read existing client")
			}
			// Updating an active integration requires disabling it explicitly first.
			if existing.Enabled {
				return nil, nil, fmt.Errorf("disable the existing client before applying an onboarding update")
			}
			client.ID = existing.ID
			for key, value := range existing.Attributes {
				if _, managed := client.Attributes[key]; !managed && key != "access.token.lifespan" {
					client.Attributes[key] = value
				}
			}
			current := toClientResponse(existing)
			preview.Existing = &current
			preview.Operation = "update"
			preview.Client = toClientResponse(client)
		}
	}
	raw, _ := json.Marshal(preview)
	sum := sha256.Sum256(raw)
	preview.ReviewToken = hex.EncodeToString(sum[:])
	return preview, client, nil
}

func (h *Handler) OnboardingContext(c *gin.Context) {
	if h.service.cfg == nil {
		response.Fail(c, 503, "NOT_CONFIGURED", "Onboarding is not configured")
		return
	}
	cfg := h.service.cfg
	if err := keycloak.ValidateIdentityEnvironment(cfg.Environment, cfg.KeycloakRealm, cfg.IdentityEnvironmentRealm); err != nil {
		response.Fail(c, 503, "ENVIRONMENT_MISMATCH", err.Error())
		return
	}
	response.OK(c, 200, gin.H{"environment": cfg.Environment, "realm": cfg.KeycloakRealm, "issuer": strings.TrimRight(cfg.KeycloakExternalURL, "/") + "/realms/" + cfg.KeycloakRealm, "profiles": keycloak.ClientProfiles()})
}

func (h *Handler) PreviewOnboarding(c *gin.Context) { h.onboarding(c, false) }
func (h *Handler) ApplyOnboarding(c *gin.Context)   { h.onboarding(c, true) }
func (h *Handler) onboarding(c *gin.Context, apply bool) {
	var req onboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "VALIDATION_FAILED", "Invalid onboarding document")
		return
	}
	preview, client, err := h.service.previewOnboarding(req.Spec)
	if err != nil {
		response.Fail(c, 400, "VALIDATION_FAILED", err.Error())
		return
	}
	if !apply {
		response.OK(c, 200, preview)
		return
	}
	if req.ReviewToken == "" || req.ReviewToken != preview.ReviewToken {
		response.Fail(c, 409, "REVIEW_REQUIRED", "Configuration changed; preview and review it again")
		return
	}
	if preview.Operation == "create" {
		client.ID, err = h.service.repo.CreateClient(client)
	} else {
		err = h.service.repo.UpdateClient(client)
	}
	if err != nil {
		h.audit(c, "client.onboarding_failed", map[string]any{"client_id": req.Spec.ClientID, "environment": req.Spec.Environment})
		response.Fail(c, 502, "PROVISIONING_FAILED", "Provisioning did not complete. The client remains disabled; review its draft and retry.")
		return
	}
	h.audit(c, "client.onboarding_applied", map[string]any{"client_id": req.Spec.ClientID, "environment": req.Spec.Environment, "realm": preview.Realm, "profile": req.Spec.Profile, "operation": preview.Operation})
	h.invalidateClientsCache(c.Request.Context())
	response.OK(c, http.StatusOK, gin.H{"client": toClientResponse(client), "issuer": preview.Issuer, "message": "Provisioned and verified. Client is disabled; complete integration checks and access mapping before enabling it."})
}
