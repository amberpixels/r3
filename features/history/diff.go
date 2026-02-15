package history

import (
	"reflect"
	"strings"
	"time"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// timeType is used to distinguish time.Time from other structs.
var timeType = reflect.TypeFor[time.Time]()

// Diff computes field-level changes between two entities of the same type.
// It compares exported fields that have `db`, `bson`, or `r3` struct tags,
// using flat dot-notation for nested structs (e.g. "address.city").
//
// Returns nil if there are no differences.
func Diff[T any](old, cur T) []FieldChange {
	oldVal := reflect.ValueOf(old)
	newVal := reflect.ValueOf(cur)

	if oldVal.Kind() == reflect.Pointer {
		oldVal = oldVal.Elem()
	}
	if newVal.Kind() == reflect.Pointer {
		newVal = newVal.Elem()
	}

	var changes []FieldChange
	diffStructFields(oldVal, newVal, "", &changes)
	return changes
}

// DiffWithFields computes changes only for the specified field names.
// Field names should match the column/field names (snake_case), not Go struct names.
func DiffWithFields[T any](old, cur T, fields []string) []FieldChange {
	if len(fields) == 0 {
		return nil
	}

	allChanges := Diff(old, cur)
	if len(allChanges) == 0 {
		return nil
	}

	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}

	var filtered []FieldChange
	for _, c := range allChanges {
		if fieldSet[c.Field] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// DiffCreate generates FieldChanges for a newly created entity.
// Every field gets OldValue=nil, NewValue=<current value>.
func DiffCreate[T any](entity T) []FieldChange {
	val := reflect.ValueOf(entity)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	var changes []FieldChange
	createFields(val, "", &changes)
	return changes
}

// DiffDelete generates FieldChanges for a deleted entity.
// Every field gets OldValue=<current value>, NewValue=nil.
func DiffDelete[T any](entity T) []FieldChange {
	val := reflect.ValueOf(entity)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	var changes []FieldChange
	deleteFields(val, "", &changes)
	return changes
}

// diffStructFields recursively compares two struct values and appends changes.
func diffStructFields(oldVal, newVal reflect.Value, prefix string, changes *[]FieldChange) {
	typ := oldVal.Type()

	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		// Skip relation-like fields (slices, maps)
		if isRelationKind(field.Type) {
			continue
		}

		colName := resolveColumnName(field)
		if colName == "-" || colName == "" {
			continue
		}

		fullName := colName
		if prefix != "" {
			fullName = prefix + "." + colName
		}

		oldField := oldVal.Field(i)
		newField := newVal.Field(i)

		// Handle nested structs (except time.Time) with dot-notation
		if field.Type.Kind() == reflect.Struct && field.Type != timeType {
			diffStructFields(oldField, newField, fullName, changes)
			continue
		}

		// Handle pointer-to-struct (except *time.Time)
		if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct &&
			field.Type.Elem() != timeType {
			switch {
			case oldField.IsNil() && newField.IsNil():
				continue
			case oldField.IsNil():
				createFields(newField.Elem(), fullName, changes)
			case newField.IsNil():
				deleteFields(oldField.Elem(), fullName, changes)
			default:
				diffStructFields(oldField.Elem(), newField.Elem(), fullName, changes)
			}
			continue
		}

		// Compare scalar values
		if !reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
			*changes = append(*changes, FieldChange{
				Field:    fullName,
				OldValue: normalizeValue(oldField),
				NewValue: normalizeValue(newField),
			})
		}
	}
}

// createFields generates FieldChanges for all fields in a struct (for create operations).
func createFields(val reflect.Value, prefix string, changes *[]FieldChange) {
	typ := val.Type()

	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if isRelationKind(field.Type) {
			continue
		}

		colName := resolveColumnName(field)
		if colName == "-" || colName == "" {
			continue
		}

		fullName := colName
		if prefix != "" {
			fullName = prefix + "." + colName
		}

		fieldVal := val.Field(i)

		if field.Type.Kind() == reflect.Struct && field.Type != timeType {
			createFields(fieldVal, fullName, changes)
			continue
		}

		*changes = append(*changes, FieldChange{
			Field:    fullName,
			OldValue: nil,
			NewValue: normalizeValue(fieldVal),
		})
	}
}

// deleteFields generates FieldChanges for all fields in a struct (for delete operations).
func deleteFields(val reflect.Value, prefix string, changes *[]FieldChange) {
	typ := val.Type()

	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if isRelationKind(field.Type) {
			continue
		}

		colName := resolveColumnName(field)
		if colName == "-" || colName == "" {
			continue
		}

		fullName := colName
		if prefix != "" {
			fullName = prefix + "." + colName
		}

		fieldVal := val.Field(i)

		if field.Type.Kind() == reflect.Struct && field.Type != timeType {
			deleteFields(fieldVal, fullName, changes)
			continue
		}

		*changes = append(*changes, FieldChange{
			Field:    fullName,
			OldValue: normalizeValue(fieldVal),
			NewValue: nil,
		})
	}
}

// resolveColumnName determines the storage column name for a struct field.
// Priority: r3 tag > db tag > bson tag > snake_case of Go field name.
func resolveColumnName(field reflect.StructField) string {
	// Try r3 tag first
	if tag := field.Tag.Get("r3"); tag != "" {
		name := tagFirstPart(tag)
		if name == "-" {
			return "-"
		}
		// Skip relation tags
		if strings.HasPrefix(name, "rel:") {
			return ""
		}
		// Skip known keywords that aren't column names
		if name != "pk" && name != "soft_delete" && name != "" {
			return name
		}
	}

	// Try db tag
	if tag := field.Tag.Get("db"); tag != "" {
		name := tagFirstPart(tag)
		if name == "-" {
			return "-"
		}
		if name != "" {
			return name
		}
	}

	// Try bson tag
	if tag := field.Tag.Get("bson"); tag != "" {
		name := tagFirstPart(tag)
		if name == "-" {
			return "-"
		}
		if name != "" {
			return name
		}
	}

	// Fallback: snake_case of Go field name
	return r3utils.ToSnakeCase(field.Name)
}

// tagFirstPart returns the first comma-separated part of a struct tag value.
func tagFirstPart(tag string) string {
	if name, _, ok := strings.Cut(tag, ","); ok {
		return name
	}
	return tag
}

// normalizeValue converts a reflect.Value to a plain Go value suitable for JSON serialization.
// Nil pointers become nil. Everything else becomes its Interface() value.
func normalizeValue(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
	}
	return v.Interface()
}

// isRelationKind returns true for types that represent relations (slices, maps)
// which should be skipped during diffing.
func isRelationKind(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice, reflect.Map:
		return true
	default:
		return false
	}
}
