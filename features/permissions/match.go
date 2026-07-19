package permissions

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/amberpixels/k1/reflectish"
	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
)

// entityMatchesFilters reports whether entity satisfies ALL of the filters
// (top-level AND). It enforces Scoper row-level filters on single-entity Get,
// where the backend looks up by PK and applies no query filters; field names
// resolve via the same column derivation as the engines (r3/db tags, snake_case
// fallback), so a scope behaves the same on List and Get. An unevaluable filter
// (unknown field or unsupported operator) returns an error; access-control
// callers must treat any error as "not in scope" (fail closed).
func entityMatchesFilters(entity any, filters r3.Filters) (bool, error) {
	cols := columnValues(entity)
	for _, f := range filters {
		ok, err := matchFilter(cols, f)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// columnValues maps column name -> field value for entity's exported fields,
// using the same tag conventions as the engines.
func columnValues(entity any) map[string]any {
	rv := reflectish.IndirectDeep(reflect.ValueOf(entity))
	out := map[string]any{}
	if rv.Kind() != reflect.Struct {
		return out
	}
	rt := rv.Type()
	for i := range rt.NumField() {
		sf := rt.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}
		ct := r3tag.ParseColumnTag(sf)
		if ct.Skip {
			continue
		}
		// Unwrap pointer fields so a nullable column (e.g. a *int64 foreign key)
		// compares against scalar filter values the same way the SQL engines do.
		// A nil pointer becomes nil (matching no scalar value; use the Exists
		// operator to test presence).
		out[ct.Column] = reflectish.IndirectInterface(rv.Field(i))
	}
	return out
}

// matchFilter evaluates a single FilterSpec (leaf or AND/OR group) against the
// resolved column values.
func matchFilter(cols map[string]any, f *r3.FilterSpec) (bool, error) {
	if f == nil {
		return true, nil
	}

	if len(f.And) > 0 {
		for _, child := range f.And {
			ok, err := matchFilter(cols, child)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}

	if len(f.Or) > 0 {
		for _, child := range f.Or {
			ok, err := matchFilter(cols, child)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}

	// A relationship ("has") filter needs the database, not in-memory columns.
	// Fail closed so it can never silently match-all (the decorator routes
	// relationship-scoped Get through a query).
	if f.Relation != "" {
		return false, fmt.Errorf(
			"permissions: relationship scope filter %q must be evaluated by the database, not in memory",
			f.Relation,
		)
	}

	if f.Field == nil {
		return true, nil
	}

	col := f.Field.String()
	val, ok := cols[col]
	if !ok {
		return false, fmt.Errorf("permissions: scope filter references unknown field %q", col)
	}
	return matchOperator(val, f.Operator, f.Value)
}

// matchOperator evaluates a single operator against a field value. The
// comparison semantics mirror the file engine's in-memory evaluation.
func matchOperator(fieldVal any, op r3.FilterOperatorSpec, filterVal any) (bool, error) {
	switch op {
	case r3.OperatorEq, r3.OperatorUnspecified:
		return compareEqual(fieldVal, filterVal), nil
	case r3.OperatorNe:
		return !compareEqual(fieldVal, filterVal), nil
	case r3.OperatorExists:
		return !isZeroValue(fieldVal), nil
	case r3.OperatorGt:
		return compareOrdered(fieldVal, filterVal) > 0, nil
	case r3.OperatorGte:
		return compareOrdered(fieldVal, filterVal) >= 0, nil
	case r3.OperatorLt:
		return compareOrdered(fieldVal, filterVal) < 0, nil
	case r3.OperatorLte:
		return compareOrdered(fieldVal, filterVal) <= 0, nil
	case r3.OperatorIn:
		return valueIn(fieldVal, filterVal), nil
	case r3.OperatorNotIn:
		return !valueIn(fieldVal, filterVal), nil
	case r3.OperatorBetween:
		return matchBetween(fieldVal, filterVal, true, true)
	case r3.OperatorBetweenEx:
		return matchBetween(fieldVal, filterVal, false, false)
	case r3.OperatorBetweenExInc:
		return matchBetween(fieldVal, filterVal, false, true)
	case r3.OperatorBetweenIncEx:
		return matchBetween(fieldVal, filterVal, true, false)
	case r3.OperatorLike:
		return matchLike(fieldVal, filterVal, false), nil
	case r3.OperatorNotLike:
		return !matchLike(fieldVal, filterVal, false), nil
	case r3.OperatorILike:
		return matchLike(fieldVal, filterVal, true), nil
	case r3.OperatorWeekdayIn:
		return matchTimePattern(fieldVal, filterVal, r3.EvalWeekdayIn)
	case r3.OperatorTimeOfDayBetween:
		return matchTimePattern(fieldVal, filterVal, r3.EvalTimeOfDayBetween)
	default:
		return false, fmt.Errorf("permissions: unsupported scope filter operator %v", op)
	}
}

// matchTimePattern applies a recurring-time operator to a scope field value. A
// NULL time value never matches (SQL semantics), mirroring the file engine.
func matchTimePattern(fieldVal, filterVal any, eval func(time.Time, any) (bool, error)) (bool, error) {
	if fieldVal == nil {
		return false, nil
	}
	if rv := reflect.ValueOf(fieldVal); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return false, nil
	}
	t, ok := toTime(fieldVal)
	if !ok {
		return false, fmt.Errorf("permissions: time-pattern scope requires a time field, got %T", fieldVal)
	}
	return eval(t, filterVal)
}

func compareEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if reflect.DeepEqual(a, b) {
		return true
	}
	if af, aok := toFloat64(a); aok {
		if bf, bok := toFloat64(b); bok {
			return af == bf
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareOrdered(a, b any) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}
	if at, ok := toTime(a); ok {
		if bt, ok := toTime(b); ok {
			switch {
			case at.Before(bt):
				return -1
			case at.After(bt):
				return 1
			default:
				return 0
			}
		}
	}
	if af, aok := toFloat64(a); aok {
		if bf, bok := toFloat64(b); bok {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
	}
	as, bs := fmt.Sprintf("%v", a), fmt.Sprintf("%v", b)
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}

func matchBetween(fieldVal, filterVal any, inclusiveLow, inclusiveHigh bool) (bool, error) {
	low, high, err := r3.ExtractBetweenBounds(filterVal)
	if err != nil {
		return false, err
	}
	cmpLow := compareOrdered(fieldVal, low)
	cmpHigh := compareOrdered(fieldVal, high)
	lowOK := cmpLow > 0 || (inclusiveLow && cmpLow == 0)
	highOK := cmpHigh < 0 || (inclusiveHigh && cmpHigh == 0)
	return lowOK && highOK, nil
}

func valueIn(fieldVal, filterVal any) bool {
	rv := reflect.ValueOf(filterVal)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return compareEqual(fieldVal, filterVal)
	}
	for i := range rv.Len() {
		if compareEqual(fieldVal, rv.Index(i).Interface()) {
			return true
		}
	}
	return false
}

func matchLike(fieldVal, filterVal any, caseInsensitive bool) bool {
	s := fmt.Sprintf("%v", fieldVal)
	pattern := fmt.Sprintf("%v", filterVal)
	if caseInsensitive {
		s = strings.ToLower(s)
		pattern = strings.ToLower(pattern)
	}
	return likeMatch(s, pattern, 0, 0)
}

func likeMatch(s, p string, si, pi int) bool {
	for pi < len(p) {
		switch p[pi] {
		case '%':
			for pi < len(p) && p[pi] == '%' {
				pi++
			}
			if pi == len(p) {
				return true
			}
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

func toTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t != nil {
			return *t, true
		}
	}
	return time.Time{}, false
}

func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsZero()
}
