package r3

import (
	"fmt"
	"reflect"
	"slices"
	"time"
)

// Time-component filter operators match a time-valued field against a recurring
// weekly wall-clock pattern that no combination of range operators can express:
// a weekday set (an infinite union of days) or a minute-of-day window (an
// infinite union of intervals). They are deliberately narrow, closed-form
// primitives - the core knows how to compare extracted time components but has
// no vocabulary of its own. The keyword layer that turns "weekends" or
// "mornings" into these operators lives in dialects/when, exactly like the
// Django-style parser turns "age__gte=18" into an Fop.
//
// Semantics contract: both operators evaluate the field's stored wall-clock
// value as-is. No engine performs timezone conversion. Apps that need locality
// (per-row timezones) store a local wall-clock column and filter on that. This
// is what keeps the feature backend-portable and the core timezone-free.
//
// One portability caveat follows from "as-is": each engine reads whatever the
// stored value's clock is. The in-memory engines use the Go time.Time's own
// location (t.Weekday()/t.Hour()); Mongo reads the stored BSON date as UTC. A
// value carrying a non-UTC zone can therefore match different windows on Mongo
// than on the file engine. Store UTC (or a normalized local wall-clock) to keep
// results identical across backends.

const (
	minutesPerHour = 60
	minutesPerDay  = 24 * minutesPerHour
)

// WeekdayIn matches rows whose time field's weekday is one of days. Weekdays use
// Go's numbering (time.Sunday=0 .. time.Saturday=6); every lowering converts to
// its backend's convention at the edge (e.g. Mongo's $dayOfWeek is 1..7).
func WeekdayIn(field string, days ...time.Weekday) *FilterSpec {
	return Fop(NewFieldSpec(field), OperatorWeekdayIn, days)
}

// TimeOfDayBetween matches rows whose time field's minute-of-day lies in
// [loMin, hiMin). Bounds are minutes from midnight. loMin > hiMin means the
// window wraps midnight (e.g. 1320, 300 = 22:00-05:00). Minute precision matches
// schedule.TimeOfDay and sidesteps seconds edge-cases. The value mirrors
// [Between] ([]any{lo, hi}) so it round-trips through every serialization
// dialect as a plain 2-element array.
func TimeOfDayBetween(field string, loMin, hiMin int) *FilterSpec {
	return Fop(NewFieldSpec(field), OperatorTimeOfDayBetween, []any{loMin, hiMin})
}

// WeekdaysValue normalizes a [WeekdayIn] filter value into a []time.Weekday. It
// accepts the shapes that survive a serialization round-trip: []time.Weekday
// (the native sugar), []int, and []float64 (JSON decodes numbers as float64).
// Each element must be in 0..6 (Go's Sunday..Saturday). Every engine and dialect
// that lowers OperatorWeekdayIn goes through this one door so the accepted wire
// forms stay consistent.
func WeekdaysValue(value any) ([]time.Weekday, error) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("weekday_in requires a slice of weekdays, got %T", value)
	}

	days := make([]time.Weekday, 0, rv.Len())
	for i := range rv.Len() {
		elem := rv.Index(i).Interface()
		n, ok := toIntExact(elem)
		if !ok {
			return nil, fmt.Errorf("weekday_in: element %d is not an integer weekday (%T)", i, elem)
		}
		if n < int(time.Sunday) || n > int(time.Saturday) {
			return nil, fmt.Errorf("weekday_in: weekday %d out of range 0..6 (Sunday..Saturday)", n)
		}
		days = append(days, time.Weekday(n))
	}
	return days, nil
}

// TimeOfDayBounds normalizes a [TimeOfDayBetween] value into its [lo, hi) minute
// bounds. The value is the 2-element [lo, hi] the sugar produces; bounds may
// arrive as int or float64 (JSON). Each must be a whole minute in 0..1440 (1440
// is the exclusive end-of-day upper bound). Reusing [ExtractBetweenBounds] keeps
// the shape identical to Between.
func TimeOfDayBounds(value any) (int, int, error) {
	loV, hiV, err := ExtractBetweenBounds(value)
	if err != nil {
		return 0, 0, fmt.Errorf("tod_between: %w", err)
	}

	lo, ok := toIntExact(loV)
	if !ok {
		return 0, 0, fmt.Errorf("tod_between: low bound is not an integer minute (%T)", loV)
	}
	hi, ok := toIntExact(hiV)
	if !ok {
		return 0, 0, fmt.Errorf("tod_between: high bound is not an integer minute (%T)", hiV)
	}
	if lo < 0 || lo > minutesPerDay || hi < 0 || hi > minutesPerDay {
		return 0, 0, fmt.Errorf("tod_between: minute bounds must be within 0..1440, got [%d, %d]", lo, hi)
	}
	return lo, hi, nil
}

// EvalWeekdayIn reports whether t's wall-clock weekday is in the WeekdayIn value.
// In-memory engines (the file engine, the permissions matcher) call it so the
// membership and value-normalization rules live in exactly one place.
func EvalWeekdayIn(t time.Time, value any) (bool, error) {
	days, err := WeekdaysValue(value)
	if err != nil {
		return false, err
	}
	return slices.Contains(days, t.Weekday()), nil
}

// EvalTimeOfDayBetween reports whether t's wall-clock minute-of-day falls in the
// TimeOfDayBetween window [lo, hi). lo > hi wraps midnight (match m >= lo OR
// m < hi); lo == hi matches nothing.
func EvalTimeOfDayBetween(t time.Time, value any) (bool, error) {
	lo, hi, err := TimeOfDayBounds(value)
	if err != nil {
		return false, err
	}
	m := t.Hour()*minutesPerHour + t.Minute()
	if lo <= hi {
		return m >= lo && m < hi, nil
	}
	// Wrapping window: [lo, 1440) ∪ [0, hi).
	return m >= lo || m < hi, nil
}

// toIntExact converts an integer-valued number to int. Floats (JSON's number
// form) are accepted only when they carry no fractional part.
func toIntExact(v any) (int, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		//nolint:gosec // weekday (0..6) and minute (0..1440) values are range-checked by callers
		return int(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if f != float64(int64(f)) {
			return 0, false
		}
		return int(f), true
	default:
		return 0, false
	}
}
