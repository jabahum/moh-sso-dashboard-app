package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var _ BatchRepository = (*MemoryBatchRepository)(nil)

// MemoryBatchRepository is an in-memory BatchRepository for tests and tools.
// It mirrors PostgresBatchRepository, including migration 63's row
// constraints, so state-machine bugs fail unit tests too.
type MemoryBatchRepository struct {
	mu   sync.Mutex
	runs map[string]BatchRun
	rows map[string]map[string]BatchRow
}

// NewMemoryBatchRepository returns an empty repository.
func NewMemoryBatchRepository() *MemoryBatchRepository {
	return &MemoryBatchRepository{runs: map[string]BatchRun{}, rows: map[string]map[string]BatchRow{}}
}

func checkRowConstraints(r BatchRow) error {
	if r.Decision != DecisionPending && (strings.TrimSpace(r.ReviewedBy) == "" || r.ReviewedAt == nil) {
		return errors.New("violates ihp_identity_sync_rows_reviewer")
	}
	if (r.Proposal == ProposalLinkedNoAccess || r.Proposal == ProposalRevoked) && r.GrantStatus == GrantGranted {
		return errors.New("violates ihp_identity_sync_rows_no_grant")
	}
	if r.LastError != "" && strings.ContainsAny(r.LastError, " :{}\"") {
		return fmt.Errorf("last_error %q is not a stable code", r.LastError)
	}
	return nil
}

func (m *MemoryBatchRepository) CreateRun(_ context.Context, run BatchRun, rows []BatchRow) (BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run.ID = uuid.NewString()
	run.CreatedAt, run.UpdatedAt = time.Now(), time.Now()
	m.runs[run.ID] = run
	m.rows[run.ID] = map[string]BatchRow{}
	for _, r := range rows {
		if _, dup := m.rows[run.ID][r.AccountID]; dup {
			return BatchRun{}, errors.New("duplicate account")
		}
		if err := checkRowConstraints(r); err != nil {
			return BatchRun{}, err
		}
		r.ID, r.RunID = uuid.NewString(), run.ID
		m.rows[run.ID][r.AccountID] = r
	}
	return run, nil
}

func (m *MemoryBatchRepository) GetRun(_ context.Context, id string) (BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return BatchRun{}, ErrRunNotFound
	}
	return run, nil
}

func (m *MemoryBatchRepository) ListRuns(_ context.Context, app string, limit int) ([]BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []BatchRun
	for _, run := range m.runs {
		if run.ApplicationClientID == app {
			out = append(out, run)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryBatchRepository) ListRows(_ context.Context, id string) ([]BatchRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []BatchRow
	for _, r := range m.rows[id] {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out, nil
}

func (m *MemoryBatchRepository) TransitionRow(_ context.Context, runID, accountID string, mutate func(BatchRun, *BatchRow) error) (BatchRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[runID]
	if !ok {
		return BatchRow{}, ErrRunNotFound
	}
	row, ok := m.rows[runID][accountID]
	if !ok {
		return BatchRow{}, ErrRowNotFound
	}
	next := row
	if err := mutate(run, &next); err != nil {
		return BatchRow{}, err
	}
	next.ID, next.RunID, next.AccountID, next.EmailHint = row.ID, row.RunID, row.AccountID, row.EmailHint
	if err := checkRowConstraints(next); err != nil {
		return BatchRow{}, err
	}
	next.UpdatedAt = time.Now()
	m.rows[runID][accountID] = next
	run.UpdatedAt = time.Now()
	m.runs[runID] = run
	return next, nil
}

func (m *MemoryBatchRepository) ClaimRun(_ context.Context, runID string, from []RunStatus, staleAfter time.Duration) (BatchRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[runID]
	if !ok {
		return BatchRun{}, ErrRunNotFound
	}
	before := run
	switch {
	case run.Status == RunCancelled:
		return BatchRun{}, ErrRunCancelled
	case run.Status == RunApplying && time.Since(run.UpdatedAt) < staleAfter:
		return BatchRun{}, ErrRunBusy
	}
	allowed := run.Status == RunApplying
	for _, s := range from {
		allowed = allowed || s == run.Status
	}
	if !allowed {
		return BatchRun{}, fmt.Errorf("run is %s", run.Status)
	}
	run.Status, run.UpdatedAt = RunApplying, time.Now()
	m.runs[runID] = run
	return before, nil
}

func (m *MemoryBatchRepository) FinishRun(_ context.Context, runID string, status RunStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.runs[runID]
	if run.Status != RunApplying {
		return errors.New("not applying")
	}
	run.Status = status
	m.runs[runID] = run
	return nil
}
