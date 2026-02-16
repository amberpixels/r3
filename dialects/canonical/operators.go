package canonical

import (
	"fmt"

	"github.com/amberpixels/r3"
)

// Canonical operator strings used by all serialization dialects.
// These are the primary names; dialects may define additional aliases
// (e.g., JSON's "==" for "eq") that they resolve before calling
// ParseFilterOperator.
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
}

// nameByOperator is the reverse mapping: r3 operator → canonical string.
var nameByOperator = func() map[r3.FilterOperatorSpec]string {
	m := make(map[r3.FilterOperatorSpec]string, len(operatorByName))
	for name, op := range operatorByName {
		// If an operator already has a name, keep the first one.
		// Since we control the map above, every operator has exactly one
		// canonical name (no duplicates for the same value).
		if _, exists := m[op]; !exists {
			m[op] = name
		}
	}
	return m
}()

// ParseFilterOperator parses a canonical operator string to r3.FilterOperatorSpec.
// Recognized strings: "eq", "ne", "exists", "gt", "gte", "lt", "lte",
// "between", "between_exc", "between_exc_inc", "between_inc_exc",
// "in", "nin", "like", "notlike", "ilike".
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
