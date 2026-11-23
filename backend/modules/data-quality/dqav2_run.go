package dataquality

// Orchestration: compile a table's enabled rules and execute them against
// the DWH, persisting the run + flags + measurements. This is the Go
// equivalent of the Python app's runner.py + engine/store.store_v2_run.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mohuganda/moh-sso-dashboard/backend/modules/data-quality/dqa"
)

// RunResult is what a triggered scan reports back to the caller.
type RunResult struct {
	RunID          int64            `json:"run_id"`
	TableID        string           `json:"table_id"`
	RulesEvaluated int              `json:"rules_evaluated"`
	TotalFlags     int              `json:"total_flags"`
	Fails          int              `json:"fails"`
	Warns          int              `json:"warns"`
	CompileErrors  []RuleCompileErr `json:"compile_errors,omitempty"`
}

// RuleCompileErr names a rule that could not even be compiled (so it never
// ran) — distinct from an engine-error flag, which is a rule that compiled
// but failed at execution time (e.g. a missing column at runtime).
type RuleCompileErr struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

// RunTable compiles and executes every enabled rule registered for tableID,
// against dwhDB, and persists the results. triggeredBy is an optional
// username/email for audit.
func (s *DQAStore) RunTable(ctx context.Context, dwhDB *sql.DB, tableID string, scope RunScope, triggeredBy string) (RunResult, error) {
	tables, err := s.TableMap(ctx)
	if err != nil {
		return RunResult{}, fmt.Errorf("loading table registry: %w", err)
	}
	if _, ok := tables[tableID]; !ok {
		return RunResult{}, fmt.Errorf("table_id %q is not registered (see /data-quality/dqa/tables)", tableID)
	}

	cfg, found, err := s.GetTable(ctx, tableID)
	if err != nil {
		return RunResult{}, fmt.Errorf("loading table config: %w", err)
	}

	scopePredicate := ""
	if !scope.IsEmpty() {
		if !found {
			return RunResult{}, fmt.Errorf("table_id %q is not configured, so a run cannot be scoped", tableID)
		}
		if err := validatePeriodColumn(ctx, dwhDB, cfg, scope); err != nil {
			return RunResult{}, err
		}
		if scopePredicate, err = scope.Predicate(cfg); err != nil {
			return RunResult{}, err
		}
	}

	records, err := s.ListRules(ctx, tableID)
	if err != nil {
		return RunResult{}, fmt.Errorf("loading rules: %w", err)
	}

	var compiled []*dqa.CompiledRule
	var compileErrs []RuleCompileErr
	for _, rec := range records {
		if !rec.Enabled {
			continue
		}
		rule := rec.Rule
		rule.RowFilter = combinePredicates(rule.RowFilter, scopePredicate)
		c, err := dqa.CompileRule(rule, tables, nil)
		if err != nil {
			compileErrs = append(compileErrs, RuleCompileErr{Code: rec.Code, Error: err.Error()})
			continue
		}
		compiled = append(compiled, c)
	}

	opts := dqa.ExecuteOptions{CollectSamples: true}
	if found {
		// Empty DimColumns keeps every identifying value on the row; a configured
		// list narrows it to the columns the table owner chose to keep.
		opts.DimColumns = cfg.DimColumns
		if cfg.PeriodColumn != "" {
			identity := make(map[string]string, len(dqa.DefaultIdentity))
			for k, v := range dqa.DefaultIdentity {
				identity[k] = v
			}
			identity["period"] = cfg.PeriodColumn
			opts.Identity = identity
		}
	}

	flags, measurements := dqa.Execute(ctx, dwhDB, compiled, opts)

	periodStart, periodEnd := scope.Range()
	runID, err := s.StoreRun(ctx, tableID, len(compiled), flags, measurements, triggeredBy, periodStart, periodEnd)
	if err != nil {
		return RunResult{}, fmt.Errorf("persisting run: %w", err)
	}

	res := RunResult{
		RunID: runID, TableID: tableID, RulesEvaluated: len(compiled),
		TotalFlags: len(flags), CompileErrors: compileErrs,
	}
	for _, f := range flags {
		if f.Severity == string(dqa.SeverityFail) {
			res.Fails++
		} else if f.Severity == string(dqa.SeverityWarn) {
			res.Warns++
		}
	}
	return res, nil
}

// CompilePreview compiles a rule WITHOUT executing or persisting it, so the
// rule-builder UI can show the generated SQL before saving.
func (s *DQAStore) CompilePreview(ctx context.Context, rule dqa.Rule) (*dqa.CompiledRule, error) {
	tables, err := s.TableMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading table registry: %w", err)
	}
	return dqa.CompileRule(rule, tables, nil)
}
