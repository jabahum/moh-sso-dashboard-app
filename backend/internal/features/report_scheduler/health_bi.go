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
	DownloadArtifact(context.Context, string, int64) ([]byte, string, error)
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
	client := &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	return &healthBIClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		tokens:     &keycloakTokenProvider{endpoint: strings.TrimSpace(tokenURL), clientID: strings.TrimSpace(clientID), clientSecret: clientSecret, client: client},
		httpClient: client,
	}
}

// HealthBIRequestError preserves the status and decodes the public error envelope.
type HealthBIRequestError struct {
	StatusCode int
	Message    string
}

func (e *HealthBIRequestError) Error() string {
	return fmt.Sprintf("Health BI returned %d: %s", e.StatusCode, e.Message)
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
	resp, err := c.request(ctx, method, path, payload, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read Health BI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := "request failed"
		var envelope struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(payload, &envelope) == nil && strings.TrimSpace(envelope.Error) != "" {
			message = strings.TrimSpace(envelope.Error)
		}
		if len(message) > 500 {
			message = message[:500]
		}
		return &HealthBIRequestError{StatusCode: resp.StatusCode, Message: message}
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

func (c *healthBIClient) request(ctx context.Context, method, path string, payload []byte, accept string) (*http.Response, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Health BI is not configured")
	}
	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.tokens.get(ctx)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("create Health BI request: %w", err)
		}
		req.Header.Set("Accept", accept)
		if len(payload) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Health BI request failed: %w", err)
		}
		if resp.StatusCode != http.StatusUnauthorized || attempt == 1 {
			break
		}
		resp.Body.Close()
		c.tokens.invalidate(token)
	}
	return resp, nil
}

// Fetch only the configured service's job artifact endpoint. Never follow a job's
// externally supplied artifact URL with the service-account bearer credential.
func (c *healthBIClient) DownloadArtifact(ctx context.Context, jobID string, maxBytes int64) ([]byte, string, error) {
	if strings.TrimSpace(jobID) == "" {
		return nil, "", fmt.Errorf("Health BI job ID is required for the email attachment")
	}
	if maxBytes <= 0 {
		return nil, "", fmt.Errorf("attachment size limit must be positive")
	}
	resp, err := c.request(ctx, http.MethodGet, "/api/v1/report-jobs/"+url.PathEscape(jobID)+"/artifact", nil, "application/pdf, text/csv")
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download Health BI attachment returned %s", resp.Status)
	}
	if resp.ContentLength > maxBytes {
		return nil, "", fmt.Errorf("report exceeds email attachment limit (%d bytes)", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read Health BI attachment: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("report exceeds email attachment limit (%d bytes)", maxBytes)
	}
	return data, resp.Header.Get("Content-Type"), nil
}
