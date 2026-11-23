package health_context

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// ModuleDirectory exposes the hierarchy to modules through the core contract.
// Errors keep their host cause and also match the modulekit sentinels.
type ModuleDirectory struct{ Service *Service }

var _ modulekit.HealthContextDirectory = ModuleDirectory{}

func (d ModuleDirectory) GetNode(ctx context.Context, nodeID string) (modulekit.HealthContextNode, error) {
	id, err := parseModuleID(nodeID)
	if err != nil {
		return modulekit.HealthContextNode{}, err
	}
	node, err := d.Service.GetNode(ctx, id)
	return toModuleNode(node), moduleError(err)
}

func (d ModuleDirectory) ListAncestors(ctx context.Context, nodeID string) ([]modulekit.HealthContextNode, error) {
	id, err := parseModuleID(nodeID)
	if err != nil {
		return nil, err
	}
	nodes, err := d.Service.ListAncestors(ctx, id)
	if err != nil {
		return nil, moduleError(err)
	}
	out := make([]modulekit.HealthContextNode, len(nodes))
	for i, node := range nodes {
		out[i] = toModuleNode(node)
	}
	return out, nil
}

func (d ModuleDirectory) ListAliases(ctx context.Context, nodeID string) ([]modulekit.HealthContextAlias, error) {
	id, err := parseModuleID(nodeID)
	if err != nil {
		return nil, err
	}
	aliases, err := d.Service.ListAliases(ctx, id)
	return toModuleAliases(aliases), moduleError(err)
}

func (d ModuleDirectory) ListAliasesInScope(ctx context.Context, nodeID string, includeDescendants bool) ([]modulekit.HealthContextAlias, error) {
	id, err := parseModuleID(nodeID)
	if err != nil {
		return nil, err
	}
	aliases, err := d.Service.ListAliasesInScope(ctx, id, includeDescendants)
	return toModuleAliases(aliases), moduleError(err)
}

func (d ModuleDirectory) ResolveAlias(ctx context.Context, namespace, externalID string) (modulekit.HealthContextNode, error) {
	node, err := d.Service.ResolveAlias(ctx, namespace, externalID)
	return toModuleNode(node), moduleError(err)
}

func (d ModuleDirectory) IsDescendant(ctx context.Context, ancestorID, nodeID string) (bool, error) {
	ancestor, err := parseModuleID(ancestorID)
	if err != nil {
		return false, err
	}
	node, err := parseModuleID(nodeID)
	if err != nil {
		return false, err
	}
	contained, err := d.Service.IsDescendant(ctx, ancestor, node)
	return contained, moduleError(err)
}

func parseModuleID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %w", modulekit.ErrHealthContextInvalid, ErrInvalidInput)
	}
	return id, nil
}

func moduleError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound):
		return fmt.Errorf("%w: %w", modulekit.ErrHealthContextNotFound, err)
	case errors.Is(err, ErrInvalidInput):
		return fmt.Errorf("%w: %w", modulekit.ErrHealthContextInvalid, err)
	default:
		return err
	}
}

func toModuleNode(node Node) modulekit.HealthContextNode {
	out := modulekit.HealthContextNode{
		Code: node.Code, Name: node.Name, ContextType: string(node.ContextType), Enabled: node.Enabled,
	}
	if node.ID != uuid.Nil {
		out.ID = node.ID.String()
	}
	if node.ParentID != nil {
		out.ParentID = node.ParentID.String()
	}
	return out
}

func toModuleAliases(aliases []Alias) []modulekit.HealthContextAlias {
	if aliases == nil {
		return nil
	}
	out := make([]modulekit.HealthContextAlias, len(aliases))
	for i, alias := range aliases {
		out[i] = modulekit.HealthContextAlias{NodeID: alias.ContextNodeID.String(), Namespace: alias.Namespace, ExternalID: alias.ExternalID}
	}
	return out
}
