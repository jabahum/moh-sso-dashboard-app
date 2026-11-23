package routes

import (
	authfeature "github.com/moh-sso-dashboard/internal/features/auth"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"

	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	healthcontextfeature "github.com/moh-sso-dashboard/internal/features/health_context"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	"github.com/moh-sso-dashboard/module"
	adminunitsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units"
	auditfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/audit"
	dataqualityfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/data-quality"
	documenttemplatesfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates"
	documentsmodule "github.com/mohuganda/moh-sso-dashboard/backend/modules/documents"
	geojsonfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
	metricsfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
	storagelocationfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/storage-locations"
	surveillancefeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance"
	visualiserfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/visualiser"
)

type Dependencies struct {
	Modules *module.Registry
	Limiter *ratelimit.Limiter

	Auth              *authfeature.Handler
	Clients           *clientfeature.Handler
	Users             *userfeature.Handler
	Metrics           *metricsfeature.Module
	Audit             *auditfeature.Module
	Notifications     *notificationsfeature.Handler
	Documents         *documentsmodule.Module
	DocumentTemplates *documenttemplatesfeature.Module
	StorageLocations  *storagelocationfeature.Module
	Sessions          *sessionfeature.Handler
	DataQuality       *dataqualityfeature.Module

	AdminUnits     *adminunitsfeature.Module
	Visualiser     *visualiserfeature.Module
	Surveillance   *surveillancefeature.Module
	GeoJSON        *geojsonfeature.Module
	Email          *emailfeature.Handler
	RBAC           *rbacfeature.Handler
	HealthContexts *healthcontextfeature.Handler

	AuthenticatedRateLimitPerMin  int
	AuthLoginRateLimitPerMin      int
	AuthCallbackRateLimitPerMin   int
	AuthSessionRateLimitPerMinute int
}
