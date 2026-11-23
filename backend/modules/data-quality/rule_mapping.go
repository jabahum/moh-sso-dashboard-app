package dataquality

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/modules/data-quality/dqa"
)

// Rule mapping: take rules that already exist on other tables and work out which
// of them fit a target table's column structure, so they can be applied to it.

// RuleMappingCandidate is one existing rule checked against a target table.
type RuleMappingCandidate struct {
	Code           string `json:"code"`
	SourceTableID  string `json:"source_table_id"`
	Type           string `json:"type"`
	Category       string `json:"category,omitempty"`
	Name           string `json:"name,omitempty"`
	Mappable       bool   `json:"mappable"`
	Reason         string `json:"reason,omitempty"`
	AlreadyOnTable bool   `json:"already_on_table"`
}

// PreviewRuleMapping compiles every rule from other tables against targetTableID
// and has Postgres plan it. Planning resolves column names without running the
// query, so an unmappable rule reports exactly which column is missing.
func (s *DQAStore) PreviewRuleMapping(ctx context.Context, dwhDB *sql.DB, targetTableID string) ([]RuleMappingCandidate, error) {
	tables, err := s.TableMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading table registry: %w", err)
	}
	if _, ok := tables[targetTableID]; !ok {
		return nil, fmt.Errorf("table_id %q is not registered", targetTableID)
	}

	all, err := s.ListRules(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("loading rules: %w", err)
	}

	onTarget := map[string]bool{}
	for _, rec := range all {
		if rec.TableID == targetTableID {
			onTarget[rec.Code] = true
		}
	}

	seen := map[string]bool{}
	out := []RuleMappingCandidate{}
	for _, rec := range all {
		if rec.TableID == targetTableID || seen[rec.Code] {
			continue
		}
		seen[rec.Code] = true

		candidate := RuleMappingCandidate{
			Code:           rec.Code,
			SourceTableID:  rec.TableID,
			Type:           string(rec.RuleType),
			Category:       rec.Category,
			Name:           rec.Name,
			AlreadyOnTable: onTarget[rec.Code],
		}

		retargeted := rec.Rule
		retargeted.TableID = targetTableID
		compiled, err := dqa.CompileRule(retargeted, tables, nil)
		if err != nil {
			candidate.Reason = err.Error()
			out = append(out, candidate)
			continue
		}
		if err := explain(ctx, dwhDB, compiled.MeasureSQL); err != nil {
			candidate.Reason = explainReason(err)
			out = append(out, candidate)
			continue
		}
		candidate.Mappable = true
		out = append(out, candidate)
	}
	return out, nil
}

func explain(ctx context.Context, db *sql.DB, query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("rule compiled to an empty query")
	}
	rows, err := db.QueryContext(ctx, "EXPLAIN "+query)
	if err != nil {
		return err
	}
	return rows.Close()
}

// explainReason trims Postgres' error to the part a reviewer needs.
func explainReason(err error) string {
	msg := err.Error()
	if idx := strings.Index(msg, "pq: "); idx >= 0 {
		msg = msg[idx+len("pq: "):]
	}
	return msg
}

// RuleMappingResult reports what an apply did.
type RuleMappingResult struct {
	TableID string           `json:"table_id"`
	Applied []string         `json:"applied"`
	Failed  []RuleCompileErr `json:"failed"`
}

// ApplyRuleMapping copies the chosen rules onto the target table.
func (s *DQAStore) ApplyRuleMapping(ctx context.Context, targetTableID string, codes []string, appliedBy string) (RuleMappingResult, error) {
	result := RuleMappingResult{TableID: targetTableID, Applied: []string{}, Failed: []RuleCompileErr{}}
	if len(codes) == 0 {
		return result, fmt.Errorf("no rules selected")
	}

	all, err := s.ListRules(ctx, "")
	if err != nil {
		return result, fmt.Errorf("loading rules: %w", err)
	}

	wanted := map[string]bool{}
	for _, code := range codes {
		wanted[code] = true
	}

	source := map[string]dqa.Rule{}
	for _, rec := range all {
		if rec.TableID == targetTableID || !wanted[rec.Code] {
			continue
		}
		if _, taken := source[rec.Code]; !taken {
			source[rec.Code] = rec.Rule
		}
	}

	for _, code := range codes {
		rule, ok := source[code]
		if !ok {
			result.Failed = append(result.Failed, RuleCompileErr{Code: code, Error: "rule not found on any other table"})
			continue
		}
		rule.TableID = targetTableID
		compiled, err := s.CompilePreview(ctx, rule)
		if err != nil {
			result.Failed = append(result.Failed, RuleCompileErr{Code: code, Error: err.Error()})
			continue
		}
		if _, err := s.UpsertRule(ctx, rule, compiled.MeasureSQL, appliedBy); err != nil {
			result.Failed = append(result.Failed, RuleCompileErr{Code: code, Error: err.Error()})
			continue
		}
		result.Applied = append(result.Applied, code)
	}
	return result, nil
}
