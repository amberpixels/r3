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

	// Relation, when non-empty, makes this a relationship ("has") filter: it
	// matches rows whose declared relation `Relation` (by the same struct field
	// name used for preloads) has at least one related row satisfying all of
	// RelationFilter. Field/Operator/Value/And/Or are ignored when Relation is
	// set.
	//
	// Relationship filters are resolved by the driver into a key-set In filter
	// before SQL translation (see the GORM driver's relation lowering), so they
	// work on any backend regardless of native subquery support. A dialect may
	// later compile them natively (EXISTS) as an optimization.
	// omitempty so a non-relationship filter serializes exactly as before (the
	// relationship fields are absent unless used).
	Relation       string  `json:",omitempty"`
	RelationFilter Filters `json:",omitempty"`
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
	clone.RelationFilter = f.RelationFilter.Clone()
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

//
// Ergonomic short-form helpers.
//
// These take a plain field name (string) instead of a *FieldSpec and cover the
// common case without ceremony: r3.Eq("name", "Berlin"), r3.Gt("age", 18).
// They are sugar over Fop + NewFieldSpec; reach for F/Fop with a hand-built
// FieldSpec when you need table hints or nested paths.

// Eq builds a `field = value` filter.
func Eq(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorEq, value) }

// Ne builds a `field != value` filter.
func Ne(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorNe, value) }

// Gt builds a `field > value` filter.
func Gt(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorGt, value) }

// Gte builds a `field >= value` filter.
func Gte(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorGte, value) }

// Lt builds a `field < value` filter.
func Lt(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorLt, value) }

// Lte builds a `field <= value` filter.
func Lte(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorLte, value) }

// In builds a `field IN (values...)` filter. The value is typically a slice.
func In(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorIn, value) }

// NotIn builds a `field NOT IN (values...)` filter. The value is typically a slice.
func NotIn(field string, value any) *FilterSpec {
	return Fop(NewFieldSpec(field), OperatorNotIn, value)
}

// Like builds a case-sensitive `field LIKE value` filter.
func Like(field string, value any) *FilterSpec { return Fop(NewFieldSpec(field), OperatorLike, value) }

// NotLike builds a case-sensitive `field NOT LIKE value` filter.
func NotLike(field string, value any) *FilterSpec {
	return Fop(NewFieldSpec(field), OperatorNotLike, value)
}

// ILike builds a case-insensitive `field ILIKE value` filter.
func ILike(field string, value any) *FilterSpec {
	return Fop(NewFieldSpec(field), OperatorILike, value)
}

// Exists builds a `field exists` filter (presence check). The value is the
// expected existence as a bool where the backend supports it.
func Exists(field string, value any) *FilterSpec {
	return Fop(NewFieldSpec(field), OperatorExists, value)
}

// Between builds an inclusive `field BETWEEN lo AND hi` filter.
func Between(field string, lo, hi any) *FilterSpec {
	return Fop(NewFieldSpec(field), OperatorBetween, []any{lo, hi})
}

// Has builds a relationship ("has") filter: it matches rows whose declared
// relation `relation` (by struct field name — the same name used for preloads)
// has at least one related row satisfying all of `inner`. Example:
//
//	r3.Has("Squads", r3.In("id", []int64{1, 3}))  // rows linked to squad 1 or 3
//
// The inner filters are evaluated against the related entity. Drivers resolve
// the relation to a key set and rewrite this to an In filter, so it works on
// every backend regardless of native subquery support.
//
// Resolution happens in the driver, so a Has filter does not round-trip through
// the serialization dialects (json/url/yaml/toml) — build it in Go. When used
// as a permission scope, enforcing it on Get requires permissions.WithIDFunc.
func Has(relation string, inner ...*FilterSpec) *FilterSpec {
	return &FilterSpec{Relation: relation, RelationFilter: inner}
}

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
