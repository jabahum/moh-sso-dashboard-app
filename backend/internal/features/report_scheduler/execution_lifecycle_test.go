package report_scheduler

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

type orphanExecutionRepository struct {
	Repository
	status, message string
}

func (r *orphanExecutionRepository) ClaimGeneratingExecutions(context.Context, int) ([]Execution, error) {
	return []Execution{{ID: "orphan", HealthBIJobID: "existing-job"}}, nil
}
func (r *orphanExecutionRepository) UpdateExecutionJob(_ context.Context, _, _, status, message string) error {
	r.status, r.message = status, message
	return nil
}
func TestPollDetachedExecutionCancels(t *testing.T) {
	repo := &orphanExecutionRepository{}
	service := NewService(repo, nil, nil, nil, nil, nil)
	if err := service.PollGenerating(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if repo.status != "cancelled" || repo.message != "Report schedule was deleted" {
		t.Fatalf("status=%s message=%s", repo.status, repo.message)
	}
}

func TestDeleteSchedulePostgresLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_REPORT_SCHEDULER_DB_URL")
	if dsn == "" {
		t.Skip("set TEST_REPORT_SCHEDULER_DB_URL to a disposable database")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE report_schedules(id uuid PRIMARY KEY,created_by text);
 CREATE TEMP TABLE report_executions(id uuid PRIMARY KEY,schedule_id uuid REFERENCES report_schedules(id) ON DELETE SET NULL,status text,error_message text,finished_at timestamptz,next_retry_at timestamptz,poll_claimed_at timestamptz,health_bi_job_id text);
 CREATE TEMP TABLE report_deliveries(execution_id uuid,status text,last_error text,next_retry_at timestamptz,updated_at timestamptz);
 INSERT INTO report_schedules VALUES ('00000000-0000-0000-0000-000000000001','owner');
 INSERT INTO report_executions(id,schedule_id,status) VALUES
 ('00000000-0000-0000-0000-000000000002','00000000-0000-0000-0000-000000000001','generating'),
 ('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000001','completed');
 INSERT INTO report_deliveries(execution_id,status) VALUES ('00000000-0000-0000-0000-000000000002','pending'),('00000000-0000-0000-0000-000000000003','sent');`)
	if err != nil {
		t.Fatal(err)
	}
	repo := &postgresRepository{db: db}
	schedule := "00000000-0000-0000-0000-000000000001"
	execution := "00000000-0000-0000-0000-000000000002"
	if err := repo.DeleteSchedule(ctx, schedule, "wrong-owner", false); err != ErrScheduleNotFound {
		t.Fatalf("unauthorized delete: %v", err)
	}
	if err := repo.DeleteSchedule(ctx, schedule, "owner", false); err != nil {
		t.Fatal(err)
	}
	// A poll result or retry already in flight must not revive cancelled work.
	if err := repo.UpdateExecutionJob(ctx, execution, "job", "generating", ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.ScheduleExecutionRetry(ctx, execution, "failure", time.Now()); err != nil {
		t.Fatal(err)
	}
	var cancelled, completed, detached, failedDelivery, sentDelivery int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status='cancelled'),count(*) FILTER (WHERE status='completed'),count(*) FILTER (WHERE schedule_id IS NULL) FROM report_executions`).Scan(&cancelled, &completed, &detached); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status='failed'),count(*) FILTER (WHERE status='sent') FROM report_deliveries`).Scan(&failedDelivery, &sentDelivery); err != nil {
		t.Fatal(err)
	}
	if cancelled != 1 || completed != 1 || detached != 2 || failedDelivery != 1 || sentDelivery != 1 {
		t.Fatalf("cancelled=%d completed=%d detached=%d failedDelivery=%d sentDelivery=%d", cancelled, completed, detached, failedDelivery, sentDelivery)
	}
}
