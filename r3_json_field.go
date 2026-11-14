package r3

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// JSON field form. A field naming a key inside a JSON column is encoded in the
// FieldSpec string as `column->seg->seg`, optionally suffixed `~number`. The
// separator is one ValidateIdentifier rejects, so a code path that does not know
// the form fails loudly instead of reading it as a column, and it holds no dot,
// so it is never mistaken for a relation path. `~` is URL-unreserved and no url
// sort or filter format splits on it.
const (
	jsonPathSep      = "->"
	jsonNumberSuffix = "~number"
)

// ErrInvalidJSONField is returned when a JSON field (see [JSONField]) is
// malformed: a column or path segment that is not a plain identifier, an empty
// path, or an unknown suffix. It wraps [ErrInvalidIdentifier], so existing
// errors.Is checks for unsafe field names keep matching.
var ErrInvalidJSONField = fmt.Errorf("%w: invalid JSON field", ErrInvalidIdentifier)

// ErrJSONFieldNotJSONColumn is returned by schema validation when a JSON field
// names a column the schema does not declare as JSON. It is caught when the
// query is built rather than surfacing as a database error.
var ErrJSONFieldNotJSONColumn = errors.New("JSON field on a non-JSON column")

// ErrJSONFieldValueType is returned when a JSON field's filter value does not
// match its comparison mode: a numeric value on a text-mode key (use
// [FieldSpec.AsNumber]) or a non-numeric value on a number-mode key. Each SQL
// flavor compares a text extraction with a number differently, so a mismatch is
// rejected rather than left to behave differently per backend.
var ErrJSONFieldValueType = errors.New("JSON field value type mismatch")

// ErrJSONFieldNotSupported is returned when a backend cannot render a JSON field
// (e.g. a SQL flavor with no JSON-path hook, or the django url syntax).
var ErrJSONFieldNotSupported = errors.New("r3: JSON field not supported by this backend")

// JSONField names a key inside a JSON column, for filters, sorts and group-by:
//
//	r3.JSONField("profile", "breed")                 // profile -> breed
//	r3.JSONField("profile", "vaccinations", "count") // nested
//
// The key compares as text; call [FieldSpec.AsNumber] for a numeric key. A
// missing key reads as NULL: Eq does not match it, Eq(nil) does. The column must
// be declared as a JSON column in the schema. Segments are validated when the
// query is built, never interpolated unchecked.
func JSONField(column string, path ...string) *FieldSpec {
	return NewFieldSpec(column + jsonPathSep + strings.Join(path, jsonPathSep))
}

// AsNumber returns a copy of a JSON field that compares and sorts numerically.
// Without it a JSON key compares as text, which orders "10" before "9". Calling
// it on a plain field yields a field that fails validation.
func (f *FieldSpec) AsNumber() *FieldSpec {
	s := f.String()
	if strings.HasSuffix(s, jsonNumberSuffix) {
		return NewFieldSpec(s)
	}
	return NewFieldSpec(s + jsonNumberSuffix)
}

// IsJSONField reports whether f uses the JSON field form (valid or not). Code
// that must route a JSON field differently checks this, then validates.
func (f *FieldSpec) IsJSONField() bool {
	return f != nil && strings.Contains(string(*f), jsonPathSep)
}

// JSONPath parses a JSON field into its column, its key path and whether it
// compares numerically; the last result is false for a plain field. It does
// not validate; see [ValidateJSONField].
func (f *FieldSpec) JSONPath() (string, []string, bool, bool) {
	if !f.IsJSONField() {
		return "", nil, false, false
	}
	s := string(*f)
	numeric := strings.HasSuffix(s, jsonNumberSuffix)
	s = strings.TrimSuffix(s, jsonNumberSuffix)
	parts := strings.Split(s, jsonPathSep)
	return parts[0], parts[1:], numeric, true
}

