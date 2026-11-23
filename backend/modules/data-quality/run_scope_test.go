package dataquality

import (
	"strings"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/modules/data-quality/dqa"
)

func testTableConfig() TableMapping {
	return TableMapping{
		TableID:       "vht_monthly",
		PhysicalTable: "report.cht_form_097b_with_vhts",
		PeriodColumn:  "period_date",
		FilterColumns: []string{"district", "facility"},
	}
}

func TestRunScopePredicate(t *testing.T) {
	cfg := testTableConfig()

	t.Run("empty scope has no predicate", func(t *testing.T) {
		got, err := RunScope{}.Predicate(cfg)
		if err != nil || got != "" {
			t.Fatalf("got %q, %v", got, err)
		}
	})

	t.Run("months become half-open ranges", func(t *testing.T) {
		got, err := RunScope{Periods: []RunPeriod{{2026, 7}, {2026, 12}}}.Predicate(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			`"period_date" >= DATE '2026-07-01'`,
			`"period_date" < DATE '2026-08-01'`,
			`"period_date" < DATE '2027-01-01'`, // December must roll into the next year
			" OR ",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %q in %q", want, got)
			}
		}
	})

	t.Run("periods need a configured period column", func(t *testing.T) {
		bare := cfg
		bare.PeriodColumn = ""
		if _, err := (RunScope{Periods: []RunPeriod{{2026, 7}}}).Predicate(bare); err == nil {
			t.Fatal("expected an error when no period column is configured")
		}
	})

	t.Run("filter columns are restricted to the configured ones", func(t *testing.T) {
		_, err := RunScope{Filters: []RunFilter{{Column: "secret_column", Value: "x"}}}.Predicate(cfg)
		if err == nil {
			t.Fatal("expected an unconfigured column to be rejected")
		}
	})

	t.Run("out of range periods are rejected", func(t *testing.T) {
		for _, p := range []RunPeriod{{2026, 0}, {2026, 13}, {1800, 1}} {
			if _, err := (RunScope{Periods: []RunPeriod{p}}).Predicate(cfg); err == nil {
				t.Fatalf("expected %v to be rejected", p)
			}
		}
	})
}

// A filter value is attacker-controlled, so the rendered predicate must survive
// SafePredicate — the same validation any user-written row filter goes through.
func TestRunScopePredicateIsInjectionSafe(t *testing.T) {
	cfg := testTableConfig()

	values := []string{
		"Wakiso'; DROP TABLE dqa.dqa_rules; --",
		"' OR 1=1 --",
		"it's fine",
	}

	for _, value := range values {
		scope := RunScope{Filters: []RunFilter{{Column: "district", Value: value}}}
		pred, err := scope.Predicate(cfg)
		if err != nil {
			t.Fatalf("building predicate for %q: %v", value, err)
		}

		rendered, err := dqa.SafePredicate(pred)
		if err != nil {
			t.Fatalf("SafePredicate rejected %q: %v", pred, err)
		}

		// The payload is only safe if it stays inside the string literal, so
		// assert against the SQL with every literal removed rather than the raw
		// text — "DROP" inside a literal is just data.
		outside := stripStringLiterals(rendered)
		for _, banned := range []string{"DROP", "--", ";"} {
			if strings.Contains(strings.ToUpper(outside), banned) {
				t.Fatalf("%q escaped its literal: %q (outside literals: %q)", value, rendered, outside)
			}
		}

		// And the literal must still carry the original value intact.
		if got := literalValue(rendered); got != value {
			t.Fatalf("literal round-trip: got %q, want %q", got, value)
		}
	}
}

// stripStringLiterals removes '...' literals, honouring ” escapes.
func stripStringLiterals(sql string) string {
	var out strings.Builder
	for i := 0; i < len(sql); {
		if sql[i] != '\'' {
			out.WriteByte(sql[i])
			i++
			continue
		}
		i++ // opening quote
		for i < len(sql) {
			if sql[i] == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					i += 2
					continue
				}
				i++ // closing quote
				break
			}
			i++
		}
	}
	return out.String()
}

// literalValue returns the decoded contents of the first string literal.
func literalValue(sql string) string {
	start := strings.Index(sql, "'")
	if start < 0 {
		return ""
	}
	var out strings.Builder
	for i := start + 1; i < len(sql); i++ {
		if sql[i] == '\'' {
			if i+1 < len(sql) && sql[i+1] == '\'' {
				out.WriteByte('\'')
				i++
				continue
			}
			break
		}
		out.WriteByte(sql[i])
	}
	return out.String()
}

func TestCombinePredicates(t *testing.T) {
	tests := []struct{ rowFilter, scope, want string }{
		{"", "", ""},
		{`"a" = 1`, "", `"a" = 1`},
		{"", `"b" = 2`, `"b" = 2`},
		{`"a" = 1`, `"b" = 2`, `("a" = 1) AND ("b" = 2)`},
	}
	for _, tt := range tests {
		if got := combinePredicates(tt.rowFilter, tt.scope); got != tt.want {
			t.Fatalf("combine(%q,%q) = %q, want %q", tt.rowFilter, tt.scope, got, tt.want)
		}
	}
}
