package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/internal/ratelimit"
	scheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

// NewReportSchedulerModule supplies the dashboard's request context to the
// standalone scheduler. The module owns route dispatch and authorization.
func NewReportSchedulerModule(service *scheduler.Service) *scheduler.HTTPModule {
	if service == nil {
		return nil
	}
	module, err := scheduler.NewHTTPModule(service, scheduler.HTTPOptions{
		Actor: func(r *http.Request) (scheduler.HTTPActor, bool) {
			actor, ok := modulehttp.RequestAccess(r)
			return scheduler.HTTPActor{
				UserID:        actor.Subject,
				HealthContext: scheduler.HealthContext{District: actor.HealthScope.District, Facility: actor.HealthScope.Facility},
				HasSystem:     actor.HasSystem,
				HasPermission: actor.HasPermission,
			}, ok
		},
		RequestID: modulehttp.RequestID,
		// The public Gin group handles rate limiting using the trusted proxy
		// configuration before dispatching the module handler.
		LimitPublicDownload: func(_ string, _ int, next http.Handler) http.Handler { return next },
	})
	if err != nil {
		panic(err)
	}
	return module
}

func RegisterReportScheduler(public, protected *gin.RouterGroup, schedulerModule *scheduler.HTTPModule, limiter *ratelimit.Limiter) {
	if schedulerModule == nil {
		return
	}
	if public != nil {
		public = public.Group("", ratelimit.MiddlewareForPolicy(limiter,
			ratelimit.PerIPPolicy(scheduler.DeliveryDownloadPolicy, scheduler.DeliveryDownloadLimit)))
	}
	modulehttp.RegisterSurfaces(public, protected, schedulerModule)
}
