package r3

import (
	"errors"
	"fmt"
)

type FilterOperator interface {
	// Stringer is needed for debugging purposes, so each filter can be printed.
	fmt.Stringer

	// ToDialect converts the FilterOperatorSpec into its dialect-specific representation.
	// E.g. r3 => SQL (Where Clause, etc)
	ToDialect(FilterOperatorOutboundDialector) (DialectValue, error)

	// FromDialect makes up a FilterOperatorSpec from an FieldInboundDialector and its DialectValue
	// e.g. JSON => r3
	FromDialect(FilterOperatorInboundDialector, DialectValue) error
}

var _ FilterOperator = (*FilterOperatorSpec)(nil)

// FilterOperatorSpec represents an operator to be used in a filter.
// For now, not all r3 operators are supported by every possible dialect.
// But idea is to provide here in r3 operator the most possibly full list of operators that we need.
// TODO: might be refactor into a more complex struct
type FilterOperatorSpec int8

const (
	OperatorUnspecified  FilterOperatorSpec = iota
	OperatorEq                              // =
	OperatorNe                              // !=
	OperatorExists                          // exists
	OperatorGt                              // >
	OperatorGte                             // >=
	OperatorLt                              // <
	OperatorLte                             // <=
	OperatorBetween                         // between_inc meaning []
	OperatorBetweenEx                       // between_exc meaning ()
	OperatorBetweenExInc                    // between_exc_inc meaning (]
	OperatorBetweenIncEx                    // between_inc_exc meaning [)
	OperatorIn                              // in
	OperatorNotIn                           // not in
	OperatorLike                            // like
	OperatorNotLike                         // not like
	OperatorILike                           // ilike (like + case insensitive)
)

// String is implemented for debugging purposes, so the FilterOperatorSpec is a fmt.Stringer.
// Note: Protect with the `exhausted` linter.
func (op *FilterOperatorSpec) String() string {
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

type (
	// FilterOperatorOutboundDialector defines a contract for converting a FilterOperatorSpec
	// to its dialect-specific representation.
	FilterOperatorOutboundDialector interface {
		TranslateFilterOperatorSpec(op *FilterOperatorSpec) (DialectValue, error)
	}

	FilterOperatorInboundDialector interface {
		TranslateIntoFilterOperatorSpec(op DialectValue) (*FilterOperatorSpec, error)
	}
)

// ToDialect converts the FilterOperatorSpec into a dialect-specific value
// e.g. r3 => SQL.
func (op *FilterOperatorSpec) ToDialect(dialector FilterOperatorOutboundDialector) (DialectValue, error) {
	return dialector.TranslateFilterOperatorSpec(op)
}

// FromDialect makes up a FilterOperatorSpec from an FieldInboundDialector and its DialectValue
// e.g. JSON => r3.
func (op *FilterOperatorSpec) FromDialect(dialector FilterOperatorInboundDialector, inValue DialectValue) error {
	if op == nil {
		return errors.New("FromDialect must be called on a non-nil FilterOperatorSpec")
	}

	translated, err := dialector.TranslateIntoFilterOperatorSpec(inValue)
	if err != nil {
		return fmt.Errorf("inbound dialector fialed: %w", err)
	}

	*op = *translated
	return nil
}
