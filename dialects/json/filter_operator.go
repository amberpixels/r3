package r3json

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// ErrInvalidFilterOperator is returned when a filter operator string cannot be parsed.
var ErrInvalidFilterOperator = errors.New("invalid filter operator")

// JSONFilterOperator represents a filter operator in JSON format.
//
//nolint:recvcheck // codec pattern: value receivers marshal, pointer receivers unmarshal; pointer-only would stop values satisfying json/text Marshaler
type JSONFilterOperator int8

// JSON filter operator constants.
const (
	OperatorUnspecified      JSONFilterOperator = iota
	OperatorEq                                  // eq, ==
	OperatorNe                                  // ne, !=
	OperatorExists                              // exists
	OperatorGt                                  // gt, >
	OperatorGte                                 // gte, >=
	OperatorLt                                  // lt, <
	OperatorLte                                 // lte, <=
	OperatorBetween                             // between (inclusive both)
	OperatorBetweenEx                           // between_exc (exclusive both)
	OperatorBetweenExInc                        // between_exc_inc
	OperatorBetweenIncEx                        // between_inc_exc
	OperatorIn                                  // in
	OperatorNotIn                               // nin
	OperatorLike                                // like, *=
	OperatorNotLike                             // notlike, !*=
	OperatorILike                               // ilike, **=
	OperatorWeekdayIn                           // weekday_in
	OperatorTimeOfDayBetween                    // tod_between
)

// Enum value maps for JSONFilterOperator.
var (
	// FilterOperatorNames maps operator constants to their canonical string names.
	FilterOperatorNames = map[JSONFilterOperator]string{
		OperatorUnspecified:      "",
		OperatorEq:               "eq",
		OperatorNe:               "ne",
		OperatorExists:           "exists",
		OperatorGt:               "gt",
		OperatorGte:              "gte",
		OperatorLt:               "lt",
		OperatorLte:              "lte",
		OperatorBetween:          "between",
		OperatorBetweenEx:        "between_exc",
		OperatorBetweenExInc:     "between_exc_inc",
		OperatorBetweenIncEx:     "between_inc_exc",
		OperatorIn:               "in",
		OperatorNotIn:            "nin",
		OperatorLike:             "like",
		OperatorNotLike:          "notlike",
		OperatorILike:            "ilike",
		OperatorWeekdayIn:        "weekday_in",
		OperatorTimeOfDayBetween: "tod_between",
	}
	// FilterOperatorValues maps string representations (including aliases) to operator constants.
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
		"weekday_in":      OperatorWeekdayIn,
		"tod_between":     OperatorTimeOfDayBetween,
	}
)

// String returns the operator's canonical name.
func (op JSONFilterOperator) String() string { return FilterOperatorNames[op] }

// UnmarshalJSON implements json.Unmarshaler, expecting a quoted operator string.
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

// MarshalJSON implements json.Marshaler.
func (op JSONFilterOperator) MarshalJSON() ([]byte, error) {
	return []byte(`"` + op.String() + `"`), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (op *JSONFilterOperator) UnmarshalText(text []byte) error {
	parsed := FilterOperatorValues[string(text)]
	if parsed == OperatorUnspecified {
		return fmt.Errorf("%w: %s", ErrInvalidFilterOperator, string(text))
	}

	*op = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (op JSONFilterOperator) MarshalText() ([]byte, error) {
	return []byte(op.String()), nil
}

// ToFilterOperatorSpec converts a JSONFilterOperator to an r3.FilterOperatorSpec.
func (op JSONFilterOperator) ToFilterOperatorSpec() (r3.FilterOperatorSpec, error) {
	name := FilterOperatorNames[op]
	if name == "" {
		return r3.OperatorUnspecified, fmt.Errorf("unsupported JSON filter operator: %s", op)
	}
	return canonical.ParseFilterOperator(name)
}
