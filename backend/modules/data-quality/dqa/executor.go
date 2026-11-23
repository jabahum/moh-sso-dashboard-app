package dqa

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Pushdown executor.
//
// Runs each CompiledRule's SQL inside Postgres, evaluates the measured value
// against the rule's warn/fail zones, and turns violations into RuleFlag
// values (with failed-row samples). A rule that errors (missing column,
// timeout, ...) never aborts the scan — it rolls back and becomes an
// "engine-error" flag so the caller can see exactly which rule broke.

// DefaultIdentity are the columns used to stamp identity onto a sampled
// failing row, when the caller does not supply its own mapping.
var DefaultIdentity = map[string]string{
	"entity": "vht_uuid", "period": "period_date",
	"district": "district", "name": "chw_name", "id": "id",
}

// dimAliases maps each promoted flag dimension to the source column names that
// carry it. The eCHIS datasets disagree on spelling: the facility-level 097b
// matview uses "Health Facility"/"subcounty", the VHT-level form uses
// "facility"/"sub_county", and only the latter identifies a VHT at all.
var dimAliases = map[string][]string{
	"year":      {"year"},
	"month":     {"month"},
	"region":    {"region"},
	"district":  {"district", "district_norm"},
	"subcounty": {"sub_county", "subcounty"},
	"village":   {"village"},
	"facility":  {"health facility", "facility", "facility_name", "dhis2_facility_id"},
	"vht":       {"vht_uuid", "vht", "vht_name", "chw_name", "username"},
}

// lowerIndex maps lower-cased column names back to their actual key in rec, so
// aliases resolve regardless of how the source spells the case.
func lowerIndex(rec map[string]any) map[string]string {
	idx := make(map[string]string, len(rec))
	for k := range rec {
		idx[strings.ToLower(strings.TrimSpace(k))] = k
	}
	return idx
}

// resolveDim returns the first aliased, non-empty value present in rec.
func resolveDim(rec map[string]any, idx map[string]string, field string) string {
	for _, alias := range dimAliases[field] {
		key, ok := idx[alias]
		if !ok {
			continue
		}
		if v := rec[key]; v != nil {
			if s := strings.TrimSpace(fmt.Sprintf("%v", v)); s != "" {
				return s
			}
		}
	}
	return ""
}

