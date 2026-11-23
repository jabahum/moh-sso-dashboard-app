package bootstrap

import (
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	visualisermodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/visualiser"
)

// buildVisualiserModule queries the data warehouse and constrains organisation
// units through the health-context directory.
func buildVisualiserModule(deps handlerDependencies) *visualisermodule.Module {
	module, err := visualisermodule.NewModule(visualisermodule.Dependencies{
		Database:       deps.Databases.DWH,
		HealthContexts: healthcontextfeature.ModuleDirectory{Service: deps.Services.HealthContexts},
	})
	if err != nil {
		panic(err)
	}
	return module
}
