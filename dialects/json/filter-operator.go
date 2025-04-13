package r3json

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidFilterOperator = errors.New("invalid filter operator")

type FilterOperator int8

const (
	OperatorUnspecified FilterOperator = iota
	OperatorEq
	OperatorNe
	OperatorExists
	OperatorGt
	OperatorGte
	OperatorLt
	OperatorLte
	OperatorBetween
	OperatorBetweenEx
	OperatorBetweenExInc
	OperatorBetweenIncEx
	OperatorIn
	OperatorNotIn
	OperatorLike
	OperatorNotLike
	OperatorILike
)

// Enum value maps for Operator.
var (
	FilterOperatorNames = map[FilterOperator]string{
		OperatorUnspecified:  "",
		OperatorEq:           "eq",
		OperatorNe:           "ne",
		OperatorExists:       "exists",
		OperatorGt:           "gt",
		OperatorGte:          "gte",
		OperatorLt:           "lt",
		OperatorLte:          "lte",
		OperatorBetween:      "between",
		OperatorBetweenEx:    "between_exc",
		OperatorBetweenExInc: "between_exc_inc",
		OperatorBetweenIncEx: "between_inc_exc",
		OperatorIn:           "in",
		OperatorNotIn:        "nin",
		OperatorLike:         "like",
		OperatorNotLike:      "notlike",
		OperatorILike:        "ilike",
	}
	FilterOperatorValues = map[string]FilterOperator{
		"":                OperatorUnspecified,
		"eq":              OperatorEq,
		"==":              OperatorEq,
		"ne":              OperatorNe,
		"!=":              OperatorNe,
		"exists":          OperatorExists,
		"gt":              OperatorGt,
		">":               OperatorGt,
		"gte":             OperatorGte,
		">=":              OperatorGte,
		"lt":              OperatorLt,
		"<":               OperatorLt,
		"lte":             OperatorLte,
		"<=":              OperatorLte,
		"between":         OperatorBetween,
		"between_inc":     OperatorBetween,
		"between_exc":     OperatorBetweenEx,
		"between_exc_inc": OperatorBetweenExInc,
		"between_inc_exc": OperatorBetweenIncEx,
		"in":              OperatorIn,
		"nin":             OperatorNotIn,
		"like":            OperatorLike,
		"*=":              OperatorLike,
		"notlike":         OperatorNotLike,
		"!*=":             OperatorNotLike,
		"ilike":           OperatorILike,
		"**=":             OperatorILike,
	}
)

// String is implemented for debugging purposes, so the FilterOperator is a fmt.Stringer.
func (op FilterOperator) String() string { return FilterOperatorNames[op] }

// UnmarshalJSON implements the json.Unmarshaler interface.
// It expects the JSON value to be a string representing the operator.
func (op *FilterOperator) UnmarshalJSON(data []byte) error {
	str := string(data)
	str = strings.Trim(str, `"`)

	parsed := FilterOperatorValues[str]
	if parsed == OperatorUnspecified {
		return fmt.Errorf("%w: %s", ErrInvalidFilterOperator, string(data))
	}

	*op = parsed
	return nil
}

func (op FilterOperator) MarshalJSON() ([]byte, error) { return []byte(op.String()), nil }

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// This is used to decode operator from plain text.
func (op *FilterOperator) UnmarshalText(text []byte) error {
	parsed := FilterOperatorValues[string(text)]
	if parsed == OperatorUnspecified {
		return fmt.Errorf("%w: %s", ErrInvalidFilterOperator, string(text))
	}

	*op = parsed
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
// This is used to encode operator as plain text.
func (op FilterOperator) MarshalText() ([]byte, error) {
	return []byte(op.String()), nil
}
