package dqa

import (
	"strings"
	"testing"
)

// ── thresholds ───────────────────────────────────────────────────────────

func TestEvaluatePredicate(t *testing.T) {
	cases := []struct {
		predicate string
		value     float64
		want      bool
	}{
		{"when > 10", 11, true},
		{"when > 10", 10, false},
		{"> 10", 11, true},
		{">= 5", 5, true},
		{"<= 5", 6, false},
		{"!= 0", 1, true},
		{"<> 0", 0, false},
		{"= 3", 3, true},
		{"when between 1 and 10", 5, true},
		{"when between 1 and 10", 11, false},
		{"when not between -10 and 10", 50, true},
		{"when not between -10 and 10", 0, false},
		{"between 10 and 1", 5, true}, // reversed bounds normalise
	}
	for _, c := range cases {
		got, err := EvaluatePredicate(c.value, c.predicate)
		if err != nil {
			t.Fatalf("EvaluatePredicate(%v, %q) error: %v", c.value, c.predicate, err)
		}
		if got != c.want {
			t.Errorf("EvaluatePredicate(%v, %q) = %v, want %v", c.value, c.predicate, got, c.want)
		}
	}
}

func TestEvaluatePredicateRejectsGarbage(t *testing.T) {
	if _, err := EvaluatePredicate(1, "wat"); err == nil {
		t.Fatal("expected an error for an unparseable predicate")
	}
}

func TestEvaluateZonesFailBeatsWarn(t *testing.T) {
	z := Zone{Warn: "> 5", Fail: "> 10"}
	if got, _ := EvaluateZones(20, z); got != "fail" {
		t.Errorf("value 20 = %q, want fail", got)
	}
	if got, _ := EvaluateZones(7, z); got != "warn" {
		t.Errorf("value 7 = %q, want warn", got)
	}
	if got, _ := EvaluateZones(1, z); got != "" {
		t.Errorf("value 1 = %q, want pass", got)
	}
}

// ── safety layer ─────────────────────────────────────────────────────────

func TestParseExpressionRejectsUnsafeInput(t *testing.T) {
	bad := []string{
		"1; DROP TABLE users",
		"col -- comment",
		"col /* comment */ > 1",
		"(SELECT 1) > 0",
		"col > (SELECT max(x) FROM other)",
		"pg_sleep(10) > 0",
		"pg_read_file('/etc/passwd') IS NOT NULL",
		"dblink('x','y') IS NOT NULL",
		"DELETE FROM t",
		"col > 1 UNION SELECT 1",
		"",
		"   ",
		"col > (1",
	}
	for _, in := range bad {
		if _, err := ParseExpression(in); err == nil {
			t.Errorf("ParseExpression(%q) accepted unsafe input", in)
		}
	}
}

func TestParseExpressionAcceptsValidExpressions(t *testing.T) {
	good := []string{
		"total_tested >= total_positive",
		"COUNT(*) > 0",
		"ABS(a - b) <= 10",
		"COALESCE(x, 0) > 0",
		"col IS NOT NULL",
		"a > 1 AND b < 2 OR NOT c = 3",
		"value BETWEEN 1 AND 10",
		"EXTRACT(YEAR FROM report_date) = 2024",
		"lower(name) LIKE 'abc%'",
		"a.col = b.col",
		"ROUND(value::numeric, 2) > 1",
	}
	for _, in := range good {
		if _, err := ParseExpression(in); err != nil {
			t.Errorf("ParseExpression(%q) rejected valid input: %v", in, err)
		}
	}
}

func TestRenderQuotesIdentifiers(t *testing.T) {
	got, err := SafePredicate("total_tested >= total_positive")
	if err != nil {
		t.Fatalf("SafePredicate error: %v", err)
	}
	if !strings.Contains(got, `"total_tested"`) || !strings.Contains(got, `"total_positive"`) {
		t.Errorf("identifiers not quoted: %s", got)
	}
}

func TestRenderKeepsKeywordsAndFunctionsBare(t *testing.T) {
	got, err := SafePredicate("COUNT(*) > 0 AND col IS NOT NULL")
	if err != nil {
		t.Fatalf("SafePredicate error: %v", err)
	}
	if strings.Contains(got, `"COUNT"`) || strings.Contains(got, `"AND"`) || strings.Contains(got, `"IS"`) {
		t.Errorf("keyword/function got quoted as an identifier: %s", got)
	}
	if !strings.Contains(got, `"col"`) {
		t.Errorf("column not quoted: %s", got)
	}
}

func TestQuoteTable(t *testing.T) {
	if got := QuoteTable("report.foo"); got != `"report"."foo"` {
		t.Errorf("QuoteTable(report.foo) = %s", got)
	}
	if got := QuoteTable("foo"); got != `"foo"` {
		t.Errorf("QuoteTable(foo) = %s", got)
	}
}

