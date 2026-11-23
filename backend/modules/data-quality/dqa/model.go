// Package dqa is a Go port of the eCHIS Data Quality Assessment engine.
//
// A rule is a declarative predicate or metric that compiles to SQL and is
// pushed down into the DWH Postgres. Severity follows Soda-style graduated
// warn/fail zones evaluated against a single measured value.
package dqa

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RuleType is one of the six compile shapes.
type RuleType string

const (
	// RuleTypeMetric measures an aggregate and compares it to threshold zones.
	RuleTypeMetric RuleType = "metric"
	// RuleTypeRowExpression counts rows failing a boolean predicate.
	RuleTypeRowExpression RuleType = "row_expression"
	// RuleTypeRowJoin evaluates a predicate across two row-aligned tables.
	RuleTypeRowJoin RuleType = "row_join"
	// RuleTypeAggregateCompare compares cross-table totals within a tolerance.
	RuleTypeAggregateCompare RuleType = "aggregate_compare"
	// RuleTypeReference checks referential integrity via an anti-join.
	RuleTypeReference RuleType = "reference"
	// RuleTypeRawSQL is the escape hatch: a SELECT whose rows are violations.
	RuleTypeRawSQL RuleType = "raw_sql"
)

// Valid reports whether t is a known rule type.
func (t RuleType) Valid() bool {
	switch t {
	case RuleTypeMetric, RuleTypeRowExpression, RuleTypeRowJoin,
		RuleTypeAggregateCompare, RuleTypeReference, RuleTypeRawSQL:
		return true
	}
	return false
}

// Severity escalates pass < warn < fail.
type Severity string

const (
	SeverityWarn Severity = "warn"
	SeverityFail Severity = "fail"
)

// Zone holds Soda-style graduated alert predicates evaluated against one
// measurement, e.g. "when > 10" or "when not between -10 and 10".
type Zone struct {
	Warn string `json:"warn,omitempty"`
	Fail string `json:"fail,omitempty"`
}

// Rule is a single declarative data-quality rule.
type Rule struct {
	Code      string   `json:"code"`
	TableID   string   `json:"table_id"`
	RuleType  RuleType `json:"type"`
	Category  string   `json:"category"`
	Name      string   `json:"name"`
	Identity  string   `json:"identity,omitempty"`
	Source    string   `json:"source"` // "yaml" | "db"
	Enabled   bool     `json:"enabled"`
	RowFilter string   `json:"row_filter,omitempty"`
	GroupBy   []string `json:"group_by,omitempty"`
	Zones     Zone     `json:"zones"`
	// Definition carries the type-specific payload: expr, chain, metric,
	// column, left, right, join_keys, tolerance, fail_query, …
	Definition map[string]any `json:"definition"`
}

// Key is the merge/identity key used when combining rule sources.
func (r Rule) Key() string { return r.TableID + ":" + r.Code }

