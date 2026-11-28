package apidocgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// BreakingChanges compares two versions of an OpenAPI contract and lists the
// changes that can break an existing client:
//
//   - a removed path or operation, or a removed success response;
//   - a new required parameter or request field, an optional one that became
//     required, or a request body that became required;
//   - a response field that was removed or is no longer always present;
//   - a changed type of any parameter, request or response field.
//
// A higher major version in current accepts every change: breaking changes
// ship as a new major version (and a new /api/vN path), not within one.
func BreakingChanges(base, current []byte) ([]string, error) {
	var b, c object
	if err := yaml.Unmarshal(dropHeader(base), &b); err != nil {
		return nil, fmt.Errorf("parse base contract: %w", err)
	}
	if err := yaml.Unmarshal(dropHeader(current), &c); err != nil {
		return nil, fmt.Errorf("parse current contract: %w", err)
	}
	if major(c) > major(b) {
		return nil, nil
	}
	cmp := comparison{base: b, current: c}
	basePaths, _ := b["paths"].(object)
	currentPaths, _ := c["paths"].(object)
	for _, path := range sortedKeys(basePaths) {
		baseItem, _ := basePaths[path].(object)
		currentItem, ok := currentPaths[path].(object)
		if !ok {
			cmp.add(path, "", "path removed")
			continue
		}
		for _, method := range sortedKeys(baseItem) {
			if !isHTTPMethod(strings.ToUpper(method)) {
				continue
			}
			baseOp, _ := baseItem[method].(object)
			currentOp, ok := currentItem[method].(object)
			if !ok {
				cmp.add(path, method, "operation removed")
				continue
			}
			cmp.operation(path, method, baseOp, currentOp)
		}
	}
	sort.Strings(cmp.changes)
	return cmp.changes, nil
}

type comparison struct {
	base, current object
	changes       []string
}

func (c *comparison) add(path, method, format string, args ...any) {
	where := path
	if method != "" {
		where = strings.ToUpper(method) + " " + path
	}
	c.changes = append(c.changes, where+": "+fmt.Sprintf(format, args...))
}

func (c *comparison) operation(path, method string, base, current object) {
	baseParams := parametersByKey(base)
	for key, param := range parametersByKey(current) {
		was, existed := baseParams[key]
		required, _ := param["required"].(bool)
		wasRequired, _ := was["required"].(bool)
		switch {
		case required && !existed:
			c.add(path, method, "new required parameter %s", key)
		case required && !wasRequired:
			c.add(path, method, "parameter %s became required", key)
		}
		if existed {
			c.schema(path, method, "parameter "+key, was["schema"], param["schema"], false, map[string]bool{})
		}
	}

	baseBody, _ := base["requestBody"].(object)
	currentBody, _ := current["requestBody"].(object)
	if currentBody != nil {
		required, _ := currentBody["required"].(bool)
		wasRequired, _ := baseBody["required"].(bool)
		if required && !wasRequired {
			c.add(path, method, "request body became required")
		}
		if baseBody != nil {
			c.schema(path, method, "request", jsonSchema(baseBody), jsonSchema(currentBody), false, map[string]bool{})
		}
	}

	baseResponses, _ := base["responses"].(object)
	currentResponses, _ := current["responses"].(object)
	for _, status := range sortedKeys(baseResponses) {
		code, err := strconv.Atoi(status)
		if err != nil || code < 200 || code > 299 {
			continue
		}
		currentResponse, ok := currentResponses[status].(object)
		if !ok {
			c.add(path, method, "response %s removed", status)
			continue
		}
		baseResponse, _ := baseResponses[status].(object)
		c.schema(path, method, "response "+status, jsonSchema(baseResponse), jsonSchema(currentResponse), true, map[string]bool{})
	}
}

// schema compares two schemas at location. In responses, removing a field or
// making it optional breaks clients; in requests, adding a required field does.
func (c *comparison) schema(path, method, location string, base, current any, response bool, seen map[string]bool) {
	b, bRef := resolve(c.base, base)
	cur, cRef := resolve(c.current, current)
	if b == nil || cur == nil {
		return
	}
	if bRef != "" && cRef != "" {
		key := location + "|" + bRef + "|" + cRef
		if seen[key] {
			return
		}
		seen[key] = true
	}
	if bt, ct := b["type"], cur["type"]; bt != nil && ct != nil && fmt.Sprint(bt) != fmt.Sprint(ct) {
		c.add(path, method, "%s type changed from %v to %v", location, bt, ct)
		return
	}
	baseProps, _ := b["properties"].(object)
	currentProps, _ := cur["properties"].(object)
	baseRequired, currentRequired := stringSet(b["required"]), stringSet(cur["required"])
	if response {
		for _, name := range sortedKeys(baseProps) {
			if _, ok := currentProps[name]; !ok {
				c.add(path, method, "%s field %s removed", location, name)
				continue
			}
			if baseRequired[name] && !currentRequired[name] {
				c.add(path, method, "%s field %s is no longer always present", location, name)
			}
		}
	} else {
		for _, name := range sortedKeys(currentRequired) {
			if !baseRequired[name] {
				c.add(path, method, "%s field %s became required", location, name)
			}
		}
	}
	for _, name := range sortedKeys(baseProps) {
		if next, ok := currentProps[name]; ok {
			c.schema(path, method, location+"."+name, baseProps[name], next, response, seen)
		}
	}
	if items, ok := b["items"]; ok {
		if next, ok := cur["items"]; ok {
			c.schema(path, method, location+"[]", items, next, response, seen)
		}
	}
}

// resolve follows a local component $ref and returns the schema and ref name.
func resolve(spec object, value any) (object, string) {
	schema, _ := value.(object)
	ref, _ := schema["$ref"].(string)
	if ref == "" {
		return schema, ""
	}
	components, _ := spec["components"].(object)
	schemas, _ := components["schemas"].(object)
	resolved, _ := schemas[strings.TrimPrefix(ref, schemaRefPrefix)].(object)
	return resolved, ref
}

func jsonSchema(holder object) any {
	content, _ := holder["content"].(object)
	media, _ := content["application/json"].(object)
	return media["schema"]
}

func parametersByKey(op object) map[string]object {
	params := map[string]object{}
	list, _ := op["parameters"].([]any)
	for _, item := range list {
		if param, ok := item.(object); ok {
			params[fmt.Sprint(param["in"])+" "+fmt.Sprint(param["name"])] = param
		}
	}
	return params
}

func stringSet(value any) map[string]bool {
	set := map[string]bool{}
	list, _ := value.([]any)
	for _, item := range list {
		if s, ok := item.(string); ok {
			set[s] = true
		}
	}
	return set
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func major(spec object) int {
	info, _ := spec["info"].(object)
	version, _ := info["version"].(string)
	n, _ := strconv.Atoi(strings.SplitN(version, ".", 2)[0])
	return n
}
