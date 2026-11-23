package dqa

import (
	"fmt"
	"strings"
)

// Rule → SQL compiler.
//
// Turns a declarative Rule into a CompiledRule holding the Postgres SQL to
// run. Every user expression is validated by the safety layer (function
// allowlist, no statements/subqueries, quoted identifiers). Nothing here
// executes SQL — it only produces strings, so it is fully unit-testable.
//
// Uniform execution shapes:
//
//	metric            -> SELECT <agg> AS value FROM t [WHERE filter]
//	row_expression    -> SELECT count(*) FILTER (WHERE NOT(<expr>)) AS violations,
//	                            count(*) AS rows_tested FROM t [WHERE filter]
//	row_join          -> ... FROM a JOIN b ON <keys> ...
//	aggregate_compare -> SELECT (agg_left) AS left_val, (agg_right) AS right_val
//	reference         -> anti-join: count orphan rows in a not present in b
//	raw_sql           -> SELECT count(*) FROM (<fail_query>) _t

// CompileError is returned when a rule cannot be safely compiled to SQL.
type CompileError struct{ msg string }

func (e *CompileError) Error() string { return e.msg }

func compileErr(format string, args ...any) error {
	return &CompileError{msg: fmt.Sprintf(format, args...)}
}

// Compiled kinds.
const (
	KindMetric           = "metric"
	KindPredicate        = "predicate"
	KindAggregateCompare = "aggregate_compare"
)

// CompiledRule is the executable form of a Rule.
type CompiledRule struct {
	Rule       Rule
	Kind       string // metric | predicate | aggregate_compare
	MeasureSQL string
	SampleSQL  string
	Meta       map[string]any
}

const defaultSampleLimit = 100

// resolveTable maps a logical table_id to its physical table and quotes it.
func resolveTable(tableID string, tables map[string]string) string {
	db := tableID
	if tables != nil {
		if v, ok := tables[tableID]; ok && v != "" {
			db = v
		}
	}
	return QuoteTable(db)
}

// whereClause renders the optional row filter.
func whereClause(r Rule) (string, error) {
	if strings.TrimSpace(r.RowFilter) == "" {
		return "", nil
	}
	pred, err := SafePredicate(r.RowFilter)
	if err != nil {
		return "", err
	}
	return " WHERE " + pred, nil
}

// exprText resolves a row/join predicate from expr or a chain spec.
func exprText(r Rule) (string, error) {
	if e := r.defString("expr"); strings.TrimSpace(e) != "" {
		return e, nil
	}
	if raw, ok := r.Definition["chain"]; ok {
		var operands []string
		if arr, ok := raw.([]any); ok {
			for _, v := range arr {
				operands = append(operands, fmt.Sprintf("%v", v))
			}
		} else if arr, ok := raw.([]string); ok {
			operands = arr
		}
		op := r.defString("op")
		if op == "" {
			op = ">"
		}
		return BuildChain(operands, op)
	}
	return "", compileErr("rule %s: %s needs 'expr' or 'chain'", r.Code, r.RuleType)
}

type joinKey struct{ Left, Right string }

func normalizeKeys(raw any) ([]joinKey, error) {
	var out []joinKey
	arr, ok := raw.([]any)
	if !ok {
		if typed, ok2 := raw.([]joinKey); ok2 {
			out = typed
			arr = nil
		} else if raw != nil {
			return nil, compileErr("bad join keys: %v", raw)
		}
	}
	for _, k := range arr {
		switch v := k.(type) {
		case string:
			out = append(out, joinKey{Left: v, Right: v})
		case map[string]any:
			l, lok := v["left"]
			r, rok := v["right"]
			if !lok || !rok {
				return nil, compileErr("bad join key: %v", v)
			}
			out = append(out, joinKey{Left: fmt.Sprintf("%v", l), Right: fmt.Sprintf("%v", r)})
		default:
			return nil, compileErr("bad join key: %v", k)
		}
	}
	if len(out) == 0 {
		return nil, compileErr("cross-table rule needs at least one join key")
	}
	return out, nil
}

func joinOn(keys []joinKey) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("a.%s = b.%s", QuoteIdent(k.Left), QuoteIdent(k.Right)))
	}
	return strings.Join(parts, " AND ")
}

// builtinMetricSQL renders one of the supported aggregate metrics.
func builtinMetricSQL(metric, column string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(metric))
	switch m {
	case "row_count":
		return "COUNT(*)", nil
	case "sum", "avg", "min", "max", "stddev", "stddev_pop",
		"stddev_samp", "variance", "var_pop", "var_samp":
		if column == "" {
			return "", compileErr("metric %q needs a column", m)
		}
		return fmt.Sprintf("%s(%s)", strings.ToUpper(m), QuoteIdent(column)), nil
	case "missing_count":
		if column == "" {
			return "", compileErr("metric %q needs a column", m)
		}
		return fmt.Sprintf("COUNT(*) FILTER (WHERE %s IS NULL)", QuoteIdent(column)), nil
	case "missing_percent":
		if column == "" {
			return "", compileErr("metric %q needs a column", m)
		}
		return fmt.Sprintf(
			"100.0 * COUNT(*) FILTER (WHERE %s IS NULL) / NULLIF(COUNT(*), 0)",
			QuoteIdent(column)), nil
	case "duplicate_count":
		if column == "" {
			return "", compileErr("metric %q needs a column", m)
		}
		return fmt.Sprintf("COUNT(*) - COUNT(DISTINCT %s)", QuoteIdent(column)), nil
	}
	return "", compileErr("unknown builtin metric: %q", metric)
}

