package bootstrap

import (
	kitadapter "github.com/moh-sso-dashboard/internal/adapters/modulekit"
	"github.com/moh-sso-dashboard/internal/service"
	documenttemplates "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates"
)

// buildDocumentTemplatesModule supplies the host capabilities the Document
// Templates module consumes. It is built whenever the full profile runs,
// because the document import reads template structures through its Service
// even when the module's routes are disabled.
func buildDocumentTemplatesModule(deps serviceDependencies, notifications service.NotificationsService) *documenttemplates.Module {
	module, err := documenttemplates.NewModule(documenttemplates.Dependencies{
		Database:  deps.Databases.Primary,
		Warehouse: deps.Databases.Remote,
		Notifier:  kitadapter.Notifier{Service: notifications},
		Platform:  kitadapter.Platform(deps.Config),
	})
	if err != nil {
		panic(err)
	}
	return module
}
