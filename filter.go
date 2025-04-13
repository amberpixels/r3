package r3

import (
	"encoding/json"
	"fmt"
)

// Filter stands for generic Filter condition interface.
// For now, we have 1 implementation:
//  1. ColumnFilter - a single column from the destination.
type Filter interface {
	// Stringer is needed for debugging purposes, so each filter can be printed.
	fmt.Stringer

	// ToDialect converts the Filter into its dialect-specific representation.
	ToDialect(FilterDialector) (DialectValue, error)
}

// Filters represents a list of Filters.
// It is intentionally a slice of interface, so any filter can be inside.
type Filters []Filter

type (
	// FilterDialector is a generic dialector (visitor) interface for conversion.
	FilterDialector interface {
		FromColumnFilter(cf *ColumnFilter) (DialectValue, error)
	}

	// FilterInboundDialector is a generic inbound dialector (visitor) interface for conversion.
	FilterInboundDialector interface {
		ToFilter(f DialectValue) (Filter, error)
	}
)

// MergeWith merges (combines) filters with other filters.
func (fs Filters) MergeWith(other Filters) Filters { return mergeWith(fs, other) }

// ColumnFilter represents a filtering criteria with a field, an operator, and a value.
type ColumnFilter struct {
	Field    Field
	Operator FilterOperator
	Value    any

	// Children groups:
	// Note: When using AND/OR the Field,Operator,Value fields of the parent filter are ignored.

	// AND Children should be declared inside AND
	And Filters
	// OR Children should be declared inside OR
	Or Filters
}

func (f *ColumnFilter) ToDialect(dialector FilterDialector) (DialectValue, error) {
	return dialector.FromColumnFilter(f)
}

// String returns just a string representation of the filter (ColunFilter as a json)
// As all fields are exported, we're OK with this.
func (f *ColumnFilter) String() string {
	jj, err := json.Marshal(f)
	if err != nil {
		return fmt.Sprintf("<corrupted filted (%s)>", err.Error())
	}

	return string(jj)
}

//
// Auxiliary helpers for quick filter scaffolding:
//

// NewColumnFilter is a simple constructor for the ColumnFilter (not an AND/OR group).
func NewColumnFilter(field Field, operator FilterOperator, value any) *ColumnFilter {
	return &ColumnFilter{Field: field, Operator: operator, Value: value}
}

// Fop is a shorthand for NewFilter()
// Fop is "F" for filter and "op" for operator.
func Fop(field Field, operator FilterOperator, value any) *ColumnFilter {
	return NewColumnFilter(field, operator, value)
}

// F is a shorthand for NewColumnFilter(field, OperatorEq, value).
func F(field Field, value any) *ColumnFilter      { return Fop(field, OperatorEq, value) }
func FLike(field Field, value any) *ColumnFilter  { return Fop(field, OperatorLike, value) }
func FILike(field Field, value any) *ColumnFilter { return Fop(field, OperatorILike, value) }

// And is a constructor of AND group of filters.
func And(filters ...Filter) *ColumnFilter { return &ColumnFilter{And: filters} }

// Or is a constructor of OR group of filters.
func Or(filters ...Filter) *ColumnFilter { return &ColumnFilter{Or: filters} }
