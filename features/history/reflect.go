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

// fieldValuesByColumn returns entity's values for the named storage columns,
// resolved through the same tag rules the diff uses (r3, then db, then bson,
// then snake_case). ok is false when any column matches no field, so a caller
// falls back rather than querying on a value it could not read.
func fieldValuesByColumn(entity any, columns []string) ([]any, bool) {
	v := reflect.ValueOf(entity)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, false
	}

	byColumn := make(map[string]reflect.Value, v.NumField())
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if name := resolveColumnName(field); name != "" && name != "-" {
			byColumn[name] = v.Field(i)
		}
	}

	out := make([]any, 0, len(columns))
	for _, col := range columns {
		fv, ok := byColumn[col]
		if !ok {
			return nil, false
		}
		out = append(out, fv.Interface())
	}
	return out, true
}
