package health_context

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/moh-sso-dashboard/internal/geography"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	surveillance "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func TestGeographySyncAcrossDatabases(t *testing.T) {
	dsn := os.Getenv("MOH_HEALTH_CONTEXT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL fixture")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	openDatabase := func(prefix string) *sql.DB {
		name := prefix + uuid.NewString()[:8]
		if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		u.Path = "/" + name
		db, err := sql.Open("postgres", u.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			db.Close()
			if _, err := admin.ExecContext(ctx, `DROP DATABASE "`+name+`" WITH (FORCE)`); err != nil {
				t.Error(err)
			}
		})
		return db
	}
	primary := openDatabase("health_primary_")
	observatory := openDatabase("health_observatory_")
	if _, err := primary.ExecContext(ctx, `CREATE TABLE ihp_rbac_groups (id uuid PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000054_add_health_contexts.up.sql", "000056_bridge_surveillance_health_contexts.up.sql"} {
		contents, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := primary.ExecContext(ctx, string(contents)); err != nil {
			t.Fatal(err)
		}
	}
	// An upgrade must reuse a previously assigned context and preserve disabled state.
	region, district, facility := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		namespace, kind, name, parent string
		id, parentID                  uuid.UUID
	}{
		{"surveillance-region", "REGION", "Existing region", "", region, uuid.Nil},
		{"surveillance-district", "DISTRICT", "Existing district", "surveillance-region", district, region},
		{"surveillance-facility", "FACILITY", "Existing facility", "surveillance-district", facility, district},
	} {
		if _, err := primary.ExecContext(ctx, `SELECT sync_surveillance_health_context($1,$2::uuid,$7,$3,$4,NULLIF($5,''),NULLIF($6,'00000000-0000-0000-0000-000000000000')::uuid)`, q.namespace, q.id, q.name, q.kind, q.parent, q.parentID.String(), q.namespace+q.id.String()); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(primary)
	before, err := repo.GetNodeByAlias(ctx, "surveillance-facility", facility.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := primary.ExecContext(ctx, `UPDATE health_context_nodes SET enabled=FALSE WHERE id=$1;`, before.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := primary.ExecContext(ctx, `INSERT INTO user_health_context_assignments(user_id,context_node_id) VALUES('assigned-user',$1)`, before.ID); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "000060_primary_geography_registry.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := primary.ExecContext(ctx, string(contents)); err != nil {
		t.Fatal(err)
	}
	// No Observatory schema or connection is needed to list the complete registry.
	regions, err := repo.ListNodes(ctx, nil, "REGION", true)
	if err != nil || len(regions) != 18 {
		t.Fatalf("offline regions: %d %v", len(regions), err)
	}
	after, err := repo.GetNodeByAlias(ctx, "geography-facility", facility.String())
	if err != nil || after.ID != before.ID || after.Enabled {
		t.Fatalf("upgrade changed assigned context: %+v %v", after, err)
	}
	ancestors, err := repo.ListAncestors(ctx, after.ID)
	if err != nil || len(ancestors) != 4 {
		t.Fatalf("upgraded context lost ancestry: %d %v", len(ancestors), err)
	}
	if err := repo.DeleteNode(ctx, after.ID); !errors.Is(err, ErrContextInUse) {
		t.Fatalf("registry context deletion allowed: %v", err)
	}
	var assignments int
	if err := primary.QueryRowContext(ctx, `SELECT count(*) FROM user_health_context_assignments WHERE user_id='assigned-user' AND context_node_id=$1`, before.ID).Scan(&assignments); err != nil || assignments != 1 {
		t.Fatalf("assignment lost: %d %v", assignments, err)
	}
	for _, name := range []string{"000001_surveillance.up.sql", "000002_staging_writes.up.sql", "000003_reference_geography.up.sql", "000004_geography_replica.up.sql"} {
		path := filepath.Join("..", "..", "db", "surveillance_migrations", name)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = observatory.ExecContext(ctx, string(contents)); err != nil {
			t.Fatal(err)
		}
	}
	var sourceID, remoteID string
	if err := primary.QueryRowContext(ctx, `SELECT id FROM geography.regions WHERE name='Acholi'`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := observatory.QueryRowContext(ctx, `SELECT id FROM regions WHERE name='Acholi'`).Scan(&remoteID); err != nil {
		t.Fatal(err)
	}
	if sourceID == remoteID {
		t.Fatal("fixture must exercise differing registry UUIDs")
	}
	custom := uuid.New()
	if _, err := observatory.ExecContext(ctx, `INSERT INTO facilities(id,name,region_id,facility_type) VALUES($1,'Operational facility',$2,'Original type')`, custom, remoteID); err != nil {
		t.Fatal(err)
	}
	if _, err := observatory.ExecContext(ctx, `INSERT INTO facility_weekly_metrics(facility_id,disease_id,epi_week_id,metric_value,source_name) SELECT $1,d.id,w.id,42,'preserve-facts' FROM diseases d CROSS JOIN epi_weeks w ORDER BY d.id,w.id LIMIT 1`, custom); err != nil {
		t.Fatal(err)
	}
	replica := geography.NewReplicator(primary, observatory)
	if err := replica.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	customContext, err := repo.GetNodeByAlias(ctx, "surveillance-facility", custom.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := primary.ExecContext(ctx, `UPDATE geography.facilities SET name='Primary renamed facility',facility_type='Primary type' WHERE id=$1`, custom); err != nil {
		t.Fatal(err)
	}
	if err := replica.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	var name, kind string
	if err := observatory.QueryRowContext(ctx, `SELECT name,facility_type FROM facilities WHERE id=$1`, custom).Scan(&name, &kind); err != nil || name != "Primary renamed facility" || kind != "Primary type" {
		t.Fatalf("replica did not follow primary: %q %q %v", name, kind, err)
	}
	var reports int
	if err := observatory.QueryRowContext(ctx, `SELECT count(*) FROM facility_weekly_metrics WHERE facility_id=$1 AND metric_value=42`, custom).Scan(&reports); err != nil || reports != 1 {
		t.Fatalf("existing report lost: %d %v", reports, err)
	}
	if _, err := observatory.ExecContext(ctx, `UPDATE facilities SET name='Replica edit' WHERE id=$1`, custom); err != nil {
		t.Fatal(err)
	}
	if err := geography.NewReplicator(primary, observatory).Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := observatory.QueryRowContext(ctx, `SELECT name FROM facilities WHERE id=$1`, custom).Scan(&name); err != nil || name != "Primary renamed facility" {
		t.Fatalf("replica became authoritative after restart: %q %v", name, err)
	}
	current, err := repo.GetNodeByAlias(ctx, "surveillance-facility", custom.String())
	if err != nil || current.ID != customContext.ID {
		t.Fatalf("context identity changed: %+v %v", current, err)
	}
	// Location APIs and per-row fact imports share this writer (now owned by the
	// Surveillance module). A returned ID is always the destination ID, even
	// when its parent has a different canonical ID.
	surveillanceModule, err := surveillance.NewModule(surveillance.Dependencies{
		Database: observatory, GeographyPrimary: primary, ReplicateGeography: replica.Sync,
		HealthContext: ModuleDirectory{Service: NewService(repo)},
	})
	if err != nil {
		t.Fatal(err)
	}
	callSurveillance := func(method, target, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, target, strings.NewReader(body))
		request = request.WithContext(modulekit.WithRequest(ctx, modulekit.Request{Authenticated: true, Access: core.RequestAccess{
			Identity: core.Identity{Subject: "geography-test"}, Systems: []string{surveillance.System},
			Permissions: []string{surveillance.ReadPermission, surveillance.ManageLocationsPermission},
		}}))
		response := httptest.NewRecorder()
		surveillanceModule.ServeHTTP(response, request)
		return response
	}
	response := callSurveillance("POST", "/surveillance/districts", `{"name":"Primary API district","region_id":"`+remoteID+`"}`)
	var createdDistrict struct {
		Data struct {
			ID       uuid.UUID `json:"id"`
			RegionID string    `json:"region_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &createdDistrict); err != nil || response.Code != http.StatusOK {
		t.Fatalf("district upsert: %d %s", response.Code, response.Body.String())
	}
	if createdDistrict.Data.RegionID != remoteID {
		t.Fatal("API returned primary parent UUID instead of replica UUID")
	}
	var canonicalRegion string
	if err := primary.QueryRowContext(ctx, `SELECT region_id FROM geography.districts WHERE name='Primary API district'`).Scan(&canonicalRegion); err != nil || canonicalRegion != sourceID {
		t.Fatalf("write did not resolve canonical parent: %s %v", canonicalRegion, err)
	}
	// A facility-metrics import creates its sub-county and facility inside the
	// row transaction; both must still be written to primary.
	var regionName, diseaseName string
	if err := observatory.QueryRowContext(ctx, `SELECT name FROM regions WHERE id=$1`, remoteID).Scan(&regionName); err != nil {
		t.Fatal(err)
	}
	if err := observatory.QueryRowContext(ctx, `SELECT name FROM diseases ORDER BY name LIMIT 1`).Scan(&diseaseName); err != nil {
		t.Fatal(err)
	}
	document, batch := uuid.New(), uuid.New()
	payload, _ := json.Marshal(map[string]any{"record_id": "r1", "facility": "Imported facility", "region": regionName, "district": "Primary API district",
		"sub_county": "Imported subcounty", "disease": diseaseName, "value": 3, "year": 2026, "weeks": 2})
	if _, err := observatory.ExecContext(ctx, `INSERT INTO surveillance_import_batches(id,source_name,dataset_type,status,total_rows,document_id) VALUES($1,'test','facility_metrics','PENDING',1,$2)`, batch, document); err != nil {
		t.Fatal(err)
	}
	if _, err := observatory.ExecContext(ctx, `INSERT INTO surveillance_import_raw_rows(batch_id,row_number,payload) VALUES($1,1,$2)`, batch, payload); err != nil {
		t.Fatal(err)
	}
	if err := surveillanceModule.ProcessImportBatch(ctx, surveillance.ImportJob{DocumentID: document}); err != nil {
		t.Fatal(err)
	}
	var imported int
	if err := primary.QueryRowContext(ctx, `SELECT count(*) FROM geography.facilities WHERE name='Imported facility'`).Scan(&imported); err != nil || imported != 1 {
		t.Fatalf("import bypassed primary: %d %v", imported, err)
	}
	if response := callSurveillance("DELETE", "/surveillance/subcounties/"+uuid.NewString(), ""); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "GEOGRAPHY_DELETE_RESTRICTED") {
		t.Fatalf("destructive delete must be blocked: %d %s", response.Code, response.Body.String())
	}
	// Re-applying a data-preserving migration rollback cannot adopt destination
	// UUIDs a second time, replace canonical rows or change active assignments.
	contents, err = os.ReadFile(filepath.Join("..", "..", "db", "migrations", "000060_primary_geography_registry.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = primary.ExecContext(ctx, string(contents)); err != nil {
		t.Fatal(err)
	}
	var registryID string
	if err := primary.QueryRowContext(ctx, `SELECT id FROM geography.registry`).Scan(&registryID); err != nil {
		t.Fatal(err)
	}
	if _, err := primary.ExecContext(ctx, `UPDATE geography.registry SET id=gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}
	if err := geography.NewReplicator(primary, observatory).Sync(ctx); err == nil {
		t.Fatal("replica accepted a different primary registry")
	}
	if _, err := primary.ExecContext(ctx, `UPDATE geography.registry SET id=$1`, registryID); err != nil {
		t.Fatal(err)
	}
	observatory.Close()
	deadline, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := replica.Sync(deadline); err == nil {
		t.Fatal("closed replica must not report successful synchronization")
	}
	if _, err := repo.ListNodes(ctx, nil, "REGION", true); err != nil {
		t.Fatalf("Observatory outage blocked primary contexts: %v", err)
	}
}
