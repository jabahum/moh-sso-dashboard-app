package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type recorderFunc func(context.Context, AuditEvent) error

func (f recorderFunc) RecordAudit(ctx context.Context, event AuditEvent) error { return f(ctx, event) }

func fullPlatform() Platform {
	return Platform{
		Access:        func(*http.Request) (RequestAccess, bool) { return RequestAccess{}, true },
		AuthorizeAny:  func(*http.Request, ...string) error { return nil },
		RequireAdmin:  func(*http.Request) error { return nil },
		RequireSystem: func(*http.Request, string) error { return nil },
		Limit:         func(*http.Request, string, int, time.Duration) (http.Header, error) { return nil, nil },
		RequestID:     func(*http.Request) string { return "request" },
		ReportError:   func(*http.Request, error) {},
		Audit:         recorderFunc(func(context.Context, AuditEvent) error { return nil }),
	}
}

func allCapabilities() []Capability {
	return []Capability{CapabilityAccess, CapabilityAuthorizeAny, CapabilityRequireAdmin, CapabilityRequireSystem,
		CapabilityLimit, CapabilityRequestID, CapabilityReportError, CapabilityAudit}
}

func TestPlatformRequireAcceptsSuppliedCapabilities(t *testing.T) {
	if err := fullPlatform().Require(allCapabilities()...); err != nil {
		t.Fatal(err)
	}
	if err := (Platform{}).Require(); err != nil {
		t.Fatalf("requiring nothing must succeed: %v", err)
	}
}

func TestPlatformRequireNamesEveryMissingCapability(t *testing.T) {
	p := fullPlatform()
	p.Limit, p.Audit = nil, nil
	err := p.Require(CapabilityAuthorizeAny, CapabilityLimit, CapabilityAudit)
	if !errors.Is(err, ErrMissingCapability) {
		t.Fatalf("expected ErrMissingCapability, got %v", err)
	}
	if !strings.Contains(err.Error(), "limit") || !strings.Contains(err.Error(), "audit") || strings.Contains(err.Error(), "authorize-any") {
		t.Fatalf("error must name exactly the missing capabilities: %v", err)
	}
}

func TestPlatformRequireRejectsUnknownCapability(t *testing.T) {
	if err := fullPlatform().Require("telepathy"); !errors.Is(err, ErrMissingCapability) {
		t.Fatalf("unknown capabilities must fail closed, got %v", err)
	}
}

func TestEmptyPlatformLacksEveryCapability(t *testing.T) {
	for _, capability := range allCapabilities() {
		if err := (Platform{}).Require(capability); !errors.Is(err, ErrMissingCapability) {
			t.Fatalf("%s reported present on an empty platform", capability)
		}
	}
}