// defString reads a string out of the definition payload.
func (r Rule) defString(key string) string {
	if r.Definition == nil {
		return ""
	}
	switch v := r.Definition[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// defInt reads an int out of the definition payload, falling back to def.
func (r Rule) defInt(key string, def int) int {
	if r.Definition == nil {
		return def
	}
	switch v := r.Definition[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return def
}

// defFloat reads a float out of the definition payload, falling back to def.
func (r Rule) defFloat(key string, def float64) float64 {
	if r.Definition == nil {
		return def
	}
	switch v := r.Definition[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	}
	return def
}

// RuleFromMap builds a Rule from a decoded YAML/JSON mapping or a DB row.
//
// Zones may be given as a nested {warn, fail} mapping or as convenience keys
// warn:/fail: at the top level. Keys that are not rule attributes are folded
// into Definition, so a metric rule can say expr:/metric:/column: inline.
func RuleFromMap(data map[string]any, source string, tableID string) (Rule, error) {
	src := make(map[string]any, len(data))
	for k, v := range data {
		src[k] = v
	}

	pop := func(keys ...string) any {
		for _, k := range keys {
			if v, ok := src[k]; ok {
				delete(src, k)
				if v != nil {
					return v
				}
			}
		}
		return nil
	}
	popStr := func(keys ...string) string {
		if v := pop(keys...); v != nil {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	code := popStr("code")
	if strings.TrimSpace(code) == "" {
		return Rule{}, fmt.Errorf("rule is missing a code")
	}

	tid := tableID
	if tid == "" {
		tid = popStr("table_id", "table")
	} else {
		pop("table_id", "table")
	}
	if strings.TrimSpace(tid) == "" {
		return Rule{}, fmt.Errorf("rule %q is missing a table id", code)
	}

	rtype := popStr("type", "rule_type")
	if rtype == "" {
		rtype = string(RuleTypeRowExpression)
	}
	if !RuleType(rtype).Valid() {
		return Rule{}, fmt.Errorf("rule %q: unsupported rule_type %q", code, rtype)
	}

	var zones Zone
	if z := pop("zones"); z != nil {
		if m, ok := z.(map[string]any); ok {
			if s, ok := m["warn"].(string); ok {
				zones.Warn = s
			}
			if s, ok := m["fail"].(string); ok {
				zones.Fail = s
			}
		}
		pop("warn", "fail")
	} else {
		zones.Warn = popStr("warn")
		zones.Fail = popStr("fail")
	}

	category := popStr("category")
	if category == "" {
		category = "custom"
	}
	name := popStr("name")
	identity := popStr("identity")
	rowFilter := popStr("row_filter", "filter")

	enabled := true
	if v := pop("enabled"); v != nil {
		if b, ok := v.(bool); ok {
			enabled = b
		}
	}

	var groupBy []string
	if v := pop("group_by"); v != nil {
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				groupBy = append(groupBy, fmt.Sprintf("%v", item))
			}
		}
	}

	// An explicit definition: block wins; otherwise the leftover keys are it.
	definition := map[string]any{}
	if v := pop("definition"); v != nil {
		if m, ok := v.(map[string]any); ok {
			definition = m
		}
	} else {
		definition = src
	}

	if source == "" {
		source = "yaml"
	}

	return Rule{
		Code:       code,
		TableID:    tid,
		RuleType:   RuleType(rtype),
		Category:   category,
		Name:       name,
		Identity:   identity,
		Source:     source,
		Enabled:    enabled,
		RowFilter:  rowFilter,
		GroupBy:    groupBy,
		Zones:      zones,
		Definition: definition,
	}, nil
}

// RuleFlag is one violation produced by a rule (maps to report.dqa_flags).
type RuleFlag struct {
	RuleCode   string `json:"rule_code"`
	TableID    string `json:"table_id"`
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Detail     string `json:"detail"`
	EntityID   string `json:"entity_id,omitempty"`
	PeriodDate string `json:"period_date,omitempty"`
	District   string `json:"district,omitempty"`
	EntityName string `json:"entity_name,omitempty"`
	RowID      string `json:"row_id,omitempty"`

	// Promoted filter dimensions: first-class columns rather than dims entries
	// because the dashboard filters and groups by them.
	Year      *int   `json:"year,omitempty"`
	Month     *int   `json:"month,omitempty"`
	Region    string `json:"region,omitempty"`
	Subcounty string `json:"subcounty,omitempty"`
	Village   string `json:"village,omitempty"`
	Facility  string `json:"facility,omitempty"`
	VHT       string `json:"vht,omitempty"`

	// Dims captures every remaining identifying dimension of the flagged row, so
	// a violation stays traceable on datasets we have no dedicated column for.
	Dims map[string]any `json:"dims,omitempty"`
}

// EngineResult is the outcome of a single scan of one table.
type EngineResult struct {
	TableID        string         `json:"table_id"`
	RulesEvaluated int            `json:"rules_evaluated"`
	Flags          []RuleFlag     `json:"flags"`
	Measurements   map[string]any `json:"measurements,omitempty"`
	Notes          []string       `json:"notes,omitempty"`
}

// TotalFlags is the number of violations recorded.
func (e EngineResult) TotalFlags() int { return len(e.Flags) }

// Fails counts violations at fail severity.
func (e EngineResult) Fails() int { return e.countSeverity(string(SeverityFail)) }

// Warns counts violations at warn severity.
func (e EngineResult) Warns() int { return e.countSeverity(string(SeverityWarn)) }

func (e EngineResult) countSeverity(s string) int {
	n := 0
	for _, f := range e.Flags {
		if f.Severity == s {
			n++
		}
	}
	return n
}
