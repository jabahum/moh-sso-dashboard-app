package health_context

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

type ContextType string

const (
	ContextNational     ContextType = "NATIONAL"
	ContextRegion       ContextType = "REGION"
	ContextDistrict     ContextType = "DISTRICT"
	ContextCity         ContextType = "CITY"
	ContextDivision     ContextType = "DIVISION"
	ContextMunicipality ContextType = "MUNICIPALITY"
	ContextCounty       ContextType = "COUNTY"
	ContextSubCounty    ContextType = "SUB_COUNTY"
	ContextParish       ContextType = "PARISH"
	ContextFacility     ContextType = "FACILITY"
	ContextProgram      ContextType = "PROGRAM"
	ContextDepartment   ContextType = "DEPARTMENT"
	ContextTeam         ContextType = "TEAM"
	ContextCustom       ContextType = "CUSTOM"
)

type ScopeMode string

const (
	ScopeNodeOnly           ScopeMode = core.ContextNodeOnly
	ScopeNodeAndDescendants ScopeMode = core.ContextNodeAndDescendants
)

type Node struct {
	ID          uuid.UUID
	Code        string
	Name        string
	ContextType ContextType
	ParentID    *uuid.UUID
	Source      string
	Metadata    json.RawMessage
	Enabled     bool
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Alias struct {
	ID            uuid.UUID
	ContextNodeID uuid.UUID
	Namespace     string
	ExternalID    string
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

type Assignment struct {
	ID              uuid.UUID
	UserID          string
	GroupID         *uuid.UUID
	ContextNodeID   uuid.UUID
	ScopeMode       ScopeMode
	IsDefault       bool
	ValidFrom       *time.Time
	ValidUntil      *time.Time
	Source          string
	SourceReference *string
	CreatedBy       *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type EffectiveContext struct {
	Node
	ScopeMode       ScopeMode
	AssignmentType  string
	AssignmentID    uuid.UUID
	SourceGroupID   *uuid.UUID
	SourceGroupPath *string
	IsDefault       bool
	IsActive        bool
}

type AccessExplanation struct {
	Allowed           bool
	RequestedContext  Node
	AssignmentContext *Node
	ScopeMode         ScopeMode
	AssignmentType    string
	SourceGroupID     *uuid.UUID
	SourceGroupPath   *string
	Reason            string
}

type CreateNodeInput struct {
	Code        string
	Name        string
	ContextType ContextType
	ParentID    *uuid.UUID
	Source      string
	Metadata    json.RawMessage
	Enabled     bool
}

type UpdateNodeInput struct {
	ID          uuid.UUID
	Code        string
	Name        string
	ContextType ContextType
	ParentID    *uuid.UUID
	Source      string
	Metadata    json.RawMessage
	Enabled     bool
	Version     int
}

type UpsertAliasInput struct {
	ContextNodeID uuid.UUID
	Namespace     string
	ExternalID    string
	Metadata      json.RawMessage
}

type AssignmentInput struct {
	ContextNodeID   uuid.UUID
	ScopeMode       ScopeMode
	IsDefault       bool
	ValidFrom       *time.Time
	ValidUntil      *time.Time
	Source          string
	SourceReference *string
}

type GroupMappingDrift struct {
	GroupID           uuid.UUID
	GroupPath         string
	KeycloakGroupID   string
	GroupEnabled      bool
	ContextNodeID     *uuid.UUID
	ContextCode       string
	ContextName       string
	ContextEnabled    bool
	ScopeMode         ScopeMode
	Status            string
	RecommendedAction string
}

type GroupContextSyncMapping struct {
	GroupID       uuid.UUID
	ContextNodeID uuid.UUID
	ScopeMode     ScopeMode
}

type GroupContextSyncResult struct {
	AppliedMappings int
	AffectedGroups  int
}
