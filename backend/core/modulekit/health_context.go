package modulekit

import (
	"context"
	"errors"
)

// HealthContextSelection is the caller's selected health-context node, already
// authorized by the host for context-scoped routes.
type HealthContextSelection struct {
	// NodeID is the selected node's UUID in canonical string form.
	NodeID string
	// IncludeDescendants is true when the caller's assignment covers the node
	// and everything beneath it, not only the node itself.
	IncludeDescendants bool
}

// HealthContextNode is a read-only view of a health-context hierarchy node.
type HealthContextNode struct {
	ID          string
	Code        string
	Name        string
	ContextType string
	ParentID    string
	Enabled     bool
}

// HealthContextAlias maps a node to an identifier in an external system
// (for example a warehouse organisation unit).
type HealthContextAlias struct {
	NodeID     string
	Namespace  string
	ExternalID string
}

// HealthContextDirectory is the host's read-only health-context hierarchy.
// Modules use it to translate a selection into their own identifiers; the
// host remains the authority for who may select which node.
type HealthContextDirectory interface {
	GetNode(ctx context.Context, nodeID string) (HealthContextNode, error)
	ListAncestors(ctx context.Context, nodeID string) ([]HealthContextNode, error)
	ListAliases(ctx context.Context, nodeID string) ([]HealthContextAlias, error)
	ListAliasesInScope(ctx context.Context, nodeID string, includeDescendants bool) ([]HealthContextAlias, error)
	ResolveAlias(ctx context.Context, namespace, externalID string) (HealthContextNode, error)
	IsDescendant(ctx context.Context, ancestorID, nodeID string) (bool, error)
}

// NationalContextCode is the code of the root (whole-country) node.
const NationalContextCode = "UG"

// Errors a HealthContextDirectory wraps so modules can branch on them.
var (
	ErrHealthContextNotFound = errors.New("health context not found")
	ErrHealthContextInvalid  = errors.New("invalid health context input")
)
