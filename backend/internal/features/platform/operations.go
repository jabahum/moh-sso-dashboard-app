package platform

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/moh-sso-dashboard/internal/http/response"
)

type WorkerHealth struct {
	LocalStatus *core.WorkerSnapshot `json:"localStatus,omitempty"`
	Name        string               `json:"name"`
	LastSeen    string               `json:"lastSeen,omitempty"`
	Healthy     bool                 `json:"healthy"`
}
type Readiness struct {
	Ready   bool           `json:"ready"`
	Workers []WorkerHealth `json:"workers"`
}
type ApplicationOperations struct {
	ApplicationID       string         `json:"applicationId"`
	LifecycleStatus     string         `json:"lifecycleStatus"`
	Enabled             bool           `json:"enabled"`
	IdentityCount       int            `json:"identityCount"`
	EnabledIdentities   int            `json:"enabledIdentities"`
	Requests24Hours     int            `json:"requests24Hours"`
	StatusCounts24Hours map[string]int `json:"statusCounts24Hours"`
	OldestPendingAt     string         `json:"oldestPendingAt,omitempty"`
	Workers             []WorkerHealth `json:"workers"`
}

func (h *Handler) GetReadiness(c *gin.Context) {
	if h.CoreReady != nil {
		if err := h.CoreReady(c.Request.Context()); err != nil {
			fail(c, ErrUnavailable)
			return
		}
	}
	health, err := h.enabledWorkerHealth(c.Request.Context())
	if err != nil {
		fail(c, ErrUnavailable)
		return
	}
	result := Readiness{Ready: true, Workers: health}
	for _, worker := range health {
		result.Ready = result.Ready && worker.Healthy
	}

	if !result.Ready {
		response.OK(c, http.StatusServiceUnavailable, result)
		return
	}
	response.OK(c, http.StatusOK, result)
}
func (h *Handler) GetOperations(c *gin.Context) {
	result, err := h.Repository.Operations(c.Request.Context(), c.Param("clientId"))
	if err != nil {
		fail(c, err)
		return
	}
	result.Workers, err = h.enabledWorkerHealth(c.Request.Context())
	if err != nil {
		fail(c, ErrUnavailable)
		return
	}
	response.OK(c, http.StatusOK, result)
}
func (h *Handler) enabledWorkerHealth(ctx context.Context) ([]WorkerHealth, error) {
	required := h.RequiredWorkers
	if required == nil {
		required = []string{"notification-email", "email-outbox"}
	}
	result := make([]WorkerHealth, 0, len(required))
	if len(required) == 0 {
		return result, nil
	}
	health, err := h.Repository.WorkerHealth(ctx, h.staleAfter())
	if err != nil {
		return nil, err
	}
	for _, name := range required {
		item := WorkerHealth{Name: name}
		for _, candidate := range health {
			if candidate.Name == name {
				item = candidate
				break
			}
		}
		if status := h.WorkerStatuses[name]; status != nil {
			snapshot := status.Snapshot()
			item.LocalStatus = &snapshot
		}
		result = append(result, item)
	}
	return result, nil
}
func (h *Handler) staleAfter() time.Duration {
	if h.WorkerStaleAfter < 2*time.Minute {
		return 2 * time.Minute
	}
	return h.WorkerStaleAfter
}
func (r *PostgresRepository) RecordWorkerHeartbeat(ctx context.Context, name string) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO ihp_platform_worker_heartbeats(worker_name) VALUES ($1) ON CONFLICT(worker_name) DO UPDATE SET last_seen=now()`, name)
	return err
}
func (r *PostgresRepository) WorkerHealth(ctx context.Context, stale time.Duration) ([]WorkerHealth, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT names.name,COALESCE(h.last_seen::text,''),COALESCE(h.last_seen > now()-($1::double precision*interval '1 second'),false)
 FROM (VALUES ('notification-email'),('email-outbox')) names(name) LEFT JOIN ihp_platform_worker_heartbeats h ON h.worker_name=names.name ORDER BY names.name`, stale.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]WorkerHealth, 0, 2)
	for rows.Next() {
		var item WorkerHealth
		if err := rows.Scan(&item.Name, &item.LastSeen, &item.Healthy); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *PostgresRepository) Operations(ctx context.Context, app string) (ApplicationOperations, error) {
	result := ApplicationOperations{ApplicationID: app, StatusCounts24Hours: map[string]int{}}
	err := r.DB.QueryRowContext(ctx, `SELECT lifecycle_status,enabled FROM ihp_systems WHERE client_id=$1`, app).Scan(&result.LifecycleStatus, &result.Enabled)
	if err != nil {
		return result, err
	}
	err = r.DB.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE enabled) FROM ihp_application_identities WHERE application_client_id=$1`, app).Scan(&result.IdentityCount, &result.EnabledIdentities)
	if err != nil {
		return result, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT `+notificationOverallStatus+`,count(*) `+notificationStatusJoin+` WHERE n.application_client_id=$1 AND n.created_at>now()-interval '24 hours' GROUP BY 1`, app)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return result, err
		}
		result.StatusCounts24Hours[status] = count
		result.Requests24Hours += count
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	err = r.DB.QueryRowContext(ctx, `SELECT COALESCE(min(n.created_at)::text,'') `+notificationStatusJoin+` WHERE n.application_client_id=$1 AND (`+notificationOverallStatus+`) IN ('PENDING','PROCESSING','RETRY')`, app).Scan(&result.OldestPendingAt)
	return result, err
}
