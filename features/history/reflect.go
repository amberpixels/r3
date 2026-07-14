package history

import (
	"fmt"
	"reflect"
)

// typeName returns the unqualified struct name of T (e.g. "Order", "OrderItem").
func typeName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// extractFieldByName returns the named Go struct field as a string, or "" when
// absent or zero-valued.
func extractFieldByName(entity any, fieldName string) string {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	f := v.FieldByName(fieldName)
	if !f.IsValid() {
		return ""
	}
	return fmt.Sprint(f.Interface())
}
