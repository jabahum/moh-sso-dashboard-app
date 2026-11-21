package health_context

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/moh-sso-dashboard/internal/authz"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateNode(ctx context.Context, input CreateNodeInput) (Node, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Source = defaultString(input.Source, "PORTAL")
	if input.Code == "" || input.Name == "" || !validContextType(input.ContextType) {
		return Node{}, ErrInvalidInput
	}
	if input.ParentID != nil {
		parent, err := s.repository.GetNode(ctx, *input.ParentID)
		if err != nil {
			return Node{}, err
		}
		if !parent.Enabled {
			return Node{}, ErrInactiveContext
		}
	}
	return s.repository.CreateNode(ctx, input)
}

func (s *Service) UpdateNode(ctx context.Context, input UpdateNodeInput) (Node, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Source = defaultString(input.Source, "PORTAL")
	if input.ID == uuid.Nil || input.Code == "" || input.Name == "" ||
		!validContextType(input.ContextType) || input.Version < 1 {
		return Node{}, ErrInvalidInput
	}

	current, err := s.repository.GetNode(ctx, input.ID)
	if err != nil {
		return Node{}, err
	}
	if input.ParentID != nil {
		if *input.ParentID == input.ID {
			return Node{}, ErrCycle
		}
		parent, err := s.repository.GetNode(ctx, *input.ParentID)
		if err != nil {
			return Node{}, err
		}
		if !parent.Enabled {
			return Node{}, ErrInactiveContext
		}
		parentIsDescendant, err := s.repository.IsDescendant(ctx, input.ID, *input.ParentID)
		if err != nil {
			return Node{}, err
		}
		if parentIsDescendant {
			return Node{}, ErrCycle
		}
	}

	if current.ParentID == nil && input.ParentID == nil {
		return s.repository.UpdateNode(ctx, input)
	}
	if current.ParentID != nil && input.ParentID != nil && *current.ParentID == *input.ParentID {
		return s.repository.UpdateNode(ctx, input)
	}
	return s.repository.UpdateNode(ctx, input)
}

func (s *Service) DeleteNode(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidInput
	}
	if _, err := s.repository.GetNode(ctx, id); err != nil {
		return err
	}
	return s.repository.DeleteNode(ctx, id)
}

func (s *Service) GetNode(ctx context.Context, id uuid.UUID) (Node, error) {
	return s.repository.GetNode(ctx, id)
}

func (s *Service) ListNodes(
	ctx context.Context,
	parentID *uuid.UUID,
	contextType ContextType,
	includeDisabled bool,
) ([]Node, error) {
	if contextType != "" && !validContextType(contextType) {
		return nil, ErrInvalidInput
	}
	return s.repository.ListNodes(ctx, parentID, contextType, includeDisabled)
}

func (s *Service) ListAncestors(ctx context.Context, id uuid.UUID) ([]Node, error) {
	return s.repository.ListAncestors(ctx, id)
}

func (s *Service) ListDescendants(ctx context.Context, id uuid.UUID) ([]Node, error) {
	return s.repository.ListDescendants(ctx, id)
}

func (s *Service) UpsertAlias(ctx context.Context, input UpsertAliasInput) (Alias, error) {
	input.Namespace = strings.ToLower(strings.TrimSpace(input.Namespace))
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	if input.ContextNodeID == uuid.Nil || input.Namespace == "" || input.ExternalID == "" {
		return Alias{}, ErrInvalidInput
	}
	if _, err := s.repository.GetNode(ctx, input.ContextNodeID); err != nil {
		return Alias{}, err
	}
	return s.repository.UpsertAlias(ctx, input)
}

func (s *Service) ResolveAlias(
	ctx context.Context,
	namespace string,
	externalID string,
) (Node, error) {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	externalID = strings.TrimSpace(externalID)
	if namespace == "" || externalID == "" {
		return Node{}, ErrInvalidInput
	}
	return s.repository.GetNodeByAlias(ctx, namespace, externalID)
}