func resolveDimInt(rec map[string]any, idx map[string]string, field string) *int {
	s := resolveDim(rec, idx, field)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

// isDimensionValue reports whether a sampled value identifies a row rather than
// measuring it. Measures are the numbers rules are written against; keeping them
// out of dims stops every indicator column landing in the JSONB blob.
func isDimensionValue(v any) bool {
	switch v.(type) {
	case nil, int, int32, int64, float32, float64:
		return false
	}
	return true
}

// Querier is the subset of *sql.DB / *sql.Tx / *sql.Conn the executor needs.
// Accepting an interface (rather than *sql.DB directly) keeps this testable
// against any driver, real or fake.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ExecuteOptions configures a run.
type ExecuteOptions struct {
	CollectSamples bool
	Identity       map[string]string // entity/period/district/name/id -> column name
	DimColumns     []string          // extra identifying dimensions to capture
}

// aggregateHolds reports whether the aggregate comparison HOLDS (no
// violation) — mirrors the Python `_cmp`.
func aggregateHolds(left, right, op string, tolerance float64) (bool, error) {
	l := parseNumeric(left)
	r := parseNumeric(right)
	d := l - r
	switch op {
	case "=":
		return absF(d) <= tolerance, nil
	case "!=", "<>":
		return absF(d) > tolerance, nil
	case ">":
		return d > tolerance, nil
	case "<":
		return d < -tolerance, nil
	case ">=":
		return d >= -tolerance, nil
	case "<=":
		return d <= tolerance, nil
	}
	return false, fmt.Errorf("unknown aggregate op: %q", op)
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func parseNumeric(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%g", &f)
	return f
}

func flagFromSample(rule Rule, state string, rec map[string]any, identity map[string]string, dimColumns []string) RuleFlag {
	idn := identity
	if idn == nil {
		idn = DefaultIdentity
	}
	get := func(col string) string {
		if col == "" {
			return ""
		}
		v, ok := rec[col]
		if !ok || v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
	idx := lowerIndex(rec)
	dims := map[string]any{}
	if len(dimColumns) > 0 {
		for _, c := range dimColumns {
			if v, ok := rec[c]; ok {
				dims[c] = v
			}
		}
	} else {
		// No explicit list: keep every identifying value on the row so a
		// violation can be traced on datasets we have no dedicated column for.
		for k, v := range rec {
			if isDimensionValue(v) {
				dims[k] = v
			}
		}
	}
	detail := rule.Name
	if strings.TrimSpace(detail) == "" {
		detail = fmt.Sprintf("%s violated [%s]", rule.Code, state)
	}
	return RuleFlag{
		RuleCode:   rule.Code,
		TableID:    rule.TableID,
		Severity:   state,
		Category:   rule.Category,
		Detail:     detail,
		EntityID:   get(idn["entity"]),
		PeriodDate: get(idn["period"]),
		District:   firstNonEmpty(get(idn["district"]), resolveDim(rec, idx, "district")),
		EntityName: get(idn["name"]),
		RowID:      get(idn["id"]),

		Year:      resolveDimInt(rec, idx, "year"),
		Month:     resolveDimInt(rec, idx, "month"),
		Region:    resolveDim(rec, idx, "region"),
		Subcounty: resolveDim(rec, idx, "subcounty"),
		Village:   resolveDim(rec, idx, "village"),
		Facility:  resolveDim(rec, idx, "facility"),
		VHT:       resolveDim(rec, idx, "vht"),

		Dims: dims,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func scanRow(rows *sql.Rows) (map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	rec := make(map[string]any, len(cols))
	for i, c := range cols {
		v := vals[i]
		if b, ok := v.([]byte); ok {
			v = string(b)
		}
		rec[c] = v
	}
	return rec, nil
}

// Execute runs compiled rules and returns (flags, measurements).
// measurements maps rule code -> measured value, for persistence to
// dqa.dqa_measurements (history / anomaly baselines).
func Execute(ctx context.Context, q Querier, compiled []*CompiledRule, opts ExecuteOptions) ([]RuleFlag, map[string]any) {
	var flags []RuleFlag
	measurements := make(map[string]any, len(compiled))

	for _, c := range compiled {
		rule := c.Rule
		if err := executeOne(ctx, q, c, opts, &flags, measurements); err != nil {
			// A rule that references missing columns (or any SQL error) must
			// not abort the whole scan — record it as an engine-error flag.
			measurements[rule.Code] = nil
			flags = append(flags, RuleFlag{
				RuleCode: rule.Code, TableID: rule.TableID, Severity: string(SeverityFail),
				Category: "engine-error",
				Detail:   fmt.Sprintf("Rule could not run: %s", firstLine(err.Error(), 300)),
			})
		}
	}
	return flags, measurements
}

func firstLine(s string, max int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > max {
		s = s[:max]
	}
	return s
}

func executeOne(ctx context.Context, q Querier, c *CompiledRule, opts ExecuteOptions,
	flags *[]RuleFlag, measurements map[string]any) error {
	rule := c.Rule

	switch c.Kind {
	case KindMetric:
		rows, err := q.QueryContext(ctx, c.MeasureSQL)
		if err != nil {
			return err
		}
		var value float64
		if rows.Next() {
			var raw sql.NullFloat64
			if err := rows.Scan(&raw); err != nil {
				rows.Close()
				return err
			}
			if raw.Valid {
				value = raw.Float64
			}
		}
		rows.Close()
		measurements[rule.Code] = value
		state, err := EvaluateZones(value, rule.Zones)
		if err != nil {
			return err
		}
		if state != "" {
			metric, _ := c.Meta["metric"].(string)
			if metric == "" {
				metric = "metric"
			}
			*flags = append(*flags, RuleFlag{
				RuleCode: rule.Code, TableID: rule.TableID, Severity: state,
				Category: rule.Category,
				Detail:   fmt.Sprintf("%s = %g [%s]", metric, value, state),
			})
		}
		return nil

	case KindAggregateCompare:
		rows, err := q.QueryContext(ctx, c.MeasureSQL)
		if err != nil {
			return err
		}
		var left, right sql.NullString
		if rows.Next() {
			if err := rows.Scan(&left, &right); err != nil {
				rows.Close()
				return err
			}
		}
		rows.Close()
		measurements[rule.Code] = map[string]any{"left": left.String, "right": right.String}
		op, _ := c.Meta["op"].(string)
		tol, _ := c.Meta["tolerance"].(float64)
		holds, err := aggregateHolds(left.String, right.String, op, tol)
		if err != nil {
			return err
		}
		if !holds {
			*flags = append(*flags, RuleFlag{
				RuleCode: rule.Code, TableID: rule.TableID, Severity: string(SeverityFail),
				Category: rule.Category,
				Detail: fmt.Sprintf("%s %s %s (tolerance ±%g) failed",
					left.String, op, right.String, tol),
			})
		}
		return nil

	default: // predicate — row_expression / row_join / reference / raw_sql
		// The measure SQL's column count varies by kind: row_expression/
		// row_join/reference select (violations, rows_tested); raw_sql
		// selects only (violations). Scan generically and read column 0 —
		// the same tolerance the Python engine gets for free from row[0].
		rows, err := q.QueryContext(ctx, c.MeasureSQL)
		if err != nil {
			return err
		}
		var violations int64
		if rows.Next() {
			cols, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			dest := make([]any, len(cols))
			var first sql.NullInt64
			dest[0] = &first
			for i := 1; i < len(dest); i++ {
				dest[i] = new(any)
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return err
			}
			violations = first.Int64
		}
		rows.Close()
		measurements[rule.Code] = violations

		fail, warn := rule.Zones.Fail, rule.Zones.Warn
		if fail == "" && warn == "" {
			fail = "when > 0" // default: any violation fails
		}
		state, err := EvaluateZones(float64(violations), Zone{Warn: warn, Fail: fail})
		if err != nil {
			return err
		}
		if state == "" {
			return nil
		}

		made := false
		if opts.CollectSamples && c.SampleSQL != "" {
			srows, err := q.QueryContext(ctx, c.SampleSQL)
			if err != nil {
				return err
			}
			for srows.Next() {
				rec, err := scanRow(srows)
				if err != nil {
					srows.Close()
					return err
				}
				*flags = append(*flags, flagFromSample(rule, state, rec, opts.Identity, opts.DimColumns))
				made = true
			}
			srows.Close()
		}
		if !made {
			*flags = append(*flags, RuleFlag{
				RuleCode: rule.Code, TableID: rule.TableID, Severity: state,
				Category: rule.Category,
				Detail:   fmt.Sprintf("%d violating row(s) [%s]", violations, state),
			})
		}
		return nil
	}
}
