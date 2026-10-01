package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/config"
)

func TestCORSUsesFrontendOriginRatherThanPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, environment := range []string{"development", "staging", "production"} {
		t.Run(environment, func(t *testing.T) {
			cfg := &config.Config{
				Environment:         environment,
				FrontendBaseURL:     "https://portal.example.test/portal",
				FrontendRedirectURI: "https://portal.example.test/portal/home?tab=mine",
			}
			router := gin.New()
			router.Use(cors.New(corsConfig(cfg)))
			router.POST("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
			for _, origin := range []string{"https://portal.example.test", "https://untrusted.example.test", "http://localhost:5173"} {
				req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
				req.Header.Set("Origin", origin)
				req.Header.Set("Access-Control-Request-Method", "POST")
				res := httptest.NewRecorder()
				router.ServeHTTP(res, req)
				allowed := origin == "https://portal.example.test" || (environment == "development" && origin == "http://localhost:5173")
				if allowed {
					if res.Code != http.StatusNoContent || res.Header().Get("Access-Control-Allow-Origin") != origin || res.Header().Get("Access-Control-Allow-Credentials") != "true" {
						t.Errorf("expected credentialed preflight for %s, got %d %v", origin, res.Code, res.Header())
					}
				} else if res.Code != http.StatusForbidden {
					t.Errorf("expected forbidden origin %s, got %d", origin, res.Code)
				}
			}
		})
	}
}

func TestAllowedOriginsRejectUnsafeURLsAndDeduplicate(t *testing.T) {
	for _, raw := range []string{"*", "https://*.example.test", "/portal", "javascript:alert(1)", "https://user:pass@example.test"} {
		origins := buildAllowedOrigins(&config.Config{Environment: "production", FrontendBaseURL: raw})
		if len(origins) != 1 || origins[0] != "https://dashboards.health.go.ug" {
			t.Errorf("unsafe configured origin %q accepted: %v", raw, origins)
		}
	}
	origins := buildAllowedOrigins(&config.Config{
		Environment:         "production",
		FrontendBaseURL:     "https://portal.example.test/portal",
		FrontendRedirectURI: "https://portal.example.test/callback",
	})
	if len(origins) != 2 {
		t.Fatalf("expected one configured origin plus production default, got %v", origins)
	}
}