// IsDescendant reports whether descendantID is contained by ancestorID.
// Feature adapters call it only after resolving domain IDs through aliases.
func (s *Service) IsDescendant(
	ctx context.Context,
	ancestorID uuid.UUID,
	descendantID uuid.UUID,
) (bool, error) {
	if ancestorID == uuid.Nil || descendantID == uuid.Nil {
		return false, ErrInvalidInput
	}
	return s.repository.IsDescendant(ctx, ancestorID, descendantID)
}

func (s *Service) ListAliases(ctx context.Context, contextID uuid.UUID) ([]Alias, error) {
	if contextID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.repository.GetNode(ctx, contextID); err != nil {
		return nil, err
	}
	return s.repository.ListAliases(ctx, contextID)
}

func (s *Service) ListAliasesInScope(
	ctx context.Context,
	contextID uuid.UUID,
	includeDescendants bool,
) ([]Alias, error) {
	if contextID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.repository.GetNode(ctx, contextID); err != nil {
		return nil, err
	}
	return s.repository.ListAliasesInScope(ctx, contextID, includeDescendants)
}

func (s *Service) DeleteAlias(ctx context.Context, contextID uuid.UUID, aliasID uuid.UUID) error {
	if contextID == uuid.Nil || aliasID == uuid.Nil {
		return ErrInvalidInput
	}
	return s.repository.DeleteAlias(ctx, contextID, aliasID)
}

func (s *Service) ListEffectiveContexts(ctx context.Context, userID string) ([]EffectiveContext, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidInput
	}
	contexts, err := s.repository.ListEffectiveContexts(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(contexts) == 0 {
		return contexts, nil
	}

	hasActive := false
	for i := range contexts {
		if contexts[i].IsActive {
			hasActive = true
			break
		}
	}
	if !hasActive {
		for i := range contexts {
			if contexts[i].IsDefault {
				contexts[i].IsActive = true
				break
			}
		}
	}
	return contexts, nil
}

func (s *Service) ResolveAudienceUserIDs(
	ctx context.Context,
	actorID string,
	contextIDs []uuid.UUID,
	includeDescendants bool,
) ([]string, error) {
	if strings.TrimSpace(actorID) == "" || len(contextIDs) == 0 {
		return nil, ErrInvalidInput
	}

	seen := make(map[uuid.UUID]struct{}, len(contextIDs))
	cleaned := make([]uuid.UUID, 0, len(contextIDs))
	for _, contextID := range contextIDs {
		if contextID == uuid.Nil {
			return nil, ErrInvalidInput
		}
		if _, exists := seen[contextID]; exists {
			continue
		}
		explanation, err := s.ExplainAccess(ctx, actorID, contextID)
		if err != nil {
			return nil, err
		}
		if !explanation.Allowed {
			return nil, ErrContextForbidden
		}
		seen[contextID] = struct{}{}
		cleaned = append(cleaned, contextID)
	}

	return s.repository.ListAudienceUserIDs(ctx, cleaned, includeDescendants)
}

func (s *Service) ListAudienceUserIDs(
	ctx context.Context,
	contextIDs []uuid.UUID,
	includeDescendants bool,
) ([]string, error) {
	if len(contextIDs) == 0 {
		return nil, ErrInvalidInput
	}
	for _, contextID := range contextIDs {
		if contextID == uuid.Nil {
			return nil, ErrInvalidInput
		}
		node, err := s.repository.GetNode(ctx, contextID)
		if err != nil {
			return nil, err
		}
		if !node.Enabled {
			return nil, ErrInactiveContext
		}
	}
	return s.repository.ListAudienceUserIDs(ctx, contextIDs, includeDescendants)
}