func compileMetric(r Rule, ref string) (*CompiledRule, error) {
	var aggSQL, label string
	if e := r.defString("expression"); strings.TrimSpace(e) != "" {
		rendered, err := SafePredicate(e)
		if err != nil {
			return nil, err
		}
		aggSQL, label = rendered, "expression"
	} else {
		s, err := builtinMetricSQL(r.defString("metric"), r.defString("column"))
		if err != nil {
			return nil, err
		}
		aggSQL, label = s, strings.ToLower(r.defString("metric"))
	}

	where, err := whereClause(r)
	if err != nil {
		return nil, err
	}
	return &CompiledRule{
		Rule:       r,
		Kind:       KindMetric,
		MeasureSQL: fmt.Sprintf("SELECT %s AS value FROM %s%s", aggSQL, ref, where),
		Meta:       map[string]any{"metric": label},
	}, nil
}

func compilePredicate(r Rule, ref string) (*CompiledRule, error) {
	text, err := exprText(r)
	if err != nil {
		return nil, err
	}
	pred, err := SafePredicate(text)
	if err != nil {
		return nil, err
	}
	where, err := whereClause(r)
	if err != nil {
		return nil, err
	}

	measure := fmt.Sprintf(
		"SELECT COUNT(*) FILTER (WHERE NOT (%s)) AS violations, COUNT(*) AS rows_tested FROM %s%s",
		pred, ref, where)

	limit := r.defInt("sample_limit", defaultSampleLimit)
	joiner := " WHERE "
	if where != "" {
		joiner = " AND "
	}
	sample := fmt.Sprintf("SELECT * FROM %s%s%sNOT (%s) LIMIT %d", ref, where, joiner, pred, limit)

	return &CompiledRule{Rule: r, Kind: KindPredicate, MeasureSQL: measure, SampleSQL: sample}, nil
}

func compileRowJoin(r Rule, tables map[string]string) (*CompiledRule, error) {
	left := resolveTable(r.defString("left"), tables)
	right := resolveTable(r.defString("right"), tables)
	keys, err := normalizeKeys(r.Definition["join_keys"])
	if err != nil {
		return nil, err
	}
	text, err := exprText(r)
	if err != nil {
		return nil, err
	}
	pred, err := SafePredicate(text)
	if err != nil {
		return nil, err
	}
	on := joinOn(keys)

	measure := fmt.Sprintf(
		"SELECT COUNT(*) FILTER (WHERE NOT (%s)) AS violations, COUNT(*) AS rows_tested "+
			"FROM %s AS a JOIN %s AS b ON %s", pred, left, right, on)
	limit := r.defInt("sample_limit", defaultSampleLimit)
	sample := fmt.Sprintf(
		"SELECT * FROM %s AS a JOIN %s AS b ON %s WHERE NOT (%s) LIMIT %d",
		left, right, on, pred, limit)

	return &CompiledRule{
		Rule: r, Kind: KindPredicate, MeasureSQL: measure, SampleSQL: sample,
		Meta: map[string]any{"cross_table": true, "mode": "row_join"},
	}, nil
}

func compileAggregateCompare(r Rule, tables map[string]string) (*CompiledRule, error) {
	side := func(key string) (table string, agg string, err error) {
		raw, ok := r.Definition[key].(map[string]any)
		if !ok {
			return "", "", compileErr("rule %s: %s side must be an object with table/agg", r.Code, key)
		}
		t, _ := raw["table"].(string)
		a, _ := raw["agg"].(string)
		if t == "" || a == "" {
			return "", "", compileErr("rule %s: %s side needs table and agg", r.Code, key)
		}
		rendered, err := SafePredicate(a)
		if err != nil {
			return "", "", err
		}
		return resolveTable(t, tables), rendered, nil
	}

	leftTable, leftAgg, err := side("left")
	if err != nil {
		return nil, err
	}
	rightTable, rightAgg, err := side("right")
	if err != nil {
		return nil, err
	}

	measure := fmt.Sprintf(
		"SELECT (SELECT %s FROM %s) AS left_val, (SELECT %s FROM %s) AS right_val",
		leftAgg, leftTable, rightAgg, rightTable)

	op := r.defString("op")
	if op == "" {
		op = "="
	}
	return &CompiledRule{
		Rule: r, Kind: KindAggregateCompare, MeasureSQL: measure,
		Meta: map[string]any{"op": op, "tolerance": r.defFloat("tolerance", 0)},
	}, nil
}

