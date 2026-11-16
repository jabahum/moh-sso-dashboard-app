package rbac

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	"github.com/moh-sso-dashboard/module"
)

func manifestTestRepository(t *testing.T) (*postgresRepository, *sql.DB) {
	t.Helper()
	db := registryTestDatabase(t)
	for _, name := range []string{"000026_create_system_rbac_schema.up.sql", "000027_extend_rbac_governance.up.sql", "000030_add_realm_role_system_roles.up.sql", "000033_extend_system_launch_behavior.up.sql", "000048_application_registry_lifecycle.up.sql", "000059_grandfather_existing_client_lifecycle.up.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	return &postgresRepository{db: db}, db
}

func TestApplicationManifestPostgres(t *testing.T) {
	repo, db := manifestTestRepository(t)
	ctx := context.Background()
	manifest, input, err := parseApplicationManifest(manifestFixture(t, "external"), "development", nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.ReviewManifest(ctx, manifest, input, "", false, "")
	if err != nil || preview.Operation != "create" || len(preview.RolesToCreate) != 1 {
		t.Fatalf("invalid preview: %+v %v", preview, err)
	}
	if _, err := repo.GetSystem(ctx, manifest.Metadata.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("preview wrote a system: %v", err)
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, "", true, "admin"); !errors.Is(err, ErrManifestReview) {
		t.Fatalf("unreviewed import accepted: %v", err)
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin"); err != nil {
		t.Fatal(err)
	}
	current, err := repo.GetSystem(ctx, manifest.Metadata.ID)
	if err != nil || current.Enabled || current.LifecycleStatus != "registered" || len(current.AccessRoles) != 1 || len(current.Roles) != 1 {
		t.Fatalf("invalid registration: %+v %v", current, err)
	}
	repeated, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin")
	if err != nil || repeated.Operation != "noop" {
		t.Fatalf("retry was not idempotent: %+v %v", repeated, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM ihp_rbac_audit_events WHERE action='application.manifest.imported'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate import audit: %d %v", count, err)
	}
	service := NewService(repo)
	actor := authz.Context{UserID: "admin", Permissions: []authz.Permission{authz.PermissionRBACWrite, authz.PermissionRBACRolesWrite}}
	for _, status := range []string{"configured", "validated", "approved", "active"} {
		if _, err := service.TransitionSystem(ctx, manifest.Metadata.ID, LifecycleTransitionInput{Status: status, Reason: "integration test review"}, actor); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.LoadApplicationManifest(ctx, manifest.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	for _, access := range []bool{false, true} {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			actor := authz.Context{UserID: "reader", Permissions: []authz.Permission{authz.PermissionSystemsLaunch}}
			if access {
				actor.AccessibleSystems = []string{manifest.Metadata.ID}
			}
			c.Set(authz.ContextKey, actor)
		})
		r.GET("/applications/:clientId/manifest", NewHandler(service).GetApplicationManifestRuntime)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/applications/"+manifest.Metadata.ID+"/manifest", nil))
		want := http.StatusForbidden
		if access {
			want = http.StatusOK
		}
		if response.Code != want {
			t.Fatalf("runtime access=%t: %d %s", access, response.Code, response.Body.String())
		}
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin"); err != nil {
		t.Fatalf("active idempotent retry failed: %v", err)
	}
	manifest.Metadata.Version = "1.1.0"
	preview, err = repo.ReviewManifest(ctx, manifest, input, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("changed active app accepted: %v", err)
	}
	if _, err := service.TransitionSystem(ctx, manifest.Metadata.ID, LifecycleTransitionInput{Status: "suspended", Reason: "update manifest"}, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin"); !errors.Is(err, ErrManifestReview) {
		t.Fatalf("stale lifecycle preview accepted: %v", err)
	}
	preview, err = repo.ReviewManifest(ctx, manifest, input, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE ihp_systems SET description='operator change' WHERE client_id=$1`, manifest.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin"); !errors.Is(err, ErrManifestReview) {
		t.Fatalf("stale metadata preview accepted: %v", err)
	}
}

func TestApplicationManifestNavigationAndStartupPostgres(t *testing.T) {
	repo, db := manifestTestRepository(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO ihp_permissions(permission_key) VALUES('metrics:read')`); err != nil {
		t.Fatal(err)
	}
	modules := []module.Info{{ID: "metrics", Version: "v0.1.0", Enabled: true}}
	manifest, input, err := parseApplicationManifest(manifestFixture(t, "internal"), "development", modules)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.ReviewManifest(ctx, manifest, input, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ihp_system_role_permissions(system_role_id,permission_id) SELECT sr.id,p.id FROM ihp_system_roles sr CROSS JOIN ihp_permissions p WHERE sr.role_name='metrics_viewer' AND p.permission_key='metrics:read'`); err != nil {
		t.Fatal(err)
	}
	seed := systemrbac.SeedFile{Systems: []systemrbac.SeedSystem{{ClientID: manifest.Metadata.ID, DisplayName: "Discovered name", LaunchURL: "/apps/changed", SystemType: "platform", AccessRoles: []string{"unexpected"}, Roles: []systemrbac.SeedRole{{Name: "unexpected"}}}}}
	startup := systemrbac.WithStartupDiscovery(ctx)
	if err := systemrbac.ApplySeed(startup, db, seed); err != nil {
		t.Fatal(err)
	}
	service := NewService(repo)
	if preserve, err := service.skipManifestStartup(startup, manifest.Metadata.ID); err != nil || !preserve {
		t.Fatalf("discovery would overwrite manifest: %t %v", preserve, err)
	}
	current, err := repo.GetSystem(ctx, manifest.Metadata.ID)
	if err != nil || len(current.Roles) != 1 || current.DisplayName != manifest.Metadata.Name {
		t.Fatalf("startup changed manifest application: %+v %v", current, err)
	}
	manifest.Metadata.Version = "1.1.0"
	manifest.Spec.Navigation = nil
	raw, _ := json.Marshal(manifest)
	manifest, input, err = parseApplicationManifest(string(raw), "development", modules)
	if err != nil {
		t.Fatal(err)
	}
	preview, err = repo.ReviewManifest(ctx, manifest, input, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReviewManifest(ctx, manifest, input, preview.ReviewToken, true, "admin"); err != nil {
		t.Fatal(err)
	}
	current, err = repo.GetSystem(ctx, manifest.Metadata.ID)
	if err != nil || current.Navigation != "" || current.DisplayInSideNav || len(current.Roles[0].Permissions) != 1 {
		t.Fatalf("navigation removal or role preservation failed: %+v %v", current, err)
	}
}

func TestApplicationManifestRollbackPostgres(t *testing.T) {
	repo, db := manifestTestRepository(t)
	_, err := db.Exec(`CREATE FUNCTION reject_manifest_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='application.manifest.imported' THEN RAISE EXCEPTION 'injected audit failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_manifest_audit BEFORE INSERT ON ihp_rbac_audit_events FOR EACH ROW EXECUTE FUNCTION reject_manifest_audit();`)
	if err != nil {
		t.Fatal(err)
	}
	manifest, input, err := parseApplicationManifest(manifestFixture(t, "external"), "development", nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.ReviewManifest(context.Background(), manifest, input, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReviewManifest(context.Background(), manifest, input, preview.ReviewToken, true, "admin"); err == nil {
		t.Fatal("injected failure was ignored")
	}
	for _, table := range []string{"ihp_systems", "ihp_system_roles", "ihp_system_access_roles", "ihp_rbac_audit_events"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial import in %s: %d %v", table, count, err)
		}
	}
}

func TestConcurrentApplicationManifestImportsPostgres(t *testing.T) {
	repo, db := manifestTestRepository(t)
	manifest, input, err := parseApplicationManifest(manifestFixture(t, "external"), "development", nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.ReviewManifest(context.Background(), manifest, input, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := repo.ReviewManifest(context.Background(), manifest, input, preview.ReviewToken, true, "admin")
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrManifestReview) {
			t.Fatal(err)
		}
	}
	if successes == 0 {
		t.Fatal("no concurrent import succeeded")
	}
	for _, table := range []string{"ihp_systems", "ihp_system_roles", "ihp_system_access_roles"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("duplicate or missing %s: %d %v", table, count, err)
		}
	}
}
