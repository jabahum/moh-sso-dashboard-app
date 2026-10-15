package report_scheduler

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/moh-sso-dashboard/internal/authz"
)

type recipientOptionsRepository struct {
	Repository
	called bool
}

func (r *recipientOptionsRepository) ListRecipientOptions(context.Context, string, string, int, []string) ([]RecipientOption, error) {
	r.called = true
	return []RecipientOption{{ID: "user:123", Type: "user", Value: "123", Label: "Test User (test@example.invalid)"}}, nil
}

func TestRecipientOptionsRequireScheduleWritePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name                  string
		permissions           []authz.Permission
		system, authenticated bool
		status                int
	}{
		{"unauthenticated", nil, false, false, 401},
		{"read only", []authz.Permission{authz.PermissionReportSchedulerRead}, true, true, 403},
		{"creator", []authz.Permission{authz.PermissionReportSchedulerCreate}, true, true, 200},
		{"editor", []authz.Permission{authz.PermissionReportSchedulerUpdate}, true, true, 200},
		{"no parent access", []authz.Permission{authz.PermissionReportSchedulerCreate}, false, true, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &recipientOptionsRepository{}
			router := gin.New()
			group := router.Group("/api/v1", func(c *gin.Context) {
				if tc.authenticated {
					permissions := authz.Context{Permissions: tc.permissions}
					if tc.system {
						permissions.AccessibleSystems = []string{authz.SystemDataStatistics}
					}
					c.Set(authz.ContextKey, permissions)
				}
			})
			RegisterProtectedRoutes(group, NewHandler(NewService(repo, nil, nil, nil, nil, nil)))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/report-scheduler/recipients/options?kind=email", nil))
			if response.Code != tc.status || repo.called != (tc.status == 200) {
				t.Fatalf("status=%d called=%v body=%s", response.Code, repo.called, response.Body.String())
			}
		})
	}
}

func TestRecipientOptionsValidateQueries(t *testing.T) {
	for _, query := range []string{"kind=admin", "kind=email&limit=0", "kind=email&limit=101", "kind=email&limit=no", "search=" + strings.Repeat("x", 201)} {
		router := gin.New()
		repo := &recipientOptionsRepository{}
		handler := NewHandler(NewService(repo, nil, nil, nil, nil, nil))
		router.GET("/options", handler.RecipientOptions)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/options?"+query, nil))
		if response.Code != 400 || repo.called {
			t.Fatalf("invalid query accepted: %s status=%d", query, response.Code)
		}
	}
}

func TestRecipientOptionsPostgresDirectory(t *testing.T) {
	dsn := os.Getenv("TEST_REPORT_SCHEDULER_DB_URL")
	if dsn == "" {
		t.Skip("set TEST_REPORT_SCHEDULER_DB_URL to a disposable PostgreSQL database")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx := context.Background()
	// Session-local temporary tables shadow real tables and are removed on close.
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE users(id TEXT,username TEXT,first_name TEXT,last_name TEXT,email TEXT,enabled BOOLEAN);
 CREATE TEMP TABLE ihp_rbac_groups(id TEXT,name TEXT,display_name TEXT,path TEXT,enabled BOOLEAN);
 INSERT INTO users VALUES ('one','testuser','Test','User','test@example.invalid',TRUE),('two','noemail','No','Email','',TRUE),('three','disabled','Hidden','User','hidden@example.invalid',FALSE),('four','service-account-worker','','','service@example.invalid',TRUE);
 INSERT INTO ihp_rbac_groups VALUES ('group-one','Schedulers','Report Schedulers','/MOH/Schedulers',TRUE),('group-two','Disabled','Disabled','/Hidden',FALSE);`)
	if err != nil {
		t.Fatal(err)
	}
	repo := &postgresRepository{db: db}
	options, err := repo.ListRecipientOptions(ctx, "email", "", 100, nil)
	if err != nil || len(options) != 1 || options[0].Value != "one" || !strings.Contains(options[0].Label, "test@example.invalid") {
		t.Fatalf("email directory: %+v %v", options, err)
	}
	options, err = repo.ListRecipientOptions(ctx, "portal", "", 100, nil)
	if err != nil || len(options) != 2 {
		t.Fatalf("portal directory: %+v %v", options, err)
	}
	options, err = repo.ListRecipientOptions(ctx, "group", "Report", 100, nil)
	if err != nil || len(options) != 1 || options[0].Value != "group-one" || !strings.Contains(options[0].Label, "/MOH/Schedulers") {
		t.Fatalf("groups: %+v %v", options, err)
	}
	options, err = repo.ListRecipientOptions(ctx, "email", "not-found", 100, nil)
	if err != nil || len(options) != 0 {
		t.Fatalf("search not applied: %+v %v", options, err)
	}
	options, err = repo.ListRecipientOptions(ctx, "portal", "not-found", 100, []string{"one"})
	if err != nil || len(options) != 1 || options[0].Value != "one" {
		t.Fatalf("saved user selection missing: %+v %v", options, err)
	}
	options, err = repo.ListRecipientOptions(ctx, "group", "not-found", 100, []string{"/MOH/Schedulers"})
	if err != nil || len(options) != 1 || options[0].Value != "/MOH/Schedulers" {
		t.Fatalf("saved group path not preserved: %+v %v", options, err)
	}
	options, err = repo.ListRecipientOptions(ctx, "email", "not-found", 100, []string{"test@example.invalid"})
	if err != nil || len(options) != 1 || options[0].Type != "email" || options[0].Value != "test@example.invalid" {
		t.Fatalf("saved email not preserved: %+v %v", options, err)
	}
	options, err = repo.ListRecipientOptions(ctx, "portal", "", 1, nil)
	if err != nil || len(options) != 1 {
		t.Fatalf("limit not applied: %+v %v", options, err)
	}
}
