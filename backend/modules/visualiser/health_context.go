package visualiser

import (
	"errors"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

var errHealthContextAliasMissing = errors.New("health context has no DWH organisation-unit alias")

var visualiserOrgUnitAliasNamespaces = map[string]struct{}{
	"dwh-org-unit":            {},
	"surveillance-region":     {},
	"surveillance-district":   {},
	"surveillance-sub-county": {},
	"surveillance-facility":   {},
}

// scopeDataValuesRequest constrains the requested organisation units to the
// warehouse aliases of the caller's selected health context. Without a
// selection the request is unconstrained.
func (h handlers) scopeDataValuesRequest(
	c *modulekit.Context,
	request DataValuesRequest,
) (DataValuesRequest, error) {
	selection := c.HealthContext()
	if selection == nil {
		return request, nil
	}
	contextID := strings.TrimSpace(selection.NodeID)
	if contextID == "" {
		return request, errHealthContextAliasMissing
	}

	node, err := h.healthContexts.GetNode(c.Context(), contextID)
	if err != nil {
		return request, err
	}
	includeDescendants := selection.IncludeDescendants

	// Uganda with descendant scope already represents the complete DWH domain.
	if node.Code == modulekit.NationalContextCode && includeDescendants {
		return request, nil
	}

	aliases, err := h.healthContexts.ListAliasesInScope(
		c.Context(),
		contextID,
		includeDescendants,
	)
	if err != nil {
		return request, err
	}
	allowed := make(map[string]struct{})
	for _, alias := range aliases {
		if _, supported := visualiserOrgUnitAliasNamespaces[alias.Namespace]; supported {
			externalID := strings.TrimSpace(alias.ExternalID)
			if externalID != "" {
				allowed[externalID] = struct{}{}
			}
		}
	}
	if len(allowed) == 0 {
		return request, errHealthContextAliasMissing
	}

	if len(request.OU) == 0 {
		request.OU = make([]string, 0, len(allowed))
		for externalID := range allowed {
			request.OU = append(request.OU, externalID)
		}
		return request, nil
	}

	scoped := make([]string, 0, len(request.OU))
	for _, externalID := range request.OU {
		if _, ok := allowed[strings.TrimSpace(externalID)]; ok {
			scoped = append(scoped, externalID)
		}
	}
	if len(scoped) == 0 {
		return request, errHealthContextAliasMissing
	}
	request.OU = scoped
	return request, nil
}