func TestQuoteIdentEscapes(t *testing.T) {
	if got := QuoteIdent(`we"ird`); got != `"we""ird"` {
		t.Errorf("QuoteIdent did not escape: %s", got)
	}
}

func TestBuildChain(t *testing.T) {
	got, err := BuildChain([]string{"x", "y", "z"}, ">")
	if err != nil {
		t.Fatalf("BuildChain error: %v", err)
	}
	if got != "x > y AND y > z" {
		t.Errorf("BuildChain = %q", got)
	}
	if _, err := BuildChain([]string{"x"}, ">"); err == nil {
		t.Error("expected error for a single operand")
	}
	if _, err := BuildChain([]string{"x", "y"}, "; DROP"); err == nil {
		t.Error("expected error for a bad operator")
	}
}

func TestValidateColumns(t *testing.T) {
	schema := map[string]string{"total_tested": "int", "total_positive": "int"}
	if err := ValidateColumns("total_tested >= total_positive", schema); err != nil {
		t.Errorf("valid columns rejected: %v", err)
	}
	if err := ValidateColumns("total_tested >= nope", schema); err == nil {
		t.Error("expected unknown column to be rejected")
	}
	// functions and keywords must not be treated as columns
	if err := ValidateColumns("COUNT(*) > 0 AND total_tested IS NOT NULL", schema); err != nil {
		t.Errorf("function/keyword misread as column: %v", err)
	}
}

// ── compiler ─────────────────────────────────────────────────────────────

func mustRule(t *testing.T, m map[string]any) Rule {
	t.Helper()
	r, err := RuleFromMap(m, "yaml", "")
	if err != nil {
		t.Fatalf("RuleFromMap: %v", err)
	}
	return r
}

func TestCompileMetric(t *testing.T) {
	r := mustRule(t, map[string]any{
		"code": "M1", "table": "vht_monthly", "type": "metric",
		"metric": "missing_percent", "column": "district", "fail": "> 10",
	})
	c, err := CompileRule(r, map[string]string{"vht_monthly": "report.vht_monthly"}, nil)
	if err != nil {
		t.Fatalf("CompileRule: %v", err)
	}
	if c.Kind != KindMetric {
		t.Errorf("kind = %s", c.Kind)
	}
	for _, want := range []string{`"report"."vht_monthly"`, `"district"`, "AS value", "NULLIF"} {
		if !strings.Contains(c.MeasureSQL, want) {
			t.Errorf("measure SQL missing %q: %s", want, c.MeasureSQL)
		}
	}
}

func TestCompileRowExpression(t *testing.T) {
	r := mustRule(t, map[string]any{
		"code": "R1", "table": "t", "type": "row_expression",
		"expr": "tested >= positive", "filter": "district IS NOT NULL",
	})
	c, err := CompileRule(r, nil, nil)
	if err != nil {
		t.Fatalf("CompileRule: %v", err)
	}
	if !strings.Contains(c.MeasureSQL, "COUNT(*) FILTER (WHERE NOT (") {
		t.Errorf("unexpected measure SQL: %s", c.MeasureSQL)
	}
	if !strings.Contains(c.MeasureSQL, "WHERE") || !strings.Contains(c.MeasureSQL, `"district"`) {
		t.Errorf("row filter not applied: %s", c.MeasureSQL)
	}
	if !strings.Contains(c.SampleSQL, "LIMIT 100") {
		t.Errorf("sample SQL missing limit: %s", c.SampleSQL)
	}
	// filter present => sample must AND the predicate, not start a second WHERE
	if strings.Count(c.SampleSQL, "WHERE") != 1 {
		t.Errorf("sample SQL has malformed WHERE: %s", c.SampleSQL)
	}
}

func TestCompileChainRule(t *testing.T) {
	r := mustRule(t, map[string]any{
		"code": "C1", "table": "t", "type": "row_expression",
		"chain": []any{"a", "b", "c"}, "op": ">=",
	})
	c, err := CompileRule(r, nil, nil)
	if err != nil {
		t.Fatalf("CompileRule: %v", err)
	}
	if !strings.Contains(c.MeasureSQL, `"a" >= "b"`) || !strings.Contains(c.MeasureSQL, `"b" >= "c"`) {
		t.Errorf("chain not expanded: %s", c.MeasureSQL)
	}
}

