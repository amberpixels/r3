package enginefile

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/amberpixels/r3"
)

// filterEntities returns only the entities matching every filter.
func filterEntities[T any](entities []T, filters r3.Filters, meta *StructMeta) ([]T, error) {
	if len(filters) == 0 {
		return entities, nil
	}

	var result []T
	for _, entity := range entities {
		match, err := matchesFilters(entity, filters, meta)
		if err != nil {
			return nil, err
		}
		if match {
			result = append(result, entity)
		}
	}
	return result, nil
}

// matchesFilters checks if an entity matches ALL top-level filters (implicit AND).
func matchesFilters[T any](entity T, filters r3.Filters, meta *StructMeta) (bool, error) {
	for _, filter := range filters {
		match, err := matchesFilter(entity, filter, meta)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

// matchesFilter evaluates a single FilterSpec against an entity.
func matchesFilter[T any](entity T, filter *r3.FilterSpec, meta *StructMeta) (bool, error) {
	if filter == nil {
		return true, nil
	}

	if len(filter.And) > 0 {
		for _, f := range filter.And {
			match, err := matchesFilter(entity, f, meta)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil
			}
		}
		return true, nil
	}

	if len(filter.Or) > 0 {
		for _, f := range filter.Or {
			match, err := matchesFilter(entity, f, meta)
			if err != nil {
				return false, err
			}
			if match {
				return true, nil
			}
		}
		return false, nil
	}

	// Leaf: Field/Operator/Value.
	if filter.Field == nil {
		return true, nil
	}

	fieldName := filter.Field.String()
	fieldVal, ok := meta.GetFieldValue(entity, fieldName)
	if !ok {
		return false, fmt.Errorf("unknown field %q in filter", fieldName)
	}

	return evaluateOperator(fieldVal, filter.Operator, filter.Value)
}

// evaluateOperator evaluates a filter operator against a field value.
func evaluateOperator(fieldVal any, op r3.FilterOperatorSpec, filterVal any) (bool, error) {
	switch op {
	case r3.OperatorEq, r3.OperatorUnspecified:
		return compareEqual(fieldVal, filterVal), nil

	case r3.OperatorNe:
		return !compareEqual(fieldVal, filterVal), nil

	case r3.OperatorExists:
		// exists == SQL "IS NOT NULL": present non-null value. A non-pointer zero
		// (0, "", false, zero time) is a real value and DOES exist; only an
		// absent field or a nil pointer is NULL.
		return !isNullValue(fieldVal), nil

	case r3.OperatorGt:
		return compareOrdered(fieldVal, filterVal) > 0, nil

	case r3.OperatorGte:
		return compareOrdered(fieldVal, filterVal) >= 0, nil

	case r3.OperatorLt:
		return compareOrdered(fieldVal, filterVal) < 0, nil

	case r3.OperatorLte:
		return compareOrdered(fieldVal, filterVal) <= 0, nil

	case r3.OperatorBetween:
		return evaluateBetween(fieldVal, filterVal, true, true)

	case r3.OperatorBetweenEx:
		return evaluateBetween(fieldVal, filterVal, false, false)

	case r3.OperatorBetweenExInc:
		return evaluateBetween(fieldVal, filterVal, false, true)

	case r3.OperatorBetweenIncEx:
		return evaluateBetween(fieldVal, filterVal, true, false)

	case r3.OperatorIn:
		return valueIn(fieldVal, filterVal), nil

	case r3.OperatorNotIn:
		return !valueIn(fieldVal, filterVal), nil

	case r3.OperatorLike:
		return matchLike(fieldVal, filterVal, false)

	case r3.OperatorNotLike:
		ok, err := matchLike(fieldVal, filterVal, false)
		return !ok, err

	case r3.OperatorILike:
		return matchLike(fieldVal, filterVal, true)

	case r3.OperatorWeekdayIn:
		return evalTimePattern(fieldVal, filterVal, r3.EvalWeekdayIn)

	case r3.OperatorTimeOfDayBetween:
		return evalTimePattern(fieldVal, filterVal, r3.EvalTimeOfDayBetween)

	default:
		return false, fmt.Errorf("unsupported filter operator: %d", op)
	}
}

// evalTimePattern applies a recurring-time operator (weekday_in, tod_between) to
// a field value. A NULL time field never matches - the same as SQL, where
// weekday(NULL)/hour(NULL) is NULL - rather than failing the whole query; a
// present value that is not a time is a genuine misuse and errors.
func evalTimePattern(fieldVal, filterVal any, eval func(time.Time, any) (bool, error)) (bool, error) {
	if isNullValue(fieldVal) {
		return false, nil
	}
	t, ok := toTime(fieldVal)
	if !ok {
		return false, fmt.Errorf("time-pattern operator requires a time field, got %T", fieldVal)
	}
	return eval(t, filterVal)
}

// compareEqual checks equality with type coercion for common numeric mismatches.
func compareEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if reflect.DeepEqual(a, b) {
		return true
	}

	// Numeric coercion first (JSON decodes numbers as float64), then a string
	// fallback for everything else.
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)
	if aOk && bOk {
		return aFloat == bFloat
	}

	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// compareOrdered performs ordered comparison. Returns -1, 0, or 1.
