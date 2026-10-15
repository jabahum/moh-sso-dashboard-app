package report_scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDownloadReportArtifactWithKeycloakAndBoundedSize(t *testing.T) {
	for _, scenario := range []string{"pdf", "chunked-oversize", "oversize", "unavailable", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					json.NewEncoder(w).Encode(map[string]any{"access_token": "service-token", "token_type": "Bearer", "expires_in": 300})
					return
				}
				if r.URL.Path != "/report-browser/api/v1/report-jobs/job-1/artifact" || r.Header.Get("Authorization") != "Bearer service-token" {
					t.Errorf("wrong artifact endpoint/authentication: %s", r.URL.Path)
					w.WriteHeader(401)
					return
				}
				if scenario == "unavailable" {
					w.WriteHeader(404)
					return
				}
				if scenario == "redirect" {
					w.Header().Set("Location", "/unexpected-download-target")
					w.WriteHeader(http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", "application/pdf")
				if scenario == "chunked-oversize" {
					w.(http.Flusher).Flush()
				}
				fmt.Fprint(w, "%PDF-1.7\nreport")
			}))
			defer server.Close()
			client := NewKeycloakHealthBIClient(server.URL+"/report-browser", server.URL+"/token", "worker", "secret", time.Second)
			limit := int64(1024)
			if strings.Contains(scenario, "oversize") {
				limit = 4
			}
			data, contentType, err := client.DownloadArtifact(context.Background(), "job-1", limit)
			if scenario == "pdf" {
				if err != nil || !strings.HasPrefix(string(data), "%PDF-") || contentType != "application/pdf" {
					t.Fatalf("download: %s %s %v", data, contentType, err)
				}
			} else if err == nil {
				t.Fatal("invalid artifact download was accepted")
			}
		})
	}
}
