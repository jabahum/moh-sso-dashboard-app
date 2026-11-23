package modulekit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// BindJSON decodes r's JSON body into target, then applies `binding` struct
// tags with the same meaning as the host framework's validator:
//
//   - required: the field must not be its zero value (nil for pointers,
//     slices, maps and interfaces; "" for strings; 0; false).
//   - omitempty: skip the remaining rules when the field is its zero value.
//   - oneof=a b c: the field's string form must be one of the listed values.
//
// Nested structs (and non-nil pointers to structs) are validated recursively.
// An empty body is an error.
func BindJSON(r *http.Request, target any) error {
	if r == nil || r.Body == nil {
		return errors.New("missing request body")
	}
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return err
	}
	return Validate(target)
}

// Validate applies `binding` struct tags to value (see BindJSON).
func Validate(value any) error {
	return validateValue(reflect.ValueOf(value), "")
}

func validateValue(value reflect.Value, path string) error {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	kind := value.Type()
	for i := 0; i < kind.NumField(); i++ {
		field := kind.Field(i)
		if !field.IsExported() {
			continue
		}
		name := path + field.Name
		current := value.Field(i)
		if err := applyRules(field.Tag.Get("binding"), current, name); err != nil {
			return err
		}
		if err := validateValue(current, name+"."); err != nil {
			return err
		}
	}
	return nil
}

func applyRules(tag string, value reflect.Value, name string) error {
	if tag == "" || tag == "-" {
		return nil
	}
	for _, rule := range strings.Split(tag, ",") {
		rule = strings.TrimSpace(rule)
		switch {
		case rule == "":
		case rule == "required":
			if !hasValue(value) {
				return fmt.Errorf("Key: '%s' Error:Field validation for '%s' failed on the 'required' tag", name, lastSegment(name))
			}
		case rule == "omitempty":
			if !hasValue(value) {
				return nil
			}
		case strings.HasPrefix(rule, "oneof="):
			allowed := strings.Fields(strings.TrimPrefix(rule, "oneof="))
			actual := stringForm(value)
			match := false
			for _, candidate := range allowed {
				if candidate == actual {
					match = true
					break
				}
			}
			if !match {
				return fmt.Errorf("Key: '%s' Error:Field validation for '%s' failed on the 'oneof' tag", name, lastSegment(name))
			}
		default:
			return fmt.Errorf("unsupported binding rule %q on %s", rule, name)
		}
	}
	return nil
}

func hasValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface, reflect.Chan, reflect.Func:
		return !value.IsNil()
	case reflect.Invalid:
		return false
	default:
		return !value.IsZero()
	}
}

func stringForm(value reflect.Value) string {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		return value.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10)
	default:
		return fmt.Sprint(value.Interface())
	}
}

func lastSegment(name string) string {
	if index := strings.LastIndex(name, "."); index >= 0 {
		return name[index+1:]
	}
	return name
}

func formatInt(value int64) string { return strconv.FormatInt(value, 10) }
