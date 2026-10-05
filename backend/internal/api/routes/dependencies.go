package routes

import (
	adminunitsfeature "github.com/moh-sso-dashboard/internal/features/admin_units"
	announcementfeature "github.com/moh-sso-dashboard/internal/features/announcements"
	auditfeature "github.com/moh-sso-dashboard/internal/features/audit"
	authfeature "github.com/moh-sso-dashboard/internal/features/auth"
	clientfeature "github.com/moh-sso-dashboard/internal/features/clients"
	dataqualityfeature "github.com/moh-sso-dashboard/internal/features/data_quality"
	documenttemplatesfeature "github.com/moh-sso-dashboard/internal/features/document_templates"
	documentsfeature "github.com/moh-sso-dashboard/internal/features/documents"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	geojsonfeature "github.com/moh-sso-dashboard/internal/features/geojson"
	metricsfeature "github.com/moh-sso-dashboard/internal/features/metrics"
	notificationsfeature "github.com/moh-sso-dashboard/internal/features/notifications"
	rbacfeature "github.com/moh-sso-dashboard/internal/features/rbac"
 reportschedulerfeature "github.com/moh-sso-dashboard/internal/features/report_scheduler"
	sessionfeature "github.com/moh-sso-dashboard/internal/features/sessions"
	storagelocationfeature "github.com/moh-sso-dashboard/internal/features/storage_locations"
	surveillancefeature "github.com/moh-sso-dashboard/internal/features/surveillance"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	visualiserfeature "github.com/moh-sso-dashboard/internal/features/visualiser"
	"github.com/moh-sso-dashboard/internal/ratelimit"
)

type Dependencies struct {
	Limiter *ratelimit.Limiter

	Auth                    *authfeature.Handler
	Clients                 *clientfeature.Handler
	Users                   *userfeature.Handler
	Metrics                 *metricsfeature.Handler
	Audit                   *auditfeature.Handler
	Notifications           *notificationsfeature.Handler
	Documents               *documentsfeature.Handler
	DocumentTemplates       *documenttemplatesfeature.Handler
	DocumentTemplateSheets  *documenttemplatesfeature.SheetHandler
	DocumentTemplateColumns *documenttemplatesfeature.ColumnHandler
	StorageLocations        *storagelocationfeature.Handler
	Sessions                *sessionfeature.Handler
	DataQuality             *dataqualityfeature.Handler
	Announcements           *announcementfeature.Handler
	AdminUnits              *adminunitsfeature.Handler
	Visualiser              *visualiserfeature.Handler
	Surveillance            *surveillancefeature.Handler
	GeoJSON                 *geojsonfeature.Handler
	Email                   *emailfeature.Handler
	RBAC                    *rbacfeature.Handler
 ReportScheduler *reportschedulerfeature.Handler

	AuthenticatedRateLimitPerMin  int
	AuthLoginRateLimitPerMin      int
	AuthCallbackRateLimitPerMin   int
	AuthSessionRateLimitPerMinute int
}
