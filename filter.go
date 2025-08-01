package r3

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Filter stands for generic Filter condition interface.
type Filter interface {
	// Stringer is needed for debugging purposes, so each filter can be printed.
	fmt.Stringer

	// ToDialect converts the Filter (r3=>SQL) into its dialect-specific representation.
	ToDialect(FilterOutboundDialector) (DialectValue, error)

	// FromDialect makes up the Filter (JSON=>r3) from an FieldInboundDialector and its DialectValue
	FromDialect(FilterInboundDialector, DialectValue) error
}

// Filters represents a list of Filters.
// It is intentionally a slice of interface, so any filter can be inside.
type Filters []Filter

// MergeWith merges (combines) filters with other filters.
func (fs Filters) MergeWith(other Filters) Filters { return mergeWith(fs, other) }

type (
	// FilterOutboundDialector is a generic dialector (visitor) interface for conversion.
	FilterOutboundDialector interface {
		TranslateFilterSpec(cf *FilterSpec) (DialectValue, error)
	}

	// FilterInboundDialector is a generic inbound dialector (visitor) interface for conversion.
	FilterInboundDialector interface {
		TranslateIntoFilterSpec(f DialectValue) (*FilterSpec, error)
	}
)

// FilterSpec represents a filtering criteria with a field, an operator, and a value.
type FilterSpec struct {
	Field    *FieldSpec
	Operator FilterOperatorSpec
	Value    any

	// Children groups:
	// Note: When using AND/OR the Field,Operator,Value fields of the parent filter are ignored.

	// AND Children should be declared inside AND
	And Filters
	// OR Children should be declared inside OR
	Or Filters
}

func (f *FilterSpec) ToDialect(dialector FilterOutboundDialector) (DialectValue, error) {
	return dialector.TranslateFilterSpec(f)
}

func (f *FilterSpec) FromDialect(dialector FilterInboundDialector, inValue DialectValue) error {
	if f == nil {
		return errors.New("FromDialect must be called on a non-nil FilterSpec")
	}

	translated, err := dialector.TranslateIntoFilterSpec(inValue)
	if err != nil {
		return fmt.Errorf("inbound dialector failed: %w", err)
	}

	*f = *translated
	return nil
}

// String returns just a string representation of the filter (ColunFilter as a json)
// As all fields are exported, we're OK with this.
func (f *FilterSpec) String() string {
	jj, err := json.Marshal(f)
	if err != nil {
		return fmt.Sprintf("<corrupted filted (%s)>", err.Error())
	}

	return string(jj)
}

//
// Auxiliary helpers for quick filter scaffolding:
//

// NewFilterSpec constructs a FilterSpec (not an AND/OR group).
func NewFilterSpec(field *FieldSpec, operator FilterOperatorSpec, value any) *FilterSpec {
	return &FilterSpec{Field: field, Operator: operator, Value: value}
}

func NewFilterSpecAndGroup(filters ...Filter) *FilterSpec {
	return &FilterSpec{And: filters}
}
func NewFilterSpecOrGroup(filters ...Filter) *FilterSpec {
	return &FilterSpec{Or: filters}
}

// Fop is a shorthand for NewFilter()
// Fop is "F" for filter and "op" for operator.
func Fop(field *FieldSpec, operator FilterOperatorSpec, value any) *FilterSpec {
	return NewFilterSpec(field, operator, value)
}

// F is a shorthand for NewFilterSpec(field, OperatorEq, value).
func F(field *FieldSpec, value any) *FilterSpec      { return Fop(field, OperatorEq, value) }
func FLike(field *FieldSpec, value any) *FilterSpec  { return Fop(field, OperatorLike, value) }
func FILike(field *FieldSpec, value any) *FilterSpec { return Fop(field, OperatorILike, value) }

// And is a shortcut for NewFilterSpecAndGroup.
func And(filters ...Filter) *FilterSpec { return NewFilterSpecAndGroup(filters...) }

// Or is a shortcut for NewFilterSpecOrGroup.
func Or(filters ...Filter) *FilterSpec { return NewFilterSpecOrGroup(filters...) }
