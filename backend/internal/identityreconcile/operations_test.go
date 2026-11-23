package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// operationsFixture runs Operations on the batch fixture. Jobs run when the
// test calls drain, so a test can observe a running job.
type operationsFixture struct {
	*batchFixture
	ops       *Operations
	pending   []func()
	targetErr error
}

func newOperationsFixture() *operationsFixture {
	f := &operationsFixture{batchFixture: newBatchFixture()}
	f.ops = &Operations{Batch: f.svc}
	f.ops.spawn = func(run func()) { f.pending = append(f.pending, run) }
	f.ops.target = func(context.Context, string) (BatchTarget, error) {
		if f.targetErr != nil {
			return BatchTarget{}, f.targetErr
		}
		return f.target, nil
	}
	return f
}

func (f *operationsFixture) drain() {
	for len(f.pending) > 0 {
		run := f.pending[0]
		f.pending = f.pending[1:]
		run()
	}
}

func (f *operationsFixture) onlyJob(t *testing.T) Job {
	t.Helper()
	jobs := f.ops.Jobs("alerts")
	if len(jobs) == 0 {
		t.Fatal("no job recorded")
	}
	return jobs[0]
}

func TestOperationsStartRunJob(t *testing.T) {
	f := newOperationsFixture()
	job, err := f.ops.StartRun(context.Background(), "alerts", "portal:admin@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != JobRunning || job.Kind != JobStartRun || job.Actor != "portal:admin@example.org" {
		t.Fatalf("started job = %+v", job)
	}
	// A second job for the same application is refused while one runs.
	if _, err := f.ops.StartRun(context.Background(), "alerts", "portal:admin@example.org"); !errors.Is(err, ErrJobRunning) {
		t.Fatalf("second start = %v, want ErrJobRunning", err)
	}
	f.drain()
	done := f.onlyJob(t)
	if done.Status != JobSucceeded || done.RunID == "" || done.FinishedAt == nil {
		t.Fatalf("finished job = %+v", done)
	}
	if done.Summary["total"] != 10 || done.Summary["proposal:reuse"] != 1 {
		t.Fatalf("summary = %v", done.Summary)
	}
	run, rows, err := f.ops.GetRun(context.Background(), "alerts", done.RunID)
	if err != nil || run.CreatedBy != "portal:admin@example.org" || len(rows) != 10 {
		t.Fatalf("GetRun = %+v, %d rows, %v", run, len(rows), err)
	}
	// Jobs returns copies: changing one does not change the record.
	done.Summary["total"] = 99
	if f.onlyJob(t).Summary["total"] != 10 {
		t.Fatal("Jobs leaked its internal summary map")
	}
}

func TestOperationsRefusesBeforeQueueingWhenTheIntegrationCannotBeUsed(t *testing.T) {
	f := newOperationsFixture()
	f.targetErr = fmt.Errorf("connector: %w", ErrIntegrationDisabled)
	if _, err := f.ops.StartRun(context.Background(), "alerts", "portal:admin"); !errors.Is(err, ErrIntegrationDisabled) {
		t.Fatalf("start = %v", err)
	}
	if len(f.ops.Jobs("alerts")) != 0 || len(f.pending) != 0 {
		t.Fatal("a refused start must not record or queue a job")
	}
	f.targetErr = nil
	if _, err := f.ops.StartRun(context.Background(), "alerts", " "); !errors.Is(err, ErrReviewerRequired) {
		t.Fatalf("blank actor = %v, want ErrReviewerRequired", err)
	}
	if len(f.ops.Jobs("alerts")) != 0 {
		t.Fatal("a refused start must not record a job")
	}
}

func TestOperationsRunsBelongToTheirApplication(t *testing.T) {
	f := newOperationsFixture()
	run, _ := f.start(t)
	if _, _, err := f.ops.GetRun(context.Background(), "other-app", run.ID); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("GetRun for another application = %v", err)
	}
	if _, err := f.ops.Decide(context.Background(), "other-app", run.ID, []string{"a-reuse"}, true, "r@example.org", ""); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("Decide for another application = %v", err)
	}
	if _, err := f.ops.ApplyRun(context.Background(), "other-app", run.ID, "portal:admin", false); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("ApplyRun for another application = %v", err)
	}
	if f.rows(t, run.ID)["a-reuse"].Decision != DecisionPending {
		t.Fatal("a refused decision changed the row")
	}
}