func (s *Service) ResolveAuthAccess(
	ctx context.Context,
	userID string,
) ([]authz.HealthContextAccess, *authz.HealthContextAccess, error) {
	contexts, err := s.ListEffectiveContexts(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	result := make([]authz.HealthContextAccess, 0, len(contexts))
	var active *authz.HealthContextAccess
	for _, item := range contexts {
		access := authz.HealthContextAccess{
			ID: item.ID.String(), Code: item.Code, Name: item.Name, Type: string(item.ContextType),
			ScopeMode: string(item.ScopeMode), Source: item.AssignmentType,
			SourceGroupPath: item.SourceGroupPath, IsDefault: item.IsDefault,
			IsActive: item.IsActive,
		}
		result = append(result, access)
		if access.IsActive {
			selected := access
			active = &selected
		}
	}
	if active == nil && len(result) > 0 {
		selected := result[0]
		active = &selected
	}
	return result, active, nil
}

func (s *Service) ReplaceUserAssignments(
	ctx context.Context,
	userID string,
	actorID string,
	inputs []AssignmentInput,
) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(actorID) == "" {
		return ErrInvalidAssignment
	}
	if err := s.validateAssignments(ctx, inputs, true); err != nil {
		return err
	}
	return s.repository.ReplaceUserAssignments(ctx, userID, actorID, inputs)
}

func (s *Service) ReplaceGroupAssignments(
	ctx context.Context,
	groupID uuid.UUID,
	actorID string,
	inputs []AssignmentInput,
) error {
	if groupID == uuid.Nil || strings.TrimSpace(actorID) == "" {
		return ErrInvalidAssignment
	}
	if err := s.validateAssignments(ctx, inputs, false); err != nil {
		return err
	}
	return s.repository.ReplaceGroupAssignments(ctx, groupID, actorID, inputs)
}

func (s *Service) ListUserAssignments(ctx context.Context, userID string) ([]Assignment, error) {
	return s.repository.ListUserAssignments(ctx, userID)
}

func (s *Service) ListGroupAssignments(ctx context.Context, groupID uuid.UUID) ([]Assignment, error) {
	return s.repository.ListGroupAssignments(ctx, groupID)
}

func (s *Service) GetGroupMappingDrift(ctx context.Context) ([]GroupMappingDrift, error) {
	return s.repository.ListGroupMappingDrift(ctx)
}

func (s *Service) PreviewGroupContextSync(
	ctx context.Context,
	mappings []GroupContextSyncMapping,
) ([]GroupMappingDrift, error) {
	if err := s.validateGroupContextSyncMappings(ctx, mappings); err != nil {
		return nil, err
	}
	return s.repository.ListGroupMappingDrift(ctx)
}

func (s *Service) ApplyGroupContextSync(
	ctx context.Context,
	actorID string,
	mappings []GroupContextSyncMapping,
	replaceExisting bool,
) (GroupContextSyncResult, error) {
	if strings.TrimSpace(actorID) == "" {
		return GroupContextSyncResult{}, ErrInvalidAssignment
	}
	if err := s.validateGroupContextSyncMappings(ctx, mappings); err != nil {
		return GroupContextSyncResult{}, err
	}
	return s.repository.ApplyGroupContextMappings(
		ctx,
		actorID,
		mappings,
		replaceExisting,
	)
}

func (s *Service) validateGroupContextSyncMappings(
	ctx context.Context,
	mappings []GroupContextSyncMapping,
) error {
	if len(mappings) == 0 {
		return ErrInvalidAssignment
	}
	seen := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		if mapping.GroupID == uuid.Nil ||
			mapping.ContextNodeID == uuid.Nil ||
			!validScopeMode(mapping.ScopeMode) {
			return ErrInvalidAssignment
		}
		key := mapping.GroupID.String() + ":" + mapping.ContextNodeID.String()
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidAssignment
		}
		seen[key] = struct{}{}
		node, err := s.repository.GetNode(ctx, mapping.ContextNodeID)
		if err != nil {
			return err
		}
		if !node.Enabled {
			return ErrInactiveContext
		}
	}
	return nil
}

