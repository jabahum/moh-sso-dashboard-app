package conformance

import (
	"fmt"
	"sort"
)

// Field describes one property of a response object.
type Field struct {
	// Type is the JSON type: string, boolean, integer, number, array or object.
	Type string
	// Always means the portal always sends the field.
	Always bool
	// Nullable accepts JSON null.
	Nullable bool
}

// Shapes are the response objects the suite checks, by contract schema name
// (components.schemas in platform.openapi.yaml). A backend test keeps the
// field names and types identical to the published contract.
var Shapes = map[string]map[string]Field{
	"platform.Principal": {
		"applicationId": {Type: "string", Always: true},
		"clientId":      {Type: "string", Always: true},
		"subject":       {Type: "string", Always: true},
		"scopes":        {Type: "array", Always: true},
	},
	"platform.Readiness": {
		"ready":   {Type: "boolean", Always: true},
		"workers": {Type: "array", Always: true, Nullable: true},
	},
	"platform.WorkerHealth": {
		"name":        {Type: "string", Always: true},
		"healthy":     {Type: "boolean", Always: true},
		"lastSeen":    {Type: "string"},
		"localStatus": {Type: "object", Nullable: true},
	},
	"platform.NotificationStatus": {
		"id":               {Type: "string", Always: true},
		"applicationId":    {Type: "string", Always: true},
		"channel":          {Type: "string", Always: true},
		"status":           {Type: "string", Always: true},
		"dispatchStatus":   {Type: "string", Always: true},
		"dispatchAttempts": {Type: "integer", Always: true},
		"deliveryAttempts": {Type: "integer", Always: true},
		"createdAt":        {Type: "string", Always: true},
		"replayed":         {Type: "boolean", Always: true},
	},
	"platform.UserAccess": {
		"applicationId": {Type: "string", Always: true},
		"subject":       {Type: "string", Always: true},
		"roles":         {Type: "array", Always: true, Nullable: true},
		"permissions":   {Type: "array", Always: true, Nullable: true},
	},
}

// ShapeProblems lists how obj differs from the named shape: missing fields the
// portal always sends, and fields of the wrong type. Unknown fields are
// allowed, because new response fields are additive.
func ShapeProblems(shape string, obj map[string]any) []string {
	fields, ok := Shapes[shape]
	if !ok {
		return []string{"unknown shape " + shape}
	}
	if obj == nil {
		return []string{shape + " is not an object"}
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	var problems []string
	for _, name := range names {
		field := fields[name]
		value, present := obj[name]
		switch {
		case !present:
			if field.Always {
				problems = append(problems, fmt.Sprintf("%s.%s is missing", shape, name))
			}
		case value == nil:
			if !field.Nullable {
				problems = append(problems, fmt.Sprintf("%s.%s is null", shape, name))
			}
		case jsonType(value) != field.Type && !(field.Type == "integer" && isInteger(value)):
			problems = append(problems, fmt.Sprintf("%s.%s is %s, want %s", shape, name, jsonType(value), field.Type))
		}
	}
	return problems
}

func jsonType(value any) string {
	switch v := value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64:
		if v == float64(int64(v)) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

func isInteger(value any) bool {
	f, ok := value.(float64)
	return ok && f == float64(int64(f))
}
