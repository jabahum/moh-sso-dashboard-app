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
	tokens     *keycloakTokenProvider
	httpClient *http.Client
}

func NewKeycloakHealthBIClient(baseURL, tokenURL, clientID, clientSecret string, timeout time.Duration) HealthBIClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	return &healthBIClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		tokens:     &keycloakTokenProvider{endpoint: strings.TrimSpace(tokenURL), clientID: strings.TrimSpace(clientID), clientSecret: clientSecret, client: client},
		httpClient: client,
	}
}

func (c *healthBIClient) Enabled() bool {
	return c != nil && c.baseURL != "" && c.tokens != nil && c.tokens.configured()
}

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
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Health BI request: %w", err)
		}
	}
	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.tokens.get(ctx)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create Health BI request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("Health BI request failed: %w", err)
		}
		if resp.StatusCode != http.StatusUnauthorized || attempt == 1 {
			break
		}
		resp.Body.Close()
		c.tokens.invalidate(token)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read Health BI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(payload))
		if len(message) > 500 {
			message = message[:500]
		}
		return fmt.Errorf("Health BI returned %s: %s", resp.Status, message)
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("decode Health BI response data: %w", err)
		}
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode Health BI response: %w", err)
	}
	return nil
}
