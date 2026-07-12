package r3

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Filters represents a list of *FilterSpec.
type Filters []*FilterSpec

// MergeWith combines these filters with other.
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

	// And and Or hold child groups. When either is set, the parent's
	// Field/Operator/Value are ignored.

	And Filters
	Or  Filters

	// Relation, when non-empty, makes this a relationship ("has") filter matching
	// rows whose declared relation (by preload struct-field name) has at least one
	// related row satisfying RelationFilter. Field/Operator/Value/And/Or are then
	// ignored. The driver lowers it to a key-set In filter before SQL translation,
	// so it works on any backend regardless of native subquery support.
	// omitempty keeps a non-relationship filter serializing exactly as before.
	Relation       string  `json:",omitempty"`
	RelationFilter Filters `json:",omitempty"`

	// RelationNegate inverts a relationship filter (see [HasNo]): match rows whose
	// relation has NO related row satisfying RelationFilter - an anti-join /
	// NOT EXISTS. Only meaningful with Relation set; the driver lowers it to a
	// NOT-IN key set.
	RelationNegate bool `json:",omitempty"`
}

// String renders the filter as JSON.
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

// Auxiliary helpers for quick filter scaffolding.

// NewFilterSpec constructs a FilterSpec (not an AND/OR group).
func NewFilterSpec(field *FieldSpec, operator FilterOperatorSpec, value any) *FilterSpec {
	return &FilterSpec{Field: field, Operator: operator, Value: value}
}

// NewFilterSpecAndGroup wraps filters in an AND group.
func NewFilterSpecAndGroup(filters ...*FilterSpec) *FilterSpec {
	return &FilterSpec{And: filters}
}

// NewFilterSpecOrGroup wraps filters in an OR group.
func NewFilterSpecOrGroup(filters ...*FilterSpec) *FilterSpec {
	return &FilterSpec{Or: filters}
}

// Fop is shorthand for [NewFilterSpec] ("F" for filter, "op" for operator).
func Fop(field *FieldSpec, operator FilterOperatorSpec, value any) *FilterSpec {
	return NewFilterSpec(field, operator, value)
}

// F is a shorthand for NewFilterSpec(field, OperatorEq, value).
func F(field *FieldSpec, value any) *FilterSpec      { return Fop(field, OperatorEq, value) }
func FLike(field *FieldSpec, value any) *FilterSpec  { return Fop(field, OperatorLike, value) }
func FILike(field *FieldSpec, value any) *FilterSpec { return Fop(field, OperatorILike, value) }

// Short-form helpers take a plain field name instead of a *FieldSpec for the
// common case: r3.Eq("name", "Berlin"), r3.Gt("age", 18). They are sugar over
// Fop + NewFieldSpec; use F/Fop with a hand-built FieldSpec for table hints or
// nested paths.

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
// relation (by preload struct-field name) has at least one related row
// satisfying all of inner, evaluated against the related entity. Example:
//
//	r3.Has("Squads", r3.In("id", []int64{1, 3}))  // rows linked to squad 1 or 3
//
// The driver resolves the relation to a key set and rewrites this to an In
// filter, so it works on every backend regardless of native subquery support.
// It does not round-trip through the serialization dialects - build it in Go.
// As a permission scope, enforcing it on Get requires permissions.WithIDFunc.
func Has(relation string, inner ...*FilterSpec) *FilterSpec {
	return &FilterSpec{Relation: relation, RelationFilter: inner}
}

// HasNo is the negated (anti-join) counterpart of [Has]: it matches rows whose
// declared relation has NO related row satisfying inner - a NOT EXISTS. With no
// inner filters it matches rows with no related row at all. Example:
//
//	r3.HasNo("Translations")                 // rows with no translation yet
//	r3.HasNo("Squads", r3.Eq("archived", true)) // rows in no archived squad
//
// Like [Has], the driver resolves it to a key set (a NOT-IN filter); it works on
// every backend and does not round-trip through the serialization dialects.
func HasNo(relation string, inner ...*FilterSpec) *FilterSpec {
	return &FilterSpec{Relation: relation, RelationFilter: inner, RelationNegate: true}
}

// And is a shortcut for NewFilterSpecAndGroup.
func And(filters ...*FilterSpec) *FilterSpec { return NewFilterSpecAndGroup(filters...) }

// Or is a shortcut for NewFilterSpecOrGroup.
func Or(filters ...*FilterSpec) *FilterSpec { return NewFilterSpecOrGroup(filters...) }

// ExtractBetweenBounds returns the low and high bounds from a between value,
// which must be a slice or array of exactly 2 elements.
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
