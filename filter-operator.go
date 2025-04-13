package r3

import (
	"fmt"
)

var ErrUnsupportedOperator = fmt.Errorf("unsupported operator")

// FilterOperator represents an operator to be used in a filter.
// For now, not all r3 operators are supported by every possible dialect.
// But idea is to provide here in r3 operator the most possibly full list of operators that we need.
type FilterOperator int8

const (
	OperatorUnspecified  FilterOperator = iota
	OperatorEq                          // =
	OperatorNe                          // !=
	OperatorExists                      // exists
	OperatorGt                          // >
	OperatorGte                         // >=
	OperatorLt                          // <
	OperatorLte                         // <=
	OperatorBetween                     // between_inc meaning []
	OperatorBetweenEx                   // between_exc meaning ()
	OperatorBetweenExInc                // between_exc_inc meaning (]
	OperatorBetweenIncEx                // between_inc_exc meaning [)
	OperatorIn                          // in
	OperatorNotIn                       // not in
	OperatorLike                        // like
	OperatorNotLike                     // not like
	OperatorILike                       // ilike (like + case insensitive)
)

// String is implemented for debugging purposes, so the FilterOperator is a fmt.Stringer.
// Note: Protect with the `exhausted` linter.
func (op FilterOperator) String() string {
	switch op {
	case OperatorEq:
		return "eq"
	case OperatorNe:
		return "ne"
	case OperatorExists:
		return "exists"
	case OperatorGt:
		return "gt"
	case OperatorGte:
		return "gte"
	case OperatorLt:
		return "lt"
	case OperatorLte:
		return "lte"
	case OperatorBetween:
		return "between"
	case OperatorBetweenEx:
		return "between_exc"
	case OperatorBetweenExInc:
		return "between_exc_inc"
	case OperatorBetweenIncEx:
		return "between_inc_exc"
	case OperatorLike:
		return "like"
	case OperatorNotLike:
		return "notlike"
	case OperatorILike:
		return "ilike"
	case OperatorIn:
		return "in"
	case OperatorNotIn:
		return "not in"
	case OperatorUnspecified:
		return "unspecified"
	default:
		return ""
	}
}

// FilterOperatorDialector defines a contract for converting a FilterOperator
// to its dialect-specific representation.
type FilterOperatorDialector interface {
	Eq() DialectValue
	Ne() DialectValue
	Exists() DialectValue
	Gt() DialectValue
	Gte() DialectValue
	Lt() DialectValue
	Lte() DialectValue
	Between() DialectValue
	BetweenEx() DialectValue
	BetweenExInc() DialectValue
	BetweenIncEx() DialectValue
	In() DialectValue
	NotIn() DialectValue
	Like() DialectValue
	NotLike() DialectValue
	ILike() DialectValue
}

// ToDialect converts the FilterOperator into a dialect-specific value
// using the provided dialector (visitor).
// Note: Protect with the `exhausted` linter.
func (op FilterOperator) ToDialect(dialector FilterOperatorDialector) (DialectValue, error) {
	switch op {
	case OperatorEq:
		return dialector.Eq(), nil
	case OperatorNe:
		return dialector.Ne(), nil
	case OperatorExists:
		return dialector.Exists(), nil
	case OperatorGt:
		return dialector.Gt(), nil
	case OperatorGte:
		return dialector.Gte(), nil
	case OperatorLt:
		return dialector.Lt(), nil
	case OperatorLte:
		return dialector.Lte(), nil
	case OperatorBetween:
		return dialector.Between(), nil
	case OperatorBetweenEx:
		return dialector.BetweenEx(), nil
	case OperatorBetweenExInc:
		return dialector.BetweenExInc(), nil
	case OperatorBetweenIncEx:
		return dialector.BetweenIncEx(), nil
	case OperatorIn:
		return dialector.In(), nil
	case OperatorNotIn:
		return dialector.NotIn(), nil
	case OperatorLike:
		return dialector.Like(), nil
	case OperatorNotLike:
		return dialector.NotLike(), nil
	case OperatorILike:
		return dialector.ILike(), nil
	case OperatorUnspecified:
		fallthrough
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedOperator, op)
	}
}
