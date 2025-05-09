package r3

import (
	"errors"
	"fmt"
)

// todo delete?
var ErrUnsupportedOperator = errors.New("unsupported operator")

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
func (op *FilterOperator) String() string {
	if op == nil {
		return ""
	}

	switch *op {
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

// FilterOperatorOutboundDialector defines a contract for converting a FilterOperator
// to its dialect-specific representation.
type FilterOperatorOutboundDialector interface {
	TranslateFilterOperator(op *FilterOperator) (DialectValue, error)
}

type FilterOperatorInboundDialector interface {
	TranslateIntoFilterOperator(op DialectValue) (*FilterOperator, error)
}

// ToDialect converts the FilterOperator into a dialect-specific value
// e.g. r3 => SQL
func (op *FilterOperator) ToDialect(dialector FilterOperatorOutboundDialector) (DialectValue, error) {
	return dialector.TranslateFilterOperator(op)
}

// FromDialect makes up a FilterOperator from an FieldInboundDialector and its DialectValue
// e.g. JSON => r3
func (op *FilterOperator) FromDialect(dialector FilterOperatorInboundDialector, inValue DialectValue) error {
	if op == nil {
		return fmt.Errorf("FromDialect must be called on a non-nil FilterOperator")
	}

	translated, err := dialector.TranslateIntoFilterOperator(inValue)
	if err != nil {
		return fmt.Errorf("inbound dialector fialed: %w", err)
	}

	*op = *translated
	return nil
}

// Note: make it a public interface
// As probably we'll have something more complex for operators e.g. structs
// that will hold some more complex options for operators (e.g. case sensitivity, strictness, etc)
// for now we declare a private interface
type iFilterOperator interface {
	// Stringer is needed for debugging purposes, so each filter can be printed.
	fmt.Stringer

	// ToDialect converts the FilterOperator into its dialect-specific representation.
	// E.g. r3 => SQL (Where Clause, etc)
	ToDialect(FilterOperatorOutboundDialector) (DialectValue, error)

	// FromDialect makes up a FilterOperator from an FieldInboundDialector and its DialectValue
	// e.g. JSON => r3
	FromDialect(FilterOperatorInboundDialector, DialectValue) error
}

var _ iFilterOperator = (*FilterOperator)(nil)
