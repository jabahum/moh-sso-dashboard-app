package data_quality

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
)

// A request that omits the array fields must still reach a NOT NULL column as an
// empty array: pq.Array renders a nil slice as SQL NULL, which the database
// rejects with "null value in column \"dim_columns\" ... violates not-null".
func TestTextArrayNeverRendersNull(t *testing.T) {
	for name, values := range map[string][]string{
		"nil slice":   nil,
		"empty slice": {},
	} {
		t.Run(name, func(t *testing.T) {
			value, err := textArray(values).(driver.Valuer).Value()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value == nil {
				t.Fatal("rendered as SQL NULL, which violates the not-null constraint")
			}
			if got, ok := value.(string); !ok || got != "{}" {
				t.Fatalf("got %#v, want \"{}\"", value)
			}
		})
	}
}

func TestTextArrayKeepsValues(t *testing.T) {
	value, err := textArray([]string{"district", "facility"}).(driver.Valuer).Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// pq quotes each element, so the literal is {"district","facility"}.
	if got := value.(string); got != `{"district","facility"}` {
		t.Fatalf("got %q, want %q", got, `{"district","facility"}`)
	}
}

// The column configuration has to survive the request DTO, or the pickers save
// nothing and the engine silently keeps its defaults.
func TestTableRequestCarriesColumnConfiguration(t *testing.T) {
	body := `{
		"table_id": "vht_monthly",
		"physical_table": "report.cht_form_097b_with_vhts",
		"period_column": "period_date",
		"dim_columns": ["district", "facility"],
		"filter_columns": ["district"]
	}`

	var req dqaTableRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	mapping := req.toMapping()

	if mapping.PeriodColumn != "period_date" {
		t.Fatalf("period column lost: %q", mapping.PeriodColumn)
	}
	if len(mapping.DimColumns) != 2 || mapping.DimColumns[0] != "district" {
		t.Fatalf("dimension columns lost: %#v", mapping.DimColumns)
	}
	if len(mapping.FilterColumns) != 1 || mapping.FilterColumns[0] != "district" {
		t.Fatalf("filter columns lost: %#v", mapping.FilterColumns)
	}
	if !mapping.IsActive {
		t.Fatal("is_active should default to true when omitted")
	}
}

func TestTableRequestWithoutColumnsIsSafe(t *testing.T) {
	var req dqaTableRequest
	if err := json.Unmarshal([]byte(`{"table_id":"t","physical_table":"report.t"}`), &req); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	mapping := req.toMapping()

	for name, values := range map[string][]string{
		"dim_columns":    mapping.DimColumns,
		"filter_columns": mapping.FilterColumns,
	} {
		value, err := textArray(values).(driver.Valuer).Value()
		if err != nil || value == nil {
			t.Fatalf("%s would be stored as NULL (value=%#v err=%v)", name, value, err)
		}
	}
}
