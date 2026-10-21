package apidocgen

import (
	"go/ast"
	"go/types"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

type object = map[string]any

type schemas struct{ definitions object }

var schemaName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func (s *schemas) of(t types.Type) object {
	if t == nil {
		return object{}
	}
	if alias, ok := t.(*types.Alias); ok {
		return s.of(types.Unalias(alias))
	}
	if ptr, ok := t.(*types.Pointer); ok {
		value := s.of(ptr.Elem())
		if _, ref := value["$ref"]; ref {
			return object{"allOf": []any{value}, "nullable": true}
		}
		result := object{}
		for key, v := range value {
			result[key] = v
		}
		result["nullable"] = true
		return result
	}
	if n, ok := t.(*types.Named); ok {
		pkg := n.Obj().Pkg()
		if pkg != nil {
			full := pkg.Path() + "." + n.Obj().Name()
			switch full {
			case "time.Time":
				return object{"type": "string", "format": "date-time"}
			case "github.com/google/uuid.UUID":
				return object{"type": "string", "format": "uuid"}
			case "github.com/google/uuid.NullUUID":
				return object{"type": "string", "format": "uuid", "nullable": true}
			case "encoding/json.RawMessage", "github.com/sqlc-dev/pqtype.NullRawMessage":
				return object{}
			}
			if _, basic := n.Underlying().(*types.Basic); basic {
				return s.of(n.Underlying())
			}
			name := pkg.Name() + "." + n.Obj().Name()
			if n.TypeArgs() != nil && n.TypeArgs().Len() > 0 {
				name = types.TypeString(n, func(p *types.Package) string { return p.Name() })
			}
			name = schemaName.ReplaceAllString(name, "_")
			if _, found := s.definitions[name]; !found {
				s.definitions[name] = object{} // Break recursive DTO cycles.
				s.definitions[name] = s.of(n.Underlying())
			}
			return object{"$ref": "#/components/schemas/" + name}
		}
		return s.of(n.Underlying())
	}
	switch value := t.(type) {
	case *types.Basic:
		switch {
		case value.Info()&types.IsBoolean != 0:
			return object{"type": "boolean"}
		case value.Info()&types.IsInteger != 0:
			return object{"type": "integer", "format": "int64"}
		case value.Info()&types.IsFloat != 0:
			return object{"type": "number", "format": "double"}
		case value.Info()&types.IsString != 0:
			return object{"type": "string"}
		}
	case *types.Slice:
		if b, ok := value.Elem().(*types.Basic); ok && b.Kind() == types.Uint8 {
			return object{"type": "string", "format": "byte"}
		}
		return object{"type": "array", "items": s.of(value.Elem())}
	case *types.Array:
		return object{"type": "array", "items": s.of(value.Elem()), "minItems": value.Len(), "maxItems": value.Len()}
	case *types.Map:
		return object{"type": "object", "additionalProperties": s.of(value.Elem())}
	case *types.Struct:
		properties := object{}
		required := []string{}
		for i := 0; i < value.NumFields(); i++ {
			field := value.Field(i)
			if !field.Exported() {
				continue
			}
			tag := reflect.StructTag(value.Tag(i))
			jsonTag := strings.Split(tag.Get("json"), ",")
			if jsonTag[0] == "-" {
				continue
			}
			if field.Embedded() && jsonTag[0] == "" {
				embedded := field.Type()
				if p, ok := embedded.(*types.Pointer); ok {
					embedded = p.Elem()
				}
				if n, ok := embedded.(*types.Named); ok {
					embedded = n.Underlying()
				}
				if _, ok := embedded.(*types.Struct); ok {
					fields := s.of(embedded)
					for key, val := range fields["properties"].(object) {
						properties[key] = val
					}
					if keys, ok := fields["required"].([]string); ok {
						required = append(required, keys...)
					}
					continue
				}
			}
			name := jsonTag[0]
			if name == "" {
				name = field.Name()
			}
			property := s.of(field.Type())
			if strings.Contains(tag.Get("json"), ",string") {
				property = object{"type": "string"}
			}
			applyValidation(property, tag.Get("binding"))
			if example := tag.Get("example"); example != "" {
				property["example"] = example
			}
			properties[name] = property
			if containsRule(tag.Get("binding"), "required") {
				required = append(required, name)
			}
		}
		result := object{"type": "object", "properties": properties}
		if len(required) > 0 {
			result["required"] = required
		}
		return result
	}
	// Interfaces and raw JSON are deliberately unconstrained, not invented DTOs.
	return object{}
}

func containsRule(rules, rule string) bool {
	for _, item := range strings.Split(rules, ",") {
		if item == rule {
			return true
		}
	}
	return false
}

func applyValidation(schema object, rules string) {
	for _, rule := range strings.Split(rules, ",") {
		if rule == "dive" {
			break
		}
		parts := strings.SplitN(rule, "=", 2)
		if parts[0] == "email" {
			schema["format"] = "email"
		}
		if parts[0] == "uuid" || parts[0] == "uuid4" {
			schema["format"] = "uuid"
		}
		if len(parts) < 2 {
			continue
		}
		if parts[0] == "oneof" {
			values := []any{}
			for _, value := range strings.Fields(parts[1]) {
				if schema["type"] == "integer" {
					if n, err := strconv.Atoi(value); err == nil {
						values = append(values, n)
						continue
					}
				}
				values = append(values, value)
			}
			schema["enum"] = values
		}
		if n, err := strconv.ParseFloat(parts[1], 64); err == nil {
			if parts[0] == "min" || parts[0] == "gte" || parts[0] == "max" || parts[0] == "lte" {
				key := "minimum"
				if parts[0] == "max" || parts[0] == "lte" {
					key = "maximum"
				}
				switch schema["type"] {
				case "string":
					if key == "minimum" {
						key = "minLength"
					} else {
						key = "maxLength"
					}
				case "array":
					if key == "minimum" {
						key = "minItems"
					} else {
						key = "maxItems"
					}
				}
				schema[key] = n
			}
		}
	}
}

func (s *schemas) expression(pkg *packages.Package, expr ast.Expr) object {
	// gin.H/map literals have known JSON keys even though their Go type is map[string]any.
	if literal, ok := expr.(*ast.CompositeLit); ok {
		if _, isMap := pkg.TypesInfo.TypeOf(expr).Underlying().(*types.Map); isMap {
			properties := object{}
			for _, element := range literal.Elts {
				if kv, ok := element.(*ast.KeyValueExpr); ok {
					if key, ok := stringValue(pkg, kv.Key); ok {
						properties[key] = s.expression(pkg, kv.Value)
					}
				}
			}
			return object{"type": "object", "properties": properties}
		}
	}
	return s.of(pkg.TypesInfo.TypeOf(expr))
}
