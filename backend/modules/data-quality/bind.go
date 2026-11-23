package dataquality

import (
	"errors"
	"reflect"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// bindJSON decodes and validates a request body with modulekit.BindJSON, then
// rewrites a validation failure into the exact message the host framework's
// validator produced before extraction. Several handlers return that message
// to clients (INVALID_BODY), so it must stay byte-identical: every failed
// field is listed, one per line, namespaced by the request type's name, e.g.
//
//	Key: 'dqaRunRequest.TableID' Error:Field validation for 'TableID' failed on the 'required' tag
//
// JSON syntax/type errors are returned unchanged, as before.
func bindJSON(c *modulekit.Context, target any) error {
	err := c.BindJSON(target)
	if err == nil || !strings.HasPrefix(err.Error(), "Key: '") {
		return err
	}
	value := reflect.ValueOf(target)
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	failures := requiredFailures(value, value.Type().Name()+".")
	if len(failures) == 0 {
		return err
	}
	return errors.New(strings.Join(failures, "\n"))
}

// requiredFailures lists the `binding:"required"` violations in declaration
// order, descending into nested structs like the framework validator.
func requiredFailures(value reflect.Value, namespace string) []string {
	if value.Kind() != reflect.Struct {
		return nil
	}
	var out []string
	kind := value.Type()
	for i := 0; i < kind.NumField(); i++ {
		field := kind.Field(i)
		if !field.IsExported() {
			continue
		}
		current := value.Field(i)
		for _, rule := range strings.Split(field.Tag.Get("binding"), ",") {
			if strings.TrimSpace(rule) == "required" && !hasBindingValue(current) {
				out = append(out, "Key: '"+namespace+field.Name+"' Error:Field validation for '"+field.Name+"' failed on the 'required' tag")
			}
		}
		nested := current
		if nested.Kind() == reflect.Pointer && !nested.IsNil() {
			nested = nested.Elem()
		}
		if nested.Kind() == reflect.Struct && nested.Type().PkgPath() != "time" {
			out = append(out, requiredFailures(nested, namespace+field.Name+".")...)
		}
	}
	return out
}

func hasBindingValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface, reflect.Chan, reflect.Func:
		return !value.IsNil()
	case reflect.Invalid:
		return false
	default:
		return !value.IsZero()
	}
}
