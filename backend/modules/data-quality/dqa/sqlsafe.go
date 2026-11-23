package dqa

import (
	"fmt"
	"strings"
)

// SQL safety layer.
//
// User-supplied rule text is tokenised and validated before it is ever placed
// in a query. The Python original leaned on sqlglot's AST; this port enforces
// the same guarantees with an explicit tokeniser, which keeps the trust
// boundary small and auditable:
//
//  1. reject ';', comments and anything that could start a statement
//  2. reject statement/DDL/DML keywords (no subqueries, no mutation)
//  3. enforce a function allowlist on every call site
//  4. re-render with every bare identifier double-quoted
//
// Nothing here touches a database — it only parses and renders strings.

// SafetyError is returned when user SQL fails a safety check.
type SafetyError struct{ msg string }

func (e *SafetyError) Error() string { return e.msg }

func safetyErr(format string, args ...any) error {
	return &SafetyError{msg: fmt.Sprintf(format, args...)}
}

// allowedFunctions gates every call site in a user expression.
var allowedFunctions = map[string]bool{
	// aggregates
	"avg": true, "sum": true, "count": true, "min": true, "max": true,
	"stddev": true, "stddev_pop": true, "stddev_samp": true,
	"variance": true, "var_pop": true, "var_samp": true,
	"percentile_cont": true, "percentile_disc": true,
	// arithmetic / numeric
	"abs": true, "round": true, "ceil": true, "ceiling": true, "floor": true,
	"sqrt": true, "power": true, "pow": true, "mod": true, "ln": true,
	"log": true, "log10": true, "exp": true, "sign": true, "trunc": true,
	"least": true, "greatest": true, "coalesce": true, "nullif": true,
	// strings
	"length": true, "char_length": true, "lower": true, "upper": true,
	"trim": true, "btrim": true, "left": true, "right": true,
	// window
	"lag": true, "lead": true, "row_number": true, "rank": true,
	"dense_rank": true, "ntile": true, "first_value": true, "last_value": true,
	// dates
	"extract": true, "date_trunc": true, "date_part": true, "age": true, "now": true,
}

// forbiddenKeywords indicate a statement rather than a scalar/boolean
// expression. Any of these anywhere in the input is rejected outright.
var forbiddenKeywords = map[string]bool{
	"select": true, "insert": true, "update": true, "delete": true,
	"create": true, "drop": true, "alter": true, "merge": true,
	"truncate": true, "grant": true, "revoke": true, "copy": true,
	"union": true, "intersect": true, "except": true, "with": true,
	"set": true, "use": true, "call": true, "do": true, "execute": true,
	"prepare": true, "deallocate": true, "vacuum": true, "analyze": true,
	"listen": true, "notify": true, "lock": true, "begin": true,
	"commit": true, "rollback": true, "savepoint": true, "declare": true,
	"fetch": true, "into": true, "returning": true, "values": true,
}

// bareKeywords are emitted verbatim during rendering rather than being quoted
// as identifiers. This covers operators, CASE/CAST syntax, EXTRACT fields and
// the type names a cast may target.
var bareKeywords = map[string]bool{
	"and": true, "or": true, "not": true, "is": true, "null": true,
	"true": true, "false": true, "between": true, "in": true,
	"like": true, "ilike": true, "similar": true, "to": true, "escape": true,
	"case": true, "when": true, "then": true, "else": true, "end": true,
	"cast": true, "as": true, "distinct": true, "filter": true, "where": true,
	"over": true, "partition": true, "by": true, "order": true,
	"asc": true, "desc": true, "nulls": true, "first": true, "last": true,
	"within": true, "group": true, "all": true, "any": true, "some": true,
	"interval": true, "at": true, "time": true, "zone": true, "from": true,
	"collate": true, "unknown": true,
	// EXTRACT / date_part fields
	"year": true, "month": true, "day": true, "hour": true, "minute": true,
	"second": true, "dow": true, "doy": true, "week": true, "quarter": true,
	"epoch": true, "century": true, "decade": true, "millennium": true,
	"microseconds": true, "milliseconds": true, "isodow": true, "isoyear": true,
	// cast target types
	"int": true, "integer": true, "int2": true, "int4": true, "int8": true,
	"bigint": true, "smallint": true, "numeric": true, "decimal": true,
	"real": true, "double": true, "precision": true, "float": true,
	"float4": true, "float8": true, "text": true, "varchar": true,
	"char": true, "character": true, "varying": true, "boolean": true,
	"bool": true, "date": true, "timestamp": true, "timestamptz": true,
	"jsonb": true, "json": true, "uuid": true,
}

