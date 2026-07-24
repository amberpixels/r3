package i18n

import (
	"fmt"
	"reflect"
	"strings"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// resolveFields maps requested storage names (e.g. "title") to struct field
// indexes on T, matching by the first token of the `db` tag or the snake_case Go
// name when untagged. Only string fields are translatable; anything else is a
// configuration error.
func resolveFields[T any](names []string) (map[string]int, error) {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil || t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: %T is not a struct", ErrNotTranslatable, zero)
	}

	byStorageName := make(map[string]int, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("db"), ",")
		if name == "" || name == "-" {
			name = r3utils.ToSnakeCase(f.Name)
		}
		byStorageName[name] = i
	}

	resolved := make(map[string]int, len(names))
	for _, name := range names {
		idx, ok := byStorageName[name]
		if !ok {
			return nil, fmt.Errorf("%w: no field %q on %s", ErrNotTranslatable, name, t.Name())
		}
		if t.Field(idx).Type.Kind() != reflect.String {
			return nil, fmt.Errorf("%w: field %q on %s is not a string", ErrNotTranslatable, name, t.Name())
		}
		resolved[name] = idx
	}
	return resolved, nil
}

// fieldValue reads the string value of the struct field at idx.
func fieldValue[T any](entity T, idx int) string {
	return reflect.ValueOf(entity).Field(idx).String()
}

// setFieldValue writes val into the struct field at idx via the pointer.
func setFieldValue[T any](entity *T, idx int, val string) {
	reflect.ValueOf(entity).Elem().Field(idx).SetString(val)
}
