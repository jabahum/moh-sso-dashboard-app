package metrics

import (
	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *Handler, authorize gin.HandlerFunc) {
	if authorize == nil {
		panic("metrics authorization middleware is required")
	}
	metrics := admin.Group("/metrics")
	metrics.Use(authorize)
	{
		metrics.GET("/overview", handler.Overview)
		metrics.GET("/system/count-users", handler.CountUsers)
		metrics.GET("/system/count-disabled-users", handler.CountDisabledUsers)
		metrics.GET("/system/active-today", handler.ActiveUsersToday)
		metrics.GET("/system/active-this-week", handler.ActiveUsersThisWeek)
		metrics.GET("/system/login-trend", handler.LoginTrend)
		metrics.GET("/system/login-trend-range", handler.LoginTrendByDay)
		metrics.GET("/security/failed-logins", handler.CountFailedLogins)
		metrics.GET("/security/failed-logins-range", handler.CountFailedLoginsInRange)
		metrics.GET("/security/suspicious-logins", handler.SuspiciousLogins)
		metrics.GET("/clients/count", handler.CountClients)
		metrics.GET("/clients/most-accessed", handler.MostAccessedClients)
		metrics.GET("/clients/login-count", handler.LoginCountForClient)
		metrics.GET("/clients/active-today", handler.ActiveUsersPerClientToday)
		metrics.GET("/users/new-range", handler.NewUsersInRange)
		metrics.GET("/users/new-trend", handler.NewUsersTrend)
		metrics.GET("/users/never-logged-in", handler.NeverLoggedInUsers)
		metrics.GET("/users/last-login/:userID", handler.LastLoginForUser)
		metrics.GET("/users/client-usage/:userID", handler.UserClientUsage)
	}
}
