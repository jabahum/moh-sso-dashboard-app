package rbac

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
)

type lifecycleTestRepository struct {
	Repository
	system      System
	accessRoles int
	events      []AuditEvent
	auditErr    error
}

func (r *lifecycleTestRepository) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	if r.auditErr != nil {
		return r.auditErr
	}
	r.events = append(r.events, event)
	return nil
}
func (r *lifecycleTestRepository) TransitionSystem(ctx context.Context, clientID string, input LifecycleTransitionInput, actor string, validate func(System, int) error) (System, error) {
	if err := validate(r.system, r.accessRoles); err != nil {
		_ = r.RecordAuditEvent(ctx, lifecycleAudit(clientID, actor, "system.lifecycle.denied", r.system.LifecycleStatus, input, err.Error()))
		return System{}, err
	}
	r.system.LifecycleStatus = input.Status
	r.system.Enabled = input.Status == "active"
	_ = r.RecordAuditEvent(ctx, lifecycleAudit(clientID, actor, "system.lifecycle.changed", "", input, "accepted"))
	return r.system, nil
}
func validApplication() System {
	return System{ClientID: "pilot", DisplayName: "Pilot", OwnerTeam: "Integration team", OwnerName: "Owner", OwnerEmail: "owner@example.org", SupportURL: "https://example.org/support", Environment: "test", SystemType: "external", IntegrationType: "external", ContractVersion: "1", LaunchMode: "new_tab", LaunchURL: "https://example.org", LifecycleStatus: "registered"}
}
func lifecycleActor() authz.Context {
	return authz.Context{UserID: "reviewer", Permissions: []authz.Permission{authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite}}
}

func TestApplicationLifecycleRequiresSequentialOnboarding(t *testing.T) {
	repo := &lifecycleTestRepository{system: validApplication(), accessRoles: 1}
	service := NewService(repo)
	transition := func(status string) error {
		_, err := service.TransitionSystem(context.Background(), "pilot", LifecycleTransitionInput{Status: status, Reason: "Reviewed"}, lifecycleActor())
		return err
	}
	if err := transition("active"); !errors.Is(err, ErrLifecycleConflict) {
		t.Fatalf("activation skipped onboarding: %v", err)
	}
	for _, status := range []string{"configured", "validated", "approved", "active", "suspended"} {
		if err := transition(status); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if repo.system.Enabled != (status == "active") {
			t.Fatalf("unexpected enabled flag for %s", status)
		}
	}
	if err := transition("active"); !errors.Is(err, ErrLifecycleConflict) {
		t.Fatalf("suspension bypassed approval: %v", err)
	}
	for _, status := range []string{"configured", "validated", "approved", "active", "retired"} {
		if err := transition(status); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
	}
	for status := range lifecycleTransitions {
		if err := transition(status); !errors.Is(err, ErrLifecycleConflict) {
			t.Fatalf("retirement must be terminal: %s: %v", status, err)
		}
	}
}