func TestOperationsApplyAndResumeJobs(t *testing.T) {
	f := newOperationsFixture()
	run, _ := f.start(t)
	if _, err := f.ops.Decide(context.Background(), "alerts", run.ID, []string{"a-reuse"}, true, "reviewer@example.org", "portal"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ops.Decide(context.Background(), "alerts", run.ID, []string{"c-ambiguous"}, true, "reviewer@example.org", ""); !errors.Is(err, ErrRowNotApprovable) {
		t.Fatalf("approving a review row = %v", err)
	}
	if _, err := f.ops.ApplyRun(context.Background(), "alerts", run.ID, "portal:admin", false); err != nil {
		t.Fatal(err)
	}
	f.drain()
	job := f.onlyJob(t)
	if job.Status != JobSucceeded || job.Kind != JobApplyRun || job.RunID != run.ID || job.Summary["outcome:done"] != 1 {
		t.Fatalf("apply job = %+v", job)
	}
	if row := f.rows(t, run.ID)["a-reuse"]; !row.Finished() {
		t.Fatalf("row = %+v", row)
	}
	if _, err := f.ops.ApplyRun(context.Background(), "alerts", run.ID, "portal:admin", true); err != nil {
		t.Fatal(err)
	}
	f.drain()
	if job := f.onlyJob(t); job.Kind != JobResumeRun || job.Status != JobSucceeded || job.Summary["rows"] != 0 {
		t.Fatalf("resume job = %+v", job)
	}
	if got := len(f.ops.Jobs("alerts")); got != 2 {
		t.Fatalf("jobs kept = %d, want 2", got)
	}
}

func TestOperationsFailedJobHidesUpstreamDetail(t *testing.T) {
	f := newOperationsFixture()
	if _, err := f.ops.StartRun(context.Background(), "alerts", "portal:admin"); err != nil {
		t.Fatal(err)
	}
	// The target fails inside the job with a detailed upstream error.
	f.targetErr = errors.New("GET https://clims.internal/api?token=s3cret: HTTP 500 body")
	f.drain()
	job := f.onlyJob(t)
	if job.Status != JobFailed || job.Error != "job_failed" {
		t.Fatalf("job = %+v", job)
	}
	if strings.Contains(job.ErrorMessage, "s3cret") || strings.Contains(job.ErrorMessage, "clims.internal") {
		t.Fatalf("error message leaks upstream detail: %q", job.ErrorMessage)
	}
	// A new job is accepted once the failed one has finished.
	f.targetErr = nil
	if _, err := f.ops.StartRun(context.Background(), "alerts", "portal:admin"); err != nil {
		t.Fatal(err)
	}
}

func TestOperationsJobPanicIsRecorded(t *testing.T) {
	f := newOperationsFixture()
	job, err := f.ops.begin("alerts", JobStartRun, "", "portal:admin")
	if err != nil {
		t.Fatal(err)
	}
	f.ops.launch(job, func(context.Context) (string, map[string]int, error) { panic("boom") })
	f.drain()
	if got := f.onlyJob(t); got.Status != JobFailed || got.Error != "job_failed" {
		t.Fatalf("job = %+v", got)
	}
}

func TestOperationsKeepsRecentJobsOnly(t *testing.T) {
	f := newOperationsFixture()
	for i := 0; i < jobsKept+3; i++ {
		if _, err := f.ops.StartRun(context.Background(), "alerts", "portal:admin"); err != nil {
			t.Fatal(err)
		}
		f.drain()
	}
	if got := len(f.ops.Jobs("alerts")); got != jobsKept {
		t.Fatalf("jobs kept = %d, want %d", got, jobsKept)
	}
}

func TestJobErrorCode(t *testing.T) {
	for err, want := range map[error]string{
		fmt.Errorf("wrapped: %w", ErrRunBusy):         "run_busy",
		fmt.Errorf("x: %w", ErrIntegrationDisabled):   "integration_disabled",
		fmt.Errorf("x: %w", context.DeadlineExceeded): "job_timeout",
		errors.New("anything else"):                   "job_failed",
	} {
		if code, msg := JobErrorCode(err); code != want || msg == "" {
			t.Errorf("JobErrorCode(%v) = %q, %q; want %q", err, code, msg, want)
		}
	}
}
