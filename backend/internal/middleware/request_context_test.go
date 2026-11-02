package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/observability"
)

func TestRequestContextPropagatesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestContext())
	router.GET("/ping", func(c *gin.Context) {
		response.OK(c, http.StatusOK, gin.H{"ok": true})
	})

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set(observability.RequestIDHeader, "req-123")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get(observability.RequestIDHeader); got != "req-123" {
		t.Fatalf("expected response request id header req-123, got %q", got)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"requestId":"req-123"`) {
		t.Fatalf("expected response meta request id, got %s", body)
	}
}

func TestRequestContextBoundsUntrustedTraceHeaders(t *testing.T) {
	router := gin.New()
	router.Use(RequestContext())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set(observability.RequestIDHeader, strings.Repeat("x", 129))
	request.Header.Set(observability.CorrelationIDHeader, "bad id\nwith newline")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	id := recorder.Header().Get(observability.RequestIDHeader)
	if id == "" || len(id) > 128 || strings.ContainsAny(id, " \r\n") {
		t.Fatal("untrusted request ID was reflected")
	}
	if recorder.Header().Get(observability.CorrelationIDHeader) != id {
		t.Fatal("invalid correlation ID did not fall back to request ID")
	}
}
