package apidocgen

import (
	"go/token"
	"go/types"
	"reflect"
	"testing"

	scheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

func TestRequiredPointerBooleanRejectsNullInContract(t *testing.T) {
	s := &schemas{definitions: object{}}
	field := types.NewVar(token.NoPos, nil, "Enabled", types.NewPointer(types.Typ[types.Bool]))
	for _, required := range []bool{false, true} {
		tag := `json:"enabled"`
		if required {
			tag += ` binding:"required"`
		}
		schema := s.of(types.NewStruct([]*types.Var{field}, []string{tag}))
		property := schema["properties"].(object)["enabled"].(object)
		if property["type"] != "boolean" {
			t.Fatalf("unexpected property: %v", property)
		}
		if required && property["nullable"] != nil {
			t.Fatalf("required pointer must reject null: %v", property)
		}
		if !required && property["nullable"] != true {
			t.Fatalf("optional pointer should allow null: %v", property)
		}
	}
}

func TestSchedulerExtractionPreservesPublicSchemaNames(t *testing.T) {
	schemasFor := func(path, name string) object {
		pkg := types.NewPackage(path, name)
		field := types.NewVar(token.NoPos, pkg, "ID", types.Typ[types.String])
		record := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Schedule", nil), types.NewStruct([]*types.Var{field}, []string{`json:"id"`}), nil)
		schemas := &schemas{definitions: object{}}
		return object{"reference": schemas.of(record), "definitions": schemas.definitions}
	}
	legacy := schemasFor("github.com/moh-sso-dashboard/internal/adapters/modules/report_scheduler", "report_scheduler")
	extracted := schemasFor(scheduler.ModulePath, "reportscheduler")
	if !reflect.DeepEqual(legacy, extracted) {
		t.Fatalf("scheduler schema changed after extraction: old=%v new=%v", legacy, extracted)
	}
}

func TestPublishedModuleSchemasMergeAndRejectCollisions(t *testing.T) {
	definitions := object{}
	operations, err := moduleOperations(definitions)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"OrgUnitFull", "Facility", "OrgUnitSimple", "TreeNode"} {
		if definitions["admin_units."+name] == nil {
			t.Fatal("missing module-owned schema", name)
		}
	}
	full := definitions["admin_units.OrgUnitFull"].(map[string]any)
	properties := full["properties"].(map[string]any)
	if properties["org_unit_id"].(map[string]any)["nullable"] != true {
		t.Fatal("lost nullable ID")
	}
	tree := definitions["admin_units.TreeNode"].(map[string]any)
	children := tree["properties"].(map[string]any)["children"].(map[string]any)
	if children["items"].(map[string]any)["$ref"] != "#/components/schemas/admin_units.TreeNode" {
		t.Fatal("recursive hierarchy schema lost")
	}
	if len(operations) != 87 {
		t.Fatal("expected 26 Announcements, seven Audit, 19 Metrics, nine unit, one GeoJSON and 25 scheduler routes", len(operations))
	}
	if _, err := moduleOperations(object{"admin_units.TreeNode": "collision"}); err == nil {
		t.Fatal("module schema conflict silently accepted")
	}
}
