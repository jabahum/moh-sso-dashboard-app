package dataquality

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/modules/data-quality/dqa"
)

// RunScope narrows a scan to chosen reporting periods and filter values. It is
// rendered into each rule's row filter, so it passes through the same
// SafePredicate validation as any user-written predicate.
type RunScope struct {
	Periods []RunPeriod `json:"periods"`
	Filters []RunFilter `json:"filters"`
}

type RunPeriod struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

type RunFilter struct {
	Column string `json:"column"`
	Value  string `json:"value"`
}

func (s RunScope) IsEmpty() bool { return len(s.Periods) == 0 && len(s.Filters) == 0 }

// Predicate renders the scope as a SQL predicate for cfg's table. Filter columns
// must be ones the table was configured to expose, so a caller cannot filter on
// an arbitrary column.
func (s RunScope) Predicate(cfg TableMapping) (string, error) {
	if s.IsEmpty() {
		return "", nil
	}

	var parts []string

	if len(s.Periods) > 0 {
		if strings.TrimSpace(cfg.PeriodColumn) == "" {
			return "", fmt.Errorf("table %q has no period column configured", cfg.TableID)
		}
		column := dqa.QuoteIdent(cfg.PeriodColumn)
		months := make([]string, 0, len(s.Periods))
		for _, p := range s.Periods {
			if p.Year < 1900 || p.Year > 9999 {
				return "", fmt.Errorf("year %d is out of range", p.Year)
			}
			if p.Month < 1 || p.Month > 12 {
				return "", fmt.Errorf("month %d is out of range", p.Month)
			}
			start := time.Date(p.Year, time.Month(p.Month), 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, 0)
			months = append(months, fmt.Sprintf("(%s >= DATE '%s' AND %s < DATE '%s')",
				column, start.Format("2006-01-02"), column, end.Format("2006-01-02")))
		}
		parts = append(parts, "("+strings.Join(months, " OR ")+")")
	}

	allowed := map[string]bool{}
	for _, c := range cfg.FilterColumns {
		allowed[c] = true
	}
	for _, f := range s.Filters {
		column := strings.TrimSpace(f.Column)
		if !allowed[column] {
			return "", fmt.Errorf("column %q is not a configured filter column for %q", column, cfg.TableID)
		}
		parts = append(parts, fmt.Sprintf("%s = %s", dqa.QuoteIdent(column), quoteLiteral(f.Value)))
	}

	return strings.Join(parts, " AND "), nil
}

// Range returns the span the chosen periods cover, for recording on the run.
func (s RunScope) Range() (string, string) {
	if len(s.Periods) == 0 {
		return "", ""
	}
	first, last := s.Periods[0], s.Periods[0]
	for _, p := range s.Periods {
		if p.Year < first.Year || (p.Year == first.Year && p.Month < first.Month) {
			first = p
		}
		if p.Year > last.Year || (p.Year == last.Year && p.Month > last.Month) {
			last = p
		}
	}
	start := time.Date(first.Year, time.Month(first.Month), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(last.Year, time.Month(last.Month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

// validatePeriodColumn fails early when periods are chosen but the configured
// period column cannot be compared to a date, which would otherwise surface as
// an opaque SQL error on every rule.
func validatePeriodColumn(ctx context.Context, dwhDB *sql.DB, cfg TableMapping, scope RunScope) error {
	if len(scope.Periods) == 0 {
		return nil
	}
	if strings.TrimSpace(cfg.PeriodColumn) == "" {
		return fmt.Errorf("table %q has no period column configured", cfg.TableID)
	}
	columns, err := ListPhysicalColumns(ctx, dwhDB, cfg.PhysicalTable)
	if err != nil {
		return fmt.Errorf("reading columns of %s: %w", cfg.PhysicalTable, err)
	}
	for _, c := range columns {
		if c.Name != cfg.PeriodColumn {
			continue
		}
		if strings.Contains(c.Type, "date") || strings.Contains(c.Type, "timestamp") {
			return nil
		}
		return fmt.Errorf("period column %q is %s; scoping by period needs a date or timestamp column",
			cfg.PeriodColumn, c.Type)
	}
	return fmt.Errorf("period column %q does not exist on %s", cfg.PeriodColumn, cfg.PhysicalTable)
}

// quoteLiteral renders a single-quoted SQL literal, doubling embedded quotes.
// SafePredicate re-validates the result before it reaches the database.
func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// combinePredicates ANDs a rule's own row filter with the run scope.
func combinePredicates(rowFilter, scope string) string {
	rowFilter = strings.TrimSpace(rowFilter)
	scope = strings.TrimSpace(scope)
	switch {
	case rowFilter == "":
		return scope
	case scope == "":
		return rowFilter
	default:
		return "(" + rowFilter + ") AND (" + scope + ")"
	}
}
