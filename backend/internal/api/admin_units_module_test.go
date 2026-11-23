package router

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/authz"
	"github.com/moh-sso-dashboard/internal/config"
	health "github.com/moh-sso-dashboard/internal/features/health_context"
	"github.com/moh-sso-dashboard/internal/middleware"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	"github.com/moh-sso-dashboard/module"
	adminunits "github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units"
)

const otherAdminUnitModules = "metrics,audit,geojson,report-scheduler,announcements," + extractedModules

func TestAdminUnitsInitializationRequiresEnabledDependencies(t *testing.T) {
	for _, missing := range []string{"handler", "disabled"} {
		handlers := HandlerSet{AdminUnits: testAdminUnitsModule(t, &adminUnitService{}, &health.Service{}), HealthContexts: &health.Handler{}}
		ids := otherAdminUnitModules
		if missing == "handler" {
			handlers.AdminUnits = nil
		}
		if missing == "disabled" {
			handlers = HandlerSet{}
			ids += ", admin-units "
		}
		registry, err := NewModuleRegistry(handlers, ids, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		stop, err := registry.Initialize(context.Background())
		if missing != "disabled" {
			if err == nil {
				t.Fatalf("missing %s accepted", missing)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		defer stop(context.Background())
	}
}

func TestAdminUnitsProductionRouterPreservesRoutesAndDisablement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var baseline map[string]bool
	var unrelated map[string]bool
	for _, mode := range []string{"legacy", "enabled", "disabled"} {
		handlers := HandlerSet{AdminUnits: testAdminUnitsModule(t, &adminUnitService{}, &health.Service{}), HealthContexts: &health.Handler{}}
		var registry *module.Registry
		if mode != "legacy" {
			ids := otherAdminUnitModules
			if mode == "disabled" {
				ids += ",admin-units"
				handlers.AdminUnits = nil
			}
			var err error
			registry, err = NewModuleRegistry(handlers, ids, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			stop, err := registry.Initialize(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer stop(context.Background())
			found := false
			for _, item := range registry.Inventory() {
				if item.ID == "admin-units" {
					found = true
					if item.Version != adminunits.Version() || item.Enabled != (mode == "enabled") {
						t.Fatalf("inventory: %+v", item)
					}
				}
			}
			if !found {
				t.Fatal("inventory missing admin-units")
			}
		}
		engine := SetupRouter(RouterDependencies{Config: &config.Config{}, Handlers: handlers, Modules: registry})
		actual, other := map[string]bool{}, map[string]bool{}
		for _, route := range engine.Routes() {
			key := route.Method + " " + route.Path
			if strings.Contains(route.Path, "/visualizer/adminunits/") {
				actual[key] = true
			} else if strings.HasPrefix(route.Path, "/api/v1/visualizer/") {
				other[key] = true
			}
		}
		if mode == "legacy" {
			baseline, unrelated = actual, other
			if len(baseline) != 9 {
				t.Fatalf("route count %d", len(baseline))
			}
			continue
		}
		if len(other) != len(unrelated) {
			t.Fatal("unrelated visualizer routes changed")
		}
		for key := range unrelated {
			if !other[key] {
				t.Fatalf("lost unrelated route %s", key)
			}
		}
		if mode == "enabled" {
			if len(actual) != len(baseline) {
				t.Fatal("registry route count changed")
			}
			for key := range baseline {
				if !actual[key] {
					t.Fatalf("lost %s", key)
				}
			}
			continue
		}
		if len(actual) != 0 {
			t.Fatalf("disabled routes leaked: %v", actual)
		}
		for key := range baseline {
			method, path, _ := strings.Cut(key, " ")
			res := httptest.NewRecorder()
			engine.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader("{}")))
			if res.Code != 404 {
				t.Fatalf("disabled %s: %d", key, res.Code)
			}
		}
	}
}

type adminUnitService struct {
	adminunits.Repository
	calls int
}

func ptr(s string) *string { return &s }
func (s *adminUnitService) ListOrgUnits(context.Context) ([]adminunits.OrgUnitFull, error) {
	s.calls++
	return []adminunits.OrgUnitFull{{DimOrgHierarchyKey: 1, OrgUnitID: ptr("allowed"), OrgUnitName: ptr("Allowed"), Level: "1"}, {DimOrgHierarchyKey: 2, OrgUnitID: ptr("outside"), OrgUnitName: ptr("Outside"), Level: "1"}}, nil
}
func (s *adminUnitService) ListFacilities(context.Context) ([]adminunits.Facility, error) {
	s.calls++
	return []adminunits.Facility{{DimOrgHierarchyKey: 1, OrgUnitID: ptr("allowed")}, {DimOrgHierarchyKey: 2, OrgUnitID: ptr("outside")}}, nil
}
func (s *adminUnitService) simple() ([]adminunits.OrgUnitSimple, error) {
	s.calls++
	return []adminunits.OrgUnitSimple{{DimOrgHierarchyKey: 1, OrgUnitID: ptr("allowed")}, {DimOrgHierarchyKey: 2, OrgUnitID: ptr("outside")}}, nil
}
func (s *adminUnitService) ListDistricts(context.Context) ([]adminunits.OrgUnitSimple, error) {
	return s.simple()
}
func (s *adminUnitService) ListRegions(context.Context) ([]adminunits.OrgUnitSimple, error) {
	return s.simple()
}
func (s *adminUnitService) ListNational(context.Context) ([]adminunits.OrgUnitSimple, error) {
	return s.simple()
}
func (s *adminUnitService) ListSubCounties(context.Context, *string) ([]adminunits.OrgUnitSimple, error) {
	return s.simple()
}
func (s *adminUnitService) ListLocalGovt(context.Context, *string) ([]adminunits.OrgUnitSimple, error) {
	return s.simple()
}
func (s *adminUnitService) ListDistrictsByRegion(context.Context, *string) ([]adminunits.OrgUnitSimple, error) {
	return s.simple()
}

type adminUnitHealthRepository struct {
	health.Repository
	node     health.Node
	assigned bool
	unmapped bool
	err      error
	calls    int
}

func (r *adminUnitHealthRepository) ListEffectiveContexts(context.Context, string) ([]health.EffectiveContext, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if !r.assigned {
		return nil, nil
	}
	return []health.EffectiveContext{{Node: r.node, ScopeMode: health.ScopeNodeOnly}}, nil
}
func (r *adminUnitHealthRepository) GetNode(_ context.Context, id uuid.UUID) (health.Node, error) {
	r.calls++
	if id != r.node.ID {
		return health.Node{}, errors.New("unknown context")
	}
	return r.node, r.err
}
func (r *adminUnitHealthRepository) ListAliasesInScope(context.Context, uuid.UUID, bool) ([]health.Alias, error) {
	r.calls++
	if r.unmapped {
		return nil, nil
	}
	return []health.Alias{{Namespace: "dwh-org-unit", ExternalID: "allowed"}}, r.err
}

func TestAdminUnitsModulePreservesAuthSystemContextAndReadPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, disabled := range []bool{false, true} {
		for _, identity := range []string{"anonymous", "no-system", "no-permission", "unassigned", "missing-context", "invalid-header", "denied-context", "scoped", "unmapped", "health-error", "documents-reader", "dqa-reader", "surveillance-reader"} {
			t.Run(identity+map[bool]string{true: "/disabled", false: "/enabled"}[disabled], func(t *testing.T) {
				service := &adminUnitService{}
				repo := &adminUnitHealthRepository{node: health.Node{ID: uuid.New(), Code: "D1", Enabled: true}, assigned: true}
				if identity == "unassigned" {
					repo.assigned = false
				}
				repo.unmapped = identity == "unmapped"
				if identity == "health-error" {
					repo.err = errors.New("health service unavailable")
				}
				contexts := health.NewService(repo)
				handlers := HandlerSet{AdminUnits: testAdminUnitsModule(t, service, contexts), HealthContexts: health.NewHandler(contexts, nil)}
				ids := otherAdminUnitModules
				if disabled {
					ids += ",admin-units"
				}
				registry, err := NewModuleRegistry(handlers, ids, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				stop, err := registry.Initialize(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer stop(context.Background())
				engine := gin.New()
				protected := engine.Group("/api/v1", func(c *gin.Context) {
					if identity == "anonymous" {
						return
					}
					actor := authz.Context{UserID: uuid.NewString(), AccessibleSystems: []string{authz.SystemDataStatistics}, Permissions: []authz.Permission{authz.PermissionReportBrowserRead}}
					if identity == "no-system" {
						actor.AccessibleSystems = nil
					}
					if identity == "no-permission" {
						actor.Permissions = nil
					}
					switch identity {
					case "documents-reader":
						actor.Permissions = []authz.Permission{authz.PermissionDocumentsRead}
					case "dqa-reader":
						actor.Permissions = []authz.Permission{authz.PermissionDataQualityRead}
					case "surveillance-reader":
						actor.Permissions = []authz.Permission{authz.PermissionSurveillanceRead}
					}
					c.Set("user_id", actor.UserID)
					c.Set(authz.ContextKey, actor)
				}, middleware.RequireAuth())
				registry.RegisterRoutes(module.Routes{Protected: protected})
				for _, endpoint := range []struct{ method, path string }{{"GET", "orgunits"}, {"GET", "facilities"}, {"GET", "district"}, {"POST", "subcounties"}, {"POST", "localgovt"}, {"POST", "districts"}, {"GET", "region"}, {"GET", "national"}, {"GET", "hierarchy"}} {
					req := httptest.NewRequest(endpoint.method, "/api/v1/visualizer/adminunits/"+endpoint.path, strings.NewReader("{}"))
					req.Header.Set("Content-Type", "application/json")
					if identity != "unassigned" && identity != "missing-context" && identity != "health-error" {
						req.Header.Set("X-Health-Context-ID", repo.node.ID.String())
					}
					if identity == "invalid-header" {
						req.Header.Set("X-Health-Context-ID", "invalid")
					}
					if identity == "denied-context" {
						req.Header.Set("X-Health-Context-ID", uuid.NewString())
					}
					res := httptest.NewRecorder()
					before := service.calls
					engine.ServeHTTP(res, req)
					want := 200
					switch identity {
					case "anonymous":
						want = 401
					case "no-system", "no-permission", "denied-context", "unmapped":
						want = 403
					case "missing-context", "invalid-header":
						want = 400
					case "health-error":
						want = 503
					}
					if disabled {
						want = 404
					}
					if res.Code != want {
						t.Fatalf("%s: got %d want %d: %s", endpoint.path, res.Code, want, res.Body.String())
					}
					if want == 200 {
						var body struct{ Data []any }
						if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
							t.Fatal(err)
						}
						count := 1
						if identity == "unassigned" {
							count = 2
						}
						if len(body.Data) != count {
							t.Fatalf("scope row count: %s", res.Body.String())
						}
						if !strings.Contains(res.Body.String(), "Allowed") && endpoint.path == "hierarchy" {
							t.Fatal("allowed hierarchy disappeared")
						}
						if identity != "unassigned" && (strings.Contains(res.Body.String(), "outside") || strings.Contains(res.Body.String(), "Outside")) {
							t.Fatal("scope leaked outside records")
						}
					} else if identity != "unmapped" && service.calls != before {
						t.Fatal("denial/disablement touched administrative units service")
					}
				}
				if disabled && (service.calls != 0 || repo.calls != 0) {
					t.Fatal("disabled module touched dependencies")
				}
			})
		}
	}
}

func testAdminUnitsModule(t *testing.T, repository adminunits.Repository, contexts *health.Service) *adminunits.Module {
	t.Helper()
	m, err := adminunits.NewModule(adminunits.Dependencies{
		Repository: repository, RequireSystem: modulehttp.RequireSystem, AuthorizeAny: modulehttp.AuthorizeAny,
		SelectContext: contexts.SelectContextHTTP, ContextNodeCode: contexts.NodeCode, ContextAliases: contexts.AliasIDs, RequestID: modulehttp.RequestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
