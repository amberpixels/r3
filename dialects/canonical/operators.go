package canonical

import (
	"fmt"

	"github.com/amberpixels/r3"
)

// operatorByName holds the primary names. A dialect may add its own aliases
// (e.g. JSON's "==" for "eq") and resolve them before calling ParseFilterOperator.
var operatorByName = map[string]r3.FilterOperatorSpec{
	"eq":              r3.OperatorEq,
	"ne":              r3.OperatorNe,
	"exists":          r3.OperatorExists,
	"gt":              r3.OperatorGt,
	"gte":             r3.OperatorGte,
	"lt":              r3.OperatorLt,
	"lte":             r3.OperatorLte,
	"between":         r3.OperatorBetween,
	"between_exc":     r3.OperatorBetweenEx,
	"between_exc_inc": r3.OperatorBetweenExInc,
	"between_inc_exc": r3.OperatorBetweenIncEx,
	"in":              r3.OperatorIn,
	"nin":             r3.OperatorNotIn,
	"like":            r3.OperatorLike,
	"notlike":         r3.OperatorNotLike,
	"ilike":           r3.OperatorILike,
	"weekday_in":      r3.OperatorWeekdayIn,
	"tod_between":     r3.OperatorTimeOfDayBetween,
}

// nameByOperator is the reverse of operatorByName.
var nameByOperator = func() map[r3.FilterOperatorSpec]string {
	m := make(map[r3.FilterOperatorSpec]string, len(operatorByName))
	for name, op := range operatorByName {
		// Each operator has exactly one canonical name; keep the first seen.
		if _, exists := m[op]; !exists {
			m[op] = name
		}
	}
	return m
}()

// ParseFilterOperator parses a canonical operator string (the keys of
// operatorByName) to r3.FilterOperatorSpec.
func ParseFilterOperator(s string) (r3.FilterOperatorSpec, error) {
	op, ok := operatorByName[s]
	if !ok {
		return r3.OperatorUnspecified, fmt.Errorf("unknown canonical operator: %q", s)
	}
	return op, nil
}

// FormatFilterOperator returns the canonical string for an r3.FilterOperatorSpec.
func FormatFilterOperator(op r3.FilterOperatorSpec) string {
	if name, ok := nameByOperator[op]; ok {
		return name
	}
	return ""
}
