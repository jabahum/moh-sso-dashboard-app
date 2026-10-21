package apidocgen

import (
	"fmt"
	"strconv"
	"strings"
)

// Validate checks the contract invariants we can enforce without a live backend:
// route/method parity, unique IDs, path parameters, responses and local references.
func Validate(spec object) error {
	if spec["openapi"] != "3.0.3" {
		return fmt.Errorf("expected OpenAPI 3.0.3")
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		return fmt.Errorf("specification has no paths")
	}
	ids := map[string]bool{}
	for path, item := range paths {
		methods, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid path %s", path)
		}
		for method, item := range methods {
			if !isHTTPMethod(strings.ToUpper(method)) {
				continue
			}
			op, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid operation %s %s", method, path)
			}
			id, _ := op["operationId"].(string)
			if id == "" || ids[id] {
				return fmt.Errorf("missing or duplicate operationId for %s %s: %s", method, path, id)
			}
			ids[id] = true
			responses, ok := op["responses"].(map[string]any)
			if !ok || len(responses) == 0 {
				return fmt.Errorf("missing responses for %s %s", method, path)
			}
			for code, item := range responses {
				response, ok := item.(map[string]any)
				if !ok || response["description"] == "" || response["description"] == nil {
					return fmt.Errorf("missing response description for %s %s (%s)", method, path, code)
				}
			}
			parameters := map[string]bool{}
			if list, ok := op["parameters"].([]any); ok {
				for _, item := range list {
					if parameter, ok := item.(map[string]any); ok {
						if parameter["in"] == "path" && parameter["required"] == true {
							name, _ := parameter["name"].(string)
							parameters[name] = true
						}
					}
				}
			}
			for _, name := range parameterPattern.FindAllString(path, -1) {
				if !parameters[strings.Trim(name, "{}")] {
					return fmt.Errorf("missing required path parameter %s in %s %s", name, method, path)
				}
			}
		}
	}
	for _, route := range Routes() {
		path := OpenAPIPath(route.Path)
		methods, _ := paths[path].(map[string]any)
		if methods[strings.ToLower(route.Method)] == nil {
			return fmt.Errorf("undocumented endpoint: %s %s", route.Method, path)
		}
	}
	var walk func(any) error
	walk = func(value any) error {
		switch node := value.(type) {
		case map[string]any:
			if ref, ok := node["$ref"].(string); ok {
				if !strings.HasPrefix(ref, "#/") {
					return fmt.Errorf("external references are not bundled: %s", ref)
				}
				var current any = spec
				for _, segment := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					segment = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
					switch container := current.(type) {
					case map[string]any:
						current = container[segment]
					case []any:
						index, err := strconv.Atoi(segment)
						if err != nil || index < 0 || index >= len(container) {
							current = nil
						} else {
							current = container[index]
						}
					default:
						current = nil
					}
					if current == nil {
						return fmt.Errorf("unresolved reference: %s", ref)
					}
				}
			}
			for _, child := range node {
				if err := walk(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(spec)
}
