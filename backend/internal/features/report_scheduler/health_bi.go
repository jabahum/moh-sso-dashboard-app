package report_scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HealthBIClient interface {
	Enabled() bool
	ListReports(context.Context) ([]HealthBIReport, error)
	GetReport(context.Context, string) (HealthBIReport, error)
	GetReportParameters(context.Context, string) ([]HealthBIParameter, error)
	GenerateReport(context.Context, string, GenerateReportRequest) (HealthBIJob, error)
	GetJob(context.Context, string) (HealthBIJob, error)
}

type healthBIClient struct {
	baseURL    string
	token      string
	authHeader string
	httpClient *http.Client
}

func NewHealthBIClient(baseURL, token, authHeader string, timeout time.Duration) HealthBIClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if strings.TrimSpace(authHeader) == "" {
		authHeader = "Authorization"
	}
	return &healthBIClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token: strings.TrimSpace(token),
		authHeader: strings.TrimSpace(authHeader),
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *healthBIClient) Enabled() bool { return c != nil && c.baseURL != "" }

func (c *healthBIClient) ListReports(ctx context.Context) ([]HealthBIReport, error) {
	var out []HealthBIReport
	err := c.do(ctx, http.MethodGet, "/api/v1/reports", nil, &out)
	return out, err
}

func (c *healthBIClient) GetReport(ctx context.Context, reportID string) (HealthBIReport, error) {
	var out HealthBIReport
	err := c.do(ctx, http.MethodGet, "/api/v1/reports/"+url.PathEscape(reportID), nil, &out)
	return out, err
}

func (c *healthBIClient) GetReportParameters(ctx context.Context, reportID string) ([]HealthBIParameter, error) {
	var out []HealthBIParameter
	err := c.do(ctx, http.MethodGet, "/api/v1/reports/"+url.PathEscape(reportID)+"/parameters", nil, &out)
	return out, err
}

func (c *healthBIClient) GenerateReport(ctx context.Context, reportID string, request GenerateReportRequest) (HealthBIJob, error) {
	var out HealthBIJob
	err := c.do(ctx, http.MethodPost, "/api/v1/reports/"+url.PathEscape(reportID)+"/generate", request, &out)
	return out, err
}

func (c *healthBIClient) GetJob(ctx context.Context, jobID string) (HealthBIJob, error) {
	var out HealthBIJob
	err := c.do(ctx, http.MethodGet, "/api/v1/report-jobs/"+url.PathEscape(jobID), nil, &out)
	return out, err
}

func (c *healthBIClient) do(ctx context.Context, method, path string, body any, target any) error {
	if !c.Enabled() {
		return fmt.Errorf("Health BI is not configured")
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil { return fmt.Errorf("encode Health BI request: %w", err) }
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil { return fmt.Errorf("create Health BI request: %w", err) }
	req.Header.Set("Accept", "application/json")
	if body != nil { req.Header.Set("Content-Type", "application/json") }
	if c.token != "" {
		value := c.token
		if strings.EqualFold(c.authHeader, "Authorization") && !strings.Contains(value, " ") { value = "Bearer " + value }
		req.Header.Set(c.authHeader, value)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil { return fmt.Errorf("Health BI request failed: %w", err) }
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil { return fmt.Errorf("read Health BI response: %w", err) }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(payload))
		if len(message) > 500 { message = message[:500] }
		return fmt.Errorf("Health BI returned %s: %s", resp.Status, message)
	}
	if target == nil || len(payload) == 0 { return nil }
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, target); err != nil { return fmt.Errorf("decode Health BI response data: %w", err) }
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil { return fmt.Errorf("decode Health BI response: %w", err) }
	return nil
}
