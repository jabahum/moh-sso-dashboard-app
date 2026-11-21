package rbac

import (
	"context"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// NewCoreApplicationRegistry adapts the authoritative registry without moving
// provisioning, manifest review, HTTP DTOs or transaction ownership into core.
func NewCoreApplicationRegistry(service *Service) core.ApplicationRegistry {
	if service == nil {
		return nil
	}
	return coreApplicationRegistry{service: service}
}

type coreApplicationRegistry struct{ service *Service }

var _ core.ApplicationRegistry = coreApplicationRegistry{}

func (r coreApplicationRegistry) GetApplication(ctx context.Context, clientID string) (core.Application, error) {
	detail, err := r.service.GetSystem(ctx, clientID)
	if err != nil {
		return core.Application{}, err
	}
	application := coreApplication(detail.System)
	application.Access.Roles = append([]string(nil), detail.AccessRoles...)
	if detail.AccessRoles != nil && application.Access.Roles == nil {
		application.Access.Roles = []string{}
	}
	return application, nil
}
func coreApplication(s System) core.Application {
	return core.Application{
		ID: s.ID, Description: s.Description, OwnerTeam: s.OwnerTeam, OwnerName: s.OwnerName,
		OwnerEmail: s.OwnerEmail, SupportURL: s.SupportURL, DocumentationURL: s.DocumentationURL,
		Environment: s.Environment, Criticality: s.Criticality, Enabled: s.Enabled, SortOrder: s.SortOrder,
		LifecycleStatus: s.LifecycleStatus, IntegrationType: s.IntegrationType, ContractVersion: s.ContractVersion,
		Access: core.SystemAccess{
			ClientID: s.ClientID, DisplayName: s.DisplayName, LaunchURL: s.LaunchURL,
			AuthenticatedLaunchURL: s.AuthenticatedLaunchURL, Icon: s.Icon, Category: s.Category,
			Navigation: s.Navigation, SystemType: s.SystemType, DisplayInLauncher: s.DisplayInLauncher,
			DisplayInSideNav: s.DisplayInSideNav, LaunchMode: s.LaunchMode,
		},
	}
}
