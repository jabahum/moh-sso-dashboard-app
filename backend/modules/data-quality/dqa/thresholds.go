package dqa

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Soda-style threshold / alert-zone evaluation.
//
// A zone predicate is a small grammar evaluated against one measured value:
//
//	when > 10
//	when >= 5
//	when != 0
//	when between 1 and 10
//	when not between -10 and 10
//
// The leading "when" is optional. EvaluateZones returns the most-severe
// triggered state (fail beats warn) or "" for pass.

const numPattern = `-?\d+(?:\.\d+)?`

var (
	whenPrefixRe = regexp.MustCompile(`(?i)^\s*when\s+`)
	notBetweenRe = regexp.MustCompile(`(?i)^not\s+between\s+(` + numPattern + `)\s+and\s+(` + numPattern + `)$`)
	betweenRe    = regexp.MustCompile(`(?i)^between\s+(` + numPattern + `)\s+and\s+(` + numPattern + `)$`)
	comparisonRe = regexp.MustCompile(`^(>=|<=|!=|<>|=|>|<)\s*(` + numPattern + `)$`)
)

// EvaluatePredicate reports whether value triggers predicate.
// It returns an error if the predicate cannot be parsed.
func EvaluatePredicate(value float64, predicate string) (bool, error) {
	p := strings.TrimSpace(whenPrefixRe.ReplaceAllString(strings.TrimSpace(predicate), ""))
	if p == "" {
		return false, nil
	}

	if m := notBetweenRe.FindStringSubmatch(p); m != nil {
		lo, hi, err := parseBounds(m[1], m[2])
		if err != nil {
			return false, err
		}
		return !(value >= lo && value <= hi), nil
	}

	if m := betweenRe.FindStringSubmatch(p); m != nil {
		lo, hi, err := parseBounds(m[1], m[2])
		if err != nil {
			return false, err
		}
		return value >= lo && value <= hi, nil
	}

	if m := comparisonRe.FindStringSubmatch(p); m != nil {
		n, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return false, fmt.Errorf("unparseable threshold predicate: %q", predicate)
		}
		switch m[1] {
		case ">":
			return value > n, nil
		case "<":
			return value < n, nil
		case ">=":
			return value >= n, nil
		case "<=":
			return value <= n, nil
		case "=":
			return value == n, nil
		case "!=", "<>":
			return value != n, nil
		}
	}

	return false, fmt.Errorf("unparseable threshold predicate: %q", predicate)
}

func parseBounds(a, b string) (float64, float64, error) {
	lo, err := strconv.ParseFloat(a, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("bad lower bound %q", a)
	}
	hi, err := strconv.ParseFloat(b, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("bad upper bound %q", b)
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi, nil
}

// EvaluateZones returns "fail", "warn" or "" (pass). Fail takes precedence.
func EvaluateZones(value float64, z Zone) (string, error) {
	if z.Fail != "" {
		hit, err := EvaluatePredicate(value, z.Fail)
		if err != nil {
			return "", err
		}
		if hit {
			return string(SeverityFail), nil
		}
	}
	if z.Warn != "" {
		hit, err := EvaluatePredicate(value, z.Warn)
		if err != nil {
			return "", err
		}
		if hit {
			return string(SeverityWarn), nil
		}
	}
	return "", nil
}