func compileReference(r Rule, tables map[string]string) (*CompiledRule, error) {
	left := resolveTable(r.defString("left"), tables)
	right := resolveTable(r.defString("right"), tables)
	keys, err := normalizeKeys(r.Definition["join_keys"])
	if err != nil {
		return nil, err
	}
	on := joinOn(keys)
	firstRight := QuoteIdent(keys[0].Right)

	measure := fmt.Sprintf(
		"SELECT COUNT(*) AS violations, COUNT(*) AS rows_tested "+
			"FROM %s AS a LEFT JOIN %s AS b ON %s WHERE b.%s IS NULL",
		left, right, on, firstRight)
	limit := r.defInt("sample_limit", defaultSampleLimit)
	sample := fmt.Sprintf(
		"SELECT a.* FROM %s AS a LEFT JOIN %s AS b ON %s WHERE b.%s IS NULL LIMIT %d",
		left, right, on, firstRight, limit)

	return &CompiledRule{
		Rule: r, Kind: KindPredicate, MeasureSQL: measure, SampleSQL: sample,
		Meta: map[string]any{"cross_table": true, "mode": "reference"},
	}, nil
}

// rawSQLForbidden are keywords a fail_query may never contain. A raw query is
// allowed to be a SELECT (unlike a scalar expression), so the statement check
// is narrower than the expression tokeniser's.
var rawSQLForbidden = []string{
	"insert", "update", "delete", "create", "drop", "alter", "merge",
	"truncate", "grant", "revoke", "copy", "call", "do", "execute",
	"prepare", "vacuum", "analyze", "lock", "commit", "rollback", "set",
}

func compileRawSQL(r Rule) (*CompiledRule, error) {
	q := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(r.defString("fail_query")), ";"))
	if q == "" {
		return nil, compileErr("raw_sql needs a 'fail_query'")
	}
	if strings.Contains(q, ";") {
		return nil, compileErr("raw_sql fail_query must be a single statement")
	}
	if strings.Contains(q, "--") || strings.Contains(q, "/*") {
		return nil, compileErr("raw_sql fail_query may not contain comments")
	}

	lower := strings.ToLower(q)
	if !strings.HasPrefix(strings.TrimSpace(lower), "select") &&
		!strings.HasPrefix(strings.TrimSpace(lower), "with") {
		return nil, compileErr("raw_sql fail_query must be a SELECT")
	}
	for _, kw := range rawSQLForbidden {
		if containsWord(lower, kw) {
			return nil, compileErr("raw_sql fail_query may not mutate data (found %q)", kw)
		}
	}

	// The rule's own SQL is opaque, so the filter is applied outside it. Without
	// this a scoped run silently scans everything and reports violations from
	// outside the scope.
	where, err := whereClause(r)
	if err != nil {
		return nil, err
	}

	limit := r.defInt("sample_limit", defaultSampleLimit)
	return &CompiledRule{
		Rule: r, Kind: KindPredicate,
		MeasureSQL: fmt.Sprintf("SELECT COUNT(*) AS violations FROM (%s) AS _dqa_sub%s", q, where),
		SampleSQL:  fmt.Sprintf("SELECT * FROM (%s) AS _dqa_sub%s LIMIT %d", q, where, limit),
		Meta:       map[string]any{"raw": true},
	}, nil
}

// containsWord reports whether haystack contains word on a token boundary.
func containsWord(haystack, word string) bool {
	idx := 0
	for {
		i := strings.Index(haystack[idx:], word)
		if i < 0 {
			return false
		}
		i += idx
		beforeOK := i == 0 || !isIdentPart(haystack[i-1])
		end := i + len(word)
		afterOK := end >= len(haystack) || !isIdentPart(haystack[end])
		if beforeOK && afterOK {
			return true
		}
		idx = i + len(word)
		if idx >= len(haystack) {
			return false
		}
	}
}

// CompileRule compiles a rule to its executable form.
//
// tables maps a logical table_id to a physical db table (identity by default).
// schema, when supplied, maps column -> type and enables column validation for
// single-table rules.
func CompileRule(rule Rule, tables map[string]string, schema map[string]string) (*CompiledRule, error) {
	switch rule.RuleType {
	case RuleTypeRowJoin:
		return compileRowJoin(rule, tables)
	case RuleTypeAggregateCompare:
		return compileAggregateCompare(rule, tables)
	case RuleTypeReference:
		return compileReference(rule, tables)
	case RuleTypeRawSQL:
		return compileRawSQL(rule)
	case RuleTypeMetric:
		ref := resolveTable(rule.TableID, tables)
		if len(schema) > 0 {
			if col := rule.defString("column"); col != "" {
				if err := ValidateColumns(col, schema); err != nil {
					return nil, compileErr("rule %s: %v", rule.Code, err)
				}
			}
		}
		return compileMetric(rule, ref)
	case RuleTypeRowExpression:
		ref := resolveTable(rule.TableID, tables)
		if len(schema) > 0 {
			text, err := exprText(rule)
			if err != nil {
				return nil, err
			}
			if err := ValidateColumns(text, schema); err != nil {
				return nil, compileErr("rule %s: %v", rule.Code, err)
			}
		}
		return compilePredicate(rule, ref)
	}
	return nil, compileErr("rule %s: unsupported rule_type %s", rule.Code, rule.RuleType)
}