func TestCompileRowJoinAndReference(t *testing.T) {
	tables := map[string]string{"a": "report.a", "b": "report.b"}

	rj := mustRule(t, map[string]any{
		"code": "J1", "table": "a", "type": "row_join",
		"left": "a", "right": "b",
		"join_keys": []any{"vht_id"},
		"expr":      "a.total = b.total",
	})
	c, err := CompileRule(rj, tables, nil)
	if err != nil {
		t.Fatalf("row_join: %v", err)
	}
	if !strings.Contains(c.MeasureSQL, `JOIN "report"."b" AS b ON a."vht_id" = b."vht_id"`) {
		t.Errorf("bad join SQL: %s", c.MeasureSQL)
	}

	ref := mustRule(t, map[string]any{
		"code": "F1", "table": "a", "type": "reference",
		"left": "a", "right": "b",
		"join_keys": []any{map[string]any{"left": "vht_id", "right": "id"}},
	})
	c2, err := CompileRule(ref, tables, nil)
	if err != nil {
		t.Fatalf("reference: %v", err)
	}
	if !strings.Contains(c2.MeasureSQL, "LEFT JOIN") || !strings.Contains(c2.MeasureSQL, `b."id" IS NULL`) {
		t.Errorf("bad anti-join SQL: %s", c2.MeasureSQL)
	}
}

func TestCompileAggregateCompare(t *testing.T) {
	r := mustRule(t, map[string]any{
		"code": "A1", "table": "a", "type": "aggregate_compare",
		"left":      map[string]any{"table": "a", "agg": "SUM(total)"},
		"right":     map[string]any{"table": "b", "agg": "SUM(total)"},
		"tolerance": 0.5, "op": "=",
	})
	c, err := CompileRule(r, map[string]string{"a": "report.a", "b": "report.b"}, nil)
	if err != nil {
		t.Fatalf("CompileRule: %v", err)
	}
	if c.Kind != KindAggregateCompare {
		t.Errorf("kind = %s", c.Kind)
	}
	if !strings.Contains(c.MeasureSQL, "AS left_val") || !strings.Contains(c.MeasureSQL, "AS right_val") {
		t.Errorf("bad aggregate SQL: %s", c.MeasureSQL)
	}
	if c.Meta["tolerance"].(float64) != 0.5 {
		t.Errorf("tolerance not carried: %v", c.Meta["tolerance"])
	}
}

func TestCompileRawSQLGuards(t *testing.T) {
	mk := func(q string) Rule {
		return mustRule(t, map[string]any{
			"code": "X1", "table": "t", "type": "raw_sql", "fail_query": q,
		})
	}
	if _, err := CompileRule(mk("SELECT 1 WHERE false"), nil, nil); err != nil {
		t.Fatalf("valid raw_sql rejected: %v", err)
	}
	bad := []string{
		"DELETE FROM t",
		"SELECT 1; DROP TABLE t",
		"UPDATE t SET x = 1",
		"SELECT 1 -- sneaky",
		"",
	}
	for _, q := range bad {
		if _, err := CompileRule(mk(q), nil, nil); err == nil {
			t.Errorf("raw_sql accepted unsafe query: %q", q)
		}
	}
}

func TestCompileRejectsInjectionInRuleFields(t *testing.T) {
	r := mustRule(t, map[string]any{
		"code": "BAD", "table": "t", "type": "row_expression",
		"expr": "1=1); DROP TABLE users; --",
	})
	if _, err := CompileRule(r, nil, nil); err == nil {
		t.Fatal("compiler accepted an injection payload")
	}
}

func TestRuleFromMapDefaultsAndZones(t *testing.T) {
	r := mustRule(t, map[string]any{
		"code": "Z1", "table": "t",
		"zones": map[string]any{"warn": "> 1", "fail": "> 2"},
	})
	if r.RuleType != RuleTypeRowExpression {
		t.Errorf("default rule type = %s", r.RuleType)
	}
	if r.Category != "custom" || !r.Enabled {
		t.Errorf("defaults wrong: category=%s enabled=%v", r.Category, r.Enabled)
	}
	if r.Zones.Warn != "> 1" || r.Zones.Fail != "> 2" {
		t.Errorf("zones not parsed: %+v", r.Zones)
	}
	if r.Key() != "t:Z1" {
		t.Errorf("key = %s", r.Key())
	}
}

func TestRuleFromMapRequiresCodeAndTable(t *testing.T) {
	if _, err := RuleFromMap(map[string]any{"table": "t"}, "yaml", ""); err == nil {
		t.Error("expected error for missing code")
	}
	if _, err := RuleFromMap(map[string]any{"code": "c"}, "yaml", ""); err == nil {
		t.Error("expected error for missing table id")
	}
	if _, err := RuleFromMap(map[string]any{"code": "c", "table": "t", "type": "nope"}, "yaml", ""); err == nil {
		t.Error("expected error for unknown rule type")
	}
}
