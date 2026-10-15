package report_scheduler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type rejectedGenerationRepository struct {
	Repository
	retried bool
	status  string
}

func (r *rejectedGenerationRepository) MarkGenerationAttempt(context.Context, string) error {
	return nil
}
func (r *rejectedGenerationRepository) ScheduleExecutionRetry(context.Context, string, string, time.Time) error {
	r.retried = true
	return nil
}
func (r *rejectedGenerationRepository) UpdateExecutionJob(_ context.Context, _, _, status, _ string) error {
	r.status = status
	return nil
}
func (r *rejectedGenerationRepository) ClaimFailureNotification(context.Context, string) (bool, error) {
	return false, nil
}

type rejectedGenerationClient struct {
	HealthBIClient
	err error
}

func (c *rejectedGenerationClient) GenerateReport(context.Context, string, GenerateReportRequest) (HealthBIJob, error) {
	return HealthBIJob{}, c.err
}
func TestGenerationRejectionsDoNotRetryUnchangedRequests(t *testing.T) {
	for _, code := range []int{400, 404, 422, 429, 500, 503} {
		repo := &rejectedGenerationRepository{}
		service := NewService(repo, &rejectedGenerationClient{err: &HealthBIRequestError{StatusCode: code, Message: "rejected"}}, nil, nil, nil, nil)
		if err := service.submitExecution(context.Background(), Schedule{}, Execution{MaxGenerationAttempts: 3}); err != nil {
			t.Fatal(err)
		}
		permanent := code == 400 || code == 404 || code == 422
		if repo.retried == permanent || permanent && repo.status != "failed" {
			t.Fatalf("code=%d retried=%v status=%s", code, repo.retried, repo.status)
		}
	}
}
func TestHealthBIErrorEnvelopeIsReadable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			w.Write([]byte(`{"access_token":"test","expires_in":300,"token_type":"Bearer"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"component \"ANC1 \u0026 PNC\" cannot apply the requested month filter"}`))
	}))
	defer server.Close()
	client := NewKeycloakHealthBIClient(server.URL, server.URL+"/token", "worker", "secret", time.Second)
	_, err := client.GetReport(context.Background(), "sample")
	var rejection *HealthBIRequestError
	if !errors.As(err, &rejection) || rejection.StatusCode != 400 || !strings.Contains(err.Error(), `ANC1 & PNC`) || strings.Contains(err.Error(), `\u0026`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
