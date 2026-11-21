package apidocgen

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	router "github.com/moh-sso-dashboard/internal/api"
	scheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

func TestSchedulerContractMatchesMountedSurfaces(t *testing.T) {
	operations, err := moduleOperations(object{})
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	handler := router.NewReportSchedulerModule(scheduler.NewService(nil, nil, nil, nil, nil, nil))
	router.RegisterReportScheduler(engine.Group("/public"), engine.Group("/protected"), handler, nil)
	for _, route := range engine.Routes() {
		surface, path, _ := strings.Cut(strings.TrimPrefix(route.Path, "/"), "/")
		public := surface == "public"
		key := route.Method + " /api/v1/" + OpenAPIPath(path)
		op, found := operations[key]
		if !found {
			t.Fatalf("mounted scheduler operation missing from OpenAPI: %s", key)
		}
		security, anonymous := op["security"].([]any)
		if public != (anonymous && len(security) == 0) {
			t.Errorf("scheduler operation %s security differs from its mounted surface", key)
		}
		delete(operations, key)
	}
	for key := range operations {
		if strings.Contains(key, " /api/v1/report-scheduler") {
			t.Errorf("OpenAPI describes unmounted scheduler operation %s", key)
		}
	}
}
