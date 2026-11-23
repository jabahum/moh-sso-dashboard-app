package apidocgen

import (
	"reflect"
	"testing"
)

// Module contracts arrive as decoded JSON; their required form fields must
// survive the override merge.
func TestApplyOverridesKeepsRequiredFieldsFromModuleContracts(t *testing.T) {
	schema := object{"properties": object{"file": object{}}, "required": []any{"file"}}
	op := object{"requestBody": object{"content": object{"multipart/form-data": object{"schema": schema}}}}
	applyOverrides(op, object{"x-required-form-fields": []any{"storage_location"}})
	if got := schema["required"]; !reflect.DeepEqual(got, []string{"file", "storage_location"}) {
		t.Fatalf("required = %v", got)
	}
}
