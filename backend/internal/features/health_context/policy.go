package health_context

import (
	"context"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/moh-sso-dashboard/internal/authz"
)

type Policy struct {
	service *Service
}

func NewPolicy(service *Service) *Policy {
	return &Policy{service: service}
}

type AuthorizationInput struct {
	UserID             string
	Permissions        []authz.Permission
	RequiredPermission authz.Permission
	Systems            []string
	RequiredSystem     string
	ContextID          uuid.UUID
}

type AliasAuthorizationInput struct {
	UserID             string
	Permissions        []authz.Permission
	RequiredPermission authz.Permission
	Systems            []string
	RequiredSystem     string
	Namespace          string
	ExternalID         string
}

func (p *Policy) Authorize(ctx context.Context, input AuthorizationInput) (AccessExplanation, error) {
	permissions := make([]string, len(input.Permissions))
	for i, permission := range input.Permissions {
		permissions[i] = string(permission)
	}
	decision, err := core.AuthorizeContext[uuid.UUID, AccessExplanation](ctx, coreContextResolver{p.service}, core.ContextAuthorization[uuid.UUID]{
		Subject: input.UserID, Permissions: permissions, RequiredPermission: string(input.RequiredPermission),
		Systems: input.Systems, RequiredSystem: input.RequiredSystem, ContextID: input.ContextID,
	})
	if err != nil && err != core.ErrContextForbidden {
		return AccessExplanation{}, err
	}
	explanation := decision.Details
	explanation.Allowed, explanation.Reason = decision.Allowed, decision.Reason
	return explanation, err
}

type coreContextResolver struct{ service *Service }

var _ core.ContextAccessResolver[uuid.UUID, AccessExplanation] = coreContextResolver{}

func (r coreContextResolver) ExplainContextAccess(ctx context.Context, userID string, id uuid.UUID) (core.ContextDecision[AccessExplanation], error) {
	explanation, err := r.service.ExplainAccess(ctx, userID, id)
	if err != nil {
		return core.ContextDecision[AccessExplanation]{}, err
	}
	return core.ContextDecision[AccessExplanation]{Allowed: explanation.Allowed, Reason: explanation.Reason, Details: explanation}, nil
}

func (p *Policy) AuthorizeAlias(
	ctx context.Context,
	input AliasAuthorizationInput,
) (AccessExplanation, error) {
	node, err := p.service.ResolveAlias(ctx, input.Namespace, input.ExternalID)
	if err != nil {
		return AccessExplanation{}, err
	}
	return p.Authorize(ctx, AuthorizationInput{
		UserID:             input.UserID,
		Permissions:        input.Permissions,
		RequiredPermission: input.RequiredPermission,
		Systems:            input.Systems,
		RequiredSystem:     input.RequiredSystem,
		ContextID:          node.ID,
	})
}