func (s *Service) SelectActiveContext(ctx context.Context, userID string, contextID uuid.UUID) error {
	explanation, err := s.ExplainAccess(ctx, userID, contextID)
	if err != nil {
		return err
	}
	if !explanation.Allowed {
		return ErrContextForbidden
	}
	return s.repository.SetActiveContext(ctx, userID, contextID)
}

func (s *Service) ExplainAccess(
	ctx context.Context,
	userID string,
	requestedID uuid.UUID,
) (AccessExplanation, error) {
	requested, err := s.repository.GetNode(ctx, requestedID)
	if err != nil {
		return AccessExplanation{}, err
	}
	if !requested.Enabled {
		return AccessExplanation{
			RequestedContext: requested,
			Reason:           "requested context is inactive",
		}, nil
	}

	assignments, err := s.ListEffectiveContexts(ctx, userID)
	if err != nil {
		return AccessExplanation{}, err
	}
	scopes := make([]core.ContextAssignment[uuid.UUID], len(assignments))
	for i, assignment := range assignments {
		scopes[i] = core.ContextAssignment[uuid.UUID]{ContextID: assignment.ID, ScopeMode: string(assignment.ScopeMode), AssignmentType: assignment.AssignmentType}
	}
	decision, err := core.ResolveContextScope(ctx, s.repository, requested.ID, requested.Enabled, scopes)
	if err != nil {
		return AccessExplanation{}, err
	}
	explanation := AccessExplanation{Allowed: decision.Allowed, RequestedContext: requested, Reason: decision.Reason}
	if decision.Allowed {
		assignment := assignments[decision.AssignmentIndex]
		node := assignment.Node
		explanation.AssignmentContext = &node
		explanation.ScopeMode = assignment.ScopeMode
		explanation.AssignmentType = assignment.AssignmentType
		explanation.SourceGroupID = assignment.SourceGroupID
		explanation.SourceGroupPath = assignment.SourceGroupPath
	}
	return explanation, nil
}

func (s *Service) validateAssignments(
	ctx context.Context,
	inputs []AssignmentInput,
	allowDefault bool,
) error {
	defaults := 0
	seen := make(map[uuid.UUID]struct{}, len(inputs))
	now := time.Now()
	for _, input := range inputs {
		if input.ContextNodeID == uuid.Nil || !validScopeMode(input.ScopeMode) {
			return ErrInvalidAssignment
		}
		if _, duplicate := seen[input.ContextNodeID]; duplicate {
			return ErrInvalidAssignment
		}
		seen[input.ContextNodeID] = struct{}{}
		node, err := s.repository.GetNode(ctx, input.ContextNodeID)
		if err != nil {
			return err
		}
		if !node.Enabled {
			return ErrInactiveContext
		}
		if input.ValidUntil != nil && input.ValidUntil.Before(now) {
			return ErrInvalidAssignment
		}
		if input.ValidFrom != nil && input.ValidUntil != nil && !input.ValidUntil.After(*input.ValidFrom) {
			return ErrInvalidAssignment
		}
		if input.IsDefault {
			defaults++
		}
	}
	if !allowDefault && defaults > 0 {
		return ErrInvalidAssignment
	}
	if defaults > 1 {
		return ErrInvalidAssignment
	}
	return nil
}

func validScopeMode(value ScopeMode) bool {
	return value == ScopeNodeOnly || value == ScopeNodeAndDescendants
}

func validContextType(value ContextType) bool {
	switch value {
	case ContextNational, ContextRegion, ContextDistrict, ContextCity, ContextDivision,
		ContextMunicipality, ContextCounty, ContextSubCounty, ContextParish,
		ContextFacility, ContextProgram, ContextDepartment, ContextTeam, ContextCustom:
		return true
	default:
		return false
	}
}

func IsAccessDenied(err error) bool {
	return errors.Is(err, ErrContextForbidden) || errors.Is(err, ErrInactiveContext)
}
