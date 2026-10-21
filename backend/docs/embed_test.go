package docs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEmbeddedDocumentation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router)
	for _, path := range []string{"/swagger/", "/swagger/index.html", "/swagger/swagger-ui-bundle.js", "/swagger/swagger-ui.css", "/swagger/swagger-initializer.js", "/swagger/openapi.yaml", "/swagger/openapi.json"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
			if recorder.Code != 200 {
				t.Fatalf("HTTP %d", recorder.Code)
			}
			if recorder.Body.Len() == 0 {
				t.Fatal("embedded asset is empty")
			}
			if path == "/swagger/openapi.json" {
				var spec map[string]any
				if err := json.Unmarshal(recorder.Body.Bytes(), &spec); err != nil {
					t.Fatal(err)
				}
				if spec["openapi"] != "3.0.3" {
					t.Fatal("not an OpenAPI document")
				}
			}
			if path == "/swagger/" && strings.Contains(recorder.Body.String(), "https://") {
				t.Fatal("UI must serve self-hosted assets")
			}
		})
	}
	redirect := httptest.NewRecorder()
	router.ServeHTTP(redirect, httptest.NewRequest("GET", "/swagger", nil))
	if redirect.Code != 307 || redirect.Header().Get("Location") != "swagger/" {
		t.Fatal("redirect must remain relative to reverse proxy prefix")
	}
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest("GET", "/swagger/../../openapi.overrides.yaml", nil))
	if missing.Code != 404 {
		t.Fatal("non-public embedded paths must not be served")
	}
}

func TestDocumentationBehindProxyPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	backend := gin.New()
	Register(backend)
	proxy := http.StripPrefix("/backend", backend)
	base, _ := url.Parse("http://example.test/backend/swagger")
	redirect := httptest.NewRecorder()
	proxy.ServeHTTP(redirect, httptest.NewRequest("GET", base.String(), nil))
	location, err := url.Parse(redirect.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if got := base.ResolveReference(location).Path; got != "/backend/swagger/" {
		t.Fatalf("proxy prefix lost: %s", got)
	}
	specResponse := httptest.NewRecorder()
	proxy.ServeHTTP(specResponse, httptest.NewRequest("GET", "http://example.test/backend/swagger/openapi.json", nil))
	if specResponse.Code != 200 {
		t.Fatal(specResponse.Code)
	}
	var spec map[string]any
	if err := json.Unmarshal(specResponse.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	server := spec["servers"].([]any)[0].(map[string]any)["url"].(string)
	specURL, _ := url.Parse("http://example.test/backend/swagger/openapi.json")
	relative, _ := url.Parse(server)
	if got := specURL.ResolveReference(relative).Path; got != "/backend/" {
		t.Fatalf("API calls would lose proxy prefix: %s", got)
	}
}
