package report_scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/authz"
)

func TestHealthBIAdapterContract(t *testing.T) {
	const id = "Programs/Malaria/Test + report"
	const key = "scheduler-test-service-key-at-least-32-characters"
	var generated bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "report-scheduler-worker" || r.Form.Get("client_secret") != key {
				t.Error("invalid service account token request")
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "keycloak-access-token", "token_type": "Bearer", "expires_in": 300})
			return
		}
		if r.Header.Get("Authorization") != "Bearer keycloak-access-token" || r.Header.Get("X-API-Key") != "" {
			t.Errorf("expected dedicated service authentication")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		reportPath := "/report-browser/api/v1/reports/" + id
		switch r.URL.Path {
		case "/report-browser/api/v1/reports":
			json.NewEncoder(w).Encode(map[string]any{"data": []HealthBIReport{{ID: id, Name: "Test", SupportedFormats: []string{"pdf", "csv"}}}})
		case reportPath:
			if r.URL.EscapedPath() != "/report-browser/api/v1/reports/"+url.PathEscape(id) {
				t.Errorf("category ID not escaped: %s", r.URL.EscapedPath())
			}
			json.NewEncoder(w).Encode(map[string]any{"data": HealthBIReport{ID: id, Name: "Test", SupportedFormats: []string{"pdf", "csv"}, Metadata: map[string]any{"periodStrategies": []string{"previous_month"}}}})
		case reportPath + "/parameters":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "basket", "type": "string", "required": false, "defaultValue": "EMHS"}}})
		case reportPath + "/generate":
			if r.Method != "POST" {
				t.Errorf("method: %s", r.Method)
			}
			var input GenerateReportRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				return
			}
			period, _ := input.Parameters["reporting_period"].(map[string]any)
			scope, _ := input.Parameters["health_context"].(map[string]any)
			if input.Format != "csv" || period["start"] != "2026-08-31T21:00:00Z" || period["end"] != "2026-09-30T20:59:59Z" || scope["district"] != "Wakiso District" {
				t.Errorf("unexpected generation payload: %+v", input)
			}
			generated = true
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]any{"data": HealthBIJob{ID: "job-1", ReportID: id, Status: "queued"}})
		case "/report-browser/api/v1/report-jobs/job-1":
			json.NewEncoder(w).Encode(map[string]any{"data": HealthBIJob{ID: "job-1", ReportID: id, Status: "completed", ArtifactURL: "https://datawarehouse-report-browser.example/report-browser/api/v1/report-artifacts/job-1?token=signed"}})
		default:
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	client := NewKeycloakHealthBIClient(upstream.URL+"/report-browser/", upstream.URL+"/token", "report-scheduler-worker", key, time.Second)
	ctx := context.Background()
	parameters, err := client.GetReportParameters(ctx, id)
	if err != nil || len(parameters) != 1 || parameters[0].DefaultValue != "EMHS" {
		t.Fatalf("report parameter defaults lost: %+v, %v", parameters, err)
	}
	reports, err := client.ListReports(ctx)
	if err != nil || len(reports) != 1 || reports[0].ID != id {
		t.Fatalf("catalogue: %+v, %v", reports, err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	allowed := authz.NewContext("test-user", nil, map[string][]string{authz.SystemDataStatistics: {authz.ReportSchedulerAccess}})
	group := router.Group("/api/v1", func(c *gin.Context) { c.Set(authz.ContextKey, allowed) })
	RegisterProtectedRoutes(group, NewHandler(NewService(nil, client, nil, nil, nil, nil)))
	for _, path := range []string{"/report", "/report-parameters"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/report-scheduler"+path+"?reportId="+url.QueryEscape(id), nil))
		if res.Code != 200 {
			t.Fatalf("nested report route %s: %d %s", path, res.Code, res.Body.String())
		}
	}
	for _, path := range []string{"/report", "/report-parameters"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("GET", "/api/v1/report-scheduler"+path, nil))
		if res.Code != 400 {
			t.Fatalf("missing report ID: %d", res.Code)
		}
	}
	service := NewService(nil, client, nil, nil, nil, nil)
	input := CreateScheduleRequest{HealthBIReportID: id, Frequency: "monthly", Timezone: "Africa/Kampala", PeriodStrategy: "previous_month", OutputFormat: "csv"}
	if _, err := service.validateSchedule(ctx, input, HealthContext{}); err != nil {
		t.Fatalf("compatible schedule rejected: %v", err)
	}
	input.PeriodStrategy = "previous_day"
	if _, err := service.validateSchedule(ctx, input, HealthContext{}); err == nil {
		t.Fatal("unsupported daily reporting period accepted")
	}
	input.PeriodStrategy = "previous_month"
	input.Timezone = "UTC"
	if _, err := service.validateSchedule(ctx, input, HealthContext{}); err == nil {
		t.Fatal("incompatible reporting timezone accepted")
	}
	period, err := ResolveReportingPeriod("previous_month", "Africa/Kampala", time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	job, err := client.GenerateReport(ctx, id, GenerateReportRequest{Format: "csv", Parameters: buildExecutionParameters(Schedule{HealthContext: HealthContext{Level: "district", District: "Wakiso District"}}, period)})
	if err != nil || !generated || job.Status != "queued" {
		t.Fatalf("generate: %+v, %v", job, err)
	}
	job, err = client.GetJob(ctx, job.ID)
	if err != nil || job.Status != "completed" || !strings.Contains(job.ArtifactURL, "?token=") {
		t.Fatalf("poll: %+v, %v", job, err)
	}
}

func TestValidateReportPeriod(t *testing.T) {
	for _, tc := range []struct {
		name           string
		metadata       map[string]any
		strategy, zone string
		wantError      bool
	}{
		{"supported", map[string]any{"periodStrategies": []any{"previous_month"}}, "previous_month", "Africa/Kampala", false},
		{"daily rejected", map[string]any{"periodStrategies": []any{"previous_month"}}, "previous_day", "Africa/Kampala", true},
		{"empty rejected", map[string]any{"periodStrategies": []any{}}, "previous_month", "Africa/Kampala", true},
		{"timezone rejected", map[string]any{"periodStrategies": []any{"previous_month"}}, "previous_month", "UTC", true},
		{"malformed rejected", map[string]any{"periodStrategies": "previous_month"}, "previous_month", "Africa/Kampala", true},
		{"legacy omitted", nil, "previous_day", "UTC", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReportPeriod(HealthBIReport{Name: "Test", Metadata: tc.metadata}, tc.strategy, tc.zone)
			if (err != nil) != tc.wantError {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestHealthBIRequiresServiceKey(t *testing.T) {
	client := NewKeycloakHealthBIClient("https://datawarehouse-report-browser.example/report-browser", "https://keycloak.example/token", "report-scheduler-worker", "", time.Second)
	if client.Enabled() {
		t.Fatal("scheduler integration must remain disabled without its service key")
	}
	if _, err := client.ListReports(context.Background()); err == nil {
		t.Fatal("unconfigured integration must fail before making an HTTP request")
	}
}
