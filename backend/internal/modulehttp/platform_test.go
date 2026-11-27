package modulehttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

type auditSink struct{ events []core.AuditEvent }

func (s *auditSink) RecordAudit(_ context.Context, event core.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

var everyCapability = []core.Capability{core.CapabilityAccess, core.CapabilityAuthorizeAny, core.CapabilityRequireAdmin,
	core.CapabilityRequireSystem, core.CapabilityLimit, core.CapabilityRequestID, core.CapabilityReportError, core.CapabilityAudit}

func TestNewPlatformSuppliesEveryCapability(t *testing.T) {
	platform := NewPlatform(&limitStore{ready: true, allowed: true}, &auditSink{})
	if err := platform.Require(everyCapability...); err != nil {
		t.Fatal(err)
	}
	if err := NewPlatform(nil, nil).Require(core.CapabilityAudit); !errors.Is(err, core.ErrMissingCapability) {
		t.Fatalf("a host without an audit recorder must not claim the audit capability: %v", err)
	}
}

func TestPlatformCapabilitiesReadOnlyVerifiedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	platform := NewPlatform(&limitStore{ready: true, allowed: true}, nil)
	for _, verified := range []bool{true, false} {
		engine := gin.New()
		group := engine.Group("/api/v1", func(c *gin.Context) {
			if verified {
				c.Set(authz.ContextKey, authz.Context{UserID: "verified-subject", IsAdmin: true, AccessibleSystems: []string{"clims"}})
			}
		})
		NewRegistrar(group).Handle("GET /probe", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			access, ok := platform.Access(r)
			adminErr := platform.RequireAdmin(r)
			systemErr := platform.RequireSystem(r, "clims")
			_, limitErr := platform.Limit(r, "probe", 5, time.Minute)
			if verified && (!ok || access.Subject != "verified-subject" || adminErr != nil || systemErr != nil || limitErr != nil) {
				t.Errorf("verified caller refused: ok=%v access=%+v admin=%v system=%v limit=%v", ok, access, adminErr, systemErr, limitErr)
			}
			if !verified && (ok || adminErr == nil || systemErr == nil || limitErr == nil) {
				t.Errorf("unauthenticated caller granted: ok=%v admin=%v system=%v limit=%v", ok, adminErr, systemErr, limitErr)
			}
		}))
		request := httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
		request.Header.Set("X-User-ID", "forged-header-identity")
		engine.ServeHTTP(httptest.NewRecorder(), request)
	}
}