func TestLifecycleValidationRejectsIncompleteApplications(t *testing.T) {
	cases := []struct {
		name   string
		change func(*System)
		roles  int
	}{
		{"owner", func(s *System) { s.OwnerTeam = "" }, 1},
		{"email", func(s *System) { s.OwnerEmail = "invalid" }, 1},
		{"support", func(s *System) { s.SupportURL = "" }, 1},
		{"environment", func(s *System) { s.Environment = "unknown" }, 1},
		{"contract", func(s *System) { s.ContractVersion = "99" }, 1},
		{"integration", func(s *System) { s.IntegrationType = "hosted_module" }, 1},
		{"launch", func(s *System) { s.LaunchURL = "javascript:alert(1)" }, 1},
		{"empty launch", func(s *System) { s.LaunchURL = "" }, 1},
		{"access role", func(s *System) {}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			system := validApplication()
			tc.change(&system)
			if err := validateApplication(system, tc.roles); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected validation rejection, got %v", err)
			}
		})
	}
	if err := validateApplication(validApplication(), 1); err != nil {
		t.Fatal(err)
	}
	hosted := validApplication()
	hosted.SystemType = "platform"
	hosted.IntegrationType = "hosted_module"
	hosted.LaunchMode = "internal"
	hosted.LaunchURL = "/apps/pilot"
	if err := validateApplication(hosted, 1); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleHandlerPermissionsAndAudit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		permissions []authz.Permission
		status      string
		want        int
	}{
		{"reader", []authz.Permission{authz.PermissionRBACRead}, "configured", http.StatusForbidden},
		{"configuration writer", []authz.Permission{authz.PermissionRBACWrite}, "approved", http.StatusForbidden},
		{"role manager", []authz.Permission{authz.PermissionRBACRolesWrite}, "approved", http.StatusForbidden},
		{"reviewer", lifecycleActor().Permissions, "approved", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			system := validApplication()
			system.LifecycleStatus = "validated"
			repo := &lifecycleTestRepository{system: system, accessRoles: 1}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(authz.ContextKey, authz.Context{UserID: "actor", Permissions: tc.permissions})
			})
			RegisterAdminRoutes(router.Group("/api/v1/admin"), NewHandler(NewService(repo)), nil)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rbac/systems/pilot/lifecycle", strings.NewReader(`{"status":"`+tc.status+`","reason":"Review complete"}`))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("got %d: %s", res.Code, res.Body.String())
			}
			if len(repo.events) != 1 || repo.events[0].ActorUserID != "actor" {
				t.Fatalf("missing attributable audit: %+v", repo.events)
			}
			if tc.want == http.StatusForbidden && repo.system.LifecycleStatus != "validated" {
				t.Fatal("denied transition changed state")
			}
		})
	}
}

func TestLifecycleRequiresReasonAndFailsWhenDenialCannotBeAudited(t *testing.T) {
	repo := &lifecycleTestRepository{system: validApplication()}
	service := NewService(repo)
	if _, err := service.TransitionSystem(context.Background(), "pilot", LifecycleTransitionInput{Status: "configured"}, lifecycleActor()); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	repo.auditErr = errors.New("audit unavailable")
	_, err := service.TransitionSystem(context.Background(), "pilot", LifecycleTransitionInput{Status: "configured", Reason: "Reviewed"}, authz.Context{})
	if !errors.Is(err, repo.auditErr) {
		t.Fatalf("audit failure was ignored: %v", err)
	}
	if repo.system.LifecycleStatus != "registered" {
		t.Fatal("state changed")
	}
}

type registrationTestRepository struct {
	Repository
	input UpsertSystemInput
}

func (r *registrationTestRepository) GetSystem(context.Context, string) (SystemDetail, error) {
	return SystemDetail{}, sql.ErrNoRows
}
func (r *registrationTestRepository) UpsertSystem(_ context.Context, input UpsertSystemInput) (System, error) {
	r.input = input
	return System{ClientID: input.ClientID, LifecycleStatus: "registered", IntegrationType: input.IntegrationType, ContractVersion: input.ContractVersion}, nil
}
func TestRegistrationDoesNotAcceptCallerLifecycleOrAuditActor(t *testing.T) {
	repo := &registrationTestRepository{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(authz.ContextKey, lifecycleActor())
		c.Set("user_id", "actual-registrar")
	})
	RegisterAdminRoutes(router.Group("/api/v1/admin"), NewHandler(NewService(repo)), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rbac/systems", strings.NewReader(`{"clientId":"pilot","displayName":"Pilot","systemType":"platform","launchUrl":"/apps/pilot","lifecycleStatus":"active","actorUserId":"impersonated","registerOnly":false}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("%d: %s", res.Code, res.Body.String())
	}
	if repo.input.ActorUserID != "actual-registrar" || !repo.input.RegisterOnly {
		t.Fatalf("caller controlled governance fields: %+v", repo.input)
	}
	if repo.input.IntegrationType != "hosted_module" || repo.input.ContractVersion != "1" {
		t.Fatalf("incorrect default contract: %+v", repo.input)
	}
	if !strings.Contains(res.Body.String(), `"lifecycleStatus":"registered"`) {
		t.Fatal(res.Body.String())
	}
}
