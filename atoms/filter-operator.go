package r3atoms

import (
	"fmt"
	"strings"
)

// FilterOperator represents an operator for filtering queries,
// compatible with the operators GORM supports.
type FilterOperator int8

const (
	OperatorUnspecified FilterOperator = iota
	OperatorEq                         // =
	OperatorNe                         // !=
	OperatorGt                         // >
	OperatorGte                        // >=
	OperatorLt                         // <
	OperatorLte                        // <=
	OperatorLike                       // LIKE
	OperatorNotLike                    // NOT LIKE
	OperatorILike                      // ILIKE
	OperatorIn                         // IN
	OperatorNotIn                      // NOT IN
)

// DialectString returns the Dialect (SQL-ready) representation of the FilterOperator.
func (op FilterOperator) DialectString() string {
	switch op {
	case OperatorEq:
		return "="
	case OperatorNe:
		return "!="
	case OperatorGt:
		return ">"
	case OperatorGte:
		return ">="
	case OperatorLt:
		return "<"
	case OperatorLte:
		return "<="
	case OperatorLike:
		return "LIKE"
	case OperatorNotLike:
		return "NOT LIKE"
	case OperatorILike:
		return "ILIKE"
	case OperatorIn:
		return "IN"
	case OperatorNotIn:
		return "NOT IN"
	default:
		return ""
	}
}

// String is implemented for debugging purposes, so the ColumnFilter is a fmt.Stringer.
func (op FilterOperator) String() string { return op.DialectString() }

// filterOperatorFromString creates a FilterOperator from a string
// It accepts both symbolic and word representations (e.g., "=", "eq", "gt", etc.).
// Note: FilterOperatorUnspecified means given string is not a valid FilterOperator.
func filterOperatorFromString(s string) FilterOperator {
	switch strings.ToLower(s) {
	case "=", "eq":
		return OperatorEq
	case "!=", "ne":
		return OperatorNe
	case ">", "gt":
		return OperatorGt
	case ">=", "gte":
		return OperatorGte
	case "<", "lt":
		return OperatorLt
	case "<=", "lte":
		return OperatorLte
	case "like":
		return OperatorLike
	case "not like":
		return OperatorNotLike
	case "ilike":
		return OperatorILike
	case "in":
		return OperatorIn
	case "not in":
		return OperatorNotIn
	default:
		return OperatorUnspecified
	}
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// This is used to decode operator from plain text.
func (op *FilterOperator) UnmarshalText(text []byte) error {
	parsed := filterOperatorFromString(string(text))
	if parsed == OperatorUnspecified {
		return fmt.Errorf("invalid filter operator: %s", text)
	}

	*op = parsed
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
// This is used to encode operator as plain text.
func (op FilterOperator) MarshalText() ([]byte, error) {
	return []byte(op.String()), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It expects the JSON value to be a string representing the operator.
func (op *FilterOperator) UnmarshalJSON(data []byte) error {
	str := string(data)
	str = strings.Trim(str, `"`)

	parsed := filterOperatorFromString(str)
	if parsed == OperatorUnspecified {
		return fmt.Errorf("invalid filter operator: %s", string(data))
	}

	*op = parsed
	return nil
}

func (op FilterOperator) MarshalJSON() ([]byte, error) {
	return []byte(op.String()), nil
}