type tokenKind int

const (
	tokIdent  tokenKind = iota // bare identifier
	tokQuoted                  // "already quoted"
	tokString                  // 'literal'
	tokNumber
	tokOperator
	tokPunct // ( ) , .
	tokStar
)

type token struct {
	kind tokenKind
	text string
}

// Expression is a validated user expression, held as tokens.
type Expression struct {
	tokens []token
	raw    string
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '$'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// tokenize splits the input, rejecting anything structurally unsafe.
func tokenize(text string) ([]token, error) {
	var out []token
	depth := 0

	for i := 0; i < len(text); {
		c := text[i]

		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++

		case c == ';':
			return nil, safetyErr("';' is not allowed in an expression")

		case c == '-' && i+1 < len(text) && text[i+1] == '-':
			return nil, safetyErr("SQL comments are not allowed")

		case c == '/' && i+1 < len(text) && text[i+1] == '*':
			return nil, safetyErr("SQL comments are not allowed")

		case c == '"':
			j := i + 1
			for j < len(text) && text[j] != '"' {
				j++
			}
			if j >= len(text) {
				return nil, safetyErr("unterminated quoted identifier")
			}
			out = append(out, token{tokQuoted, text[i+1 : j]})
			i = j + 1

		case c == '\'':
			j := i + 1
			for j < len(text) {
				if text[j] == '\'' {
					if j+1 < len(text) && text[j+1] == '\'' { // escaped ''
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= len(text) {
				return nil, safetyErr("unterminated string literal")
			}
			out = append(out, token{tokString, text[i : j+1]})
			i = j + 1

		case isDigit(c) || (c == '.' && i+1 < len(text) && isDigit(text[i+1])):
			j := i
			for j < len(text) && (isDigit(text[j]) || text[j] == '.') {
				j++
			}
			// exponent
			if j < len(text) && (text[j] == 'e' || text[j] == 'E') {
				k := j + 1
				if k < len(text) && (text[k] == '+' || text[k] == '-') {
					k++
				}
				if k < len(text) && isDigit(text[k]) {
					for k < len(text) && isDigit(text[k]) {
						k++
					}
					j = k
				}
			}
			out = append(out, token{tokNumber, text[i:j]})
			i = j

		case isIdentStart(c):
			j := i
			for j < len(text) && isIdentPart(text[j]) {
				j++
			}
			word := text[i:j]
			if forbiddenKeywords[strings.ToLower(word)] {
				return nil, safetyErr("statements/DDL are not allowed — expression only (found %q)", word)
			}
			out = append(out, token{tokIdent, word})
			i = j

		case c == '(':
			depth++
			out = append(out, token{tokPunct, "("})
			i++

		case c == ')':
			depth--
			if depth < 0 {
				return nil, safetyErr("unbalanced parentheses")
			}
			out = append(out, token{tokPunct, ")"})
			i++

		case c == ',' || c == '.':
			out = append(out, token{tokPunct, string(c)})
			i++

		case c == '*':
			out = append(out, token{tokStar, "*"})
			i++

		case strings.ContainsRune("+-/%^<>=!|:~&", rune(c)):
			j := i
			for j < len(text) && strings.ContainsRune("+-/%^<>=!|:~&", rune(text[j])) {
				j++
			}
			op := text[i:j]
			if !isAllowedOperator(op) {
				return nil, safetyErr("operator not allowed: %s", op)
			}
			out = append(out, token{tokOperator, op})
			i = j

		default:
			return nil, safetyErr("unexpected character %q in expression", string(c))
		}
	}

	if depth != 0 {
		return nil, safetyErr("unbalanced parentheses")
	}
	if len(out) == 0 {
		return nil, safetyErr("empty expression")
	}
	return out, nil
}

func isAllowedOperator(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%", "^",
		"<", ">", "=", "<=", ">=", "<>", "!=",
		"||", "::":
		return true
	}
	return false
}

// checkFunctions enforces the allowlist on every identifier used as a call.
func checkFunctions(toks []token) error {
	for i, t := range toks {
		if t.kind != tokIdent {
			continue
		}
		if i+1 < len(toks) && toks[i+1].kind == tokPunct && toks[i+1].text == "(" {
			name := strings.ToLower(t.text)
			if bareKeywords[name] {
				continue // CAST(...), CASE ... — syntax, not a call
			}
			if !allowedFunctions[name] {
				return safetyErr("function not allowed: %s()", name)
			}
		}
	}
	return nil
}

// ParseExpression validates a scalar/boolean expression and returns it.
func ParseExpression(text string) (*Expression, error) {
	if strings.TrimSpace(text) == "" {
		return nil, safetyErr("empty expression")
	}
	toks, err := tokenize(text)
	if err != nil {
		return nil, err
	}
	if err := checkFunctions(toks); err != nil {
		return nil, err
	}
	return &Expression{tokens: toks, raw: text}, nil
}

// Render emits Postgres SQL with every bare identifier double-quoted.
func (e *Expression) Render() string {
	var b strings.Builder
	for i, t := range e.tokens {
		if i > 0 && needsSpace(e.tokens[i-1], t) {
			b.WriteByte(' ')
		}
		switch t.kind {
		case tokIdent:
			lower := strings.ToLower(t.text)
			isCall := i+1 < len(e.tokens) &&
				e.tokens[i+1].kind == tokPunct && e.tokens[i+1].text == "("
			if bareKeywords[lower] || isCall {
				b.WriteString(t.text)
			} else {
				b.WriteString(QuoteIdent(t.text))
			}
		case tokQuoted:
			b.WriteString(QuoteIdent(t.text))
		default:
			b.WriteString(t.text)
		}
	}
	return b.String()
}

// needsSpace decides whether to separate two adjacent tokens.
func needsSpace(prev, cur token) bool {
	if prev.kind == tokPunct && (prev.text == "(" || prev.text == ".") {
		return false
	}
	if cur.kind == tokPunct && (cur.text == ")" || cur.text == "," || cur.text == ".") {
		return false
	}
	if prev.kind == tokPunct && prev.text == "," {
		return true
	}
	if cur.kind == tokPunct && cur.text == "(" {
		// keep func( tight, but separate keywords like IN (
		return prev.kind == tokIdent && bareKeywords[strings.ToLower(prev.text)]
	}
	if prev.kind == tokOperator && prev.text == "::" {
		return false
	}
	if cur.kind == tokOperator && cur.text == "::" {
		return false
	}
	return true
}

// QuoteIdent double-quotes an identifier, escaping embedded quotes.
func QuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// QuoteTable quotes a possibly schema-qualified table name,
// e.g. report.foo -> "report"."foo".
func QuoteTable(dbTable string) string {
	parts := strings.Split(dbTable, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.Trim(p, `"`))
		if p == "" {
			continue
		}
		out = append(out, QuoteIdent(p))
	}
	if len(out) == 0 {
		return QuoteIdent(dbTable)
	}
	return strings.Join(out, ".")
}

// SafePredicate parses, validates and renders a boolean/scalar predicate.
func SafePredicate(text string) (string, error) {
	e, err := ParseExpression(text)
	if err != nil {
		return "", err
	}
	return e.Render(), nil
}

// BuildChain expands a chained comparison into a conjunction.
//
//	BuildChain([]string{"x","y","a"}, ">") -> "x > y AND y > a"
//
// The result is still validated by ParseExpression before use.
func BuildChain(operands []string, op string) (string, error) {
	if len(operands) < 2 {
		return "", safetyErr("a chain needs at least two operands")
	}
	switch op {
	case ">", "<", ">=", "<=", "=", "!=", "<>":
	default:
		return "", safetyErr("chain operator must be one of <, <=, <>, =, >, >=, !=")
	}
	parts := make([]string, 0, len(operands)-1)
	for i := 0; i < len(operands)-1; i++ {
		parts = append(parts, fmt.Sprintf("%s %s %s", operands[i], op, operands[i+1]))
	}
	return strings.Join(parts, " AND "), nil
}

// ValidateColumns reports an error if the expression references a column that
// is not present in schema (column name -> type). Only bare identifiers that
// are not keywords, function names or qualifier prefixes are checked.
func ValidateColumns(exprText string, schema map[string]string) error {
	if len(schema) == 0 {
		return nil
	}
	e, err := ParseExpression(exprText)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(schema))
	for col := range schema {
		known[strings.ToLower(col)] = true
	}

	for i, t := range e.tokens {
		if t.kind != tokIdent && t.kind != tokQuoted {
			continue
		}
		lower := strings.ToLower(t.text)
		if t.kind == tokIdent && bareKeywords[lower] {
			continue
		}
		// skip function names
		if i+1 < len(e.tokens) && e.tokens[i+1].kind == tokPunct && e.tokens[i+1].text == "(" {
			continue
		}
		// skip a qualifier such as the "a" in a.col
		if i+1 < len(e.tokens) && e.tokens[i+1].kind == tokPunct && e.tokens[i+1].text == "." {
			continue
		}
		if !known[lower] {
			return safetyErr("unknown column reference: %s", t.text)
		}
	}
	return nil
}
