package routes

import (
	authfeature "github.com/moh-sso-dashboard/internal/features/auth"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	dataqualityfeature "github.com/moh-sso-dashboard/internal/features/data_quality"
	documenttemplatesfeature "github.com/moh-sso-dashboard/internal/features/document_templates"
	documentsfeature "github.com/moh-sso-dashboard/internal/features/documents"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	storagelocationfeature "github.com/moh-sso-dashboard/internal/features/storage_locations"
	surveillancefeature "github.com/moh-sso-dashboard/internal/features/surveillance"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	visualiserfeature "github.com/moh-sso-dashboard/internal/features/visualiser"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/moh-sso-dashboard/module"
	adminunitsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units"
	auditfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/audit"
	geojsonfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
	metricsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
)

type Dependencies struct {
	Modules *module.Registry
	Limiter *ratelimit.Limiter

	Auth                    *authfeature.Handler
	Clients                 *clientfeature.Handler
	Users                   *userfeature.Handler
	Metrics                 *metricsfeature.Module
	Audit                   *auditfeature.Module
	Notifications           *notificationsfeature.Handler
	Documents               *documentsfeature.Handler
	DocumentTemplates       *documenttemplatesfeature.Handler
	DocumentTemplateSheets  *documenttemplatesfeature.SheetHandler
	DocumentTemplateColumns *documenttemplatesfeature.ColumnHandler
	StorageLocations        *storagelocationfeature.Handler
	Sessions                *sessionfeature.Handler
	DataQuality             *dataqualityfeature.Handler

	AdminUnits     *adminunitsfeature.Module
	Visualiser     *visualiserfeature.Handler
	Surveillance   *surveillancefeature.Handler
	GeoJSON        *geojsonfeature.Module
	Email          *emailfeature.Handler
	RBAC           *rbacfeature.Handler
	HealthContexts *healthcontextfeature.Handler

	AuthenticatedRateLimitPerMin  int
	AuthLoginRateLimitPerMin      int
	AuthCallbackRateLimitPerMin   int
	AuthSessionRateLimitPerMinute int
}
