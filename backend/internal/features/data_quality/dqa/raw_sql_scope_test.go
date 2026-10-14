package dqa

import (
	"strings"
	"testing"
)

func rawRule(rowFilter string) Rule {
	return Rule{
		Code:      "RAW-01",
		TableID:   "t",
		RuleType:  RuleTypeRawSQL,
		RowFilter: rowFilter,
		Definition: map[string]any{
			"fail_query": "SELECT vht_uuid, district FROM report.t WHERE total < 0",
		},
	}
}

// A scoped run must narrow raw_sql rules too. Previously the filter was dropped,
// so a run scoped to one district silently counted violations nationwide.
func TestRawSQLAppliesRowFilter(t *testing.T) {
	compiled, err := CompileRule(rawRule(`district = 'Wakiso District'`), map[string]string{"t": "report.t"}, nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for name, sql := range map[string]string{
		"measure": compiled.MeasureSQL,
		"sample":  compiled.SampleSQL,
	} {
		if !strings.Contains(sql, "_dqa_sub WHERE") {
			t.Fatalf("%s SQL does not filter the wrapped query: %s", name, sql)
		}
		if !strings.Contains(sql, "Wakiso District") {
			t.Fatalf("%s SQL lost the filter value: %s", name, sql)
		}
	}

	// The filter must sit outside the rule's own SQL, never spliced into it: the
	// rule's own WHERE has to come first, the scope's after the closing wrapper.
	inner := strings.Index(compiled.MeasureSQL, "WHERE total < 0")
	outer := strings.Index(compiled.MeasureSQL, "_dqa_sub WHERE")
	if inner < 0 || outer < 0 {
		t.Fatalf("expected both the rule's own filter and the scope filter: %s", compiled.MeasureSQL)
	}
	if outer < inner {
		t.Fatalf("scope filter was spliced inside the rule's SQL: %s", compiled.MeasureSQL)
	}
}

func TestRawSQLWithoutRowFilterIsUnchanged(t *testing.T) {
	compiled, err := CompileRule(rawRule(""), map[string]string{"t": "report.t"}, nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if strings.Contains(compiled.MeasureSQL, "_dqa_sub WHERE") {
		t.Fatalf("unscoped rule gained a filter: %s", compiled.MeasureSQL)
	}
	if !strings.HasSuffix(compiled.MeasureSQL, "AS _dqa_sub") {
		t.Fatalf("unexpected measure SQL: %s", compiled.MeasureSQL)
	}
}

// An unsafe filter must be rejected rather than reaching the database.
func TestRawSQLRejectsUnsafeRowFilter(t *testing.T) {
	_, err := CompileRule(rawRule("district = 'x'; DROP TABLE dqa.dqa_rules"), map[string]string{"t": "report.t"}, nil)
	if err == nil {
		t.Fatal("expected an unsafe row filter to be rejected")
	}
}
