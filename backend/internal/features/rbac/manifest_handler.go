package rbac

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/module"
)

func (s *Service) skipManifestStartup(ctx context.Context, clientID string) (bool, error) {
	if !systemrbac.IsStartupDiscovery(ctx) {
		return false, nil
	}
	repository, ok := s.repository.(manifestRepository)
	if !ok {
		return false, nil
	}
	return repository.HasApplicationManifest(ctx, clientID)
}

func (s *Service) ConfigureManifests(environment string, modules []module.Info) {
	s.manifestEnvironment = environment
	s.manifestModules = append([]module.Info(nil), modules...)
}

func bindManifestRequest(c *gin.Context) (ManifestRequest, bool) {
	var input ManifestRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*maxManifestBytes)
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_MANIFEST", "Provide a manifest document of at most 128 KiB")
		return input, false
	}
	return input, true
}

func (h *Handler) ValidateApplicationManifest(c *gin.Context) {
	input, ok := bindManifestRequest(c)
	if !ok {
		return
	}
	manifest, _, err := parseApplicationManifest(input.Document, h.service.manifestEnvironment, h.service.manifestModules)
	if err != nil {
		writeError(c, err, "MANIFEST_VALIDATION_FAILED")
		return
	}
	response.OK(c, http.StatusOK, manifest)
}

func (h *Handler) PreviewApplicationManifest(c *gin.Context) { h.reviewApplicationManifest(c, false) }
func (h *Handler) ImportApplicationManifest(c *gin.Context)  { h.reviewApplicationManifest(c, true) }

func (h *Handler) reviewApplicationManifest(c *gin.Context, apply bool) {
	input, ok := bindManifestRequest(c)
	if !ok {
		return
	}
	manifest, configuration, err := parseApplicationManifest(input.Document, h.service.manifestEnvironment, h.service.manifestModules)
	if err != nil {
		writeError(c, err, "MANIFEST_VALIDATION_FAILED")
		return
	}
	repository, ok := h.service.repository.(manifestRepository)
	if !ok {
		response.Fail(c, http.StatusServiceUnavailable, "MANIFESTS_UNAVAILABLE", "Application manifest storage is unavailable")
		return
	}
	preview, err := repository.ReviewManifest(c.Request.Context(), manifest, configuration, input.ReviewToken, apply, c.GetString("user_id"))
	if errors.Is(err, ErrManifestReview) {
		response.Fail(c, http.StatusConflict, "MANIFEST_REVIEW_REQUIRED", "Registry configuration changed. Preview and review the manifest again.")
		return
	}
	if err != nil {
		writeError(c, err, "MANIFEST_IMPORT_FAILED")
		return
	}
	response.OK(c, http.StatusOK, preview)
}

type ManifestRuntime struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Launch  ManifestLaunch `json:"launch"`
}

func (h *Handler) GetApplicationManifestRuntime(c *gin.Context) {
	actor, ok := authz.FromGin(c)
	clientID := c.Param("clientId")
	allowed := false
	if ok {
		for _, system := range actor.AccessibleSystems {
			if system == clientID {
				allowed = true
			}
		}
		allowed = actor.HasPermission(authz.PermissionSystemsLaunch) && (allowed || actor.IsAdmin)
	}
	if !allowed {
		response.Fail(c, http.StatusForbidden, "FORBIDDEN", "Application access is required")
		return
	}
	repository, ok := h.service.repository.(manifestRepository)
	if !ok {
		response.Fail(c, http.StatusServiceUnavailable, "MANIFESTS_UNAVAILABLE", "Application manifest storage is unavailable")
		return
	}
	manifest, err := repository.LoadApplicationManifest(c.Request.Context(), clientID)
	if err != nil {
		writeError(c, err, "APPLICATION_UNAVAILABLE")
		return
	}
	response.OK(c, http.StatusOK, ManifestRuntime{ID: manifest.Metadata.ID, Name: manifest.Metadata.Name, Version: manifest.Metadata.Version, Launch: manifest.Spec.Launch})
}
