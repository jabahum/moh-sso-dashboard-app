// Package apidocgen builds the API contract from the real Gin router and Go handler types.
package apidocgen

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	router "github.com/moh-sso-dashboard/internal/api"
	"github.com/moh-sso-dashboard/internal/api/handler"
	adminunits "github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units"
	announcements "github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements"
	audit "github.com/mohuganda/moh-sso-dashboard/backend/modules/audit"
	geojson "github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson"
	metrics "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
	reportScheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

// Routes uses inert handlers: route registration never needs live DB/Redis/SSO.
// Initialize every handler so nil-guarded optional routes are included too.
func Routes() gin.RoutesInfo {
	gin.SetMode(gin.TestMode)
	old := gin.DefaultWriter
	gin.DefaultWriter = io.Discard
	defer func() { gin.DefaultWriter = old }()
	handlers := documentationHandlers()

	modules, err := router.NewModuleRegistry(handlers, "", nil, nil)
	if err != nil {
		panic(err)
	}
	r := router.SetupRouter(router.RouterDependencies{Handlers: handlers, Modules: modules})
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

func documentationHandlers() router.HandlerSet {
	gin.SetMode(gin.TestMode)
	var handlers router.HandlerSet
	v := reflect.ValueOf(&handlers).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.Pointer {
			field.Set(reflect.New(field.Type().Elem()))
		}
	}
	var err error
	handlers.ReportScheduler = router.NewReportSchedulerModule(reportScheduler.NewService(nil, nil, nil, nil, nil, nil))
	handlers.GeoJSON, err = geojson.NewModule(geojson.Dependencies{AuthorizeAny: func(*http.Request, ...string) error { return geojson.ErrUnauthorized }})
	if err != nil {
		panic(err)
	}
	handlers.AdminUnits, err = adminunits.NewModule(adminunits.Dependencies{
		Database:        new(sql.DB),
		RequireSystem:   func(*http.Request, string) error { return adminunits.ErrUnauthorized },
		AuthorizeAny:    func(*http.Request, ...string) error { return adminunits.ErrUnauthorized },
		SelectContext:   func(*http.Request) (string, bool, error) { return "", false, adminunits.ErrUnauthorized },
		ContextNodeCode: func(context.Context, string) (string, error) { return "", adminunits.ErrUnauthorized },
		ContextAliases: func(context.Context, string, bool) (map[string][]string, error) {
			return nil, adminunits.ErrUnauthorized
		},
	})
	if err != nil {
		panic(err)
	}
	handlers.Metrics, err = metrics.NewModule(metrics.Dependencies{
		Database: new(sql.DB), RequireAdmin: func(*http.Request) error { return metrics.ErrUnauthorized },
		AuthorizeAny: func(*http.Request, ...string) error { return metrics.ErrUnauthorized },
	})
	if err != nil {
		panic(err)
	}
	handlers.Audit, err = audit.NewModule(audit.Dependencies{
		Database: new(sql.DB), RequireAdmin: func(*http.Request) error { return audit.ErrUnauthorized },
		AuthorizeAny: func(*http.Request, ...string) error { return audit.ErrUnauthorized },
		LimitExport: func(*http.Request, string, int, time.Duration) (http.Header, error) {
			return nil, audit.ErrUnauthorized
		},
	})
	if err != nil {
		panic(err)
	}
	handlers.Announcements, err = announcements.NewModule(announcements.Dependencies{
		Database: new(sql.DB), Actor: func(*http.Request) (announcements.Actor, error) {
			return announcements.Actor{}, announcements.ErrUnauthorized
		},
		RequireAdmin: func(*http.Request) error { return announcements.ErrUnauthorized },
		AuthorizeAny: func(*http.Request, ...string) error { return announcements.ErrUnauthorized },
		Limit: func(*http.Request, string, int, time.Duration) (http.Header, error) {
			return nil, announcements.ErrUnauthorized
		},
	})
	if err != nil {
		panic(err)
	}
	return handlers
}
func moduleOperations(definitions object) (map[string]object, error) {
	contracts := map[string]object{}
	handlers := documentationHandlers()
	v := reflect.ValueOf(handlers)
	for i := 0; i < v.NumField(); i++ {
		described, ok := v.Field(i).Interface().(interface{ OpenAPI() []byte })
		if !ok {
			continue
		}
		var document struct {
			Paths      map[string]map[string]object
			Components struct{ Schemas map[string]any }
		}
		if err := json.Unmarshal(described.OpenAPI(), &document); err != nil {
			return nil, err
		}
		for name, schema := range document.Components.Schemas {
			if previous, exists := definitions[name]; exists && !reflect.DeepEqual(previous, schema) {
				return nil, fmt.Errorf("conflicting module schema %s", name)
			}
			definitions[name] = schema
		}
		for path, methods := range document.Paths {
			for method, operation := range methods {
				key := strings.ToUpper(method) + " " + "/api/v1" + path
				if _, exists := contracts[key]; exists {
					return nil, fmt.Errorf("duplicate module operation %s", key)
				}
				if tags, ok := operation["tags"].([]any); ok {
					names := []string{}
					for _, tag := range tags {
						name, ok := tag.(string)
						if !ok {
							return nil, fmt.Errorf("invalid module tag")
						}
						names = append(names, name)
					}
					operation["tags"] = names
				}
				contracts[key] = operation
			}
		}
	}
	return contracts, nil
}
