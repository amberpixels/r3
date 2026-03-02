package r3

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Filters represents a list of *FilterSpec.
type Filters []*FilterSpec

// MergeWith merges (combines) filters with other filters.
func (fs Filters) MergeWith(other Filters) Filters { return mergeWith(fs, other) }

// Clone returns a safe full-clone of the filters list.
func (fs Filters) Clone() Filters {
	cloned := make(Filters, len(fs))
	for i, f := range fs {
		cloned[i] = f.Clone()
	}
	return cloned
}

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

// String returns just a string representation of the filter (as JSON).
// As all fields are exported, we're OK with this.
func (f *FilterSpec) String() string {
	jj, err := json.Marshal(f)
	if err != nil {
		return fmt.Sprintf("<corrupted filter (%s)>", err.Error())
	}

	return string(jj)
}

// Clone returns a deep clone of the filter.
func (f *FilterSpec) Clone() *FilterSpec {
	clone := *f
	clone.Field = f.Field.Clone()
	clone.And = f.And.Clone()
	clone.Or = f.Or.Clone()
	return &clone
}

//
// Auxiliary helpers for quick filter scaffolding:
//

// NewFilterSpec constructs a FilterSpec (not an AND/OR group).
func NewFilterSpec(field *FieldSpec, operator FilterOperatorSpec, value any) *FilterSpec {
	return &FilterSpec{Field: field, Operator: operator, Value: value}
}

func NewFilterSpecAndGroup(filters ...*FilterSpec) *FilterSpec {
	return &FilterSpec{And: filters}
}
func NewFilterSpecOrGroup(filters ...*FilterSpec) *FilterSpec {
	return &FilterSpec{Or: filters}
}

// Fop is a shorthand for NewFilterSpec()
// Fop is "F" for filter and "op" for operator.
func Fop(field *FieldSpec, operator FilterOperatorSpec, value any) *FilterSpec {
	return NewFilterSpec(field, operator, value)
}

// F is a shorthand for NewFilterSpec(field, OperatorEq, value).
func F(field *FieldSpec, value any) *FilterSpec      { return Fop(field, OperatorEq, value) }
func FLike(field *FieldSpec, value any) *FilterSpec  { return Fop(field, OperatorLike, value) }
func FILike(field *FieldSpec, value any) *FilterSpec { return Fop(field, OperatorILike, value) }

// And is a shortcut for NewFilterSpecAndGroup.
func And(filters ...*FilterSpec) *FilterSpec { return NewFilterSpecAndGroup(filters...) }

// Or is a shortcut for NewFilterSpecOrGroup.
func Or(filters ...*FilterSpec) *FilterSpec { return NewFilterSpecOrGroup(filters...) }

// ExtractBetweenBounds extracts low and high values from a between filter value.
// The value must be a slice or array with exactly 2 elements.
func ExtractBetweenBounds(value any) (any, any, error) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, nil, fmt.Errorf("between operator requires a slice/array value with 2 elements, got %T", value)
	}
	if rv.Len() != 2 {
		return nil, nil, fmt.Errorf("between operator requires exactly 2 elements, got %d", rv.Len())
	}
	return rv.Index(0).Interface(), rv.Index(1).Interface(), nil
}
