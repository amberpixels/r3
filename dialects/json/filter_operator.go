package r3json

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

var ErrInvalidFilterOperator = errors.New("invalid filter operator")

type JSONFilterOperator int8

const (
	OperatorUnspecified JSONFilterOperator = iota
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
	FilterOperatorNames = map[JSONFilterOperator]string{
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
	FilterOperatorValues = map[string]JSONFilterOperator{
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

// String is implemented for debugging purposes, so the JSONFilterOperator is a fmt.Stringer.
func (op JSONFilterOperator) String() string { return FilterOperatorNames[op] }

// UnmarshalJSON implements the json.Unmarshaler interface.
// It expects the JSON value to be a string representing the operator.
func (op *JSONFilterOperator) UnmarshalJSON(data []byte) error {
	str := string(data)
	str = strings.Trim(str, `"`)

	parsed := FilterOperatorValues[str]
	if parsed == OperatorUnspecified {
		return fmt.Errorf("%w: %s", ErrInvalidFilterOperator, string(data))
	}

	*op = parsed
	return nil
}

func (op JSONFilterOperator) MarshalJSON() ([]byte, error) {
	return []byte(`"` + op.String() + `"`), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// This is used to decode operator from plain text.
func (op *JSONFilterOperator) UnmarshalText(text []byte) error {
	parsed := FilterOperatorValues[string(text)]
	if parsed == OperatorUnspecified {
		return fmt.Errorf("%w: %s", ErrInvalidFilterOperator, string(text))
	}

	*op = parsed
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
// This is used to encode operator as plain text.
func (op JSONFilterOperator) MarshalText() ([]byte, error) {
	return []byte(op.String()), nil
}

func (op JSONFilterOperator) ToFilterOperatorSpec() (r3.FilterOperatorSpec, error) {
	switch op {
	case OperatorEq:
		return r3.OperatorEq, nil
	case OperatorNe:
		return r3.OperatorNe, nil
	case OperatorExists:
		return r3.OperatorExists, nil
	case OperatorGt:
		return r3.OperatorGt, nil
	case OperatorGte:
		return r3.OperatorGte, nil
	case OperatorLt:
		return r3.OperatorLt, nil
	case OperatorLte:
		return r3.OperatorLte, nil
	case OperatorBetween:
		return r3.OperatorBetween, nil
	case OperatorBetweenEx:
		return r3.OperatorBetweenEx, nil
	case OperatorBetweenExInc:
		return r3.OperatorBetweenExInc, nil
	case OperatorBetweenIncEx:
		return r3.OperatorBetweenIncEx, nil
	case OperatorIn:
		return r3.OperatorIn, nil
	case OperatorNotIn:
		return r3.OperatorNotIn, nil
	case OperatorLike:
		return r3.OperatorLike, nil
	case OperatorNotLike:
		return r3.OperatorNotLike, nil
	case OperatorILike:
		return r3.OperatorILike, nil
	case OperatorUnspecified:
		fallthrough
	default:
		return r3.OperatorUnspecified, fmt.Errorf("unsupported JSON filter operator: %s", op)
	}
}