// ValidateJSONField checks a JSON field: the column and every path segment
// must match [a-zA-Z_][a-zA-Z0-9_]*, the path must be non-empty, and the only
// suffix allowed is the one [FieldSpec.AsNumber] adds. Array indexes and keys
// on a related entity's column are not supported.
func ValidateJSONField(s string) error {
	col, path, _, ok := NewFieldSpec(s).JSONPath()
	if !ok || len(path) == 0 {
		return fmt.Errorf("%w: %q", ErrInvalidJSONField, s)
	}
	if !isValidIdentifierSegment(col) {
		return fmt.Errorf("%w: column in %q", ErrInvalidJSONField, s)
	}
	for _, seg := range path {
		if !isValidIdentifierSegment(seg) {
			return fmt.Errorf("%w: key %q in %q", ErrInvalidJSONField, seg, s)
		}
	}
	return nil
}

// ValidateFieldName accepts a plain or dotted identifier ([ValidateIdentifier])
// or a valid JSON field ([ValidateJSONField]). Read paths that may carry a JSON
// field (filters, sorts, group-by) validate with it.
func ValidateFieldName(s string) error {
	if NewFieldSpec(s).IsJSONField() {
		return ValidateJSONField(s)
	}
	return ValidateIdentifier(s)
}

// ValidateJSONFieldValue checks that a filter value matches a JSON field's
// comparison mode (see [ErrJSONFieldValueType]). nil (IS NULL) and Exists are
// always allowed, and slices are checked element-wise. A number-mode key also
// accepts a numeric string, since the url dialects carry every value as text;
// see [JSONFieldNumericArg].
func ValidateJSONFieldValue(f *FieldSpec, op FilterOperatorSpec, value any) error {
	_, _, numeric, ok := f.JSONPath()
	if !ok || value == nil || op == OperatorExists {
		return nil
	}
	for _, v := range flattenFilterValue(value) {
		if v == nil {
			continue
		}
		isNum := isNumericValue(v)
		switch {
		case numeric && !isNum && !isNumericString(v):
			return fmt.Errorf("%w: %q compares as a number, got %T", ErrJSONFieldValueType, f.String(), v)
		case !numeric && isNum:
			return fmt.Errorf("%w: %q compares as text, got %T (use AsNumber())", ErrJSONFieldValueType, f.String(), v)
		}
	}
	return nil
}

// ValidateJSONFieldFilters walks filters (AND/OR groups included) and applies
// [ValidateJSONFieldValue] to every JSON-field leaf. It is structural, so it
// runs with or without a schema.
func ValidateJSONFieldFilters(filters Filters) error {
	for _, f := range filters {
		if f == nil || f.Relation != "" {
			continue
		}
		if err := ValidateJSONFieldFilters(f.And); err != nil {
			return err
		}
		if err := ValidateJSONFieldFilters(f.Or); err != nil {
			return err
		}
		if f.Field.IsJSONField() {
			if err := ValidateJSONFieldValue(f.Field, f.Operator, f.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

// JSONFieldNumericArg converts a numeric string to float64 for binding against
// a number-mode JSON key; any other value is returned unchanged. Backends call
// it on number-mode args so a value parsed from a URL compares as a number.
func JSONFieldNumericArg(v any) any {
	if s, ok := v.(string); ok {
		if n, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return n
		}
	}
	return v
}

// flattenFilterValue expands a slice or array value into its elements (a
// []byte stays one scalar); any other value is a one-element list.
func flattenFilterValue(value any) []any {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return []any{value}
		}
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = rv.Index(i).Interface()
		}
		return out
	default:
		return []any{value}
	}
}

func isNumericValue(v any) bool {
	switch reflect.ValueOf(v).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func isNumericString(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

// JSONFieldNumericValue applies [JSONFieldNumericArg] to a filter value, element
// by element for a slice (In, Between), returning a []any in that case.
func JSONFieldNumericValue(value any) any {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return JSONFieldNumericArg(value)
	}
	vals := flattenFilterValue(value)
	for i, v := range vals {
		vals[i] = JSONFieldNumericArg(v)
	}
	return vals
}
