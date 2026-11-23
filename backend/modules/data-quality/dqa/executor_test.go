package dqa

import "testing"

func TestAggregateHolds(t *testing.T) {
	cases := []struct {
		left, right, op string
		tol             float64
		want            bool
	}{
		{"10", "10", "=", 0, true},
		{"10", "10.4", "=", 0.5, true},
		{"10", "11", "=", 0.5, false},
		{"10", "10", "!=", 0, false},
		{"11", "10", ">", 0, true},
		{"9", "10", "<", 0, true},
		{"10", "10", ">=", 0, true},
		{"10", "10", "<=", 0, true},
	}
	for _, c := range cases {
		got, err := aggregateHolds(c.left, c.right, c.op, c.tol)
		if err != nil {
			t.Fatalf("aggregateHolds(%v,%v,%v,%v) error: %v", c.left, c.right, c.op, c.tol, err)
		}
		if got != c.want {
			t.Errorf("aggregateHolds(%v,%v,%v,%v) = %v, want %v", c.left, c.right, c.op, c.tol, got, c.want)
		}
	}
	if _, err := aggregateHolds("1", "1", "??", 0); err == nil {
		t.Error("expected an error for an unknown operator")
	}
}

func TestFlagFromSampleUsesDefaultIdentity(t *testing.T) {
	rule := Rule{Code: "R1", TableID: "t", Category: "custom", Name: "check"}
	rec := map[string]any{
		"vht_uuid": "abc-1", "period_date": "2024-01-01", "district": "Gomba",
		"chw_name": "Jane", "id": int64(42), "extra_col": "x",
	}
	f := flagFromSample(rule, "fail", rec, nil, []string{"extra_col"})
	if f.EntityID != "abc-1" || f.PeriodDate != "2024-01-01" || f.District != "Gomba" ||
		f.EntityName != "Jane" || f.RowID != "42" {
		t.Errorf("identity not extracted correctly: %+v", f)
	}
	if f.Dims["extra_col"] != "x" {
		t.Errorf("dim column not captured: %+v", f.Dims)
	}
	if f.RuleCode != "R1" || f.Severity != "fail" || f.Detail != "check" {
		t.Errorf("core fields wrong: %+v", f)
	}
}

func TestFlagFromSampleCustomIdentity(t *testing.T) {
	rule := Rule{Code: "R2", TableID: "t"}
	rec := map[string]any{"my_entity": "e1"}
	f := flagFromSample(rule, "warn", rec, map[string]string{"entity": "my_entity"}, nil)
	if f.EntityID != "e1" {
		t.Errorf("custom identity mapping not honoured: %+v", f)
	}
	if f.Detail == "" {
		t.Errorf("expected a fallback detail when Name is empty")
	}
}