func compareOrdered(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	aTime, aIsTime := toTime(a)
	bTime, bIsTime := toTime(b)
	if aIsTime && bIsTime {
		if aTime.Before(bTime) {
			return -1
		}
		if aTime.After(bTime) {
			return 1
		}
		return 0
	}

	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)
	if aOk && bOk {
		if aFloat < bFloat {
			return -1
		}
		if aFloat > bFloat {
			return 1
		}
		return 0
	}

	// Fall back to string ordering.
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	if aStr < bStr {
		return -1
	}
	if aStr > bStr {
		return 1
	}
	return 0
}

// evaluateBetween reports whether fieldVal lies between two bounds. filterVal
// must be a 2-element slice/array [low, high].
func evaluateBetween(fieldVal, filterVal any, inclusiveLow, inclusiveHigh bool) (bool, error) {
	rv := reflect.ValueOf(filterVal)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return false, fmt.Errorf("between operator requires a slice/array value with 2 elements, got %T", filterVal)
	}
	if rv.Len() != 2 {
		return false, fmt.Errorf("between operator requires exactly 2 elements, got %d", rv.Len())
	}

	low := rv.Index(0).Interface()
	high := rv.Index(1).Interface()

	cmpLow := compareOrdered(fieldVal, low)
	cmpHigh := compareOrdered(fieldVal, high)

	lowOK := cmpLow > 0 || (inclusiveLow && cmpLow == 0)
	highOK := cmpHigh < 0 || (inclusiveHigh && cmpHigh == 0)

	return lowOK && highOK, nil
}

// valueIn checks if fieldVal is contained in filterVal (expected to be a slice).
func valueIn(fieldVal, filterVal any) bool {
	rv := reflect.ValueOf(filterVal)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return false
	}
	for i := range rv.Len() {
		if compareEqual(fieldVal, rv.Index(i).Interface()) {
			return true
		}
	}
	return false
}

// matchLike implements SQL LIKE matching: % matches any run of characters, _
// matches a single character.
func matchLike(fieldVal, filterVal any, caseInsensitive bool) (bool, error) {
	fieldStr := fmt.Sprintf("%v", fieldVal)
	pattern := fmt.Sprintf("%v", filterVal)

	if caseInsensitive {
		fieldStr = strings.ToLower(fieldStr)
		pattern = strings.ToLower(pattern)
	}

	return matchLikePattern(fieldStr, pattern), nil
}

// matchLikePattern matches a string against a SQL LIKE pattern.
func matchLikePattern(s, pattern string) bool {
	return likeMatch(s, pattern, 0, 0)
}

func likeMatch(s, p string, si, pi int) bool {
	for pi < len(p) {
		switch p[pi] {
		case '%':
			// Skip consecutive %
			for pi < len(p) && p[pi] == '%' {
				pi++
			}
			if pi == len(p) {
				return true
			}
			// Try matching the rest of the pattern from every position
			for si <= len(s) {
				if likeMatch(s, p, si, pi) {
					return true
				}
				si++
			}
			return false
		case '_':
			if si >= len(s) {
				return false
			}
			si++
			pi++
		default:
			if si >= len(s) || s[si] != p[pi] {
				return false
			}
			si++
			pi++
		}
	}
	return si == len(s)
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v any) (float64, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	default:
		return 0, false
	}
}

// toTime attempts to convert a value to time.Time.
func toTime(v any) (time.Time, bool) {
	if t, ok := v.(time.Time); ok {
		return t, true
	}
	if t, ok := v.(*time.Time); ok && t != nil {
		return *t, true
	}
	return time.Time{}, false
}

// isNullValue reports whether a value represents SQL NULL: an absent field, a
// nil interface, or a nil pointer/map/slice. A present non-pointer zero (0, "",
// false, the zero time) is NOT null - it is a real value, so nulls-ordering and
// the `exists` operator treat it as present, matching SQL and Mongo. (A nil
// pointer arrives here as a typed nil whose reflect Kind is Pointer.)
func isNullValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}
