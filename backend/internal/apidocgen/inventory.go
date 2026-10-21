// Package apidocgen builds the API contract from the real Gin router and Go handler types.
package apidocgen

import (
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/api/handler"
)

// Routes uses inert handlers: route registration never needs live DB/Redis/SSO.
// Initialize every handler so nil-guarded optional routes are included too.
func Routes() gin.RoutesInfo {
	gin.SetMode(gin.TestMode)
	old := gin.DefaultWriter
	gin.DefaultWriter = io.Discard
	defer func() { gin.DefaultWriter = old }()
	var handlers router.HandlerSet
	v := reflect.ValueOf(&handlers).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.Pointer {
			field.Set(reflect.New(field.Type().Elem()))
		}
	}
	r := router.SetupRouter(router.RouterDependencies{Handlers: handlers})
	router.RegisterHealthRoutes(r, &handler.HealthHandler{})
	var result gin.RoutesInfo
	for _, route := range r.Routes() {
		if strings.HasPrefix(route.Path, "/swagger") {
			continue
		}
		result = append(result, route)
	}
	return result
}

var pathParameter = regexp.MustCompile(`[:*]([A-Za-z_][A-Za-z0-9_]*)`)

func OpenAPIPath(path string) string {
	return pathParameter.ReplaceAllString(path, "{$1}")
}

func publicRoute(path string) bool {
	if !strings.HasPrefix(path, "/api/") {
		return true
	}
	switch path {
	case "/api/v1/auth/login", "/api/v1/auth/launch", "/api/v1/auth/callback", "/api/v1/auth/logout",
		"/api/v1/announcements/public", "/api/v1/announcements/active":
		return true
	}
	return false
}
